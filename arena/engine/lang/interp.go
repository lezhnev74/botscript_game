package lang

// Interp is a tree-walking evaluator over a validated Program. One Interp
// belongs to one bot and is driven by one goroutine; it has no goroutines or
// locks of its own. Blocking (actions, budget) happens inside the Actions
// hooks, so a stepper wraps it by implementing them with channels.
//
// Semantics decided here (LANGUAGE.md is silent on most of them):
//   - Numbers are ints clamped to -9999..9999 after every operation, so
//     9999 + 9999 - 9999 is 0 (the intermediate 19998 clamps to 9999).
//     Number literals and readings are clamped too.
//   - "/" is integer division truncating toward zero (-7 / 2 is -3), "%"
//     keeps the sign of the dividend (-7 % 3 is -1); x / 0 and x % 0 are 0.
//   - Comparisons and ! give 1 or 0; any non-zero value is true. && and ||
//     short-circuit and, like JS, yield the deciding operand (0 && x is 0,
//     5 && 6 is 6, 0 || 2 is 2, 4 || x is 4); true and false are 1 and 0.
//   - Directions >= 0 given to actions and turning built-ins wrap modulo 4
//     (move(5) is move(E), left(N) is W). A negative direction given to
//     move, scan or fire makes the beat a wait() (move(path(p)) with -1
//     stands still).
//   - A function without a return value yields 0. Actions yield 0.
//   - trail[i] outside 0..trail.length-1 reads -1 in every field.
//   - num[i] outside 0..7 reads 0 and writes nothing (a computed index; a
//     constant one is rejected by the validator).
//   - pin[i] outside 0..7 reads none and writes nothing (a computed index; a
//     constant one is rejected by the validator). none.x and none.y are -1.
//     Places compare with == / != by cell; at(x, y) keeps its coordinates as given.
//   - Ops: every executed statement and every evaluated expression node costs
//     1 op. Assignment targets and ++/-- operands are not evaluated, but
//     their index expression is, so num[0] = 1 costs the assignment, the
//     index and the value; calling a user function costs only its call node and body.
//     The budget is per beat: the counter carries over between Run calls
//     and is reset to 0 when an action is issued (before Actions.Do runs).
//     When it passes Budget the counter is set to 1 (the waiting op belongs
//     to the new beat) and Actions.OutOfBudget runs. Budget 0 = unlimited.
type Interp struct {
	Prog   *Program // replace to hot-swap; Mem and dice are kept
	Mem    Mem      // survives Run calls, interrupts and hot-swaps
	Budget int      // ops per beat, DefaultBudget unless changed

	env  Env
	acts Actions
	rng  rng

	scratch int // target of out-of-range num[i] accesses, see cell
	ops     int
	line    int // line of the latest action call (0 = none yet), for badges
	frames  []frame
	ret     int
}

// Mem is the memory a bot keeps for life: eight integers (+-9999, start 0)
// and eight places (start none).
type Mem struct {
	Num [NumSize]int
	Pin [PinSize]Place
}

type frame struct {
	fn   *Func
	args []int
}

// NewInterp returns an interpreter with zeroed memory. seed is the bot's dice
// seed, see SeedFor.
func NewInterp(p *Program, env Env, acts Actions, seed uint64) *Interp {
	return &Interp{Prog: p, Budget: DefaultBudget, env: env, acts: acts, rng: newRNG(seed)}
}

// Line is the source line of the latest action call, 0 before the first.
func (in *Interp) Line() int { return in.line }

// Ops is the op count of the current beat (since the last action or stall).
func (in *Interp) Ops() int { return in.ops }

// Run calls fn with args (entry points: Prog.Loop, OnHit, OnBump) and returns
// when it returns. unwound is true if an Actions hook aborted it with
// panic(Unwind{}); any other panic propagates.
func (in *Interp) Run(fn *Func, args ...int) (unwound bool) {
	in.frames = in.frames[:0]
	in.ret = 0
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(Unwind); !ok {
				panic(r)
			}
			unwound = true
		}
	}()
	in.invoke(fn, args)
	return false
}

func (in *Interp) tick() {
	in.ops++
	if in.Budget > 0 && in.ops > in.Budget {
		in.ops = 1
		in.acts.OutOfBudget()
	}
}

func (in *Interp) invoke(fn *Func, args []int) int {
	in.frames = append(in.frames, frame{fn, args})
	f := in.execList(fn.Decl.Body.List)
	in.frames = in.frames[:len(in.frames)-1]
	r := 0
	if f == flowReturn {
		r = in.ret
	}
	in.ret = 0
	return r
}

// param returns the value of a parameter of the running function.
func (in *Interp) param(name string) (int, bool) {
	fr := in.frames[len(in.frames)-1]
	for i, p := range fr.fn.Params {
		if p == name {
			return fr.args[i], true
		}
	}
	return 0, false
}
