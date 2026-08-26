// Package eval implements a tree-walking evaluator for Kolang.
//
// The evaluator walks the AST produced by the parser. Runtime values implement
// the Value interface; control flow (return/break/continue/raise) is
// propagated through Go errors (returnSignal, breakSignal, continueSignal,
// raiseSignal). Environments form a scope chain with closure semantics; OOP
// is class-based with single inheritance; generators run in private
// goroutines and communicate with the caller through rendezvous channels.
package eval

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/faralidev/kolang/internal/ast"
)

// Value is any runtime value. The concrete types are the Go primitives
// (int64, float64, bool, string, nil for تهی) and the composite types defined
// below (*List, *Tuple, *Dict, *Set, *Range, *Function, *Builtin, *Module,
// *Class, *Instance, *Interface, *Super, *Generator, *Channel).
type Value interface{}

// --- Composite value types ---

// List is a mutable ordered collection (فهرست).
type List struct{ Vals []Value }

// Tuple is an immutable ordered collection (قفسه).
type Tuple struct{ Vals []Value }

// Dict is a key/value map (گنجه). Keys are stored by their string form
// (Stringify), so all keys must stringify to unique values.
type Dict struct{ M map[string]Value }

// Function is the user-defined callable value: a تعریف definition bound to its
// defining environment.
type Function struct {
	Name   string
	Params []*ast.Param
	Body   *ast.Block
	Env    *Env
	// IsGenerator marks a function whose body contains a بساز / بساز‌از
	// statement. Calling it returns a *Generator instead of running the body.
	IsGenerator bool
	// RetType is the optional return type annotation, checked on return
	// (gradual typing). Empty means unannotated (dynamic).
	RetType string
}

// Generator is a lazily-evaluated generator produced by calling a generator
// function. It runs in its own goroutine, pausing at each «بساز» (yield) and
// sending the value to the caller. Iterating the generator (via «برای ... در
// gen» or yield-from) pulls values one at a time. When the body completes or
// returns, the generator is exhausted.
//
// Concurrency caveat (v0.4): each generator runs in its own goroutine using a
// private *Eval so frames and defer stacks do not race with the caller. The
// enclosing lexical environment and the output writer are still shared, which
// is the accepted "concurrency semantics leak" for this phase. If a caller
// stops consuming (e.g. breaks a for loop) mid-yield, the paused goroutine is
// left blocked until the program exits.
type Generator struct {
	Fn      *Function
	Args    []Value
	Kwargs  map[string]Value
	Line    int
	genEval *Eval
	// genExpRunner, when non-nil, replaces the default Fn-based run loop.
	// It's used by lazy generator expressions (GenExp) to iterate comprehension
	// clauses and yield elements without synthesizing a Function body.
	genExpRunner func(gen *Generator) error
	ch           chan Value
	resume       chan struct{}
	done         chan struct{} // closed by close() to abort a blocked generator
	started      bool
	finished     bool
	err          error
}

func (g *Generator) String() string {
	if g.Fn != nil {
		return "<جنریتور " + g.Fn.Name + ">"
	}
	return "<جنریتور>"
}

// Builtin is a native function implemented in Go (بنویس, طول, بازه, ...).
type Builtin struct {
	Name string
	Fn   func(args []Value) (Value, error)
}

func (b *Builtin) String() string { return "<builtin " + b.Name + ">" }

// Module wraps a standard-library module.
type Module struct {
	Name    string
	Members map[string]Value
}

func (m *Module) String() string { return "<module " + m.Name + ">" }

// Function is the user-defined callable value.
func (f *Function) String() string { return "<function " + f.Name + ">" }

// --- Environment ---

// Env is a lexical scope. Scopes form a parent chain: Get walks up to find a
// binding, Set binds locally, and Assign walks up to update an existing
// binding (or defines one in the current scope). Each Env carries its own
// mutex so scopes can be shared safely between a goroutine/generator and the
// caller. The types map records declared type annotations for gradual typing;
// globals/nonlocs record جهانی / نامحلی declarations.
type Env struct {
	mu       sync.RWMutex
	store    map[string]Value
	types    map[string]string // declared type annotation per variable name
	parent   *Env
	globals  map[string]bool // names declared جهانی in this scope
	nonlocs  map[string]bool // names declared نامحلی in this scope
}

// NewEnv creates a fresh scope whose parent is the given enclosing scope
// (nil for the module-level root scope).
func NewEnv(parent *Env) *Env {
	return &Env{store: map[string]Value{}, types: map[string]string{}, parent: parent}
}

// Get looks up a variable by name, walking up the parent chain. ok is false
// if the name is not bound in any scope.
func (e *Env) Get(name string) (Value, bool) {
	for env := e; env != nil; env = env.parent {
		env.mu.RLock()
		v, ok := env.store[name]
		env.mu.RUnlock()
		if ok {
			return v, true
		}
	}
	return nil, false
}

// Set binds (or rebinds) a name in this scope only.
func (e *Env) Set(name string, v Value) {
	e.mu.Lock()
	e.store[name] = v
	e.mu.Unlock()
}

// Assign walks up the parent chain to find an existing binding and updates it
// in place. If the name doesn't exist anywhere in the scope chain, it's
// defined in the current scope (Python implicit-local behavior). This makes
// closures mutate their captured variables (spec §5.x closure semantics),
// rather than silently shadowing them. Loops and function params still use Set
// (local binding) — Assign is only for plain assignment `x = v`.
//
// This is a lenient model (closer to Lua/JavaScript than Python's `nonlocal`):
// mutations walk up by default, no declaration needed. جهانی (global) and
// نامحلی (nonlocal) keywords are supported for explicit control.
func (e *Env) Assign(name string, v Value) {
	// جهانی (global): always bind in the module (root) scope.
	if e.globals != nil && e.globals[name] {
		root := e
		for root.parent != nil {
			root = root.parent
		}
		root.Set(name, v)
		return
	}
	// نامحلی (nonlocal): bind in the nearest enclosing scope that already
	// has the name (skipping the current scope). If none has it yet, define
	// it in the immediate parent (so the first assignment creates the binding
	// the declaration refers to).
	if e.nonlocs != nil && e.nonlocs[name] {
		for env := e.parent; env != nil; env = env.parent {
			env.mu.Lock()
			if _, ok := env.store[name]; ok {
				env.store[name] = v
				env.mu.Unlock()
				return
			}
			env.mu.Unlock()
		}
		// Not yet defined in an enclosing scope: create it in the parent.
		if e.parent != nil {
			e.parent.Set(name, v)
		}
		return
	}
	for env := e; env != nil; env = env.parent {
		env.mu.Lock()
		if _, ok := env.store[name]; ok {
			env.store[name] = v
			env.mu.Unlock()
			return
		}
		env.mu.Unlock()
	}
	// Not found anywhere: define in current scope.
	e.Set(name, v)
}

// DeclareGlobal marks a name as جهانی (global) in this scope: subsequent
// assignments to it bind in the module (root) scope.
func (e *Env) DeclareGlobal(name string) {
	if e.globals == nil {
		e.globals = map[string]bool{}
	}
	e.globals[name] = true
}

// DeclareNonlocal marks a name as نامحلی (nonlocal) in this scope: subsequent
// assignments to it bind in the nearest enclosing scope that has the name.
func (e *Env) DeclareNonlocal(name string) {
	if e.nonlocs == nil {
		e.nonlocs = map[string]bool{}
	}
	e.nonlocs[name] = true
}

// SetType records the declared type annotation for a variable name. It is used
// to re-check later un-annotated assignments against the variable's declared
// type (spec §5.5: annotated variables are runtime-checked on every assignment).
func (e *Env) SetType(name, typeStr string) {
	e.mu.Lock()
	e.types[name] = typeStr
	e.mu.Unlock()
}

// GetType returns the recorded declared type for a variable, walking the parent
// chain. ok is false if the variable has no recorded annotation.
func (e *Env) GetType(name string) (string, bool) {
	for env := e; env != nil; env = env.parent {
		env.mu.RLock()
		t, ok := env.types[name]
		env.mu.RUnlock()
		if ok {
			return t, true
		}
	}
	return "", false
}

// --- Errors & control-flow signals ---
//
// RuntimeError is a catchable evaluation error reported to the user. The
// signal types below are Go errors used purely for control flow; they are
// never reported to the user, only intercepted by the evaluator.
//
// returnSignal propagates a «برگردان» out of a function body.
// breakSignal propagates an «اتمام» out of the innermost loop.
// continueSignal propagates a «بروبعدی» to the innermost loop.
// raiseSignal carries an in-flight exception instance.

// RuntimeError is a catchable evaluation error with a source line. It is
// either raised from the evaluator itself or converted from an uncaught
// raiseSignal at module scope.
type RuntimeError struct {
	Line int
	Msg  string
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("خطای اجرا در خط %d: %s", e.Line, e.Msg)
}

// returnSignal is the control-flow signal for «برگردان», carrying the return
// value (or nil for a bare return).
type returnSignal struct{ v Value }

func (returnSignal) Error() string { return "return" }

// breakSignal is the control-flow signal for «اتمام» (loop break).
type breakSignal struct{}

func (breakSignal) Error() string { return "break" }

// continueSignal is the control-flow signal for «بروبعدی» (loop continue).
type continueSignal struct{}

func (continueSignal) Error() string { return "continue" }

// raiseSignal is an exception in flight: a raised exception instance being
// propagated up until a «بپا» handler catches it (or it escapes uncaught).
type raiseSignal struct {
	exc  *Instance
	line int
}

func (raiseSignal) Error() string { return "raise" }

// --- Evaluator ---

// Eval is the tree-walking evaluator for Kolang programs. It holds the output
// writer, the frame stack (for خود / والد resolution), the deferred-call
// stack, the pre-registered exception classes, and the WaitGroup joining برو
// goroutines. Each call to EvalProgram evaluates a complete program;
// EvalReplExpr/EvalReplStmts evaluate single inputs in a shared REPL
// environment.
type Eval struct {
	out    io.Writer
	frames []*Frame

	// outMu serializes writes to out, since a generator goroutine and the
	// caller may both print to the same underlying writer.
	outMu *sync.Mutex

	// wg tracks spawned برو (goroutine) tasks. EvalProgram waits for them
	// before returning, so spawned goroutines complete before the program
	// exits (mirroring Go's main-returns-when-done, but here we explicitly
	// join so the interpreter's top-level run is deterministic). It is a
	// pointer so goroutine clones share the same WaitGroup: a برو goroutine
	// that itself spawns further برو goroutines is joined transitively.
	wg *sync.WaitGroup

	// currentGen is the generator currently executing on THIS evaluator
	// instance. It is set only on a generator's private eval while its body
	// runs, so بساز statements can rendezvous with the caller. It is nil for
	// ordinary (non-generator) evals.
	currentGen *Generator

	// deferStack holds the pending deferred calls for each active function
	// call. A fresh slice is pushed on function entry and popped (then run in
	// reverse) on exit — even when the function exits via return, raise, or
	// other control-flow signal. Each defer returns (Value, error) so a defer
	// that raises can propagate its exception.
	deferStack [][]func() (Value, error)

	// Pre-registered exception classes (see eval_exceptions.go).
	excBase *Class // خطا
	excZero *Class // خطای‌صفر
	excVal  *Class // خطای‌مقدار
	excType *Class // خطای‌نوع
	excKey  *Class // خطای‌کلید
	excIdx  *Class // خطای‌نمایه
	excFile *Class // خطای‌فایل
	excStop *Class // توقف‌تکرار (StopIteration)
}

// New creates an evaluator that writes program output to the given writer.
func New(out io.Writer) *Eval { return &Eval{out: out, outMu: &sync.Mutex{}, wg: &sync.WaitGroup{}} }

// EvalProgram evaluates a list of top-level statements.
func (e *Eval) EvalProgram(stmts []ast.Stmt) error {
	global := NewEnv(nil)
	e.installBuiltins(global)
	_, err := e.evalTopLevel(stmts, global)
	// Join any spawned برو goroutines so they finish before the program
	// returns (their output is still captured). A goroutine blocked forever on
	// a channel with no counterpart will hang here — the program is required
	// to eventually synchronize/close its channels.
	e.wg.Wait()
	if err != nil {
		if rs, ok := err.(raiseSignal); ok {
			return &RuntimeError{Line: rs.line, Msg: "استثنای مدیریت‌نشده: " + Stringify(rs.exc)}
		}
		// A break/continue/return signal that reaches module scope from inside
		// nested control flow (e.g. a break inside an اگر that is not in a
		// loop) terminates the rest of the module, preserving legacy behavior.
		if _, isCtrl := err.(returnSignal); isCtrl {
			return nil
		}
		if _, isCtrl := err.(breakSignal); isCtrl {
			return nil
		}
		if _, isCtrl := err.(continueSignal); isCtrl {
			return nil
		}
		return err
	}
	return nil
}

// evalTopLevel evaluates statements at module (root) scope. L9: a bare
// break/continue/return statement directly at this level is an error, not a
// silent no-op. Signals arising from nested control flow are left to the
// caller (EvalProgram / the REPL), which terminates the block silently.
func (e *Eval) evalTopLevel(stmts []ast.Stmt, env *Env) (Value, error) {
	var last Value
	for _, s := range stmts {
		switch s.(type) {
		case *ast.BreakStmt:
			return nil, &RuntimeError{Line: s.Line(), Msg: "اتمام در سطح بالا مجاز نیست"}
		case *ast.ContinueStmt:
			return nil, &RuntimeError{Line: s.Line(), Msg: "بروبعدی در سطح بالا مجاز نیست"}
		case *ast.ReturnStmt:
			return nil, &RuntimeError{Line: s.Line(), Msg: "برگردان در سطح بالا مجاز نیست"}
		}
		v, err := e.evalStmt(s, env)
		if err != nil {
			return v, err
		}
		last = v
	}
	return last, nil
}

// evalBlock evaluates a block, propagating control-flow signals as errors.
func (e *Eval) evalBlock(stmts []ast.Stmt, env *Env) (Value, error) {
	var last Value
	for _, s := range stmts {
		v, err := e.evalStmt(s, env)
		if err != nil {
			return v, err
		}
		last = v
	}
	return last, nil
}

// GlobalEnv returns a fresh root (global) environment with the builtins and
// the math module installed. The REPL creates one and reuses it across lines
// so definitions and assignments persist between inputs.
func (e *Eval) GlobalEnv() *Env {
	global := NewEnv(nil)
	e.installBuiltins(global)
	global.Set("ریاضی", mathModule()) // preload math in the REPL
	return global
}

// EvalReplExpr evaluates a single expression in the given environment (the
// REPL's shared global env), with builtins available.
func (e *Eval) EvalReplExpr(expr ast.Expr, env *Env) (Value, error) {
	return e.evalExpr(expr, env)
}

// EvalReplStmts evaluates statements in the given environment (the REPL's
// shared global env), returning the value of the final statement if any.
// Bare top-level break/continue/return are errors (via evalTopLevel); signals
// escaping from nested control flow terminate the rest of the input silently,
// mirroring EvalProgram.
func (e *Eval) EvalReplStmts(stmts []ast.Stmt, env *Env) (Value, error) {
	v, err := e.evalTopLevel(stmts, env)
	if err != nil {
		switch err.(type) {
		case raiseSignal:
			rs := err.(raiseSignal)
			return nil, &RuntimeError{Line: rs.line, Msg: "استثنای مدیریت‌نشده: " + Stringify(rs.exc)}
		case returnSignal, breakSignal, continueSignal:
			// Nested control-flow signal at module scope: stop the rest of
			// this input without an error.
			return nil, nil
		}
	}
	return v, err
}

// evalStmt dispatches on the concrete AST statement type to the appropriate
// evaluator. It is the single entry point for evaluating one statement in any
// scope.
func (e *Eval) evalStmt(s ast.Stmt, env *Env) (Value, error) {
	switch st := s.(type) {
	case *ast.Block:
		return e.evalBlock(st.Stmts, env)
	case *ast.ExprStmt:
		return e.evalExpr(st.Expr, env)
	case *ast.Assign:
		v, err := e.evalExpr(st.Value, env)
		if err != nil {
			return nil, err
		}
		name := ""
		if id, ok := st.Target.(*ast.Ident); ok {
			name = id.Name
		}
		if st.Ann != "" {
			if err := e.checkType(v, st.Ann, env, st.L); err != nil {
				return nil, err
			}
			if name != "" {
				env.SetType(name, st.Ann) // record the declared type
			}
		} else if name != "" {
			// Re-assignment without annotation: enforce the declared type if one
			// was recorded at the (first) annotated assignment.
			if prev, ok := env.GetType(name); ok {
				if err := e.checkType(v, prev, env, st.L); err != nil {
					return nil, err
				}
			}
		}
		return nil, e.assign(st.Target, v, env)
	case *ast.MultiAssign:
		vals := make([]Value, 0, len(st.Values))
		for _, v := range st.Values {
			val, err := e.evalExpr(v, env)
			if err != nil {
				return nil, err
			}
			vals = append(vals, val)
		}
		// unpack a single tuple result: نتیجه و خطا = کاری()
		if len(vals) == 1 && len(st.Targets) > 1 {
			if tup, ok := vals[0].(*Tuple); ok {
				for i, tgt := range st.Targets {
					if i < len(tup.Vals) {
						if err := e.assign(tgt, tup.Vals[i], env); err != nil {
							return nil, err
						}
					}
				}
				return nil, nil
			}
		}
		if len(st.Targets) == 1 && len(vals) > 1 {
			// A single target receiving multiple values becomes a قفسه (tuple),
			// e.g. `نتیجه = کاری()` when کاری returns two values.
			return nil, e.assign(st.Targets[0], &Tuple{Vals: vals}, env)
		}
		for i, tgt := range st.Targets {
			if i < len(vals) {
				if err := e.assign(tgt, vals[i], env); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	case *ast.CompoundAssign:
		cur, err := e.evalExpr(st.Target, env)
		if err != nil {
			return nil, err
		}
		rv, err := e.evalExpr(st.Value, env)
		if err != nil {
			return nil, err
		}
		op := strings.TrimSuffix(st.Op, "=") // += -> +
		nv, err := e.applyBinop(op, cur, rv, st.L)
		if err != nil {
			return nil, err
		}
		return nil, e.assign(st.Target, nv, env)
	case *ast.PrintStmt:
		var parts []string
		for _, a := range st.Args {
			v, err := e.evalExpr(a, env)
			if err != nil {
				return nil, err
			}
			parts = append(parts, Stringify(v))
		}
		e.writeOut(strings.Join(parts, " "))
		return nil, nil
	case *ast.InputStmt:
		// read a line from stdin
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		return nil, e.assign(st.Target, line, env)
	case *ast.ReturnStmt:
		if len(st.Vals) == 0 {
			return nil, returnSignal{nil}
		}
		if len(st.Vals) == 1 {
			v, err := e.evalExpr(st.Vals[0], env)
			if err != nil {
				return nil, err
			}
			return nil, returnSignal{v}
		}
		vals := make([]Value, 0, len(st.Vals))
		for _, v := range st.Vals {
			val, err := e.evalExpr(v, env)
			if err != nil {
				return nil, err
			}
			vals = append(vals, val)
		}
		return nil, returnSignal{&Tuple{Vals: vals}}
	case *ast.BreakStmt:
		return nil, breakSignal{}
	case *ast.ContinueStmt:
		return nil, continueSignal{}
	case *ast.GoStmt:
		return e.evalGoStmt(st, env)
	case *ast.GlobalStmt:
		for _, name := range st.Names {
			env.DeclareGlobal(name)
		}
		return nil, nil
	case *ast.NonlocalStmt:
		for _, name := range st.Names {
			env.DeclareNonlocal(name)
		}
		return nil, nil
	case *ast.SendStmt:
		return e.evalSendStmt(st, env)
	case *ast.CloseStmt:
		return e.evalCloseStmt(st, env)
	case *ast.DefStmt:
		fn := &Function{Name: st.Name, Params: st.Params, Body: st.Body, Env: env, RetType: st.RetType}
		fn.IsGenerator = containsYield(st.Body.Stmts)
		if len(st.Decorators) > 0 {
			v, err := e.applyDecorators(fn, st.Decorators, env)
			if err != nil {
				return nil, err
			}
			env.Set(st.Name, v)
			return v, nil
		}
		env.Set(st.Name, fn)
		return fn, nil
	case *ast.YieldStmt:
		return e.evalYieldStmt(st, env)
	case *ast.YieldFromStmt:
		return e.evalYieldFromStmt(st, env)
	case *ast.ClassDef:
		return e.evalClassDef(st, env)
	case *ast.InterfaceDef:
		return e.evalInterfaceDef(st, env)
	case *ast.ImportStmt:
		return nil, e.importModule(st.Module, env)
	case *ast.TryStmt:
		return e.evalTryStmt(st, env)
	case *ast.RaiseStmt:
		return e.evalRaiseStmt(st, env)
	case *ast.DeferStmt:
		return e.evalDeferStmt(st, env)
	case *ast.FromImportStmt:
		alias := st.Alias
		if alias == "" {
			alias = st.Name
		}
		return nil, e.importFrom(st.Module, st.Name, alias, env)
	case *ast.IfStmt:
		return e.evalIf(st, env)
	case *ast.WhileStmt:
		return e.evalWhile(st, env)
	case *ast.ForRange:
		return e.evalForRange(st, env)
	case *ast.ForIn:
		return e.evalForIn(st, env)
	case *ast.WithStmt:
		return e.evalWithStmt(st, env)
	case *ast.AppendStmt:
		listV, err := e.evalExpr(st.List, env)
		if err != nil {
			return nil, err
		}
		val, err := e.evalExpr(st.Value, env)
		if err != nil {
			return nil, err
		}
		lst, ok := listV.(*List)
		if !ok {
			return nil, &RuntimeError{Line: st.L, Msg: "بیافزا به فهرست نیاز دارد"}
		}
		lst.Vals = append(lst.Vals, val)
		return nil, nil
	case *ast.RemoveStmt:
		listV, err := e.evalExpr(st.List, env)
		if err != nil {
			return nil, err
		}
		val, err := e.evalExpr(st.Value, env)
		if err != nil {
			return nil, err
		}
		lst, ok := listV.(*List)
		if !ok {
			return nil, &RuntimeError{Line: st.L, Msg: "حذف‌کن به فهرست نیاز دارد"}
		}
		for i, v := range lst.Vals {
			if equal(v, val) {
				lst.Vals = append(lst.Vals[:i], lst.Vals[i+1:]...)
				break
			}
		}
		return nil, nil
	default:
		return nil, &RuntimeError{Line: s.Line(), Msg: "دستور ناشناخته"}
	}
}

// --- Expressions ---

// writeOut prints a line to the output writer, serializing concurrent writes
// from the caller and generator goroutines.
func (e *Eval) writeOut(s string) {
	e.outMu.Lock()
	defer e.outMu.Unlock()
	fmt.Fprintln(e.out, s)
}

// newGeneratorEval builds a private *Eval for a generator goroutine. It shares
// the (read-only) output writer and the pre-registered exception classes with
// the caller, but starts with its own frames and defer stacks so the two don't
// race on that mutable state.
func (e *Eval) newGeneratorEval() *Eval {
	return e.cloneEval()
}

// newGoroutineEval builds a private *Eval for a برو (goroutine) task. Like the
// generator eval, it shares the output writer and exception classes but gets
// fresh frames, defer stacks and generator context, so it never races with the
// caller on that mutable state.
func (e *Eval) newGoroutineEval() *Eval {
	return e.cloneEval()
}

// cloneEval returns a sibling *Eval that shares the immutable/read-only parts
// (output writer + its mutex, pre-registered exception class pointers) with e
// but starts with its own frames, defer stack and currentGen. It is safe to
// use on its own goroutine.
func (e *Eval) cloneEval() *Eval {
	return &Eval{
		out:     e.out,
		outMu:   e.outMu,
		wg:      e.wg,
		excBase: e.excBase,
		excZero: e.excZero,
		excVal:  e.excVal,
		excType: e.excType,
		excKey:  e.excKey,
		excIdx:  e.excIdx,
		excFile: e.excFile,
		excStop: e.excStop,
	}
}

