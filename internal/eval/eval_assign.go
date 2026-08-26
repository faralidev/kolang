package eval

import (
	"github.com/faralidev/kolang/internal/ast"
)

// assign stores a value into an assignment target: a plain identifier
// (through the environment's Assign), an ezafe attribute (via setAttr), or an
// index (via setIndex). Other target shapes are syntax errors.
func (e *Eval) assign(target ast.Expr, v Value, env *Env) error {
	switch t := target.(type) {
	case *ast.Ident:
		env.Assign(t.Name, v)
		return nil
	case *ast.MemberAccess:
		recv, err := e.evalExpr(t.Receiver, env)
		if err != nil {
			return err
		}
		attr := t.Attr.(*ast.Ident).Name
		return e.setAttr(recv, attr, v, t.L)
	case *ast.Index:
		recv, err := e.evalExpr(t.Target, env)
		if err != nil {
			return err
		}
		idx, err := e.evalExpr(t.Index, env)
		if err != nil {
			return err
		}
		return e.setIndex(recv, idx, v, t.L)
	default:
		return &RuntimeError{Line: target.Line(), Msg: "به این عبارت نمی‌توان مقدار داد"}
	}
}

// setIndex mutates an element by index: «xs[i] = v» or «d[k] = v».
func (e *Eval) setIndex(recv, idx, v Value, line int) error {
	switch t := recv.(type) {
	case *List:
		i, err := toNumber(idx)
		if err != nil {
			return &RuntimeError{Line: line, Msg: "نمایه باید یک عدد صحیح باشد"}
		}
		if i < 0 {
			i += int64(len(t.Vals))
		}
		if i < 0 || i >= int64(len(t.Vals)) {
			return e.raise(e.excIdx, "نمایه خارج از محدوده", line)
		}
		t.Vals[i] = v
		return nil
	case *Dict:
		key := Stringify(idx)
		t.M[key] = v
		return nil
	default:
		return &RuntimeError{Line: line, Msg: "این مقدار را نمی‌توان با نمایه تغییر داد"}
	}
}

// setAttr writes an attribute on a receiver: dict/module members, instance
// fields, or class-scope variables. Other receivers are read-only.
func (e *Eval) setAttr(recv Value, attr string, v Value, line int) error {
	switch r := recv.(type) {
	case *Dict:
		r.M[attr] = v
		return nil
	case *Module:
		r.Members[attr] = v
		return nil
	case *Instance:
		r.Fields[attr] = v
		return nil
	case *Class:
		r.Env.Set(attr, v)
		return nil
	default:
		return &RuntimeError{Line: line, Msg: "ویژگی این مقدار را نمی‌توان تغییر داد"}
	}
}
