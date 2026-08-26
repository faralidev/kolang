package eval

import (
	"fmt"
	"math"
	"sync"
)

// mathModule builds the native ریاضی (math) module.
func mathModule() *Module {
	m := &Module{Name: "ریاضی", Members: map[string]Value{}}

	num1 := func(name string, fn func(float64) float64) {
		m.Members[name] = &Builtin{Name: name, Fn: func(args []Value) (Value, error) {
			if len(args) != 1 {
				return nil, &RuntimeError{Line: 0, Msg: name + " یک آرگومان می‌خواهد"}
			}
			f, ok := toFloat(args[0])
			if !ok {
				return nil, &RuntimeError{Line: 0, Msg: name + " یک عدد می‌خواهد"}
			}
			return fn(f), nil
		}}
	}
	num2 := func(name string, fn func(float64, float64) float64) {
		m.Members[name] = &Builtin{Name: name, Fn: func(args []Value) (Value, error) {
			if len(args) != 2 {
				return nil, &RuntimeError{Line: 0, Msg: name + " دو آرگومان می‌خواهد"}
			}
			a, ok := toFloat(args[0])
			if !ok {
				return nil, &RuntimeError{Line: 0, Msg: name + " عدد می‌خواهد"}
			}
			b, ok := toFloat(args[1])
			if !ok {
				return nil, &RuntimeError{Line: 0, Msg: name + " عدد می‌خواهد"}
			}
			return fn(a, b), nil
		}}
	}
	intRes := func(name string, fn func(float64) float64) {
		m.Members[name] = &Builtin{Name: name, Fn: func(args []Value) (Value, error) {
			if len(args) != 1 {
				return nil, &RuntimeError{Line: 0, Msg: name + " یک آرگومان می‌خواهد"}
			}
			f, ok := toFloat(args[0])
			if !ok {
				return nil, &RuntimeError{Line: 0, Msg: name + " یک عدد می‌خواهد"}
			}
			r := fn(f)
			if r == math.Trunc(r) {
				return int64(r), nil
			}
			return r, nil
		}}
	}

	num1("جذر", math.Sqrt)
	num1("سینوس", math.Sin)
	num1("کسینوس", math.Cos)
	num1("تانژانت", math.Tan)
	num1("قوسسینوس", math.Asin)
	num1("قوسکسینوس", math.Acos)
	num1("قوستانژانت", math.Atan)
	num2("توان", math.Pow)
	num1("قدرمطلق", math.Abs)
	num1("لاگ", math.Log)
	num1("لاگ۱۰", math.Log10)
	num1("لاگ۲", math.Log2)
	num1("تابعنما", math.Exp)

	intRes("کف", math.Floor)
	intRes("سقف", math.Ceil)
	intRes("گرد", math.Round)

	m.Members["پی"] = math.Pi
	m.Members["ای"] = math.E

	return m
}

// moduleMap maps Persian module names to their loader.
var moduleLoaders = map[string]func() *Module{
	"ریاضی": mathModule,
}

// importModule implements «ریاضی بیار».
func (e *Eval) importModule(name string, env *Env) error {
	m, err := e.loadModule(name)
	if err != nil {
		return err
	}
	env.Set(name, m)
	return nil
}

// importFrom implements «از ریاضی جذر [بانام ریشه] بیار».
func (e *Eval) importFrom(module, name, alias string, env *Env) error {
	m, err := e.loadModule(module)
	if err != nil {
		return err
	}
	v, ok := m.Members[name]
	if !ok {
		return e.raise(e.excVal, fmt.Sprintf("ماژول «%s» عضو «%s» را ندارد", module, name), 0)
	}
	env.Set(alias, v)
	return nil
}

// moduleCache caches built modules so repeated «X بیار» reuse one *Module
// instance instead of rebuilding it on every import.
var moduleCache sync.Map

// loadModule returns the module for a Persian name, building it once via its
// registered loader and caching the result (moduleCache). Unimplemented
// modules raise خطای‌مقدار.
func (e *Eval) loadModule(name string) (*Module, error) {
	if cached, ok := moduleCache.Load(name); ok {
		return cached.(*Module), nil
	}
	if loader, ok := moduleLoaders[name]; ok {
		m := loader()
		moduleCache.Store(name, m)
		return m, nil
	}
	return nil, e.raise(e.excVal, fmt.Sprintf("ماژول «%s» هنوز پیاده‌سازی نشده است", name), 0)
}