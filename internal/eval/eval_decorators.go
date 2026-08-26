package eval

import (
	"github.com/faralidev/kolang/internal/ast"
)

// elseClause returns the statements of an if-statement's else branch, or nil
// when there is no else. Used while scanning for yields inside conditionals.
func elseClause(st *ast.IfStmt) []ast.Stmt {
	if st.Else != nil {
		return st.Else.Stmts
	}
	return nil
}

// applyDecorators wraps fn with its decorators (پوشش), applied bottom-up:
// the last decorator listed in source runs first, wrapping the function, and
// its result feeds the previous decorator, etc.
func (e *Eval) applyDecorators(fn *Function, decs []*ast.DecoratorStmt, env *Env) (Value, error) {
	cur := Value(fn)
	for i := len(decs) - 1; i >= 0; i-- {
		dec := decs[i]
		decorator, err := e.evalDecorator(dec, env)
		if err != nil {
			return nil, err
		}
		cur, err = e.call(decorator, []Value{cur}, nil, dec.L)
		if err != nil {
			return nil, err
		}
	}
	return cur, nil
}

// evalDecorator resolves a decorator to a callable: a bare «پوشش NAME»
// evaluates the name, while «پوشش NAME(args)» calls NAME(args) to produce the
// actual decorator.
func (e *Eval) evalDecorator(dec *ast.DecoratorStmt, env *Env) (Value, error) {
	if len(dec.Args) > 0 {
		call := &ast.Call{L: dec.L, Fn: &ast.Ident{L: dec.L, Name: dec.Name}, Args: dec.Args}
		return e.evalExpr(call, env)
	}
	return e.evalExpr(&ast.Ident{L: dec.L, Name: dec.Name}, env)
}
