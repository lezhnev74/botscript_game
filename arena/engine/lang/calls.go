package lang

import "github.com/dop251/goja/ast"

var actionKinds = map[string]ActionKind{"scan": ActScan, "move": ActMove, "fire": ActFire, "wait": ActWait}

// call evaluates the arguments left to right, then runs a user function, an
// action or a free built-in. The validator guarantees the callee exists.
func (in *Interp) call(n *ast.CallExpression) int {
	name := n.Callee.(*ast.Identifier).Name.String()
	if f, ok := placeTakers[name]; ok {
		return f(in, in.place(n.ArgumentList[0]))
	}
	args := in.evalArgs(n)
	if f, ok := in.Prog.Funcs[name]; ok {
		return in.invoke(f, args)
	}
	if k, ok := actionKinds[name]; ok {
		in.line, _ = in.Prog.Pos(n.Idx0())
		return in.action(k, args)
	}
	return freeBuiltins[name](in, args)
}

func (in *Interp) evalArgs(n *ast.CallExpression) []int {
	args := make([]int, len(n.ArgumentList))
	for i, a := range n.ArgumentList {
		args[i] = in.eval(a)
	}
	return args
}

// action restarts the op counter (a new beat begins) and hands the action to
// the host (a negative direction turns move/scan/fire into a wait); resetting first also covers an Unwind panic out of Do.
func (in *Interp) action(k ActionKind, args []int) int {
	a := Action{Kind: k}
	switch {
	case len(args) == 0:
	case args[0] < 0:
		a.Kind = ActWait
	default:
		a.Dir = wrap(args[0])
	}
	in.ops = 0
	in.acts.Do(a)
	return 0
}

var freeBuiltins = map[string]func(*Interp, []int) int{
	"left":    func(_ *Interp, a []int) int { return wrap(a[0] - 1) },
	"right":   func(_ *Interp, a []int) int { return wrap(a[0] + 1) },
	"back":    func(_ *Interp, a []int) int { return wrap(a[0] + 2) },
	"abs":     func(_ *Interp, a []int) int { return clamp(abs(a[0])) },
	"min":     func(_ *Interp, a []int) int { return min(a[0], a[1]) },
	"max":     func(_ *Interp, a []int) int { return max(a[0], a[1]) },
	"dist":    func(_ *Interp, a []int) int { return clamp(abs(a[0]-a[2]) + abs(a[1]-a[3])) },
	"chance":  (*Interp).chance,
	"rand":    (*Interp).randRange,
	"pick":    (*Interp).pick,
	"free":    (*Interp).free,
	"count":   (*Interp).count,
	"nearest": (*Interp).nearest,
	"wall":    func(in *Interp, a []int) int { return b2i(in.env.Wall(a[0], a[1])) },
}

// intn draws from the bot's own dice.
func (in *Interp) intn(n int) int {
	var v int
	v, in.rng = in.rng.intn(n)
	return v
}

// chance(p) is true p percent of the time; p <= 0 never, p >= 100 always.
func (in *Interp) chance(a []int) int { return b2i(in.intn(100) < a[0]) }

// randRange is rand(lo, hi), both ends inclusive; swapped bounds are fixed.
func (in *Interp) randRange(a []int) int {
	lo, hi := min(a[0], a[1]), max(a[0], a[1])
	return lo + in.intn(hi-lo+1)
}

// pick chooses uniformly among the listed directions that are not walls (so
// repeating a direction weights it). With none open it returns the first.
func (in *Interp) pick(a []int) int {
	open := make([]int, 0, len(a))
	for _, d := range a {
		if d = wrap(d); in.env.Open(d) {
			open = append(open, d)
		}
	}
	if len(open) == 0 {
		return wrap(a[0])
	}
	return open[in.intn(len(open))]
}
