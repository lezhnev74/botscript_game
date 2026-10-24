package lang_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/lang"
)

func TestExpressions(t *testing.T) {
	rows := []struct {
		expr string
		want int
	}{
		{"1 + 2", 3}, {"5 - 7", -2}, {"3 * 4", 12}, {"7 / 2", 3}, {"7 % 4", 3},
		{"-5", -5}, {"- -5", 5}, {"!0", 1}, {"!5", 0}, {"!!7", 1},
		{"2 + 3 * 4", 14}, {"(2 + 3) * 4", 20}, {"10 - 3 - 2", 5}, {"100 / 10 / 5", 2},
		{"1 + 2 == 3", 1}, {"1 < 2 == 1", 1}, {"-2 * -3", 6},
		{"true", 1}, {"false", 0}, {"true + true", 2},
		{"5.0", 5}, {"1e3", 1000},
		{"3 == 3", 1}, {"3 == 4", 0}, {"3 != 4", 1}, {"3 != 3", 0},
		{"3 < 4", 1}, {"4 < 4", 0}, {"4 <= 4", 1}, {"5 <= 4", 0},
		{"5 > 4", 1}, {"4 > 4", 0}, {"4 >= 4", 1}, {"3 >= 4", 0},
		{"2 && 3", 3}, {"2 && 0", 0}, {"0 && 3", 0}, {"0 || 0", 0}, {"0 || 2", 2}, {"5 && 0 || 6", 6}, {"0 || 7", 7}, {"4 || 0", 4},
		{"1 ? 10 : 20", 10}, {"0 ? 10 : 20", 20}, {"0 ? 1 : 0 ? 2 : 3", 3},
		{"1 || 0 && 0", 1}, {"(1 || 0) && 0", 0},
	}
	for _, r := range rows {
		t.Run(r.expr, func(t *testing.T) {
			if got := evalA(t, r.expr); got != r.want {
				t.Fatalf("%s = %d, want %d", r.expr, got, r.want)
			}
		})
	}
}

// Integer division truncates toward zero, % keeps the dividend's sign,
// and dividing by zero gives 0 (documented in interp.go).
func TestDivisionEdges(t *testing.T) {
	rows := []struct {
		expr string
		want int
	}{
		{"7 / 2", 3}, {"-7 / 2", -3}, {"7 / -2", -3}, {"-7 / -2", 3},
		{"7 % 3", 1}, {"-7 % 3", -1}, {"7 % -3", 1}, {"-7 % -3", -1},
		{"1 / 0", 0}, {"-1 / 0", 0}, {"5 % 0", 0}, {"0 / 5", 0}, {"1 / 2", 0},
	}
	for _, r := range rows {
		t.Run(r.expr, func(t *testing.T) {
			if got := evalA(t, r.expr); got != r.want {
				t.Fatalf("%s = %d, want %d", r.expr, got, r.want)
			}
		})
	}
}

func TestClamping(t *testing.T) {
	rows := []struct {
		expr string
		want int
	}{
		{"9999", 9999}, {"10000", 9999}, {"123456789", 9999}, {"-10000", -9999}, {"1e30", 9999},
		{"9999 + 1", 9999}, {"-9999 - 1", -9999}, {"9999 + -1", 9998},
		{"100 * 100", 9999}, {"-100 * 100", -9999}, {"99 * 100", 9900},
		{"9999 * 9999", 9999}, {"-9999 * 9999", -9999},
		{"-(0 - 9999)", 9999},
		{"9999 + 9999 - 9999", 0}, // each operation clamps: 19998 -> 9999, then - 9999
		{"10000 - 10000", 0},      // literals clamp first: 9999 - 9999
		{"5000 + 5000", 9999},
	}
	for _, r := range rows {
		t.Run(r.expr, func(t *testing.T) {
			if got := evalA(t, r.expr); got != r.want {
				t.Fatalf("%s = %d, want %d", r.expr, got, r.want)
			}
		})
	}
}

func TestShortCircuit(t *testing.T) {
	src := `function loop() {
  num[0] = 0 && f();
  num[1] = 1 || f();
  num[2] = 1 ? 0 : f();
}
function f() { wait(); return 1; }`
	r := runLoop(t, src)
	if len(r.acts.calls) != 0 {
		t.Fatalf("right side evaluated: %v", r.acts.calls)
	}
	r2 := runLoop(t, "function loop() { num[0] = 1 && f(); num[1] = 0 || f(); num[2] = 0 ? 0 : f(); }\nfunction f() { num[1]++; return 1; }")
	if [3]int(r2.in.Mem.Num[:3]) != [3]int{1, 2, 1} {
		t.Fatalf("slots = %v", r2.in.Mem.Num[:3])
	}
}

func TestSlotsAssignment(t *testing.T) {
	rows := []struct {
		body string
		want [3]int
	}{
		{"num[0] = 5; num[1] = num[0]; num[2] = num[1] + 1;", [3]int{5, 5, 6}},
		{"num[0] = 5; num[0] += 3;", [3]int{8, 0, 0}},
		{"num[0] = 5; num[0] -= 7;", [3]int{-2, 0, 0}},
		{"num[0] += 20000;", [3]int{9999, 0, 0}},
		{"num[0] -= 20000;", [3]int{-9999, 0, 0}},
		{"num[0]++; num[0]++; num[1]--;", [3]int{2, -1, 0}},
		{"++num[0]; --num[1];", [3]int{1, -1, 0}},
		{"num[0] = 9999; num[0]++;", [3]int{9999, 0, 0}},
		{"num[0] = -9999; --num[0];", [3]int{-9999, 0, 0}},
		{"num[1] = num[0]++;", [3]int{1, 0, 0}},
		{"num[1] = ++num[0];", [3]int{1, 1, 0}},
		{"num[1] = num[0]--;", [3]int{-1, 0, 0}},
		{"num[1] = --num[0];", [3]int{-1, -1, 0}},
		{"num[0] = num[1] = 4;", [3]int{4, 4, 0}},
		{"num[0] = 3; num[1] = (num[0] += 2) * 2;", [3]int{5, 10, 0}},
		{"num[2] = 1; num[2] += num[2]; num[2] += num[2];", [3]int{0, 0, 4}},
	}
	for _, r := range rows {
		t.Run(r.body, func(t *testing.T) {
			got := [3]int(runLoop(t, "function loop() {\n  "+r.body+"\n}").in.Mem.Num[:3])
			if got != r.want {
				t.Fatalf("slots = %v, want %v", got, r.want)
			}
		})
	}
}

// The old value of the slot is read before the right side runs (as in JS).
func TestCompoundReadsLeftFirst(t *testing.T) {
	r := runLoop(t, "function loop() { num[0] = 10; num[0] += f(); }\nfunction f() { num[0] = 1; return 5; }")
	if r.in.Mem.Num[0] != 15 {
		t.Fatalf("a = %d, want 15", r.in.Mem.Num[0])
	}
}

func TestStatements(t *testing.T) {
	rows := []struct {
		name, body string
		want       [3]int
	}{
		{"if true", "if (1) num[0] = 1;", [3]int{1, 0, 0}},
		{"if false", "if (0) num[0] = 1;", [3]int{}},
		{"else", "if (0) num[0] = 1; else num[0] = 2;", [3]int{2, 0, 0}},
		{"else if", "num[1] = 2; if (num[1] == 1) num[0] = 1; else if (num[1] == 2) num[0] = 2; else num[0] = 3;", [3]int{2, 2, 0}},
		{"block", "if (1) { num[0] = 1; num[1] = 2; }", [3]int{1, 2, 0}},
		{"while", "while (num[0] < 5) num[0]++;", [3]int{5, 0, 0}},
		{"while never", "while (0) num[0] = 1;", [3]int{}},
		{"for", "for (num[0] = 0; num[0] < 4; num[0]++) num[1] += num[0];", [3]int{4, 6, 0}},
		{"for empty parts", "for (;;) { num[0]++; if (num[0] == 3) break; }", [3]int{3, 0, 0}},
		{"for no init", "for (; num[0] < 2; ) num[0]++;", [3]int{2, 0, 0}},
		{"break", "while (1) { num[0]++; if (num[0] > 2) break; }", [3]int{3, 0, 0}},
		{"continue", "for (num[0] = 0; num[0] < 5; num[0]++) { if (num[0] == 2) continue; num[1]++; }", [3]int{5, 4, 0}},
		{"nested break only inner", "for (num[0] = 0; num[0] < 3; num[0]++) { while (1) { num[1]++; break; } }", [3]int{3, 3, 0}},
		{"return stops", "num[0] = 1; return; num[0] = 2;", [3]int{1, 0, 0}},
		{"return in loop", "while (1) { num[0]++; if (num[0] == 4) return; }", [3]int{4, 0, 0}},
		{"empty statement", ";; num[0] = 1;;", [3]int{1, 0, 0}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got := [3]int(runLoop(t, "function loop() {\n  "+r.body+"\n}").in.Mem.Num[:3])
			if got != r.want {
				t.Fatalf("slots = %v, want %v", got, r.want)
			}
		})
	}
}

func TestFunctions(t *testing.T) {
	rows := []struct {
		name, src string
		want      int
	}{
		{"params", "function loop() { num[0] = f(3, 4); }\nfunction f(p, q) { return p * 10 + q; }", 34},
		{"no return is 0", "function loop() { num[0] = 7; num[0] = f(); }\nfunction f() { num[1] = 1; }", 0},
		{"bare return is 0", "function loop() { num[0] = 7; num[0] = f(); }\nfunction f() { return; }", 0},
		{"params are per call", "function loop() { num[0] = f(1) + f(2); }\nfunction f(p) { return p; }", 3},
		{"depth 3", "function loop() { num[0] = f(1); }\nfunction f(p) { return g(p) + 1; }\nfunction g(q) { return q * 2; }", 3},
		{"arg order", "function loop() { num[0] = f(1, 2); }\nfunction f(p, q) { return p - q; }", -1},
		{"args evaluated in caller scope", "function loop() { num[0] = f(2); }\nfunction f(p) { return g(p + 1) + p; }\nfunction g(p) { return p * 10; }", 32},
		{"return inside loop", "function loop() { num[0] = f(); }\nfunction f() { while (1) { return 9; } }", 9},
		{"return clamps", "function loop() { num[0] = f(); }\nfunction f() { return 99999; }", 9999},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if got := runLoop(t, r.src).in.Mem.Num[0]; got != r.want {
				t.Fatalf("a = %d, want %d", got, r.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	rows := []struct {
		name string
		want int
	}{
		{"N", 0}, {"E", 1}, {"S", 2}, {"W", 3},
		{"SCAN", 0}, {"MOVE", 1}, {"FIRE", 2}, {"WAIT", 3}, {"IDLE", 4},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if got := evalA(t, r.name); got != r.want {
				t.Fatalf("%s = %d, want %d", r.name, got, r.want)
			}
		})
	}
}

func TestReadings(t *testing.T) {
	for _, name := range []string{"enemy", "coin", "bullet", "last", "hp", "loaded", "bonus", "x", "y", "WIDTH", "HEIGHT"} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, "function loop() {\n  num[0] = "+name+";\n}")
			r.env.read[name] = 7
			r.in.Run(r.prog.Loop)
			if r.in.Mem.Num[0] != 7 {
				t.Fatalf("%s = %d, want 7", name, r.in.Mem.Num[0])
			}
		})
	}
}

func TestReadingsAreClamped(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = x; }")
	r.env.read["x"] = 50000
	r.in.Run(r.prog.Loop)
	if r.in.Mem.Num[0] != 9999 {
		t.Fatalf("a = %d", r.in.Mem.Num[0])
	}
}

func TestReadingsAreLive(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = enemy; wait(); num[1] = enemy; }")
	r.acts.onDo = func(int) { r.env.read["enemy"] = 3 }
	r.in.Run(r.prog.Loop)
	if r.in.Mem.Num[0] != 0 || r.in.Mem.Num[1] != 3 {
		t.Fatalf("slots = %v", r.in.Mem.Num[:3])
	}
}

func TestParamShadowsNothingButReads(t *testing.T) {
	r := runLoop(t, "function loop() { num[0] = f(N, 5); }\nfunction f(p, q) { return p + q + hp; }")
	if r.in.Mem.Num[0] != 5 {
		t.Fatalf("a = %d", r.in.Mem.Num[0])
	}
}

func TestTrail(t *testing.T) {
	r := newRig(t, `function loop() {
  num[0] = trail.length;
  num[1] = trail[0].x * 100 + trail[1].y;
  num[2] = trail[trail.length - 1].bump;
}`)
	r.env.trail = []map[string]int{{"x": 4, "y": 1, "bump": 2}, {"x": 3, "y": 8, "bump": -1}}
	r.in.Run(r.prog.Loop)
	if [3]int(r.in.Mem.Num[:3]) != [3]int{2, 408, -1} {
		t.Fatalf("slots = %v", r.in.Mem.Num[:3])
	}
}

func TestTrailFields(t *testing.T) {
	for _, f := range []string{"x", "y", "act", "dir", "ok", "hit", "bump"} {
		t.Run(f, func(t *testing.T) {
			r := newRig(t, "function loop() { num[0] = trail[0]."+f+"; }")
			r.env.trail = []map[string]int{{f: 6}}
			r.in.Run(r.prog.Loop)
			if r.in.Mem.Num[0] != 6 {
				t.Fatalf("a = %d", r.in.Mem.Num[0])
			}
		})
	}
}

// Reading outside trail.length gives -1 ("nothing happened there").
func TestTrailOutOfRange(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = trail[0].x; num[1] = trail[3].act; num[2] = trail[0 - 1].hit; }")
	r.in.Run(r.prog.Loop)
	if [3]int(r.in.Mem.Num[:3]) != [3]int{-1, -1, -1} {
		t.Fatalf("slots = %v", r.in.Mem.Num[:3])
	}
}

func TestTurning(t *testing.T) {
	rows := []struct {
		expr string
		want int
	}{
		{"left(N)", 3}, {"left(E)", 0}, {"left(S)", 1}, {"left(W)", 2},
		{"right(N)", 1}, {"right(E)", 2}, {"right(S)", 3}, {"right(W)", 0},
		{"back(N)", 2}, {"back(E)", 3}, {"back(S)", 0}, {"back(W)", 1},
		{"right(left(N))", 0}, {"left(left(N))", 2},
		{"right(7)", 0}, {"left(0 - 1)", 2}, {"back(0 - 5)", 1}, {"left(4)", 3},
	}
	for _, r := range rows {
		t.Run(r.expr, func(t *testing.T) {
			if got := evalA(t, r.expr); got != r.want {
				t.Fatalf("%s = %d, want %d", r.expr, got, r.want)
			}
		})
	}
}

func TestMath(t *testing.T) {
	rows := []struct {
		expr string
		want int
	}{
		{"abs(5)", 5}, {"abs(-5)", 5}, {"abs(0)", 0}, {"abs(-9999)", 9999},
		{"min(3, 4)", 3}, {"min(4, 3)", 3}, {"min(-1, 1)", -1}, {"min(2, 2)", 2},
		{"max(3, 4)", 4}, {"max(4, 3)", 4}, {"max(-1, 1)", 1}, {"max(2, 2)", 2},
		{"dist(0, 0, 3, 4)", 7}, {"dist(3, 4, 0, 0)", 7}, {"dist(5, 5, 5, 5)", 0},
		{"dist(-9999, 0, 9999, 0)", 9999}, {"dist(-1, -1, 1, 1)", 4},
		{"abs(min(2, 0 - 7))", 7},
	}
	for _, r := range rows {
		t.Run(r.expr, func(t *testing.T) {
			if got := evalA(t, r.expr); got != r.want {
				t.Fatalf("%s = %d, want %d", r.expr, got, r.want)
			}
		})
	}
}

func TestActions(t *testing.T) {
	rows := []struct {
		call string
		want lang.Action
	}{
		{"scan(E)", lang.Action{Kind: lang.ActScan, Dir: 1}},
		{"move(S)", lang.Action{Kind: lang.ActMove, Dir: 2}},
		{"fire(W)", lang.Action{Kind: lang.ActFire, Dir: 3}},
		{"wait()", lang.Action{Kind: lang.ActWait}},
		{"move(5)", lang.Action{Kind: lang.ActMove, Dir: 1}}, // directions wrap mod 4
		{"fire(0 - 1)", lang.Action{Kind: lang.ActWait}},     // a negative direction is a wait
		{"move(-1)", lang.Action{Kind: lang.ActWait}},
		{"scan(-9999)", lang.Action{Kind: lang.ActWait}},
		{"fire(-4)", lang.Action{Kind: lang.ActWait}},
		{"move(0)", lang.Action{Kind: lang.ActMove, Dir: 0}},
		{"scan(4)", lang.Action{Kind: lang.ActScan, Dir: 0}},
		{"fire(7)", lang.Action{Kind: lang.ActFire, Dir: 3}},
		{"move(left(N))", lang.Action{Kind: lang.ActMove, Dir: 3}}, // turning built-ins still wrap
		{"scan(right(W))", lang.Action{Kind: lang.ActScan, Dir: 0}},
	}
	for _, r := range rows {
		t.Run(r.call, func(t *testing.T) {
			got := runLoop(t, "function loop() { "+r.call+"; }").acts.calls
			if len(got) != 1 || got[0] != r.want {
				t.Fatalf("calls = %v, want [%v]", got, r.want)
			}
		})
	}
}

func TestActionsRunInOrderAndReturnZero(t *testing.T) {
	r := runLoop(t, "function loop() { num[0] = 5; num[0] = wait(); move(N); scan(E); }")
	if r.in.Mem.Num[0] != 0 || len(r.acts.calls) != 3 {
		t.Fatalf("a=%d calls=%v", r.in.Mem.Num[0], r.acts.calls)
	}
	if r.acts.calls[1].Kind != lang.ActMove || r.acts.calls[2].Kind != lang.ActScan {
		t.Fatalf("order: %v", r.acts.calls)
	}
}

func TestSlotsPersistAcrossRuns(t *testing.T) {
	r := newRig(t, "function loop() { num[0]++; num[1] += num[0]; }")
	for range 4 {
		r.in.Run(r.prog.Loop)
	}
	if [3]int(r.in.Mem.Num[:3]) != [3]int{4, 10, 0} {
		t.Fatalf("slots = %v", r.in.Mem.Num[:3])
	}
}

func TestRunWithArguments(t *testing.T) {
	r := newRig(t, "function loop() { wait(); }\nfunction onHit(from) { num[0] = from + 1; }")
	r.in.Run(r.prog.OnHit, 2)
	if r.in.Mem.Num[0] != 3 {
		t.Fatalf("a = %d", r.in.Mem.Num[0])
	}
}

type interrupt struct{}

// A Do that panics lang.Unwind aborts the whole call stack; Run reports it.
func TestUnwind(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = 1; f(); num[0] = 2; }\nfunction f() { move(N); num[1] = 1; }")
	r.acts.onDo = func(int) { panic(lang.Unwind{}) }
	if !r.in.Run(r.prog.Loop) {
		t.Fatal("Run did not report the unwind")
	}
	if [3]int(r.in.Mem.Num[:3]) != [3]int{1, 0, 0} {
		t.Fatalf("slots = %v", r.in.Mem.Num[:3])
	}
	r.acts.onDo = nil
	if r.in.Run(r.prog.Loop) {
		t.Fatal("clean run reported an unwind")
	}
	if r.in.Mem.Num[0] != 2 || r.in.Mem.Num[1] != 1 {
		t.Fatalf("after clean run: %v", r.in.Mem.Num[:3])
	}
}

func TestForeignPanicPropagates(t *testing.T) {
	r := newRig(t, "function loop() { wait(); }")
	r.acts.onDo = func(int) { panic(interrupt{}) }
	defer func() {
		if _, ok := recover().(interrupt); !ok {
			t.Fatal("foreign panic was swallowed")
		}
	}()
	r.in.Run(r.prog.Loop)
}

// --- op counting -------------------------------------------------------------

func opsOf(t *testing.T, body string) int {
	t.Helper()
	r := runLoop(t, "function loop() {\n  "+body+"\n}")
	return r.in.Ops()
}

func TestOpCounts(t *testing.T) {
	// Every executed statement and every evaluated expression node is 1 op.
	// Assignment targets and ++/-- operands are not evaluated, but the index of
	// a num[i] target or operand is, so num[0] = 1 costs stmt, assign, index, value.
	rows := []struct {
		name, body string
		want       int
	}{
		{"empty", "", 0},
		{"literal assign", "num[0] = 1;", 4},
		{"binary", "num[0] = 1 + 2;", 6},
		{"memory read", "num[0] = num[1];", 5},
		{"increment", "num[0]++;", 3},
		{"unary", "num[0] = -1;", 5},
		{"builtin call", "num[0] = abs(1);", 5},
		{"if false", "if (0) num[0] = 1;", 2},
		{"if true", "if (1) num[0] = 1;", 6},
		{"and short-circuit", "num[0] = 0 && 1;", 5},
		{"and full", "num[0] = 1 && 1;", 6},
		{"ternary", "num[0] = 1 ? 2 : 3;", 6},
		{"while 2 iterations", "while (num[0] < 2) num[0]++;", 1 + 3*4 + 2*3},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if got := opsOf(t, r.body); got != r.want {
				t.Fatalf("ops = %d, want %d", got, r.want)
			}
		})
	}
}

func TestOpCountUserCall(t *testing.T) {
	// stmt, assign, index, call, arg literal, then callee: return stmt, literal.
	r := runLoop(t, "function loop() { num[0] = f(1); }\nfunction f(p) { return 2; }")
	if got := r.in.Ops(); got != 7 {
		t.Fatalf("ops = %d, want 7", got)
	}
}

func TestOpsResetAfterAction(t *testing.T) {
	r := runLoop(t, "function loop() { num[0] = 1; wait(); num[0] = 2; }")
	// the counter is already reset when Do runs (so an Unwind from Do leaves a fresh beat)
	if r.acts.opsAtCall[0] != 0 {
		t.Fatalf("ops at call = %d, want 0", r.acts.opsAtCall[0])
	}
	if r.in.Ops() != 4 {
		t.Fatalf("ops after action = %d, want 4", r.in.Ops())
	}
}

// The budget is per beat: re-running loop() in the same beat keeps counting.
func TestOpsCarryOverAcrossRuns(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = 1; }")
	r.in.Run(r.prog.Loop)
	r.in.Run(r.prog.Loop)
	if r.in.Ops() != 8 {
		t.Fatalf("ops = %d, want 8", r.in.Ops())
	}
}

func TestBudgetStallsAndResumes(t *testing.T) {
	r := newRig(t, "function loop() { while (num[0] < 10) num[0]++; }")
	r.in.Budget = 20
	r.in.Run(r.prog.Loop)
	if r.in.Mem.Num[0] != 10 {
		t.Fatalf("a = %d: the program must resume where it stopped", r.in.Mem.Num[0])
	}
	if r.acts.stalls != 3 { // 75 ops in total, a stall at every 21st
		t.Fatalf("stalls = %d, want 3", r.acts.stalls)
	}
	for _, o := range r.acts.opsAtStal {
		if o != 1 {
			t.Fatalf("ops during stall = %d, want 1 (the waiting op starts the new beat)", o)
		}
	}
}

func TestDefaultBudget(t *testing.T) {
	r := newRig(t, "function loop() { while (1) { } }")
	if r.in.Budget != lang.DefaultBudget || lang.DefaultBudget != 100000 {
		t.Fatalf("budget = %d", r.in.Budget)
	}
	// An endless loop stalls every 100001st op; unwind on the 3rd stall.
	stop := &stopper{max: 3}
	in := lang.NewInterp(r.prog, r.env, stop, 1)
	if !in.Run(r.prog.Loop) || stop.n != 3 {
		t.Fatalf("stalls = %d", stop.n)
	}
}

type stopper struct{ n, max int }

func (s *stopper) Do(lang.Action) {}
func (s *stopper) OutOfBudget() {
	s.n++
	if s.n == s.max {
		panic(lang.Unwind{})
	}
}

func TestZeroBudgetMeansUnlimited(t *testing.T) {
	r := newRig(t, "function loop() { while (num[0] < 300) num[0]++; }")
	r.in.Budget = 0
	r.in.Run(r.prog.Loop)
	if r.acts.stalls != 0 || r.in.Mem.Num[0] != 300 {
		t.Fatalf("stalls=%d a=%d", r.acts.stalls, r.in.Mem.Num[0])
	}
}

// --- built-ins with dice -----------------------------------------------------

func TestPickSkipsWalls(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = pick(N, E, S, W); }")
	r.env.walls = map[int]bool{0: true, 1: true, 3: true}
	for range 20 {
		r.in.Run(r.prog.Loop)
		if r.in.Mem.Num[0] != 2 {
			t.Fatalf("picked %d, only S is open", r.in.Mem.Num[0])
		}
	}
}

func TestPickAllWallsReturnsFirst(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = pick(W, N, E); }")
	r.env.walls = map[int]bool{0: true, 1: true, 2: true, 3: true}
	r.in.Run(r.prog.Loop)
	if r.in.Mem.Num[0] != 3 {
		t.Fatalf("picked %d, want first (W=3)", r.in.Mem.Num[0])
	}
}

func TestPickWrapsDirections(t *testing.T) {
	for expr, want := range map[string]int{"pick(5)": 1, "pick(0 - 1)": 3} {
		if got := evalA(t, expr); got != want {
			t.Fatalf("%s = %d, want %d", expr, got, want)
		}
	}
	r := newRig(t, "function loop() { num[0] = pick(5); num[1] = pick(0 - 1); }")
	r.env.walls = map[int]bool{0: true, 1: true, 2: true, 3: true}
	r.in.Run(r.prog.Loop)
	if r.in.Mem.Num[0] != 1 || r.in.Mem.Num[1] != 3 {
		t.Fatalf("all-walls fallback = %v, want wrapped first args 1 and 3", r.in.Mem.Num[:3])
	}
}

func TestPickSingle(t *testing.T) {
	if got := evalA(t, "pick(E)"); got != 1 {
		t.Fatalf("got %d", got)
	}
}

func TestPickIsWeighted(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = pick(N, N, N, S); num[1] += num[0] == 0; num[2] += num[0] == 2; }")
	for range 2000 {
		r.in.Run(r.prog.Loop)
	}
	n, s := r.in.Mem.Num[1], r.in.Mem.Num[2]
	if n+s != 2000 || n < 1350 || n > 1650 {
		t.Fatalf("N=%d S=%d, want about 3:1", n, s)
	}
}

func TestChance(t *testing.T) {
	rows := []struct{ p, lo, hi int }{{0, 0, 0}, {-5, 0, 0}, {100, 2000, 2000}, {150, 2000, 2000}, {30, 480, 720}, {50, 880, 1120}}
	for _, r := range rows {
		t.Run(fmt.Sprint(r.p), func(t *testing.T) {
			rg := newRig(t, fmt.Sprintf("function loop() { num[0] += chance(%d); }", r.p))
			for range 2000 {
				rg.in.Run(rg.prog.Loop)
			}
			if got := rg.in.Mem.Num[0]; got < r.lo || got > r.hi {
				t.Fatalf("chance(%d) hit %d/2000, want %d..%d", r.p, got, r.lo, r.hi)
			}
		})
	}
}

func TestRandRange(t *testing.T) {
	rows := []struct{ lo, hi int }{{1, 6}, {-3, 3}, {5, 5}, {9, 2}}
	for _, r := range rows {
		t.Run(fmt.Sprintf("%d..%d", r.lo, r.hi), func(t *testing.T) {
			rg := newRig(t, fmt.Sprintf("function loop() { num[0] = rand(%d, %d); }", r.lo, r.hi))
			checkSpan(t, drawSlotA(rg, 500), min(r.lo, r.hi), max(r.lo, r.hi))
		})
	}
}

// checkSpan fails unless seen is exactly every value in lo..hi.
func checkSpan(t *testing.T, seen map[int]bool, lo, hi int) {
	t.Helper()
	for v := range seen {
		if v < lo || v > hi {
			t.Fatalf("rand = %d outside %d..%d", v, lo, hi)
		}
	}
	if len(seen) != hi-lo+1 {
		t.Fatalf("saw %d distinct values, want %d (both ends inclusive)", len(seen), hi-lo+1)
	}
}

// drawSlotA runs loop n times and returns the set of values a took.
func drawSlotA(r *rig, n int) map[int]bool {
	seen := map[int]bool{}
	for range n {
		r.in.Run(r.prog.Loop)
		seen[r.in.Mem.Num[0]] = true
	}
	return seen
}

// diceTrace records 40 draws of every dice built-in for one bot.
func diceTrace(t *testing.T, seed uint64, bot string) []int {
	t.Helper()
	p, errs := lang.Validate("function loop() { num[0] = rand(0, 1000); num[1] = pick(N, E, S, W); num[2] = chance(50); }")
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	in := lang.NewInterp(p, &fakeEnv{}, &fakeActs{}, lang.SeedFor(seed, bot))
	var out []int
	for range 40 {
		in.Run(p.Loop)
		out = append(out, in.Mem.Num[:]...)
	}
	return out
}

func TestDiceAreDeterministicPerSeedAndBot(t *testing.T) {
	a, b := diceTrace(t, 7, "A"), diceTrace(t, 7, "A")
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Fatal("same seed and bot gave different dice")
	}
	if fmt.Sprint(a) == fmt.Sprint(diceTrace(t, 8, "A")) {
		t.Fatal("different match seeds gave the same dice")
	}
	if fmt.Sprint(a) == fmt.Sprint(diceTrace(t, 7, "B")) {
		t.Fatal("different bots gave the same dice")
	}
}

func TestSeedForIsStable(t *testing.T) {
	// Pinned: changing the derivation changes every recorded replay.
	if got := lang.SeedFor(1, "A"); got != lang.SeedFor(1, "A") || got == lang.SeedFor(1, "B") || got == lang.SeedFor(2, "A") {
		t.Fatal("SeedFor is not a function of both inputs")
	}
	if got := lang.SeedFor(1, "A"); got != 15729488487897787696 {
		t.Fatalf("SeedFor(1, A) = %d, derivation changed", got)
	}
	if lang.SeedFor(0, "") == 0 {
		t.Fatal("zero seed for the zero inputs")
	}
}

// --- the house bots run ------------------------------------------------------

func TestHouseBotsRunOneLoop(t *testing.T) {
	for _, name := range []string{"rookie", "lighthouse", "tracker", "forager", "hound"} {
		t.Run(name, func(t *testing.T) {
			src := readBot(t, name)
			p, errs := lang.Validate(src)
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			env := &fakeEnv{read: map[string]int{"enemy": 3}}
			acts := &fakeActs{}
			in := lang.NewInterp(p, env, acts, lang.SeedFor(1, name))
			acts.in = in
			in.Run(p.Loop)
			if len(acts.calls) == 0 {
				t.Fatal("no action issued")
			}
		})
	}
}

func readBot(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("bots/" + name + ".js")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestActionKindBrainMapping(t *testing.T) {
	rows := []struct {
		a lang.ActionKind
		b brain.Kind
	}{
		{lang.ActScan, brain.Scan}, {lang.ActMove, brain.Move}, {lang.ActFire, brain.Fire}, {lang.ActWait, brain.Wait},
	}
	for _, r := range rows {
		if got := r.a.BrainKind(); got != r.b {
			t.Errorf("%v.BrainKind() = %v, want %v", r.a, got, r.b)
		}
		if got := lang.FromBrainKind(r.b); got != r.a {
			t.Errorf("FromBrainKind(%v) = %v, want %v", r.b, got, r.a)
		}
	}
}

// --- wall(x, y) ---------------------------------------------------------------

func TestWallReadsTheTerrain(t *testing.T) {
	r := newRig(t, "function loop() { num[0] = wall(2, 3); num[1] = wall(0, 0); num[2] = wall(0 - 1, 5); }")
	r.env.cells = map[[2]int]bool{{2, 3}: true, {-1, 5}: true}
	r.in.Run(r.prog.Loop)
	if got := r.in.Mem.Num[:3]; got[0] != 1 || got[1] != 0 || got[2] != 1 {
		t.Fatalf("slots %v, want 1 0 1", got)
	}
}

func TestWallCostsOpsLikeOtherBuiltins(t *testing.T) {
	cost := func(expr string) int {
		r := runLoop(t, "function loop() { num[0] = "+expr+"; wait(); }")
		return r.acts.opsAtCall[0]
	}
	if w, m := cost("wall(1, 2)"), cost("min(1, 2)"); w != m {
		t.Fatalf("wall costs %d ops, min %d", w, m)
	}
}

func TestWallIsFreeNoBeat(t *testing.T) {
	r := runLoop(t, "function loop() { num[0] = wall(1, 1); }")
	if len(r.acts.calls) != 0 {
		t.Fatalf("wall queued actions: %v", r.acts.calls)
	}
}

// --- num[i] memory --------------------------------------------------------------

func TestMemoryComputedIndex(t *testing.T) {
	r := runLoop(t, "function loop() { num[7] = 4; num[1] = 6; num[num[1] + 1] = num[7] + 1; num[num[7] - 4]++; }")
	if got := r.in.Mem.Num; got != [8]int{0, 7, 0, 0, 0, 0, 0, 5} {
		t.Fatalf("num = %v", got)
	}
}

func TestMemoryOutOfRangeReadsZeroAndWritesNothing(t *testing.T) {
	rows := []struct {
		name, body string
		want       [8]int
	}{
		{"write high", "num[0 + 8] = 5;", [8]int{}},
		{"write negative", "num[0 - 1] = 5;", [8]int{}},
		{"add high", "num[3 * 3] += 5;", [8]int{}},
		{"increment high", "num[2 + 7]++;", [8]int{}},
		{"decrement negative", "--num[0 - 3];", [8]int{}},
		{"nested write", "num[num[0 - 9]] = 5; num[0 - 2] = num[1] = 0;", [8]int{5}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if got := runLoop(t, "function loop() {\n  "+r.body+"\n}").in.Mem.Num; got != r.want {
				t.Fatalf("num = %v, want %v", got, r.want)
			}
		})
	}
}

func TestMemoryOutOfRangeReadsZero(t *testing.T) {
	if got := evalA(t, "num[0 + 9] + num[0 - 1] + 7"); got != 7 {
		t.Fatalf("out-of-range reads: %d, want 7", got)
	}
}

func TestMemoryOutOfRangeWriteStillRunsTheRightSide(t *testing.T) {
	r := runLoop(t, "function loop() { num[0 + 8] = f(); }\nfunction f() { num[1] = 3; return 1; }")
	if got := r.in.Mem.Num; got != [8]int{0, 3} {
		t.Fatalf("num = %v", got)
	}
}

func TestMemoryClampsEveryCell(t *testing.T) {
	r := runLoop(t, "function loop() { num[7] = 9999; num[7]++; num[6] = 0 - 9999; num[6] -= 5; num[5] += 99999; }")
	if got := r.in.Mem.Num; got[7] != 9999 || got[6] != -9999 || got[5] != 9999 {
		t.Fatalf("num = %v", got)
	}
}

func TestMemoryCellsAreIndependentAndPersistAcrossRuns(t *testing.T) {
	r := newRig(t, "function loop() { num[0]++; num[7] += num[0]; }")
	r.in.Run(r.prog.Loop)
	r.in.Run(r.prog.Loop)
	r.in.Run(r.prog.Loop)
	if got := r.in.Mem.Num; got != [8]int{3, 0, 0, 0, 0, 0, 0, 6} {
		t.Fatalf("num = %v", got)
	}
}
