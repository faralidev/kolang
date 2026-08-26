// Package lexer tokenizes Kolang (کلنگ) source code into a slice of tokens.
//
// The lexer scans a UTF-8 source string, handling Persian digits, the kasra
// ezafe member-access operator (U+0650), indentation-based block structure
// (INDENT/DEDENT), comments, and Persian keywords. It is used by the parser.
package lexer

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/faralidev/kolang/internal/token"
)

// Lexer scans a UTF-8 source string into tokens. It tracks the current rune
// position, line/column, and an indentation stack that drives INDENT/DEDENT
// emission for block-structured statements.
type Lexer struct {
	src     []rune
	pos     int
	line    int
	col     int
	indents []int
	tokens  []token.Token
}

// persianDigit maps U+06F0..U+06F9 to Latin '0'..'9'; returns 0,false otherwise.
func persianDigit(r rune) (byte, bool) {
	if r >= 0x06F0 && r <= 0x06F9 {
		return byte(r - 0x06F0 + '0'), true
	}
	return 0, false
}

// Lex tokenizes the whole source string and returns the token slice.
func Lex(src string) []token.Token {
	l := &Lexer{
		src:     []rune(src),
		line:    1,
		col:     1,
		indents: []int{0},
	}
	l.run()
	return l.tokens
}

// peek returns the rune at the current position without consuming it.
func (l *Lexer) peek() (rune, bool) {
	if l.pos < len(l.src) {
		return l.src[l.pos], true
	}
	return 0, false
}

// peekAt returns the rune `off` positions ahead of the current one.
func (l *Lexer) peekAt(off int) (rune, bool) {
	if l.pos+off < len(l.src) {
		return l.src[l.pos+off], true
	}
	return 0, false
}

// advance consumes and returns the next rune, updating line/column tracking.
func (l *Lexer) advance() rune {
	r := l.src[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

// emit appends a token at the current position, computing its column from the
// literal length.
func (l *Lexer) emit(t token.Type, lit string) {
	l.tokens = append(l.tokens, token.Token{Type: t, Literal: lit, Line: l.line, Col: l.col - len([]rune(lit))})
}

// emitCur appends a token with an explicit source position (used for error
// tokens where the current position has already moved past the offending text).
func (l *Lexer) emitCur(t token.Type, lit string, line, col int) {
	l.tokens = append(l.tokens, token.Token{Type: t, Literal: lit, Line: line, Col: col})
}

// run performs the full tokenization loop.
func (l *Lexer) run() {
	atLineStart := true
	for l.pos < len(l.src) {
		if atLineStart {
			l.handleLineStart()
			atLineStart = false
			if l.pos >= len(l.src) {
				break
			}
		}
		startLine, startCol := l.line, l.col
		r := l.src[l.pos]

		switch {
		case r == ' ' || r == '\t':
			l.advance() // stray whitespace separator
		case r == '\n':
			l.advance()
			l.emit(token.NEWLINE, "\n")
			atLineStart = true
		case r == '«':
			l.lexString(startLine, startCol)
		case r == '/':
			l.skipComment()
		case r >= '0' && r <= '9' || (r >= 0x06F0 && r <= 0x06F9):
			l.lexNumber(startLine, startCol)
		case isIdentStart(r):
			l.lexIdentifier(startLine, startCol)
		default:
			l.lexOperator(startLine, startCol)
		}
	}

	// Close remaining blocks.
	for len(l.indents) > 1 {
		l.indents = l.indents[:len(l.indents)-1]
		l.emit(token.DEDENT, "")
	}
	l.emit(token.EOF, "")
}

// handleLineStart measures the indentation of the next non-blank,
// non-comment-only line and emits INDENT/DEDENT accordingly. It is called at
// the start of each physical line (and once at the very beginning).
func (l *Lexer) handleLineStart() {
	// Look ahead through blank lines and comment-only lines to find the first
	// line that actually produces a token.
	for {
		// measure leading whitespace
		indent := 0
		for {
			r, ok := l.peek()
			if !ok {
				return
			}
			if r == ' ' {
				l.advance()
				indent++
			} else if r == '\t' {
				l.advance()
				indent += 4
			} else {
				break
			}
		}

		r, ok := l.peek()
		if !ok {
			return
		}

		// A blank line (only whitespace then newline).
		if r == '\n' {
			l.advance() // consume newline, no token
			continue
		}

		// A comment-only line.
		if r == '/' {
			savePos, saveLine, saveCol := l.pos, l.line, l.col
			saveToks := len(l.tokens)
			l.skipComment()
			// After consuming leading comment(s), check remainder of line.
			rr, ok2 := l.peek()
			for ok2 && (rr == ' ' || rr == '\t') {
				l.advance()
				rr, ok2 = l.peek()
			}
			if !ok2 {
				// EOF: nothing after comment
				l.pos, l.line, l.col = savePos, saveLine, saveCol
				l.tokens = l.tokens[:saveToks]
				return
			}
			if rr == '\n' {
				l.advance() // consume newline; comment-only line
				continue
			}
			// There is code after the comment on the same line.
			// Restore position before the comment (we only wanted to inspect).
			l.pos, l.line, l.col = savePos, saveLine, saveCol
			l.tokens = l.tokens[:saveToks]
			l.emitIndents(indent)
			return
		}

		// Real code line: adjust indent stack.
		l.emitIndents(indent)
		return
	}
}

// emitIndents adjusts the indentation stack against the measured indent width,
// emitting INDENT tokens when a block opens and DEDENT tokens when one (or
// more) closes. An inconsistent dedent is tolerated by resetting the stack top.
func (l *Lexer) emitIndents(indent int) {
	top := l.indents[len(l.indents)-1]
	if indent > top {
		l.indents = append(l.indents, indent)
		l.emit(token.INDENT, "")
	} else if indent < top {
		for len(l.indents) > 1 && l.indents[len(l.indents)-1] > indent {
			l.indents = l.indents[:len(l.indents)-1]
			l.emit(token.DEDENT, "")
		}
		if l.indents[len(l.indents)-1] != indent {
			// Inconsistent dedent: tolerate by resetting top.
			l.indents[len(l.indents)-1] = indent
		}
	}
}

// skipComment consumes a single-line comment (up to end of line) or a
// block comment «// ... //». An unterminated block comment is reported as an
// ILLEGAL token rather than silently consuming the rest of the file.
func (l *Lexer) skipComment() {
	startLine, startCol := l.line, l.col
	// `/` already at current position.
	l.advance() // consume /
	if r, ok := l.peek(); ok && r == '/' {
		// block comment // ... //
		l.advance() // consume second /
		for l.pos < len(l.src) {
			if l.src[l.pos] == '/' {
				if r2, ok2 := l.peekAt(1); ok2 && r2 == '/' {
					l.advance()
					l.advance()
					return
				}
			}
			l.advance()
		}
		// Unterminated block comment: the rest of the file was consumed as
		// comment, so report it instead of silently tolerating it.
		l.emitCur(token.ILLEGAL, "کامنت بلوک بسته نشده", startLine, startCol)
		return
	}
	// single-line comment until end of line
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
}

// isIdentStart reports whether a rune may begin an identifier (a letter or
// underscore; Persian letters included).
func isIdentStart(r rune) bool {
	return isLetter(r) || r == '_'
}

// isLetter reports whether r is a Latin or Arabic/Persian letter. Combining
// diacritic marks (U+064B–U+065F, which include the kasra/ezafe U+0650) are
// excluded so the ezafe is never absorbed into an identifier.
func isLetter(r rune) bool {
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
		return true
	}
	// Arabic/Persian letters, but NOT the combining diacritic marks
	// (U+064B–U+065F), which include the kasra/ezafe U+0650.
	if r >= 0x0600 && r <= 0x06FF {
		if r >= 0x064B && r <= 0x065F {
			return false
		}
		return true
	}
	return (r >= 0xFB50 && r <= 0xFDFF) ||
		(r >= 0xFE70 && r <= 0xFEFF)
}

// isIdentPart reports whether a rune may appear inside an identifier: letters,
// digits, underscore, or a ZWNJ (U+200C).
func isIdentPart(r rune) bool {
	if isLetter(r) || (r >= '0' && r <= '9') || r == '_' {
		return true
	}
	// ZWNJ (U+200C) is part of identifiers.
	return r == 0x200C
}

// lexIdentifier consumes an identifier or Persian keyword. Keywords are looked
// up in token.Keywords; anything else is emitted as an IDENT.
func (l *Lexer) lexIdentifier(startLine, startCol int) {
	var sb strings.Builder
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		sb.WriteRune(l.advance())
	}
	lit := sb.String()
	if t, ok := token.Keywords[lit]; ok {
		l.emit(t, lit)
		_ = startLine
		_ = startCol
		return
	}
	l.emit(token.IDENT, lit)
}

// lexNumber consumes a numeric literal: decimal or prefixed (0x/0b/0o)
// integers, floats, Persian digits, and stripped digit-group separators.
// Out-of-range integers are promoted to float rather than silently truncated.
func (l *Lexer) lexNumber(startLine, startCol int) {
	var sb strings.Builder
	base := 10
	isFloat := false

	// Detect 0x / 0b / 0o prefix (works with Persian zero too).
	first, _ := l.peek()
	if first == '0' || first == 0x06F0 {
		if nxt, ok := l.peekAt(1); ok && (nxt == 'x' || nxt == 'X' || nxt == 'b' || nxt == 'B' || nxt == 'o' || nxt == 'O') {
			l.advance() // consume 0
			switch nxt {
			case 'x', 'X':
				base = 16
			case 'b', 'B':
				base = 2
			case 'o', 'O':
				base = 8
			}
			l.advance() // consume prefix letter
			for l.pos < len(l.src) {
				r := l.src[l.pos]
				if d, ok := persianDigit(r); ok {
					sb.WriteByte(d)
					l.advance()
					continue
				}
				if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
					sb.WriteByte(byte(r))
					l.advance()
					continue
				}
				break
			}
			v, err := strconv.ParseInt(sb.String(), base, 64)
			if err != nil {
				// Malformed (e.g. «0x» with no digits, «0b102») or out-of-range
				// prefixed literal: report it instead of silently emitting 0.
				l.emitCur(token.ILLEGAL, fmt.Sprintf("عدد نامعتبر: %q", sb.String()), startLine, startCol)
				return
			}
			lit := strconv.FormatInt(v, 10)
			l.tokens = append(l.tokens, token.Token{Type: token.INT, Literal: lit, Line: startLine, Col: startCol})
			return
		}
	}

	// consume integer digits (Persian or Latin), stripping group separators.
	for {
		r, ok := l.peek()
		if !ok {
			break
		}
		if d, isd := persianDigit(r); isd {
			sb.WriteByte(d)
			l.advance()
			continue
		}
		if r >= '0' && r <= '9' {
			sb.WriteByte(byte(r))
			l.advance()
			continue
		}
		if r == 0x066C || r == ',' { // group separator: strip
			l.advance()
			continue
		}
		break
	}

	// fractional part (٫ U+066B or .)
	if r, ok := l.peek(); ok && (r == '.' || r == 0x066B) {
		isFloat = true
		sb.WriteByte('.')
		l.advance()
		for {
			r, ok := l.peek()
			if !ok {
				break
			}
			if d, isd := persianDigit(r); isd {
				sb.WriteByte(d)
				l.advance()
				continue
			}
			if r >= '0' && r <= '9' {
				sb.WriteByte(byte(r))
				l.advance()
				continue
			}
			if r == 0x066C || r == ',' {
				l.advance()
				continue
			}
			break
		}
	}

	lit := sb.String()
	if isFloat {
		v, err := strconv.ParseFloat(lit, 64)
		if err != nil {
			l.emitCur(token.ILLEGAL, fmt.Sprintf("عدد نامعتبر: %q", lit), startLine, startCol)
			return
		}
		l.tokens = append(l.tokens, token.Token{Type: token.FLOAT, Literal: strconv.FormatFloat(v, 'f', -1, 64), Line: startLine, Col: startCol})
		return
	}
	v, err := strconv.ParseInt(lit, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			// Integer too large for int64: promote to float (Python-like) so a
			// huge literal is not silently truncated to 0.
			if fv, ferr := strconv.ParseFloat(lit, 64); ferr == nil {
				l.tokens = append(l.tokens, token.Token{Type: token.FLOAT, Literal: strconv.FormatFloat(fv, 'f', -1, 64), Line: startLine, Col: startCol})
				return
			}
		}
		l.emitCur(token.ILLEGAL, fmt.Sprintf("عدد نامعتبر: %q", lit), startLine, startCol)
		return
	}
	l.tokens = append(l.tokens, token.Token{Type: token.INT, Literal: strconv.FormatInt(v, 10), Line: startLine, Col: startCol})
}

// lexString consumes a «...» string literal. An unterminated string emits an
// ILLEGAL token.
func (l *Lexer) lexString(startLine, startCol int) {
	l.advance() // consume opening «
	var sb strings.Builder
	for l.pos < len(l.src) {
		r := l.advance()
		if r == '»' {
			l.emit(token.STRING, sb.String())
			return
		}
		sb.WriteRune(r)
	}
	// unterminated string
	l.tokens = append(l.tokens, token.Token{Type: token.ILLEGAL, Literal: "متن بسته نشده", Line: startLine, Col: startCol})
}

// lexOperator consumes a multi- or single-character operator, the kasra ezafe,
// or a punctuation token. Unknown characters emit an ILLEGAL token.
func (l *Lexer) lexOperator(startLine, startCol int) {
	r := l.src[l.pos]
	// multi-char operators
	two := ""
	if l.pos+1 < len(l.src) {
		two = string(r) + string(l.src[l.pos+1])
	}
	three := ""
	if l.pos+2 < len(l.src) {
		three = string(r) + string(l.src[l.pos+1]) + string(l.src[l.pos+2])
	}

	switch three {
	case "÷/=":
		l.advance()
		l.advance()
		l.advance()
		l.emit(token.FLOORDIV_EQ, "÷/=")
		return
	}

	switch two {
	case "==":
		l.advance()
		l.advance()
		l.emit(token.EQ, "==")
		return
	case "<=":
		l.advance()
		l.advance()
		l.emit(token.LTE, "<=")
		return
	case ">=":
		l.advance()
		l.advance()
		l.emit(token.GTE, ">=")
		return
	case "÷/":
		l.advance()
		l.advance()
		l.emit(token.FLOORDIV, "÷/")
		return
	case "+=":
		l.advance()
		l.advance()
		l.emit(token.PLUS_EQ, "+=")
		return
	case "-=":
		l.advance()
		l.advance()
		l.emit(token.MINUS_EQ, "-=")
		return
	case "×=":
		l.advance()
		l.advance()
		l.emit(token.STAR_EQ, "×=")
		return
	case "*=":
		l.advance()
		l.advance()
		l.emit(token.POW_EQ, "*=")
		return
	case "÷=":
		l.advance()
		l.advance()
		l.emit(token.DIV_EQ, "÷=")
		return
	case "%=":
		l.advance()
		l.advance()
		l.emit(token.PERCENT_EQ, "%=")
		return
	case "<<":
		l.advance()
		l.advance()
		l.emit(token.SEND, "<<")
		return
	case ">>":
		l.advance()
		l.advance()
		l.emit(token.RECV, ">>")
		return
	case "->":
		l.advance()
		l.advance()
		l.emit(token.ARROW, "->")
		return
	case "|>":
		l.advance()
		l.advance()
		l.emit(token.PIPE, "|>")
		return
	}

	switch r {
	case 0x0650: // kasra ezafe
		l.advance()
		l.emit(token.EZAFE, "ِ")
		return
	case '=':
		l.advance()
		l.emit(token.ASSIGN, "=")
	case '+':
		l.advance()
		l.emit(token.PLUS, "+")
	case '-':
		l.advance()
		l.emit(token.MINUS, "-")
	case '*':
		l.advance()
		l.emit(token.POW, "*")
	case '×': // U+00D7 multiplication sign
		l.advance()
		l.emit(token.STAR, "×")
	case '÷':
		l.advance()
		l.emit(token.DIV, "÷")
	case '%':
		l.advance()
		l.emit(token.PERCENT, "%")
	case '<':
		l.advance()
		l.emit(token.LT, "<")
	case '>':
		l.advance()
		l.emit(token.GT, ">")
	case '(':
		l.advance()
		l.emit(token.LPAREN, "(")
	case ')':
		l.advance()
		l.emit(token.RPAREN, ")")
	case '[':
		l.advance()
		l.emit(token.LBRACKET, "[")
	case ']':
		l.advance()
		l.emit(token.RBRACKET, "]")
	case '{':
		l.advance()
		l.emit(token.LBRACE, "{")
	case '}':
		l.advance()
		l.emit(token.RBRACE, "}")
	case ':':
		l.advance()
		l.emit(token.COLON, ":")
	default:
		l.advance()
		l.emit(token.ILLEGAL, fmt.Sprintf("کاراکتر نامعتبر: %q", string(r)))
	}
}
