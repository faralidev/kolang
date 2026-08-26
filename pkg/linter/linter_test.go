package linter_test

import (
	"testing"

	"github.com/faralidev/kolang/pkg/linter"
)

// Smoke test: the public package must lex, parse, and allow type-switching
// over the aliased AST types — proving external tooling can use it without
// touching internal/.
func TestPublicAPIEndToEnd(t *testing.T) {
	src := "سن : صحیح = ۲۵\nبنویس(سن)"

	toks := linter.Lex(src)
	if len(toks) == 0 {
		t.Fatal("Lex returned no tokens")
	}

	stmts, err := linter.ParseProgram(src)
	if err != nil {
		t.Fatalf("ParseProgram failed: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}

	// The public type aliases must be identical to the real types, so a type
	// switch over the aliased names works.
	if _, ok := stmts[0].(*linter.Assign); !ok {
		t.Fatalf("first stmt is %T, want *linter.Assign", stmts[0])
	}
	exprStmt, ok := stmts[1].(*linter.ExprStmt)
	if !ok {
		t.Fatalf("second stmt is %T, want *linter.ExprStmt", stmts[1])
	}
	if _, ok := exprStmt.Expr.(*linter.Call); !ok {
		t.Fatalf("inner expr is %T, want *linter.Call", exprStmt.Expr)
	}

	// A token constant from the public package must compare equal to one
	// produced by the lexer.
	if toks[0].Type != linter.IDENT {
		t.Fatalf("first token type = %q, want IDENT", toks[0].Type)
	}
}
