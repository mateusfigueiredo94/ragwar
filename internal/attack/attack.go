// Package attack implements the DDoS attack engines used by the worker.
package attack

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Options configures a single attack run.
type Options struct {
	Target   net.IP        // destination address (IPv4)
	Port     uint16        // destination port
	PPS      uint64        // target packets-per-second (flood attacks)
	Conns    int           // connections to hold open (hold attack)
	Payload  int           // UDP payload size in bytes
	Workers  int           // sender goroutines (0 = auto)
	Duration time.Duration // informational; the caller controls ctx
}

// Stats is a cumulative snapshot of an attack run.
type Stats struct {
	Packets uint64 // packets sent on the wire
	Bytes   uint64 // bytes sent (headers included)
	Conns   int64  // currently open connections (hold)
	Opened  uint64 // total connections established (hold)
	Errors  uint64 // send/dial failures
}

// Attack is a single attack engine.
type Attack interface {
	Name() string
	// Start runs the attack until ctx is canceled, then returns.
	Start(ctx context.Context, opts Options) error
	Stats() Stats
}

var (
	mu       sync.Mutex
	registry = map[string]func() Attack{}
)

// Register adds an attack constructor (called from init).
func Register(name string, ctor func() Attack) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = ctor
}

// Names returns the registered attack types, sorted.
func Names() []string {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// New builds an attack engine by name.
func New(name string) (Attack, error) {
	mu.Lock()
	ctor, ok := registry[name]
	mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("unknown attack type %q (available: %v)", name, Names())
	}
	return ctor(), nil
}

// ParseTarget parses "ip:port" into an IPv4 address and port.
func ParseTarget(s string) (net.IP, uint16, error) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return nil, 0, fmt.Errorf("target must be ip:port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return nil, 0, fmt.Errorf("IPv4 target required, got %q", host)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, 0, fmt.Errorf("invalid port %q", portStr)
	}
	return ip.To4(), uint16(port), nil
}

// autoWorkers picks a sensible default sender count.
func autoWorkers() int {
	w := 2 * runtime.GOMAXPROCS(0)
	if w > 32 {
		w = 32
	}
	return w
}

// perWorkerInterval converts a total pps budget into the per-goroutine pacing interval.
func perWorkerInterval(workers int, pps uint64) time.Duration {
	if workers <= 0 {
		workers = autoWorkers()
	}
	if pps == 0 {
		return time.Second / 1000 // sane default pace when the caller left pps unset
	}
	return time.Duration(float64(time.Second) * float64(workers) / float64(pps))
}

// paceLoop runs fn repeatedly, pacing each call to `interval`, until ctx is done.
// Intervals below ~2µs can't be honored by the runtime timer, so it degrades to
// a tight send loop (max NIC rate) — the measured pps is what actually matters.
func paceLoop(ctx context.Context, interval time.Duration, fn func()) {
	if interval < 2*time.Microsecond {
		for ctx.Err() == nil {
			fn()
		}
		return
	}
	next := time.Now()
	for ctx.Err() == nil {
		now := time.Now()
		if next.After(now) {
			time.Sleep(next.Sub(now))
			now = time.Now()
		}
		fn()
		next = next.Add(interval)
		if next.Before(now) {
			next = now // fell behind; resync instead of bursting to catch up
		}
	}
}
