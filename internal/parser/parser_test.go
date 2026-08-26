package parser

import (
	"strings"
	"testing"

	"github.com/faralidev/kolang/internal/ast"
)

func expectNoError(t *testing.T, src string) []ast.Stmt {
	t.Helper()
	stmts, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("unexpected parse error for %q: %v", src, err)
	}
	return stmts
}

func TestParseHello(t *testing.T) {
	stmts := expectNoError(t, "«سلام دنیا!» بنویس\n")
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if _, ok := stmts[0].(*ast.PrintStmt); !ok {
		t.Fatalf("want PrintStmt, got %T", stmts[0])
	}
}

func TestParseEzafeMethodCall(t *testing.T) {
	src := "صدادهیِ()خود\n"
	stmts := expectNoError(t, src)
	es, ok := stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("want ExprStmt, got %T", stmts[0])
	}
	mc, ok := es.Expr.(*ast.MethodCall)
	if !ok {
		t.Fatalf("want MethodCall, got %T", es.Expr)
	}
	if _, ok := mc.Receiver.(*ast.Ident); !ok {
		t.Errorf("receiver should be Ident, got %T", mc.Receiver)
	}
	if id, ok := mc.Method.(*ast.Ident); !ok || id.Name != "صدادهی" {
		t.Errorf("method = %v, want صدادهی", mc.Method)
	}
}

func TestParseEzavaAttr(t *testing.T) {
	src := "جذرِ ریاضی(۱۶) بنویس\n"
	stmts := expectNoError(t, src)
	ps, ok := stmts[0].(*ast.PrintStmt)
	if !ok {
		t.Fatalf("want PrintStmt, got %T", stmts[0])
	}
	call, ok := ps.Args[0].(*ast.Call)
	if !ok {
		t.Fatalf("want Call, got %T", ps.Args[0])
	}
	ma, ok := call.Fn.(*ast.MemberAccess)
	if !ok {
		t.Fatalf("want MemberAccess, got %T", call.Fn)
	}
	if id, ok := ma.Attr.(*ast.Ident); !ok || id.Name != "جذر" {
		t.Errorf("attr = %v, want جذر", ma.Attr)
	}
}

func TestParseImperativeVerbs(t *testing.T) {
	src := "«x» بنویس\nنام بگیر\nالف + ب برگردان\nریاضی بیار\n"
	stmts := expectNoError(t, src)
	wantTypes := []ast.Stmt{
		&ast.PrintStmt{}, &ast.InputStmt{}, &ast.ReturnStmt{}, &ast.ImportStmt{},
	}
	if len(stmts) != len(wantTypes) {
		t.Fatalf("want %d statements, got %d", len(wantTypes), len(stmts))
	}
	for i, w := range wantTypes {
		if strings.TrimSpace(strings.SplitN(src, "\n", i+2)[i]) == "" {
			continue
		}
		switch w.(type) {
		case *ast.PrintStmt:
			if _, ok := stmts[i].(*ast.PrintStmt); !ok {
				t.Errorf("stmt %d: want PrintStmt, got %T", i, stmts[i])
			}
		case *ast.InputStmt:
			if _, ok := stmts[i].(*ast.InputStmt); !ok {
				t.Errorf("stmt %d: want InputStmt, got %T", i, stmts[i])
			}
		case *ast.ReturnStmt:
			if _, ok := stmts[i].(*ast.ReturnStmt); !ok {
				t.Errorf("stmt %d: want ReturnStmt, got %T", i, stmts[i])
			}
		case *ast.ImportStmt:
			if _, ok := stmts[i].(*ast.ImportStmt); !ok {
				t.Errorf("stmt %d: want ImportStmt, got %T", i, stmts[i])
			}
		}
	}
}

func TestParseIfCopula(t *testing.T) {
	src := "اگر سن == ۱۸ باشد:\n    «بزرگسال» بنویس\nوگرنه اگر سن > ۱۸ باشد:\n    «بزرگ‌تر» بنویس\nوگرنه:\n    «کودک» بنویس\n"
	stmts := expectNoError(t, src)
	is, ok := stmts[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("want IfStmt, got %T", stmts[0])
	}
	if len(is.Elifs) != 1 {
		t.Errorf("want 1 elif, got %d", len(is.Elifs))
	}
	if is.Else == nil {
		t.Errorf("want else block")
	}
}

func TestParseIfNegative(t *testing.T) {
	src := "اگر x == ۵ نباشد:\n    «ok» بنویس\n"
	stmts := expectNoError(t, src)
	is := stmts[0].(*ast.IfStmt)
	un, ok := is.Cond.(*ast.Unary)
	if !ok {
		t.Fatalf("condition should be negated Unary, got %T", is.Cond)
	}
	if un.Op != "not" {
		t.Errorf("unary op = %q, want not", un.Op)
	}
}

func TestParseFunctionDef(t *testing.T) {
	src := "تعریف جمع(الف و ب):\n    الف + ب برگردان\n"
	stmts := expectNoError(t, src)
	ds, ok := stmts[0].(*ast.DefStmt)
	if !ok {
		t.Fatalf("want DefStmt, got %T", stmts[0])
	}
	if ds.Name != "جمع" {
		t.Errorf("name = %q, want جمع", ds.Name)
	}
	if len(ds.Params) != 2 {
		t.Errorf("want 2 params, got %d", len(ds.Params))
	}
}

func TestParseStringInterpExpr(t *testing.T) {
	e, err := ParseSingleExpr("نام")
	if err != nil {
		t.Fatalf("ParseSingleExpr error: %v", err)
	}
	if id, ok := e.(*ast.Ident); !ok || id.Name != "نام" {
		t.Errorf("got %v, want Ident(نام)", e)
	}
}

func TestParseErrors(t *testing.T) {
	// implicit truthiness is invalid
	if _, err := ParseProgram("اگر x:\n    print\n"); err == nil {
		t.Errorf("expected error for bare condition, got none")
	}
	// missing copula
	if _, err := ParseProgram("اگر x == 5:\n    print\n"); err == nil {
		t.Errorf("expected error for missing copula")
	}
}

func TestParseAssignment(t *testing.T) {
	stmts := expectNoError(t, "نتیجه و خطا = کاری()\n")
	if _, ok := stmts[0].(*ast.MultiAssign); !ok {
		t.Errorf("want MultiAssign, got %T", stmts[0])
	}
}

func TestParseUnterminated(t *testing.T) {
	if _, err := ParseProgram("/باز\n"); err != nil {
		t.Logf("comment ok: %v", err)
	}
	_ = strings.TrimSpace
}

func TestParseYieldVerbFinal(t *testing.T) {
	stmts := expectNoError(t, "ای بساز\n")
	ys, ok := stmts[0].(*ast.YieldStmt)
	if !ok {
		t.Fatalf("want YieldStmt, got %T", stmts[0])
	}
	if id, ok := ys.Value.(*ast.Ident); !ok || id.Name != "ای" {
		t.Errorf("yield value = %v, want Ident(ای)", ys.Value)
	}
}

func TestParseYieldFromVerbFinal(t *testing.T) {
	stmts := expectNoError(t, "الف() بساز‌از\n")
	yf, ok := stmts[0].(*ast.YieldFromStmt)
	if !ok {
		t.Fatalf("want YieldFromStmt, got %T", stmts[0])
	}
	if _, ok := yf.Value.(*ast.Call); !ok {
		t.Errorf("yield-from value = %T, want Call", yf.Value)
	}
}

func TestParseDecoratorAttachesToDef(t *testing.T) {
	stmts := expectNoError(t, "پوشش دوبار\nپوشش سه‌بار\nتعریف ف():\n\tمثل\n")
	var ds *ast.DefStmt
	for _, s := range stmts {
		if d, ok := s.(*ast.DefStmt); ok {
			ds = d
		}
	}
	if ds == nil {
		t.Fatalf("want a DefStmt")
	}
	if len(ds.Decorators) != 2 {
		t.Fatalf("want 2 decorators, got %d", len(ds.Decorators))
	}
	if ds.Decorators[0].Name != "دوبار" || ds.Decorators[1].Name != "سه‌بار" {
		t.Errorf("decorator order = %v, want دوبار then سه‌بار", ds.Decorators)
	}
}

func TestParseDecoratorRequiresDef(t *testing.T) {
	if _, err := ParseProgram("پوشش دوبار\nx = ۵\n"); err == nil {
		t.Fatalf("expected error for پوشش not followed by تعریف")
	}
}

// L5: the SPEC's ZWNJ spelling «حذف‌کن» and the plain «حذفکن» both parse as
// remove.
func TestParseRemoveWithZWNJ(t *testing.T) {
	for _, src := range []string{"۱ از xs حذف‌کن\n", "۱ از xs حذفکن\n"} {
		stmts := expectNoError(t, src)
		if _, ok := stmts[0].(*ast.RemoveStmt); !ok {
			t.Fatalf("want RemoveStmt for %q, got %T", src, stmts[0])
		}
	}
}

// L10/L15: a huge integer literal promotes to a float instead of silently
// becoming 0.
func TestParseHugeNumberPromotesToFloat(t *testing.T) {
	stmts := expectNoError(t, "x = ۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹۹\n")
	as := stmts[0].(*ast.Assign)
	nl, ok := as.Value.(*ast.NumberLit)
	if !ok {
		t.Fatalf("want NumberLit, got %T", as.Value)
	}
	if nl.Int {
		t.Errorf("huge literal should be a float, got int %d", nl.IntVal)
	}
	if nl.FVal != 1e20 {
		t.Errorf("float value = %v, want 1e20", nl.FVal)
	}
}

// L11: an unterminated block comment is a syntax error, not silently ignored.
func TestParseUnterminatedBlockCommentIsError(t *testing.T) {
	if _, err := ParseProgram("// this never closes\n"); err == nil {
		t.Fatalf("expected syntax error for unterminated block comment, got none")
	}
}

// L19: yield is usable as an expression inside parentheses / argument lists.
func TestParseYieldExprInArgs(t *testing.T) {
	stmts := expectNoError(t, "گروه(۱ بساز)\n")
	es := stmts[0].(*ast.ExprStmt)
	call, ok := es.Expr.(*ast.Call)
	if !ok {
		t.Fatalf("want Call, got %T", es.Expr)
	}
	if len(call.Args) != 1 {
		t.Fatalf("want 1 arg, got %d", len(call.Args))
	}
	if _, ok := call.Args[0].(*ast.YieldExpr); !ok {
		t.Fatalf("want YieldExpr arg, got %T", call.Args[0])
	}
}

// L19: «تعریف f(): (x بساز)» — a parenthesized yield expression statement.
func TestParseYieldExprInParens(t *testing.T) {
	stmts := expectNoError(t, "تعریف f():\n\t(ای بساز)\n")
	ds := stmts[0].(*ast.DefStmt)
	es, ok := ds.Body.Stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("want ExprStmt, got %T", ds.Body.Stmts[0])
	}
	if _, ok := es.Expr.(*ast.YieldExpr); !ok {
		t.Fatalf("want YieldExpr, got %T", es.Expr)
	}
}

// L19: verb-initial «بساز expr» works inside argument lists.
func TestParseYieldExprVerbInitial(t *testing.T) {
	stmts := expectNoError(t, "گروه(بساز ای)\n")
	es := stmts[0].(*ast.ExprStmt)
	call, ok := es.Expr.(*ast.Call)
	if !ok {
		t.Fatalf("want Call, got %T", es.Expr)
	}
	if _, ok := call.Args[0].(*ast.YieldExpr); !ok {
		t.Fatalf("want YieldExpr arg, got %T", call.Args[0])
	}
}

// L19: «expr بساز‌از» as an expression inside argument lists.
func TestParseYieldFromExprInArgs(t *testing.T) {
	stmts := expectNoError(t, "گروه(الف() بساز‌از)\n")
	es := stmts[0].(*ast.ExprStmt)
	call, ok := es.Expr.(*ast.Call)
	if !ok {
		t.Fatalf("want Call, got %T", es.Expr)
	}
	if _, ok := call.Args[0].(*ast.YieldFromExpr); !ok {
		t.Fatalf("want YieldFromExpr arg, got %T", call.Args[0])
	}
}

// L19: a bare بساز at statement level (outside parentheses) is still an error.
func TestParseBareYieldStillError(t *testing.T) {
	if _, err := ParseProgram("بساز ای\n"); err == nil {
		t.Fatalf("expected error for bare بساز at statement level")
	}
}

// --- v0.6: concurrency ---

func TestParseGoStmt(t *testing.T) {
	stmts := expectNoError(t, "برو کار(۱)\n")
	gs, ok := stmts[0].(*ast.GoStmt)
	if !ok {
		t.Fatalf("want GoStmt, got %T", stmts[0])
	}
	if _, ok := gs.Expr.(*ast.Call); !ok {
		t.Fatalf("برو expression should be a Call, got %T", gs.Expr)
	}
}

func TestParseChannelLit(t *testing.T) {
	stmts := expectNoError(t, "ch = کانال(صحیح و ۱۰)\n")
	as := stmts[0].(*ast.Assign)
	cl, ok := as.Value.(*ast.ChannelLit)
	if !ok {
		t.Fatalf("want ChannelLit, got %T", as.Value)
	}
	if cl.Type == nil || cl.Size == nil {
		t.Fatalf("want Type and Size set")
	}
}

func TestParseSendRecvClose(t *testing.T) {
	stmts := expectNoError(t, "ch << ۱\nمقدار = >>ch\nch ببند\n")
	ss, ok := stmts[0].(*ast.SendStmt)
	if !ok {
		t.Fatalf("want SendStmt, got %T", stmts[0])
	}
	_ = ss
	as := stmts[1].(*ast.Assign)
	if _, ok := as.Value.(*ast.RecvExpr); !ok {
		t.Fatalf("want RecvExpr, got %T", as.Value)
	}
	if _, ok := stmts[2].(*ast.CloseStmt); !ok {
		t.Fatalf("want CloseStmt, got %T", stmts[2])
	}
}

func TestParseClosedCheck(t *testing.T) {
	stmts := expectNoError(t, "اگر بسته‌استِ ch == درست باشد:\n\tمثل\n")
	is := stmts[0].(*ast.IfStmt)
	be := is.Cond.(*ast.BinaryOp)
	ma, ok := be.Left.(*ast.MemberAccess)
	if !ok {
		t.Fatalf("want MemberAccess for بسته‌استِ, got %T", be.Left)
	}
	if id, ok := ma.Attr.(*ast.Ident); !ok || id.Name != "بسته‌است" {
		t.Fatalf("attr = %v, want بسته‌است", ma.Attr)
	}
}

// --- S7: نباشد as a logical-negation expression (not just a copula) ---

func TestParseNegationExpression(t *testing.T) {
	// postfix form: y = x نباشد
	stmts := expectNoError(t, "x = درست\ny = x نباشد\n")
	as, ok := stmts[1].(*ast.Assign)
	if !ok {
		t.Fatalf("stmt 1: want Assign, got %T", stmts[1])
	}
	un, ok := as.Value.(*ast.Unary)
	if !ok {
		t.Fatalf("assignment value: want Unary, got %T", as.Value)
	}
	if un.Op != "not" {
		t.Errorf("unary op = %q, want not", un.Op)
	}
	if id, ok := un.Expr.(*ast.Ident); !ok || id.Name != "x" {
		t.Errorf("negation operand = %v, want Ident(x)", un.Expr)
	}

	// prefix form: y = نباشد x
	stmts = expectNoError(t, "y = نباشد x\n")
	as = stmts[0].(*ast.Assign)
	un, ok = as.Value.(*ast.Unary)
	if !ok {
		t.Fatalf("want Unary for prefix negation, got %T", as.Value)
	}
	if un.Op != "not" {
		t.Errorf("unary op = %q, want not", un.Op)
	}
}

// The suffix copula in conditions must not be consumed as a negation
// expression.
func TestParseNegationCopulaStillWorks(t *testing.T) {
	stmts := expectNoError(t, "اگر x == ۵ نباشد:\n\tمثل\n")
	is := stmts[0].(*ast.IfStmt)
	un, ok := is.Cond.(*ast.Unary)
	if !ok {
		t.Fatalf("condition should be negated Unary, got %T", is.Cond)
	}
	if un.Op != "not" {
		t.Errorf("unary op = %q, want not", un.Op)
	}
}

// --- S8: varargs (*args) and kwargs (**kwargs) parameter parsing ---

func TestParseVarargsParam(t *testing.T) {
	stmts := expectNoError(t, "تعریف جمع‌همه(*اعداد):\n\tاعداد بنویس\n")
	ds := stmts[0].(*ast.DefStmt)
	if len(ds.Params) != 1 {
		t.Fatalf("want 1 param, got %d", len(ds.Params))
	}
	if !ds.Params[0].Variadic {
		t.Errorf("param should be variadic")
	}
	if ds.Params[0].Kwargs {
		t.Errorf("param should not be kwargs")
	}
	if ds.Params[0].Name != "اعداد" {
		t.Errorf("param name = %q, want اعداد", ds.Params[0].Name)
	}
}

func TestParseKwargsParam(t *testing.T) {
	stmts := expectNoError(t, "تعریف ف(*args و **kwargs):\n\tمثل\n")
	ds := stmts[0].(*ast.DefStmt)
	if len(ds.Params) != 2 {
		t.Fatalf("want 2 params, got %d", len(ds.Params))
	}
	if !ds.Params[0].Variadic || ds.Params[0].Kwargs {
		t.Errorf("param 0: want Variadic, got Variadic=%v Kwargs=%v", ds.Params[0].Variadic, ds.Params[0].Kwargs)
	}
	if !ds.Params[1].Kwargs || ds.Params[1].Variadic {
		t.Errorf("param 1: want Kwargs, got Variadic=%v Kwargs=%v", ds.Params[1].Variadic, ds.Params[1].Kwargs)
	}
}

// Varargs params must not swallow regular params.
func TestParseMixedParams(t *testing.T) {
	stmts := expectNoError(t, "تعریف ف(الف و ب و *بقیه):\n\tمثل\n")
	ds := stmts[0].(*ast.DefStmt)
	if len(ds.Params) != 3 {
		t.Fatalf("want 3 params, got %d", len(ds.Params))
	}
	if ds.Params[0].Variadic || ds.Params[1].Variadic {
		t.Errorf("regular params should not be variadic")
	}
	if !ds.Params[2].Variadic || ds.Params[2].Name != "بقیه" {
		t.Errorf("param 2: want variadic بقیه, got Variadic=%v Name=%q", ds.Params[2].Variadic, ds.Params[2].Name)
	}
}

// --- S3: با (with) statement ---

func TestParseWithStmt(t *testing.T) {
	src := "با بازکردن(«فایل.txt» و «r») بانام ف:\n\tف بنویس\n"
	stmts := expectNoError(t, src)
	ws, ok := stmts[0].(*ast.WithStmt)
	if !ok {
		t.Fatalf("want WithStmt, got %T", stmts[0])
	}
	if _, ok := ws.Context.(*ast.Call); !ok {
		t.Errorf("context should be a Call, got %T", ws.Context)
	}
	if ws.Name != "ف" {
		t.Errorf("name = %q, want ف", ws.Name)
	}
	if len(ws.Body) != 1 {
		t.Fatalf("want 1 body statement, got %d", len(ws.Body))
	}
	if _, ok := ws.Body[0].(*ast.PrintStmt); !ok {
		t.Errorf("body stmt = %T, want PrintStmt", ws.Body[0])
	}
}

// With without بانام is accepted (name is empty).
func TestParseWithNoAlias(t *testing.T) {
	stmts := expectNoError(t, "با بازکردن(«فایل.txt»):\n\tمثل\n")
	ws := stmts[0].(*ast.WithStmt)
	if ws.Name != "" {
		t.Errorf("name = %q, want empty", ws.Name)
	}
}

// --- S37: bare-variable conditions are a syntax error ---

func TestParseBareVariableConditionIsError(t *testing.T) {
	_, err := ParseProgram("اگر x باشد:\n\tمثل\n")
	if err == nil {
		t.Fatalf("expected syntax error for bare-variable condition «اگر x باشد:»")
	}
	_, err = ParseProgram("اگر x + ۱ باشد:\n\tمثل\n")
	if err == nil {
		t.Fatalf("expected syntax error for non-comparison condition «اگر x + ۱ باشد:»")
	}
}

// Valid condition forms must keep parsing (spec §17.7 exceptions).
func TestParseValidConditionForms(t *testing.T) {
	for _, src := range []string{
		"اگر x == ۵ باشد:\n\tمثل\n",
		"تاوقتی درست باشد:\n\tمثل\n",
		"اگر x در لیست باشد:\n\tمثل\n",
		"اگر x == ۵ نباشد:\n\tمثل\n",
		"اگر x در لیست نباشد:\n\tمثل\n",
		"اگر x نباشد:\n\tمثل\n", // negation of a boolean variable
	} {
		expectNoError(t, src)
	}
}

// --- S4/S5: بخوان is a plain method name on the ezafe call form ---

func TestParseBekhanEzafeMethod(t *testing.T) {
	stmts := expectNoError(t, "محتوا = بخوانِ()ف\n")
	as, ok := stmts[0].(*ast.Assign)
	if !ok {
		t.Fatalf("want Assign, got %T", stmts[0])
	}
	mc, ok := as.Value.(*ast.MethodCall)
	if !ok {
		t.Fatalf("want MethodCall, got %T", as.Value)
	}
	if id, ok := mc.Method.(*ast.Ident); !ok || id.Name != "بخوان" {
		t.Errorf("method = %v, want Ident(بخوان)", mc.Method)
	}
	if id, ok := mc.Receiver.(*ast.Ident); !ok || id.Name != "ف" {
		t.Errorf("receiver = %v, want Ident(ف)", mc.Receiver)
	}
}

// --- Phase 8: comprehensive spec-coverage parser tests ---

func TestParseTypedVarAnnotation(t *testing.T) {
	stmts := expectNoError(t, "سن: صحیح = ۲۵\n")
	as, ok := stmts[0].(*ast.Assign)
	if !ok {
		t.Fatalf("want Assign, got %T", stmts[0])
	}
	if as.Ann != "صحیح" {
		t.Errorf("annotation = %q, want صحیح", as.Ann)
	}
}

func TestParseFunctionAnnotations(t *testing.T) {
	stmts := expectNoError(t, "تعریف جمع(الف: صحیح و ب: صحیح) -> صحیح:\n\tالف + ب برگردان\n")
	ds := stmts[0].(*ast.DefStmt)
	if len(ds.Params) != 2 {
		t.Fatalf("want 2 params, got %d", len(ds.Params))
	}
	if ds.Params[0].Ann != "صحیح" || ds.Params[1].Ann != "صحیح" {
		t.Errorf("param annotations = %q and %q, want صحیح both", ds.Params[0].Ann, ds.Params[1].Ann)
	}
	if ds.RetType != "صحیح" {
		t.Errorf("return annotation = %q, want صحیح", ds.RetType)
	}
}

func TestParseTernaryExpression(t *testing.T) {
	stmts := expectNoError(t, "وضعیت = «بزرگ» اگر سن >= ۱۸ باشد وگرنه «کوچک»\n")
	as := stmts[0].(*ast.Assign)
	tr, ok := as.Value.(*ast.TernaryExpr)
	if !ok {
		t.Fatalf("want TernaryExpr, got %T", as.Value)
	}
	if _, ok := tr.Cond.(*ast.BinaryOp); !ok {
		t.Errorf("ternary condition should be a comparison BinaryOp, got %T", tr.Cond)
	}
}

func TestParseListComprehension(t *testing.T) {
	stmts := expectNoError(t, "ن = [ای * ۲ برای ای در بازه(۱۰)]\n")
	as := stmts[0].(*ast.Assign)
	if _, ok := as.Value.(*ast.ListComp); !ok {
		t.Fatalf("want ListComp, got %T", as.Value)
	}
}

func TestParseDictComprehension(t *testing.T) {
	stmts := expectNoError(t, "گ = {ای: ای * ۲ برای ای در بازه(4)}\n")
	as := stmts[0].(*ast.Assign)
	if _, ok := as.Value.(*ast.DictComp); !ok {
		t.Fatalf("want DictComp, got %T", as.Value)
	}
}

func TestParseSetComprehension(t *testing.T) {
	stmts := expectNoError(t, "س = {ای % ۳ برای ای در بازه(۱۰)}\n")
	as := stmts[0].(*ast.Assign)
	if _, ok := as.Value.(*ast.SetComp); !ok {
		t.Fatalf("want SetComp, got %T", as.Value)
	}
}

func TestParseGenExp(t *testing.T) {
	stmts := expectNoError(t, "گ = (ای × ۲ برای ای در بازه(4))\n")
	as := stmts[0].(*ast.Assign)
	if _, ok := as.Value.(*ast.GenExp); !ok {
		t.Fatalf("want GenExp, got %T", as.Value)
	}
}

func TestParseComprehensionFilter(t *testing.T) {
	stmts := expectNoError(t, "ن = [ای برای ای در بازه(۱۰) اگر ای % ۲ == ۰ باشد]\n")
	as := stmts[0].(*ast.Assign)
	lc, ok := as.Value.(*ast.ListComp)
	if !ok {
		t.Fatalf("want ListComp, got %T", as.Value)
	}
	if len(lc.Clauses) != 1 || lc.Clauses[0].Filter == nil {
		t.Fatalf("want 1 clause with a filter, got %d clauses", len(lc.Clauses))
	}
}

func TestParsePipeExpression(t *testing.T) {
	stmts := expectNoError(t, "۵ |> دوبرابر |> بنویس\n")
	es, ok := stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("want ExprStmt, got %T", stmts[0])
	}
	pipe, ok := es.Expr.(*ast.PipeExpr)
	if !ok {
		t.Fatalf("want PipeExpr, got %T", es.Expr)
	}
	if _, ok := pipe.Left.(*ast.PipeExpr); !ok {
		t.Errorf("chained pipe left should itself be a PipeExpr, got %T", pipe.Left)
	}
}

func TestParseForRangeWithStep(t *testing.T) {
	stmts := expectNoError(t, "برای ای از ۰ تا ۱۰ گام ۲:\n\tای بنویس\n")
	fr, ok := stmts[0].(*ast.ForRange)
	if !ok {
		t.Fatalf("want ForRange, got %T", stmts[0])
	}
	if fr.Step == nil {
		t.Fatalf("want Step set")
	}
}

func TestParseForIn(t *testing.T) {
	stmts := expectNoError(t, "برای ای در xs:\n\tای بنویس\n")
	if _, ok := stmts[0].(*ast.ForIn); !ok {
		t.Fatalf("want ForIn, got %T", stmts[0])
	}
}

func TestParseWhile(t *testing.T) {
	stmts := expectNoError(t, "تاوقتی ای < ۵ باشد:\n\tای += ۱\n")
	if _, ok := stmts[0].(*ast.WhileStmt); !ok {
		t.Fatalf("want WhileStmt, got %T", stmts[0])
	}
}

func TestParseTryExceptFinally(t *testing.T) {
	stmts := expectNoError(t, "بپا:\n\tمثل\nخطای‌نوع بگیر بانام e:\n\tمثل\nدرنهایت:\n\tمثل\n")
	ts, ok := stmts[0].(*ast.TryStmt)
	if !ok {
		t.Fatalf("want TryStmt, got %T", stmts[0])
	}
	if len(ts.Handlers) != 1 {
		t.Fatalf("want 1 handler, got %d", len(ts.Handlers))
	}
	if ts.Handlers[0].Alias != "e" {
		t.Errorf("handler alias = %q, want e", ts.Handlers[0].Alias)
	}
	if ts.Handlers[0].Exception == nil {
		t.Errorf("want a typed exception handler")
	}
	if ts.Finally == nil {
		t.Errorf("want finally block")
	}
}

func TestParseBareExcept(t *testing.T) {
	stmts := expectNoError(t, "بپا:\n\tمثل\nبگیر:\n\tمثل\n")
	ts := stmts[0].(*ast.TryStmt)
	if len(ts.Handlers) != 1 {
		t.Fatalf("want 1 handler, got %d", len(ts.Handlers))
	}
	if ts.Handlers[0].Exception != nil {
		t.Errorf("bare بگیر should have nil exception, got %T", ts.Handlers[0].Exception)
	}
}

func TestParseRaiseStmt(t *testing.T) {
	stmts := expectNoError(t, "خطای‌مقدار(«boom») بده\n")
	if _, ok := stmts[0].(*ast.RaiseStmt); !ok {
		t.Fatalf("want RaiseStmt, got %T", stmts[0])
	}
}

func TestParseDeferStmt(t *testing.T) {
	stmts := expectNoError(t, "پاک‌سازی() تأخیری\n")
	if _, ok := stmts[0].(*ast.DeferStmt); !ok {
		t.Fatalf("want DeferStmt, got %T", stmts[0])
	}
}

func TestParseClassDefInheritance(t *testing.T) {
	stmts := expectNoError(t, "گونه سگ وارث حیوان:\n\tمثل\n")
	cd, ok := stmts[0].(*ast.ClassDef)
	if !ok {
		t.Fatalf("want ClassDef, got %T", stmts[0])
	}
	if cd.Parent != "حیوان" {
		t.Errorf("parent = %q, want حیوان", cd.Parent)
	}
}

func TestParseClassDefImplements(t *testing.T) {
	stmts := expectNoError(t, "گونه سگ رهی حیوانات:\n\tمثل\n")
	cd := stmts[0].(*ast.ClassDef)
	if cd.Implements != "حیوانات" {
		t.Errorf("implements = %q, want حیوانات", cd.Implements)
	}
}

func TestParseInterfaceDef(t *testing.T) {
	stmts := expectNoError(t, "رابط حیوانات:\n\tتعریف صدادهی (خود)\n\tتعریف نام (خود) -> متن\n")
	id, ok := stmts[0].(*ast.InterfaceDef)
	if !ok {
		t.Fatalf("want InterfaceDef, got %T", stmts[0])
	}
	if len(id.Methods) != 2 {
		t.Fatalf("want 2 interface methods, got %d", len(id.Methods))
	}
	if id.Methods[1].RetType != "متن" {
		t.Errorf("method 1 return type = %q, want متن", id.Methods[1].RetType)
	}
}

func TestParseImportStatements(t *testing.T) {
	stmts := expectNoError(t, "ریاضی بیار\n")
	is, ok := stmts[0].(*ast.ImportStmt)
	if !ok {
		t.Fatalf("want ImportStmt, got %T", stmts[0])
	}
	if is.Module != "ریاضی" {
		t.Errorf("module = %q, want ریاضی", is.Module)
	}

	stmts = expectNoError(t, "از ریاضی جذر بانام ریشه بیار\n")
	fi, ok := stmts[0].(*ast.FromImportStmt)
	if !ok {
		t.Fatalf("want FromImportStmt, got %T", stmts[0])
	}
	if fi.Module != "ریاضی" || fi.Name != "جذر" || fi.Alias != "ریشه" {
		t.Errorf("from-import = %+v, want ریاضی/جذر/ریشه", fi)
	}
}

func TestParseScopeDeclarations(t *testing.T) {
	stmts := expectNoError(t, "جهانی شمارنده و مقدار\nنامحلی شماره\n")
	gs, ok := stmts[0].(*ast.GlobalStmt)
	if !ok {
		t.Fatalf("want GlobalStmt, got %T", stmts[0])
	}
	if len(gs.Names) != 2 || gs.Names[0] != "شمارنده" || gs.Names[1] != "مقدار" {
		t.Errorf("global names = %v", gs.Names)
	}
	ns, ok := stmts[1].(*ast.NonlocalStmt)
	if !ok {
		t.Fatalf("want NonlocalStmt, got %T", stmts[1])
	}
	if len(ns.Names) != 1 || ns.Names[0] != "شماره" {
		t.Errorf("nonlocal names = %v", ns.Names)
	}
}

func TestParseLiteralTypes(t *testing.T) {
	tests := []struct {
		src  string
		want func(stmt ast.Stmt) bool
	}{
		{"t = (۱ و ۲)\n", func(s ast.Stmt) bool { _, ok := s.(*ast.Assign).Value.(*ast.TupleLit); return ok }},
		{"s = {۱ و ۲}\n", func(s ast.Stmt) bool { _, ok := s.(*ast.Assign).Value.(*ast.SetLit); return ok }},
		{"d = {«a»: ۱}\n", func(s ast.Stmt) bool { _, ok := s.(*ast.Assign).Value.(*ast.DictLit); return ok }},
		{"x = ۵\n", func(s ast.Stmt) bool { _, ok := s.(*ast.Assign).Value.(*ast.NumberLit); return ok }},
	}
	for _, tc := range tests {
		stmts := expectNoError(t, tc.src)
		if !tc.want(stmts[0]) {
			t.Errorf("%q: unexpected AST shape", tc.src)
		}
	}
}

func TestParseCompoundAssign(t *testing.T) {
	stmts := expectNoError(t, "x += ۵\n")
	ca, ok := stmts[0].(*ast.CompoundAssign)
	if !ok {
		t.Fatalf("want CompoundAssign, got %T", stmts[0])
	}
	if ca.Op != "+=" {
		t.Errorf("op = %q, want +=", ca.Op)
	}
}

func TestParseMultiAssignUnpack(t *testing.T) {
	stmts := expectNoError(t, "الف و ب = جفت()\n")
	if _, ok := stmts[0].(*ast.MultiAssign); !ok {
		t.Fatalf("want MultiAssign, got %T", stmts[0])
	}
}

func TestParseConditionalCopulaForms(t *testing.T) {
	for _, src := range []string{
		"اگر x == ۵ باشد:\n\tمثل\n",
		"اگر x == ۵ نباشد:\n\tمثل\n",
		"اگر x در لیست باشد:\n\tمثل\n",
		"اگر x در لیست نباشد:\n\tمثل\n",
		"تاوقتی درست باشد:\n\tمثل\n",
	} {
		expectNoError(t, src)
	}
}

// --- Phase 10: comprehensive spec-coverage parser tests (appended) ---

// S7: negation «x نباشد» / «نباشد x» parses as a Unary expression.
func TestParseNegationPrefix(t *testing.T) {
	// postfix form: y = x نباشد
	stmts := expectNoError(t, "x = درست\ny = x نباشد\n")
	as, ok := stmts[1].(*ast.Assign)
	if !ok {
		t.Fatalf("stmt 1: want Assign, got %T", stmts[1])
	}
	un, ok := as.Value.(*ast.Unary)
	if !ok {
		t.Fatalf("assignment value: want Unary, got %T", as.Value)
	}
	if un.Op != "not" {
		t.Errorf("unary op = %q, want not", un.Op)
	}
	if id, ok := un.Expr.(*ast.Ident); !ok || id.Name != "x" {
		t.Errorf("negation operand = %v, want Ident(x)", un.Expr)
	}

	// prefix form: y = نباشد x
	stmts = expectNoError(t, "y = نباشد x\n")
	as = stmts[0].(*ast.Assign)
	un, ok = as.Value.(*ast.Unary)
	if !ok {
		t.Fatalf("want Unary for prefix negation, got %T", as.Value)
	}
	if un.Op != "not" {
		t.Errorf("unary op = %q, want not", un.Op)
	}
}

// S8: «تعریف f(*args):» marks the parameter as variadic.
func TestParseVarargs(t *testing.T) {
	stmts := expectNoError(t, "تعریف f(*args):\n\tمثل\n")
	ds, ok := stmts[0].(*ast.DefStmt)
	if !ok {
		t.Fatalf("want DefStmt, got %T", stmts[0])
	}
	if len(ds.Params) != 1 {
		t.Fatalf("want 1 param, got %d", len(ds.Params))
	}
	if !ds.Params[0].Variadic || ds.Params[0].Kwargs {
		t.Errorf("param: want Variadic=true Kwargs=false, got Variadic=%v Kwargs=%v", ds.Params[0].Variadic, ds.Params[0].Kwargs)
	}
	if ds.Params[0].Name != "args" {
		t.Errorf("param name = %q, want args", ds.Params[0].Name)
	}
}

// S8: «تعریف f(**kw):» marks the parameter as **kwargs.
func TestParseKwargs(t *testing.T) {
	stmts := expectNoError(t, "تعریف f(**kw):\n\tمثل\n")
	ds, ok := stmts[0].(*ast.DefStmt)
	if !ok {
		t.Fatalf("want DefStmt, got %T", stmts[0])
	}
	if len(ds.Params) != 1 {
		t.Fatalf("want 1 param, got %d", len(ds.Params))
	}
	if !ds.Params[0].Kwargs || ds.Params[0].Variadic {
		t.Errorf("param: want Kwargs=true Variadic=false, got Kwargs=%v Variadic=%v", ds.Params[0].Kwargs, ds.Params[0].Variadic)
	}
	if ds.Params[0].Name != "kw" {
		t.Errorf("param name = %q, want kw", ds.Params[0].Name)
	}
}

// S3: «با EXPR بانام NAME: body» parses to a WithStmt.
func TestParseWithStatement(t *testing.T) {
	// the context expression may be a plain identifier
	stmts := expectNoError(t, "با فایل بانام ف:\n\tف بنویس\n")
	ws, ok := stmts[0].(*ast.WithStmt)
	if !ok {
		t.Fatalf("want WithStmt, got %T", stmts[0])
	}
	if _, ok := ws.Context.(*ast.Ident); !ok {
		t.Errorf("context = %T, want Ident", ws.Context)
	}
	if ws.Name != "ف" {
		t.Errorf("name = %q, want ف", ws.Name)
	}
	if len(ws.Body) != 1 {
		t.Fatalf("want 1 body statement, got %d", len(ws.Body))
	}
	if _, ok := ws.Body[0].(*ast.PrintStmt); !ok {
		t.Errorf("body stmt = %T, want PrintStmt", ws.Body[0])
	}
}

// S37: a bare variable condition «اگر x باشد:» is a syntax error (no implicit
// truthiness), but bare boolean literals are allowed.
func TestParseBareBoolCondition(t *testing.T) {
	if _, err := ParseProgram("اگر x باشد:\n\tمثل\n"); err == nil {
		t.Fatalf("expected syntax error for bare boolean condition")
	}
	// bare boolean literals are the only valid non-comparison conditions
	expectNoError(t, "اگر درست باشد:\n\tمثل\n")
	expectNoError(t, "اگر غلط نباشد:\n\tمثل\n")
}

// L4/L6: «{۱ و ۲}» parses to a SetLit (not a dict literal).
func TestParseSetLiteral(t *testing.T) {
	stmts := expectNoError(t, "س = {۱ و ۲}\n")
	as, ok := stmts[0].(*ast.Assign)
	if !ok {
		t.Fatalf("want Assign, got %T", stmts[0])
	}
	sl, ok := as.Value.(*ast.SetLit)
	if !ok {
		t.Fatalf("want SetLit, got %T", as.Value)
	}
	if len(sl.Elems) != 2 {
		t.Errorf("want 2 elements, got %d", len(sl.Elems))
	}
}

// L7: «methodِ(args)receiver» parses to a MethodCall carrying args, the method
// name, and the receiver.
func TestParseEzafeMethodCallExtra(t *testing.T) {
	src := "تنظیمِ(۵ و ۶) سنج\n"
	stmts := expectNoError(t, src)
	es, ok := stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("want ExprStmt, got %T", stmts[0])
	}
	mc, ok := es.Expr.(*ast.MethodCall)
	if !ok {
		t.Fatalf("want MethodCall, got %T", es.Expr)
	}
	if len(mc.Args) != 2 {
		t.Fatalf("want 2 args, got %d", len(mc.Args))
	}
	if id, ok := mc.Method.(*ast.Ident); !ok || id.Name != "تنظیم" {
		t.Errorf("method = %v, want Ident(تنظیم)", mc.Method)
	}
	if id, ok := mc.Receiver.(*ast.Ident); !ok || id.Name != "سنج" {
		t.Errorf("receiver = %v, want Ident(سنج)", mc.Receiver)
	}
}
