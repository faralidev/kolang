package lexer

import (
	"testing"

	"github.com/faralidev/kolang/internal/token"
)

func TestLexBasics(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []token.Type
	}{
		{
			name: "persian digits normalized",
			src:  "۱۲۳ ۴٫۵ ۱٬۲۳۴",
			want: []token.Type{token.INT, token.FLOAT, token.INT},
		},
		{
			name: "hello world",
			src:  "«سلام دنیا!» بنویس",
			want: []token.Type{token.STRING, token.PRINT},
		},
		{
			name: "ezafe token",
			src:  "جذرِ ریاضی",
			want: []token.Type{token.IDENT, token.EZAFE, token.IDENT},
		},
		{
			name: "zwnj identifier",
			src:  "خطای‌صفر",
			want: []token.Type{token.IDENT},
		},
		{
			name: "operators",
			src:  "a + b - c × d ÷ e % f * g",
			want: []token.Type{token.IDENT, token.PLUS, token.IDENT, token.MINUS, token.IDENT, token.STAR, token.IDENT, token.DIV, token.IDENT, token.PERCENT, token.IDENT, token.POW, token.IDENT},
		},
		{
			name: "comparisons",
			src:  "a == b < c > d <= e >= f",
			want: []token.Type{token.IDENT, token.EQ, token.IDENT, token.LT, token.IDENT, token.GT, token.IDENT, token.LTE, token.IDENT, token.GTE, token.IDENT},
		},
		{
			name: "string with braces",
			src:  "«سلام {نام}!»",
			want: []token.Type{token.STRING},
		},
		{
			name: "keyword recognition",
			src:  "تعریف اگر درست غلط تهی",
			want: []token.Type{token.DEF, token.IF, token.TRUE, token.FALSE, token.NONE},
		},
		{
			name: "compound assignment",
			src:  "a += ۱ و b ×= ۲",
			want: []token.Type{token.IDENT, token.PLUS_EQ, token.INT, token.SEP, token.IDENT, token.STAR_EQ, token.INT},
		},
		{
			name: "number prefix hex",
			src:  "۰x1F",
			want: []token.Type{token.INT},
		},
		{
			name: "yield and yield-from keywords",
			src:  "ای بساز\nبساز‌از الف",
			want: []token.Type{token.IDENT, token.YIELD, token.NEWLINE, token.YIELDFROM, token.IDENT},
		},
		{
			name: "decorator keyword",
			src:  "پوشش دوبار",
			want: []token.Type{token.DECOR, token.IDENT},
		},
		{
			name: "concurrency keywords",
			src:  "برو کار() کانال(صحیح و ۱۰) بسته‌استِ ch ch ببند",
			want: []token.Type{token.GO, token.IDENT, token.LPAREN, token.RPAREN, token.CHANNEL, token.LPAREN, token.IDENT, token.SEP, token.INT, token.RPAREN, token.CLOSED, token.EZAFE, token.IDENT, token.IDENT, token.CLOSE},
		},
		{
			name: "channel operators",
			src:  "ch << ۱\nمقدار = >>ch",
			want: []token.Type{token.IDENT, token.SEND, token.INT, token.NEWLINE, token.IDENT, token.ASSIGN, token.RECV, token.IDENT},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks := Lex(tt.src)
			// strip trailing EOF
			if len(toks) > 0 && toks[len(toks)-1].Type == token.EOF {
				toks = toks[:len(toks)-1]
			}
			if len(toks) != len(tt.want) {
				got := make([]token.Type, len(toks))
				for i, tk := range toks {
					got[i] = tk.Type
				}
				t.Fatalf("got %d tokens %v, want %v", len(toks), got, tt.want)
			}
			for i, tk := range toks {
				if tk.Type != tt.want[i] {
					t.Errorf("token %d: got %s, want %s", i, tk.Type, tt.want[i])
				}
			}
		})
	}
}

func TestLexValues(t *testing.T) {
	toks := Lex("۱۲۳ ۱۲٫۵")
	// spaces produce no tokens
	if toks[0].Literal != "123" {
		t.Errorf("int literal = %q, want 123", toks[0].Literal)
	}
	if toks[1].Literal != "12.5" {
		t.Errorf("float literal = %q, want 12.5", toks[1].Literal)
	}
}

// L5: keywords are accepted with and without ZWNJ (U+200C) so users typing the
// SPEC's canonical «حذف‌کن» or the plain «حذفکن» both work.
func TestLexZWNJKeywordVariants(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want token.Type
	}{
		{"remove with zwnj", "حذف‌کن", token.REMOVE},
		{"remove without zwnj", "حذفکن", token.REMOVE},
		{"yieldfrom without zwnj", "بسازاز", token.YIELDFROM},
		{"yieldfrom with zwnj", "بساز‌از", token.YIELDFROM},
		{"closed without zwnj", "بستهاست", token.CLOSED},
		{"closed with zwnj", "بسته‌است", token.CLOSED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks := Lex(tt.src)
			if len(toks) == 0 || toks[0].Type != tt.want {
				got := token.Type("none")
				if len(toks) > 0 {
					got = toks[0].Type
				}
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// L11: an unterminated block comment must be reported, not silently tolerated.
func TestLexUnterminatedBlockComment(t *testing.T) {
	toks := Lex("// this never closes")
	if len(toks) > 0 && toks[len(toks)-1].Type == token.EOF {
		toks = toks[:len(toks)-1]
	}
	if len(toks) != 1 || toks[0].Type != token.ILLEGAL {
		got := make([]token.Type, len(toks))
		for i, tk := range toks {
			got[i] = tk.Type
		}
		t.Fatalf("got %v, want [ILLEGAL]", got)
	}
	if toks[0].Literal != "کامنت بلوک بسته نشده" {
		t.Errorf("message = %q, want کامنت بلوک بسته نشده", toks[0].Literal)
	}
	// a properly closed block comment still lexes to nothing but EOF
	toks = Lex("// ok //")
	if len(toks) != 1 || toks[0].Type != token.EOF {
		t.Errorf("closed block comment should lex to EOF only, got %v", toks)
	}
}

// L10: a huge integer literal must not silently become 0 — it promotes to a
// float (Python-like).
func TestLexHugeNumberPromotesToFloat(t *testing.T) {
	toks := Lex("۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹")
	if len(toks) == 0 {
		t.Fatal("no tokens")
	}
	if toks[0].Type != token.FLOAT {
		t.Fatalf("got %s, want FLOAT", toks[0].Type)
	}
	if toks[0].Literal == "0" {
		t.Errorf("huge literal silently became 0")
	}
}

// L10: malformed prefixed literals are reported instead of silently becoming 0.
func TestLexMalformedNumbers(t *testing.T) {
	for _, src := range []string{"0x", "0b", "0o", "0b102", "0xGG"} {
		toks := Lex(src)
		if len(toks) == 0 || toks[0].Type != token.ILLEGAL {
			got := token.Type("none")
			if len(toks) > 0 {
				got = toks[0].Type
			}
			t.Errorf("%q: got %s, want ILLEGAL", src, got)
		}
	}
}

func TestLexIndentation(t *testing.T) {
	src := "اگر سن == ۱۸ باشد:\n    «بزرگسال» بنویس\nوگرنه:\n    «کودک» بنویس\n"
	toks := Lex(src)
	var types []token.Type
	for _, tk := range toks {
		types = append(types, tk.Type)
	}
	want := []token.Type{
		token.IF, token.IDENT, token.EQ, token.INT, token.COP_POS, token.COLON, token.NEWLINE,
		token.INDENT, token.STRING, token.PRINT, token.NEWLINE,
		token.DEDENT,
		token.ELSE, token.COLON, token.NEWLINE,
		token.INDENT, token.STRING, token.PRINT, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	}
	if len(types) != len(want) {
		t.Fatalf("token count = %d, want %d\ngot  %v\nwant %v", len(types), len(want), types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("token %d: got %s, want %s", i, types[i], want[i])
		}
	}
}

// --- Phase 8: comprehensive spec-coverage lexer tests ---

func TestLexPersianDigits(t *testing.T) {
	toks := Lex("۰ ۱۲۳ ۹۹۹")
	if len(toks) < 3 {
		t.Fatalf("expected 3 number tokens, got %d", len(toks))
	}
	want := []string{"0", "123", "999"}
	for i, w := range want {
		if toks[i].Type != token.INT || toks[i].Literal != w {
			t.Errorf("token %d: got %s(%q), want INT(%q)", i, toks[i].Type, toks[i].Literal, w)
		}
	}
}

func TestLexDigitGroupSeparators(t *testing.T) {
	tests := []struct{ src, want string }{
		{"۱۲۳٬۴۵۶", "123456"}, // Persian thousands separator ٬ (U+066C)
		{"123,456", "123456"}, // Latin comma separator
		{"۱٬۲۳۴٬۵۶۷", "1234567"},
	}
	for _, tc := range tests {
		toks := Lex(tc.src)
		if len(toks) < 2 || toks[0].Type != token.INT || toks[0].Literal != tc.want {
			got := token.Type("none")
			if len(toks) > 0 {
				got = toks[0].Type
			}
			t.Errorf("%q: got %s(%q), want INT(%q)", tc.src, got, tokLiteral(toks), tc.want)
		}
	}
}

func TestLexFloatSeparators(t *testing.T) {
	tests := []struct{ src, want string }{
		{"۴٫۵", "4.5"}, // Persian decimal separator ٫ (U+066B)
		{"4.5", "4.5"}, // Latin decimal point
		{"۱٫۲۵", "1.25"},
	}
	for _, tc := range tests {
		toks := Lex(tc.src)
		if len(toks) < 2 || toks[0].Type != token.FLOAT || toks[0].Literal != tc.want {
			t.Errorf("%q: got FLOAT(%q), want FLOAT(%q)", tc.src, tokLiteral(toks), tc.want)
		}
	}
}

func TestLexRadixPrefixes(t *testing.T) {
	tests := []struct{ src, want string }{
		{"۰x1F", "31"}, // Persian zero + hex digits
		{"۰b101", "5"},
		{"۰o17", "15"},
		{"0x10", "16"}, // Latin zero + x
		{"0b11", "3"},
		{"0o7", "7"},
	}
	for _, tc := range tests {
		toks := Lex(tc.src)
		if len(toks) < 2 || toks[0].Type != token.INT || toks[0].Literal != tc.want {
			t.Errorf("%q: got %s(%q), want INT(%q)", tc.src, tokType(toks), tokLiteral(toks), tc.want)
		}
	}
}

func TestLexFloorDivisionAndCompounds(t *testing.T) {
	toks := Lex("a ÷/ b ÷/= c += d -= e ×= f *= g %= h")
	want := []token.Type{
		token.IDENT, token.FLOORDIV, token.IDENT, token.FLOORDIV_EQ, token.IDENT,
		token.PLUS_EQ, token.IDENT, token.MINUS_EQ, token.IDENT,
		token.STAR_EQ, token.IDENT, token.POW_EQ, token.IDENT, token.PERCENT_EQ, token.IDENT,
	}
	if len(toks) != len(want)+1 { // +EOF
		got := make([]token.Type, len(toks))
		for i, tk := range toks {
			got[i] = tk.Type
		}
		t.Fatalf("got %d tokens %v, want %v", len(toks), got, want)
	}
	for i, w := range want {
		if toks[i].Type != w {
			t.Errorf("token %d: got %s, want %s", i, toks[i].Type, w)
		}
	}
}

func TestLexAllKeywords(t *testing.T) {
	src := "تعریف برگردان اگر وگرنه تاوقتی برای از تا گام اتمام بروبعدی بنویس بگیر مثل بیار بیافزا حذفکن بده درست غلط تهی خود والد بانام با و به همچنین یا باشد نباشد بپا درنهایت تأخیری بساز بسازاز پوشش رهی وارث برو کانال ببند بستهاست جهانی نامحلی گونه رابط در است"
	want := []token.Type{
		token.DEF, token.RETURN, token.IF, token.ELSE, token.WHILE, token.FOR,
		token.FROM, token.TO, token.STEP, token.BREAK, token.CONTINUE, token.PRINT,
		token.INPUT, token.PASS, token.IMPORT, token.APPEND, token.REMOVE, token.RAISE,
		token.TRUE, token.FALSE, token.NONE, token.SELF, token.SUPER, token.AS,
		token.WITH, token.SEP, token.BEH, token.AND, token.OR, token.COP_POS, token.COP_NEG,
		token.TRY, token.FINALLY, token.DEFER, token.YIELD, token.YIELDFROM, token.DECOR,
		token.IMPLEMENTS, token.EXTENDS, token.GO, token.CHANNEL, token.CLOSE, token.CLOSED,
		token.GLOBAL, token.NONLOCAL, token.CLASS, token.INTERF, token.IN, token.COP_POS,
	}
	toks := Lex(src)
	if len(toks) != len(want)+1 { // +EOF
		got := make([]token.Type, len(toks))
		for i, tk := range toks {
			got[i] = tk.Type
		}
		t.Fatalf("got %d tokens %v, want %v", len(toks), got, want)
	}
	for i, w := range want {
		if toks[i].Type != w {
			t.Errorf("token %d: got %s, want %s", i, toks[i].Type, w)
		}
	}
}

func TestLexStringInterpolationToken(t *testing.T) {
	// Interpolation happens at eval time; the lexer emits a single STRING.
	toks := Lex("«سلام {نام}!»")
	if len(toks) < 2 || toks[0].Type != token.STRING {
		t.Fatalf("interpolated string: got %s(%q), want STRING", tokType(toks), tokLiteral(toks))
	}
	if toks[0].Literal != "سلام {نام}!" {
		t.Errorf("literal = %q, want the raw text including braces", toks[0].Literal)
	}
}

func TestLexComments(t *testing.T) {
	// Single-line comment: / rest-of-line is dropped.
	toks := Lex("x = ۱ / این یک کامن است\n")
	var types []token.Type
	for _, tk := range toks {
		types = append(types, tk.Type)
	}
	want := []token.Type{token.IDENT, token.ASSIGN, token.INT, token.NEWLINE, token.EOF}
	if len(types) != len(want) {
		t.Fatalf("single-line comment: got %v, want %v", types, want)
	}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("token %d: got %s, want %s", i, types[i], w)
		}
	}

	// Block comment: // ... // is dropped entirely.
	toks = Lex("// comment // x = ۲")
	types = types[:0]
	for _, tk := range toks {
		types = append(types, tk.Type)
	}
	want = []token.Type{token.IDENT, token.ASSIGN, token.INT, token.EOF}
	if len(types) != len(want) {
		t.Fatalf("block comment: got %v, want %v", types, want)
	}
}

func TestLexZWNJKeywordVariantsMore(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want token.Type
	}{
		{"closed zwnj", "بسته‌است", token.CLOSED},
		{"closed no zwnj", "بستهاست", token.CLOSED},
		{"yieldfrom zwnj", "بساز‌از", token.YIELDFROM},
		{"yieldfrom no zwnj", "بسازاز", token.YIELDFROM},
		{"remove zwnj", "حذف‌کن", token.REMOVE},
		{"remove no zwnj", "حذفکن", token.REMOVE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks := Lex(tt.src)
			if len(toks) == 0 || toks[0].Type != tt.want {
				got := token.Type("none")
				if len(toks) > 0 {
					got = toks[0].Type
				}
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLexNegativeNumbers(t *testing.T) {
	toks := Lex("-۵ ۱-")
	// "-۵" → MINUS INT; "۱-" → INT MINUS (no merged negative literal)
	want := []token.Type{token.MINUS, token.INT, token.INT, token.MINUS}
	if len(toks) != len(want)+1 { // +EOF
		got := make([]token.Type, len(toks))
		for i, tk := range toks {
			got[i] = tk.Type
		}
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if toks[i].Type != w {
			t.Errorf("token %d: got %s, want %s", i, toks[i].Type, w)
		}
	}
}

func tokType(toks []token.Token) token.Type {
	if len(toks) == 0 {
		return token.Type("none")
	}
	return toks[0].Type
}

func tokLiteral(toks []token.Token) string {
	if len(toks) == 0 {
		return ""
	}
	return toks[0].Literal
}

// lexTypes returns the token types of toks, excluding the trailing EOF.
func lexTypes(toks []token.Token) []token.Type {
	if len(toks) > 0 && toks[len(toks)-1].Type == token.EOF {
		toks = toks[:len(toks)-1]
	}
	var out []token.Type
	for _, tk := range toks {
		out = append(out, tk.Type)
	}
	return out
}

// --- Phase 10: comprehensive spec-coverage lexer tests (appended) ---

// L2: «...» guillemets produce a single STRING token whose literal is the raw
// inner text (interpolation happens at eval time).
func TestLexGuillemetString(t *testing.T) {
	toks := Lex("«سلام دنیا»")
	if got := lexTypes(toks); len(got) != 1 || got[0] != token.STRING {
		t.Fatalf("guillemet string: got %v, want [STRING]", got)
	}
	if toks[0].Literal != "سلام دنیا" {
		t.Errorf("literal = %q, want سلام دنیا", toks[0].Literal)
	}

	// a multiline «...» string is still a single STRING token
	toks = Lex("«خط اول\nخط دوم»")
	if got := lexTypes(toks); len(got) != 1 || got[0] != token.STRING {
		t.Fatalf("multiline string: got %v, want [STRING]", got)
	}

	// an unterminated string reports ILLEGAL with a Persian message
	toks = Lex("«ناتمام")
	if len(toks) == 0 || toks[0].Type != token.ILLEGAL {
		t.Fatalf("unterminated string: got %v, want ILLEGAL", lexTypes(toks))
	}
	if toks[0].Literal != "متن بسته نشده" {
		t.Errorf("message = %q, want متن بسته نشده", toks[0].Literal)
	}
}

// L3: comments. `/` is a single-line comment; `// ... //` is a block comment;
// an unterminated block comment is an ILLEGAL token.
func TestLexCommentsExtra(t *testing.T) {
	// single-line comment after code
	toks := Lex("x = ۱ / این یک کامنت است\n")
	want := []token.Type{token.IDENT, token.ASSIGN, token.INT, token.NEWLINE}
	if got := lexTypes(toks); !slicesEqual(got, want) {
		t.Fatalf("inline comment: got %v, want %v", got, want)
	}

	// single-line comment at end of input (no newline)
	toks = Lex("a + b / کامنت")
	want = []token.Type{token.IDENT, token.PLUS, token.IDENT}
	if got := lexTypes(toks); !slicesEqual(got, want) {
		t.Fatalf("trailing comment: got %v, want %v", got, want)
	}

	// block comment spanning multiple lines
	toks = Lex("// کامنت\nچندخطی //\ny = ۲")
	want = []token.Type{token.IDENT, token.ASSIGN, token.INT}
	if got := lexTypes(toks); !slicesEqual(got, want) {
		t.Fatalf("block comment: got %v, want %v", got, want)
	}

	// unterminated block comment
	toks = Lex("// باز")
	if len(toks) == 0 || toks[0].Type != token.ILLEGAL {
		t.Fatalf("unterminated block comment: got %v, want ILLEGAL", lexTypes(toks))
	}
}

func slicesEqual(a, b []token.Type) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// L4: Persian digits, ٫ decimal, ٬ group separator, and ۰x/۰b/۰o prefixes all
// normalize to Latin literals.
func TestLexNumbers(t *testing.T) {
	tests := []struct {
		src      string
		wantType token.Type
		wantLit  string
	}{
		{"۰", token.INT, "0"},
		{"۱۲۳", token.INT, "123"},
		{"۴٫۵", token.FLOAT, "4.5"},
		{"۱٬۲۳۴", token.INT, "1234"},
		{"۰xFF", token.INT, "255"},
		{"۰b101", token.INT, "5"},
		{"۰o17", token.INT, "15"},
	}
	for _, tc := range tests {
		toks := Lex(tc.src)
		if tokType(toks) != tc.wantType || tokLiteral(toks) != tc.wantLit {
			t.Errorf("%q: got %s(%q), want %s(%q)", tc.src, tokType(toks), tokLiteral(toks), tc.wantType, tc.wantLit)
		}
	}
}

// L5: the operator glyphs — × ÷ * ÷/ % << >> |> and the kasra ezafe ِ.
func TestLexOperators(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []token.Type
	}{
		{"multiply", "a × b", []token.Type{token.IDENT, token.STAR, token.IDENT}},
		{"divide", "a ÷ b", []token.Type{token.IDENT, token.DIV, token.IDENT}},
		{"power", "a * b", []token.Type{token.IDENT, token.POW, token.IDENT}},
		{"floor-div", "a ÷/ b", []token.Type{token.IDENT, token.FLOORDIV, token.IDENT}},
		{"modulo", "a % b", []token.Type{token.IDENT, token.PERCENT, token.IDENT}},
		{"send", "ch << ۱", []token.Type{token.IDENT, token.SEND, token.INT}},
		{"recv", "x = >>ch", []token.Type{token.IDENT, token.ASSIGN, token.RECV, token.IDENT}},
		{"pipe", "x |> f", []token.Type{token.IDENT, token.PIPE, token.IDENT}},
		{"ezafe", "جذرِ ریاضی", []token.Type{token.IDENT, token.EZAFE, token.IDENT}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks := Lex(tt.src)
			got := lexTypes(toks)
			if !slicesEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// L5: the ZWNJ (U+200C) is part of an identifier: خطای‌صفر is ONE IDENT token.
func TestLexZWNJ(t *testing.T) {
	toks := Lex("خطای‌صفر")
	if got := lexTypes(toks); len(got) != 1 || got[0] != token.IDENT {
		t.Fatalf("got %v, want [IDENT]", got)
	}
	if toks[0].Literal != "خطای‌صفر" {
		t.Errorf("literal = %q, want خطای‌صفر", toks[0].Literal)
	}

	// a space splits it into two identifiers
	toks = Lex("خطای صفر")
	if got := lexTypes(toks); len(got) != 2 || got[0] != token.IDENT || got[1] != token.IDENT {
		t.Fatalf("space-split: got %v, want [IDENT IDENT]", got)
	}
}
