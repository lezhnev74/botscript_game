package lang_test

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/lang"
)

// fakeEnv is a hand-made world view. Readings absent from the map are 0.
type fakeEnv struct {
	read  map[string]int
	trail []map[string]int
	walls map[int]bool    // directions that are walls
	cells map[[2]int]bool // wall cells for wall(x, y); everything else is floor
	heard lang.Place      // the heard reading
	grid  *geo.Grid       // terrain for steps / path (nil = no map: everything unreachable)
}

func (e *fakeEnv) Reading(name string) int { return e.read[name] }
func (e *fakeEnv) TrailLen() int           { return len(e.trail) }
func (e *fakeEnv) TrailField(i int, f string) int {
	return e.trail[i][f]
}
func (e *fakeEnv) Open(d int) bool    { return !e.walls[d] }
func (e *fakeEnv) Steps(x, y int) int { return e.nav().Steps(e.at(), geo.Pos{X: x, Y: y}) }
func (e *fakeEnv) Path(x, y int) int  { return e.nav().Path(e.at(), geo.Pos{X: x, Y: y}) }
func (e *fakeEnv) Heard() lang.Place {
	return e.heard
}
func (e *fakeEnv) Wall(x, y int) bool { return e.cells[[2]int{x, y}] }

// fakeActs records action calls and budget stalls.
type fakeActs struct {
	in        *lang.Interp
	calls     []lang.Action
	opsAtCall []int
	stalls    int
	opsAtStal []int
	onDo      func(n int) // optional hook, called with the call count so far
}

func (f *fakeActs) Do(a lang.Action) {
	f.calls = append(f.calls, a)
	f.opsAtCall = append(f.opsAtCall, f.in.Ops())
	if f.onDo != nil {
		f.onDo(len(f.calls))
	}
}

func (f *fakeActs) OutOfBudget() {
	f.stalls++
	f.opsAtStal = append(f.opsAtStal, f.in.Ops())
}

type rig struct {
	in   *lang.Interp
	env  *fakeEnv
	acts *fakeActs
	prog *lang.Program
}

func newRig(t *testing.T, src string) *rig {
	t.Helper()
	p, errs := lang.Validate(src)
	if len(errs) > 0 {
		t.Fatalf("source rejected: %v\n%s", errs, src)
	}
	env := &fakeEnv{read: map[string]int{}}
	acts := &fakeActs{}
	in := lang.NewInterp(p, env, acts, 1)
	acts.in = in
	return &rig{in: in, env: env, acts: acts, prog: p}
}

// runLoop runs loop() once and returns the rig.
func runLoop(t *testing.T, src string) *rig {
	t.Helper()
	r := newRig(t, src)
	r.in.Run(r.prog.Loop)
	return r
}

// evalA runs "num[0] = <expr>;" in loop and returns num[0].
func evalA(t *testing.T, expr string) int {
	t.Helper()
	return runLoop(t, "function loop() {\n  num[0] = "+expr+";\n}\n").in.Mem.Num[0]
}

func (e *fakeEnv) at() geo.Pos { return geo.Pos{X: e.read["x"], Y: e.read["y"]} }

func (e *fakeEnv) nav() *lang.Nav {
	if e.grid == nil {
		return lang.NewNav(geo.Grid{})
	}
	return lang.NewNav(*e.grid)
}
