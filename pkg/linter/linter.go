// Package linter exposes the Kolang lexer, parser, AST, and token types as a
// public API for external tooling — notably the standalone kolang-linter.
//
// The interpreter's own lexer/parser/ast/token packages live under internal/
// and are therefore not importable from outside this module. This package
// re-exports exactly the symbols an external linter needs, using type aliases
// so that values produced here are identical to the internal types (type
// switches and field access in the linter work unchanged).
//
// The entry points are [Lex] and [ParseProgram]; the type aliases below cover
// every AST node and token type the linter consumes.
package linter

import (
	"github.com/faralidev/kolang/internal/ast"
	"github.com/faralidev/kolang/internal/lexer"
	"github.com/faralidev/kolang/internal/parser"
	"github.com/faralidev/kolang/internal/token"
)

// --- Entry points ---

// Lex tokenizes src, returning the full token stream (including EOF).
func Lex(src string) []Token { return lexer.Lex(src) }

// ParseProgram parses src into a statement list. The returned error is nil on
// success; on failure the returned slice may be partial or nil.
func ParseProgram(src string) ([]Stmt, error) { return parser.ParseProgram(src) }

// --- Token (re-exported as aliases) ---

// TokenType is the kind of a token, aliased to the internal token.Type.
type TokenType = token.Type

// Token is a single lexical token, aliased to the internal token.Token.
type Token = token.Token

// Token type constants used by external linters. These are aliases of the
// internal constants and may be compared directly.
const (
	ILLEGAL     = token.ILLEGAL
	EOF         = token.EOF
	NEWLINE     = token.NEWLINE
	INDENT      = token.INDENT
	DEDENT      = token.DEDENT
	IDENT       = token.IDENT
	INT         = token.INT
	FLOAT       = token.FLOAT
	STRING      = token.STRING
	ASSIGN      = token.ASSIGN
	PLUS        = token.PLUS
	MINUS       = token.MINUS
	STAR        = token.STAR
	DIV         = token.DIV
	FLOORDIV    = token.FLOORDIV
	PERCENT     = token.PERCENT
	POW         = token.POW
	PLUS_EQ     = token.PLUS_EQ
	MINUS_EQ    = token.MINUS_EQ
	STAR_EQ     = token.STAR_EQ
	DIV_EQ      = token.DIV_EQ
	FLOORDIV_EQ = token.FLOORDIV_EQ
	POW_EQ      = token.POW_EQ
	PERCENT_EQ  = token.PERCENT_EQ
	EQ          = token.EQ
	LT          = token.LT
	GT          = token.GT
	LTE         = token.LTE
	GTE         = token.GTE
	SEND        = token.SEND
	RECV        = token.RECV
	ARROW       = token.ARROW
	PIPE        = token.PIPE
	EZAFE       = token.EZAFE
	LPAREN      = token.LPAREN
	RPAREN      = token.RPAREN
	LBRACKET    = token.LBRACKET
	RBRACKET    = token.RBRACKET
	LBRACE      = token.LBRACE
	RBRACE      = token.RBRACE
	COLON       = token.COLON
	DEF         = token.DEF
	RETURN      = token.RETURN
	IF          = token.IF
	ELSE        = token.ELSE
	WHILE       = token.WHILE
	FOR         = token.FOR
	IN          = token.IN
	FROM        = token.FROM
	TO          = token.TO
	STEP        = token.STEP
	BREAK       = token.BREAK
	CONTINUE    = token.CONTINUE
	PRINT       = token.PRINT
	INPUT       = token.INPUT
	PASS        = token.PASS
	IMPORT      = token.IMPORT
	APPEND      = token.APPEND
	REMOVE      = token.REMOVE
	RAISE       = token.RAISE
	TRUE        = token.TRUE
	FALSE       = token.FALSE
	NONE        = token.NONE
	SELF        = token.SELF
	SUPER       = token.SUPER
	AS          = token.AS
	WITH        = token.WITH
	SEP         = token.SEP
	AND         = token.AND
	OR          = token.OR
	BEH         = token.BEH
	COP_POS     = token.COP_POS
	COP_NEG     = token.COP_NEG
	GO          = token.GO
	CHANNEL     = token.CHANNEL
	CLOSE       = token.CLOSE
	CLOSED      = token.CLOSED
	GLOBAL      = token.GLOBAL
	NONLOCAL    = token.NONLOCAL
	CLASS       = token.CLASS
	INTERF      = token.INTERF
	TRY         = token.TRY
	FINALLY     = token.FINALLY
	DEFER       = token.DEFER
	YIELD       = token.YIELD
	YIELDFROM   = token.YIELDFROM
	DECOR       = token.DECOR
	IMPLEMENTS  = token.IMPLEMENTS
	EXTENDS     = token.EXTENDS
)

// --- AST interfaces (aliased) ---

// Node is the base interface for all AST nodes.
type Node = ast.Node

// Expr is the interface implemented by expression nodes.
type Expr = ast.Expr

// Stmt is the interface implemented by statement nodes.
type Stmt = ast.Stmt

// --- AST expression nodes (aliased) ---

type (
	NumberLit     = ast.NumberLit
	StrLit        = ast.StrLit
	BoolLit       = ast.BoolLit
	NoneLit       = ast.NoneLit
	Ident         = ast.Ident
	Unary         = ast.Unary
	BinaryOp      = ast.BinaryOp
	Call          = ast.Call
	Kwarg         = ast.Kwarg
	Index         = ast.Index
	Slice         = ast.Slice
	MemberAccess  = ast.MemberAccess
	MethodCall    = ast.MethodCall
	ListLit       = ast.ListLit
	TupleLit      = ast.TupleLit
	DictLit       = ast.DictLit
	SetLit        = ast.SetLit
	PipeExpr      = ast.PipeExpr
	TernaryExpr   = ast.TernaryExpr
	CompClause    = ast.CompClause
	ListComp      = ast.ListComp
	DictComp      = ast.DictComp
	SetComp       = ast.SetComp
	GenExp        = ast.GenExp
	ChannelLit    = ast.ChannelLit
	RecvExpr      = ast.RecvExpr
	YieldExpr     = ast.YieldExpr
	YieldFromExpr = ast.YieldFromExpr
)

// --- AST statement nodes (aliased) ---

type (
	ExprStmt       = ast.ExprStmt
	GoStmt         = ast.GoStmt
	GlobalStmt     = ast.GlobalStmt
	NonlocalStmt   = ast.NonlocalStmt
	SendStmt       = ast.SendStmt
	CloseStmt      = ast.CloseStmt
	Assign         = ast.Assign
	MultiAssign    = ast.MultiAssign
	CompoundAssign = ast.CompoundAssign
	PrintStmt      = ast.PrintStmt
	InputStmt      = ast.InputStmt
	ReturnStmt     = ast.ReturnStmt
	IfStmt         = ast.IfStmt
	ElifBranch     = ast.ElifBranch
	WhileStmt      = ast.WhileStmt
	ForRange       = ast.ForRange
	ForIn          = ast.ForIn
	Param          = ast.Param
	DecoratorStmt  = ast.DecoratorStmt
	DefStmt        = ast.DefStmt
	YieldStmt      = ast.YieldStmt
	YieldFromStmt  = ast.YieldFromStmt
	ImportStmt     = ast.ImportStmt
	FromImportStmt = ast.FromImportStmt
	BreakStmt      = ast.BreakStmt
	ContinueStmt   = ast.ContinueStmt
	AppendStmt     = ast.AppendStmt
	RemoveStmt     = ast.RemoveStmt
	ClassDef       = ast.ClassDef
	TryStmt        = ast.TryStmt
	ExceptHandler  = ast.ExceptHandler
	RaiseStmt      = ast.RaiseStmt
	DeferStmt      = ast.DeferStmt
	WithStmt       = ast.WithStmt
	InterfaceDef   = ast.InterfaceDef
	Block          = ast.Block
)
