package eval

import (
	"math"

	"github.com/faralidev/kolang/internal/ast"
)

// evalIf evaluates an اگر statement: the condition, then the matching branch
// (body / first true elif / else). Conditions must already be booleans
// (enforced by the parser and toBool).
func (e *Eval) evalIf(st *ast.IfStmt, env *Env) (Value, error) {
	c, err := e.evalExpr(st.Cond, env)
	if err != nil {
		return nil, err
	}
	b, err := e.toBool(c, st.L)
	if err != nil {
		return nil, err
	}
	if b {
		return e.evalBlock(st.Body, env)
	}
	for _, el := range st.Elifs {
		ec, err := e.evalExpr(el.Cond, env)
		if err != nil {
			return nil, err
		}
		eb, err := e.toBool(ec, el.L)
		if err != nil {
			return nil, err
		}
		if eb {
			return e.evalBlock(el.Body.Stmts, env)
		}
	}
	if st.Else != nil {
		return e.evalBlock(st.Else.Stmts, env)
	}
	return nil, nil
}

// evalWhile evaluates a تاوقتی loop, re-evaluating the condition each
// iteration and intercepting break/continue signals from the body.
func (e *Eval) evalWhile(st *ast.WhileStmt, env *Env) (Value, error) {
	for {
		c, err := e.evalExpr(st.Cond, env)
		if err != nil {
			return nil, err
		}
		b, err := e.toBool(c, st.L)
		if err != nil {
			return nil, err
		}
		if !b {
			break
		}
		_, err = e.evalBlock(st.Body.Stmts, env)
		if err != nil {
			switch err.(type) {
			case breakSignal:
				return nil, nil
			case continueSignal:
				continue
			default:
				return nil, err
			}
		}
	}
	return nil, nil
}

// evalForRange evaluates a numeric range loop «برای VAR از START تا END». The
// bounds must be whole numbers; a zero step raises خطای‌مقدار. Each iteration
// runs in a fresh child env so closures capture their own loop variable.
func (e *Eval) evalForRange(st *ast.ForRange, env *Env) (Value, error) {
	sv, err := e.evalExpr(st.Start, env)
	if err != nil {
		return nil, err
	}
	ev, err := e.evalExpr(st.End, env)
	if err != nil {
		return nil, err
	}
	start, err := toNumber(sv)
	if err != nil {
		return nil, &RuntimeError{Line: st.L, Msg: "حدود بازه باید عدد باشند"}
	}
	// L12: fractional float bounds (e.g. ۰.۵) would be silently truncated;
	// raise خطای‌نوع instead. Whole-number floats like ۳.۰ are fine.
	if f, ok := sv.(float64); ok && f != math.Trunc(f) {
		return nil, e.raise(e.excType, "حدود بازه باید صحیح باشند", st.L)
	}
	end, err := toNumber(ev)
	if err != nil {
		return nil, &RuntimeError{Line: st.L, Msg: "حدود بازه باید عدد باشند"}
	}
	if f, ok := ev.(float64); ok && f != math.Trunc(f) {
		return nil, e.raise(e.excType, "حدود بازه باید صحیح باشند", st.L)
	}
	step := int64(1)
	if st.Step != nil {
		spv, err := e.evalExpr(st.Step, env)
		if err != nil {
			return nil, err
		}
		sp, err := toNumber(spv)
		if err != nil {
			return nil, &RuntimeError{Line: st.L, Msg: "گام باید عدد باشد"}
		}
		step = sp
	}
	// H2: a zero step would loop forever.
	if step == 0 {
		return nil, e.raise(e.excVal, "گام نمی‌تواند صفر باشد", st.L)
	}

	name := ""
	if id, ok := st.Var.(*ast.Ident); ok {
		name = id.Name
	} else {
		return nil, &RuntimeError{Line: st.L, Msg: "متغیر حلقه باید یک نام باشد"}
	}

	for i := start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
		// H9: a fresh child env per iteration so closures created in the body
		// capture their own binding of the loop variable (no late-binding).
		iterEnv := NewEnv(env)
		iterEnv.Set(name, i)
		_, err := e.evalBlock(st.Body.Stmts, iterEnv)
		if err != nil {
			switch err.(type) {
			case breakSignal:
				return nil, nil
			case continueSignal:
				continue
			default:
				return nil, err
			}
		}
	}
	return nil, nil
}

// evalForIn evaluates «برای VARS در ITER: body». Generators, channels, and
// ranges are iterated lazily; all other iterables are materialized via
// iterValues. Each iteration runs in a fresh child env (H9).
func (e *Eval) evalForIn(st *ast.ForIn, env *Env) (Value, error) {
	iv, err := e.evalExpr(st.Iter, env)
	if err != nil {
		return nil, err
	}
	// Generators are pulled lazily (they may be infinite), so they cannot go
	// through iterValues which materializes the whole sequence.
	if gen, ok := iv.(*Generator); ok {
		return e.evalGeneratorForIn(gen, st, env)
	}
	// Channels are consumed lazily, receiving values until the channel is
	// closed (like Go's «for v := range ch»).
	if ch, ok := iv.(*Channel); ok {
		return e.evalChannelForIn(ch, st, env)
	}
	// Ranges are also iterated lazily: a huge بازه() would otherwise be
	// materialized into a slice of the same size via iterValues.
	if r, ok := iv.(*Range); ok {
		return e.evalRangeForIn(r, st, env)
	}
	items, err := iterValues(iv)
	if err != nil {
		return nil, &RuntimeError{Line: st.L, Msg: err.Error()}
	}
	for _, item := range items {
		// H9: fresh child env per iteration (see evalForRange).
		iterEnv := NewEnv(env)
		if err := e.forInSetVars(st, iterEnv, item); err != nil {
			return nil, err
		}
		_, err := e.evalBlock(st.Body.Stmts, iterEnv)
		if err != nil {
			switch err.(type) {
			case breakSignal:
				return nil, nil
			case continueSignal:
				continue
			default:
				return nil, err
			}
		}
	}
	return nil, nil
}

// evalRangeForIn iterates a بازه() value element by element, without
// materializing it into a slice (H3). Each iteration gets a fresh child env so
// closures capture their own loop-variable binding (H9).
func (e *Eval) evalRangeForIn(r *Range, st *ast.ForIn, env *Env) (Value, error) {
	for i := r.start; (r.step > 0 && i < r.end) || (r.step < 0 && i > r.end); i += r.step {
		iterEnv := NewEnv(env)
		if err := e.forInSetVars(st, iterEnv, i); err != nil {
			return nil, err
		}
		_, err := e.evalBlock(st.Body.Stmts, iterEnv)
		if err != nil {
			switch err.(type) {
			case breakSignal:
				return nil, nil
			case continueSignal:
				continue
			default:
				return nil, err
			}
		}
	}
	return nil, nil
}

// evalGeneratorForIn iterates a generator, pulling values one at a time until
// it is exhausted or raises.
func (e *Eval) evalGeneratorForIn(gen *Generator, st *ast.ForIn, env *Env) (Value, error) {
	for {
		v, ok, err := gen.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		// H9: fresh child env per iteration (see evalForRange).
		iterEnv := NewEnv(env)
		if err := e.forInSetVars(st, iterEnv, v); err != nil {
			return nil, err
		}
		_, err = e.evalBlock(st.Body.Stmts, iterEnv)
		if err != nil {
			switch err.(type) {
			case breakSignal:
				gen.close() // caller stopped early: unblock the generator
				return nil, nil
			case continueSignal:
				continue
			default:
				gen.close() // exception in loop body: unblock the generator
				return nil, err
			}
		}
	}
}

// forInSetVars binds one iterated item to the loop variable(s) of a برای loop,
// unpacking tuples/lists when there are multiple variables.
func (e *Eval) forInSetVars(st *ast.ForIn, env *Env, item Value) error {
	if len(st.Vars) == 1 {
		env.Set(st.Vars[0].(*ast.Ident).Name, item)
		return nil
	}
	var tup *Tuple
	switch t := item.(type) {
	case *Tuple:
		tup = t
	case *List:
		tup = &Tuple{Vals: t.Vals}
	}
	if tup != nil {
		for i, v := range st.Vars {
			if i < len(tup.Vals) {
				env.Set(v.(*ast.Ident).Name, tup.Vals[i])
			}
		}
	}
	return nil
}

// evalWithStmt implements «با EXPR بانام NAME: body». The context expression
// is evaluated, NAME is bound to its value in a fresh child scope, and the
// body runs there. On exit — whether the body completes or raises — the
// context is closed if it exposes a closeable interface (a «ببند» method).
// A dedicated File type does not exist yet, so closeable detection is by the
// presence of a «ببند» attribute; otherwise the context is just bound and run.
func (e *Eval) evalWithStmt(st *ast.WithStmt, env *Env) (Value, error) {
	ctx, err := e.evalExpr(st.Context, env)
	if err != nil {
		return nil, err
	}
	withEnv := NewEnv(env)
	withEnv.Set(st.Name, ctx)

	closeCtx := func() {
		closer, ok := e.getAttr(ctx, "ببند")
		if !ok {
			return
		}
		// Best-effort cleanup: a close failure must not mask the body's result.
		_, _ = e.call(closer, []Value{ctx}, nil, st.L)
	}

	v, err := e.evalBlock(st.Body, withEnv)
	closeCtx()
	return v, err
}

// toBool converts a value to a boolean for condition contexts. Kolang forbids
// implicit truthiness: only actual booleans are accepted, anything else raises
// خطای‌نوع.
func (e *Eval) toBool(v Value, line int) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, e.raise(e.excType, "شرط باید بولی باشد (نه مقدار ضمنی)", line)
	}
	return b, nil
}
