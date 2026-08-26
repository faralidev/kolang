// Package parser implements a recursive-descent parser for Kolang.
//
// The parser consumes the token stream produced by the lexer and builds an
// AST. Expressions use Pratt-style precedence climbing for binary operators;
// the kasra ezafe (member access) is parsed postfix. Conditions require an
// explicit copula (باشد / نباشد), enforced here as a syntax rule.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/faralidev/kolang/internal/ast"
	"github.com/faralidev/kolang/internal/lexer"
	"github.com/faralidev/kolang/internal/token"
)

// Parser parses a token stream into an AST.
type Parser struct {
	toks []token.Token
	pos  int
	// pendingDecorators holds «پوشش» lines parsed before a function definition.
	// A decorator statement is only meaningful when immediately followed by a
	// تعریف, so the next def consumes (and clears) them.
	pendingDecorators []*ast.DecoratorStmt
	// compDepth guards against interpreting an إذا as a ternary while parsing
	// inside a comprehension clause (where إذا introduces a filter).
	compDepth int
	// parenDepth tracks how deep we are inside «(...)» (parenthesized
	// expressions and argument lists). Yield (بساز/بساز‌از) is treated as an
	// expression only at parenDepth > 0, so the statement forms «expr بساز»
	// and «expr بساز‌از» remain unambiguous (L19).
	parenDepth int
	// condDepth is > 0 while parsing a condition (if/while conditions and the
	// ternary «... اگر cond باشد ...»). In those contexts نباشد is the suffix
	// copula and is consumed by the condition parser, NOT as a postfix
	// negation expression.
	condDepth int
}

// New creates a parser for the given token slice.
func New(toks []token.Token) *Parser {
	return &Parser{toks: toks}
}

// ParseProgram parses a full source string into a program (list of statements).
func ParseProgram(src string) ([]ast.Stmt, error) {
	toks := lexer.Lex(src)
	p := New(toks)
	stmts, err := p.parseProgram()
	if err != nil {
		return nil, err
	}
	return stmts, nil
}

// ParseSingleExpr parses a single expression from source (used for string
// interpolation and the REPL). It must be the only statement on the line.
func ParseSingleExpr(src string) (ast.Expr, error) {
	toks := lexer.Lex(src)
	p := New(toks)
	e, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.peek().Type == token.NEWLINE {
		p.next()
	}
	if p.peek().Type != token.EOF {
		return nil, p.errf("انتظار پایان عبارت بود، اما %q دیده شد", p.peek().Literal)
	}
	return e, nil
}

func (p *Parser) peek() token.Token {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return token.Token{Type: token.EOF}
}

func (p *Parser) peekAt(n int) token.Token {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n]
	}
	return token.Token{Type: token.EOF}
}

func (p *Parser) next() token.Token {
	t := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

func (p *Parser) match(t token.Type) bool {
	if p.peek().Type == t {
		p.next()
		return true
	}
	return false
}

func (p *Parser) expect(t token.Type) (token.Token, error) {
	tk := p.peek()
	if tk.Type != t {
		return tk, p.errf("انتظار %s بود، اما %q دیده شد", persianType(t), tk.Literal)
	}
	return p.next(), nil
}

func (p *Parser) errf(format string, args ...interface{}) error {
	ln := p.peek().Line
	return fmt.Errorf("خطای نحو در خط %d: %s", ln, fmt.Sprintf(format, args...))
}

// persianType returns the Persian display name of a token type so error
// messages like «انتظار دو نقطه بود» are understandable for beginners.
func persianType(t token.Type) string {
	switch t {
	case token.NEWLINE:
		return "خط جدید"
	case token.INDENT:
		return "تورفتگی"
	case token.DEDENT:
		return "پایان بلوک"
	case token.LPAREN:
		return "پرانتز باز"
	case token.RPAREN:
		return "پرانتز بسته"
	case token.LBRACKET:
		return "کروشه باز"
	case token.RBRACKET:
		return "کروشه بسته"
	case token.LBRACE:
		return "آکولاد باز"
	case token.RBRACE:
		return "آکولاد بسته"
	case token.COLON:
		return "دو نقطه"
	case token.ASSIGN:
		return "نشانه ="
	case token.APPEND:
		return "بیافزا"
	case token.REMOVE:
		return "حذف‌کن"
	case token.TO:
		return "تا"
	case token.IN:
		return "در"
	case token.IMPORT:
		return "بیار"
	case token.COP_POS:
		return "باشد"
	case token.ELSE:
		return "وگرنه"
	case token.SEP:
		return "و"
	case token.IDENT:
		return "نام"
	case token.EOF:
		return "پایان فایل"
	default:
		return string(t)
	}
}

// readTypeAnnUntil reads the raw text of a type annotation, accumulating token
// literals until the given stop token type (which is NOT consumed). It supports
// simple names («صحیح») and composite/tuple annotations («( صحیح و خطا )»).
// Parens are tracked so «)» inside a tuple is part of the annotation.
func (p *Parser) readTypeAnnUntil(stop token.Type) string {
	var parts []string
	depth := 0
	for {
		tk := p.peek()
		switch tk.Type {
		case token.LPAREN:
			depth++
			parts = append(parts, tk.Literal)
			p.next()
		case token.RPAREN:
			if depth > 0 {
				depth--
				parts = append(parts, tk.Literal)
				p.next()
				continue
			}
			return strings.Join(parts, " ")
		case token.NEWLINE, token.DEDENT, token.EOF:
			return strings.Join(parts, " ")
		default:
			if tk.Type == stop {
				// «و» inside a tuple is part of the type; at depth 0 it is the
				// caller's separator and terminates.
				if stop == token.SEP && depth > 0 {
					parts = append(parts, tk.Literal)
					p.next()
					continue
				}
				return strings.Join(parts, " ")
			}
			parts = append(parts, tk.Literal)
			p.next()
		}
	}
}

// parseProgram parses statements until EOF, skipping stray NEWLINE/DEDENT
// tokens at the top level.
func (p *Parser) parseProgram() ([]ast.Stmt, error) {
	var stmts []ast.Stmt
	for p.peek().Type != token.EOF {
		if p.peek().Type == token.NEWLINE {
			p.next()
			continue
		}
		if p.peek().Type == token.DEDENT {
			p.next()
			continue
		}
		s, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if s != nil {
			stmts = append(stmts, s)
		}
		// Block statements (def/if/for/while) consume their DEDENT and the
		// next statement may follow directly; simple statements are followed
		// by a NEWLINE token. We are lenient here because the lexer skips
		// blank lines entirely.
		if p.peek().Type == token.NEWLINE {
			p.next()
		}
	}
	return stmts, nil
}

// parseStatement dispatches on the leading token.
func (p *Parser) parseStatement() (ast.Stmt, error) {
	tk := p.peek()
	switch tk.Type {
	case token.DEF:
		return p.parseDef()
	case token.IF:
		return p.parseIf()
	case token.WHILE:
		return p.parseWhile()
	case token.FOR:
		return p.parseFor()
	case token.WITH:
		return p.parseWith()
	case token.FROM:
		return p.parseFromImport()
	case token.BREAK:
		p.next()
		return &ast.BreakStmt{L: tk.Line}, nil
	case token.CONTINUE:
		p.next()
		return &ast.ContinueStmt{L: tk.Line}, nil
	case token.PASS:
		p.next()
		return nil, nil // مثل (pass): a no-op statement, skipped entirely
	case token.CLASS:
		return p.parseClassDef()
	case token.INTERF:
		return p.parseInterfaceDef()
	case token.TRY:
		return p.parseTry()
	case token.FINALLY, token.RAISE:
		return nil, fmt.Errorf("خط %d: %q فقط داخل بلوک بپا مجاز است", tk.Line, tk.Literal)
	case token.YIELD, token.YIELDFROM:
		return nil, fmt.Errorf("خط %d: «%s» به‌تنهایی فقط بعد از یک عبارت («عبارت %s») یا داخل پرانتز مجاز است", tk.Line, tk.Literal, tk.Literal)
	case token.GO:
		p.next() // برو
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &ast.GoStmt{L: tk.Line, Expr: expr}, nil
	case token.GLOBAL, token.NONLOCAL:
		kind := tk.Type
		p.next() // جهانی / نامحلی
		names := []string{}
		for {
			nm, err := p.expect(token.IDENT)
			if err != nil {
				return nil, err
			}
			names = append(names, nm.Literal)
			if !p.match(token.SEP) {
				break
			}
		}
		if kind == token.GLOBAL {
			return &ast.GlobalStmt{L: tk.Line, Names: names}, nil
		}
		return &ast.NonlocalStmt{L: tk.Line, Names: names}, nil
	case token.DECOR:
		// پوشش is a parse-time marker only: it stashes a decorator and is
		// consumed by the immediately-following تعریف. It emits no runtime
		// statement.
		if _, err := p.parseDecorator(); err != nil {
			return nil, err
		}
		return nil, nil
	case token.NEWLINE:
		p.next()
		return nil, nil
	default:
		return p.parseExprStatement()
	}
}

// parseExprStatement parses expression-based statements: assignments,
// imperative verbs (print/input/return/import/append/remove), and bare calls.
func (p *Parser) parseExprStatement() (ast.Stmt, error) {
	// Parse an expression list separated by «و» (value list for multi-assign /
	// multi-return).
	var exprs []ast.Expr
	for {
		e, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, e)
		if p.peek().Type == token.SEP {
			p.next()
			continue
		}
		break
	}
	line := exprs[0].Line()
	next := p.peek()

	// Channel send: «ch << value» (single channel expression on the left).
	if next.Type == token.SEND {
		if len(exprs) != 1 {
			return nil, p.errf("ارسال به کانال (<<) فقط یک عبارت کانال می‌پذیرد")
		}
		p.next() // <<
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &ast.SendStmt{L: line, Channel: exprs[0], Value: val}, nil
	}

	// Channel close (verb-final): «ch ببند».
	if next.Type == token.CLOSE {
		if len(exprs) != 1 {
			return nil, p.errf("بستن کانال (ببند) فقط یک عبارت کانال می‌پذیرد")
		}
		p.next() // ببند
		return &ast.CloseStmt{L: line, Channel: exprs[0]}, nil
	}

	// Postfix defer: «EXPR تأخیری»
	if next.Type == token.DEFER {
		p.next()
		if len(exprs) != 1 {
			return nil, p.errf("تأخیری فقط یک عبارت فراخوانی می‌پذیرد")
		}
		return &ast.DeferStmt{L: line, Call: exprs[0]}, nil
	}

	// Typed annotation: ident : TYPE = value
	if len(exprs) == 1 && next.Type == token.COLON {
		if id, ok := exprs[0].(*ast.Ident); ok {
			p.next() // consume ':'
			ann := p.readTypeAnnUntil(token.ASSIGN)
			// optional assignment
			if p.peek().Type == token.ASSIGN {
				p.next()
				val, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				return &ast.Assign{L: id.L, Target: id, Value: val, Ann: ann}, nil
			}
			return nil, p.errf("اعلان نوعدار باید مقداردهی شود")
		}
	}

	switch next.Type {
	case token.ASSIGN, token.PLUS_EQ, token.MINUS_EQ, token.STAR_EQ,
		token.DIV_EQ, token.FLOORDIV_EQ, token.POW_EQ, token.PERCENT_EQ:
		return p.parseAssignment(exprs, next)
	case token.IMPORT: // «ریاضی بیار»
		p.next()
		if len(exprs) != 1 {
			return nil, p.errf("بیار فقط یک نام ماژول می‌پذیرد")
		}
		id, ok := exprs[0].(*ast.Ident)
		if !ok {
			return nil, p.errf("بیار یک نام ماژول می‌خواهد")
		}
		return &ast.ImportStmt{L: line, Module: id.Name}, nil
	case token.RETURN:
		p.next()
		return &ast.ReturnStmt{L: line, Vals: exprs}, nil
	case token.RAISE:
		p.next()
		if len(exprs) != 1 {
			return nil, p.errf("بده فقط یک استثنا می‌پذیرد")
		}
		return &ast.RaiseStmt{L: line, Value: exprs[0]}, nil
	case token.PRINT:
		p.next()
		return &ast.PrintStmt{L: line, Args: exprs}, nil
	case token.INPUT:
		p.next()
		if len(exprs) != 1 {
			return nil, p.errf("بگیر فقط یک نام می‌پذیرد")
		}
		return &ast.InputStmt{L: line, Target: exprs[0]}, nil
	case token.BEH:
		p.next()
		obj, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.APPEND); err != nil {
			return nil, err
		}
		return &ast.AppendStmt{L: line, List: obj, Value: exprs[0]}, nil
	case token.FROM: // از ... حذفکن
		p.next()
		obj, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.REMOVE); err != nil {
			return nil, err
		}
		return &ast.RemoveStmt{L: line, List: obj, Value: exprs[0]}, nil
	case token.NEWLINE, token.DEDENT, token.EOF:
		if len(exprs) != 1 {
			return nil, p.errf("فهرست عبارت «و» غیرمنتظره است")
		}
		return &ast.ExprStmt{L: line, Expr: exprs[0]}, nil
	case token.YIELD: // «expr بساز» — yield (verb-final)
		p.next()
		if len(exprs) != 1 {
			return nil, p.errf("بساز فقط یک مقدار می‌پذیرد")
		}
		return &ast.YieldStmt{L: line, Value: exprs[0]}, nil
	case token.YIELDFROM: // «expr بساز‌از» — yield-from (verb-final)
		p.next()
		if len(exprs) != 1 {
			return nil, p.errf("بساز‌از فقط یک مقدار می‌پذیرد")
		}
		return &ast.YieldFromStmt{L: line, Value: exprs[0]}, nil
	default:
		return nil, p.errf("نماد غیرمنتظره %q بعد از عبارت", next.Literal)
	}
}

// parseDecorator parses «پوشش NAME» or «پوشش NAME(args)». It stashes the
// decorator and returns it as a marker statement; the immediately-following
// تعریف consumes it. The parser enforces that a decorator is directly followed
// by a function definition.
func (p *Parser) parseDecorator() (ast.Stmt, error) {
	decorTok := p.next() // پوشش
	nameTok := p.peek()
	if nameTok.Type != token.IDENT {
		return nil, p.errf("بعد از پوشش باید نام پوشش‌دهنده بیاید، اما %q دیده شد", nameTok.Literal)
	}
	p.next()
	dec := &ast.DecoratorStmt{L: decorTok.Line, Name: nameTok.Literal}
	if p.peek().Type == token.LPAREN {
		args, _, err := p.parseArgs()
		if err != nil {
			return nil, err
		}
		dec.Args = args
	}
	// Enforce that the next real statement is another پوشش or a تعریف.
	if !p.nextIsDefOrDecor() {
		return nil, p.errf("بعد از پوشش باید بلافاصله تعریف (تابع) بیاید")
	}
	p.pendingDecorators = append(p.pendingDecorators, dec)
	return dec, nil
}

// nextIsDefOrDecor reports whether the next non-structural token is a تعریف
// (a decorated function) or another پوشش (a stacked decorator).
func (p *Parser) nextIsDefOrDecor() bool {
	for i := p.pos; i < len(p.toks); i++ {
		switch p.toks[i].Type {
		case token.NEWLINE, token.INDENT, token.DEDENT:
			continue
		case token.DEF, token.DECOR:
			return true
		default:
			return false
		}
	}
	return false
}

// parseAssignment handles `=` and compound assignment.
func (p *Parser) parseAssignment(targets []ast.Expr, op token.Token) (ast.Stmt, error) {
	p.next() // consume op
	// parse RHS value list (separated by و)
	var values []ast.Expr
	for {
		e, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		values = append(values, e)
		if p.peek().Type == token.SEP {
			p.next()
			continue
		}
		break
	}

	if op.Type == token.ASSIGN {
		if len(targets) == 1 && len(values) == 1 {
			return &ast.Assign{L: targets[0].Line(), Target: targets[0], Value: values[0]}, nil
		}
		if len(values) != 1 && len(targets) != len(values) {
			return nil, p.errf("تعداد مقصدها و مقدارها یکی نیست: %d در برابر %d", len(targets), len(values))
		}
		return &ast.MultiAssign{L: targets[0].Line(), Targets: targets, Values: values}, nil
	}
	// compound
	if len(targets) != 1 {
		return nil, p.errf("تخصیص ترکیبی فقط یک مقصد می‌پذیرد")
	}
	return &ast.CompoundAssign{L: targets[0].Line(), Op: string(op.Type), Target: targets[0], Value: values[0]}, nil
}

// parseDef parses: تعریف name(params) -> type: body
func (p *Parser) parseDef() (ast.Stmt, error) {
	defTok := p.next() // تعریف
	nameTok := p.peek()
	if nameTok.Type != token.IDENT {
		return nil, p.errf("بعد از تعریف باید نام تابع بیاید، اما %q دیده شد", nameTok.Literal)
	}
	p.next()
	st, err := p.parseFuncSig(nameTok)
	if err != nil {
		return nil, err
	}
	st.L = defTok.Line
	st.Decorators = p.pendingDecorators
	p.pendingDecorators = nil
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	st.Body = body
	return st, nil
}

// parseFuncSig parses the signature after a function/method name:
// `(params) [-> type]`. It does NOT consume the body. It returns a DefStmt
// with Body set to nil. Used for regular functions, class methods, the
// constructor (ساخت), and interface method signatures.
func (p *Parser) parseFuncSig(nameTok token.Token) (*ast.DefStmt, error) {
	st := &ast.DefStmt{L: nameTok.Line, Name: nameTok.Literal}
	if _, err := p.expect(token.LPAREN); err != nil {
		return nil, err
	}
	var params []*ast.Param
	if p.peek().Type != token.RPAREN {
		for {
			ptok := p.peek()
			// Varargs / keyword-varargs: «*args» and «**kwargs» (spec §5.7).
			// «*» is the POW token; two POWs in a row mean «**».
			if ptok.Type == token.POW {
				isKw := false
				marker := "*"
				if p.peekAt(1).Type == token.POW {
					isKw = true
					marker = "**"
					p.next() // first *
					p.next() // second *
				} else {
					p.next() // *
				}
				nm := p.peek()
				if nm.Type != token.IDENT && nm.Type != token.SELF {
					return nil, p.errf("بعد از %s باید نام پارامتر بیاید، اما %q دیده شد", marker, nm.Literal)
				}
				p.next()
				prm := &ast.Param{L: nm.Line, Name: nm.Literal, Kwargs: isKw, Variadic: !isKw}
				params = append(params, prm)
				if p.match(token.SEP) {
					continue
				}
				break
			}
			if ptok.Type != token.IDENT && ptok.Type != token.SELF {
				return nil, p.errf("انتظار نام پارامتر بود، اما %q دیده شد", ptok.Literal)
			}
			p.next()
			prm := &ast.Param{L: ptok.Line, Name: ptok.Literal}
			// optional type annotation
			if p.peek().Type == token.COLON {
				p.next()
				prm.Ann = p.readTypeAnnUntil(token.SEP)
			}
			// optional default
			if p.peek().Type == token.ASSIGN {
				p.next()
				dv, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				prm.Default = dv
				prm.HasDefault = true
			}
			params = append(params, prm)
			if p.match(token.SEP) {
				continue
			}
			break
		}
	}
	if _, err := p.expect(token.RPAREN); err != nil {
		return nil, err
	}
	// optional return type annotation: -> type
	if p.peek().Type == token.ARROW {
		p.next()
		st.RetType = p.readTypeAnnUntil(token.COLON)
	}
	st.Params = params
	return st, nil
}

// parseClassDef parses a class definition:
//
//	گونه NAME:
//	گونه NAME وارث PARENT:
//	گونه NAME رهی IFACE:
//	گونه NAME وارث PARENT رهی IFACE:
func (p *Parser) parseClassDef() (ast.Stmt, error) {
	cls := p.next() // گونه
	nameTok := p.peek()
	if nameTok.Type != token.IDENT {
		return nil, p.errf("بعد از گونه باید نام کلاس بیاید، اما %q دیده شد", nameTok.Literal)
	}
	p.next()
	cd := &ast.ClassDef{L: cls.Line, Name: nameTok.Literal}
	for {
		switch p.peek().Type {
		case token.EXTENDS: // وارث
			p.next()
			pt := p.peek()
			if pt.Type != token.IDENT {
				return nil, p.errf("بعد از وارث باید نام کلاس والد بیاید، اما %q دیده شد", pt.Literal)
			}
			p.next()
			cd.Parent = pt.Literal
		case token.IMPLEMENTS: // رهی
			p.next()
			it := p.peek()
			if it.Type != token.IDENT {
				return nil, p.errf("بعد از رهی باید نام رابط بیاید، اما %q دیده شد", it.Literal)
			}
			p.next()
			cd.Implements = it.Literal
		default:
			goto done
		}
	}
done:
	body, err := p.parseClassBlock()
	if err != nil {
		return nil, err
	}
	cd.Body = body.Stmts
	return cd, nil
}

// parseClassBlock parses the indented body of a class. It mirrors parseBlock
// but treats a bare «ساخت (...):» as a constructor definition (a method named
// ساخت) in addition to the regular تعریف methods.
func (p *Parser) parseClassBlock() (*ast.Block, error) {
	colon := p.peek()
	if _, err := p.expect(token.COLON); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.NEWLINE); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.INDENT); err != nil {
		return nil, err
	}
	blk := &ast.Block{L: colon.Line}
	for p.peek().Type != token.DEDENT && p.peek().Type != token.EOF {
		if p.peek().Type == token.NEWLINE {
			p.next()
			continue
		}
		s, err := p.parseClassStatement()
		if err != nil {
			return nil, err
		}
		if s != nil {
			blk.Stmts = append(blk.Stmts, s)
		}
		if p.peek().Type == token.NEWLINE {
			p.next()
		}
	}
	if _, err := p.expect(token.DEDENT); err != nil {
		return nil, err
	}
	return blk, nil
}

// parseClassStatement parses one statement inside a class body: a method
// definition (تعریف), a decorator (پوشش), a constructor (ساخت), a no-op
// (مثل), or a class-level expression/assignment statement.
func (p *Parser) parseClassStatement() (ast.Stmt, error) {
	tk := p.peek()
	if tk.Type == token.PASS {
		p.next()
		return nil, nil // مثل (pass): a no-op, skipped entirely
	}
	if tk.Type == token.DEF {
		return p.parseDef()
	}
	// پوشش (decorator): stash it; the following تعریف consumes it. Mirrors the
	// statement-level DECOR handling in parseStatement.
	if tk.Type == token.DECOR {
		if _, err := p.parseDecorator(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	// Constructor: ساخت ( params ) : — a method named ساخت without the تعریف
	// keyword, mirroring the spec examples.
	if tk.Type == token.IDENT && tk.Literal == "ساخت" && p.peekAt(1).Type == token.LPAREN {
		p.next() // consume ساخت
		st, err := p.parseFuncSig(tk)
		if err != nil {
			return nil, err
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		st.Body = body
		return st, nil
	}
	return p.parseExprStatement()
}

// parseInterfaceDef parses an interface (رابط):
//
//	رابط <NAME>:
//	    تعریف method(params) [-> type]
//	    ...
func (p *Parser) parseInterfaceDef() (ast.Stmt, error) {
	it := p.next() // رابط
	nameTok := p.peek()
	if nameTok.Type != token.IDENT {
		return nil, p.errf("بعد از رابط باید نام رابط بیاید، اما %q دیده شد", nameTok.Literal)
	}
	p.next()
	id := &ast.InterfaceDef{L: it.Line, Name: nameTok.Literal}
	if _, err := p.expect(token.COLON); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.NEWLINE); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.INDENT); err != nil {
		return nil, err
	}
	for p.peek().Type != token.DEDENT && p.peek().Type != token.EOF {
		if p.peek().Type == token.NEWLINE {
			p.next()
			continue
		}
		if p.peek().Type != token.DEF {
			return nil, p.errf("داخل رابط فقط تعریف متد مجاز است، اما %q دیده شد", p.peek().Literal)
		}
		p.next() // تعریف
		mname := p.peek()
		if mname.Type != token.IDENT {
			return nil, p.errf("در رابط بعد از تعریف باید نام متد بیاید، اما %q دیده شد", mname.Literal)
		}
		p.next()
		st, err := p.parseFuncSig(mname)
		if err != nil {
			return nil, err
		}
		id.Methods = append(id.Methods, st)
		if p.peek().Type == token.NEWLINE {
			p.next()
		}
	}
	if _, err := p.expect(token.DEDENT); err != nil {
		return nil, err
	}
	return id, nil
}

// parseIf parses: اگر <cond> باشد: body وگرنه اگر ... / وگرنه:
func (p *Parser) parseIf() (ast.Stmt, error) {
	ifTok := p.next() // اگر
	cond, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	st := &ast.IfStmt{L: ifTok.Line, Cond: cond, Body: body.Stmts}

	for p.peek().Type == token.ELSE {
		p.next() // وگرنه
		if p.match(token.IF) {
			ec, err := p.parseCondition()
			if err != nil {
				return nil, err
			}
			eb, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			st.Elifs = append(st.Elifs, &ast.ElifBranch{L: eb.L, Cond: ec, Body: eb})
			continue
		}
		// plain else
		eb, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		st.Else = eb
		break
	}
	return st, nil
}

// parseWhile parses «تاوقتی <cond> باشد: body».
func (p *Parser) parseWhile() (ast.Stmt, error) {
	wTok := p.next() // تاوقتی
	cond, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.WhileStmt{L: wTok.Line, Cond: cond, Body: body}, nil
}

// parseFor parses a for loop. The loop header is either a numeric range
// «برای VAR از START تا END [گام STEP]» or an iteration «برای VARS در ITER»;
// both are dispatched on the token following the loop variables.
func (p *Parser) parseFor() (ast.Stmt, error) {
	fTok := p.next() // برای
	// variable names, separated by و — or a parenthesized tuple for unpacking:
	// «برای (a و b) در iterable»
	var vars []ast.Expr
	if p.peek().Type == token.LPAREN {
		p.next() // (
		for {
			tk := p.peek()
			if tk.Type != token.IDENT && tk.Type != token.SELF {
				return nil, p.errf("در پرانتز باید نام متغیر حلقه بیاید، اما %q دیده شد", tk.Literal)
			}
			p.next()
			vars = append(vars, &ast.Ident{L: tk.Line, Name: tk.Literal})
			if p.match(token.SEP) {
				continue
			}
			break
		}
		if _, err := p.expect(token.RPAREN); err != nil {
			return nil, err
		}
	} else {
		for {
			tk := p.peek()
			if tk.Type != token.IDENT && tk.Type != token.SELF {
				return nil, p.errf("انتظار نام متغیر حلقه بود، اما %q دیده شد", tk.Literal)
			}
			p.next()
			vars = append(vars, &ast.Ident{L: tk.Line, Name: tk.Literal})
			if p.match(token.SEP) {
				continue
			}
			break
		}
	}

	switch p.peek().Type {
	case token.FROM: // برای ای از ۰ تا ۱۰ [گام ۲]
		p.next() // از
		start, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.TO); err != nil {
			return nil, err
		}
		end, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		var step ast.Expr
		if p.peek().Type == token.STEP {
			p.next()
			step, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &ast.ForRange{L: fTok.Line, Var: vars[0], Start: start, End: end, Step: step, Body: body}, nil
	case token.IN: // برای ای در iterable
		p.next() // در
		iter, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &ast.ForIn{L: fTok.Line, Vars: vars, Iter: iter, Body: body}, nil
	default:
		return nil, p.errf("در حلقه برای باید «از» یا «در» بیاید، اما %q دیده شد", p.peek().Literal)
	}
}

// parseWith parses a context-manager statement:
//
//	با EXPR بانام NAME:
//	    body
//
// The بانام NAME part is optional (a bare «با EXPR:» is accepted).
func (p *Parser) parseWith() (ast.Stmt, error) {
	wTok := p.next() // با
	ctx, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	name := ""
	if p.peek().Type == token.AS { // بانام
		p.next()
		nm := p.peek()
		if nm.Type != token.IDENT && nm.Type != token.SELF {
			return nil, p.errf("بعد از بانام باید نام بیاید، اما %q دیده شد", nm.Literal)
		}
		p.next()
		name = nm.Literal
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.WithStmt{L: wTok.Line, Context: ctx, Name: name, Body: body.Stmts}, nil
}

// parseFromImport parses «از Module Name [بانام Alias] بیار».
func (p *Parser) parseFromImport() (ast.Stmt, error) {
	fTok := p.next() // از
	modTok := p.peek()
	if modTok.Type != token.IDENT {
		return nil, p.errf("انتظار نام ماژول بود، اما %q دیده شد", modTok.Literal)
	}
	p.next()
	nameTok := p.peek()
	if nameTok.Type != token.IDENT {
		return nil, p.errf("انتظار نام برای بیار بود، اما %q دیده شد", nameTok.Literal)
	}
	p.next()
	st := &ast.FromImportStmt{L: fTok.Line, Module: modTok.Literal, Name: nameTok.Literal}
	if p.match(token.AS) { // بانام
		alias := p.peek()
		if alias.Type != token.IDENT {
			return nil, p.errf("بعد از بانام باید نام دیگر بیاید، اما %q دیده شد", alias.Literal)
		}
		p.next()
		st.Alias = alias.Literal
	}
	if _, err := p.expect(token.IMPORT); err != nil {
		return nil, err
	}
	return st, nil
}

// parseTry parses a try/except/finally block:
//
//	بپا:
//	    ...
//	خطای‌صفر بگیر:
//	    ...
//	خطای‌نوع بگیر بانام err:
//	    ...
//	درنهایت:
//	    ...
func (p *Parser) parseTry() (ast.Stmt, error) {
	tryTok := p.next() // بپا
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	st := &ast.TryStmt{L: tryTok.Line, Body: body}

	// One or more except clauses: <TYPE> بگیر [بانام name]: or a bare بگیر:
	for {
		tk := p.peek()
		if tk.Type == token.INPUT && p.peekAt(1).Type == token.COLON {
			// bare «بگیر:» — catches every exception
			p.next()
			h := &ast.ExceptHandler{L: tk.Line}
			hb, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			h.Body = hb
			st.Handlers = append(st.Handlers, h)
			continue
		}
		if tk.Type == token.IDENT && p.peekAt(1).Type == token.INPUT {
			typeExpr, err := p.parseExpression() // the exception type name
			if err != nil {
				return nil, err
			}
			h := &ast.ExceptHandler{L: tk.Line, Exception: typeExpr}
			p.next()                       // بگیر
			if p.peek().Type == token.AS { // بانام
				p.next()
				alias := p.peek()
				if alias.Type != token.IDENT {
					return nil, p.errf("بعد از بانام باید نام دیگر بیاید، اما %q دیده شد", alias.Literal)
				}
				p.next()
				h.Alias = alias.Literal
			}
			hb, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			h.Body = hb
			st.Handlers = append(st.Handlers, h)
			continue
		}
		break
	}

	if p.peek().Type == token.FINALLY { // درنهایت
		p.next()
		fb, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		st.Finally = fb
	}
	return st, nil
}

// parseCondition parses a condition expression and requires the copula
// باشد/نباشد followed by ':'.
func (p *Parser) parseCondition() (ast.Expr, error) {
	p.condDepth++
	defer func() { p.condDepth-- }()
	e, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	switch p.peek().Type {
	case token.COP_POS: // باشد
		p.next()
		if err := p.checkCondition(e); err != nil {
			return nil, err
		}
		return e, nil
	case token.COP_NEG: // نباشد
		p.next()
		neg := &ast.Unary{L: e.Line(), Op: "not", Expr: e}
		if err := p.checkCondition(neg); err != nil {
			return nil, err
		}
		return neg, nil
	default:
		return nil, p.errf("شرط باید با باشد یا نباشد تمام شود")
	}
}

// checkCondition enforces the no-implicit-truthiness rule (spec §17.7): a
// condition must be an explicit comparison, a membership test (در), a boolean
// literal (درست/غلط), or a negation — a bare variable/expression is a syntax
// error.
func (p *Parser) checkCondition(e ast.Expr) error {
	switch t := e.(type) {
	case *ast.BoolLit:
		return nil
	case *ast.Unary:
		return nil
	case *ast.BinaryOp:
		if isComparisonOp(t.Op) {
			return nil
		}
	}
	return p.errf("شرط باید مقایسه یا عبارت بولی باشد")
}

// isComparisonOp reports whether op is a comparison/membership operator that
// qualifies as an explicit condition.
func isComparisonOp(op string) bool {
	switch op {
	case "==", "<", ">", "<=", ">=", "در":
		return true
	}
	return false
}

// parseBlock expects ':' NEWLINE INDENT stmts DEDENT.
func (p *Parser) parseBlock() (*ast.Block, error) {
	colon := p.peek()
	if _, err := p.expect(token.COLON); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.NEWLINE); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.INDENT); err != nil {
		return nil, err
	}
	blk := &ast.Block{L: colon.Line}
	for p.peek().Type != token.DEDENT && p.peek().Type != token.EOF {
		if p.peek().Type == token.NEWLINE {
			p.next()
			continue
		}
		s, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if s != nil {
			blk.Stmts = append(blk.Stmts, s)
		}
		if p.peek().Type == token.NEWLINE {
			p.next()
		}
	}
	if _, err := p.expect(token.DEDENT); err != nil {
		return nil, err
	}
	return blk, nil
}

// --- Expressions (Pratt) ---
//
// Precedence levels for binary operators, from loosest to tightest. The pipe
// operator (|>) is lowest; power binds tighter than unary minus and is
// right-associative.
const (
	precLowest = 0
	precPIPE   = iota + 1
	precOR
	precAND
	precCMP // == < > <= >= and در
	precADD
	precMUL
	precPOW
)

// binopInfo describes a binary operator's precedence and associativity.
type binopInfo struct {
	prec       int
	rightAssoc bool
}

// binops maps each binary operator token to its precedence and associativity,
// driving the Pratt parser's precedence climbing.
var binops = map[token.Type]binopInfo{
	token.PIPE:     {precPIPE, false},
	token.OR:       {precOR, false},
	token.AND:      {precAND, false},
	token.EQ:       {precCMP, false},
	token.LT:       {precCMP, false},
	token.GT:       {precCMP, false},
	token.LTE:      {precCMP, false},
	token.GTE:      {precCMP, false},
	token.IN:       {precCMP, false},
	token.PLUS:     {precADD, false},
	token.MINUS:    {precADD, false},
	token.STAR:     {precMUL, false},
	token.DIV:      {precMUL, false},
	token.FLOORDIV: {precMUL, false},
	token.PERCENT:  {precMUL, false},
	token.POW:      {precPOW, true},
}

func (p *Parser) parseExpression() (ast.Expr, error) {
	return p.parseBinary(precLowest)
}

// parseBinary parses an expression using precedence climbing: it parses a
// prefix expression, then repeatedly merges any binary operator with
// precedence >= minPrec. It also handles the low-precedence pipe operator
// (building a PipeExpr), postfix logical negation (نباشد), and the ternary
// «... اگر cond باشد وگرنه ...».
func (p *Parser) parseBinary(minPrec int) (ast.Expr, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	for {
		tk := p.peek()
		// Pipe is the lowest-precedence operator and builds a PipeExpr rather
		// than a generic BinaryOp. It still respects minPrec so it stays
		// left-associative.
		if tk.Type == token.PIPE && precPIPE >= minPrec {
			p.next()
			right, err := p.parseBinary(precPIPE + 1)
			if err != nil {
				return nil, err
			}
			left = &ast.PipeExpr{L: left.Line(), Left: left, Right: right}
			continue
		}
		info, ok := binops[tk.Type]
		if !ok || info.prec < minPrec {
			break
		}
		p.next()
		nextPrec := info.prec + 1
		if info.rightAssoc {
			nextPrec = info.prec
		}
		right, err := p.parseBinary(nextPrec)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{L: left.Line(), Op: string(tk.Type), Left: left, Right: right}
	}
	// Postfix logical negation: «x نباشد» = not x (spec §3.2). In condition
	// contexts (condDepth > 0) نباشد is the suffix copula and is consumed by
	// the condition parser instead.
	if p.condDepth == 0 && p.peek().Type == token.COP_NEG {
		p.next()
		left = &ast.Unary{L: left.Line(), Op: "not", Expr: left}
	}
	// Ternary: «true اگر cond باشد وگرنه false». In expression context (after a
	// value), اگر starts a ternary. Condition requires the copula باشد. Not
	// inside a comprehension clause, where اگر is a filter marker.
	if p.compDepth == 0 && p.peek().Type == token.IF {
		p.next()
		p.condDepth++
		cond, err := p.parseExpression()
		p.condDepth--
		if err != nil {
			return nil, err
		}
		// The condition ends with the copula باشد (keep) or نباشد (negate).
		switch p.peek().Type {
		case token.COP_POS:
			p.next()
		case token.COP_NEG:
			p.next()
			cond = &ast.Unary{L: cond.Line(), Op: "not", Expr: cond}
		default:
			return nil, p.errf("شرط باید با باشد یا نباشد تمام شود")
		}
		if _, err := p.expect(token.ELSE); err != nil {
			return nil, err
		}
		falseBranch, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		left = &ast.TernaryExpr{L: left.Line(), Cond: cond, TrueBranch: left, FalseBranch: falseBranch}
	}
	return left, nil
}

// parsePrefix parses a prefix expression: verb-initial yield (inside
// parens), unary minus, prefix logical negation, channel receive (>>), or a
// primary expression with postfixes.
func (p *Parser) parsePrefix() (ast.Expr, error) {
	tk := p.peek()
	// Verb-initial yield used as an expression inside parentheses:
	// «(بساز expr)» / «(بساز‌از expr)». At statement level (parenDepth == 0)
	// parseStatement reports a bare بساز before we get here.
	if (tk.Type == token.YIELD || tk.Type == token.YIELDFROM) && p.parenDepth > 0 {
		p.next()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if tk.Type == token.YIELDFROM {
			return &ast.YieldFromExpr{L: tk.Line, Value: val}, nil
		}
		return &ast.YieldExpr{L: tk.Line, Value: val}, nil
	}
	if tk.Type == token.MINUS {
		p.next()
		operand, err := p.parseBinary(precPOW) // power binds tighter than unary minus
		if err != nil {
			return nil, err
		}
		return &ast.Unary{L: tk.Line, Op: "-", Expr: operand}, nil
	}
	// Prefix logical negation: «نباشد x» = not x (spec §3.2). Comparisons bind
	// tighter, so «نباشد x == ۵» = not (x == ۵).
	if tk.Type == token.COP_NEG {
		p.next()
		operand, err := p.parseBinary(precCMP)
		if err != nil {
			return nil, err
		}
		return &ast.Unary{L: tk.Line, Op: "not", Expr: operand}, nil
	}
	if tk.Type == token.RECV {
		// Channel receive: «>>ch» — a prefix operator over the channel expr.
		p.next()
		operand, err := p.parsePrimaryAndPostfix()
		if err != nil {
			return nil, err
		}
		return &ast.RecvExpr{L: tk.Line, Channel: operand}, nil
	}
	return p.parsePrimaryAndPostfix()
}

// parsePrimaryAndPostfix parses an atom followed by its postfix chain.
func (p *Parser) parsePrimaryAndPostfix() (ast.Expr, error) {
	e, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	return p.parsePostfix(e)
}

// parseReceiver parses the receiver of an ezafe expression. Per the spec
// grammar, the receiver is itself a possessive chain («والدِ خود») but it must
// NOT consume a following call/subscript — those belong to the enclosing
// member access (e.g. جذرِ ریاضی(۱۶) calls sqrt, not ریاضی).
func (p *Parser) parseReceiver() (ast.Expr, error) {
	e, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	// Special case: the receiver may be the super keyword followed by a call
	// «والد()», i.e. «super()». In «ساختِ(خود و پیام) والد()» the parens belong
	// to super(), not to the enclosing method call (so we consume them here).
	// This is limited to والد to avoid disturbing «جذرِ ریاضی(۱۶)».
	if id, ok := e.(*ast.Ident); ok && id.Name == "والد" && p.peek().Type == token.LPAREN {
		args, _, err := p.parseArgs()
		if err != nil {
			return nil, err
		}
		e = &ast.Call{L: id.L, Fn: e, Args: args}
	}
	// The receiver may itself be subscripted (طولِ xs[۰] -> len(xs[0])) but
	// must NOT consume a following call: in «جذرِ ریاضی(۱۶)» the parens bind to
	// the enclosing member access (the method call on «جذر»), not to «ریاضی».
	// The ezafe chain continues only via another EZAFE token.
	for {
		tk := p.peek()
		if tk.Type != token.LBRACKET {
			break
		}
		p.next()
		if p.peek().Type == token.COLON {
			p.next()
			low, err := p.parseSliceComponent()
			if err != nil {
				return nil, err
			}
			var high, step ast.Expr
			if p.peek().Type == token.COLON {
				p.next()
				step, _ = p.parseSliceComponent()
			}
			p.expect(token.RBRACKET)
			e = &ast.Slice{L: tk.Line, Target: e, Low: low, High: high, Step: step}
		} else {
			idx, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if p.match(token.COLON) {
				high, _ := p.parseSliceComponent()
				var step ast.Expr
				if p.peek().Type == token.COLON {
					p.next()
					step, _ = p.parseSliceComponent()
				}
				p.expect(token.RBRACKET)
				e = &ast.Slice{L: tk.Line, Target: e, Low: idx, High: high, Step: step}
			} else {
				p.expect(token.RBRACKET)
				e = &ast.Index{L: tk.Line, Target: e, Index: idx}
			}
		}
	}
	for p.peek().Type == token.EZAFE {
		tk := p.next()
		if p.peek().Type == token.LPAREN {
			args, _, err := p.parseArgs()
			if err != nil {
				return nil, err
			}
			recv, err := p.parseReceiver()
			if err != nil {
				return nil, err
			}
			e = &ast.MethodCall{L: tk.Line, Receiver: recv, Method: e, Args: args}
		} else {
			recv, err := p.parseReceiver()
			if err != nil {
				return nil, err
			}
			e = &ast.MemberAccess{L: tk.Line, Receiver: recv, Attr: e}
		}
	}
	return e, nil
}

// parsePostfix parses the postfix chain of an expression: verb-final yield
// (inside parens), ezafe member/method access, calls, and subscripts/slices.
// Ezafe chains bind right-to-left: in «aِ bِ c», the leftmost name becomes the
// attribute accessed on the innermost receiver.
func (p *Parser) parsePostfix(e ast.Expr) (ast.Expr, error) {
	for {
		tk := p.peek()
		switch tk.Type {
		case token.YIELD, token.YIELDFROM:
			// Verb-final yield as an expression, only inside parentheses
			// («(x بساز)», «گروه(x بساز‌از)», ...). At statement level the
			// caller (parseExprStatement) turns «expr بساز» into a YieldStmt.
			if p.parenDepth == 0 {
				return e, nil
			}
			p.next()
			if tk.Type == token.YIELDFROM {
				return &ast.YieldFromExpr{L: tk.Line, Value: e}, nil
			}
			return &ast.YieldExpr{L: tk.Line, Value: e}, nil
		case token.EZAFE:
			p.next()
			if p.peek().Type == token.LPAREN {
				// method call: nameِ(args)receiver
				args, _, err := p.parseArgs()
				if err != nil {
					return nil, err
				}
				recv, err := p.parseReceiver()
				if err != nil {
					return nil, err
				}
				e = &ast.MethodCall{L: tk.Line, Receiver: recv, Method: e, Args: args}
			} else {
				// attribute access: attrِ receiver
				recv, err := p.parseReceiver()
				if err != nil {
					return nil, err
				}
				e = &ast.MemberAccess{L: tk.Line, Receiver: recv, Attr: e}
			}
		case token.LPAREN:
			args, kwargs, err := p.parseArgs()
			if err != nil {
				return nil, err
			}
			e = &ast.Call{L: tk.Line, Fn: e, Args: args, KwArgs: kwargs}
		case token.LBRACKET:
			p.next()
			if p.peek().Type == token.COLON {
				p.next()
				low, err := p.parseSliceComponent()
				if err != nil {
					return nil, err
				}
				var high, step ast.Expr
				if p.peek().Type == token.COLON {
					p.next()
					step, _ = p.parseSliceComponent()
				}
				p.expect(token.RBRACKET)
				e = &ast.Slice{L: tk.Line, Target: e, Low: low, High: high, Step: step}
			} else {
				idx, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				if p.match(token.COLON) {
					high, _ := p.parseSliceComponent()
					var step ast.Expr
					if p.peek().Type == token.COLON {
						p.next()
						step, _ = p.parseSliceComponent()
					}
					p.expect(token.RBRACKET)
					e = &ast.Slice{L: tk.Line, Target: e, Low: idx, High: high, Step: step}
				} else {
					p.expect(token.RBRACKET)
					e = &ast.Index{L: tk.Line, Target: e, Index: idx}
				}
			}
		default:
			return e, nil
		}
	}
}

// parseSliceComponent parses an optional slice bound (may be empty).
func (p *Parser) parseSliceComponent() (ast.Expr, error) {
	if p.peek().Type == token.RBRACKET || p.peek().Type == token.COLON {
		return nil, nil
	}
	return p.parseExpression()
}

// parseArgs parses (expr و expr ...) and returns positional and keyword args.
func (p *Parser) parseArgs() ([]ast.Expr, []*ast.Kwarg, error) {
	if _, err := p.expect(token.LPAREN); err != nil {
		return nil, nil, err
	}
	p.parenDepth++
	defer func() { p.parenDepth-- }()
	var args []ast.Expr
	var kwargs []*ast.Kwarg
	if p.peek().Type == token.RPAREN {
		p.next()
		return args, kwargs, nil
	}
	for {
		// keyword arg: name = value
		if p.peek().Type == token.IDENT && p.peekAt(1).Type == token.ASSIGN {
			name := p.next()
			p.next() // '='
			val, err := p.parseExpression()
			if err != nil {
				return nil, nil, err
			}
			kwargs = append(kwargs, &ast.Kwarg{L: name.Line, Name: name.Literal, Value: val})
		} else {
			a, err := p.parseExpression()
			if err != nil {
				return nil, nil, err
			}
			args = append(args, a)
		}
		if p.match(token.SEP) {
			continue
		}
		break
	}
	if _, err := p.expect(token.RPAREN); err != nil {
		return nil, nil, err
	}
	return args, kwargs, nil
}

// parseAtom parses a primary expression: literals, identifiers (including the
// special keywords خود / والد / بنویس / بسته‌است / ببند), channel literals,
// and parenthesized/bracketed/braced composites.
func (p *Parser) parseAtom() (ast.Expr, error) {
	tk := p.peek()
	switch tk.Type {
	case token.INT:
		p.next()
		iv, err := strconv.ParseInt(tk.Literal, 10, 64)
		if err != nil {
			return nil, p.errf("عدد صحیح نامعتبر: %q", tk.Literal)
		}
		return &ast.NumberLit{L: tk.Line, Int: true, IntVal: iv}, nil
	case token.FLOAT:
		p.next()
		fv, err := strconv.ParseFloat(tk.Literal, 64)
		if err != nil {
			return nil, p.errf("عدد اعشاری نامعتبر: %q", tk.Literal)
		}
		return &ast.NumberLit{L: tk.Line, FVal: fv}, nil
	case token.STRING:
		p.next()
		return &ast.StrLit{L: tk.Line, Raw: tk.Literal}, nil
	case token.TRUE:
		p.next()
		return &ast.BoolLit{L: tk.Line, Value: true}, nil
	case token.FALSE:
		p.next()
		return &ast.BoolLit{L: tk.Line}, nil
	case token.NONE:
		p.next()
		return &ast.NoneLit{L: tk.Line}, nil
	case token.SELF:
		p.next()
		return &ast.Ident{L: tk.Line, Name: "خود"}, nil
	case token.SUPER:
		p.next()
		return &ast.Ident{L: tk.Line, Name: "والد"}, nil
	case token.IDENT:
		p.next()
		return &ast.Ident{L: tk.Line, Name: tk.Literal}, nil
	case token.PRINT:
		p.next()
		return &ast.Ident{L: tk.Line, Name: "بنویس"}, nil
	case token.CHANNEL:
		return p.parseChannelLit(tk)
	case token.CLOSED:
		// «بسته‌استِ ch» — the attribute name is بسته‌است; the ezafe parsing in
		// parsePostfix supplies the receiver (the channel).
		p.next()
		return &ast.Ident{L: tk.Line, Name: "بسته‌است"}, nil
	case token.CLOSE:
		// «ببند» used as an ezafe attribute/method name (e.g. ببندِ()ف), not
		// as the verb-final close — which parseExprStatement handles directly.
		p.next()
		return &ast.Ident{L: tk.Line, Name: "ببند"}, nil
	case token.LPAREN:
		return p.parseParen()
	case token.LBRACKET:
		return p.parseListLit()
	case token.LBRACE:
		return p.parseDictSetLit()
	default:
		return nil, p.errf("نماد غیرمنتظره %q در عبارت", tk.Literal)
	}
}

// parseChannelLit parses «کانال(TYPE و SIZE)», «کانال(TYPE)», or «کانال()».
// The first argument is a type annotation (runtime-ignored for v0.6) unless it
// is a bare integer, in which case it is the buffer size; the second argument,
// if present, is always the buffer size.
func (p *Parser) parseChannelLit(tk token.Token) (ast.Expr, error) {
	p.next() // کانال
	args, _, err := p.parseArgs()
	if err != nil {
		return nil, err
	}
	ch := &ast.ChannelLit{L: tk.Line}
	switch len(args) {
	case 1:
		if n, ok := args[0].(*ast.NumberLit); ok {
			ch.Size = n
		} else {
			ch.Type = args[0]
		}
	case 2:
		ch.Type = args[0]
		ch.Size = args[1]
	}
	return ch, nil
}

// parseParen parses a parenthesized expression: a plain grouping, a tuple,
// or a generator expression (when followed by برای).
func (p *Parser) parseParen() (ast.Expr, error) {
	lp := p.next() // '('
	p.parenDepth++
	defer func() { p.parenDepth-- }()
	if p.match(token.RPAREN) {
		return &ast.TupleLit{L: lp.Line}, nil
	}
	first, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.peek().Type == token.FOR {
		// generator expression: «( expr برای VAR در ITERABLE ... )»
		clauses, err := p.parseCompClauses()
		if err != nil {
			return nil, err
		}
		p.expect(token.RPAREN)
		return &ast.GenExp{L: lp.Line, Element: first, Clauses: clauses}, nil
	}
	if p.match(token.SEP) {
		// tuple
		items := []ast.Expr{first}
		for p.peek().Type != token.RPAREN {
			e, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			items = append(items, e)
			if !p.match(token.SEP) {
				break
			}
		}
		p.expect(token.RPAREN)
		return &ast.TupleLit{L: lp.Line, Elems: items}, nil
	}
	if _, err := p.expect(token.RPAREN); err != nil {
		return nil, err
	}
	return first, nil
}

// parseListLit parses a list literal, or a list comprehension when the first
// element is followed by برای.
func (p *Parser) parseListLit() (ast.Expr, error) {
	lb := p.next() // [
	var elems []ast.Expr
	if p.match(token.RBRACKET) {
		return &ast.ListLit{L: lb.Line}, nil
	}
	for {
		e, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		elems = append(elems, e)
		if p.peek().Type == token.FOR {
			// list comprehension: «[ expr برای VAR در ITERABLE ... ]»
			clauses, err := p.parseCompClauses()
			if err != nil {
				return nil, err
			}
			p.expect(token.RBRACKET)
			return &ast.ListComp{L: lb.Line, Element: e, Clauses: clauses}, nil
		}
		if p.match(token.SEP) {
			continue
		}
		break
	}
	if _, err := p.expect(token.RBRACKET); err != nil {
		return nil, err
	}
	return &ast.ListLit{L: lb.Line, Elems: elems}, nil
}

// parseCompClauses parses one or more «برای VAR در ITERABLE [إذا COND]» clauses
// of a comprehension. Multiple clauses nest. An optional filter («اگر COND»)
// requires the copula باشد and is stored on the clause it follows.
func (p *Parser) parseCompClauses() ([]*ast.CompClause, error) {
	var clauses []*ast.CompClause
	p.compDepth++
	defer func() { p.compDepth-- }()
	for {
		if p.peek().Type != token.FOR {
			break
		}
		fTok := p.next() // برای
		vTok := p.peek()
		if vTok.Type != token.IDENT && vTok.Type != token.SELF {
			return nil, p.errf("در درک‌لیست انتظار نام متغیر بود، اما %q دیده شد", vTok.Literal)
		}
		p.next()
		if _, err := p.expect(token.IN); err != nil {
			return nil, err
		}
		iterable, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		cl := &ast.CompClause{L: fTok.Line, Name: vTok.Literal, Iterable: iterable}
		if p.peek().Type == token.IF {
			p.next() // اگر
			cond, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.COP_POS); err != nil {
				return nil, err
			}
			cl.Filter = cond
		}
		clauses = append(clauses, cl)
	}
	return clauses, nil
}

// parseDictSetLit parses a braced literal, disambiguating dict literals and
// dict comprehensions (key followed by ':'), set literals, and set
// comprehensions (element followed by برای).
func (p *Parser) parseDictSetLit() (ast.Expr, error) {
	lb := p.next() // {
	if p.match(token.RBRACE) {
		return &ast.DictLit{L: lb.Line}, nil
	}
	first, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.match(token.COLON) {
		// dict: key : value [برای ...]  (dict comprehension) or literal
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.peek().Type == token.FOR {
			clauses, err := p.parseCompClauses()
			if err != nil {
				return nil, err
			}
			p.expect(token.RBRACE)
			return &ast.DictComp{L: lb.Line, Key: first, Value: val, Clauses: clauses}, nil
		}
		keys := []ast.Expr{first}
		vals := []ast.Expr{val}
		for p.match(token.SEP) {
			k, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			p.expect(token.COLON)
			v, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			keys = append(keys, k)
			vals = append(vals, v)
		}
		p.expect(token.RBRACE)
		return &ast.DictLit{L: lb.Line, Keys: keys, Values: vals}, nil
	}
	// set
	if p.peek().Type == token.FOR {
		clauses, err := p.parseCompClauses()
		if err != nil {
			return nil, err
		}
		p.expect(token.RBRACE)
		return &ast.SetComp{L: lb.Line, Element: first, Clauses: clauses}, nil
	}
	elems := []ast.Expr{first}
	for p.match(token.SEP) {
		e, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		elems = append(elems, e)
	}
	p.expect(token.RBRACE)
	return &ast.SetLit{L: lb.Line, Elems: elems}, nil
}
