package attack

import (
	"net"
	"testing"
)

// csum16 of an all-zero buffer is 0; a known byte pair must round-trip to zero
// when the checksum field is included (the internet checksum property).
func TestCsum16(t *testing.T) {
	// A buffer whose contents already sum to zero must verify as 0.
	buf := []byte{0x12, 0x34, 0x56, 0x78}
	c := csum16(buf)
	// Replacing the last two bytes with the computed checksum makes the sum zero.
	buf2 := append(append([]byte{}, buf...), byte(c>>8), byte(c&0xff))
	if got := csum16(buf2); got != 0 {
		t.Fatalf("checksum self-verify = %#x, want 0", got)
	}
}

func TestFillSyn(t *testing.T) {
	src := net.IPv4(10, 0, 0, 1)
	dst := net.IPv4(93, 184, 216, 5)
	buf := make([]byte, 40)
	fillSyn(buf, src, dst, 6900)

	if buf[0] != 0x45 {
		t.Errorf("IP version/IHL = %#x, want 0x45", buf[0])
	}
	if buf[9] != 6 {
		t.Errorf("IP proto = %d, want 6 (TCP)", buf[9])
	}
	if got := csum16(buf[0:20]); got != 0 {
		t.Errorf("IP header checksum self-verify = %#x, want 0", got)
	}
	if buf[33] != 0x02 {
		t.Errorf("TCP flags = %#x, want 0x02 (SYN)", buf[33])
	}
	if buf[32]>>4 != 5 {
		t.Errorf("TCP data offset = %d, want 5", buf[32]>>4)
	}
	// TCP checksum over pseudo-header + segment must self-verify to zero.
	pseudo := []byte{10, 0, 0, 1, 93, 184, 216, 5, 0, 6, 0, 20}
	if got := csum16(pseudo, buf[20:40]); got != 0 {
		t.Errorf("TCP checksum self-verify = %#x, want 0", got)
	}
}
