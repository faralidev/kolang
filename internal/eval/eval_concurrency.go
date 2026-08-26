package eval

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/faralidev/kolang/internal/ast"
)

// Channel is a Kolang channel (کانال). It wraps a Go channel plus a done
// channel that signals closure. A close never races a blocking send: «ch << v»
// selects on both the send and done, so it either delivers or raises a catchable
// خطا when the channel is closed — never deadlocks and never panics.
//
// Unlike a raw Go channel, we never close the underlying ch chan Value. Closure
// is signaled exclusively by closing done. Senders blocked in the select are
// released via done; receivers drain remaining buffered values and then return
// تهی. This is what lets a blocked sender coexist safely with a concurrent
// close.
//
// Concurrency model (v0.6): Kolang adopts Go's CSP model. Channels are the safe
// way to move data between goroutines. `*Instance.Fields` (object state) is NOT
// mutex-guarded and must not be shared across goroutines; use channels instead.
type Channel struct {
	ch     chan Value
	done   chan struct{} // closed by closeChannel to release blocked senders/receivers
	closed atomic.Bool   // set atomically by closeChannel; read by isClosed/sendChannel
}

func (c *Channel) String() string { return "<کانال>" }

// isClosed reports whether the channel has been closed (بسته‌استِ ch).
func (c *Channel) isClosed() bool {
	return c.closed.Load()
}

// evalChannelLit builds a channel value: «کانال(TYPE و SIZE)». The type
// annotation is runtime-ignored (Kolang channels are dynamically typed, so all
// channels wrap a `chan Value`); the size sets the buffer capacity
// (0 = unbuffered).
func (e *Eval) evalChannelLit(ex *ast.ChannelLit, env *Env) (Value, error) {
	size := 0
	if ex.Size != nil {
		sv, err := e.evalExpr(ex.Size, env)
		if err != nil {
			return nil, err
		}
		n, err := toNumber(sv)
		if err != nil {
			return nil, &RuntimeError{Line: ex.L, Msg: "اندازه بافر کانال باید یک عدد صحیح باشد"}
		}
		if n < 0 {
			n = 0
		}
		size = int(n)
	}
	return &Channel{ch: make(chan Value, size), done: make(chan struct{})}, nil
}

// evalGoStmt implements «برو EXPR»: it spawns a goroutine that evaluates EXPR
// (normally a function call) concurrently. The caller does not wait. The
// goroutine runs on its own isolated *Eval (fresh frames/defer stack/generator
// context) so it never races the caller. An uncaught exception inside the
// goroutine is printed to stderr (like a Go panic) and does not crash the
// program.
//
// The goroutine shares the caller's lexical environment (closures). That env is
// RWMutex-guarded, so concurrent reads/writes of variables are safe; but
// *Instance.Fields is NOT guarded — pass data via channels, not shared objects.
// If code still shares an Instance across goroutines, the Go runtime panics
// with "concurrent map ..." the moment two goroutines touch the Fields map.
// That fatal panic is caught here and reported as a catchable-looking خطا so
// the program can diagnose it instead of crashing with a raw Go stack dump.
func (e *Eval) evalGoStmt(st *ast.GoStmt, env *Env) (Value, error) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				if strings.Contains(fmt.Sprint(r), "concurrent map") {
					fmt.Fprintf(os.Stderr, "تارک: خطای هم‌زمانی — دسترسی هم‌زمان به شیء (Instance.Fields) از چند تارک. داده را از طریق کانال منتقل کنید، نه از طریق شیء مشترک: %v\n", r)
				} else {
					fmt.Fprintf(os.Stderr, "تارک: وحشتناک (panic) — %v\n", r)
				}
			}
		}()
		goEval := e.newGoroutineEval()
		_, err := goEval.evalExpr(st.Expr, env)
		if err != nil {
			if rs, ok := err.(raiseSignal); ok {
				fmt.Fprintf(os.Stderr, "تارک: خطای مدیریت‌نشده در خط %d: %s\n", rs.line, Stringify(rs.exc))
			} else if re, ok := err.(*RuntimeError); ok {
				fmt.Fprintf(os.Stderr, "تارک: خطا در خط %d: %s\n", re.Line, re.Msg)
			} else {
				fmt.Fprintf(os.Stderr, "تارک: خطا: %v\n", err)
			}
		}
	}()
	return nil, nil
}

// evalSendStmt implements «ch << value» — send value on the channel. It blocks
// until a receiver is ready (unbuffered) or the buffer has room (buffered).
// Sending on a closed channel raises a catchable خطا.
func (e *Eval) evalSendStmt(st *ast.SendStmt, env *Env) (Value, error) {
	chv, err := e.evalExpr(st.Channel, env)
	if err != nil {
		return nil, err
	}
	val, err := e.evalExpr(st.Value, env)
	if err != nil {
		return nil, err
	}
	ch, ok := chv.(*Channel)
	if !ok {
		return nil, &RuntimeError{Line: st.L, Msg: "ارسال (<<) فقط روی کانال معتبر است"}
	}
	return nil, e.sendChannel(ch, val, st.L)
}

// sendChannel performs the blocking send. It never holds a lock across the
// blocking operation: the send selects on both the channel and the done signal,
// so a concurrent close releases the sender with a catchable خطا instead of
// deadlocking (a sender waiting forever while the closer waits on the same
// lock). The underlying Go channel is never closed, so a send can never panic.
func (e *Eval) sendChannel(ch *Channel, v Value, line int) error {
	if ch.closed.Load() {
		return e.raise(e.excVal, "کانال بسته است — نمی‌توان ارسال کرد", line)
	}
	select {
	case ch.ch <- v:
		return nil
	case <-ch.done:
		return e.raise(e.excVal, "کانال بسته است — نمی‌توان ارسال کرد", line)
	}
}

// recvChannel blocks until a value is available or the channel is closed and
// drained. It returns (value, true) on a receive and (nil, false) once a closed
// channel has been drained. Because the underlying Go channel is never closed,
// receivers explicitly select on done to detect closure.
func (e *Eval) recvChannel(ch *Channel) (Value, bool) {
	select {
	case v, ok := <-ch.ch:
		if !ok {
			return nil, false
		}
		return v, true
	case <-ch.done:
		// Closed: drain any remaining buffered value non-blockingly, then تهی.
		select {
		case v := <-ch.ch:
			return v, true
		default:
			return nil, false
		}
	}
}

// evalRecvExpr implements «>>ch» — receive a value from the channel. It blocks
// until a value is available. On a closed-and-drained channel it returns تهی
// (nil), matching Go's zero-value-on-closed-channel behavior.
func (e *Eval) evalRecvExpr(ex *ast.RecvExpr, env *Env) (Value, error) {
	chv, err := e.evalExpr(ex.Channel, env)
	if err != nil {
		return nil, err
	}
	ch, ok := chv.(*Channel)
	if !ok {
		return nil, &RuntimeError{Line: ex.L, Msg: "دریافت (>>) فقط برای یک کانال معتبر است"}
	}
	v, _ := e.recvChannel(ch)
	return v, nil
}

// evalCloseStmt implements «ch ببند» — close the channel. Subsequent sends
// raise a خطا; receives drain the remaining buffered values, then return تهی.
// Closing an already-closed channel raises a catchable خطا.
func (e *Eval) evalCloseStmt(st *ast.CloseStmt, env *Env) (Value, error) {
	chv, err := e.evalExpr(st.Channel, env)
	if err != nil {
		return nil, err
	}
	ch, ok := chv.(*Channel)
	if !ok {
		return nil, &RuntimeError{Line: st.L, Msg: "بستن (ببند) فقط برای یک کانال معتبر است"}
	}
	return nil, e.closeChannel(ch, st.L)
}

// closeChannel atomically marks the channel closed and releases any blocked
// senders/receivers by closing the done channel. It does NOT close the
// underlying Go channel (doing so would make an in-flight select-send panic);
// the done channel is the single close signal. A close can never block behind
// a send, so a blocked sender + concurrent close cannot deadlock.
func (e *Eval) closeChannel(ch *Channel, line int) error {
	if !ch.closed.CompareAndSwap(false, true) {
		return e.raise(e.excVal, "کانال از قبل بسته شده است", line)
	}
	close(ch.done)
	return nil
}

// evalChannelForIn iterates a channel via «برای v در ch:», receiving values
// until the channel is closed and drained (like Go's «for v := range ch»). If
// the channel is never closed, the loop blocks forever — the sender must close
// it. A break stops the loop early.
func (e *Eval) evalChannelForIn(ch *Channel, st *ast.ForIn, env *Env) (Value, error) {
	for {
		v, ok := e.recvChannel(ch)
		if !ok {
			return nil, nil
		}
		if err := e.forInSetVars(st, env, v); err != nil {
			return nil, err
		}
		_, err := e.evalBlock(st.Body.Stmts, env)
		if err != nil {
			switch err.(type) {
			case breakSignal:
				return nil, nil
			case continueSignal:
				continue
			default:
				return nil, err
			}
		}
	}
}