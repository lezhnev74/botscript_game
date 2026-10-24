package sim

import (
	"slices"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
)

// beatCtx is the scratch state of one beat. It owns copies of everything
// phases mutate, so Resolve never touches its input World.
type beatCtx struct {
	w      World
	acts   map[string]brain.Action
	rules  Rules
	events []event.Event

	carried int    // bullets already flying when the beat began (they come first in w.Bullets)
	hits    []hit  // bullet-vs-bot contacts found by bulletPhase, applied by hitPhase
	shots   []shot // sounds (real shots) made this beat, in bot order, for hearPhase
}

// Phase is one step of the beat (moves, shots, bullets, scans, ...). Phases run in
// order over the same beatCtx.
type Phase func(*beatCtx)

// phases is the beat order from VISION: moves, new shots, hearing, reload, bullets fly, scans, hits, coins, respawn.
var phases = []Phase{movePhase, firePhase, hearPhase, reloadPhase, bulletPhase, scanPhase, hitPhase, coinPhase, respawnPhase, rebootPhase}

func newBeatCtx(w World, acts map[string]brain.Action, r Rules) *beatCtx {
	w.Bots = slices.Clone(w.Bots)
	w.Coins = slices.Clone(w.Coins)
	w.Fallen = slices.Clone(w.Fallen)
	w.Rebooting = slices.Clone(w.Rebooting)
	w.Beat++
	w.Bullets = slices.Clone(w.Bullets)
	for i := range w.Bots {
		a := acts[w.Bots[i].ID]
		w.Bots[i].Last = brain.Outcome{Act: a, OK: true}
	}
	return &beatCtx{w: w, acts: acts, rules: r, carried: len(w.Bullets)}
}

func (c *beatCtx) emit(e event.Event) { c.events = append(c.events, e) }

// Resolve plays one beat: it runs every phase and returns the new World and
// the events, in phase order. It is pure: w is not modified.
func Resolve(w World, acts map[string]brain.Action, r Rules) (World, []event.Event) {
	c := newBeatCtx(w, acts, r)
	for _, p := range phases {
		p(c)
	}
	return c.w, c.events
}
