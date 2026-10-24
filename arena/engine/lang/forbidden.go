package lang

import (
	"reflect"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
)

type forbiddenFn func(*checker, ast.Node) (msg, hint string, at file.Idx)

const (
	noVars       = "there are no variables: remember values in num[0]..num[7], or pass them as function parameters"
	numbersOnly  = "bots only use whole numbers; directions are N E S W and actions are SCAN MOVE FIRE WAIT IDLE"
	topLevelHint = "declare helpers as ordinary functions at the top level: function name(p) { ... }"
	noObjects    = "there are no objects; call the built-in functions directly"
)

func fixed[N ast.Node](m map[reflect.Type]forbiddenFn, msg, hint string) {
	m[reflect.TypeFor[N]()] = func(*checker, ast.Node) (string, string, file.Idx) { return msg, hint, 0 }
}

func dynamic[N ast.Node](m map[reflect.Type]forbiddenFn, f func(*checker, N) (string, string, file.Idx)) {
	m[reflect.TypeFor[N]()] = func(c *checker, n ast.Node) (string, string, file.Idx) { return f(c, n.(N)) }
}

var forbiddenTable = buildForbidden()

func buildForbidden() map[reflect.Type]forbiddenFn {
	m := map[reflect.Type]forbiddenFn{}
	buildDeclarations(m)
	buildLiterals(m)
	buildStatements(m)
	buildExpressions(m)
	return m
}

func buildDeclarations(m map[reflect.Type]forbiddenFn) {
	dynamic(m, func(_ *checker, n *ast.LexicalDeclaration) (string, string, file.Idx) {
		return "'" + n.Token.String() + "' is not allowed", noVars, 0
	})
	dynamic(m, func(_ *checker, n *ast.ForLoopInitializerLexicalDecl) (string, string, file.Idx) {
		d := n.LexicalDeclaration
		return "'" + d.Token.String() + "' is not allowed", noVars, d.Idx
	})
	fixed[*ast.VariableStatement](m, "'var' is not allowed", noVars)
	dynamic(m, func(c *checker, n *ast.ForLoopInitializerVarDeclList) (string, string, file.Idx) {
		return "'var' is not allowed", noVars, c.keywordBefore(n.Idx0(), "var")
	})
	fixed[*ast.FunctionDeclaration](m, "functions cannot be declared inside other code", topLevelHint)
	fixed[*ast.ClassDeclaration](m, "classes are not allowed", "use plain functions and num[0]..num[7]")
	fixed[*ast.ClassLiteral](m, "classes are not allowed", "use plain functions and num[0]..num[7]")
}

func buildLiterals(m map[reflect.Type]forbiddenFn) {
	fixed[*ast.StringLiteral](m, "strings are not allowed", numbersOnly)
	fixed[*ast.TemplateLiteral](m, "template strings are not allowed", numbersOnly)
	fixed[*ast.RegExpLiteral](m, "regular expressions are not allowed", numbersOnly)
	fixed[*ast.ArrayLiteral](m, "arrays are not allowed", "remember numbers in num[0]..num[7]; recent history is in trail[0..2]")
	fixed[*ast.ObjectLiteral](m, "objects are not allowed", "remember numbers in num[0]..num[7]")
	fixed[*ast.NullLiteral](m, "'null' is not allowed", "use 0 for nothing")
}

func buildStatements(m map[reflect.Type]forbiddenFn) {
	fixed[*ast.DoWhileStatement](m, "'do ... while' is not allowed", "use while (...) { ... } instead")
	fixed[*ast.SwitchStatement](m, "'switch' is not allowed", "use if / else if chains")
	fixed[*ast.ForInStatement](m, "'for ... in' is not allowed", "use a while loop with num[0] as the counter")
	fixed[*ast.ForOfStatement](m, "'for ... of' is not allowed", "use a while loop with num[0] as the counter")
	fixed[*ast.TryStatement](m, "'try' is not allowed", "there are no errors to catch; a bot cannot fail")
	fixed[*ast.ThrowStatement](m, "'throw' is not allowed", "there are no errors to throw; just return")
	fixed[*ast.LabelledStatement](m, "labels are not allowed", "use break / continue without a label")
	fixed[*ast.WithStatement](m, "'with' is not allowed", "just use num[0]..num[7] directly")
	fixed[*ast.DebuggerStatement](m, "'debugger' is not allowed", "remove it")
}

func buildExpressions(m map[reflect.Type]forbiddenFn) {
	fixed[*ast.FunctionLiteral](m, "function expressions are not allowed (no closures)", topLevelHint)
	fixed[*ast.ArrowFunctionLiteral](m, "arrow functions are not allowed (no closures)", topLevelHint)
	fixed[*ast.ThisExpression](m, "'this' is not allowed", "there are no objects")
	fixed[*ast.NewExpression](m, "'new' is not allowed", noObjects)
	fixed[*ast.SequenceExpression](m, "comma expressions are not allowed", "write separate statements, each ending with ;")
	fixed[*ast.SpreadElement](m, "spread (...) is not allowed", "list every argument separately")
}

const genericForbiddenHint = "allowed: if/else, while, for, break, continue, return, ? :, && || !, + - * / %, comparisons, = += -= ++ -- on num[i] and pin[i], trail reads, functions"

func forbiddenInfo(c *checker, n ast.Node) (msg, hint string, at file.Idx) {
	if f, ok := forbiddenTable[reflect.TypeOf(n)]; ok {
		return f(c, n)
	}
	return "this syntax is not part of BOTSCRIPT", genericForbiddenHint, 0
}

// hasForbidden reports whether n has a dedicated forbidden entry.
func hasForbidden(n ast.Node) bool {
	_, ok := forbiddenTable[reflect.TypeOf(n)]
	return ok
}
