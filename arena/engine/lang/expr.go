package lang

import (
	"fmt"
	"math"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/token"
)

var unknownNameHints = map[string]string{
	"undefined": "there is no undefined value; memory starts at 0, so use 0",
	"NaN":       "there is no NaN; all numbers are whole numbers",
	"Infinity":  "there is no Infinity; numbers are clamped to +-9999",
	"Math":      "there is no Math object; use abs(x), min(x, y), max(x, y), rand(lo, hi)",
	"console":   "bots cannot print; watch num[0]..num[7] on the phone live view instead",
}

func (c *checker) ident(n *ast.Identifier) {
	name := n.Name.String()
	switch {
	case c.params[name]:
	case name == trailName:
		c.errNode(n, "'trail' cannot be used on its own", trailHint)
	case name == numName:
		c.errNode(n, "'num' cannot be used on its own", numHint)
	case name == pinName:
		c.errNode(n, "'pin' cannot be used on its own", pinHint)
	case isPlaceName(name):
		c.notNumber(n)
	case IsReading(name):
	default:
		c.badName(n, name)
	}
}

// badName reports a name used as a value that is not one.
func (c *checker) badName(n *ast.Identifier, name string) {
	if b, ok := builtins[name]; ok {
		c.errNode(n, "'"+name+"' is a built-in function, not a value", "call it like "+b.Usage)
		return
	}
	if f, ok := c.p.Funcs[name]; ok {
		c.errNode(n, "'"+name+"' is a function, not a value", "call it like "+signature(f))
		return
	}
	hint, ok := unknownNameHints[name]
	if !ok {
		hint = "'" + name + "' is unknown: use memory (num[0]..num[7], pin[0]..pin[7]), a reading (enemy, hp, x, ...) or a parameter"
	}
	if removedSlots[name] {
		hint = removedSlotsHint
	}
	c.errNode(n, "unknown name '"+name+"'", hint)
}

func signature(f *Func) string { return f.Name + "(" + strings.Join(f.Params, ", ") + ")" }

// number accepts whole values (5, 5.0, 1e3); the interpreter clamps them to
// +-9999. Fractions and non-finite values are rejected.
func (c *checker) number(n *ast.NumberLiteral) {
	if !wholeNumber(n.Value) {
		c.errNode(n, "only whole numbers are allowed", "use an integer; division is integer division, so a / 2 rounds toward zero")
	}
}

func wholeNumber(v any) bool {
	f, ok := v.(float64)
	if !ok {
		_, isInt := v.(int64)
		return isInt
	}
	return !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f)
}

// boolean accepts true/false; the interpreter evaluates them as 1 and 0.
func (c *checker) boolean(*ast.BooleanLiteral) {}

// --- unary -----------------------------------------------------------------

var unaryHints = map[token.Token]string{
	token.TYPEOF:      "everything is a whole number, so typeof is not needed",
	token.DELETE:      "there is nothing to delete; memory (num[i]) is only ever set with =",
	token.VOID:        "write the number itself, for example 0",
	token.PLUS:        "drop the +; write the number or num[i] directly",
	token.BITWISE_NOT: "use ordinary arithmetic (+ - * / %)",
}

func (c *checker) unary(n *ast.UnaryExpression) {
	switch n.Operator {
	case token.INCREMENT, token.DECREMENT:
		c.incDec(n)
	case token.NOT, token.MINUS:
		c.visit(n.Operand)
	default:
		c.unaryForbidden(n)
	}
}

func (c *checker) unaryForbidden(n *ast.UnaryExpression) {
	hint, ok := unaryHints[n.Operator]
	if !ok {
		hint = genericForbiddenHint
	}
	c.errNode(n, "operator '"+n.Operator.String()+"' is not allowed", hint)
	c.visit(n.Operand)
}

func (c *checker) incDec(n *ast.UnaryExpression) {
	c.target(n, n.Operand)
}

// target checks the left side of an assignment or ++ / --.
func (c *checker) target(n ast.Node, e ast.Expression) {
	if msg, hint := c.assignProblem(e); msg != "" {
		c.errNode(n, msg, hint)
	} else if _, ok := numEntry(e); ok {
		c.visit(e)
	}
}

// --- binary ----------------------------------------------------------------

var allowedBinary = map[token.Token]bool{
	token.PLUS: true, token.MINUS: true, token.MULTIPLY: true, token.SLASH: true, token.REMAINDER: true,
	token.EQUAL: true, token.NOT_EQUAL: true, token.LESS: true, token.LESS_OR_EQUAL: true,
	token.GREATER: true, token.GREATER_OR_EQUAL: true, token.LOGICAL_AND: true, token.LOGICAL_OR: true,
}

const arithmeticHint = "use plain arithmetic (+ - * / %) and comparisons instead"

var binaryHints = map[token.Token]string{
	token.AND:                  "for 'and' use && (both conditions true)",
	token.OR:                   "for 'or' use || (either condition true)",
	token.EXCLUSIVE_OR:         arithmeticHint,
	token.SHIFT_LEFT:           "multiply instead, for example a * 2",
	token.SHIFT_RIGHT:          "divide instead, for example a / 2",
	token.UNSIGNED_SHIFT_RIGHT: "divide instead, for example a / 2",
	token.EXPONENT:             "multiply repeatedly, for example a * a",
	token.STRICT_EQUAL:         "use == (there is only one kind of number)",
	token.STRICT_NOT_EQUAL:     "use != (there is only one kind of number)",
	token.IN:                   arithmeticHint,
	token.INSTANCEOF:           arithmeticHint,
	token.COALESCE:             "use a ? b : c instead",
}

func (c *checker) binary(n *ast.BinaryExpression) {
	if !allowedBinary[n.Operator] {
		hint, ok := binaryHints[n.Operator]
		if !ok {
			hint = arithmeticHint
		}
		c.errNode(n, "operator '"+n.Operator.String()+"' is not allowed", hint)
	}
	if isPlaceCompare(n) {
		c.placeOperand(n.Left, compareMsg, compareHint)
		c.placeOperand(n.Right, compareMsg, compareHint)
		return
	}
	c.visit(n.Left)
	c.visit(n.Right)
}

func (c *checker) conditional(n *ast.ConditionalExpression) {
	if isPlaceExpr(n) {
		c.notNumber(n)
		c.placeExpr(n)
		return
	}
	c.visit(n.Test)
	c.visit(n.Consequent)
	c.visit(n.Alternate)
}

// --- assignment ------------------------------------------------------------

func (c *checker) assign(n *ast.AssignExpression) {
	if _, ok := pinEntry(n.Left); ok {
		if ast.Expression(n) != c.stmtExpr {
			c.errNode(n, "a place is not a number",
				"a pin assignment has no value: write it as its own statement")
		}
		c.assignPin(n)
		return
	}
	c.visit(n.Right)
	// goja stores "+=" as PLUS and "=" as ASSIGN: the operator without its "=".
	switch n.Operator {
	case token.ASSIGN, token.PLUS, token.MINUS:
	default:
		t := c.text(n.Left)
		op := n.Operator.String()
		c.errNode(n, "operator '"+op+"=' is not allowed",
			fmt.Sprintf("only =, += and -= are allowed; write it out, for example %s = %s %s 2", t, t, op))
	}
	c.target(n, n.Left)
}

// assignProblem explains why e cannot be written to, or returns "" for a num[i] cell.
func (c *checker) assignProblem(e ast.Expression) (msg, hint string) {
	id, ok := e.(*ast.Identifier)
	if !ok {
		return c.memberProblem(e)
	}
	return c.nameProblem(id.Name.String())
}

const slotsOnly = "only num[0]..num[7] (numbers) and pin[0]..pin[7] (places, set with pin[i] = at(x, y)) can be assigned"

func (c *checker) nameProblem(name string) (msg, hint string) {
	switch {
	case IsReading(name):
		return "cannot assign to '" + name + "': it is read-only", name + " is read-only (set by the game, for example scan(d) sets enemy); " + slotsOnly
	case c.params[name]:
		return "cannot assign to parameter '" + name + "'", "parameters are read-only temporaries; copy the value into num[i] to change it (" + slotsOnly + ")"
	}
	if removedSlots[name] {
		return "cannot assign to '" + name + "'", removedSlotsHint
	}
	return "cannot assign to '" + name + "'", "there are no variables; " + slotsOnly
}

func (c *checker) memberProblem(e ast.Expression) (msg, hint string) {
	if _, ok := numEntry(e); ok {
		return "", ""
	}
	if _, ok := pinEntry(e); ok {
		return pinArithmeticMsg, pinSetHint
	}
	t := c.text(e)
	if strings.HasPrefix(t, trailName) {
		return "cannot assign to '" + t + "': trail is read-only", "trail is read-only history; " + slotsOnly
	}
	return "cannot assign to '" + t + "'", slotsOnly
}

// --- members ---------------------------------------------------------------

func isTrail(e ast.Expression) bool {
	id, ok := e.(*ast.Identifier)
	return ok && id.Name.String() == trailName
}

// trailEntry reports whether e is trail[...] and returns the index node.
func trailEntry(e ast.Expression) (*ast.BracketExpression, bool) {
	b, ok := e.(*ast.BracketExpression)
	return b, ok && isTrail(b.Left)
}

// numEntry reports whether e is num[...] and returns it.
func numEntry(e ast.Expression) (*ast.BracketExpression, bool) {
	b, ok := e.(*ast.BracketExpression)
	if !ok {
		return nil, false
	}
	id, ok := b.Left.(*ast.Identifier)
	return b, ok && id.Name.String() == numName
}

const numHint = "memory is num[0]..num[7]: write num[i] with an index from 0 to 7"

func (c *checker) dot(n *ast.DotExpression) {
	field := n.Identifier.Name.String()
	if isPlaceExpr(n.Left) {
		c.placeField(n, field)
	} else if isTrail(n.Left) {
		c.trailProp(n, field)
	} else if b, ok := trailEntry(n.Left); ok {
		c.visit(b.Member)
		c.trailField(n, field)
	} else {
		c.errNode(n, "member access on '"+c.text(n)+"' is not allowed", trailHint)
	}
}

func (c *checker) trailProp(n *ast.DotExpression, field string) {
	if field != "length" {
		c.errNode(n, "'"+field+"' is not a property of trail", trailHint)
	}
}

func (c *checker) trailField(n *ast.DotExpression, field string) {
	if !trailFields[field] {
		c.errNode(n, "'"+field+"' is not a field of a trail entry", trailHint)
	}
}

// bracket is reached only for indexing that is not the left side of a dot:
// either a bare trail[i] (needs a field) or indexing something else.
func (c *checker) bracket(n *ast.BracketExpression) {
	if _, ok := numEntry(n); ok {
		c.numIndex(n)
		return
	}
	if isPlaceExpr(n) {
		c.notNumber(n)
		c.placeExpr(n)
		return
	}
	if isTrail(n.Left) {
		c.visit(n.Member)
		c.errNode(n, "'"+c.text(n)+"' needs a field", trailHint)
		return
	}
	c.errNode(n, "indexing '"+c.text(n)+"' is not allowed", trailHint)
}

// numIndex checks num[i]: a constant index must be 0..7.
func (c *checker) numIndex(n *ast.BracketExpression) { c.cellIndex(n, numName, NumSize, numHint) }

// cellIndex checks name[i]: a constant index must be inside the array.
func (c *checker) cellIndex(n *ast.BracketExpression, name string, size int, hint string) {
	c.visit(n.Member)
	if i, ok := constIndex(n.Member); ok && (i < 0 || i >= size) {
		c.errNode(n, fmt.Sprintf("%s[%d] does not exist", name, i), hint)
	}
}

// constIndex is the value of a literal index such as 3 or -1.
func constIndex(e ast.Expression) (int, bool) {
	switch v := e.(type) {
	case *ast.NumberLiteral:
		f, ok := v.Value.(float64)
		if !ok {
			i, isInt := v.Value.(int64)
			return int(i), isInt
		}
		return int(f), true
	case *ast.UnaryExpression:
		i, ok := constIndex(v.Operand)
		return -i, ok && v.Operator == token.MINUS
	}
	return 0, false
}

// --- calls -----------------------------------------------------------------

func (c *checker) call(n *ast.CallExpression) {
	if isPlaceExpr(n) {
		c.notNumber(n)
	}
	c.callChecked(n)
}

// callChecked checks a call whatever type its value has.
func (c *checker) callChecked(n *ast.CallExpression) {
	user := c.isUserCall(n)
	for _, a := range n.ArgumentList {
		if user && isPlaceExpr(a) {
			c.errNode(a, "places cannot be passed to functions", "functions take numbers: pass the pin index and read pin[i] inside")
			c.placeExpr(a)
			continue
		}
		if c.takesPlace(n) {
			c.placeOperand(a, "'"+n.Callee.(*ast.Identifier).Name.String()+"' takes a place",
				"pass a place: pin[i], here(), at(x, y) or none (for coordinates use at(x, y))")
			continue
		}
		c.visit(a)
	}
	switch callee := n.Callee.(type) {
	case *ast.Identifier:
		c.callNamed(n, callee.Name.String())
	case *ast.DotExpression:
		c.methodCall(n, callee)
	default:
		c.visit(n.Callee)
		c.errNode(n, "only named functions can be called", "call a function by its name, like f(1)")
	}
}

func (c *checker) methodCall(n *ast.CallExpression, callee *ast.DotExpression) {
	hint := "there are no objects or methods; call the built-in functions directly"
	if name := callee.Identifier.Name.String(); builtins[name].Name != "" {
		hint = fmt.Sprintf("there are no objects; call %s directly", builtins[name].Usage)
	}
	c.errNode(n, "method calls are not allowed", hint)
}

func (c *checker) callNamed(n *ast.CallExpression, name string) {
	if b, ok := builtins[name]; ok {
		c.arity(n, name, b.Min, b.Max, "usage: "+b.Usage)
	} else if _, isEntry := handlers[name]; isEntry {
		c.errNode(n, "'"+name+"' cannot be called", "loop, onHit and onBump are entry points the game calls itself")
	} else if f, ok := c.p.Funcs[name]; ok {
		c.arity(n, name, len(f.Params), len(f.Params),
			fmt.Sprintf("%s is declared with %d parameter(s): %s", name, len(f.Params), signature(f)))
		c.edges = append(c.edges, edge{from: c.fn.Name, to: name, at: n.Idx0()})
	} else {
		c.unknownCallee(n, name)
	}
}

func (c *checker) unknownCallee(n *ast.CallExpression, name string) {
	if IsReading(name) || c.params[name] {
		c.errNode(n, "'"+name+"' is not a function", "readings and parameters hold numbers; only functions can be called, like scan(d)")
		return
	}
	c.errNode(n, "unknown function '"+name+"'", "built-ins: "+builtinNames+"; or declare function "+name+"(...) { } at the top level")
}

func (c *checker) arity(n *ast.CallExpression, name string, min, max int, hint string) {
	got := len(n.ArgumentList)
	if got >= min && (max < 0 || got <= max) {
		return
	}
	c.errNode(n, fmt.Sprintf("%s expects %s but got %d", name, arityText(min, max), got), hint)
}

func arityText(min, max int) string {
	switch {
	case max < 0:
		return fmt.Sprintf("at least %d argument(s)", min)
	case min == max:
		return fmt.Sprintf("%d argument(s)", min)
	}
	return fmt.Sprintf("%d to %d argument(s)", min, max)
}
