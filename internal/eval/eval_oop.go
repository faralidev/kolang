package eval

import (
	"fmt"

	"github.com/faralidev/kolang/internal/ast"
)

// Class is a runtime class value (گونه). It is callable: calling it
// instantiates an Instance. Methods and the constructor are stored here.
type Class struct {
	Name        string
	Parent      *Class
	ParentName  string
	Methods     map[string]*Function
	Constructor *Function
	Env         *Env
	// ExBuiltin marks the pre-registered built-in exception classes. When set
	// (here or on any ancestor), instantiating the class builds an exception
	// instance with a پیام (message) field, and the instance stringifies to
	// its message.
	ExBuiltin bool
}

func (c *Class) String() string { return "<گونه " + c.Name + ">" }

// Frame records the execution context of a method call: the receiver instance
// (self) and the class where the currently-executing method was DEFINED. The
// defining class is what «والد» (super) resolves relative to, so that in a
// 3-level hierarchy each method's super goes one level up from where it was
// defined, not from the instance's (leaf) class.
type Frame struct {
	Inst  *Instance
	Class *Class
}

// lookupMethod returns the method named `name` defined on this class or any
// ancestor, along with the class on which it was actually defined. The
// constructor (ساخت) is also a method and is found via Methods.
func (c *Class) lookupMethod(name string) (*Function, *Class) {
	for k := c; k != nil; k = k.Parent {
		if m, ok := k.Methods[name]; ok {
			return m, k
		}
	}
	return nil, nil
}

// hasMethod reports whether the class (or an ancestor) defines the given method.
func (c *Class) hasMethod(name string) bool {
	m, _ := c.lookupMethod(name)
	return m != nil
}

// Instance is a runtime instance of a Class.
type Instance struct {
	Class  *Class
	Fields map[string]Value
}

// String renders an instance: exception instances stringify to their پیام
// message (or class name), ordinary instances to «<نمونه ClassName>».
func (i *Instance) String() string {
	if i.Class != nil {
		if isExceptionClass(i.Class) {
			if m, ok := i.Fields["پیام"]; ok {
				return Stringify(m)
			}
			return i.Class.Name
		}
		return "<نمونه " + i.Class.Name + ">"
	}
	return "<نمونه>"
}

// Interface is a runtime interface value (رابط). It holds the set of required
// method names. Satisfaction is structural (duck typing).
type Interface struct {
	Name        string
	MethodNames map[string]bool
}

func (i *Interface) String() string { return "<رابط " + i.Name + ">" }

// Super is a super()/parent proxy. A method called on it is looked up starting
// at Klass (the parent class) rather than the instance's class.
type Super struct {
	Obj   *Instance
	Klass *Class
}

func (s *Super) String() string { return "<والد " + s.Klass.Name + ">" }

// evalClassDef builds a Class value from a ClassDef statement, wires up the
// parent, stores methods, evaluates class-level fields, and (optionally)
// verifies explicit رهی interface satisfaction.
func (e *Eval) evalClassDef(st *ast.ClassDef, env *Env) (Value, error) {
	classEnv := NewEnv(env)
	cls := &Class{
		Name:       st.Name,
		ParentName: st.Parent,
		Methods:    map[string]*Function{},
		Env:        classEnv,
	}

	// Resolve parent (single inheritance).
	if st.Parent != "" {
		pv, ok := env.Get(st.Parent)
		if !ok {
			return nil, &RuntimeError{Line: st.L, Msg: "کلاس والد یافت نشد: " + st.Parent}
		}
		p, ok := pv.(*Class)
		if !ok {
			return nil, &RuntimeError{Line: st.L, Msg: st.Parent + " یک گونه نیست"}
		}
		cls.Parent = p
	}

	// Register the class in the enclosing scope before its body runs so
	// methods / fields may reference it.
	env.Set(st.Name, cls)

	// Process the class body.
	for _, s := range st.Body {
		m, ok := s.(*ast.DefStmt)
		if !ok {
			// class-level field or other statement: evaluate in class env.
			if _, err := e.evalStmt(s, classEnv); err != nil {
				return nil, err
			}
			continue
		}
		fn := &Function{Name: m.Name, Params: m.Params, Body: m.Body, Env: classEnv, RetType: m.RetType}
		ensureSelf(fn)
		// Apply any decorators (پوشش) attached to the method, mirroring how
		// module-level تعریف handles them.
		if len(m.Decorators) > 0 {
			dv, err := e.applyDecorators(fn, m.Decorators, classEnv)
			if err != nil {
				return nil, err
			}
			// A method must remain a *Function: classes store methods in a
			// *Function slot and dispatch them via callFunction, so a
			// decorator that returns anything else (even a callable *Builtin
			// or *Class) cannot be used here. Raise a type error instead of
			// silently dropping the decoration.
			dfn, ok := dv.(*Function)
			if !ok {
				return nil, e.raise(e.excType, "پوشش باید یک فراخوانی برگرداند", m.L)
			}
			fn = dfn
		}
		if m.Name == "ساخت" {
			cls.Constructor = fn
			cls.Methods["ساخت"] = fn
		} else {
			cls.Methods[m.Name] = fn
		}
	}

	// Optional explicit interface (رهی) — verify all methods exist.
	if st.Implements != "" {
		iv, ok := env.Get(st.Implements)
		if !ok {
			return nil, &RuntimeError{Line: st.L, Msg: "رابط یافت نشد: " + st.Implements}
		}
		ifc, ok := iv.(*Interface)
		if !ok {
			return nil, &RuntimeError{Line: st.L, Msg: st.Implements + " یک رابط نیست"}
		}
		for name := range ifc.MethodNames {
			if !cls.hasMethod(name) {
				return nil, &RuntimeError{Line: st.L, Msg: fmt.Sprintf("گونه %s متد موردنیاز رابط %s را ندارد: %s", cls.Name, ifc.Name, name)}
			}
		}
	}
	return cls, nil
}

// evalInterfaceDef creates an Interface value from an InterfaceDef statement.
func (e *Eval) evalInterfaceDef(st *ast.InterfaceDef, env *Env) (Value, error) {
	ifc := &Interface{Name: st.Name, MethodNames: map[string]bool{}}
	for _, m := range st.Methods {
		ifc.MethodNames[m.Name] = true
	}
	env.Set(st.Name, ifc)
	return ifc, nil
}

// instantiate creates an Instance of the class, then runs its constructor
// (ساخت) if one exists (found on the class or an ancestor).
func (e *Eval) instantiate(c *Class, args []Value, kwargs map[string]Value, line int) (Value, error) {
	inst := &Instance{Class: c, Fields: map[string]Value{}}

	var ctor *Function
	var ctorClass *Class
	for k := c; k != nil; k = k.Parent {
		if k.Constructor != nil {
			ctor = k.Constructor
			ctorClass = k
			break
		}
	}
	if ctor != nil {
		e.pushFrame(&Frame{Inst: inst, Class: ctorClass})
		_, err := e.callFunction(ctor, append([]Value{inst}, args...), kwargs, line)
		e.popFrame()
		if err != nil {
			if _, isCtrl := err.(returnSignal); isCtrl {
				return inst, nil
			}
			return nil, err
		}
	}

	// Defensive fallback: an exception class should always expose پیام, even if
	// no constructor was found (e.g. a subclass chain that skipped the built-in
	// ساخت). The built-in خطا base class normally provides that constructor.
	if isExceptionClass(c) && ctor == nil {
		var msg string
		if len(args) > 0 {
			if s, ok := args[0].(string); ok {
				msg = s
			} else {
				msg = Stringify(args[0])
			}
		}
		inst.Fields["پیام"] = msg
	}
	return inst, nil
}

// ensureSelf makes sure a method has خود (self) as its first parameter. The
// spec examples list خود explicitly; if the source omits it, we inject it.
func ensureSelf(fn *Function) {
	if len(fn.Params) == 0 || fn.Params[0].Name != "خود" {
		fn.Params = append([]*ast.Param{{Name: "خود"}}, fn.Params...)
	}
}

func (e *Eval) pushFrame(f *Frame) { e.frames = append(e.frames, f) }

func (e *Eval) popFrame() { e.frames = e.frames[:len(e.frames)-1] }

// currentSelf returns the innermost receiver instance (self).
func (e *Eval) currentSelf() *Instance {
	if len(e.frames) == 0 {
		return nil
	}
	return e.frames[len(e.frames)-1].Inst
}

// currentMethodClass returns the class in which the innermost method was
// DEFINED (the frame's Class), or nil if no method is executing.
func (e *Eval) currentMethodClass() *Class {
	if len(e.frames) == 0 {
		return nil
	}
	return e.frames[len(e.frames)-1].Class
}

// superMarker builds a bare super() proxy (Obj == nil) from the innermost
// enclosing method's self, so that «والد()» and «ساختِ(...)والد()» resolve the
// parent of the current method's defining class. It returns nil Obj so that a
// following method call uses the explicitly-passed arguments (which include خود).
func (e *Eval) superMarker(line int) (Value, error) {
	if len(e.frames) == 0 {
		return nil, &RuntimeError{Line: line, Msg: "والد خارج از متد مجاز نیست"}
	}
	cur := e.frames[len(e.frames)-1]
	if cur.Class.Parent == nil {
		return nil, &RuntimeError{Line: line, Msg: "کلاس والد ندارد: " + cur.Class.Name}
	}
	return &Super{Obj: nil, Klass: cur.Class.Parent}, nil
}