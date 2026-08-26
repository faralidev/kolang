// Package token defines the token types for the Kolang (کلنگ) lexer.
//
// Kolang is a Persian-language scripting language. Tokens represent the
// lexical units produced by the lexer and consumed by the parser; each token
// carries its type, literal text, and source position.
package token

// Type identifies the kind of a token.
type Type string

// Token type constants. Each Type corresponds to a lexical unit of the
// language; the literal spellings (operators, Persian keywords) are shown
// inline for reference.
const (
	ILLEGAL Type = "ILLEGAL"
	EOF     Type = "EOF"
	NEWLINE Type = "NEWLINE"
	INDENT  Type = "INDENT"
	DEDENT  Type = "DEDENT"

	IDENT  Type = "IDENT"
	INT    Type = "INT"
	FLOAT  Type = "FLOAT"
	STRING Type = "STRING"

	// Arithmetic / assignment operators
	ASSIGN      Type = "="
	PLUS        Type = "+"
	MINUS       Type = "-"
	STAR        Type = "×" // multiply (U+00D7 multiplication sign)
	DIV         Type = "÷"
	FLOORDIV    Type = "÷/"
	PERCENT     Type = "%"
	POW         Type = "*" // power (single asterisk; ** is gone)
	PLUS_EQ     Type = "+="
	MINUS_EQ    Type = "-="
	STAR_EQ     Type = "×="
	DIV_EQ      Type = "÷="
	FLOORDIV_EQ Type = "÷/="
	POW_EQ      Type = "*="
	PERCENT_EQ  Type = "%="

	// Comparison
	EQ  Type = "=="
	LT  Type = "<"
	GT  Type = ">"
	LTE Type = "<="
	GTE Type = ">="

	// Channel / misc operators
	SEND  Type = "<<"
	RECV  Type = ">>"
	ARROW Type = "->"
	PIPE  Type = "|>"

	// Ezafe (kasra U+0650) — member access operator
	EZAFE Type = "EZAFE"

	// Structural
	LPAREN   Type = "("
	RPAREN   Type = ")"
	LBRACKET Type = "["
	RBRACKET Type = "]"
	LBRACE   Type = "{"
	RBRACE   Type = "}"
	COLON    Type = ":"

	// Keywords (Persian)
	DEF      Type = "تعریف"
	RETURN   Type = "برگردان"
	IF       Type = "اگر"
	ELSE     Type = "وگرنه"
	WHILE    Type = "تاوقتی"
	FOR      Type = "برای"
	IN       Type = "در"
	FROM     Type = "از"
	TO       Type = "تا"
	STEP     Type = "گام"
	BREAK    Type = "اتمام"
	CONTINUE Type = "بروبعدی"
	PRINT    Type = "بنویس"
	INPUT    Type = "بگیر"
	PASS     Type = "مثل"
	IMPORT   Type = "بیار"
	APPEND   Type = "بیافزا"
	REMOVE   Type = "حذفکن"
	RAISE    Type = "بده"
	TRUE     Type = "درست"
	FALSE    Type = "غلط"
	NONE     Type = "تهی"
	SELF     Type = "خود"
	SUPER    Type = "والد"
	AS       Type = "بانام"
	WITH     Type = "با" // context manager: با بازکردن(...) بانام ف:
	SEP      Type = "و"
	AND      Type = "همچنین"
	OR       Type = "یا"
	BEH      Type = "به"
	COP_POS  Type = "باشد"
	COP_NEG  Type = "نباشد"

	// Concurrency (v0.6)
	GO      Type = "برو"      // spawn a goroutine: برو کار()
	CHANNEL Type = "کانال"    // create a channel: کانال(صحیح و ۱۰)
	CLOSE   Type = "ببند"     // close a channel (verb-final): ch ببند
	CLOSED  Type = "بسته‌است" // closed-check (ezafe attribute): بسته‌استِ ch

	// Scope declarations (v1.0)
	GLOBAL   Type = "جهانی"  // global declaration: جهانی نام
	NONLOCAL Type = "نامحلی" // nonlocal declaration: نامحلی نام

	// Keywords recognized but deferred to later phases
	CLASS      Type = "گونه"
	INTERF     Type = "رابط"
	TRY        Type = "بپا"
	FINALLY    Type = "درنهایت"
	DEFER      Type = "تأخیری"
	YIELD      Type = "بساز"
	YIELDFROM  Type = "بساز‌از"
	DECOR      Type = "پوشش"
	IMPLEMENTS Type = "رهی"
	EXTENDS    Type = "وارث"
)

// Token is a single lexical token: its type, the literal source text it was
// produced from, and its 1-based line/column position in the source.
type Token struct {
	Type    Type
	Literal string
	Line    int
	Col     int
}

// Keywords maps the Persian keyword spellings (and their non-ZWNJ variants)
// to their token types. The lexer consults it to classify an identifier as a
// keyword rather than a user-defined name.
var Keywords = map[string]Type{
	"تعریف":    DEF,
	"برگردان":  RETURN,
	"اگر":      IF,
	"وگرنه":    ELSE,
	"تاوقتی":   WHILE,
	"برای":     FOR,
	"از":       FROM,
	"تا":       TO,
	"گام":      STEP,
	"اتمام":    BREAK,
	"بروبعدی":  CONTINUE,
	"بنویس":    PRINT,
	"بگیر":     INPUT,
	"مثل":      PASS,
	"بیار":     IMPORT,
	"بیافزا":   APPEND,
	"حذفکن":    REMOVE,
	"حذف‌کن":   REMOVE, // ZWNJ spelling per SPEC
	"بده":      RAISE,
	"درست":     TRUE,
	"غلط":      FALSE,
	"تهی":      NONE,
	"خود":      SELF,
	"والد":     SUPER,
	"بانام":    AS,
	"با":       WITH,
	"و":        SEP,
	"به":       BEH,
	"همچنین":   AND,
	"یا":       OR,
	"باشد":     COP_POS,
	"نباشد":    COP_NEG,
	"بپا":      TRY,
	"درنهایت":  FINALLY,
	"تأخیری":   DEFER,
	"بساز":     YIELD,
	"بساز‌از":  YIELDFROM,
	"بسازاز":   YIELDFROM, // non-ZWNJ spelling of بساز‌از
	"پوشش":     DECOR,
	"رهی":      IMPLEMENTS,
	"وارث":     EXTENDS,
	"برو":      GO,
	"کانال":    CHANNEL,
	"ببند":     CLOSE,
	"بسته‌است": CLOSED,
	"بستهاست":  CLOSED, // non-ZWNJ spelling of بسته‌است
	"جهانی":    GLOBAL,
	"نامحلی":   NONLOCAL,
	"گونه":     CLASS,
	"رابط":     INTERF,
	"در":       IN,
	"است":      COP_POS,
}
