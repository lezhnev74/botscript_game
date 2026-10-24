package lang

import (
	"reflect"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
)

type edge struct {
	from, to string
	at       file.Idx
}

// checker walks the AST. Allowed node types have a rule; everything else is
// looked up in the forbidden table (specific hint) or rejected generically.
type checker struct {
	p      *Program
	errs   []Error
	fn     *Func
	params map[string]bool
	edges  []edge
	// stmtExpr is the expression standing as a whole statement or for init/update.
	stmtExpr ast.Expression
	rules    map[reflect.Type]func(ast.Node)
}

func newChecker(p *Program) *checker {
	c := &checker{p: p, rules: map[reflect.Type]func(ast.Node){}}
	c.registerRules()
	return c
}

func add[N ast.Node](m map[reflect.Type]func(ast.Node), f func(N)) {
	m[reflect.TypeFor[N]()] = func(n ast.Node) { f(n.(N)) }
}

func (c *checker) registerRules() {
	r := c.rules
	add(r, c.block)
	add(r, c.exprStmt)
	add(r, c.ifStmt)
	add(r, c.whileStmt)
	add(r, c.forStmt)
	add(r, c.forInitExpr)
	add(r, c.returnStmt)
	add(r, c.branch)
	add(r, c.empty)
	add(r, c.ident)
	add(r, c.number)
	add(r, c.boolean)
	add(r, c.unary)
	add(r, c.binary)
	add(r, c.assign)
	add(r, c.conditional)
	add(r, c.call)
	add(r, c.dot)
	add(r, c.bracket)
}

// visit dispatches one node; nil nodes (absent else, empty for-parts) are skipped.
func (c *checker) visit(n ast.Node) {
	if n == nil {
		return
	}
	if rule, ok := c.rules[reflect.TypeOf(n)]; ok {
		rule(n)
		return
	}
	c.forbidden(n)
}

func (c *checker) forbidden(n ast.Node) {
	msg, hint, at := forbiddenInfo(c, n)
	if at == 0 {
		at = n.Idx0()
	}
	c.errAt(at, msg, hint)
}

func (c *checker) errAt(idx file.Idx, msg, hint string) {
	line, col := c.p.Pos(idx)
	c.errs = append(c.errs, Error{Line: line, Col: col, Msg: msg, Hint: hint})
}

func (c *checker) errNode(n ast.Node, msg, hint string) { c.errAt(n.Idx0(), msg, hint) }

// text is the source text of a node, for messages.
func (c *checker) text(n ast.Node) string {
	base := c.p.fset.File(n.Idx0()).Base()
	lo, hi := int(n.Idx0())-base, int(n.Idx1())-base
	if lo < 0 || hi > len(c.p.Source) || lo > hi {
		return "?"
	}
	return c.p.Source[lo:hi]
}

// keywordBefore finds the last occurrence of kw before idx (the goja AST does
// not record the position of some keywords); falls back to idx.
func (c *checker) keywordBefore(idx file.Idx, kw string) file.Idx {
	base := c.p.fset.File(idx).Base()
	off := int(idx) - base
	if i := strings.LastIndex(c.p.Source[:off], kw); i >= 0 {
		return file.Idx(base + i)
	}
	return idx
}
