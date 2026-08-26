// Command kolang is the CLI entry point for the Kolang (کلنگ) interpreter.
//
// It accepts a source file path, the flags -c <code> and repl, and prints
// program output to stdout and errors to stderr.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/faralidev/kolang/internal/eval"
	"github.com/faralidev/kolang/internal/parser"
)

// version is the Kolang release version reported by -version.
const version = "0.0.1"

// main parses the command line and dispatches to the REPL, -c inline code, or
// a source file, exiting with status 1 if the command fails.
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses the command line and dispatches to the REPL, -c inline code, or
// a source file. It returns an error that main reports on stderr; a nil return
// means the command completed normally.
func run(args []string) error {
	if len(args) == 0 {
		// No arguments: start the interactive REPL.
		runRepl()
		return nil
	}
	switch args[0] {
	case "-version", "--version":
		printVersion()
		return nil
	case "-h", "-help", "--help":
		printUsage()
		return nil
	case "repl":
		runRepl()
		return nil
	case "-c":
		if len(args) < 2 {
			return errors.New("kolang -c به کد نیاز دارد")
		}
		return runSource(args[1])
	case "compile":
		return errors.New("کامپایل به .pyc هنوز پیاده‌سازی نشده است")
	case "fmt", "vet":
		return fmt.Errorf("kolang %s هنوز پیاده‌سازی نشده است", args[0])
	default:
		// treat as a file path
		src, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("نمی‌توان فایل را خواند: %v", err)
		}
		return runSource(string(src))
	}
}

// printVersion writes version information to stdout.
func printVersion() {
	fmt.Printf("کلنگ نسخه ۰.۰.۱ (kolang %s)\n", version)
	fmt.Println("github.com/faralidev/kolang")
	fmt.Printf("Go %s\n", runtime.Version())
}

// printUsage writes the Persian/English usage help to stdout.
func printUsage() {
	fmt.Print(`کلنگ — زبان برنامه‌نویسی فارسی
Kolang — Persian Programming Language

استفاده:
  kolang <فایل>           اجرای یک فایل کلنگ
  kolang -c <کد>          اجرای کد دلخواه
  kolang                  حالت تعاملی (REPL)
  kolang -version         نمایش نسخه
  kolang -help            نمایش این راهنما

Usage:
  kolang <file>           Run a Kolang file
  kolang -c <code>        Run inline code
  kolang                  Start REPL
  kolang -version         Show version
  kolang -help            Show this help
`)
}

// runSource parses and evaluates source code, reporting errors.
func runSource(src string) error {
	stmts, err := parser.ParseProgram(src)
	if err != nil {
		return err
	}
	ev := eval.New(os.Stdout)
	return ev.EvalProgram(stmts)
}

// runRepl runs an interactive read-eval-print loop. A single *Eval and a
// single global *Env are hoisted out of the per-line loop so definitions and
// assignments persist across lines. Input that opens a block (trailing ':') or
// leaves a paren/bracket unclosed continues reading until a blank line submits
// the accumulated source.
func runRepl() {
	reader := bufio.NewReader(os.Stdin)
	ev := eval.New(os.Stdout)
	global := ev.GlobalEnv()
	fmt.Println("کلنگ v0.1 — محیط تعاملی (برای خروج: خروج را بنویسید)")
	var acc []string
	for {
		if len(acc) == 0 {
			fmt.Print("کلنگ> ")
		} else {
			fmt.Print("... ")
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF: flush any pending incomplete input, then exit.
			if len(acc) > 0 {
				runReplBuffer(ev, global, strings.Join(acc, "\n"), true)
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// A blank line submits whatever was accumulated.
			if len(acc) > 0 {
				runReplBuffer(ev, global, strings.Join(acc, "\n"), true)
				acc = nil
			}
			continue
		}
		if trimmed == "خروج" && len(acc) == 0 {
			return
		}
		acc = append(acc, line)
		if !runReplBuffer(ev, global, strings.Join(acc, "\n"), false) {
			continue // incomplete input: keep reading lines
		}
		acc = nil
	}
}

// runReplBuffer evaluates the accumulated REPL input against the shared global
// env. force reports whether the input is being submitted (e.g. on a blank
// line) regardless of completeness. It returns true if the buffer was consumed
// (evaluated or errored) and false if more input is required to complete the
// current construct.
func runReplBuffer(ev *eval.Eval, global *eval.Env, src string, force bool) bool {
	// Try to parse as a single expression first so we can print its value.
	if expr, err := parser.ParseSingleExpr(src); err == nil {
		val, rerr := ev.EvalReplExpr(expr, global)
		if rerr != nil {
			fmt.Fprintln(os.Stderr, rerr)
		} else if val != nil {
			fmt.Println(eval.Stringify(val))
		}
		return true
	}
	stmts, perr := parser.ParseProgram(src)
	if perr != nil {
		if !force && needsMoreInput(src) {
			return false
		}
		fmt.Fprintln(os.Stderr, perr)
		return true
	}
	if _, err := ev.EvalReplStmts(stmts, global); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return true
}

// needsMoreInput reports whether the accumulated REPL source looks incomplete:
// the last non-blank line ends with ':' (a block opener such as تعریف/اگر/برای/
// گونه), or the source has unbalanced '(' / '[' delimiters outside of «...»
// strings.
func needsMoreInput(src string) bool {
	lines := strings.Split(src, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if strings.HasSuffix(t, ":") {
			return true
		}
		break
	}
	open := 0
	inStr := false
	for _, r := range src {
		switch {
		case r == '«':
			inStr = true
		case r == '»':
			inStr = false
		case inStr:
		case r == '(' || r == '[':
			open++
		case r == ')' || r == ']':
			open--
		}
	}
	return open > 0
}
