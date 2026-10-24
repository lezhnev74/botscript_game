package lang

import "github.com/lezhnev/arena/engine/domain/brain"

// DefaultBudget is the op budget per beat (LANGUAGE.md section 3).
const DefaultBudget = 100000

// ActionKind is the kind of an action built-in. The numbering is also the
// value of the trail constants SCAN MOVE FIRE WAIT (IDLE is 4).
type ActionKind int

// Action kinds, in the numbering of the SCAN MOVE FIRE WAIT constants.
const (
	ActScan ActionKind = iota
	ActMove
	ActFire
	ActWait
)

// Action is one queued action. Dir is 0-3 (N E S W), always 0 for ActWait.
type Action struct {
	Kind ActionKind
	Dir  int
}

// Env is the read-only view of the world the evaluator asks on every read, so
// the host may change it while an action blocks. Values are plain ints (loaded
// is 1/0); the evaluator clamps them to +-9999.
type Env interface {
	// Reading returns enemy, coin, bullet, last, hp, loaded, bonus, x, y,
	// WIDTH or HEIGHT.
	Reading(name string) int
	// TrailLen is trail.length (0..3).
	TrailLen() int
	// TrailField returns field (x y act dir ok hit bump) of trail[i] for
	// 0 <= i < TrailLen(); ok is 1/0, hit and bump are -1 or a direction.
	TrailField(i int, field string) int
	// Open reports whether the neighbouring cell in direction d (0-3) is not
	// a wall. Bots do not count: pick only avoids walls.
	Open(d int) bool
	// Wall reports whether the cell (x, y) is a wall or outside the arena.
	// Terrain only: bots, bullets and coins are never revealed.
	Wall(x, y int) bool
	// Steps is the walking distance from my cell to the cell (x, y) over
	// terrain (bots ignored), -1 if unreachable (a wall, off the map).
	Steps(x, y int) int
	// Path is the first step (direction 0-3) of a shortest walk from my
	// cell to (x, y), ties N E S W; -1 if unreachable or my own cell.
	Path(x, y int) int
	// Heard is the place of the nearest sound last heard (the heard reading);
	// unset (none) before any sound.
	Heard() Place
}

// Actions is the hook the host (Step 12's stepper) implements. Both methods
// run on the interpreter's goroutine and may block; that is how an action
// waits for the beat and how an exhausted budget waits for the next one.
type Actions interface {
	// Do is called by scan/move/fire/wait. It returns when the beat has been
	// resolved. To abort the call stack (hit/bump/hot-swap) it panics with
	// Unwind{}; Interp.Run recovers it.
	Do(a Action)
	// OutOfBudget is called when the op count passes Budget. The evaluation
	// is suspended inside the call and continues where it stopped on return.
	OutOfBudget()
}

// Unwind is the sentinel panic value that aborts a running call stack.
type Unwind struct{}

var brainKinds = [...]brain.Kind{ActScan: brain.Scan, ActMove: brain.Move, ActFire: brain.Fire, ActWait: brain.Wait}

// BrainKind maps the action kind to the sim's brain.Kind.
func (k ActionKind) BrainKind() brain.Kind { return brainKinds[k] }

// FromBrainKind is the inverse of ActionKind.BrainKind.
func FromBrainKind(k brain.Kind) ActionKind {
	for a, b := range brainKinds {
		if b == k {
			return ActionKind(a)
		}
	}
	return ActWait
}
