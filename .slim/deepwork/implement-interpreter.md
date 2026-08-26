# Deepwork: Implement Kolang Interpreter (Phase 1)

**Started:** 2026-08-21
**Goal:** Build a working Kolang interpreter in Go covering all of Phase 1 (v0.1 → v0.7): lexer, parser, AST, tree-walking evaluator, builtins, modules, OOP (گونه/رابط), exceptions (بپا/بگیر), generators (ای بساز), decorators (پوشش), gradual typing, concurrency (تارک/کانال), and CLI. Per SPEC.html §15-16.

**Language:** Go (per spec §15.4). Go not yet installed — will install via `brew install go`.
**Scope:** Full Phase 1 (user decision). Tree-walking evaluator for v0.1-v0.6 (spec §16.1 line 1230); .pyc compilation is v0.7 (may be stretch).

## Confirmed Research Context (from ora-1 assessment)
- Spec is internally consistent as of v10 (all contradictions fixed).
- Ezafe method-call grammar `methodِ(args)receiver` is LL(2) not LL(1) — spec claim of LL(1) is slightly wrong; implement with recursive descent (handles LL(2) fine).
- `/` decimal separator dropped (v10); channels use `<<`/`>>` BIDI-stable (v10); `!=` removed, `نباشد` sole negator (v10); file.read = `بخوانِ()ف` (v10); exception names use ZWNJ (v10).
- `بگیر` = input + except (context-disambiguated by trailing `:` and exception type).
- `بنویس` allows verb-final, pipe-target, and callable forms.
- Copula `باشد`/`نباشد` REQUIRED after comparisons.
- No implicit truthiness except bare `درست`/`غلط`.

## Plan: 6 Implementation Phases

Each phase ends with an @oracle review gate before proceeding.

### Phase 1: Foundation — Lexer + minimal parser/evaluator (v0.1 core)
**Owner:** @fixer (bounded, mechanical, well-specified)
**Scope:**
- Go module setup (`go.mod`, `cmd/kolang/main.go`)
- `internal/lexer/`: tokenize Persian identifiers, Persian digits (normalize to Latin), ZWNJ, kasra (ezafe) as operator, backtick strings with `{...}` interp, `« »` comments, `«« »»` block comments, indentation tokens (Python-style INDENT/DEDENT), all operators (`== < > <= >= + - * ÷ ** ÷/ % = += -= *= ÷= **= ÷/= %=`), `<< >>` channels, `->` `|>`, keywords.
- `internal/ast/`: AST node types for the v0.1 subset (Program, Let/Assign, If, While, For, FunctionDef, Return, Call, BinaryOp, UnaryOp, Identifier, Number/String/Bool/None literals, MemberAccess ezafe, Imperative verbs بنویس/بگیر/برگردان/بیار, Import).
- `internal/parser/`: recursive-descent parser for v0.1 subset (assignment, if/باشد, while, for, function def/call, imperatives, ezafe member access, arithmetic precedence).
- `internal/eval/`: tree-walking evaluator + environment/scope chain.
- Builtins: `بنویس`, `بگیر` (input), `طول`, `بازه`, `متن`, `صحیح`, `اعشاری`, `فهرست`, `گنجه`, `قفسه`, `بازه`.
- Module imports: `ریاضی بیار` / `از ریاضی جذر بیار` (map to Python modules via a static name table — since v1 runs on... wait, we're in Go, not Python. Module interop needs a Go-native approach: implement core modules natively in Go, or shell out. For v0.1, implement `ریاضی` (math) natively in Go mapping functions.)
- CLI: `kolang script.kolang`, `kolang -c "..."`, `kolang repl`.
- **Test:** `سلام دنیا` بنویس works; fibonacci works; basic if/while/for work.

**Oracle review gate #1:** Verify lexer correctness (Persian digit normalization, ezafe tokenization, BIDI/RTL handling, indentation), parser soundness (precedence, ezafe grammar LL(2) handled), evaluator semantics match spec. Critical foundation — must be solid before building on it.

### Phase 2: OOP + Interfaces (v0.2)
**Owner:** @fixer
**Scope:** `گونه` (class), `وارث` (inheritance), `خود` (self), `والد` (super), methods via ezafe, `رابط` (interface) structural typing, `رهی` (implements).
**Oracle review gate #2:** Verify class semantics, method resolution, ezafe method calls, structural interface matching.

### Phase 3: Errors + Exceptions + Defer (v0.3)
**Owner:** @fixer
**Scope:** Multiple return values, explicit `خطا` type, `بپا/خطا بگیر:/درنهایت:/x بده`, `تأخیری` (defer postfix), error-check idiom `اگر خط == تهی نباشد:`.
**Oracle review gate #3:** Verify exception flow, defer execution order (LIFO), multiple return semantics, error type.

### Phase 4: Generators + Decorators (v0.4)
**Owner:** @fixer
**Scope:** `ای بساز` (yield, verb-final), `بساز‌از` (yield from), `پوشش` (decorator, with args).
**Oracle review gate #4:** Verify generator iteration semantics, yield-from, decorator wrapping.

### Phase 5: Gradual Typing + Comprehensions + Pipes (v0.5 + misc)
**Owner:** @fixer
**Scope:** Type annotations (`سن: صحیح = ۲۵`, param/return types), runtime type checking, list/dict/set comprehensions, pipe `|>`, ternary.
**Oracle review gate #5:** Verify type check semantics, comprehension evaluation, pipe desugaring.

### Phase 6: Concurrency — تارک + کانال (v0.6)
**Owner:** @fixer
**Scope:** `برو` (goroutine → Go goroutine), `کانال(...)`, `<<` send, `>>` recv, `ch ببند`, `بسته‌استِ ch`.
**Oracle review gate #6 (final):** Verify goroutine spawn, channel send/recv blocking, close semantics, full integration.

### v0.7 (.pyc compilation) — STRETCH / DEFERRED
Likely out of scope for this session. Tree-walking evaluator is sufficient for v0.1-v0.6. Note in summary if reached.

## Phase Execution Rules
- Sequential phases (each builds on prior). No parallelism between phases.
- Within a phase, parallel @fixer lanes only where write-scope doesn't conflict (e.g., Phase 1 could split lexer vs ast vs parser, but they're tightly coupled — likely single lane).
- Oracle review is a hard gate: do not start Phase N+1 until Phase N review passes or is triaged.
- Update this file after each phase completion + review.

## Status
- [x] Phase 1: Foundation — PASS (oracle review passed, 3 critical + 4 important bugs fixed)
- [x] Phase 2: OOP — PASS (oracle review passed, super/MRO bug + مثل + class fields fixed)
- [x] Phase 3: Errors/Exceptions/Defer — PASS (oracle review passed, defer-raise + dict keys + مثل-in-class + type errors fixed)
- [x] Phase 4: Generators/Decorators — PASS (oracle review passed; decorator-on-method + self-call fixed, generator close + verb-final بساز‌از + StopIteration registered)
- [x] Phase 5: Typing/Comprehensions/Pipes — PASS (oracle review passed; reassignment typing + error-value idiom + generator close + pipe-to-method fixed)
- [x] Phase 6: Concurrency — PASS (oracle review passed; closure-mutation Env.Assign fix applied directly)
- [x] PHASE 1 COMPLETE — v0.1-v0.6 all implemented, 32 examples pass, go test -race clean
- [ ] Phase 6: Concurrency

## Phase 1 Notes (oracle review summary)
- Verdict: PASS-WITH-FIXES → fixed → PASS
- Critical bugs fixed: ÷ true division (always float), ** negative exponent (float path), parseReceiver postfix consumption (ezafe on indexed/called receivers)
- Important fixes: طولِ xs returns length value (not builtin), بنویس space separator, x=۱ و ۲ errors (no silent tuple), eval_test.go regression suite added
- Architecture confirmed sound for Phase 2-6. Env/scoping, control-flow signals, Value interface, AST node types all extensible.
- Architectural note for later: generators (Phase 4) in a tree-walker are hard — consider goroutine-based or defer to after bytecode VM. Flagged.
- Spec compliance confirmed: نباشد sole negator, ezafe-before-paren, copula required, no implicit truthiness, all v10 rules.

## Phase 2 Notes (oracle review summary)
- Verdict: NEEDS-REWORK (1 critical) → fixed → PASS
- Critical bug fixed: super/MRO infinite loop in ≥3-level hierarchies. Root cause: `والدِ خود` resolved relative to instance's class, not method-defining class. Fix: frame stack carrying defining class; `lookupMethod` returns `(*Function, *Class)`.
- مثل (pass) implemented (was missing, blocked empty bodies).
- Class-level fields now visible on instances (getAttr falls back to class env chain); class attrs writable (setAttr handles *Class).
- Frame stack refactor: `selfStack []*Instance` → `frames []*Frame{Inst, Class}`. Foundation for Phase 3 defer and Phase 5 global/nonlocal.
- Architecture confirmed ready for Phase 3: exception classes will slot into the class system; frame stack ready for defer lists.
- Deferred: نباشد as in-expression negator (I2), excess-args error (M3), isinstance (Phase 5).
