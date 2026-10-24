package lang

import (
	"fmt"
	"strconv"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// ScriptBrain runs a BOTSCRIPT program as a brain.Brain.
//
// Semantics decided here (LANGUAGE.md section 3 quoted where it decides):
//   - "Thinking is free; an action call queues the action and blocks": each
//     Decide resumes the script, which runs until its next action. Every
//     beat is exactly one action.
//   - "A bot that hasn't queued an action within its budget misses the tick
//     (stands still, trail act IDLE) and its code continues": Decide returns
//     Wait for that beat, the trail records IDLE, and the next Decide resumes
//     the script where it stopped, with a fresh budget.
//   - "if loop() returns without an action, that beat is a wait": a loop()
//     invocation that returns having queued nothing makes that beat a wait,
//     then loop() starts again on the next beat. One that did act restarts at
//     once, in the same beat.
//   - "a hit or bump this beat aborts the current call stack; the next beat
//     runs the handler": the Outcome in the Perception is the previous
//     beat's, so a Decide that sees Last.Hit unwinds the stack and starts
//     onHit(HitDir), whose first action is this beat's action.
//   - "When the handler returns, loop() starts fresh."
//   - "Hit beats bump": with both, onHit runs. If there is no onHit the hit
//     is no reaction and a bump in the same beat still runs onBump.
//   - "A new hit during onHit restarts it; a bump during onHit is ignored.
//     No handler = no reaction." A hit during onBump runs onHit; a bump
//     during onBump restarts it (the spec is silent; same rule as hits).
//   - "Hot-swap: new code starts at a fresh loop(). Memory, readings, trail,
//     HP and reload are kept." Swap takes effect at the next Decide; an
//     interrupt arriving with it is judged against the new program, with the
//     stack counted as loop() (so a bump is not ignored as "during onHit").
//   - The trail is kept by the brain from successive Perceptions and its own
//     actions, newest first, at most 3. An IDLE entry takes its ok from the
//     sim (a wait, so true). A gap in beat numbers clears it.
//   - Death and respawn (not in LANGUAGE.md): when Perception.Life changes
//     (it counts respawns, so it works with RespawnBeats = 0 where no beat
//     number is skipped) the script restarts at a fresh loop(): the stack and
//     the handler mode are dropped and the trail is cleared. Memory and dice
//     are kept. The first outcome of a new life is never an interrupt.
//   - After Err() or Close() the brain only waits and stops reading
//     perceptions, so the trail and memory stay frozen.
//
// All methods must be called from one goroutine (the match's). The script's
// goroutine is private; call Close when the match is over.
type ScriptBrain struct {
	in *Interp
	st *stepper
	p  brain.Perception

	pending *Program
	trail   []TrailEntry
	prev    *Action // what the previous beat played (IDLE kept as 4)
	prevAt  int     // its beat number

	nav *Nav // built from the first perception's terrain (static per match)

	missed int // beats lost to the op budget (IDLE)

	life      int  // Perception.Life of the latest beat
	respawned bool // the latest beat is the first of a new life
}

// NewScriptBrain returns a brain running p with dice seeded by seed (see
// SeedFor). Nothing runs until the first Decide.
func NewScriptBrain(p *Program, seed uint64) *ScriptBrain {
	b := &ScriptBrain{}
	b.in = NewInterp(p, scriptEnv{b}, nil, seed)
	b.st = newStepper(b.in)
	b.in.acts = b.st
	return b
}

// navigator is the brain's own walking-distance cache.
func (b *ScriptBrain) navigator() *Nav {
	if b.nav == nil {
		b.nav = NewNav(b.p.Grid)
	}
	return b.nav
}

// Decide implements brain.Brain.
func (b *ScriptBrain) Decide(p brain.Perception) brain.Action {
	if b.st.dead {
		return brain.Action{Kind: brain.Wait}
	}
	b.observe(p)
	a, ok := b.st.step(b.directive(p.Last))
	if !ok {
		return brain.Action{Kind: brain.Wait}
	}
	b.prev, b.prevAt = &a, p.Beat
	if a.Kind == ActIdle {
		b.missed++
	}
	return toBrain(a)
}

func toBrain(a Action) brain.Action {
	if a.Kind == ActIdle {
		return brain.Action{Kind: brain.Wait}
	}
	return brain.Action{Kind: a.Kind.BrainKind(), Dir: geo.Dir(a.Dir)}
}

// Missed is how many beats the script lost by spending its op budget.
func (b *ScriptBrain) Missed() int { return b.missed }

// Fallthroughs is how many beats were a wait only because loop() returned
// without queueing an action.
func (b *ScriptBrain) Fallthroughs() int { return b.st.fell }

// Swap replaces the program at the next Decide (hot-swap).
func (b *ScriptBrain) Swap(p *Program) { b.pending = p }

// Num returns the memory cells num[0..7].
func (b *ScriptBrain) Num() [NumSize]int { return b.in.Mem.Num }

// Pins returns the places pin[0..7].
func (b *ScriptBrain) Pins() [PinSize]Place { return b.in.Mem.Pin }

// Trail returns the trail, newest first.
func (b *ScriptBrain) Trail() []TrailEntry { return append([]TrailEntry(nil), b.trail...) }

// Err is the interpreter bug that stopped the script, if any. The brain then
// waits forever instead of hanging the match.
func (b *ScriptBrain) Err() error {
	if b.st.fault == nil {
		return nil
	}
	return fmt.Errorf("script stopped: %v", b.st.fault)
}

// Close stops the script's goroutine. It is safe to call more than once; a
// closed brain waits.
func (b *ScriptBrain) Close() { b.st.close() }

// observe takes in the new perception and the result of the previous beat.
func (b *ScriptBrain) observe(p brain.Perception) {
	b.p = p
	b.respawned, b.life = p.Life != b.life, p.Life
	switch {
	case b.respawned:
		b.trail = nil
	case b.prev == nil:
	case p.Beat == b.prevAt+1:
		b.trail = pushed(b.trail, newEntry(*b.prev, p.Last, p.Pos))
	default:
		b.trail = nil
	}
	b.prev = nil
}

// directive decides what the script does at the start of this beat.
func (b *ScriptBrain) directive(o brain.Outcome) directive {
	mode := b.st.mode
	fresh := b.respawned || b.pending != nil
	if b.pending != nil {
		b.in.Prog, b.pending, b.in.line = b.pending, nil, 0
	}
	if fresh {
		mode = entLoop
	}
	if e, ok := b.interrupt(o, mode); ok && !b.respawned {
		return directive{enter: true, to: e}
	}
	return directive{enter: fresh}
}

// interrupt applies the section 3 rules to the previous beat's outcome while
// the stack is running entry point mode.
func (b *ScriptBrain) interrupt(o brain.Outcome, mode entryKind) (entry, bool) {
	prog := b.in.Prog
	if o.Hit && prog.OnHit != nil {
		return entry{entHit, int(o.HitDir)}, true
	}
	if o.Bumped && prog.OnBump != nil && mode != entHit {
		return entry{entBump, int(o.BumpDir)}, true
	}
	return entry{}, false
}

// Probe is the state a scenario can assert on, as name -> value: the memory
// (num_0..num_7; pin_0_set, pin_0_x, pin_0_y .. pin_7_*, -1 for none), line (the source line of the latest action call,
// 0 before the first or after a hot-swap) and act, the kind of the action played on the
// latest beat (SCAN 0, MOVE 1, FIRE 2, WAIT 3, IDLE 4; -1 before the first).
func (b *ScriptBrain) Probe() map[string]int {
	act := -1
	if b.prev != nil {
		act = int(b.prev.Kind)
	}
	p := map[string]int{"act": act, "line": b.in.Line()}
	for i, v := range b.in.Mem.Num {
		p["num_"+strconv.Itoa(i)] = v
	}
	for i, pl := range b.in.Mem.Pin {
		k := "pin_" + strconv.Itoa(i)
		p[k+"_set"], p[k+"_x"], p[k+"_y"] = b2i(pl.Set), -1, -1
		if pl.Set {
			p[k+"_x"], p[k+"_y"] = pl.X, pl.Y
		}
	}
	return p
}
