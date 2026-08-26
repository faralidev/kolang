package eval

import (
	"fmt"
	"math"
	"strings"

	"github.com/faralidev/kolang/internal/ast"
	"github.com/faralidev/kolang/internal/token"
)

// evalExpr evaluates an expression node and returns its value. It dispatches
// on the concrete AST type to the appropriate handler. The special name والد
// (super) is resolved through the frame stack via superMarker.
func (e *Eval) evalExpr(n ast.Expr, env *Env) (Value, error) {
	switch ex := n.(type) {
	case *ast.NumberLit:
		if ex.Int {
			return ex.IntVal, nil
		}
		return ex.FVal, nil
	case *ast.StrLit:
		return e.interpolate(ex.Raw, env, ex.L)
	case *ast.BoolLit:
		return ex.Value, nil
	case *ast.NoneLit:
		return nil, nil
	case *ast.Ident:
		if ex.Name == "والد" {
			return e.superMarker(ex.L)
		}
		if v, ok := env.Get(ex.Name); ok {
			return v, nil
		}
		return nil, &RuntimeError{Line: ex.L, Msg: "متغیر یافت نشد: " + ex.Name}
	case *ast.Unary:
		return e.evalUnary(ex, env)
	case *ast.BinaryOp:
		return e.evalBinary(ex, env)
	case *ast.Call:
		return e.evalCall(ex, env)
	case *ast.MethodCall:
		return e.evalMethodCall(ex, env)
	case *ast.MemberAccess:
		return e.evalMemberAccess(ex, env)
	case *ast.Index:
		return e.evalIndex(ex, env)
	case *ast.Slice:
		return e.evalSlice(ex, env)
	case *ast.ListLit:
		vals := make([]Value, 0, len(ex.Elems))
		for _, el := range ex.Elems {
			v, err := e.evalExpr(el, env)
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
		}
		return &List{Vals: vals}, nil
	case *ast.TupleLit:
		vals := make([]Value, 0, len(ex.Elems))
		for _, el := range ex.Elems {
			v, err := e.evalExpr(el, env)
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
		}
		return &Tuple{Vals: vals}, nil
	case *ast.DictLit:
		m := map[string]Value{}
		for i, k := range ex.Keys {
			kv, err := e.evalExpr(k, env)
			if err != nil {
				return nil, err
			}
			vv, err := e.evalExpr(ex.Values[i], env)
			if err != nil {
				return nil, err
			}
			m[Stringify(kv)] = vv
		}
		return &Dict{M: m}, nil
	case *ast.SetLit:
		// A set literal builds a real *Set (deduplicating via Stringify keys).
		s := newSet()
		for _, el := range ex.Elems {
			v, err := e.evalExpr(el, env)
			if err != nil {
				return nil, err
			}
			s.Add(v)
		}
		return s, nil
	case *ast.PipeExpr:
		lv, err := e.evalExpr(ex.Left, env)
		if err != nil {
			return nil, err
		}
		return e.evalPipe(ex.Right, lv, env)
	case *ast.TernaryExpr:
		c, err := e.evalExpr(ex.Cond, env)
		if err != nil {
			return nil, err
		}
		b, err := e.toBool(c, ex.L)
		if err != nil {
			return nil, err
		}
		if b {
			return e.evalExpr(ex.TrueBranch, env)
		}
		return e.evalExpr(ex.FalseBranch, env)
	case *ast.ListComp:
		return e.evalListComp(ex, env)
	case *ast.DictComp:
		return e.evalDictComp(ex, env)
	case *ast.SetComp:
		return e.evalSetComp(ex, env)
	case *ast.GenExp:
		// v1.0: generator expressions are LAZY — they return a *Generator that
		// yields elements on demand, reusing the goroutine-based generator
		// infrastructure. The runner iterates the comprehension clauses (via
		// compRec) and yields each element.
		compEnv := NewEnv(env)
		lazyGen := &Generator{
			Line:    ex.L,
			genEval: e.newGeneratorEval(),
			ch:      make(chan Value),
			resume:  make(chan struct{}),
			done:    make(chan struct{}),
			genExpRunner: func(g *Generator) error {
				ge := g.genEval
				ge.currentGen = g
				return ge.compRec(ex.Clauses, 0, compEnv, func() error {
					ev, err := ge.evalExpr(ex.Element, compEnv)
					if err != nil {
						return err
					}
					_, err = ge.yieldValue(ev, g)
					return err
				})
			},
		}
		return lazyGen, nil
	case *ast.ChannelLit:
		return e.evalChannelLit(ex, env)
	case *ast.RecvExpr:
		return e.evalRecvExpr(ex, env)
	default:
		return nil, &RuntimeError{Line: n.Line(), Msg: "عبارت ناشناخته"}
	}
}

// evalUnary evaluates a unary operation: numeric negation («-x») or logical
// negation («not x»).
func (e *Eval) evalUnary(ex *ast.Unary, env *Env) (Value, error) {
	v, err := e.evalExpr(ex.Expr, env)
	if err != nil {
		return nil, err
	}
	switch ex.Op {
	case "-":
		switch t := v.(type) {
		case int64:
			return -t, nil
		case float64:
			return -t, nil
		}
		return nil, &RuntimeError{Line: ex.L, Msg: "علامت منفی فقط روی عدد کار می‌کند"}
	case "not":
		b, err := e.toBool(v, ex.L)
		if err != nil {
			return nil, err
		}
		return !b, nil
	}
	return nil, &RuntimeError{Line: ex.L, Msg: "عملگر یکتایی ناشناخته"}
}

// evalBinary evaluates a binary operation. The logical operators همچنین (and)
// and یا (or) short-circuit and always yield booleans; در (in) performs a
// membership test; everything else delegates to applyBinop.
func (e *Eval) evalBinary(ex *ast.BinaryOp, env *Env) (Value, error) {
	switch ex.Op {
	case string(token.AND):
		l, err := e.evalExpr(ex.Left, env)
		if err != nil {
			return nil, err
		}
		lb, err := e.toBool(l, ex.L)
		if err != nil {
			return nil, err
		}
		if !lb {
			return false, nil
		}
		r, err := e.evalExpr(ex.Right, env)
		if err != nil {
			return nil, err
		}
		rb, err := e.toBool(r, ex.L)
		if err != nil {
			return nil, err
		}
		return rb, nil
	case string(token.OR):
		l, err := e.evalExpr(ex.Left, env)
		if err != nil {
			return nil, err
		}
		lb, err := e.toBool(l, ex.L)
		if err != nil {
			return nil, err
		}
		if lb {
			return true, nil
		}
		r, err := e.evalExpr(ex.Right, env)
		if err != nil {
			return nil, err
		}
		rb, err := e.toBool(r, ex.L)
		if err != nil {
			return nil, err
		}
		return rb, nil
	case string(token.IN):
		l, err := e.evalExpr(ex.Left, env)
		if err != nil {
			return nil, err
		}
		r, err := e.evalExpr(ex.Right, env)
		if err != nil {
			return nil, err
		}
		return e.inOperator(l, r, ex.L)
	}

	l, err := e.evalExpr(ex.Left, env)
	if err != nil {
		return nil, err
	}
	r, err := e.evalExpr(ex.Right, env)
	if err != nil {
		return nil, err
	}
	return e.applyBinop(ex.Op, l, r, ex.L)
}

// applyBinop applies an arithmetic, comparison, or string-concatenation
// operator to two already-evaluated values. True division (÷) always yields a
// float; division by zero raises خطای‌صفر; int power is computed with
// overflow detection via intPow.
func (e *Eval) applyBinop(op string, l, r Value, line int) (Value, error) {
	switch op {
	case string(token.PLUS):
		// string concat
		if ls, ok := l.(string); ok {
			if rs, ok2 := r.(string); ok2 {
				return ls + rs, nil
			}
		}
		return e.arith(l, r, func(a, b int64) int64 { return a + b }, func(a, b float64) float64 { return a + b }, line)
	case string(token.MINUS):
		return e.arith(l, r, func(a, b int64) int64 { return a - b }, func(a, b float64) float64 { return a - b }, line)
	case string(token.STAR):
		return e.arith(l, r, func(a, b int64) int64 { return a * b }, func(a, b float64) float64 { return a * b }, line)
	case string(token.DIV):
		// true division (÷) always returns a float, like Python's /
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if !lok || !rok {
			return nil, &RuntimeError{Line: line, Msg: "تقسیم به عدد نیاز دارد"}
		}
		if rf == 0 {
			return nil, e.raise(e.excZero, "تقسیم بر صفر", line)
		}
		return lf / rf, nil
	case string(token.FLOORDIV):
		if rf, ok := toFloat(r); ok && rf == 0 {
			return nil, e.raise(e.excZero, "تقسیم بر صفر", line)
		}
		return e.arith(l, r, func(a, b int64) int64 { return a / b }, func(a, b float64) float64 { return math.Floor(a / b) }, line)
	case string(token.PERCENT):
		if rf, ok := toFloat(r); ok && rf == 0 {
			return nil, e.raise(e.excZero, "تقسیم بر صفر", line)
		}
		return e.arith(l, r, func(a, b int64) int64 { return a % b }, func(a, b float64) float64 { return math.Mod(a, b) }, line)
	case string(token.POW):
		// int path only when both operands are int AND exponent >= 0, else float
		li, lInt := l.(int64)
		ri, rInt := r.(int64)
		if lInt && rInt && ri >= 0 {
			return e.intPow(li, ri, line)
		}
		lf, _ := toFloat(l)
		rf, _ := toFloat(r)
		return math.Pow(lf, rf), nil
	case string(token.EQ):
		return equal(l, r), nil
	case string(token.LT):
		return compare(l, r, func(c int) bool { return c < 0 })
	case string(token.GT):
		return compare(l, r, func(c int) bool { return c > 0 })
	case string(token.LTE):
		return compare(l, r, func(c int) bool { return c <= 0 })
	case string(token.GTE):
		return compare(l, r, func(c int) bool { return c >= 0 })
	default:
		return nil, &RuntimeError{Line: line, Msg: "عملگر دوتایی ناشناخته: " + op}
	}
}

// intPow computes base^exp for a non-negative integer exponent using
// exponentiation by squaring, with overflow detection on every multiply.
// On overflow it raises a catchable خطای‌مقدار ("سرریز در توان") instead of
// silently returning a truncated float result (C6). Exponents with a huge
// magnitude are handled in O(log exp) steps, so e.g. ۱ * ۱۰۰۰۰۰۰۰۰۰ cannot
// hang the loop.
func (e *Eval) intPow(base, exp int64, line int) (Value, error) {
	result := int64(1)
	for exp > 0 {
		if exp&1 == 1 {
			if mulOverflows(result, base) {
				return nil, e.raise(e.excVal, "سرریز در توان", line)
			}
			result *= base
		}
		exp >>= 1
		if exp == 0 {
			break
		}
		if mulOverflows(base, base) {
			return nil, e.raise(e.excVal, "سرریز در توان", line)
		}
		base *= base
	}
	return result, nil
}

// mulOverflows reports whether a*b overflows int64.
func mulOverflows(a, b int64) bool {
	if a == 0 || b == 0 {
		return false
	}
	if a > 0 {
		if b > 0 {
			return a > math.MaxInt64/b
		}
		return b < math.MinInt64/a
	}
	if b > 0 {
		return a < math.MinInt64/b
	}
	return b < math.MaxInt64/a
}

// inOperator implements «l در r» (membership). Lists/tuples/strings are
// searched element-wise (strings by substring), sets and dicts by stringified
// key; generators are rejected because consuming them here would be a
// side effect.
func (e *Eval) inOperator(l, r Value, line int) (bool, error) {
	switch t := r.(type) {
	case *List:
		for _, v := range t.Vals {
			if equal(v, l) {
				return true, nil
			}
		}
		return false, nil
	case *Tuple:
		for _, v := range t.Vals {
			if equal(v, l) {
				return true, nil
			}
		}
		return false, nil
	case *Set:
		_, ok := t.M[Stringify(l)]
		return ok, nil
	case string:
		ls, ok := l.(string)
		if !ok {
			return false, nil
		}
		return strings.Contains(t, ls), nil
	case *Dict:
		_, ok := t.M[Stringify(l)]
		return ok, nil
	case *Generator:
		return false, &RuntimeError{Line: line, Msg: "در روی مولد مقدارها را مصرف می‌کند"}
	default:
		return false, &RuntimeError{Line: line, Msg: "در فقط روی فهرست، متن یا گنجه کار می‌کند"}
	}
}

// evalCall evaluates the callee and all arguments, then dispatches to call.
func (e *Eval) evalCall(ex *ast.Call, env *Env) (Value, error) {
	fn, err := e.evalExpr(ex.Fn, env)
	if err != nil {
		return nil, err
	}
	args := make([]Value, 0, len(ex.Args))
	for _, a := range ex.Args {
		v, err := e.evalExpr(a, env)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	kwargs := map[string]Value{}
	for _, kw := range ex.KwArgs {
		v, err := e.evalExpr(kw.Value, env)
		if err != nil {
			return nil, err
		}
		kwargs[kw.Name] = v
	}
	return e.call(fn, args, kwargs, ex.L)
}

// call invokes any callable value: *Function (user-defined), *Builtin
// (native), *Class (instantiation), or *Super (a bare والدی() marker stays
// callable as a proxy). Non-callables raise خطای‌نوع.
func (e *Eval) call(fn Value, args []Value, kwargs map[string]Value, line int) (Value, error) {
	switch f := fn.(type) {
	case *Function:
		return e.callFunction(f, args, kwargs, line)
	case *Builtin:
		return f.Fn(args)
	case *Class:
		return e.instantiate(f, args, kwargs, line)
	case *Super:
		// والدی() — a bare super() marker: it stays callable so the receiver
		// of a following method call is a Super proxy.
		return f, nil
	default:
		return nil, e.raise(e.excType, fmt.Sprintf("%s قابل فراخوانی نیست", Stringify(fn)), line)
	}
}

// maxCallDepth caps the function-call nesting depth so deep recursion raises a
// catchable خطای‌مقدار instead of blowing the Go stack with an uncatchable
// panic (L14). Every non-generator callFunction pushes a defer frame on entry,
// so len(e.deferStack) is the current call depth.
const maxCallDepth = 10000

// callFunction runs a user-defined function body in a fresh environment:
// it binds parameters, checks annotated parameter types, opens a defer frame,
// evaluates the body, then runs deferred calls in LIFO order. A generator
// function instead returns a lazily-iterated *Generator without running its
// body. Recursion depth is capped by maxCallDepth.
func (e *Eval) callFunction(f *Function, args []Value, kwargs map[string]Value, line int) (Value, error) {
	// A generator function does not run its body on call; it returns a lazily
	// iterated *Generator.
	if f.IsGenerator {
		return &Generator{
			Fn:      f,
			Args:    args,
			Kwargs:  kwargs,
			Line:    line,
			genEval: e.newGeneratorEval(),
			ch:      make(chan Value),
			resume:  make(chan struct{}),
			done:    make(chan struct{}),
		}, nil
	}

	// L14: unbounded recursion guard. Construct the raise signal directly (not
	// via e.raise, whose instantiate → constructor call would re-enter
	// callFunction at the same depth and recurse forever).
	if len(e.deferStack) >= maxCallDepth {
		return nil, raiseSignal{
			exc:  &Instance{Class: e.excVal, Fields: map[string]Value{"پیام": "عمق بازگشت بیش از حد"}},
			line: line,
		}
	}

	newEnv := NewEnv(f.Env)
	if err := e.bindParams(f, args, kwargs, newEnv, line); err != nil {
		return nil, err
	}
	// Gradual typing: check each annotated parameter's bound value.
	for _, p := range f.Params {
		if p.Ann == "" {
			continue
		}
		if v, ok := newEnv.Get(p.Name); ok {
			if err := e.checkType(v, p.Ann, newEnv, line); err != nil {
				return nil, err
			}
		}
	}
	e.pushDefers()
	_, err := e.evalBlock(f.Body.Stmts, newEnv)
	run := e.popDefers()
	if derr := e.runDefers(run); derr != nil {
		// A deferred call raised — propagate it, overriding any normal return
		// (finally-wins semantics).
		return nil, derr
	}
	if err != nil {
		if rs, ok := err.(returnSignal); ok {
			// Gradual typing: check the return value against the annotation.
			if f.RetType != "" {
				if cerr := e.checkType(rs.v, f.RetType, newEnv, line); cerr != nil {
					return nil, cerr
				}
			}
			return rs.v, nil
		}
		return nil, err
	}
	return nil, nil
}

// bindParams binds positional and keyword arguments (and defaults) to the
// function's parameter names in a fresh environment.
func (e *Eval) bindParams(f *Function, args []Value, kwargs map[string]Value, env *Env, line int) error {
	// A «*name» varargs parameter absorbs all remaining positional args and a
	// «**name» kwargs parameter absorbs all unconsumed keyword args (spec §5.7).
	usedKwargs := map[string]bool{}
	pos := 0
	for _, p := range f.Params {
		if p.Variadic {
			// Collect every remaining positional arg into a *List.
			rest := make([]Value, 0, len(args)-pos)
			if pos < len(args) {
				rest = append(rest, args[pos:]...)
			}
			env.Set(p.Name, &List{Vals: rest})
			pos = len(args)
			continue
		}
		if p.Kwargs {
			// Collect every keyword arg not already bound to a named param.
			m := map[string]Value{}
			for k, v := range kwargs {
				if !usedKwargs[k] {
					m[k] = v
				}
			}
			env.Set(p.Name, &Dict{M: m})
			for k := range kwargs {
				usedKwargs[k] = true
			}
			continue
		}
		_, kwFilled := kwargs[p.Name]
		posFilled := pos < len(args)
		if posFilled && kwFilled {
			// H10: the same parameter filled both positionally and by keyword.
			return e.raise(e.excType, "آرگومان تکراری", line)
		}
		if kwFilled {
			env.Set(p.Name, kwargs[p.Name])
			usedKwargs[p.Name] = true
			continue
		}
		if posFilled {
			env.Set(p.Name, args[pos])
			pos++
			continue
		}
		if p.HasDefault {
			dv, err := e.evalExpr(p.Default, f.Env)
			if err != nil {
				return err
			}
			env.Set(p.Name, dv)
			continue
		}
		return &RuntimeError{Line: line, Msg: "آرگومان «" + p.Name + "» داده نشده است"}
	}
	// H10: extra positional args are an error unless a variadic param consumed
	// them (it sets pos to len(args) above).
	if pos < len(args) {
		return e.raise(e.excType, "تعداد آرگومان‌ها بیش از حد است", line)
	}
	return nil
}
