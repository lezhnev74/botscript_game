// Package app holds the use cases: the clock-free Match beat loop, Records and
// their Replay, event-log hashing and match statistics.
package app

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// Match owns a World and the brains that drive its bots. It has no clock: the
// caller decides when a beat happens by calling Step. Scenarios, Replay and
// cmd/sim all run through it so there is a single beat loop.
type Match struct {
	world  sim.World
	rules  sim.Rules
	brains map[string]brain.Brain
	swaps  []swap // hot-swaps, applied at the start of their beat
	leaves []Leave
	joins  []joinOp
	err    error         // first failure of a scheduled op (see Err)
	notes  []event.Event // Joined and Left since the last beat; logged at the start of the next
}

// NewMatch starts a match from w. brains must hold one brain per bot id.
func NewMatch(w sim.World, r sim.Rules, brains map[string]brain.Brain) *Match {
	return &Match{world: w, rules: r, brains: brains}
}

// World is the state after the last beat.
func (m *Match) World() sim.World { return m.world }

// Step plays one beat: Decide from every live bot in ID order, Resolve, and
// tag the events with the beat number.
func (m *Match) Step() []event.Logged {
	beat := m.world.Beat + 1
	m.applyLeaves(beat)
	m.applyJoins(beat)
	m.applySwaps(beat)
	acts := make(map[string]brain.Action, len(m.world.Bots))
	for _, b := range m.world.Bots {
		acts[b.ID] = m.brains[b.ID].Decide(m.world.Perception(b.ID, beat))
	}
	next, evs := sim.Resolve(m.world, acts, m.rules)
	m.world = next
	logged := make([]event.Logged, 0, len(m.notes)+len(evs))
	for _, e := range append(m.notes, evs...) {
		logged = append(logged, event.Logged{Beat: beat, Event: e})
	}
	m.notes = nil
	return logged
}

// Close releases the brains that hold resources (BOTSCRIPT goroutines). It is
// safe to call more than once. Call it when the match is over.
func (m *Match) Close() {
	closeAll(m.brains)
	m.closeUnplayed()
}

func (m *Match) closeUnplayed() {
	for _, j := range m.joins {
		closeBrain(j.brain)
	}
	m.joins = nil
}

func closeAll(brains map[string]brain.Brain) {
	for _, b := range brains {
		closeBrain(b)
	}
}

func closeBrain(b brain.Brain) {
	if c, ok := b.(interface{ Close() }); ok {
		c.Close()
	}
}
