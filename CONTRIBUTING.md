# Contributing to Kolang

Thanks for your interest in contributing to Kolang (کلنگ)! Contributions of all kinds are welcome — bug fixes, new features, documentation, tests, and example programs.

## Prerequisites

- [Go](https://go.dev) 1.27 or later
- [git](https://git-scm.com/)

## Setup

```bash
git clone https://github.com/faralidev/kolang.git
cd kolang
go build -o kolang ./cmd/kolang
go test ./...
```

## Development Workflow

1. **Fork and branch** — fork the repository on GitHub, then create a feature branch:
   ```bash
   git checkout -b fix/your-branch-name
   ```
2. **Make your changes** — keep them focused on one issue or feature.
3. **Run the full test suite with the race detector**:
   ```bash
   go test -race -count=1 ./...
   ```
4. **Verify all examples still run**:
   ```bash
   go run ./cmd/kolang examples/hello.kolang
   ```
5. **Commit** with a clear, descriptive message and **submit a pull request** to the `main` branch.

## Code Style

- Run `gofmt` on all Go files before committing (`gofmt -w .`).
- Add `godoc`-style comments to all exported symbols.
- Keep Persian keywords and error messages consistent with the [SPEC](SPEC.html).
- New language features should be accompanied by tests and, ideally, an example program in `examples/`.

## Project Structure

```
cmd/kolang/          CLI entry point (run file, -c flag, REPL)
internal/token/      Token definitions
internal/lexer/      Lexer (tokenizer)
internal/ast/        Abstract syntax tree definitions
internal/parser/     Parser (source → AST)
internal/eval/       Tree-walking interpreter and builtins
examples/            Example programs written in Kolang
```

## Reporting Bugs

- Open a [GitHub Issue](https://github.com/faralidev/kolang/issues) with the **Kolang code** that triggers the bug and the **expected vs. actual** output.
- Include your OS/Go version and the commit you are running, if known.

## Suggesting Features

- Open a [GitHub Issue](https://github.com/faralidev/kolang/issues) with the **Feature** label.
- Describe the motivation, a concrete usage example, and the expected behavior.
- For larger changes, consider opening a discussion first to align on the design.

## Testing

```bash
go test -race -count=1 ./...            # full suite, race detector
go run ./cmd/kolang examples/hello.kolang
```

Always run the full suite before pushing — a pull request that breaks the race-clean test suite will need to be fixed before it can be merged.