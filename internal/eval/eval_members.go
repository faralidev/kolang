package eval

import (
	"fmt"
	"strings"

	"github.com/faralidev/kolang/internal/ast"
)

// evalMemberAccess evaluates «attrِ receiver» (attribute access). The receiver
// is evaluated and getAttr looks up the attribute; a missing attribute raises
// خطای‌نوع.
func (e *Eval) evalMemberAccess(ex *ast.MemberAccess, env *Env) (Value, error) {
	recv, err := e.evalExpr(ex.Receiver, env)
	if err != nil {
		return nil, err
	}
	attr := ""
	if id, ok := ex.Attr.(*ast.Ident); ok {
		attr = id.Name
	} else {
		return nil, &RuntimeError{Line: ex.L, Msg: "نام ویژگی باید یک نام باشد"}
	}
	v, ok := e.getAttr(recv, attr)
	if !ok {
		return nil, e.raise(e.excType, "ویژگی «"+attr+"» روی "+Stringify(recv)+" وجود ندارد", ex.L)
	}
	return v, nil
}

// evalMethodCall evaluates «methodِ(args)receiver» (method call). On an
// *Instance the method is looked up on the class chain and invoked with خود
// prepended; on a *Super proxy the lookup starts at the parent class; other
// receivers fall back to getAttr and bind the receiver as the first argument.
func (e *Eval) evalMethodCall(ex *ast.MethodCall, env *Env) (Value, error) {
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
	// Evaluate the call arguments once (shared by all dispatch paths).
	evalArgs := func() ([]Value, error) {
		args := make([]Value, 0, len(ex.Args))
		for _, a := range ex.Args {
			v, err := e.evalExpr(a, env)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		return args, nil
	}

	switch r := recv.(type) {
	case *Instance:
		fn, definingClass := r.Class.lookupMethod(attr)
		if fn == nil {
			// Fall back to a field that holds a callable value.
			if f, ok := r.Fields[attr]; ok {
				args, err := evalArgs()
				if err != nil {
					return nil, err
				}
				return e.call(f, args, nil, ex.L)
			}
			if v, ok := env.Get(attr); ok {
				if cf, isFn := v.(*Function); isFn {
					args, err := evalArgs()
					if err != nil {
						return nil, err
					}
					return e.callFunction(cf, append([]Value{r}, args...), nil, ex.L)
				}
			}
			return nil, &RuntimeError{Line: ex.L, Msg: "روش «" + attr + "» روی " + Stringify(recv) + " وجود ندارد"}
		}
		args, err := evalArgs()
		if err != nil {
			return nil, err
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
		args, err := evalArgs()
		if err != nil {
			return nil, err
		}
		callArgs := args
		boundSelf := r.Obj
		if r.Obj == nil {
			if len(args) > 0 {
				if inst, ok := args[0].(*Instance); ok {
					boundSelf = inst
				}
			}
		} else {
			callArgs = append([]Value{r.Obj}, args...)
		}
		if boundSelf != nil {
			e.pushFrame(&Frame{Inst: boundSelf, Class: definingClass})
			result, err := e.callFunction(fn, callArgs, nil, ex.L)
			e.popFrame()
			return result, err
		}
		return e.callFunction(fn, callArgs, nil, ex.L)
	default:
		method, ok := e.getAttr(recv, attr)
		if !ok {
			return nil, &RuntimeError{Line: ex.L, Msg: "روش «" + attr + "» روی " + Stringify(recv) + " وجود ندارد"}
		}
		args, err := evalArgs()
		if err != nil {
			return nil, err
		}
		args = append([]Value{recv}, args...)
		return e.call(method, args, nil, ex.L)
	}
}

// getAttr resolves a named attribute on any receiver type: module/dict
// members, class methods and fields, instance fields (checked before methods,
// so an instance field can shadow a method), the special والد attribute on
// instances, built-in list methods (طول/بیافزا/حذفکن), and the channel
// بسته‌است check.
func (e *Eval) getAttr(recv Value, attr string) (Value, bool) {
	switch r := recv.(type) {
	case *Module:
		v, ok := r.Members[attr]
		return v, ok
	case *Dict:
		v, ok := r.M[attr]
		return v, ok
	case *Class:
		if m, _ := r.lookupMethod(attr); m != nil {
			return m, true
		}
		if v, ok := r.Env.Get(attr); ok {
			return v, true
		}
	case *Instance:
		if attr == "والد" {
			curClass := e.currentMethodClass()
			if curClass == nil {
				curClass = r.Class
			}
			if curClass.Parent != nil {
				return &Super{Obj: r, Klass: curClass.Parent}, true
			}
			return nil, false
		}
		if f, ok := r.Fields[attr]; ok {
			return f, true
		}
		if m, _ := r.Class.lookupMethod(attr); m != nil {
			return m, true
		}
		for k := r.Class; k != nil; k = k.Parent {
			if v, ok := k.Env.Get(attr); ok {
				return v, true
			}
		}
	case *Super:
		if m, _ := r.Klass.lookupMethod(attr); m != nil {
			return m, true
		}
	case *List:
		switch attr {
		case "طول":
			return int64(len(r.Vals)), true
		case "بیافزا":
			return &Builtin{Name: attr, Fn: func(a []Value) (Value, error) {
				if len(a) < 2 {
					return nil, &RuntimeError{Line: 0, Msg: "بیافزا به مقدار نیاز دارد"}
				}
				lst, ok := a[0].(*List)
				if !ok {
					return nil, &RuntimeError{Line: 0, Msg: "بیافزا به فهرست نیاز دارد"}
				}
				lst.Vals = append(lst.Vals, a[1])
				return nil, nil
			}}, true
		case "حذفکن":
			return &Builtin{Name: attr, Fn: func(a []Value) (Value, error) {
				if len(a) < 2 {
					return nil, &RuntimeError{Line: 0, Msg: "حذف‌کن به مقدار نیاز دارد"}
				}
				lst, ok := a[0].(*List)
				if !ok {
					return nil, &RuntimeError{Line: 0, Msg: "حذف‌کن به فهرست نیاز دارد"}
				}
				for i, v := range lst.Vals {
					if equal(v, a[1]) {
						lst.Vals = append(lst.Vals[:i], lst.Vals[i+1:]...)
						break
					}
				}
				return nil, nil
			}}, true
		}
	case *Channel:
		if attr == "بسته‌است" {
			return r.isClosed(), true
		}
	}
	return nil, false
}

// evalIndex evaluates «target[index]»: list/string indexing (with negative
// indices counting from the end) or dict lookup by stringified key. Out of
// range / missing keys raise خطای‌نمایه / خطای‌کلید.
func (e *Eval) evalIndex(ex *ast.Index, env *Env) (Value, error) {
	target, err := e.evalExpr(ex.Target, env)
	if err != nil {
		return nil, err
	}
	idx, err := e.evalExpr(ex.Index, env)
	if err != nil {
		return nil, err
	}
	switch t := target.(type) {
	case *List:
		i, err := toIndex(idx)
		if err != nil {
			return nil, e.raise(e.excType, "نمایه باید صحیح باشد", ex.L)
		}
		if i < 0 {
			i += int64(len(t.Vals))
		}
		if i < 0 || i >= int64(len(t.Vals)) {
			return nil, e.raise(e.excIdx, fmt.Sprintf("نمایه %d خارج از محدوده است", i), ex.L)
		}
		return t.Vals[i], nil
	case string:
		i, err := toIndex(idx)
		if err != nil {
			return nil, e.raise(e.excType, "نمایه باید صحیح باشد", ex.L)
		}
		rs := []rune(t)
		if i < 0 {
			i += int64(len(rs))
		}
		if i < 0 || i >= int64(len(rs)) {
			return nil, e.raise(e.excIdx, fmt.Sprintf("نمایه %d خارج از محدوده است", i), ex.L)
		}
		return string(rs[i]), nil
	case *Dict:
		key := Stringify(idx)
		if v, ok := t.M[key]; ok {
			return v, nil
		}
		return nil, e.raise(e.excKey, "کلید یافت نشد: "+key, ex.L)
	default:
		return nil, e.raise(e.excType, "این مقدار را نمی‌توان نمایه کرد", ex.L)
	}
}

// evalSlice evaluates «target[low:high:step]» for lists and strings, following
// Python slice semantics (negative indices, step direction, out-of-range
// clamping). A zero step raises خطای‌مقدار.
func (e *Eval) evalSlice(ex *ast.Slice, env *Env) (Value, error) {
	target, err := e.evalExpr(ex.Target, env)
	if err != nil {
		return nil, err
	}
	low, high, step := int64(0), int64(0), int64(1)
	if ex.Low != nil {
		v, err := e.evalExpr(ex.Low, env)
		if err != nil {
			return nil, err
		}
		low, err = toIndex(v)
		if err != nil {
			return nil, e.raise(e.excType, "نمایه باید صحیح باشد", ex.L)
		}
	}
	if ex.High != nil {
		v, err := e.evalExpr(ex.High, env)
		if err != nil {
			return nil, err
		}
		high, err = toIndex(v)
		if err != nil {
			return nil, e.raise(e.excType, "نمایه باید صحیح باشد", ex.L)
		}
	}
	if ex.Step != nil {
		v, err := e.evalExpr(ex.Step, env)
		if err != nil {
			return nil, err
		}
		step, err = toIndex(v)
		if err != nil {
			return nil, e.raise(e.excType, "نمایه باید صحیح باشد", ex.L)
		}
	}
	if step == 0 {
		return nil, e.raise(e.excVal, "گام نمی‌تواند صفر باشد", ex.L)
	}
	switch t := target.(type) {
	case *List:
		vals := t.Vals
		n := int64(len(vals))
		lo, hi := sliceBounds(low, high, step, n, ex.Low != nil, ex.High != nil)
		var out []Value
		if step > 0 {
			for i := lo; i < hi; i += step {
				out = append(out, vals[i])
			}
		} else {
			for i := lo; i > hi; i += step {
				out = append(out, vals[i])
			}
		}
		return &List{Vals: out}, nil
	case string:
		rs := []rune(t)
		n := int64(len(rs))
		lo, hi := sliceBounds(low, high, step, n, ex.Low != nil, ex.High != nil)
		var sb strings.Builder
		if step > 0 {
			for i := lo; i < hi; i += step {
				sb.WriteRune(rs[i])
			}
		} else {
			for i := lo; i > hi; i += step {
				sb.WriteRune(rs[i])
			}
		}
		return sb.String(), nil
	default:
		return nil, &RuntimeError{Line: ex.L, Msg: "این مقدار را نمی‌توان برش داد"}
	}
}

// sliceBounds normalizes slice bounds for the given step direction, following
// Python's slice semantics:
//   - step > 0: omitted bounds default to 0 and len; out-of-range clamps to
//     [0, len].
//   - step < 0: omitted bounds default to len-1 and -1 (so index 0 is included
//     by the «i > hi» loop); out-of-range clamps to [-1, len-1].
//
// The -1 / len-1 clamps keep the step<0 loop inside the slice's index range.
func sliceBounds(low, high, step, n int64, lowSet, highSet bool) (lo, hi int64) {
	if lowSet {
		lo = clampSliceIdx(low, n, step)
	} else if step > 0 {
		lo = 0
	} else {
		lo = n - 1
	}
	if highSet {
		hi = clampSliceIdx(high, n, step)
	} else if step > 0 {
		hi = n
	} else {
		hi = -1
	}
	return lo, hi
}

// clampSliceIdx adjusts a single slice bound against a sequence of length n,
// mirroring CPython's PySlice_AdjustIndices for one index: negative indices
// count from the end; out-of-range values clamp toward the direction the slice
// is moving (n for a positive step, -1 for a negative step below range, n-1
// for a negative step above range).
func clampSliceIdx(i, n, step int64) int64 {
	if i < 0 {
		i += n
		if i < 0 {
			if step < 0 {
				return -1
			}
			return 0
		}
		return i
	}
	if i >= n {
		if step < 0 {
			return n - 1
		}
		return n
	}
	return i
}
