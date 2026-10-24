package lang_test

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/lang"
)

func TestNavAccepted(t *testing.T) {
	rows := []struct{ name, src string }{
		{"path of a pin", inLoop("move(path(pin[0]));")},
		{"path of at", inLoop("num[0] = path(at(3, 4));")},
		{"steps of here", inLoop("num[0] = steps(here());")},
		{"steps of none", inLoop("num[0] = steps(none);")},
		{"steps of a place ternary", inLoop("num[0] = steps(num[1] ? pin[0] : here());")},
		{"steps in a condition", inLoop("if (steps(pin[0]) > 3) wait();")},
		{"nearest", inLoop("num[0] = nearest();")},
		{"nearest indexes pin", inLoop("pin[1] = pin[nearest()];")},
		{"steps arithmetic", inLoop("num[0] = steps(pin[0]) + path(pin[1]) * 2;")},
		{"path from a helper", "function loop() {\n  f(1);\n}\nfunction f(p) {\n  move(path(pin[p]));\n}\n"},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if _, errs := lang.Validate(r.src); len(errs) != 0 {
				t.Fatalf("want accepted, got %v", errs)
			}
		})
	}
}

func TestNavRejected(t *testing.T) {
	rows := []rejectRow{
		rejectAt("path of a number", "num[0] = path(5);", 17, "'path' takes a place", "at(x, y)"),
		rejectAt("steps of a reading", "num[0] = steps(x);", 18, "'steps' takes a place", "pin[i]"),
		rejectAt("steps of memory", "num[0] = steps(num[0]);", 18, "'steps' takes a place", "here()"),
		rejectAt("path of a place and number ternary", "num[0] = path(num[1] ? here() : 5);", 35, "a place and a number", "both"),
		rejectAt("path no argument", "num[0] = path();", 12, "path expects 1", "path(place)"),
		rejectAt("steps two arguments", "num[0] = steps(here(), here());", 12, "steps expects 1", "steps(place)"),
		rejectAt("nearest argument", "num[0] = nearest(1);", 12, "nearest expects 0", "nearest()"),
		rejectAt("path into a pin", "pin[0] = path(pin[1]);", 12, "a pin holds a place", "here()"),
		{"function named path", "function loop() {}\nfunction path() {}", 2, 10, "path", "built-in"},
		{"param named steps", "function loop() {}\nfunction f(steps) {}", 2, 12, "steps", "built-in"},
		{"function named nearest", "function loop() {}\nfunction nearest() {}", 2, 10, "nearest", "built-in"},
	}
	runRejected(t, rows)
}

// navRig runs src on a rig standing at (x, y) of the ASCII map.
func navRig(t *testing.T, mapText string, x, y int, src string) *rig {
	t.Helper()
	m, err := geo.ParseMap(mapText)
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(t, src)
	r.env.grid = &m.Grid
	r.env.read["x"], r.env.read["y"] = x, y
	r.in.Run(r.prog.Loop)
	return r
}

const pillar = "...\n.#.\n..."

func TestNavBuiltins(t *testing.T) {
	rows := []struct {
		name, mapText string
		x, y          int
		expr          string
		want          int
	}{
		{"steps in the open", "....", 0, 0, "steps(at(3, 0))", 3},
		{"steps to my own cell", "....", 2, 0, "steps(here())", 0},
		{"steps around a pillar", pillar, 0, 0, "steps(at(2, 2))", 4},
		{"steps to a wall", pillar, 0, 0, "steps(at(1, 1))", -1},
		{"steps off the map", pillar, 0, 0, "steps(at(3, 0))", -1},
		{"steps off the map negative", pillar, 0, 0, "steps(at(0, -1))", -1},
		{"steps to none", pillar, 0, 0, "steps(none)", -1},
		{"steps to a sealed pocket", ".#.", 0, 0, "steps(at(2, 0))", -1},
		{"steps through a pin", pillar, 0, 0, "steps(pin[0])", -1},
		{"path east", "....", 0, 0, "path(at(3, 0))", 1},
		{"path west", "....", 3, 0, "path(at(0, 0))", 3},
		{"path south", ".\n.\n.", 0, 0, "path(at(0, 2))", 2},
		{"path north", ".\n.\n.", 0, 2, "path(at(0, 0))", 0},
		{"path tie prefers east over south", "...\n...", 0, 0, "path(at(1, 1))", 1},
		{"path tie prefers north over west", "...\n...", 1, 1, "path(at(0, 0))", 0},
		{"path tie prefers east over west", "...\n...", 1, 0, "path(at(1, 0))", -1},
		{"path tie prefers south over west", "...\n...", 2, 0, "path(at(1, 1))", 2},
		{"path round a pillar goes east first", pillar, 0, 0, "path(at(2, 2))", 1},
		{"path round a pillar from the east side", pillar, 2, 0, "path(at(0, 2))", 2},
		{"path on my own cell", "....", 1, 0, "path(here())", -1},
		{"path to a wall", pillar, 0, 0, "path(at(1, 1))", -1},
		{"path off the map", pillar, 0, 0, "path(at(9, 9))", -1},
		{"path to none", pillar, 0, 0, "path(none)", -1},
		{"path to a sealed pocket", ".#.", 0, 0, "path(at(2, 0))", -1},
		{"path standing in a wall", pillar, 1, 1, "path(at(0, 0))", -1},
		{"steps standing off the map", pillar, 5, 5, "steps(at(0, 0))", -1},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			src := "function loop() {\n  num[0] = " + r.expr + ";\n}\n"
			if got := navRig(t, r.mapText, r.x, r.y, src).in.Mem.Num[0]; got != r.want {
				t.Fatalf("%s from (%d,%d) = %d, want %d", r.expr, r.x, r.y, got, r.want)
			}
		})
	}
}

func TestNavWithoutAMapIsUnreachable(t *testing.T) {
	r := runLoop(t, inLoop("num[0] = steps(here()); num[1] = path(at(1, 1)); num[2] = nearest();"))
	if got := r.in.Mem.Num[:3]; got[0] != -1 || got[1] != -1 || got[2] != -1 {
		t.Fatalf("got %v, want -1 -1 -1", got)
	}
}

func TestNearest(t *testing.T) {
	const room = ".....\n.###.\n....."
	rows := []struct {
		name, pins string
		want       int
	}{
		{"no pins", "", -1},
		{"one pin", "pin[3] = at(4, 0);", 3},
		{"closest by walking, not by straight line", "pin[0] = at(2, 2); pin[1] = at(0, 2);", 1},
		{"tie goes to the lowest index", "pin[5] = at(2, 0); pin[2] = at(0, 2); pin[4] = at(0, 2);", 2},
		{"my own cell wins", "pin[0] = at(4, 0); pin[1] = here();", 1},
		{"unreachable pins are skipped", "pin[0] = at(2, 1); pin[1] = at(9, 9); pin[2] = at(4, 2);", 2},
		{"only unreachable pins", "pin[0] = at(2, 1); pin[1] = at(-1, 0);", -1},
		{"empty pins are skipped", "pin[7] = at(1, 0);", 7},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			src := "function loop() {\n  " + r.pins + "\n  num[0] = nearest();\n}\n"
			if got := navRig(t, room, 0, 0, src).in.Mem.Num[0]; got != r.want {
				t.Fatalf("nearest() = %d, want %d", got, r.want)
			}
		})
	}
}

func TestNavOpsCountLikeAnyCall(t *testing.T) {
	rows := []struct {
		expr string
		ops  int
	}{
		{"nearest()", 4},      // statement, assignment, index, call
		{"steps(none)", 5},    // + the place argument
		{"path(at(1, 2))", 7}, // + at and its two arguments (the place node and the call are one op each)
		{"steps(pin[0])", 6},
		{"steps(here())", 5},
	}
	for _, r := range rows {
		t.Run(r.expr, func(t *testing.T) {
			got := navRig(t, pillar, 0, 0, inLoop("num[0] = "+r.expr+";")).in.Ops()
			// the cost is exactly the same with a map as without one: no BFS cost
			if want := runLoop(t, inLoop("num[0] = "+r.expr+";")).in.Ops(); got != want {
				t.Fatalf("ops = %d with a map, %d without", got, want)
			}
			if got != r.ops {
				t.Fatalf("ops = %d, want %d", got, r.ops)
			}
		})
	}
}

func TestNavScriptBrainUsesPerceptionGrid(t *testing.T) {
	m, err := geo.ParseMap(pillar)
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, "function loop() {\n  pin[0] = at(2, 2);\n  move(path(pin[0]));\n}\n")
	h.p.Grid = m.Grid
	h.p.Pos = geo.Pos{X: 0, Y: 0}
	if got := h.beat(); got != act(lang.ActMove.BrainKind(), geo.E) {
		t.Fatalf("first step = %v, want move E", got)
	}
	h.p.Pos = geo.Pos{X: 2, Y: 0}
	if got := h.calm().beat(); got != act(lang.ActMove.BrainKind(), geo.S) {
		t.Fatalf("second step = %v, want move S", got)
	}
}
