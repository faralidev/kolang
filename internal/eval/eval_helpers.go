package eval

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// toNumber converts a value to an int64, truncating floats. It returns a
// non-nil error for non-numeric values.
func toNumber(v Value) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case float64:
		return int64(t), nil
	}
	return 0, fmt.Errorf("عدد نیست")
}

// toIndex converts a value to an integer index, rejecting fractional floats
// (e.g. 1.5) which would otherwise be silently truncated to the floor.
func toIndex(v Value) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case float64:
		if t != float64(int64(t)) {
			return 0, fmt.Errorf("عدد صحیح نیست")
		}
		return int64(t), nil
	}
	return 0, fmt.Errorf("عدد نیست")
}

// toFloat converts a numeric value to a float64. ok is false for non-numeric
// values.
func toFloat(v Value) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case float64:
		return t, true
	}
	return 0, false
}

// arith applies an arithmetic operator, keeping the integer path when both
// operands are int64 and the float path otherwise. Non-numeric operands raise
// خطای‌نوع.
func (e *Eval) arith(l, r Value, iFn func(int64, int64) int64, fFn func(float64, float64) float64, line int) (Value, error) {
	if li, ok := toFloat(l); ok {
		if ri, ok2 := toFloat(r); ok2 {
			if _, isI := l.(int64); isI {
				if _, isI2 := r.(int64); isI2 {
					return iFn(l.(int64), r.(int64)), nil
				}
			}
			return fFn(li, ri), nil
		}
	}
	return nil, e.raise(e.excType, "عملگر روی نوع نادرست", line)
}

// equal compares two values for equality (==). Numbers compare numerically
// across int/float; lists/tuples/dicts/sets compare structurally; reference
// types (instances, classes, functions, channels, modules, supers,
// interfaces) compare by identity.
func equal(a, b Value) bool {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		return af == bf
	}
	switch av := a.(type) {
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case nil:
		return b == nil
	case *List:
		if bv, ok := b.(*List); ok {
			if len(av.Vals) != len(bv.Vals) {
				return false
			}
			for i := range av.Vals {
				if !equal(av.Vals[i], bv.Vals[i]) {
					return false
				}
			}
			return true
		}
	case *Tuple:
		if bv, ok := b.(*Tuple); ok {
			if len(av.Vals) != len(bv.Vals) {
				return false
			}
			for i := range av.Vals {
				if !equal(av.Vals[i], bv.Vals[i]) {
					return false
				}
			}
			return true
		}
	case *Dict:
		if bv, ok := b.(*Dict); ok {
			if len(av.M) != len(bv.M) {
				return false
			}
			for k, v := range av.M {
				bvv, ok := bv.M[k]
				if !ok || !equal(v, bvv) {
					return false
				}
			}
			return true
		}
	case *Set:
		if bv, ok := b.(*Set); ok {
			if len(av.M) != len(bv.M) {
				return false
			}
			for k := range av.M {
				if _, ok := bv.M[k]; !ok {
					return false
				}
			}
			return true
		}
	case *Instance:
		if bv, ok := b.(*Instance); ok {
			return av == bv
		}
	case *Class:
		if bv, ok := b.(*Class); ok {
			return av == bv
		}
	case *Channel:
		if bv, ok := b.(*Channel); ok {
			return av == bv
		}
	case *Function:
		if bv, ok := b.(*Function); ok {
			return av == bv
		}
	case *Builtin:
		if bv, ok := b.(*Builtin); ok {
			return av == bv
		}
	case *Module:
		if bv, ok := b.(*Module); ok {
			return av == bv
		}
	case *Super:
		if bv, ok := b.(*Super); ok {
			return av == bv
		}
	case *Interface:
		if bv, ok := b.(*Interface); ok {
			return av == bv
		}
	}
	return false
}

// compare orders two values (<, >, <=, >=). Numbers compare numerically and
// strings lexicographically; any other pair raises a runtime error.
func compare(l, r Value, test func(int) bool) (bool, error) {
	lf, lok := toFloat(l)
	rf, rok := toFloat(r)
	if lok && rok {
		if lf < rf {
			return test(-1), nil
		}
		if lf > rf {
			return test(1), nil
		}
		return test(0), nil
	}
	if ls, ok := l.(string); ok {
		if rs, ok2 := r.(string); ok2 {
			c := strings.Compare(ls, rs)
			return test(c), nil
		}
	}
	return false, &RuntimeError{Line: 0, Msg: "مقایسه فقط بین عدد یا متن ممکن است"}
}

// normalizeIdx clamps a possibly-negative index into [0, n].
func normalizeIdx(i, n int64) int64 {
	if i < 0 {
		i += n
	}
	if i < 0 {
		i = 0
	}
	if i > n {
		i = n
	}
	return i
}

// iterValues materializes any iterable into a slice of values: lists, tuples,
// strings (runes), ranges, sets, dict keys, and generators (fully consumed).
// Non-iterables return an error.
func iterValues(v Value) ([]Value, error) {
	switch t := v.(type) {
	case *List:
		return t.Vals, nil
	case *Tuple:
		return t.Vals, nil
	case string:
		var out []Value
		for _, r := range t {
			out = append(out, string(r))
		}
		return out, nil
	case *Range:
		return t.values(), nil
	case *Set:
		out := make([]Value, 0, len(t.M))
		for _, v := range t.M {
			out = append(out, v)
		}
		return out, nil
	case *Dict:
		out := make([]Value, 0, len(t.M))
		for k := range t.M {
			out = append(out, k)
		}
		return out, nil
	case *Generator:
		var out []Value
		for {
			v, ok, err := t.next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			out = append(out, v)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("قابل پیمایش نیست")
	}
}

// Range is a lazy range value produced by بازه().
type Range struct{ start, end, step int64 }

// values materializes the range into a slice of int64 values.
func (r *Range) values() []Value {
	if r.step == 0 {
		return nil
	}
	var out []Value
	for i := r.start; (r.step > 0 && i < r.end) || (r.step < 0 && i > r.end); i += r.step {
		out = append(out, i)
	}
	return out
}

// Set is an unordered collection of unique values. Elements are keyed by their
// string form (Stringify) so membership is O(1).
type Set struct {
	M map[string]Value
}

// newSet creates an empty set.
func newSet() *Set {
	return &Set{M: map[string]Value{}}
}

// Add inserts a value into the set (deduplicating by its string form).
func (s *Set) Add(v Value) {
	s.M[Stringify(v)] = v
}

// Has reports whether the set contains a value equal to v.
func (s *Set) Has(v Value) bool {
	_, ok := s.M[Stringify(v)]
	return ok
}

// Remove deletes a value from the set (a no-op if absent).
func (s *Set) Remove(v Value) {
	delete(s.M, Stringify(v))
}

// Len returns the number of distinct elements in the set.
func (s *Set) Len() int {
	return len(s.M)
}

// String returns the set's printed form, with elements sorted for determinism.
func (s *Set) String() string {
	keys := make([]string, 0, len(s.M))
	for k := range s.M {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return "{" + strings.Join(keys, ", ") + "}"
}

// Stringify converts a value to its printed representation.
func Stringify(v Value) string {
	switch t := v.(type) {
	case nil:
		return "تهی"
	case bool:
		if t {
			return "درست"
		}
		return "غلط"
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		return t
	case *List:
		parts := make([]string, 0, len(t.Vals))
		for _, x := range t.Vals {
			parts = append(parts, Stringify(x))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Tuple:
		parts := make([]string, 0, len(t.Vals))
		for _, x := range t.Vals {
			parts = append(parts, Stringify(x))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *Dict:
		var parts []string
		for k, v := range t.M {
			parts = append(parts, k+": "+Stringify(v))
		}
		sort.Strings(parts)
		return "{" + strings.Join(parts, ", ") + "}"
	case *Set:
		return t.String()
	case *Function:
		return "<تابع " + t.Name + ">"
	case *Builtin:
		return "<داخلی " + t.Name + ">"
	case *Module:
		return "<ماژول " + t.Name + ">"
	case *Class:
		return t.String()
	case *Instance:
		return t.String()
	case *Interface:
		return t.String()
	case *Super:
		return t.String()
	case *Generator:
		return t.String()
	case *Channel:
		return "<کانال>"
	default:
		return fmt.Sprintf("%v", t)
	}
}
