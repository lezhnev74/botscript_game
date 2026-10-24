package lang

import (
	"sort"
	"strings"
)

// MaxLines is the program size limit (LANGUAGE.md section 8).
const MaxLines = 120

// MaxCallDepth is the deepest allowed chain of user functions, counting the
// entry point (loop -> helper -> helper is 3).
const MaxCallDepth = 3

// Builtin describes a built-in function. Max is -1 for variadic ones.
type Builtin struct {
	Name     string
	Min, Max int
	Action   bool // blocks until the next beat
	Usage    string
}

var builtins = indexBuiltins([]Builtin{
	{"scan", 1, 1, true, "scan(d)"},
	{"move", 1, 1, true, "move(d)"},
	{"fire", 1, 1, true, "fire(d)"},
	{"wait", 0, 0, true, "wait()"},
	{"left", 1, 1, false, "left(d)"},
	{"right", 1, 1, false, "right(d)"},
	{"back", 1, 1, false, "back(d)"},
	{"pick", 1, -1, false, "pick(d1, d2, ...)"},
	{"chance", 1, 1, false, "chance(percent)"},
	{"rand", 2, 2, false, "rand(lo, hi)"},
	{"abs", 1, 1, false, "abs(x)"},
	{"min", 2, 2, false, "min(x, y)"},
	{"max", 2, 2, false, "max(x, y)"},
	{"dist", 4, 4, false, "dist(x1, y1, x2, y2)"},
	{"wall", 2, 2, false, "wall(x, y)"},
	{"here", 0, 0, false, "here()"},
	{"at", 2, 2, false, "at(x, y)"},
	{"free", 0, 0, false, "free()"},
	{"count", 0, 0, false, "count()"},
	{"path", 1, 1, false, "path(place)"},
	{"steps", 1, 1, false, "steps(place)"},
	{"nearest", 0, 0, false, "nearest()"},
})

func indexBuiltins(list []Builtin) map[string]Builtin {
	m := make(map[string]Builtin, len(list))
	for _, b := range list {
		m[b.Name] = b
	}
	return m
}

// LookupBuiltin returns the built-in function called name.
func LookupBuiltin(name string) (Builtin, bool) {
	b, ok := builtins[name]
	return b, ok
}

var builtinNames = func() string {
	names := make([]string, 0, len(builtins))
	for n := range builtins {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}()

// numName is the memory array: num[0]..num[7] are the only writable variables.
const numName = "num"

// NumSize is the number of memory cells in num.
const NumSize = 8

// removedSlots are the old slot names, kept only to give a helpful hint.
var removedSlots = setOf("a", "b", "c")

const removedSlotsHint = "slots a b c were removed: use num[0], num[1], num[2]"

var readings = setOf("enemy", "coin", "bullet", "last", "hp", "loaded", "bonus", "x", "y",
	"WIDTH", "HEIGHT", "N", "E", "S", "W", "trail", "SCAN", "MOVE", "FIRE", "WAIT", "IDLE")

// IsReading reports whether name is a read-only world name (readings,
// direction and action constants, trail).
func IsReading(name string) bool { return readings[name] }

const trailName = "trail"

var trailFields = setOf("x", "y", "act", "dir", "ok", "hit", "bump")

const trailHint = "only trail has members: trail.length, and trail[0].x / .y / .act / .dir / .ok / .hit / .bump (index 0 = the latest beat)"

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// handlers are the entry points the game calls itself.
type handlerSpec struct {
	maxParams int
	sig, desc string
}

var handlers = map[string]handlerSpec{
	"loop":   {0, "loop()", "no parameters"},
	"onHit":  {1, "onHit(from)", "at most one parameter"},
	"onBump": {1, "onBump(from)", "at most one parameter"},
}

// reservedKind names what a name already is, or "" when it is free to use as
// a function or parameter name.
func reservedKind(name string) string {
	switch {
	case name == numName || name == pinName:
		return "memory"
	case isPlaceName(name):
		return "place"
	case IsReading(name):
		return "reading"
	}
	if _, ok := builtins[name]; ok {
		return "built-in function"
	}
	return ""
}
