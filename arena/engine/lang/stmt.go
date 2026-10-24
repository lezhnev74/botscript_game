package lang

import "github.com/dop251/goja/ast"

func (c *checker) block(n *ast.BlockStatement) {
	for _, s := range n.List {
		c.visit(s)
	}
}

func (c *checker) exprStmt(n *ast.ExpressionStatement) { c.visitStmtExpr(n.Expression) }

// visitStmtExpr visits an expression used as a whole statement (or a for
// init/update): the only place a pin assignment may stand.
func (c *checker) visitStmtExpr(e ast.Expression) {
	c.stmtExpr = e
	c.visit(e)
}

func (c *checker) ifStmt(n *ast.IfStatement) {
	c.visit(n.Test)
	c.visit(n.Consequent)
	c.visit(n.Alternate)
}

func (c *checker) whileStmt(n *ast.WhileStatement) {
	c.visit(n.Test)
	c.visit(n.Body)
}

func (c *checker) forStmt(n *ast.ForStatement) {
	c.visit(n.Initializer)
	c.visit(n.Test)
	c.visitStmtExpr(n.Update)
	c.visit(n.Body)
}

func (c *checker) forInitExpr(n *ast.ForLoopInitializerExpression) { c.visitStmtExpr(n.Expression) }

func (c *checker) returnStmt(n *ast.ReturnStatement) { c.visit(n.Argument) }

func (c *checker) branch(n *ast.BranchStatement) {
	if n.Label != nil {
		c.errNode(n, "labels are not allowed", "use break / continue without a label")
	}
}

func (c *checker) empty(*ast.EmptyStatement) {}
