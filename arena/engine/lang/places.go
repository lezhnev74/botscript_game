package lang

import (
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/token"
)

// Places are the second value type next to numbers (LANGUAGE.md section 4):
// none, here(), at(x, y), pin[i] and the reading heard. A place is never an
// int, so the evaluator has a separate path for them (place, below) and the
// validator a small static type check: a place expression is recognised by
// its syntax (isPlaceExpr), and every context that takes one - the right side
// of pin[i] = ..., == / !=, .x / .y - checks its operand with placeExpr; any
// other context reaches the ordinary rules and reports "a place is not a number".
// A built-in that takes a place (path, steps) only needs to run its argument
// through place instead of eval.

const (
	pinName  = "pin"
	noneName = "none"
	// heardName is the read-only place of the nearest sound last heard.
	heardName = "heard"
)

// PinSize is the number of places in pin.
const PinSize = 8

// Place is a cell of the map or nothing (Set false, the zero value). Cells
// outside the map are fine; they are just unreachable.
type Place struct {
	X, Y int
	Set  bool
}

var placeFuncs = setOf("here", "at")

var placeFields = setOf("x", "y")

func isNamed(e ast.Expression, name string) bool {
	id, ok := e.(*ast.Identifier)
	return ok && id.Name.String() == name
}

// pinEntry reports whether e is pin[...] and returns it.
func pinEntry(e ast.Expression) (*ast.BracketExpression, bool) {
	b, ok := e.(*ast.BracketExpression)
	return b, ok && isNamed(b.Left, pinName)
}

// isPlaceName reports the names that are places by themselves.
func isPlaceName(name string) bool {
	return name == noneName || name == heardName
}

// isPlaceExpr reports whether e has the type place: none, heard, pin[i],
// here(), at(x, y) or a ? : with a place branch.
func isPlaceExpr(e ast.Expression) bool {
	switch n := e.(type) {
	case *ast.Identifier:
		return isPlaceName(n.Name.String())
	case *ast.BracketExpression:
		return isNamed(n.Left, pinName)
	case *ast.CallExpression:
		id, ok := n.Callee.(*ast.Identifier)
		return ok && placeFuncs[id.Name.String()]
	case *ast.ConditionalExpression:
		return isPlaceExpr(n.Consequent) || isPlaceExpr(n.Alternate)
	}
	return false
}

// isPlaceCompare reports a == or != with a place on either side.
func isPlaceCompare(n *ast.BinaryExpression) bool {
	return (n.Operator == token.EQUAL || n.Operator == token.NOT_EQUAL) && (isPlaceExpr(n.Left) || isPlaceExpr(n.Right))
}

// --- validator -------------------------------------------------------------

const (
	pinHint          = "places live in pin[0]..pin[7]: write pin[i] with an index from 0 to 7"
	pinArithmeticMsg = "places have no arithmetic"
	pinSetHint       = "a pin is only set with =: pin[i] = at(x, y) or pin[i] = here()"
	compareMsg       = "compare a place with a place"
	compareHint      = "== and != compare two places (use none to ask if a pin is empty); for numbers use pin[i].x and pin[i].y"
)

// notNumber reports a place used where a number is needed.
func (c *checker) notNumber(e ast.Expression) {
	t := c.text(e)
	c.errNode(e, "a place is not a number",
		"places have no arithmetic: use "+t+".x or "+t+".y for a number, or compare places with == / !=")
}

// placeExpr checks an expression already known to be a place (isPlaceExpr).
func (c *checker) placeExpr(e ast.Expression) {
	switch n := e.(type) {
	case *ast.BracketExpression:
		c.cellIndex(n, pinName, PinSize, pinHint)
	case *ast.CallExpression:
		c.callChecked(n)
	case *ast.ConditionalExpression:
		c.visit(n.Test)
		c.placeOperand(n.Consequent, "a place and a number cannot be mixed in ? :", "make both branches places")
		c.placeOperand(n.Alternate, "a place and a number cannot be mixed in ? :", "make both branches places")
	}
}

// placeOperand checks e where a place is required.
func (c *checker) placeOperand(e ast.Expression, msg, hint string) {
	if isPlaceExpr(e) {
		c.placeExpr(e)
		return
	}
	c.errNode(e, msg, hint)
	c.visit(e)
}

// assignPin checks pin[i] = place; no other operator makes sense for places.
func (c *checker) assignPin(n *ast.AssignExpression) {
	if n.Operator != token.ASSIGN {
		c.errNode(n, pinArithmeticMsg, pinSetHint)
	}
	c.placeOperand(n.Right, "a pin holds a place", "use at(x, y) or here()")
	c.placeExpr(n.Left)
}

func (c *checker) placeField(n *ast.DotExpression, field string) {
	c.placeExpr(n.Left)
	if !placeFields[field] {
		c.errNode(n, "'"+field+"' is not a field of a place", "a place has only .x and .y (-1 for none)")
	}
}

// takesPlace reports a call of path or steps, whose argument is a place.
func (c *checker) takesPlace(n *ast.CallExpression) bool {
	id, ok := n.Callee.(*ast.Identifier)
	return ok && placeTakers[id.Name.String()] != nil
}

// isUserCall reports a call of a function the program declares.
func (c *checker) isUserCall(n *ast.CallExpression) bool {
	id, ok := n.Callee.(*ast.Identifier)
	if !ok {
		return false
	}
	_, builtin := builtins[id.Name.String()]
	return !builtin && c.p.Funcs[id.Name.String()] != nil
}

// --- interpreter -----------------------------------------------------------

var placeBuiltins = map[string]func(*Interp, []int) Place{
	"here": func(in *Interp, _ []int) Place {
		return Place{X: clamp(in.env.Reading("x")), Y: clamp(in.env.Reading("y")), Set: true}
	},
	"at": func(_ *Interp, a []int) Place { return Place{X: a[0], Y: a[1], Set: true} },
}

// placeTakers are the built-ins whose single argument is a place; they are
// evaluated with place instead of eval. An unset place is unreachable (-1).
var placeTakers = map[string]func(*Interp, Place) int{
	"path":  func(in *Interp, p Place) int { return in.reach(p, in.env.Path) },
	"steps": func(in *Interp, p Place) int { return in.reach(p, in.env.Steps) },
}

// reach asks the world about a place; none is unreachable.
func (in *Interp) reach(p Place, ask func(x, y int) int) int {
	if !p.Set {
		return -1
	}
	return ask(p.X, p.Y)
}

// nearest is the index of the set pin with the fewest steps from here, ties
// the lowest index; -1 if no pin is reachable.
func (in *Interp) nearest([]int) int {
	best, bestSteps := -1, -1
	for i, p := range in.Mem.Pin {
		if s := in.reach(p, in.env.Steps); s >= 0 && (best < 0 || s < bestSteps) {
			best, bestSteps = i, s
		}
	}
	return best
}

// place evaluates a place expression (1 op per node, like eval). Anything
// that is not pin[i], a call or a ? : is none.
func (in *Interp) place(e ast.Expression) Place {
	in.tick()
	switch n := e.(type) {
	case *ast.BracketExpression:
		return in.pinAt(in.eval(n.Member))
	case *ast.CallExpression:
		return placeBuiltins[n.Callee.(*ast.Identifier).Name.String()](in, in.evalArgs(n))
	case *ast.Identifier:
		return in.identPlace(n.Name.String())
	case *ast.ConditionalExpression:
		if in.eval(n.Test) != 0 {
			return in.place(n.Consequent)
		}
		return in.place(n.Alternate)
	}
	return Place{}
}

// identPlace is a named place: heard is the last sound, none (and
// anything else) is no place.
func (in *Interp) identPlace(name string) Place {
	switch name {
	case heardName:
		return in.env.Heard()
	}
	return Place{}
}

// pinAt is pin[i]; an index outside 0..7 reads none.
func (in *Interp) pinAt(i int) Place {
	if i < 0 || i >= PinSize {
		return Place{}
	}
	return in.Mem.Pin[i]
}

// assignPin is pin[i] = place; the index is read before the right side runs,
// and an index outside 0..7 writes nothing.
func (in *Interp) assignPin(n *ast.AssignExpression) int {
	i := in.eval(n.Left.(*ast.BracketExpression).Member)
	p := in.place(n.Right)
	if i >= 0 && i < PinSize {
		in.Mem.Pin[i] = p
	}
	return 0
}

// comparePlaces is a == or != between two places.
func (in *Interp) comparePlaces(n *ast.BinaryExpression) int {
	l := in.place(n.Left)
	return b2i((l == in.place(n.Right)) == (n.Operator == token.EQUAL))
}

// placeField is p.x or p.y; none reads -1.
func (in *Interp) placeField(n *ast.DotExpression) int {
	p := in.place(n.Left)
	switch {
	case !p.Set:
		return -1
	case n.Identifier.Name.String() == "x":
		return p.X
	}
	return p.Y
}

// free is the first empty pin index, -1 when all eight are set.
func (in *Interp) free([]int) int {
	for i, p := range in.Mem.Pin {
		if !p.Set {
			return i
		}
	}
	return -1
}

// count is the number of pins that are set.
func (in *Interp) count([]int) int {
	n := 0
	for _, p := range in.Mem.Pin {
		n += b2i(p.Set)
	}
	return n
}
