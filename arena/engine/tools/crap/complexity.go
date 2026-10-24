package main

import (
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
)

// funcInfo describes one function or method with its cyclomatic complexity.
type funcInfo struct {
	File                string // module-relative path
	Name                string
	StartLine, StartCol int
	EndLine, EndCol     int
	Complexity          int
}

var generatedRe = regexp.MustCompile(`(?m)^Code generated .* DO NOT EDIT\.$`)

// analyzeFile returns complexity for every function in src. Generated files yield nothing.
func analyzeFile(rel, filename string, src []byte) ([]funcInfo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if isGenerated(f) || isBuildIgnored(f) {
		return nil, nil
	}
	var infos []funcInfo
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			infos = append(infos, describe(fset, rel, fn))
		}
	}
	return infos, nil
}

// isBuildIgnored reports whether the file's //go:build constraint is false for
// the current GOOS/GOARCH and default tags.
func isBuildIgnored(f *ast.File) bool {
	for _, cg := range f.Comments {
		if cg.Pos() >= f.Package {
			break
		}
		if slices.ContainsFunc(cg.List, constraintFails) {
			return true
		}
	}
	return false
}

func constraintFails(c *ast.Comment) bool {
	if !constraint.IsGoBuild(c.Text) {
		return false
	}
	expr, err := constraint.Parse(c.Text)
	return err == nil && !expr.Eval(tagMatches)
}

func tagMatches(tag string) bool {
	ctx := build.Default
	switch {
	case tag == ctx.GOOS || tag == ctx.GOARCH || tag == "gc":
		return true
	case tag == "cgo":
		return ctx.CgoEnabled
	case tag == "unix":
		return slices.Contains([]string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "linux", "netbsd", "openbsd", "solaris"}, ctx.GOOS)
	}
	return slices.Contains(ctx.ReleaseTags, tag)
}

func isGenerated(f *ast.File) bool {
	for _, cg := range f.Comments {
		if cg.Pos() < f.Package && generatedRe.MatchString(cg.Text()) {
			return true
		}
	}
	return false
}

func describe(fset *token.FileSet, rel string, fn *ast.FuncDecl) funcInfo {
	start, end := fset.Position(fn.Pos()), fset.Position(fn.End())
	return funcInfo{
		File: rel, Name: funcName(fn),
		StartLine: start.Line, StartCol: start.Column,
		EndLine: end.Line, EndCol: end.Column,
		Complexity: complexity(fn),
	}
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return recvName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

// complexity is McCabe cyclomatic complexity: 1 + decision points (closures included).
func complexity(fn *ast.FuncDecl) int {
	c := 1
	ast.Inspect(fn, func(n ast.Node) bool {
		if isBranch(n) {
			c++
		}
		return true
	})
	return c
}

func isBranch(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
		return true
	case *ast.CaseClause:
		return v.List != nil
	case *ast.CommClause:
		return v.Comm != nil
	case *ast.BinaryExpr:
		return v.Op == token.LAND || v.Op == token.LOR
	}
	return false
}
