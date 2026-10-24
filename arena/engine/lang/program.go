package lang

import (
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
)

// Func is a validated top-level function declaration.
type Func struct {
	Name   string
	Params []string
	Decl   *ast.FunctionLiteral
	Line   int
}

// Program is a validated BOTSCRIPT program: the goja AST plus the resolved
// metadata the interpreter needs. A Program only exists if Validate found no
// errors, so the interpreter may trust it (every call target exists with the
// right arity, no recursion, depth <= MaxCallDepth, assignments hit num[i]).
type Program struct {
	Source string
	AST    *ast.Program
	Lines  int

	Funcs map[string]*Func
	Order []*Func // declaration order

	Loop   *Func // required
	OnHit  *Func // optional, nil if absent
	OnBump *Func // optional, nil if absent

	fset *file.FileSet
}

// Pos converts an AST index to a 1-based line and column (for badges such as
// "L12 fire(E)").
func (p *Program) Pos(idx file.Idx) (line, col int) {
	pos := p.fset.Position(idx)
	return pos.Line, pos.Column
}
