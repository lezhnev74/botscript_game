package lang_test

import (
	"testing"

	"github.com/lezhnev/arena/engine/lang"
)

func TestPinAccepted(t *testing.T) {
	rows := []struct{ name, src string }{
		{"pin here", inLoop("pin[0] = here();")},
		{"pin at", inLoop("pin[1] = at(3, 4);")},
		{"pin at expressions", inLoop("pin[7] = at(x + 1, y - num[0]);")},
		{"pin none", inLoop("pin[2] = none;")},
		{"pin copy", inLoop("pin[0] = pin[1];")},
		{"pin computed index", inLoop("pin[num[0]] = here();")},
		{"pin index from a parameter", "function loop() {\n  f(1);\n}\nfunction f(p) {\n  pin[p] = here();\n}\n"},
		{"compare with none", inLoop("if (pin[0] == none) wait();")},
		{"compare not none", inLoop("if (pin[0] != none) wait();")},
		{"compare two pins", inLoop("if (pin[0] == pin[1] || pin[2] != pin[3]) wait();")},
		{"compare here and at", inLoop("if (here() == pin[0] || at(1, 2) != here() || none == none) wait();")},
		{"comparison is a number", inLoop("num[0] = (pin[0] == none) + 1;")},
		{"place fields", inLoop("num[0] = pin[0].x + pin[1].y;")},
		{"fields of other places", inLoop("num[0] = none.x + here().y + at(1, 2).x;")},
		{"field in a condition", inLoop("if (pin[num[0]].x > 3) wait();")},
		{"free and count", inLoop("num[0] = free() + count();")},
		{"place ternary", inLoop("pin[0] = num[1] ? here() : none;")},
		{"place ternary fields", inLoop("num[0] = (num[1] ? pin[0] : pin[1]).x;")},
		{"place ternary compared", inLoop("if ((num[1] ? pin[0] : none) == pin[2]) wait();")},
		{"pin as for init and update", inLoop("for (pin[0] = here(); pin[0] != none; pin[0] = none) { wait(); }")},
		{"pin in a handler", "function loop() {}\nfunction onHit(d) { pin[0] = here(); }\n"},
		{"pin index is a number", inLoop("pin[pin[0].x] = here();")},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if _, errs := lang.Validate(r.src); len(errs) != 0 {
				t.Fatalf("want accepted, got %v", errs)
			}
		})
	}
}

func TestPinRejected(t *testing.T) {
	const notNumber = "not a number"
	rows := []rejectRow{
		rejectAt("pin gets a number", "pin[0] = 5;", 12, "a pin holds a place", "at(x, y) or here()"),
		rejectAt("pin gets memory", "pin[0] = num[1];", 12, "a pin holds a place", "here()"),
		rejectAt("pin gets a reading", "pin[0] = x;", 12, "a pin holds a place", "here()"),
		rejectAt("pin add assign", "pin[0] += here();", 3, "places have no arithmetic", "pin[i] = at(x, y)"),
		rejectAt("pin sub assign", "pin[0] -= here();", 3, "places have no arithmetic", "="),
		rejectAt("pin increment", "pin[0]++;", 3, "places have no arithmetic", "="),
		rejectAt("pin prefix decrement", "--pin[0];", 3, "places have no arithmetic", "="),
		rejectAt("pin index too big", "pin[8] = here();", 3, "pin[8] does not exist", "pin[0]..pin[7]"),
		rejectAt("pin index negative", "pin[-1] = here();", 3, "pin[-1] does not exist", "pin[0]..pin[7]"),
		rejectAt("pin index too big on read", "num[0] = pin[8].x;", 12, "pin[8] does not exist", "pin[0]..pin[7]"),
		rejectAt("pin plus number", "num[0] = pin[0] + 1;", 12, notNumber, "places have no arithmetic: use pin[0].x"),
		rejectAt("number plus place", "num[0] = 1 + here();", 16, notNumber, "here().x"),
		rejectAt("place into memory", "num[0] = pin[0];", 12, notNumber, "pin[0].x"),
		rejectAt("place into memory via ternary", "num[0] = num[1] ? here() : none;", 12, notNumber, "places have no arithmetic"),
		rejectAt("place as condition", "if (pin[0]) wait();", 7, notNumber, "== / !="),
		rejectAt("negated place", "num[0] = -pin[0];", 13, notNumber, "pin[0].x"),
		rejectAt("place ordering", "num[0] = pin[0] < pin[1];", 12, notNumber, "pin[0].x"),
		rejectAt("place and", "num[0] = pin[0] && 1;", 12, notNumber, "pin[0].x"),
		rejectAt("place compared with a number", "num[0] = pin[0] == 5;", 22, "compare a place with a place", "none"),
		rejectAt("number compared with a place", "num[0] = 5 != none;", 12, "compare a place with a place", "none"),
		rejectAt("none as a number", "num[0] = none;", 12, notNumber, "none.x"),
		rejectAt("here as a number", "num[0] = here();", 12, notNumber, "here().x"),
		rejectAt("at as a number", "num[0] = at(1, 2);", 12, notNumber, "at(1, 2).x"),
		rejectAt("return a place", "return pin[0];", 10, notNumber, "pin[0].x"),
		rejectAt("pin assignment as condition", "if (pin[0] = here()) wait();", 7, notNumber, "own statement"),
		rejectAt("pin assignment as value", "num[1] = (pin[0] = at(1, 2));", 13, notNumber, "own statement"),
		rejectAt("pin assignment as operand", "num[1] = 1 + (pin[0] = here());", 17, notNumber, "own statement"),
		rejectAt("pin assignment as return value", "return pin[0] = here();", 10, notNumber, "own statement"),
		rejectAt("pin assignment chained", "pin[1] = pin[0] = here();", 12, "a pin holds a place", "here()"),
		rejectAt("place statement", "here();", 3, notNumber, "here().x"),
		rejectAt("place to a user function", "f(pin[0]);", 5, "places cannot be passed to functions", "pass the pin index"),
		rejectAt("place expression to a user function", "f(here());", 5, "places cannot be passed to functions", "pin[i]"),
		rejectAt("place to a number built-in", "num[0] = abs(pin[0]);", 16, notNumber, "pin[0].x"),
		rejectAt("ternary mixes place and number", "pin[0] = num[1] ? here() : 5;", 30, "a place and a number", "both"),
		rejectAt("bad place field", "num[0] = pin[0].z;", 12, "'z' is not a field of a place", ".x and .y"),
		rejectAt("place length", "num[0] = here().length;", 12, "not a field of a place", ".x and .y"),
		rejectAt("assign a place field", "pin[0].x = 1;", 3, "cannot assign to 'pin[0].x'", "pin[i] = "),
		rejectAt("pin alone", "num[0] = pin;", 12, "'pin' cannot be used on its own", "pin[i]"),
		rejectAt("assign pin alone", "pin = here();", 3, "cannot assign to 'pin'", "pin[0]..pin[7]"),
		rejectAt("assign none", "none = here();", 3, "cannot assign to 'none'", "pin[0]..pin[7]"),
		rejectAt("pin is not callable", "pin();", 3, "unknown function 'pin'", "built-ins"),
		rejectAt("here arity", "pin[0] = here(1);", 12, "here expects 0", "here()"),
		rejectAt("at arity", "pin[0] = at(1);", 12, "at expects 2", "at(x, y)"),
		rejectAt("free arity", "num[0] = free(1);", 12, "free expects 0", "free()"),
		rejectAt("count arity", "num[0] = count(1);", 12, "count expects 0", "count()"),
		rejectAt("place of a place", "num[0] = pin[0].x.y;", 12, "member access", "trail"),
		{"function named pin", "function loop() {}\nfunction pin() {}", 2, 10, "pin", "memory"},
		{"function named none", "function loop() {}\nfunction none() {}", 2, 10, "none", "place"},
		{"function named here", "function loop() {}\nfunction here() {}", 2, 10, "here", "built-in"},
		{"param named pin", "function loop() {}\nfunction f(pin) {}", 2, 12, "pin", "memory"},
		{"param named none", "function loop() {}\nfunction f(none) {}", 2, 12, "none", "place"},
	}
	runRejected(t, rows)
}

// runPins runs loop once and returns the pin cells.
func runPins(t *testing.T, body string) [lang.PinSize]lang.Place {
	t.Helper()
	return runLoop(t, "function loop() {\n  "+body+"\n}").in.Mem.Pin
}

func TestPinsStartNone(t *testing.T) {
	r := runLoop(t, inLoop("num[0] = pin[0].x; num[1] = pin[7].y; num[2] = none.x;"))
	if got := r.in.Mem.Num[:3]; got[0] != -1 || got[1] != -1 || got[2] != -1 {
		t.Fatalf("none reads %v, want -1 -1 -1", got)
	}
	if r.in.Mem.Pin != [lang.PinSize]lang.Place{} {
		t.Fatalf("pins not empty: %v", r.in.Mem.Pin)
	}
}

func TestAtStoresCoordinatesAsGiven(t *testing.T) {
	pins := runPins(t, "pin[0] = at(3, 4); pin[1] = at(-2, 400); pin[2] = at(num[5], 1 + 1);")
	want := []lang.Place{{X: 3, Y: 4, Set: true}, {X: -2, Y: 400, Set: true}, {X: 0, Y: 2, Set: true}}
	for i, w := range want {
		if pins[i] != w {
			t.Errorf("pin[%d] = %v, want %v", i, pins[i], w)
		}
	}
}

func TestPlaceFields(t *testing.T) {
	rows := []struct {
		expr string
		want int
	}{
		{"pin[0].x", 3}, {"pin[0].y", 4}, {"pin[1].x", -1}, {"pin[1].y", -1},
		{"none.x", -1}, {"none.y", -1}, {"at(5, 6).x", 5}, {"at(5, 6).y", 6},
		{"at(0, 0).x", 0}, {"pin[num[7] + 0].y", 4},
		{"pin[num[1]].x", -1}, {"pin[0 - 1].y", -1}, // computed index outside 0..7 reads none
	}
	for _, r := range rows {
		t.Run(r.expr, func(t *testing.T) {
			src := "function loop() {\n  pin[0] = at(3, 4);\n  num[1] = num[7] + 9;\n  num[0] = " + r.expr + ";\n}\n"
			if got := runLoop(t, src).in.Mem.Num[0]; got != r.want {
				t.Fatalf("%s = %d, want %d", r.expr, got, r.want)
			}
		})
	}
}

func TestPlaceEquality(t *testing.T) {
	rows := []struct {
		cond string
		want int
	}{
		{"none == none", 1}, {"none != none", 0},
		{"none == at(0, 0)", 0}, {"at(0, 0) != none", 1},
		{"at(1, 2) == at(1, 2)", 1}, {"at(1, 2) == at(2, 1)", 0}, {"at(1, 2) != at(2, 1)", 1},
		{"pin[0] == none", 1}, {"pin[1] == at(3, 4)", 1}, {"pin[1] != pin[2]", 1},
		{"pin[1] == pin[1]", 1}, {"pin[0] == pin[3]", 1},
		{"at(-5, 7) == at(-5, 7)", 1},
		{"pin[1] == (num[0] ? none : at(3, 4))", 1},
	}
	for _, r := range rows {
		t.Run(r.cond, func(t *testing.T) {
			src := "function loop() {\n  pin[1] = at(3, 4);\n  pin[2] = at(4, 3);\n  num[0] = " + r.cond + ";\n}\n"
			if got := runLoop(t, src).in.Mem.Num[0]; got != r.want {
				t.Fatalf("%s = %d, want %d", r.cond, got, r.want)
			}
		})
	}
}

func TestHereReadsMyCell(t *testing.T) {
	r := newRig(t, "function loop() {\n  pin[num[0]] = here();\n  num[0]++;\n}\n")
	r.env.read["x"], r.env.read["y"] = 2, 5
	r.in.Run(r.prog.Loop)
	r.env.read["x"], r.env.read["y"] = 3, 5
	r.in.Run(r.prog.Loop)
	r.env.read["x"], r.env.read["y"] = 2, 5
	r.in.Run(r.prog.Loop)
	want := []lang.Place{{X: 2, Y: 5, Set: true}, {X: 3, Y: 5, Set: true}, {X: 2, Y: 5, Set: true}}
	for i, w := range want {
		if r.in.Mem.Pin[i] != w {
			t.Errorf("pin[%d] = %v, want %v", i, r.in.Mem.Pin[i], w)
		}
	}
	r.in.Mem.Num[0] = 0
	r.in.Run(r.prog.Loop) // pin[0] = here() again, same cell
	if r.in.Mem.Pin[0] != r.in.Mem.Pin[2] {
		t.Errorf("here() at the same cell differs: %v vs %v", r.in.Mem.Pin[0], r.in.Mem.Pin[2])
	}
}

func TestPinAssignment(t *testing.T) {
	rows := []struct {
		body string
		want map[int]lang.Place
	}{
		{"pin[0] = at(1, 2); pin[1] = pin[0];", map[int]lang.Place{0: {X: 1, Y: 2, Set: true}, 1: {X: 1, Y: 2, Set: true}}},
		{"pin[0] = at(1, 2); pin[0] = none;", map[int]lang.Place{}},
		{"pin[0] = at(1, 2); pin[1] = none; pin[1] = pin[0]; pin[0] = at(9, 9);", map[int]lang.Place{0: {X: 9, Y: 9, Set: true}, 1: {X: 1, Y: 2, Set: true}}},
		{"num[3] = 3; pin[num[3]] = at(5, 5);", map[int]lang.Place{3: {X: 5, Y: 5, Set: true}}},
		{"pin[8 - 1] = at(0, 0);", map[int]lang.Place{7: {X: 0, Y: 0, Set: true}}},
		{"num[0] = 8; pin[num[0]] = at(1, 1);", map[int]lang.Place{}}, // out of range write is a no-op
		{"num[0] = 0 - 1; pin[num[0]] = at(1, 1);", map[int]lang.Place{}},
		{"pin[0] = num[1] ? at(1, 1) : at(2, 2);", map[int]lang.Place{0: {X: 2, Y: 2, Set: true}}},
		{"num[1] = 1; pin[0] = num[1] ? at(1, 1) : at(2, 2);", map[int]lang.Place{0: {X: 1, Y: 1, Set: true}}},
		{"num[1] = 1; pin[0] = num[1] ? none : at(2, 2); pin[1] = num[1] ? pin[0] : here();", map[int]lang.Place{}},
	}
	for _, r := range rows {
		t.Run(r.body, func(t *testing.T) {
			pins := runPins(t, r.body)
			for i, got := range pins {
				if got != r.want[i] {
					t.Errorf("pin[%d] = %v, want %v", i, got, r.want[i])
				}
			}
		})
	}
}

func TestFreeAndCount(t *testing.T) {
	rows := []struct {
		name, body string
		free, n    int
	}{
		{"empty", "", 0, 0},
		{"first set", "pin[0] = here();", 1, 1},
		{"gap", "pin[0] = here(); pin[1] = here(); pin[3] = here();", 2, 3},
		{"cleared again", "pin[0] = here(); pin[1] = here(); pin[0] = none;", 0, 1},
		{"last only", "pin[7] = here();", 0, 1},
		{"full", "for (num[1] = 0; num[1] < 8; num[1]++) { pin[num[1]] = here(); }", -1, 8},
		{"full minus one", "for (num[1] = 0; num[1] < 8; num[1]++) { pin[num[1]] = here(); } pin[5] = none;", 5, 7},
		{"at origin counts as set", "pin[0] = at(0, 0);", 1, 1},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			m := runLoop(t, "function loop() {\n  "+r.body+"\n  num[6] = free(); num[7] = count();\n}\n").in.Mem
			if m.Num[6] != r.free || m.Num[7] != r.n {
				t.Fatalf("free %d count %d, want %d %d", m.Num[6], m.Num[7], r.free, r.n)
			}
		})
	}
}

func TestPinOpsCountLikeAnyExpression(t *testing.T) {
	rows := []struct {
		body string
		ops  int
	}{
		{"pin[0] = here();", 4},   // statement, assignment, index, call
		{"pin[0] = at(1, 2);", 6}, // + two arguments
		{"pin[0] = none;", 4},
		{"num[0] = pin[0].x;", 6}, // statement, assignment, index, dot, pin index, index value
		{"num[0] = free();", 4},
	}
	for _, r := range rows {
		t.Run(r.body, func(t *testing.T) {
			if got := runLoop(t, inLoop(r.body)).in.Ops(); got != r.ops {
				t.Fatalf("ops = %d, want %d", got, r.ops)
			}
		})
	}
}

func TestPinAssignmentReadsIndexBeforeTheRightSide(t *testing.T) {
	r := runLoop(t, inLoop("pin[num[0]] = (num[0] = 3) ? here() : none;"))
	if !r.in.Mem.Pin[0].Set || r.in.Mem.Pin[3].Set {
		t.Fatalf("pins = %v", r.in.Mem.Pin)
	}
}

func (h *harness) pinIs(i int, want lang.Place) {
	h.t.Helper()
	if got := h.b.Pins()[i]; got != want {
		h.t.Fatalf("pin[%d] = %v, want %v", i, got, want)
	}
}

var startCell = lang.Place{X: 1, Y: 1, Set: true}

func TestPinsSurviveInterruptsSwapAndRespawn(t *testing.T) {
	h := newHarness(t, "function loop() { pin[3] = here(); wait(); }\nfunction onHit(d) { move(d); }")
	h.beat()
	h.pinIs(3, startCell)
	h.hitFrom(0).beat() // interrupt unwinds the loop
	h.pinIs(3, startCell)
	h.b.Swap(mustProgram(t, "function loop() { pin[4] = pin[3]; wait(); }"))
	h.calm().beat()
	h.pinIs(3, startCell)
	h.pinIs(4, startCell)
	h.respawn().beat()
	h.pinIs(3, startCell)
}

func TestPinsAccessorStartsEmpty(t *testing.T) {
	h := newHarness(t, "function loop() { wait(); }")
	if h.b.Pins() != [lang.PinSize]lang.Place{} {
		t.Fatalf("pins = %v", h.b.Pins())
	}
}

func TestProbeReportsPins(t *testing.T) {
	h := newHarness(t, "function loop() { pin[1] = here(); pin[2] = at(0 - 3, 0); wait(); }")
	h.beat()
	got := h.b.Probe()
	want := map[string]int{
		"pin_0_set": 0, "pin_0_x": -1, "pin_0_y": -1,
		"pin_1_set": 1, "pin_1_x": 1, "pin_1_y": 1,
		"pin_2_set": 1, "pin_2_x": -3, "pin_2_y": 0,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d", k, got[k], v)
		}
	}
	if got["pin_7_x"] != -1 || got["pin_7_set"] != 0 {
		t.Errorf("pin_7 = %d %d", got["pin_7_set"], got["pin_7_x"])
	}
}
