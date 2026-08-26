package eval

import (
	"fmt"
	"strings"
)

// checkType verifies a value against a type annotation string (gradual
// typing). Tuple annotations «(A و B)» are checked element-wise; scalar
// annotations (صحیح, متن, ...) and class/interface names are checked via
// typeMatches. A mismatch raises خطای‌نوع.
func (e *Eval) checkType(v Value, ann string, env *Env, line int) error {
	t := strings.TrimSpace(ann)
	if t == "" {
		return nil
	}
	if strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") {
		inner := strings.TrimSpace(t[1 : len(t)-1])
		parts := splitTypeList(inner)
		if len(parts) == 1 {
			// A parenthesized scalar, e.g. «(صحیح)» is just صحیح, not a
			// 1-tuple type. Strip the parens and re-check as the inner type.
			return e.checkType(v, parts[0], env, line)
		}
		tup, ok := v.(*Tuple)
		if !ok {
			return e.raise(e.excType, fmt.Sprintf("خطای‌نوع: انتظار قفسه %s، داده شد %s", t, typeName(v)), line)
		}
		if len(parts) != len(tup.Vals) {
			return e.raise(e.excType, fmt.Sprintf("خطای‌نوع: تعداد عناصر خروجی باید %d باشد، داده شد %d", len(parts), len(tup.Vals)), line)
		}
		for i, p := range parts {
			if tup.Vals[i] == nil {
				continue
			}
			if err := e.checkType(tup.Vals[i], p, env, line); err != nil {
				return err
			}
		}
		return nil
	}
	if !e.typeMatches(v, t, env) {
		return e.raise(e.excType, fmt.Sprintf("خطای‌نوع: انتظار %s، داده شد %s", t, typeName(v)), line)
	}
	return nil
}

// splitTypeList splits a tuple type annotation on «و» separators, but only at
// parenthesis depth 0, so nested tuples like «(صحیح و (متن و صحیح))» are
// preserved and class names containing «و» inside parens are not mangled.
func splitTypeList(s string) []string {
	var parts []string
	depth := 0
	start := 0
	rs := []rune(s)
	for i, r := range rs {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case 'و':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(string(rs[start:i])))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(string(rs[start:])))
	return parts
}

// typeMatches reports whether a value satisfies a scalar type name. Primitive
// names (صحیح/اعشاری/متن/...) map to Go types; «هر» matches anything;
// class and interface names resolve through the environment and use
// instanceOf / satisfiesInterface. Unknown type names match anything
// (lenient, so forward references do not break annotated code).
func (e *Eval) typeMatches(v Value, typeName string, env *Env) bool {
	switch typeName {
	case "هر":
		// «هر» (any) is the gradual-typing escape hatch: it accepts any value.
		return true
	case "صحیح":
		_, ok := v.(int64)
		return ok
	case "اعشاری":
		_, ok := v.(float64)
		return ok
	case "متن":
		_, ok := v.(string)
		return ok
	case "فهرست":
		_, ok := v.(*List)
		return ok
	case "مجموعه":
		_, ok := v.(*Set)
		return ok
	case "گنجه":
		_, ok := v.(*Dict)
		return ok
	case "قفسه":
		_, ok := v.(*Tuple)
		return ok
	case "بولی":
		_, ok := v.(bool)
		return ok
	case "تهی", "تهی‌ها":
		return v == nil
	}
	tv, ok := env.Get(typeName)
	if !ok {
		return true
	}
	switch ty := tv.(type) {
	case *Class:
		if v == nil && isExceptionClass(ty) {
			return true
		}
		inst, ok := v.(*Instance)
		return ok && instanceOf(inst, ty)
	case *Interface:
		inst, ok := v.(*Instance)
		return ok && satisfiesInterface(inst.Class, ty)
	}
	return true
}

// satisfiesInterface reports whether a class implements every method of an
// interface (structural/duck typing: no explicit declaration required).
func satisfiesInterface(cls *Class, ifc *Interface) bool {
	for name := range ifc.MethodNames {
		if !cls.hasMethod(name) {
			return false
		}
	}
	return true
}

// typeName returns the Persian name of a value's runtime type (for error
// messages and the نوع builtin).
func typeName(v Value) string {
	switch v.(type) {
	case nil:
		return "تهی"
	case int64:
		return "صحیح"
	case float64:
		return "اعشاری"
	case bool:
		return "بولی"
	case string:
		return "متن"
	case *List:
		return "فهرست"
	case *Set:
		return "مجموعه"
	case *Tuple:
		return "قفسه"
	case *Dict:
		return "گنجه"
	case *Instance:
		return v.(*Instance).Class.Name
	case *Generator:
		return "جنریتور"
	case *Channel:
		return "کانال"
	}
	return "ناشناخته"
}
