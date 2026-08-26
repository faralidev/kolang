//go:build js && wasm

// Package main is the WebAssembly entry point for the Kolang playground.
// It registers a runKolang function callable from JavaScript that parses
// and evaluates Kolang source code, returning output and errors.
package main

import (
	"bytes"
	"syscall/js"

	"github.com/faralidev/kolang/internal/eval"
	"github.com/faralidev/kolang/internal/parser"
)

func main() {
	js.Global().Set("runKolang", js.FuncOf(runKolang))
	select {}
}

// runKolang is called from JavaScript as: runKolang(code) -> Promise
// The returned Promise resolves to { ok: bool, output: string, error: string }.
func runKolang(this js.Value, args []js.Value) interface{} {
	code := ""
	if len(args) > 0 {
		code = args[0].String()
	}

	// We return a Promise so JS stays async. The evaluation runs on a
	// goroutine so the Go scheduler can service other goroutines (e.g.
	// برو/تارک goroutines spawned by the program).
	handler := js.FuncOf(func(this js.Value, promiseArgs []js.Value) interface{} {
		resolve := promiseArgs[0]
		go func() {
			var buf bytes.Buffer
			ev := eval.New(&buf)

			stmts, err := parser.ParseProgram(code)
			if err != nil {
				resolve.Invoke(js.ValueOf(map[string]interface{}{
					"ok":     false,
					"output": buf.String(),
					"error":  err.Error(),
				}))
				return
			}

			if err := ev.EvalProgram(stmts); err != nil {
				resolve.Invoke(js.ValueOf(map[string]interface{}{
					"ok":     false,
					"output": buf.String(),
					"error":  err.Error(),
				}))
				return
			}

			resolve.Invoke(js.ValueOf(map[string]interface{}{
				"ok":     true,
				"output": buf.String(),
				"error":  "",
			}))
		}()
		return nil
	})

	// Create and return a Promise: new Promise(handler)
	promiseConstructor := js.Global().Get("Promise")
	return promiseConstructor.New(handler)
}
