package app

import (
	"fmt"

	"github.com/lezhnev/arena/engine/lang"
)

// swap is a validated Edit: the program to hand to a bot's brain at the start
// of a beat.
type swap struct {
	beat   int
	bot    string
	prog   *lang.Program
	reboot int // beats off the map; 0 = a plain hot-swap
}

// swappable is a brain whose program can be replaced between beats.
type swappable interface{ Swap(*lang.Program) }

// prober is a brain that exposes state scenarios can assert on.
type prober interface{ Probe() map[string]int }

// swaps validates the edit log against the brains the record built. Errors
// name the entry, e.g. "edit_log[1]: ...".
func (rec Record) swaps(known map[string]bool) ([]swap, error) {
	out := make([]swap, 0, len(rec.EditLog))
	for i, e := range rec.EditLog {
		s, err := e.swap(known)
		if err != nil {
			return nil, fmt.Errorf("edit_log[%d]: %w", i, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// known holds the ids of the program bots an edit may target.
func (e Edit) swap(known map[string]bool) (swap, error) {
	if e.Beat < 1 {
		return swap{}, fmt.Errorf("beat %d: edits start at beat 1", e.Beat)
	}
	if e.Reboot < 0 {
		return swap{}, fmt.Errorf("bot %s: reboot %d is negative", e.Bot, e.Reboot)
	}
	if e.Brain.Kind != KindProgram {
		return swap{}, fmt.Errorf("bot %s: only %q edits are supported, got %q", e.Bot, KindProgram, e.Brain.Kind)
	}
	if !known[e.Bot] {
		return swap{}, fmt.Errorf("bot %q is not a program bot on the map", e.Bot)
	}
	p, err := compile(e.Brain.Src)
	if err != nil {
		return swap{}, fmt.Errorf("bot %s: %w", e.Bot, err)
	}
	return swap{beat: e.Beat, bot: e.Bot, prog: p, reboot: e.Reboot}, nil
}

// applySwaps hands the edits of this beat to their brains, in log order.
func (m *Match) applySwaps(beat int) {
	for _, s := range m.swaps {
		if s.beat == beat {
			m.fail(m.apply(s))
		}
	}
}

// apply plays one logged edit: a hot-swap, or a reboot with the new program.
func (m *Match) apply(s swap) error {
	if s.reboot > 0 {
		return m.Reboot(s.bot, s.prog, s.reboot)
	}
	return m.Swap(s.bot, s.prog)
}

// Reboot hands a validated program to the bot's brain and takes the bot off
// the map for n beats; it comes back by the respawn-spot rule, running the new
// program from a fresh loop() with its HP, gun, memory and dice. A bot that
// is already rebooting only gets the newer program: the countdown goes on.
func (m *Match) Reboot(id string, p *lang.Program, n int) error {
	if err := m.Swap(id, p); err != nil {
		return err
	}
	m.world, _ = m.world.Reboot(id, n)
	return nil
}

// Swap hands a validated program to the bot's brain; the bot keeps its memory,
// HP and reload and starts a fresh loop() at the next beat.
func (m *Match) Swap(id string, p *lang.Program) error {
	s, ok := m.brains[id].(swappable)
	if !ok {
		return fmt.Errorf("swap: bot %q is not a program bot in the arena", id)
	}
	s.Swap(p)
	return nil
}

// Probes is the inspectable state of the program bots, keyed "<bot>.<name>"
// (num_0..num_7, pin_0_set / pin_0_x / pin_0_y .. pin_7_*, act).
func (m *Match) Probes() map[string]int {
	out := map[string]int{}
	for id, b := range m.brains {
		if p, ok := b.(prober); ok {
			for k, v := range p.Probe() {
				out[id+"."+k] = v
			}
		}
	}
	return out
}

// BotState is the inspectable state of one program bot: its memory (numbers and pins), the kind of
// its latest action (-1 before the first) and its trail, newest first.
type BotState struct {
	InArena bool
	Num     [lang.NumSize]int
	Pin     [lang.PinSize]lang.Place
	Act     int
	Trail   []lang.TrailEntry
}

type inspectable interface {
	prober
	Num() [lang.NumSize]int
	Pins() [lang.PinSize]lang.Place
	Trail() []lang.TrailEntry
}

// Inspect describes the program bot id; the zero BotState (not in the arena)
// for any other bot.
func (m *Match) Inspect(id string) BotState {
	b, ok := m.brains[id].(inspectable)
	if !ok {
		return BotState{Act: -1}
	}
	return BotState{InArena: true, Num: b.Num(), Pin: b.Pins(), Act: b.Probe()["act"], Trail: b.Trail()}
}
