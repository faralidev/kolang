// Package ast defines the abstract syntax tree nodes of Kolang.
//
// Every node implements Node (carrying its source line) and either Expr or
// Stmt. The parser produces these nodes; the evaluator walks them.
package ast

// Node is the base interface for all AST nodes.
type Node interface {
	Line() int
}

// Expr represents an expression node.
type Expr interface {
	Node
	exprNode()
}

// Stmt represents a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// --- Expressions ---

// NumberLit is an integer or floating-point literal. IntVal holds the value
// when Int is true; otherwise FVal is used.
type NumberLit struct {
	L      int
	Int    bool
	IntVal int64
	FVal   float64
}

func (n *NumberLit) Line() int { return n.L }
func (n *NumberLit) exprNode() {}

// StrLit is a raw «...» string literal. Interpolation of {expr} placeholders
// happens at evaluation time, so only the raw text is stored here.
type StrLit struct {
	L   int
	Raw string
}

func (n *StrLit) Line() int { return n.L }
func (n *StrLit) exprNode() {}

// BoolLit is a boolean literal (درست / غلط).
type BoolLit struct {
	L     int
	Value bool
}

func (n *BoolLit) Line() int { return n.L }
func (n *BoolLit) exprNode() {}

// NoneLit is the تهی (none/nil) literal.
type NoneLit struct {
	L int
}

func (n *NoneLit) Line() int { return n.L }
func (n *NoneLit) exprNode() {}

// Ident is a variable reference by name (including the special names خود and
// والد, which resolve to the current instance / parent proxy).
type Ident struct {
	L    int
	Name string
}

func (n *Ident) Line() int { return n.L }
func (n *Ident) exprNode() {}

// Unary is a prefix or postfix unary operation: negation («-x»), logical
// negation («نباشد x» / «x نباشد»), etc. Op is the operator string; Expr is
// the operand.
type Unary struct {
	L    int
	Op   string
	Expr Expr
}

func (n *Unary) Line() int { return n.L }
func (n *Unary) exprNode() {}

// BinaryOp is an infix binary operation.
type BinaryOp struct {
	L           int
	Op          string
	Left, Right Expr
}

func (n *BinaryOp) Line() int { return n.L }
func (n *BinaryOp) exprNode() {}

// Call is a function call: Fn (a name or any callable expression) applied to
// positional Args and keyword KwArgs.
type Call struct {
	L      int
	Fn     Expr
	Args   []Expr
	KwArgs []*Kwarg
}

func (n *Call) Line() int { return n.L }
func (n *Call) exprNode() {}

// Kwarg is a single keyword argument (name = value) in a call.
type Kwarg struct {
	L     int
	Name  string
	Value Expr
}

// Index is a subscript: Target[Index].
type Index struct {
	L      int
	Target Expr
	Index  Expr
}

func (n *Index) Line() int { return n.L }
func (n *Index) exprNode() {}

// Slice is a slice expression: Target[Low:High:Step]. Any bound may be nil
// (omitted).
type Slice struct {
	L               int
	Target          Expr
	Low, High, Step Expr
}

func (n *Slice) Line() int { return n.L }
func (n *Slice) exprNode() {}

// MemberAccess is attribute access: attrِ receiver → (receiver.attr)
type MemberAccess struct {
	L        int
	Receiver Expr
	Attr     Expr // must be an Ident
}

func (n *MemberAccess) Line() int { return n.L }
func (n *MemberAccess) exprNode() {}

// MethodCall is a method call: methodِ(args)receiver → receiver.method(args)
type MethodCall struct {
	L        int
	Receiver Expr
	Method   Expr
	Args     []Expr
}

func (n *MethodCall) Line() int { return n.L }
func (n *MethodCall) exprNode() {}

// ListLit is a list literal: «[elem و elem ...]».
type ListLit struct {
	L     int
	Elems []Expr
}

func (n *ListLit) Line() int { return n.L }
func (n *ListLit) exprNode() {}

// TupleLit is a tuple literal: «(elem و elem ...)».
type TupleLit struct {
	L     int
	Elems []Expr
}

func (n *TupleLit) Line() int { return n.L }
func (n *TupleLit) exprNode() {}

// DictLit is a dict literal: «{key: value و ...}». Keys and Values are kept as
// parallel slices.
type DictLit struct {
	L      int
	Keys   []Expr
	Values []Expr
}

func (n *DictLit) Line() int { return n.L }
func (n *DictLit) exprNode() {}

// SetLit is a set literal: «{elem و elem ...}».
type SetLit struct {
	L     int
	Elems []Expr
}

func (n *SetLit) Line() int { return n.L }
func (n *SetLit) exprNode() {}

// PipeExpr is the pipe operator: «x |> f» = f(x). Left is the value threaded
// through, Right is the callable (a name or call) that receives it.
type PipeExpr struct {
	L           int
	Left, Right Expr
}

func (n *PipeExpr) Line() int { return n.L }
func (n *PipeExpr) exprNode() {}

// TernaryExpr is the conditional expression: «true اگر cond باشد وگرنه false».
type TernaryExpr struct {
	L                       int
	Cond                    Expr
	TrueBranch, FalseBranch Expr
}

func (n *TernaryExpr) Line() int { return n.L }
func (n *TernaryExpr) exprNode() {}

// CompClause is one «برای VAR در ITERABLE [اگر COND]» clause of a comprehension.
// Filter is optional; when present it is the full condition (copula included).
type CompClause struct {
	L        int
	Name     string // loop variable name
	Iterable Expr
	Filter   Expr // optional condition; nil if absent
}

// ListComp is a list comprehension: «[ expr برای VAR در ITERABLE ... ]».
type ListComp struct {
	L       int
	Element Expr
	Clauses []*CompClause
}

func (n *ListComp) Line() int { return n.L }
func (n *ListComp) exprNode() {}

// DictComp is a dict comprehension: «{ key : value برای VAR در ITERABLE ... }».
type DictComp struct {
	L       int
	Key     Expr
	Value   Expr
	Clauses []*CompClause
}

func (n *DictComp) Line() int { return n.L }
func (n *DictComp) exprNode() {}

// SetComp is a set comprehension: «{ expr برای VAR در ITERABLE ... }». Sets are
// represented as deduplicated lists in this phase.
type SetComp struct {
	L       int
	Element Expr
	Clauses []*CompClause
}

func (n *SetComp) Line() int { return n.L }
func (n *SetComp) exprNode() {}

// GenExp is a generator expression: «( expr برای VAR در ITERABLE ... )». In v0.5
// it evaluates eagerly to a list (lazy genexps deferred to a later phase).
type GenExp struct {
	L       int
	Element Expr
	Clauses []*CompClause
}

func (n *GenExp) Line() int { return n.L }
func (n *GenExp) exprNode() {}

// ChannelLit creates a channel: «کانال(TYPE و SIZE)». Type and Size are both
// optional. Type is a type annotation (ignored at runtime for v0.6 — Kolang
// channels are dynamically typed); Size is the buffer capacity (0 = unbuffered).
type ChannelLit struct {
	L    int
	Type Expr // optional type annotation (صحیح / متن / ...) — runtime-ignored
	Size Expr // optional buffer size (integer literal)
}

func (n *ChannelLit) Line() int { return n.L }
func (n *ChannelLit) exprNode() {}

// RecvExpr is a channel receive: «>>ch» (prefix operator). It blocks until a
// value is available; on a closed-drained channel it yields تهی (nil).
type RecvExpr struct {
	L       int
	Channel Expr
}

func (n *RecvExpr) Line() int { return n.L }
func (n *RecvExpr) exprNode() {}

// --- Statements ---

// ExprStmt wraps a bare expression used as a statement (e.g. a standalone
// function call).
type ExprStmt struct {
	L    int
	Expr Expr
}

func (n *ExprStmt) Line() int { return n.L }
func (n *ExprStmt) stmtNode() {}

// GoStmt spawns a goroutine: «برو EXPR» — EXPR is typically a function call,
// which runs concurrently; the caller does not wait for it.
type GoStmt struct {
	L    int
	Expr Expr
}

func (n *GoStmt) Line() int { return n.L }
func (n *GoStmt) stmtNode() {}

// GlobalStmt declares names as global: «جهانی نام۱ و نام۲».
type GlobalStmt struct {
	L     int
	Names []string
}

func (n *GlobalStmt) Line() int { return n.L }
func (n *GlobalStmt) stmtNode() {}

// NonlocalStmt declares names as nonlocal: «نامحلی نام۱ و نام۲».
type NonlocalStmt struct {
	L     int
	Names []string
}

func (n *NonlocalStmt) Line() int { return n.L }
func (n *NonlocalStmt) stmtNode() {}

// SendStmt is a channel send: «ch << value».
type SendStmt struct {
	L       int
	Channel Expr
	Value   Expr
}

func (n *SendStmt) Line() int { return n.L }
func (n *SendStmt) stmtNode() {}

// CloseStmt closes a channel: «ch ببند».
type CloseStmt struct {
	L       int
	Channel Expr
}

func (n *CloseStmt) Line() int { return n.L }
func (n *CloseStmt) stmtNode() {}

// Assign is a simple assignment: «Target = Value». Ann is the optional type
// annotation checked at runtime (gradual typing).
type Assign struct {
	L      int
	Target Expr
	Value  Expr
	// Ann is the optional type annotation («سن : صحیح = ۲۵» → "صحیح").
	// Empty means unannotated (dynamic). Checked at runtime (gradual typing).
	Ann string
}

func (n *Assign) Line() int { return n.L }
func (n *Assign) stmtNode() {}

// MultiAssign assigns multiple values to multiple targets: «a, b = x, y».
// A single tuple value may also be unpacked across the targets.
type MultiAssign struct {
	L       int
	Targets []Expr
	Values  []Expr
}

func (n *MultiAssign) Line() int { return n.L }
func (n *MultiAssign) stmtNode() {}

// CompoundAssign handles +=, -=, *=, etc.
type CompoundAssign struct {
	L      int
	Op     string
	Target Expr
	Value  Expr
}

func (n *CompoundAssign) Line() int { return n.L }
func (n *CompoundAssign) stmtNode() {}

// PrintStmt is «بنویس arg و arg ...» — print its arguments separated by spaces.
type PrintStmt struct {
	L    int
	Args []Expr
}

func (n *PrintStmt) Line() int { return n.L }
func (n *PrintStmt) stmtNode() {}

// InputStmt is «بگیر name» — read one line from stdin into the target.
type InputStmt struct {
	L      int
	Target Expr
}

func (n *InputStmt) Line() int { return n.L }
func (n *InputStmt) stmtNode() {}

// ReturnStmt is «برگردان [val و val ...]». Vals is empty for a bare return,
// holds one value for a single return, and multiple values for a tuple return.
type ReturnStmt struct {
	L    int
	Vals []Expr
}

func (n *ReturnStmt) Line() int { return n.L }
func (n *ReturnStmt) stmtNode() {}

// IfStmt is an اگر conditional. Elifs holds the «وگرنه اگر» branches and Else
// the optional trailing «وگرنه» block.
type IfStmt struct {
	L     int
	Cond  Expr
	Body  []Stmt
	Elifs []*ElifBranch
	Else  *Block
}

// ElifBranch is a single «وگرنه اگر COND: body» branch of an IfStmt.
type ElifBranch struct {
	L    int
	Cond Expr
	Body *Block
}

func (n *IfStmt) Line() int { return n.L }
func (n *IfStmt) stmtNode() {}

// WhileStmt is a «تاوقتی COND: body» loop.
type WhileStmt struct {
	L    int
	Cond Expr
	Body *Block
}

func (n *WhileStmt) Line() int { return n.L }
func (n *WhileStmt) stmtNode() {}

// ForRange is a numeric range loop: «برای VAR از START تا END [گام STEP]: body».
type ForRange struct {
	L                int
	Var              Expr
	Start, End, Step Expr
	Body             *Block
}

func (n *ForRange) Line() int { return n.L }
func (n *ForRange) stmtNode() {}

// ForIn iterates an iterable: «برای VARS در ITER: body». Vars holds one or more
// names (a parenthesized tuple for unpacking).
type ForIn struct {
	L    int
	Vars []Expr
	Iter Expr
	Body *Block
}

func (n *ForIn) Line() int { return n.L }
func (n *ForIn) stmtNode() {}

// Param is a single function/method parameter: an optional type annotation
// (Ann), an optional default expression, and optional *args / **kwargs markers.
type Param struct {
	L          int
	Name       string
	Default    Expr
	HasDefault bool
	// Ann is the optional parameter type annotation («الف : صحیح» → "صحیح").
	// Empty means unannotated (dynamic).
	Ann string
	// Variadic marks a «*name» varargs parameter (spec §5.7): all extra
	// positional arguments are collected into a list at runtime.
	Variadic bool
	// Kwargs marks a «**name» keyword-varargs parameter: extra keyword
	// arguments are collected into a dict at runtime.
	Kwargs bool
}

// DecoratorStmt is a «پوشش» (decorator) applied to the following تعریف. It is
// parsed as a statement but, on evaluation, wraps the function that immediately
// follows. Multiple decorators stack (applied bottom-up).
type DecoratorStmt struct {
	L    int
	Name string
	Args []Expr // optional: پوشش NAME(args) — args produce the actual decorator
}

func (n *DecoratorStmt) Line() int { return n.L }
func (n *DecoratorStmt) stmtNode() {}

// DefStmt is a function or method definition: «تعریف NAME(params) -> type: body».
// Methods store their decorators and optional return-type annotation as well.
type DefStmt struct {
	L      int
	Name   string
	Params []*Param
	Body   *Block
	// Decorators lists the «پوشش» lines appearing immediately above this def,
	// in source order (top-to-bottom). They are applied bottom-up on eval.
	Decorators []*DecoratorStmt
	// RetType is the optional return type annotation («-> صحیح» → "صحیح").
	// Empty means unannotated (dynamic). Checked on return (gradual typing).
	RetType string
}

func (n *DefStmt) Line() int { return n.L }
func (n *DefStmt) stmtNode() {}

// YieldStmt is «expr بساز» — yield a value from a generator (verb-final).
type YieldStmt struct {
	L     int
	Value Expr
}

func (n *YieldStmt) Line() int { return n.L }
func (n *YieldStmt) stmtNode() {}

// YieldFromStmt is «expr بساز‌از» — yield all values from iterable expr,
// delegating iteration (verb-final, like بساز).
type YieldFromStmt struct {
	L     int
	Value Expr
}

func (n *YieldFromStmt) Line() int { return n.L }
func (n *YieldFromStmt) stmtNode() {}

// YieldExpr is «expr بساز» (or verb-initial «بساز expr») used as an expression
// inside parentheses — e.g. «گروه(ای بساز)» or «(ای بساز)». Kolang mirrors
// Python's yield-as-expression, but only in parenthesized contexts so that the
// statement forms «expr بساز» / «expr بساز‌از» remain unambiguous.
type YieldExpr struct {
	L     int
	Value Expr
}

func (n *YieldExpr) Line() int { return n.L }
func (n *YieldExpr) exprNode() {}

// YieldFromExpr is «expr بساز‌از» (or verb-initial «بساز‌از expr») used as an
// expression inside parentheses.
type YieldFromExpr struct {
	L     int
	Value Expr
}

func (n *YieldFromExpr) Line() int { return n.L }
func (n *YieldFromExpr) exprNode() {}

// ImportStmt is «Module بیار» — import a module under its own name.
type ImportStmt struct {
	L      int
	Module string
}

func (n *ImportStmt) Line() int { return n.L }
func (n *ImportStmt) stmtNode() {}

// FromImportStmt is «از Module Name [بانام Alias] بیار» — import one member of
// a module, optionally under an alias.
type FromImportStmt struct {
	L      int
	Module string
	Name   string
	Alias  string
}

func (n *FromImportStmt) Line() int { return n.L }
func (n *FromImportStmt) stmtNode() {}

// BreakStmt is «اتمام» — exit the innermost loop.
type BreakStmt struct {
	L int
}

func (n *BreakStmt) Line() int { return n.L }
func (n *BreakStmt) stmtNode() {}

// ContinueStmt is «بروبعدی» — skip to the next loop iteration.
type ContinueStmt struct {
	L int
}

func (n *ContinueStmt) Line() int { return n.L }
func (n *ContinueStmt) stmtNode() {}

// AppendStmt is «Value به List بیافزا» — append a value to a list.
type AppendStmt struct {
	L     int
	List  Expr
	Value Expr
}

func (n *AppendStmt) Line() int { return n.L }
func (n *AppendStmt) stmtNode() {}

// RemoveStmt is «Value از List حذفکن» — remove the first equal value from a list.
type RemoveStmt struct {
	L     int
	List  Expr
	Value Expr
}

func (n *RemoveStmt) Line() int { return n.L }
func (n *RemoveStmt) stmtNode() {}

// ClassDef defines a class (گونه): Name, optional Parent (وارث) and an
// optional explicit interface (رهی). The Body contains method definitions
// (تعریف) and the constructor (ساخت), plus optional class-level fields.
type ClassDef struct {
	L          int
	Name       string
	Parent     string
	Implements string
	Body       []Stmt
}

func (n *ClassDef) Line() int { return n.L }
func (n *ClassDef) stmtNode() {}

// TryStmt is a try/except/finally block: بپا ... خطای‌X بگیر: ... درنهایت:
type TryStmt struct {
	L        int
	Body     *Block
	Handlers []*ExceptHandler
	Finally  *Block
}

func (n *TryStmt) Line() int { return n.L }
func (n *TryStmt) stmtNode() {}

// ExceptHandler is a single «خطای‌X بگیر [بانام name]:» clause.
type ExceptHandler struct {
	L         int
	Exception Expr   // nil for a bare «بگیر:» (catches everything)
	Alias     string // optional bound exception variable name
	Body      *Block
}

func (n *ExceptHandler) Line() int { return n.L }

// RaiseStmt is «expr بده» — raise an exception instance.
type RaiseStmt struct {
	L     int
	Value Expr
}

func (n *RaiseStmt) Line() int { return n.L }
func (n *RaiseStmt) stmtNode() {}

// DeferStmt is the postfix «call تأخیری»: run the call when the current
// function returns (LIFO, even on exception/return).
type DeferStmt struct {
	L    int
	Call Expr
}

func (n *DeferStmt) Line() int { return n.L }
func (n *DeferStmt) stmtNode() {}

// WithStmt is a context-manager statement: «با EXPR بانام NAME: body». EXPR is
// the context-manager expression (typically a بازکردن call); NAME is bound to
// the managed value inside the body.
type WithStmt struct {
	L       int
	Context Expr
	Name    string
	Body    []Stmt
}

func (n *WithStmt) Line() int { return n.L }
func (n *WithStmt) stmtNode() {}

// InterfaceDef defines an interface (رابط): a set of method signatures
// (تعریف name(params)) with no bodies. Structural typing means any class with
// those methods automatically satisfies the interface.
type InterfaceDef struct {
	L       int
	Name    string
	Methods []*DefStmt
}

func (n *InterfaceDef) Line() int { return n.L }
func (n *InterfaceDef) stmtNode() {}

// Block is an indented statement list: the body of a function, loop,
// conditional, or other block-structured statement.
type Block struct {
	L     int
	Stmts []Stmt
}

func (n *Block) Line() int { return n.L }
func (n *Block) stmtNode() {}
