package lang_test

import (
	"runtime"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/lang"
)

func mustProgram(t *testing.T, src string) *lang.Program {
	t.Helper()
	p, errs := lang.Validate(src)
	if len(errs) > 0 {
		t.Fatalf("source rejected: %v\n%s", errs, src)
	}
	return p
}

// harness drives a ScriptBrain with hand-made perceptions, one per beat.
type harness struct {
	t *testing.T
	b *lang.ScriptBrain
	p brain.Perception
}

func newHarness(t *testing.T, src string) *harness {
	t.Helper()
	b := lang.NewScriptBrain(mustProgram(t, src), 1)
	t.Cleanup(b.Close)
	return &harness{t: t, b: b, p: brain.Perception{
		Pos: geo.Pos{X: 1, Y: 1}, HP: 3, Loaded: true,
		Last:  brain.Outcome{OK: true},
		Width: 7, Height: 5, Open: [4]bool{true, true, true, true},
	}}
}

// beat plays the next beat with the current perception and returns the action.
func (h *harness) beat() brain.Action {
	h.p.Beat++
	return h.b.Decide(h.p)
}

// beats plays n beats with the same perception and returns the actions.
func (h *harness) beats(n int) []brain.Action {
	var out []brain.Action
	for i := 0; i < n; i++ {
		out = append(out, h.beat())
	}
	return out
}

// then sets the outcome the next beat will see.
func (h *harness) then(o brain.Outcome) *harness {
	h.p.Last = o
	return h
}

func (h *harness) hitFrom(d geo.Dir) *harness {
	return h.then(brain.Outcome{OK: true, Hit: true, HitDir: d})
}

func (h *harness) bumpFrom(d geo.Dir) *harness {
	return h.then(brain.Outcome{Bumped: true, BumpDir: d})
}

func (h *harness) calm() *harness { return h.then(brain.Outcome{OK: true}) }

func act(k brain.Kind, d geo.Dir) brain.Action { return brain.Action{Kind: k, Dir: d} }

var wait = brain.Action{Kind: brain.Wait}

func (h *harness) want(got brain.Action, w brain.Action) {
	h.t.Helper()
	if got != w {
		h.t.Fatalf("beat %d: got %v, want %v", h.p.Beat, got, w)
	}
}

// mem asserts num[0..2]; the other cells must still be 0.
func (h *harness) mem(a, b, c int) {
	h.t.Helper()
	if w, got := [8]int{a, b, c}, h.b.Num(); got != w {
		h.t.Fatalf("num %v, want %v", got, w)
	}
}

// settle lets an exited goroutine finish unwinding without a wall clock.
func settle(limit int, done func() bool) bool {
	for i := 0; i < limit; i++ {
		if done() {
			return true
		}
		runtime.Gosched()
	}
	return done()
}
