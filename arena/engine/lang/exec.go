package lang

import (
	"reflect"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/token"
)

type flow int

const (
	flowNext flow = iota
	flowBreak
	flowContinue
	flowReturn
)

var stmtRules = map[reflect.Type]func(*Interp, ast.Statement) flow{}

func stmtRule[N ast.Statement](f func(*Interp, N) flow) {
	stmtRules[reflect.TypeFor[N]()] = func(in *Interp, s ast.Statement) flow { return f(in, s.(N)) }
}

func init() {
	stmtRule((*Interp).block)
	stmtRule((*Interp).exprStmt)
	stmtRule((*Interp).ifStmt)
	stmtRule((*Interp).whileStmt)
	stmtRule((*Interp).forStmt)
	stmtRule((*Interp).returnStmt)
	stmtRule((*Interp).branch)
	stmtRule((*Interp).empty)
}

// exec runs one statement (1 op); a nil statement (absent else) is free.
func (in *Interp) exec(s ast.Statement) flow {
	if s == nil {
		return flowNext
	}
	in.tick()
	return stmtRules[reflect.TypeOf(s)](in, s)
}

// execList stops at the first statement that is not flowNext.
func (in *Interp) execList(list []ast.Statement) flow {
	for _, s := range list {
		if f := in.exec(s); f != flowNext {
			return f
		}
	}
	return flowNext
}

func (in *Interp) block(n *ast.BlockStatement) flow { return in.execList(n.List) }

func (in *Interp) exprStmt(n *ast.ExpressionStatement) flow {
	in.eval(n.Expression)
	return flowNext
}

func (in *Interp) ifStmt(n *ast.IfStatement) flow {
	if in.eval(n.Test) != 0 {
		return in.exec(n.Consequent)
	}
	return in.exec(n.Alternate)
}

func (in *Interp) whileStmt(n *ast.WhileStatement) flow { return in.loop(n.Test, nil, n.Body) }

func (in *Interp) forStmt(n *ast.ForStatement) flow {
	if init, ok := n.Initializer.(*ast.ForLoopInitializerExpression); ok {
		in.eval(init.Expression)
	}
	return in.loop(n.Test, n.Update, n.Body)
}

// loop runs test/body/update; continue skips to update, break leaves, and a
// return passes through. Absent test (for(;;)) is true, absent update is free.
func (in *Interp) loop(test, update ast.Expression, body ast.Statement) flow {
	for in.truthy(test) {
		switch in.exec(body) {
		case flowBreak:
			return flowNext
		case flowReturn:
			return flowReturn
		}
		if update != nil {
			in.eval(update)
		}
	}
	return flowNext
}

func (in *Interp) returnStmt(n *ast.ReturnStatement) flow {
	in.ret = 0
	if n.Argument != nil {
		in.ret = in.eval(n.Argument)
	}
	return flowReturn
}

func (in *Interp) branch(n *ast.BranchStatement) flow {
	if n.Token == token.BREAK {
		return flowBreak
	}
	return flowContinue
}

func (in *Interp) empty(*ast.EmptyStatement) flow { return flowNext }
