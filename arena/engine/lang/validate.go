package lang

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/parser"
)

// Validate parses src and checks it against the BOTSCRIPT whitelist. On
// success it returns the Program and no errors; otherwise a nil Program and
// every problem found, sorted by position.
func Validate(src string) (*Program, []Error) {
	p := &Program{Source: src, Lines: countLines(src), Funcs: map[string]*Func{}, fset: &file.FileSet{}}
	var errs []Error
	if p.Lines > MaxLines {
		errs = append(errs, Error{Line: MaxLines + 1, Col: 1,
			Msg:  fmt.Sprintf("the program is longer than %d lines", MaxLines),
			Hint: fmt.Sprintf("make it shorter: at most %d lines, remove blank lines, comments or repeated code", MaxLines)})
	}
	tree, perr := parser.ParseFile(p.fset, "", src, 0, parser.WithDisableSourceMaps)
	if perr != nil {
		return nil, sortErrors(append(errs, parseErrors(perr)...))
	}
	p.AST = tree
	errs = append(errs, check(p)...)
	if errs = sortErrors(errs); len(errs) > 0 {
		return nil, errs
	}
	return p, nil
}

// countLines counts physical lines; a final newline does not start a new one.
func countLines(src string) int {
	return strings.Count(strings.TrimSuffix(src, "\n"), "\n") + 1
}

func parseErrors(err error) []Error {
	var list parser.ErrorList
	if !errors.As(err, &list) {
		return []Error{{Line: 1, Col: 1, Msg: err.Error(), Hint: syntaxHint}}
	}
	out := make([]Error, 0, len(list))
	for _, e := range list {
		out = append(out, Error{Line: e.Position.Line, Col: e.Position.Column, Msg: e.Message, Hint: syntaxHint})
	}
	return out
}

const syntaxHint = "syntax error: check brackets, parentheses and semicolons near this spot"

// check runs the whitelist walk and the call-graph checks.
func check(p *Program) []Error {
	c := newChecker(p)
	for _, s := range p.AST.Body {
		c.topLevel(s)
	}
	c.requireLoop()
	for _, f := range p.Order {
		c.function(f)
	}
	c.graph()
	return c.errs
}

func (c *checker) topLevel(s ast.Statement) {
	switch n := s.(type) {
	case *ast.FunctionDeclaration:
		c.declare(n.Function)
	case *ast.EmptyStatement:
	default:
		if hasForbidden(s) {
			c.forbidden(s)
			return
		}
		c.errNode(s, "code outside functions is not allowed", "put statements inside function loop() { ... }")
	}
}

func (c *checker) requireLoop() {
	if c.p.Loop == nil {
		c.errAt(1, "the program has no loop function", "every bot needs function loop() { ... } - it runs again each time it returns")
	}
}

func (c *checker) function(f *Func) {
	c.fn = f
	c.params = setOf(f.Params...)
	c.paramsVsFuncs(f)
	c.block(f.Decl.Body)
}

// paramsVsFuncs rejects parameters named like a user function (checked after
// all declarations are collected, so declaration order does not matter).
func (c *checker) paramsVsFuncs(f *Func) {
	for _, b := range f.Decl.ParameterList.List {
		id, ok := b.Target.(*ast.Identifier)
		if ok && c.p.Funcs[id.Name.String()] != nil {
			name := id.Name.String()
			c.errNode(b, "parameter '"+name+"' has the name of a function", "pick another name; '"+name+"' is already a function you declared")
		}
	}
}
