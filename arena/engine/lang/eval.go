package lang

import (
	"reflect"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/token"
)

var exprRules = map[reflect.Type]func(*Interp, ast.Expression) int{}

func exprRule[N ast.Expression](f func(*Interp, N) int) {
	exprRules[reflect.TypeFor[N]()] = func(in *Interp, e ast.Expression) int { return f(in, e.(N)) }
}

func init() {
	exprRule((*Interp).number)
	exprRule((*Interp).boolean)
	exprRule((*Interp).ident)
	exprRule((*Interp).unary)
	exprRule((*Interp).binary)
	exprRule((*Interp).assign)
	exprRule((*Interp).conditional)
	exprRule((*Interp).call)
	exprRule((*Interp).dot)
	exprRule((*Interp).index)
}

// eval evaluates one expression node (1 op) to a clamped int.
func (in *Interp) eval(e ast.Expression) int {
	in.tick()
	return exprRules[reflect.TypeOf(e)](in, e)
}

func (in *Interp) truthy(e ast.Expression) bool { return e == nil || in.eval(e) != 0 }

func (in *Interp) number(n *ast.NumberLiteral) int {
	if f, ok := n.Value.(float64); ok {
		return int(max(-Limit, min(Limit, f)))
	}
	return clamp(int(n.Value.(int64)))
}

func (in *Interp) boolean(n *ast.BooleanLiteral) int { return b2i(n.Value) }

var constants = map[string]int{
	"N": 0, "E": 1, "S": 2, "W": 3,
	"SCAN": int(ActScan), "MOVE": int(ActMove), "FIRE": int(ActFire), "WAIT": int(ActWait), "IDLE": 4,
}

func (in *Interp) ident(n *ast.Identifier) int { return in.lookup(n.Name.String()) }

func (in *Interp) lookup(name string) int {
	if v, ok := in.param(name); ok {
		return v
	}
	if v, ok := constants[name]; ok {
		return v
	}
	return clamp(in.env.Reading(name))
}

// index reads num[i]; an index outside 0..7 reads 0.
func (in *Interp) index(n *ast.BracketExpression) int { return *in.cell(n) }

// cell is the memory cell num[i] names. An index outside 0..7 yields a fresh
// zero scratch cell, so reads give 0 and writes vanish.
func (in *Interp) cell(n *ast.BracketExpression) *int {
	i := in.eval(n.Member)
	if i < 0 || i >= NumSize {
		in.scratch = 0
		return &in.scratch
	}
	return &in.Mem.Num[i]
}

func (in *Interp) conditional(n *ast.ConditionalExpression) int {
	if in.eval(n.Test) != 0 {
		return in.eval(n.Consequent)
	}
	return in.eval(n.Alternate)
}

func (in *Interp) binary(n *ast.BinaryExpression) int {
	switch n.Operator {
	case token.LOGICAL_AND:
		return in.andOr(n, false)
	case token.LOGICAL_OR:
		return in.andOr(n, true)
	}
	if isPlaceCompare(n) {
		return in.comparePlaces(n)
	}
	l := in.eval(n.Left)
	return binaryOps[n.Operator](l, in.eval(n.Right))
}

// andOr short-circuits and yields the deciding operand: the left one when it
// decides (zero for &&, non-zero for ||), else the right one.
func (in *Interp) andOr(n *ast.BinaryExpression, or bool) int {
	l := in.eval(n.Left)
	if (l != 0) == or {
		return l
	}
	return in.eval(n.Right)
}

func (in *Interp) unary(n *ast.UnaryExpression) int {
	switch n.Operator {
	case token.INCREMENT:
		return in.step(n, 1)
	case token.DECREMENT:
		return in.step(n, -1)
	case token.MINUS:
		return clamp(-in.eval(n.Operand))
	}
	return b2i(in.eval(n.Operand) == 0) // token.NOT
}

// step is ++ / --: the operand is a num[i] cell, only its index is evaluated.
func (in *Interp) step(n *ast.UnaryExpression, by int) int {
	c := in.cell(n.Operand.(*ast.BracketExpression))
	old := *c
	*c = clamp(old + by)
	if n.Postfix {
		return old
	}
	return *c
}

// assign is = += -=; the index and the old cell value are read before the
// right side runs.
func (in *Interp) assign(n *ast.AssignExpression) int {
	if _, ok := pinEntry(n.Left); ok {
		return in.assignPin(n)
	}
	c := in.cell(n.Left.(*ast.BracketExpression))
	old := *c
	v := in.eval(n.Right)
	if n.Operator != token.ASSIGN {
		v = binaryOps[n.Operator](old, v)
	}
	if c == &in.scratch {
		return v
	}
	*c = v
	return v
}

// dot is trail.length or trail[i].field; out of range reads -1.
func (in *Interp) dot(n *ast.DotExpression) int {
	if isPlaceExpr(n.Left) {
		return in.placeField(n)
	}
	field := n.Identifier.Name.String()
	b, ok := n.Left.(*ast.BracketExpression)
	if !ok {
		return in.env.TrailLen()
	}
	i := in.eval(b.Member)
	if i < 0 || i >= in.env.TrailLen() {
		return -1
	}
	return clamp(in.env.TrailField(i, field))
}
