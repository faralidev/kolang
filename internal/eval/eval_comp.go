package eval

import (
	"github.com/faralidev/kolang/internal/ast"
)

// compRec recursively evaluates the clauses of a comprehension: for each item
// of the current clause's iterable, the loop variable is bound (in the shared
// comp env) and the filter is checked before descending into the next clause.
// At the deepest level emit is called once per produced element. Nested
// clauses re-use the same environment, mirroring Python's comprehension scope.
func (e *Eval) compRec(clauses []*ast.CompClause, idx int, env *Env, emit func() error) error {
	if idx == len(clauses) {
		return emit()
	}
	cl := clauses[idx]
	it, err := e.evalExpr(cl.Iterable, env)
	if err != nil {
		return err
	}
	items, err := iterValues(it)
	if err != nil {
		return &RuntimeError{Line: cl.L, Msg: "مقدار درک‌لیست قابل پیمایش نیست"}
	}
	for _, item := range items {
		env.Set(cl.Name, item)
		if cl.Filter != nil {
			fv, err := e.evalExpr(cl.Filter, env)
			if err != nil {
				return err
			}
			b, err := e.toBool(fv, cl.L)
			if err != nil {
				return err
			}
			if !b {
				continue
			}
		}
		if err := e.compRec(clauses, idx+1, env, emit); err != nil {
			return err
		}
	}
	return nil
}

// evalListComp evaluates a list comprehension into a *List.
func (e *Eval) evalListComp(ex *ast.ListComp, env *Env) (Value, error) {
	compEnv := NewEnv(env)
	out := []Value{}
	err := e.compRec(ex.Clauses, 0, compEnv, func() error {
		ev, err := e.evalExpr(ex.Element, compEnv)
		if err != nil {
			return err
		}
		out = append(out, ev)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &List{Vals: out}, nil
}

// evalDictComp evaluates a dict comprehension into a *Dict, keyed by the
// stringified key expression.
func (e *Eval) evalDictComp(ex *ast.DictComp, env *Env) (Value, error) {
	compEnv := NewEnv(env)
	m := map[string]Value{}
	err := e.compRec(ex.Clauses, 0, compEnv, func() error {
		kv, err := e.evalExpr(ex.Key, compEnv)
		if err != nil {
			return err
		}
		vv, err := e.evalExpr(ex.Value, compEnv)
		if err != nil {
			return err
		}
		m[Stringify(kv)] = vv
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Dict{M: m}, nil
}

// evalSetComp evaluates a set comprehension into a *Set (deduplicated).
func (e *Eval) evalSetComp(ex *ast.SetComp, env *Env) (Value, error) {
	compEnv := NewEnv(env)
	s := newSet()
	err := e.compRec(ex.Clauses, 0, compEnv, func() error {
		ev, err := e.evalExpr(ex.Element, compEnv)
		if err != nil {
			return err
		}
		s.Add(ev)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// evalPipe implements «arg |> target»: the left operand is threaded as the
// first argument to the right-hand callable, which may be a plain name, a
// call (with the value prepended to its explicit args), or a method call.
func (e *Eval) evalPipe(target ast.Expr, arg Value, env *Env) (Value, error) {
	switch t := target.(type) {
	case *ast.Call:
		fn, err := e.evalExpr(t.Fn, env)
		if err != nil {
			return nil, err
		}
		args := []Value{arg}
		for _, a := range t.Args {
			v, err := e.evalExpr(a, env)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		kwargs := map[string]Value{}
		for _, kw := range t.KwArgs {
			v, err := e.evalExpr(kw.Value, env)
			if err != nil {
				return nil, err
			}
			kwargs[kw.Name] = v
		}
		return e.call(fn, args, kwargs, t.L)
	case *ast.MethodCall:
		return e.evalPipeMethodCall(t, arg, env)
	default:
		fn, err := e.evalExpr(target, env)
		if err != nil {
			return nil, err
		}
		return e.call(fn, []Value{arg}, nil, target.Line())
	}
}

// evalPipeMethodCall dispatches a piped method call «arg |> methodِ(args)recv»,
// mirroring evalMethodCall's instance/super/fallback dispatch with the piped
// value as the receiver's first argument.
func (e *Eval) evalPipeMethodCall(ex *ast.MethodCall, arg Value, env *Env) (Value, error) {
	recv, err := e.evalExpr(ex.Receiver, env)
	if err != nil {
		return nil, err
	}
	attr := ""
	if id, ok := ex.Method.(*ast.Ident); ok {
		attr = id.Name
	} else {
		return nil, &RuntimeError{Line: ex.L, Msg: "نام روش باید یک نام باشد"}
	}
	args := []Value{arg}
	for _, a := range ex.Args {
		v, err := e.evalExpr(a, env)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	switch r := recv.(type) {
	case *Instance:
		fn, definingClass := r.Class.lookupMethod(attr)
		if fn == nil {
			return nil, &RuntimeError{Line: ex.L, Msg: "روش «" + attr + "» روی " + Stringify(recv) + " وجود ندارد"}
		}
		e.pushFrame(&Frame{Inst: r, Class: definingClass})
		result, err := e.callFunction(fn, append([]Value{r}, args...), nil, ex.L)
		e.popFrame()
		return result, err
	case *Super:
		fn, definingClass := r.Klass.lookupMethod(attr)
		if fn == nil {
			return nil, &RuntimeError{Line: ex.L, Msg: "روش «" + attr + "» روی والد " + r.Klass.Name + " وجود ندارد"}
		}
		callArgs := args
		if r.Obj != nil {
			callArgs = append([]Value{r.Obj}, args...)
		}
		e.pushFrame(&Frame{Inst: r.Obj, Class: definingClass})
		result, err := e.callFunction(fn, callArgs, nil, ex.L)
		e.popFrame()
		return result, err
	default:
		method, ok := e.getAttr(recv, attr)
		if !ok {
			return nil, &RuntimeError{Line: ex.L, Msg: "روش «" + attr + "» روی " + Stringify(recv) + " وجود ندارد"}
		}
		callArgs := append([]Value{recv}, args...)
		return e.call(method, callArgs, nil, ex.L)
	}
}
