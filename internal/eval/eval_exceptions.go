package eval

import (
	"github.com/faralidev/kolang/internal/ast"
)

// isExceptionClass reports whether c is one of the built-in exception classes
// (خطا or any of its subclasses). A user-defined class inheriting from an
// exception class is also treated as one (its parent chain reaches an
// ExBuiltin class).
func isExceptionClass(c *Class) bool {
	for k := c; k != nil; k = k.Parent {
		if k.ExBuiltin {
			return true
		}
	}
	return false
}

// raise builds a raised-exception signal for the given exception class. It goes
// through instantiate so the class's constructor (ساخت) runs and .پیام is set
// consistently with the «خطا("msg")» factory path.
func (e *Eval) raise(excClass *Class, msg string, line int) error {
	inst, err := e.instantiate(excClass, []Value{msg}, nil, line)
	if err != nil {
		return err
	}
	return raiseSignal{exc: inst.(*Instance), line: line}
}

// instanceOf reports whether inst is an instance of cls or of a subclass of cls
// (walking the instance's class chain upward). This is the instance-of check
// used for exception matching, honouring inheritance.
func instanceOf(inst *Instance, cls *Class) bool {
	for k := inst.Class; k != nil; k = k.Parent {
		if k == cls {
			return true
		}
	}
	return false
}

// evalTryStmt evaluates a بپا/بگیر/درنهایت block. The finally block (درنهایت)
// always runs, using a Go defer so it fires on normal completion, an uncaught
// exception, a handled exception, or any control-flow signal.
func (e *Eval) evalTryStmt(st *ast.TryStmt, env *Env) (ret Value, retErr error) {
	// Finally always runs; if the try/handlers produced no error and finally
	// raises its own signal, that overrides. If try already failed, finally's
	// signal also overrides (Python-like: finally wins).
	if st.Finally != nil {
		defer func() {
			_, ferr := e.evalBlock(st.Finally.Stmts, env)
			if ferr != nil {
				switch sig := ferr.(type) {
				case returnSignal:
					// A return inside finally wins (Python semantics): the
					// finally value overrides any value or exception produced
					// by the try/handlers. It is propagated as a returnSignal
					// so the enclosing function call unwraps it into the
					// return value (callFunction reads the signal, not the
					// block value).
					ret = sig.v
					retErr = returnSignal{sig.v}
				default:
					// An exception (or break/continue) raised in finally
					// propagates, overriding the previous outcome.
					retErr = ferr
				}
			}
		}()
	}

	v, err := e.evalBlock(st.Body.Stmts, env)
	if err != nil {
		rs, isRaise := err.(raiseSignal)
		if !isRaise {
			// A non-exception error (return/break/continue/other) is not
			// caught; finally still runs via the defer above.
			return nil, err
		}
		// Try to find a matching handler.
		for _, h := range st.Handlers {
			matched, merr := e.matchesException(rs, h, env)
			if merr != nil {
				// Evaluating the exception-type expression itself failed.
				// Propagate that error — it replaces the original exception.
				return nil, merr
			}
			if matched {
				if h.Alias != "" {
					env.Set(h.Alias, rs.exc)
				}
				return e.evalBlock(h.Body.Stmts, env)
			}
		}
		// No handler matched — re-raise (finally still runs via defer).
		return nil, err
	}
	return v, nil
}

// matchesException reports whether a raised exception is caught by an except
// clause. A bare «بگیر» catches everything. The bool result is accompanied by
// an error: if evaluating the exception-type expression itself fails, that
// error is returned (instead of silently treating the handler as a no-match),
// so the failure is visible to the caller.
func (e *Eval) matchesException(rs raiseSignal, h *ast.ExceptHandler, env *Env) (bool, error) {
	inst := rs.exc
	if inst == nil {
		return false, nil
	}
	if h.Exception == nil {
		return true, nil // bare «بگیر:»
	}
	tv, err := e.evalExpr(h.Exception, env)
	if err != nil {
		// The exception-type expression raised. If it's an exception signal,
		// it replaces the current exception (chained). A plain error is
		// wrapped so the failure is reported rather than swallowed.
		if _, isRaise := err.(raiseSignal); isRaise {
			return false, err
		}
		return false, &RuntimeError{Line: h.L, Msg: "خطا در ارزیابی نوع استثنا: " + err.Error()}
	}
	cls, ok := tv.(*Class)
	if !ok {
		return false, &RuntimeError{Line: h.L, Msg: "بگیر به یک کلاس (گونه) نیاز دارد"}
	}
	return instanceOf(inst, cls), nil
}

// evalRaiseStmt implements «X بده»: X must be an exception instance, which is
// thrown as a raiseSignal.
func (e *Eval) evalRaiseStmt(st *ast.RaiseStmt, env *Env) (Value, error) {
	v, err := e.evalExpr(st.Value, env)
	if err != nil {
		return nil, err
	}
	inst, ok := v.(*Instance)
	if !ok {
		return nil, &RuntimeError{Line: st.L, Msg: "بده به یک نمونه استثنا نیاز دارد (از کلاس خطا ساخته‌شده)"}
	}
	return nil, raiseSignal{exc: inst, line: st.L}
}

// evalDeferStmt implements the postfix «call تأخیری». It captures the call
// expression and the current env, and registers it to run (LIFO) when the
// current function returns.
func (e *Eval) evalDeferStmt(st *ast.DeferStmt, env *Env) (Value, error) {
	expr := st.Call
	callEnv := env
	e.addDefers(func() (Value, error) { return e.evalExpr(expr, callEnv) })
	return nil, nil
}

// --- defer stack management ---

// pushDefers opens a fresh defer list for a function call.
func (e *Eval) pushDefers() {
	e.deferStack = append(e.deferStack, nil)
}

// popDefers removes and returns the innermost defer list.
func (e *Eval) popDefers() []func() (Value, error) {
	if len(e.deferStack) == 0 {
		return nil
	}
	d := e.deferStack[len(e.deferStack)-1]
	e.deferStack = e.deferStack[:len(e.deferStack)-1]
	return d
}

// addDefers appends a deferred call to the innermost function's defer list.
func (e *Eval) addDefers(f func() (Value, error)) {
	if len(e.deferStack) == 0 {
		// Defer outside a function: run at the end is not meaningful here, so
		// park it on a scratch entry (never executed).
		e.deferStack = append(e.deferStack, nil)
	}
	top := len(e.deferStack) - 1
	e.deferStack[top] = append(e.deferStack[top], f)
}

// runDefers executes deferred calls in LIFO order. If a deferred call raises an
// exception, that raise is returned immediately (Python finally-wins
// semantics): the first deferred raise (in LIFO execution order) overrides any
// normal return value of the enclosing function. Non-exception errors from
// defers are ignored and the remaining defers still run.
func (e *Eval) runDefers(run []func() (Value, error)) error {
	for i := len(run) - 1; i >= 0; i-- {
		_, err := run[i]()
		if err != nil {
			if _, isRaise := err.(raiseSignal); isRaise {
				return err
			}
		}
	}
	return nil
}

// installExceptions registers the built-in exception class hierarchy in the
// global environment. خطا is both the base exception class and, being
// callable, the «خطا("پیام")» error-value factory. All subclasses inherit
// from it.
func (e *Eval) installExceptions(env *Env) {
	base := &Class{Name: "خطا", Methods: map[string]*Function{}, Env: NewEnv(env), ExBuiltin: true}
	// Give the base exception class a constructor: ساخت(خود, پیام) sets
	// پیامِ خود = پیام. This ensures both the factory path («خطا("msg")» via
	// instantiate) and the raise path («raise(خطا, "msg")» via instantiate)
	// produce instances with a consistent .پیام, and lets user-defined
	// exception subclasses' own ساخت run.
	base.Constructor = &Function{
		Name:   "ساخت",
		Params: []*ast.Param{{Name: "خود"}, {Name: "پیام"}},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.Assign{
				Target: &ast.MemberAccess{Receiver: &ast.Ident{Name: "خود"}, Attr: &ast.Ident{Name: "پیام"}},
				Value:  &ast.Ident{Name: "پیام"},
			},
		}},
		Env: base.Env,
	}
	base.Methods["ساخت"] = base.Constructor
	env.Set("خطا", base)
	// استثنا is the spec's Python-style exception base class. It aliases the
	// same Class object as خطا so «وارث استثنا:» resolves and subclasses are
	// treated as exception classes (the two concepts are deliberately not
	// split yet — that is a larger redesign).
	env.Set("استثنا", base)
	e.excBase = base

	mk := func(name string, parent *Class) *Class {
		c := &Class{Name: name, Parent: parent, Methods: map[string]*Function{}, Env: NewEnv(env), ExBuiltin: true}
		env.Set(name, c)
		return c
	}

	e.excZero = mk("خطای‌صفر", base)
	e.excVal = mk("خطای‌مقدار", base)
	e.excType = mk("خطای‌نوع", base)
	e.excKey = mk("خطای‌کلید", base)
	e.excIdx = mk("خطای‌نمایه", base)
	e.excFile = mk("خطای‌فایل", base)
	e.excStop = mk("توقف‌تکرار", base)
}
