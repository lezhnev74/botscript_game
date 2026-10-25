package lang_test

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/lang"
)

// inLoop wraps one statement as line 2, column 3 of a minimal program.
func inLoop(stmt string) string { return "function loop() {\n  " + stmt + "\n}\n" }

// withHelper wraps a statement in loop and appends a helper f(p).
func withHelper(stmt string) string { return inLoop(stmt) + "function f(p) {\n  wait();\n}\n" }

func TestAccepted(t *testing.T) {
	rows := []struct{ name, src string }{
		{"empty loop", "function loop() {}"},
		{"if", inLoop("if (num[0]) wait();")},
		{"if else", inLoop("if (num[0]) wait(); else scan(N);")},
		{"else if chain", inLoop("if (num[0]) wait(); else if (num[1]) scan(N); else fire(E);")},
		{"while", inLoop("while (num[0] < 3) { num[0]++; }")},
		{"for with slot init", inLoop("for (num[0] = 0; num[0] < 3; num[0]++) { wait(); }")},
		{"for empty header", inLoop("for (;;) { break; }")},
		{"break continue", inLoop("while (num[0] < 9) { num[0]++; if (num[0] == 2) continue; break; }")},
		{"return bare", inLoop("return;")},
		{"return value", withHelper("num[0] = f(1);") + "function g() {\n  return 1;\n}\n"},
		{"ternary", inLoop("num[0] = num[1] ? 1 : 2;")},
		{"logic ops", inLoop("if (num[0] && num[1] || !num[2]) wait();")},
		{"arithmetic", inLoop("num[0] = (num[1] + 1) * 2 - num[2] / 3 % 4;")},
		{"comparisons", inLoop("if (num[0] == 1 || num[0] != 2 || num[0] < 3 || num[0] <= 4 || num[0] > 5 || num[0] >= 6) wait();")},
		{"unary minus", inLoop("num[0] = -num[1];")},
		{"assign", inLoop("num[0] = 1;")},
		{"add assign", inLoop("num[1] += 2;")},
		{"sub assign", inLoop("num[2] -= 2;")},
		{"postfix inc dec", inLoop("num[0]++; num[1]--;")},
		{"prefix inc dec", inLoop("++num[0]; --num[2];")},
		{"trail index", inLoop("num[0] = trail[0].x;")},
		{"trail index expr", inLoop("num[0] = trail[num[1] + 1].y;")},
		{"trail length", inLoop("num[0] = trail.length;")},
		{"trail fields", inLoop("num[0] = trail[0].act + trail[0].dir + trail[0].ok + trail[0].hit + trail[0].bump;")},
		{"act constants", inLoop("if (trail[0].act == SCAN || trail[0].act == MOVE || trail[0].act == FIRE || trail[0].act == WAIT || trail[0].act == IDLE) wait();")},
		{"world readings", inLoop("num[0] = enemy + coin + bullet + last + hp + loaded + bonus + x + y + WIDTH + HEIGHT;")},
		{"directions", inLoop("num[0] = N + E + S + W;")},
		{"empty statement", inLoop("if (num[0]) ;")},
		{"stray semicolon after function", "function loop() {};"},
		{"comments", "// hi\nfunction loop() { // x\n  wait(); /* y */\n}\n"},
		{"actions", inLoop("scan(N); move(E); fire(S); wait();")},
		{"free builtins", inLoop("num[0] = left(N) + right(N) + back(N) + abs(num[1]) + min(num[0], num[1]) + max(num[0], num[1]) + dist(num[0], num[1], num[2], num[0]) + rand(1, 2);")},
		{"chance", inLoop("if (chance(30)) wait();")},
		{"pick one", inLoop("num[0] = pick(N);")},
		{"wall", inLoop("num[0] = wall(1, 2);")},
		{"pick many", inLoop("num[0] = pick(N, N, S, W, E);")},
		{"helper with params", withHelper("f(1);")},
		{"helper declared before use", "function f(p) {\n  wait();\n}\nfunction loop() {\n  f(1);\n}\n"},
		{"handlers", "function loop() {}\nfunction onHit(from) { num[2] = from; }\nfunction onBump(from) { num[2] = from; }\n"},
		{"handlers without param", "function loop() {}\nfunction onHit() {}\nfunction onBump() {}\n"},
		{"depth exactly 3", "function loop() {\n  f1();\n}\nfunction f1() {\n  f2();\n}\nfunction f2() {\n  wait();\n}\n"},
		{"handler calls helper", "function loop() {}\nfunction onHit(from) {\n  f1(from);\n}\nfunction f1(d) {\n  f2(d);\n}\nfunction f2(d) {\n  move(d);\n}\n"},
		{"same helper from two roots", "function loop() {\n  f();\n}\nfunction onHit(d) {\n  f();\n}\nfunction f() {\n  wait();\n}\n"},
		{"unused helper", "function loop() {}\nfunction spare() {}\n"},
		{"nested blocks", inLoop("{ { num[0] = 1; } }")},
		{"true literal", inLoop("while (true) { num[0]++; }")},
		{"false literal", inLoop("num[0] = false;")},
		{"return true", "function loop() {}\nfunction g() {\n  return true;\n}\n"},
		{"whole float", inLoop("num[0] = 5.0 + 1e3;")},
		{"huge whole number clamps at runtime", inLoop("num[0] = 99999999999999999999;")},
		{"large number", inLoop("num[0] = 0x10 + 99999;")},
		{"memory cell 7", inLoop("num[7] = num[0];")},
		{"memory index from a parameter", "function loop() {\n  f(1);\n}\nfunction f(p) {\n  num[p] = 1;\n}\n"},
		{"memory index from memory", inLoop("num[num[1] + 1]++;")},
		{"memory index inc dec", inLoop("num[2]++; --num[3]; num[4] += 1; num[5] -= 1;")},
		{"memory as for init and update", inLoop("for (num[1] = 0; num[1] < 3; num[1]++) { wait(); }")},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			p, errs := lang.Validate(r.src)
			if len(errs) != 0 || p == nil {
				t.Fatalf("want accepted, got %v", errs)
			}
		})
	}
}

type rejectRow struct {
	name, src string
	line, col int
	msg, hint string
}

func rejectAt(name, stmt string, col int, msg, hint string) rejectRow {
	return rejectRow{name: name, src: withHelper(stmt), line: 2, col: col, msg: msg, hint: hint}
}

func TestRejectedConstructs(t *testing.T) {
	rows := []rejectRow{
		rejectAt("removed slot a", "a = 1;", 3, "cannot assign to 'a'", "slots a b c were removed: use num[0], num[1], num[2]"),
		rejectAt("removed slot b inc", "b++;", 3, "cannot assign to 'b'", "slots a b c were removed"),
		rejectAt("removed slot c read", "num[0] = c;", 12, "unknown name 'c'", "slots a b c were removed"),
		rejectAt("constant index too big", "num[8] = 1;", 3, "num[8] does not exist", "num[0]..num[7]"),
		rejectAt("constant index negative", "num[-1] = 1;", 3, "num[-1] does not exist", "num[0]..num[7]"),
		rejectAt("constant index too big on read", "num[0] = num[9];", 12, "num[9] does not exist", "num[0]..num[7]"),
		rejectAt("constant index too big on inc", "num[8]++;", 3, "num[8] does not exist", "num[0]..num[7]"),
		rejectAt("num alone is not assignable", "num = 1;", 3, "cannot assign to 'num'", "num[0]..num[7]"),
		rejectAt("num alone is not a value", "num[0] = num;", 12, "'num' cannot be used on its own", "num[i]"),
		rejectAt("num cell has no members", "num[0] = num[1].x;", 12, "member access", "trail"),
		rejectAt("num cell member assign", "num[0].x = 1;", 3, "cannot assign to 'num[0].x'", "num[0]..num[7]"),
		rejectAt("num is not callable", "num();", 3, "unknown function 'num'", "built-ins"),
		rejectAt("let", "let x = 1;", 3, "let", "num["),
		rejectAt("var", "var x = 1;", 3, "var", "num["),
		rejectAt("const", "const x = 1;", 3, "const", "num["),
		rejectAt("string double", `num[0] = "hi";`, 12, "string", "number"),
		rejectAt("string single", `num[0] = 'x';`, 12, "string", "number"),
		rejectAt("template", "num[0] = `x`;", 12, "template", "number"),
		rejectAt("array", "num[0] = [1];", 12, "array", "trail"),
		rejectAt("object", "num[0] = {};", 12, "object", "num["),
		rejectAt("function expression", "num[0] = function() {};", 12, "function", "top level"),
		rejectAt("arrow", "num[0] = () => 1;", 12, "arrow", "top level"),
		rejectAt("nested declaration", "function inner() {}", 3, "inside", "top level"),
		rejectAt("float", "num[0] = 1.5;", 12, "whole", "integer"),
		rejectAt("exponent float", "num[0] = 1e400;", 12, "whole", "integer"),
		rejectAt("method call generic", "console.log(num[0]);", 3, "method", "no objects"),
		rejectAt("method call hint only for real builtins", "Math.floor(num[0]);", 3, "method", "no objects"),
		rejectAt("null", "num[0] = null;", 12, "null", "0"),
		rejectAt("this", "num[0] = this;", 12, "this", "object"),
		rejectAt("new", "num[0] = new Foo();", 12, "new", "built-in"),
		rejectAt("regexp", "num[0] = /x/;", 12, "regular", "number"),
		rejectAt("typeof", "num[0] = typeof num[1];", 12, "typeof", "number"),
		rejectAt("delete", "delete num[0];", 3, "delete", "num["),
		rejectAt("void", "num[0] = void 0;", 12, "void", "number"),
		rejectAt("unary plus", "num[0] = +num[1];", 12, "+", "drop"),
		rejectAt("bit not", "num[0] = ~num[1];", 12, "~", "arithmetic"),
		rejectAt("comma", "num[0] = 1, num[1] = 2;", 3, "comma", "separate"),
		rejectAt("bit and", "num[0] = num[1] & 1;", 12, "&", "&&"),
		rejectAt("bit or", "num[0] = num[1] | 1;", 12, "|", "||"),
		rejectAt("bit xor", "num[0] = num[1] ^ 1;", 12, "^", "arithmetic"),
		rejectAt("shift left", "num[0] = num[1] << 1;", 12, "<<", "* 2"),
		rejectAt("shift right", "num[0] = num[1] >> 1;", 12, ">>", "/ 2"),
		rejectAt("power", "num[0] = num[1] ** 2;", 12, "**", "multiply"),
		rejectAt("strict equal", "num[0] = num[1] === 1;", 12, "===", "=="),
		rejectAt("strict not equal", "num[0] = num[1] !== 1;", 12, "!==", "!="),
		rejectAt("in", "num[0] = num[1] in num[2];", 12, "in", "arithmetic"),
		rejectAt("instanceof", "num[0] = num[1] instanceof num[2];", 12, "instanceof", "arithmetic"),
		rejectAt("coalesce", "num[0] = num[1] ?? 1;", 12, "??", "?"),
		rejectAt("mul assign", "num[0] *= 2;", 3, "*=", "num[0] = num[0] * 2"),
		rejectAt("div assign", "num[0] /= 2;", 3, "/=", "num[0] = num[0] / 2"),
		rejectAt("mod assign", "num[0] %= 2;", 3, "%=", "num[0] = num[0] % 2"),
		rejectAt("shift assign", "num[0] <<= 2;", 3, "<<=", "num[0] = num[0]"),
		rejectAt("do while", "do { num[0]++; } while (num[0] < 3);", 3, "do", "while"),
		rejectAt("switch", "switch (num[0]) { case 1: break; }", 3, "switch", "if"),
		rejectAt("for var", "for (var i = 0; i < 3; i++) {}", 8, "var", "num["),
		rejectAt("for let", "for (let i = 0; i < 3; i++) {}", 8, "let", "num["),
		rejectAt("for in", "for (k in num[0]) {}", 3, "for", "while"),
		rejectAt("try", "try { num[0] = 1; } catch (e) {}", 3, "try", "no error"),
		rejectAt("throw", "throw num[0];", 3, "throw", "no error"),
		rejectAt("label", "foo: while (num[0]) { break; }", 3, "label", "break"),
		rejectAt("break label", "while (num[0]) { break foo; }", 20, "label", "syntax"),
		rejectAt("with", "with (num[0]) { num[1] = 1; }", 3, "with", "num["),
		rejectAt("debugger", "debugger;", 3, "debugger", "remove"),
		rejectAt("class", "class Foo {}", 3, "class", "function"),
		rejectAt("spread", "num[0] = max(...b);", 19, "spread", "argument"),
		rejectAt("method call", "Math.abs(num[0]);", 3, "method", "abs("),
		rejectAt("call non-name", "num[0] = f(1)(2);", 12, "named", "function"),
		rejectAt("assign unknown", "x = 1;", 3, "x", "num["),
		rejectAt("assign readonly", "enemy = 1;", 3, "read-only", "scan"),
		rejectAt("assign direction", "N = 1;", 3, "read-only", "num[0]..num[7]"),
		rejectAt("assign trail", "trail[0].x = 1;", 3, "trail", "read-only"),
		rejectAt("assign param", "p = 1;", 3, "p", "num["),
		rejectAt("assign builtin", "scan = 1;", 3, "scan", "num["),
		rejectAt("assign member", "num[0].b = 1;", 3, "num[0].b", "num["),
		rejectAt("increment unknown", "x++;", 3, "x", "num["),
		rejectAt("increment trail", "trail[0].x++;", 3, "trail", "read-only"),
		rejectAt("prefix decrement readonly", "--hp;", 3, "hp", "read-only"),
		rejectAt("unknown ident", "num[0] = foo;", 12, "foo", "unknown"),
		rejectAt("undefined", "num[0] = undefined;", 12, "undefined", "0"),
		rejectAt("unknown function", "foo();", 3, "foo", "built-ins"),
		rejectAt("builtin as value", "num[0] = left;", 12, "left", "left("),
		rejectAt("user function as value", "num[0] = f;", 12, "f", "f("),
		rejectAt("call a reading", "hp();", 3, "not a function", "readings"),
		rejectAt("call a reading", "enemy();", 3, "not a function", "reading"),
		rejectAt("call loop", "loop();", 3, "loop", "entry"),
		rejectAt("call handler", "onHit(0);", 3, "onHit", "entry"),
		rejectAt("scan arity 0", "scan();", 3, "scan", "scan(d)"),
		rejectAt("scan arity 2", "scan(N, E);", 3, "scan", "scan(d)"),
		rejectAt("wait arity", "wait(1);", 3, "wait", "wait()"),
		rejectAt("pick arity", "num[0] = pick();", 12, "pick", "pick(d"),
		rejectAt("wall arity 1", "num[0] = wall(1);", 12, "wall", "wall(x, y)"),
		rejectAt("wall arity 3", "num[0] = wall(1, 2, 3);", 12, "wall", "wall(x, y)"),
		rejectAt("rand arity", "num[0] = rand(1);", 12, "rand", "rand(lo, hi)"),
		rejectAt("dist arity", "num[0] = dist(1, 2);", 12, "dist", "dist(x1"),
		rejectAt("chance arity", "if (chance()) wait();", 7, "chance", "chance(percent)"),
		rejectAt("user arity", "f(1, 2);", 3, "f", "1"),
		rejectAt("user arity zero", "f();", 3, "f", "1"),
		rejectAt("bare trail", "num[0] = trail;", 12, "trail", "trail[0]"),
		rejectAt("trail nested index", "num[0] = trail[0][1];", 12, "trail", "trail[0]"),
		rejectAt("trail bad property", "num[0] = trail.foo;", 12, "foo", "length"),
		rejectAt("trail bad field", "num[0] = trail[0].foo;", 12, "foo", "act"),
		rejectAt("trail field chain", "num[0] = trail[0].x.y;", 12, "trail", "act"),
		rejectAt("bare trail entry", "num[0] = trail[0];", 12, "field", "trail[0]"),
		rejectAt("for assigns non-slot", "for (i = 0; num[0] < 3; num[0]++) {}", 8, "i", "num["),
		rejectAt("index a reading", "num[0] = enemy[0];", 12, "index", "trail"),
		rejectAt("property of a reading", "num[0] = enemy.x;", 12, "enemy", "trail"),
	}
	runRejected(t, rows)
}

func TestRejectedPrograms(t *testing.T) {
	rows := []rejectRow{
		{"parse error", "function loop( {", 1, 17, "", "syntax"},
		{"missing loop", "function foo() {}", 1, 1, "loop", "function loop()"},
		{"empty source", "", 1, 1, "loop", "function loop()"},
		{"top level statement", "num[0] = 1;\nfunction loop() {}", 1, 1, "outside", "loop()"},
		{"top level var", "var x = 1;\nfunction loop() {}", 1, 1, "var", "num["},
		{"loop with params", "function loop(x) {}", 1, 15, "loop", "no parameters"},
		{"onHit two params", "function loop() {}\nfunction onHit(a2, b2) {}", 2, 20, "onHit", "one parameter"},
		{"function named like builtin", "function loop() {}\nfunction scan() {}", 2, 10, "scan", "built-in"},
		{"function named like memory", "function loop() {}\nfunction num() {}", 2, 10, "num", "memory"},
		{"function named like reading", "function loop() {}\nfunction enemy() {}", 2, 10, "enemy", "reading"},
		{"duplicate function", "function loop() {}\nfunction f() {}\nfunction f() {}", 3, 10, "f", "twice"},
		{"param named like memory", "function loop() {}\nfunction f(num) {}", 2, 12, "num", "memory"},
		{"param named like builtin", "function loop() {}\nfunction f(move) {}", 2, 12, "move", "built-in"},
		{"param named like reading", "function loop() {}\nfunction f(hp) {}", 2, 12, "hp", "reading"},
		{"param named like a user function", "function loop() {}\nfunction look(d) {}\nfunction f(look) {}", 3, 12, "look", "function"},
		{"duplicate param", "function loop() {}\nfunction f(p, p) {}", 2, 15, "p", "twice"},
		{"param default", "function loop() {}\nfunction f(p = 1) {}", 2, 12, "default", "parameter"},
		{"param rest", "function loop() {}\nfunction f(...p) {}", 2, 12, "rest", "parameter"},
		{"param destructuring", "function loop() {}\nfunction f([p]) {}", 2, 12, "parameter", "plain"},
		{"async function", "function loop() {}\nasync function f() {}", 2, 1, "async", "plain"},
		{"generator function", "function loop() {}\nfunction* f() {}", 2, 1, "generator", "plain"},
		{"direct recursion", "function loop() {\n  f(1);\n}\nfunction f(n) {\n  f(n - 1);\n}\n", 5, 3, "recursion", "itself"},
		{"mutual recursion first", "function loop() {\n  f();\n}\nfunction f() {\n  g();\n}\nfunction g() {\n  f();\n}\n", 5, 3, "recursion", "g"},
		{"mutual recursion second", "function loop() {\n  f();\n}\nfunction f() {\n  g();\n}\nfunction g() {\n  f();\n}\n", 8, 3, "recursion", "f"},
		{"loop recursion", "function loop() {\n  loop();\n}\n", 2, 3, "loop", "entry"},
		{"depth 4", "function loop() {\n  f1();\n}\nfunction f1() {\n  f2();\n}\nfunction f2() {\n  f3();\n}\nfunction f3() {\n  wait();\n}\n", 8, 3, "depth", "merge"},
		{"depth 4 from handler", "function loop() {}\nfunction onBump(d) {\n  f1();\n}\nfunction f1() {\n  f2();\n}\nfunction f2() {\n  f3();\n}\nfunction f3() {}\n", 9, 3, "depth", "merge"},
		{"depth 4 via second path", "function loop() {\n  f1();\n  f3();\n}\nfunction f1() {\n  f2();\n}\nfunction f2() {\n  f3();\n}\nfunction f3() {}\n", 9, 3, "depth", "merge"},
		{"1001 lines", loopOfLines(1001), 1001, 1, "1000", "shorter"},
	}
	runRejected(t, rows)
}

// loopOfLines builds a valid program with exactly n physical lines.
func loopOfLines(n int) string {
	return "function loop() {\n" + strings.Repeat("  wait();\n", n-2) + "}\n"
}

func (r rejectRow) matches(e lang.Error) bool {
	return e.Line == r.line && e.Col == r.col &&
		strings.Contains(strings.ToLower(e.Msg), strings.ToLower(r.msg)) &&
		strings.Contains(strings.ToLower(e.Hint), strings.ToLower(r.hint))
}

func runRejected(t *testing.T, rows []rejectRow) {
	t.Helper()
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			p, errs := lang.Validate(r.src)
			if p != nil {
				t.Fatalf("want rejected, got program")
			}
			if !slices.ContainsFunc(errs, r.matches) {
				t.Fatalf("want error %d:%d msg~%q hint~%q, got %v", r.line, r.col, r.msg, r.hint, errs)
			}
		})
	}
}

func TestAssignHintHasNoDuplicate(t *testing.T) {
	_, errs := lang.Validate(inLoop("x = 1;"))
	if len(errs) != 1 || strings.Count(errs[0].Hint, "num[0]..num[7]") != 1 {
		t.Fatalf("%v", errs)
	}
}

func TestMethodHintNamesRealBuiltin(t *testing.T) {
	_, errs := lang.Validate(inLoop("Math.abs(num[0]);"))
	if len(errs) != 1 || !strings.Contains(errs[0].Hint, "abs(") {
		t.Fatalf("%v", errs)
	}
}

func TestMethodHintGenericForUnknownMethod(t *testing.T) {
	_, errs := lang.Validate(inLoop("console.log(num[0]);"))
	if len(errs) != 1 || strings.Contains(errs[0].Hint, "log(") {
		t.Fatalf("%v", errs)
	}
}

func TestLineLimitIs1000(t *testing.T) {
	if lang.MaxLines != 1000 {
		t.Fatalf("MaxLines = %d", lang.MaxLines)
	}
}

func TestLineLimitBoundary(t *testing.T) {
	if _, errs := lang.Validate(loopOfLines(lang.MaxLines)); len(errs) != 0 {
		t.Fatalf("max lines must pass: %v", errs)
	}
	if _, errs := lang.Validate(strings.TrimSuffix(loopOfLines(lang.MaxLines), "\n")); len(errs) != 0 {
		t.Fatalf("max lines without trailing newline must pass: %v", errs)
	}
	if _, errs := lang.Validate(strings.ReplaceAll(loopOfLines(lang.MaxLines), "\n", "\r\n")); len(errs) != 0 {
		t.Fatalf("max CRLF lines must pass: %v", errs)
	}
}

func TestSingleErrorHasNoNoise(t *testing.T) {
	_, errs := lang.Validate(inLoop(`num[0] = "hi";`))
	if len(errs) != 1 {
		t.Fatalf("want exactly 1 error, got %v", errs)
	}
}

func TestAllErrorsReportedSorted(t *testing.T) {
	src := "function loop() {\n  let x = 1;\n  num[0] = \"s\";\n  y = 2;\n  foo();\n  scan();\n}\n"
	_, errs := lang.Validate(src)
	if len(errs) != 5 {
		t.Fatalf("want 5 errors, got %v", errs)
	}
	if !slices.IsSortedFunc(errs, func(a, b lang.Error) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	}) {
		t.Fatalf("not sorted: %v", errs)
	}
}

func TestErrorsAlwaysHaveMsgAndHint(t *testing.T) {
	_, errs := lang.Validate("function loop() {\n  let x = 1;\n  scan();\n}\nfunction f(num[0]) {}\nfunction")
	_, errs2 := lang.Validate("function loop() {\n  let x = 1;\n  scan();\n}\nfunction f(num[0]) {}\n")
	for _, e := range append(errs, errs2...) {
		if e.Hint == "" || e.Msg == "" || e.Error() == "" {
			t.Fatalf("empty msg/hint: %+v", e)
		}
	}
}

func TestErrorString(t *testing.T) {
	e := lang.Error{Line: 3, Col: 5, Msg: "m", Hint: "h"}
	if got := e.Error(); got != "line 3, col 5: m (h)" {
		t.Fatal(got)
	}
}

func TestValidateIsDeterministic(t *testing.T) {
	src := "function loop() {\n  f();\n  g();\n}\nfunction f() {\n  g();\n}\nfunction g() {\n  f();\n  let q = 1;\n}\n"
	_, first := lang.Validate(src)
	for i := 0; i < 20; i++ {
		if _, again := lang.Validate(src); !reflect.DeepEqual(first, again) {
			t.Fatalf("run %d differs:\n%v\n%v", i, first, again)
		}
	}
}

const metaSrc = "function loop() {\n  look(num[2]);\n}\nfunction look(d) {\n  scan(d);\n}\nfunction onHit(from) {\n  num[2] = from;\n}\n"

func metaProgram(t *testing.T) *lang.Program {
	t.Helper()
	p, errs := lang.Validate(metaSrc)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	return p
}

func TestProgramHandlers(t *testing.T) {
	p := metaProgram(t)
	if p.Loop == nil || p.Loop.Name != "loop" || p.OnHit == nil || p.OnBump != nil {
		t.Fatalf("handlers: %+v %+v %+v", p.Loop, p.OnHit, p.OnBump)
	}
	if p.Funcs["onHit"] != p.OnHit || p.Order[1] != p.Funcs["look"] {
		t.Fatal("Funcs/Order must point at the same Func values")
	}
}

func TestProgramFunction(t *testing.T) {
	p := metaProgram(t)
	look := p.Funcs["look"]
	if look == nil || !reflect.DeepEqual(look.Params, []string{"d"}) || look.Decl == nil || look.Line != 4 {
		t.Fatalf("look: %+v", look)
	}
	if line, col := p.Pos(look.Decl.Body.LeftBrace); line != 4 || col != 18 {
		t.Fatalf("pos %d:%d", line, col)
	}
}

func TestProgramTotals(t *testing.T) {
	p := metaProgram(t)
	if len(p.Funcs) != 3 || p.Lines != 9 || p.Source != metaSrc || p.AST == nil {
		t.Fatalf("program: %+v", p)
	}
}

func TestBuiltinTable(t *testing.T) {
	rows := []struct {
		name     string
		min, max int
		action   bool
	}{
		{"scan", 1, 1, true}, {"move", 1, 1, true}, {"fire", 1, 1, true}, {"wait", 0, 0, true},
		{"left", 1, 1, false}, {"right", 1, 1, false}, {"back", 1, 1, false},
		{"pick", 1, -1, false}, {"chance", 1, 1, false}, {"rand", 2, 2, false},
		{"abs", 1, 1, false}, {"min", 2, 2, false}, {"max", 2, 2, false}, {"dist", 4, 4, false},
		{"wall", 2, 2, false},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			b, ok := lang.LookupBuiltin(r.name)
			if !ok || b.Min != r.min || b.Max != r.max || b.Action != r.action || b.Usage == "" {
				t.Errorf("%+v %v", b, ok)
			}
		})
	}
}

func TestNameSets(t *testing.T) {
	_, isBuiltin := lang.LookupBuiltin("nope")
	rows := []struct {
		name string
		got  bool
		want bool
	}{
		{"nope is not a built-in", isBuiltin, false},
		{"enemy is a reading", lang.IsReading("enemy"), true},
		{"MOVE is a reading", lang.IsReading("MOVE"), true},
		{"a is not a reading", lang.IsReading("a"), false},
	}
	for _, r := range rows {
		if r.got != r.want {
			t.Errorf("%s: got %v", r.name, r.got)
		}
	}
}

func TestHouseBotsAreValid(t *testing.T) {
	files, err := filepath.Glob("bots/*.js")
	if err != nil || len(files) < 3 {
		t.Fatalf("want >= 3 house bots, got %v %v", files, err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if _, errs := lang.Validate(string(src)); len(errs) != 0 {
				t.Fatal(fmt.Sprint(errs))
			}
		})
	}
}
