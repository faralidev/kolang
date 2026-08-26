package eval

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/faralidev/kolang/internal/parser"
)

// truthy reports Python-style truthiness of a value, used by the بولی builtin.
// Numbers are truthy when non-zero, strings/collections when non-empty, and
// reference values (instances, classes, functions, ...) are truthy.
func truthy(v Value) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case int64:
		return t != 0
	case float64:
		return t != 0
	case string:
		return len(t) > 0
	case *List:
		return len(t.Vals) > 0
	case *Tuple:
		return len(t.Vals) > 0
	case *Dict:
		return len(t.M) > 0
	case *Set:
		return t.Len() > 0
	case *Range:
		return len(t.values()) > 0
	default:
		return true
	}
}

// installBuiltins registers the built-in functions in the global environment.
func (e *Eval) installBuiltins(global *Env) {
	e.installExceptions(global)

	bf := func(name string, fn func([]Value) (Value, error)) {
		global.Set(name, &Builtin{Name: name, Fn: fn})
	}

	bf("بنویس", func(args []Value) (Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, Stringify(a))
		}
		e.writeOut(strings.Join(parts, " "))
		return nil, nil
	})

	bf("بگیر", func(args []Value) (Value, error) {
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimRight(line, "\n"), nil
	})

	bf("طول", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "طول یک آرگومان می‌خواهد", 0)
		}
		switch t := args[0].(type) {
		case *List:
			return int64(len(t.Vals)), nil
		case string:
			return int64(len([]rune(t))), nil
		case *Dict:
			return int64(len(t.M)), nil
		case *Tuple:
			return int64(len(t.Vals)), nil
		}
		return nil, e.raise(e.excType, "طول برای این نوع پشتیبانی نمی‌شود", 0)
	})

	bf("بازه", func(args []Value) (Value, error) {
		if len(args) == 0 || len(args) > 3 {
			return nil, e.raise(e.excVal, "بازه یک تا سه آرگومان می‌خواهد", 0)
		}
		var start, end, step int64 = 0, 0, 1
		switch len(args) {
		case 1:
			s, _ := toNumber(args[0])
			end = s
		case 2:
			s, _ := toNumber(args[0])
			en, _ := toNumber(args[1])
			start, end = s, en
		case 3:
			s, _ := toNumber(args[0])
			en, _ := toNumber(args[1])
			sp, _ := toNumber(args[2])
			start, end, step = s, en, sp
		}
		return &Range{start: start, end: end, step: step}, nil
	})

	bf("صحیح", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "صحیح یک آرگومان می‌خواهد", 0)
		}
		switch t := args[0].(type) {
		case int64:
			return t, nil
		case float64:
			return int64(t), nil
		case bool:
			if t {
				return int64(1), nil
			}
			return int64(0), nil
		case string:
			trimmed := strings.TrimSpace(t)
			trimmed = strings.ReplaceAll(trimmed, "٫", ".")
			if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
				return int64(f), nil
			}
			return nil, e.raise(e.excVal, fmt.Sprintf("امکان تبدیل «%s» به صحیح نیست", t), 0)
		case nil:
			return int64(0), nil
		}
		return nil, e.raise(e.excVal, "امکان تبدیل به صحیح نیست", 0)
	})

	bf("اعشاری", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "اعشاری یک آرگومان می‌خواهد", 0)
		}
		switch t := args[0].(type) {
		case int64:
			return float64(t), nil
		case float64:
			return t, nil
		case string:
			trimmed := strings.ReplaceAll(strings.TrimSpace(t), "٫", ".")
			if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
				return f, nil
			}
			return nil, e.raise(e.excVal, fmt.Sprintf("امکان تبدیل «%s» به اعشاری نیست", t), 0)
		}
		return nil, e.raise(e.excVal, "امکان تبدیل به اعشاری نیست", 0)
	})

	bf("متن", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "متن یک آرگومان می‌خواهد", 0)
		}
		return Stringify(args[0]), nil
	})

	bf("فهرست", func(args []Value) (Value, error) {
		if len(args) == 0 {
			return &List{}, nil
		}
		if len(args) == 1 {
			if lst, ok := args[0].(*List); ok {
				return lst, nil
			}
			if tup, ok := args[0].(*Tuple); ok {
				return &List{Vals: tup.Vals}, nil
			}
			if s, ok := args[0].(string); ok {
				var out []Value
				for _, r := range s {
					out = append(out, string(r))
				}
				return &List{Vals: out}, nil
			}
			if rng, ok := args[0].(*Range); ok {
				return &List{Vals: rng.values()}, nil
			}
		}
		return &List{Vals: args}, nil
	})

	bf("گنجه", func(args []Value) (Value, error) {
		m := map[string]Value{}
		if len(args) == 0 {
			return &Dict{M: m}, nil
		}
		// Normalize the input into a flat sequence of items to pair as
		// key/value. گنجه accepts either a list of 2-tuples — e.g.
		// گنجه((a,1),(b,2)) — or a flat alternating sequence such as
		// گنجه(a,1,b,2).
		var items []Value
		if len(args) == 1 {
			switch t := args[0].(type) {
			case *Tuple:
				allPairs := true
				for _, el := range t.Vals {
					if p, ok := el.(*Tuple); ok && len(p.Vals) == 2 {
						m[Stringify(p.Vals[0])] = p.Vals[1]
					} else {
						allPairs = false
					}
				}
				if allPairs {
					return &Dict{M: m}, nil
				}
				items = t.Vals
			case *List:
				for _, el := range t.Vals {
					if p, ok := el.(*Tuple); ok && len(p.Vals) == 2 {
						m[Stringify(p.Vals[0])] = p.Vals[1]
					}
				}
				return &Dict{M: m}, nil
			default:
				items = args
			}
		} else {
			items = args
		}
		// Pair adjacent flat items (a,1,b,2 -> a:1, b:2). A lone 2-tuple is
		// also treated as a single key/value pair.
		i := 0
		for i < len(items) {
			if p, ok := items[i].(*Tuple); ok && len(p.Vals) == 2 {
				m[Stringify(p.Vals[0])] = p.Vals[1]
				i++
				continue
			}
			if i+1 < len(items) {
				m[Stringify(items[i])] = items[i+1]
				i += 2
				continue
			}
			i++
		}
		return &Dict{M: m}, nil
	})

	bf("قفسه", func(args []Value) (Value, error) {
		return &Tuple{Vals: append([]Value{}, args...)}, nil
	})

	bf("بولی", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "بولی یک آرگومان می‌خواهد", 0)
		}
		return truthy(args[0]), nil
	})

	bf("مطلق", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "مطلق یک آرگومان می‌خواهد", 0)
		}
		switch t := args[0].(type) {
		case int64:
			if t < 0 {
				return -t, nil
			}
			return t, nil
		case float64:
			if t < 0 {
				return -t, nil
			}
			return t, nil
		}
		return nil, e.raise(e.excType, "مطلق یک عدد می‌خواهد", 0)
	})

	bf("گرد", func(args []Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, e.raise(e.excVal, "گرد یک یا دو آرگومان می‌خواهد", 0)
		}
		f, ok := toFloat(args[0])
		if !ok {
			return nil, e.raise(e.excType, "گرد یک عدد می‌خواهد", 0)
		}
		if len(args) == 2 {
			places, err := toIndex(args[1])
			if err != nil {
				return nil, e.raise(e.excType, "گرد: مکان‌های اعشار باید صحیح باشد", 0)
			}
			p := math.Pow(10, float64(places))
			return math.Round(f*p) / p, nil
		}
		return int64(math.Round(f)), nil
	})

	bf("معکوس", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "معکوس یک آرگومان می‌خواهد", 0)
		}
		switch t := args[0].(type) {
		case *List:
			out := make([]Value, len(t.Vals))
			for i, v := range t.Vals {
				out[len(t.Vals)-1-i] = v
			}
			return &List{Vals: out}, nil
		case *Tuple:
			out := make([]Value, len(t.Vals))
			for i, v := range t.Vals {
				out[len(t.Vals)-1-i] = v
			}
			return &Tuple{Vals: out}, nil
		case string:
			rs := []rune(t)
			for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
				rs[i], rs[j] = rs[j], rs[i]
			}
			return string(rs), nil
		}
		return nil, e.raise(e.excType, "معکوس فهرست، قفسه یا متن می‌خواهد", 0)
	})

	bf("شمارش", func(args []Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, e.raise(e.excVal, "شمارش یک یا دو آرگومان می‌خواهد", 0)
		}
		items, err := iterValues(args[0])
		if err != nil {
			return nil, e.raise(e.excType, "شمارش یک دنباله‌پذیر می‌خواهد", 0)
		}
		start := int64(0)
		if len(args) == 2 {
			start, err = toIndex(args[1])
			if err != nil {
				return nil, e.raise(e.excType, "شمارش: شروع باید صحیح باشد", 0)
			}
		}
		out := make([]Value, 0, len(items))
		for i, v := range items {
			out = append(out, &Tuple{Vals: []Value{start + int64(i), v}})
		}
		return &List{Vals: out}, nil
	})

	bf("بقچه", func(args []Value) (Value, error) {
		if len(args) == 0 {
			return nil, e.raise(e.excVal, "بقچه حداقل یک دنباله می‌خواهد", 0)
		}
		seqs := make([][]Value, 0, len(args))
		minLen := -1
		for _, a := range args {
			items, err := iterValues(a)
			if err != nil {
				return nil, e.raise(e.excType, "بقچه دنباله‌پذیر می‌خواهد", 0)
			}
			seqs = append(seqs, items)
			if minLen < 0 || len(items) < minLen {
				minLen = len(items)
			}
		}
		out := make([]Value, 0, minLen)
		for i := 0; i < minLen; i++ {
			tup := make([]Value, 0, len(seqs))
			for _, s := range seqs {
				tup = append(tup, s[i])
			}
			out = append(out, &Tuple{Vals: tup})
		}
		return &List{Vals: out}, nil
	})

	bf("نگاشت", func(args []Value) (Value, error) {
		if len(args) != 2 {
			return nil, e.raise(e.excVal, "نگاشت دو آرگومان می‌خواهد", 0)
		}
		items, err := iterValues(args[1])
		if err != nil {
			return nil, e.raise(e.excType, "نگاشت یک دنباله‌پذیر می‌خواهد", 0)
		}
		out := make([]Value, 0, len(items))
		for _, it := range items {
			r, err := e.call(args[0], []Value{it}, nil, 0)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return &List{Vals: out}, nil
	})

	bf("پالایش", func(args []Value) (Value, error) {
		if len(args) != 2 {
			return nil, e.raise(e.excVal, "پالایش دو آرگومان می‌خواهد", 0)
		}
		items, err := iterValues(args[1])
		if err != nil {
			return nil, e.raise(e.excType, "پالایش یک دنباله‌پذیر می‌خواهد", 0)
		}
		out := make([]Value, 0, len(items))
		for _, it := range items {
			r, err := e.call(args[0], []Value{it}, nil, 0)
			if err != nil {
				return nil, err
			}
			b, err := e.toBool(r, 0)
			if err != nil {
				return nil, err
			}
			if b {
				out = append(out, it)
			}
		}
		return &List{Vals: out}, nil
	})

	bf("ویژگی", func(args []Value) (Value, error) {
		if len(args) != 2 {
			return nil, e.raise(e.excVal, "ویژگی دو آرگومان می‌خواهد", 0)
		}
		name, ok := args[1].(string)
		if !ok {
			return nil, e.raise(e.excType, "ویژگی: نام ویژگی باید متن باشد", 0)
		}
		v, ok2 := e.getAttr(args[0], name)
		if !ok2 {
			return nil, e.raise(e.excType, "ویژگی یافت نشد: "+name, 0)
		}
		return v, nil
	})

	bf("دارد", func(args []Value) (Value, error) {
		if len(args) != 2 {
			return nil, e.raise(e.excVal, "دارد دو آرگومان می‌خواهد", 0)
		}
		name, ok := args[1].(string)
		if !ok {
			return nil, e.raise(e.excType, "دارد: نام ویژگی باید متن باشد", 0)
		}
		_, ok2 := e.getAttr(args[0], name)
		return ok2, nil
	})

	bf("تنظیم‌ویژگی", func(args []Value) (Value, error) {
		if len(args) != 3 {
			return nil, e.raise(e.excVal, "تنظیم‌ویژگی سه آرگومان می‌خواهد", 0)
		}
		name, ok := args[1].(string)
		if !ok {
			return nil, e.raise(e.excType, "تنظیم‌ویژگی: نام ویژگی باید متن باشد", 0)
		}
		if err := e.setAttr(args[0], name, args[2], 0); err != nil {
			return nil, err
		}
		return nil, nil
	})

	bf("اجرا", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "اجرا یک آرگومان می‌خواهد", 0)
		}
		src, ok := args[0].(string)
		if !ok {
			return nil, e.raise(e.excType, "اجرا یک متن می‌خواهد", 0)
		}
		stmts, perr := parser.ParseProgram(src)
		if perr != nil {
			return nil, e.raise(e.excVal, "خطای تجزیه: "+perr.Error(), 0)
		}
		// Evaluate directly in the current (global) env, so definitions made
		// by the executed code are visible to the caller.
		last, err := e.evalTopLevel(stmts, global)
		if err != nil {
			if rs, ok := err.(raiseSignal); ok {
				return nil, &RuntimeError{Line: rs.line, Msg: "استثنای مدیریت‌نشده: " + Stringify(rs.exc)}
			}
			return nil, err
		}
		return last, nil
	})

	bf("مجموعه", func(args []Value) (Value, error) {
		if len(args) == 0 {
			return newSet(), nil
		}
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "مجموعه یک آرگومان می‌خواهد", 0)
		}
		if s, ok := args[0].(*Set); ok {
			return s, nil
		}
		items, err := iterValues(args[0])
		if err != nil {
			return nil, e.raise(e.excType, "مجموعه یک دنباله‌پذیر می‌خواهد", 0)
		}
		s := newSet()
		for _, v := range items {
			s.Add(v)
		}
		return s, nil
	})

	bf("جمع", func(args []Value) (Value, error) {
		var intSum int64
		var floatSum float64
		isFloat := false
		var seq []Value
		if len(args) == 1 {
			items, err := iterValues(args[0])
			if err != nil {
				return nil, e.raise(e.excType, "جمع: آرگومان باید دنباله‌پذیر باشد", 0)
			}
			seq = items
		} else {
			seq = args
		}
		for _, v := range seq {
			if f, ok := toFloat(v); ok {
				if _, isI := v.(int64); !isI {
					isFloat = true
				}
				floatSum += f
				intSum += int64(f)
			}
		}
		if isFloat {
			return floatSum, nil
		}
		return intSum, nil
	})

	bf("کمینه", func(args []Value) (Value, error) {
		var seq []Value
		if len(args) == 1 {
			items, err := iterValues(args[0])
			if err != nil {
				return nil, e.raise(e.excType, "کمینه دنباله‌پذیر می‌خواهد", 0)
			}
			seq = items
		} else {
			seq = args
		}
		if len(seq) == 0 {
			return nil, nil
		}
		best := seq[0]
		for _, v := range seq[1:] {
			c, _ := compare(v, best, func(c int) bool { return c < 0 })
			if c {
				best = v
			}
		}
		return best, nil
	})

	bf("بیشینه", func(args []Value) (Value, error) {
		var seq []Value
		if len(args) == 1 {
			items, err := iterValues(args[0])
			if err != nil {
				return nil, e.raise(e.excType, "بیشینه دنباله‌پذیر می‌خواهد", 0)
			}
			seq = items
		} else {
			seq = args
		}
		if len(seq) == 0 {
			return nil, nil
		}
		best := seq[0]
		for _, v := range seq[1:] {
			c, _ := compare(v, best, func(c int) bool { return c > 0 })
			if c {
				best = v
			}
		}
		return best, nil
	})

	bf("مرتب", func(args []Value) (Value, error) {
		var seq []Value
		if len(args) == 1 {
			items, err := iterValues(args[0])
			if err != nil {
				return nil, e.raise(e.excType, "مرتب دنباله‌پذیر می‌خواهد", 0)
			}
			seq = items
		} else {
			seq = args
		}
		out := append([]Value{}, seq...)
		sort.SliceStable(out, func(i, j int) bool {
			c, _ := compare(out[i], out[j], func(c int) bool { return c < 0 })
			return c
		})
		return &List{Vals: out}, nil
	})

	bf("نوع", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "نوع یک آرگومان می‌خواهد", 0)
		}
		switch args[0].(type) {
		case nil:
			return "تهی", nil
		case int64:
			return "صحیح", nil
		case float64:
			return "اعشاری", nil
		case bool:
			return "بولی", nil
		case string:
			return "متن", nil
		case *List:
			return "فهرست", nil
		case *Set:
			return "مجموعه", nil
		case *Tuple:
			return "قفسه", nil
		case *Dict:
			return "گنجه", nil
		case *Instance:
			return args[0].(*Instance).Class.Name, nil
		case *Class:
			return "گونه", nil
		case *Interface:
			return "رابط", nil
		case *Super:
			return "والد", nil
		case *Generator:
			return "جنریتور", nil
		case *Channel:
			return "کانال", nil
		}
		return "ناشناخته", nil
	})

	bf("هویت", func(args []Value) (Value, error) {
		if len(args) != 1 {
			return nil, e.raise(e.excVal, "هویت یک آرگومان می‌خواهد", 0)
		}
		// Only reference types have a stable identity. For value types
		// (صحیح/اعشاری/متن/بولی/تهی) there is no meaningful object identity,
		// so raising خطای‌نوع is cleaner than returning a synthetic address.
		switch args[0].(type) {
		case *Instance, *List, *Dict, *Function, *Class, *Channel, *Module, *Generator, *Set, *Tuple, *Builtin:
			return fmt.Sprintf("%p", args[0]), nil
		}
		return nil, e.raise(e.excType, "هویت فقط برای مقادیر مرجع تعریف شده است", 0)
	})
}
