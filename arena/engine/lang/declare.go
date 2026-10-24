package lang

import (
	"fmt"

	"github.com/dop251/goja/ast"
)

// declare registers a top-level function and checks its name and parameters.
func (c *checker) declare(lit *ast.FunctionLiteral) {
	name := lit.Name.Name.String()
	c.checkName(lit)
	params := c.paramNames(lit)
	f := &Func{Name: name, Params: params, Decl: lit}
	f.Line, _ = c.p.Pos(lit.Idx0())
	c.p.Order = append(c.p.Order, f)
	if _, dup := c.p.Funcs[name]; dup {
		c.errAt(lit.Name.Idx, fmt.Sprintf("function '%s' is declared twice", name), "a name may be used only once, but you wrote it twice - rename or remove one")
		return
	}
	c.p.Funcs[name] = f
	c.noteHandler(name, f, lit)
}

func (c *checker) checkName(lit *ast.FunctionLiteral) {
	name := lit.Name.Name.String()
	if kind := reservedKind(name); kind != "" {
		c.errAt(lit.Name.Idx, fmt.Sprintf("'%s' is already a %s name", name, kind),
			fmt.Sprintf("pick another name; '%s' is a %s of the language", name, kind))
	}
	if lit.Async {
		c.errNode(lit, "async functions are not allowed", "declare a plain function")
	}
	if lit.Generator {
		c.errNode(lit, "generator functions are not allowed", "declare a plain function")
	}
}

func (c *checker) noteHandler(name string, f *Func, lit *ast.FunctionLiteral) {
	spec, ok := handlers[name]
	if !ok {
		return
	}
	switch name {
	case "loop":
		c.p.Loop = f
	case "onHit":
		c.p.OnHit = f
	default:
		c.p.OnBump = f
	}
	if list := lit.ParameterList.List; len(list) > spec.maxParams {
		c.errNode(list[spec.maxParams], fmt.Sprintf("'%s' has too many parameters", name),
			fmt.Sprintf("declare it as function %s: %s", spec.sig, spec.desc))
	}
}

func (c *checker) paramNames(lit *ast.FunctionLiteral) []string {
	var names []string
	seen := map[string]bool{}
	for _, b := range lit.ParameterList.List {
		if name, ok := c.param(b, seen); ok {
			names = append(names, name)
		}
	}
	if lit.ParameterList.Rest != nil {
		c.errAt(lit.ParameterList.Opening+1, "rest parameters are not allowed", "parameters are plain names, like f(p, q)")
	}
	return names
}

func (c *checker) param(b *ast.Binding, seen map[string]bool) (string, bool) {
	if b.Initializer != nil {
		c.errNode(b, "default values are not allowed", "parameters are plain names; pass every argument explicitly")
		return "", false
	}
	id, ok := b.Target.(*ast.Identifier)
	if !ok {
		c.errNode(b, "a parameter must be a plain name", "use plain names like f(p, q)")
		return "", false
	}
	name := id.Name.String()
	if msg := paramClash(name, seen); msg != "" {
		c.errNode(b, msg, "pick another name for the parameter ("+msg+")")
		return "", false
	}
	seen[name] = true
	return name, true
}

func paramClash(name string, seen map[string]bool) string {
	if seen[name] {
		return fmt.Sprintf("parameter '%s' is declared twice", name)
	}
	if kind := reservedKind(name); kind != "" {
		return fmt.Sprintf("'%s' is already a %s name", name, kind)
	}
	return ""
}
