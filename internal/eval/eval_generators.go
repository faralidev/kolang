package eval

import (
	"fmt"
	"sync"

	"github.com/faralidev/kolang/internal/ast"
)

// genMu guards the Generator.started/finished flag transitions so next() is
// race-free even if two goroutines (incorrectly) iterate the same generator.
// Kolang generators are single-consumer: iterating one from multiple goroutines
// concurrently is undefined behavior, but it must not be a data race and must
// not double-start the run goroutine. close() takes the same lock so a
// concurrent close (e.g. inner generator released by a yield-from while a
// break/exception unwinds) cannot double-close done.
var genMu sync.Mutex

// next pulls the next value from the generator, starting its run goroutine on
// the first call and blocking until the generator yields. It returns
// (value, true) on a yield, (nil, false) once the generator is exhausted, and
// an error if the generator body raised one.
func (g *Generator) next() (Value, bool, error) {
	genMu.Lock()
	if g.finished {
		genMu.Unlock()
		return nil, false, g.err
	}
	if !g.started {
		g.started = true
		go g.run()
	}
	genMu.Unlock()
	v, ok := <-g.ch
	if !ok {
		genMu.Lock()
		g.finished = true
		genMu.Unlock()
		return nil, false, g.err
	}
	// Rendezvous complete: let the generator continue toward its next yield
	// (or finish) before this call returns.
	g.resume <- struct{}{}
	return v, true, nil
}

// run executes the generator body on its private goroutine. It binds
// parameters, pushes a defer frame, evaluates the body (yielding via بساز),
// then runs deferred calls. Control-flow signals from the body are swallowed;
// real errors are stored in g.err and surfaced by next. Lazy generator
// expressions use genExpRunner instead of a Function body.
func (g *Generator) run() {
	gen := g.genEval
	defer close(g.ch)
	defer func() {
		if r := recover(); r != nil {
			if g.err == nil {
				g.err = fmt.Errorf("خطای داخلی جنریتور: %v", r)
			}
		}
	}()

	// Lazy generator expressions use a custom runner instead of a Function body.
	if g.genExpRunner != nil {
		gen.currentGen = g
		err := g.genExpRunner(g)
		if err != nil {
			switch err.(type) {
			case returnSignal, breakSignal, continueSignal:
			default:
				if g.err == nil {
					g.err = err
				}
			}
		}
		return
	}

	newEnv := NewEnv(g.Fn.Env)
	if err := gen.bindParams(g.Fn, g.Args, g.Kwargs, newEnv, g.Line); err != nil {
		g.err = err
		return
	}
	gen.currentGen = g
	gen.pushDefers()
	_, err := gen.evalBlock(g.Fn.Body.Stmts, newEnv)
	run := gen.popDefers()
	if derr := gen.runDefers(run); derr != nil && g.err == nil {
		g.err = derr
	}
	if err != nil {
		switch err.(type) {
		case returnSignal, breakSignal, continueSignal:
		default:
			if g.err == nil {
				g.err = err
			}
		}
	}
}

// evalYieldStmt implements «expr بساز» — yield expr to the caller. It is only
// valid while a generator body is executing on this evaluator.
func (e *Eval) evalYieldStmt(st *ast.YieldStmt, env *Env) (Value, error) {
	gen := e.currentGen
	if gen == nil {
		return nil, &RuntimeError{Line: st.L, Msg: "بساز فقط داخل یک جنریتور مجاز است (تابعی که بساز دارد)"}
	}
	v, err := e.evalExpr(st.Value, env)
	if err != nil {
		return nil, err
	}
	return e.yieldValue(v, gen)
}

// evalYieldFromStmt implements «expr بساز‌از» — delegate iteration to an inner
// iterable, yielding each of its values.
func (e *Eval) evalYieldFromStmt(st *ast.YieldFromStmt, env *Env) (Value, error) {
	gen := e.currentGen
	if gen == nil {
		return nil, &RuntimeError{Line: st.L, Msg: "بساز‌از فقط داخل یک جنریتور مجاز است (تابعی که بساز دارد)"}
	}
	inner, err := e.evalExpr(st.Value, env)
	if err != nil {
		return nil, err
	}
	return e.yieldFrom(inner, gen)
}

// yieldFrom yields every value of inner to gen. When inner is itself a
// generator it is pulled lazily and always released (via close) when this
// yield-from exits, so its run goroutine cannot leak.
func (e *Eval) yieldFrom(inner Value, gen *Generator) (Value, error) {
	if ig, ok := inner.(*Generator); ok {
		// Always release the inner generator when this yield-from exits —
		// normally or via an exception/abandon — so its run goroutine is
		// unblocked (via done) and cannot leak.
		defer ig.close()
		for {
			v, ok, err := ig.next()
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, nil
			}
			if _, err := e.yieldValue(v, gen); err != nil {
				return nil, err
			}
		}
	}
	items, err := iterValues(inner)
	if err != nil {
		return nil, &RuntimeError{Line: gen.Line, Msg: "بساز‌از به یک دنباله‌پذیر نیاز دارد"}
	}
	for _, v := range items {
		if _, err := e.yieldValue(v, gen); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// yieldValue rendezvous-sends v to the generator's channel, then waits for the
// consumer's acknowledgement. Both steps select on done so a close() unblocks
// a paused generator instead of leaking its goroutine forever.
func (e *Eval) yieldValue(v Value, gen *Generator) (Value, error) {
	select {
	case gen.ch <- v:
	case <-gen.done:
		return nil, returnSignal{nil}
	}
	// Wait for the consumer to acknowledge the yield. Also select on done so a
	// close() (consumer broke / yield-from released us) unblocks this generator
	// instead of leaking its goroutine forever.
	select {
	case <-gen.resume:
		return nil, nil
	case <-gen.done:
		return nil, returnSignal{nil}
	}
}

// close aborts a blocked generator by closing its done channel (idempotent).
// It is called when a consumer stops early (break) or an exception unwinds.
func (g *Generator) close() {
	genMu.Lock()
	defer genMu.Unlock()
	select {
	case <-g.done:
	default:
		close(g.done)
	}
}

// containsYield reports whether a statement list contains a بساز / بساز‌از
// yield, recursing into nested blocks (if/while/for/try). It determines
// whether a function definition is a generator function.
func containsYield(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		switch st := s.(type) {
		case *ast.YieldStmt, *ast.YieldFromStmt:
			return true
		case *ast.IfStmt:
			if containsYield(st.Body) || containsYield(elseClause(st)) {
				return true
			}
			for _, el := range st.Elifs {
				if containsYield(el.Body.Stmts) {
					return true
				}
			}
		case *ast.WhileStmt:
			if containsYield(st.Body.Stmts) {
				return true
			}
		case *ast.ForRange:
			if containsYield(st.Body.Stmts) {
				return true
			}
		case *ast.ForIn:
			if containsYield(st.Body.Stmts) {
				return true
			}
		case *ast.TryStmt:
			if containsYield(st.Body.Stmts) {
				return true
			}
			for _, h := range st.Handlers {
				if containsYield(h.Body.Stmts) {
					return true
				}
			}
			if st.Finally != nil && containsYield(st.Finally.Stmts) {
				return true
			}
		}
	}
	return false
}
