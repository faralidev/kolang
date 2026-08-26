//go:generate go run gen_keywords.go -o ../../keywords.json

// categories.go supplies the semantic category annotations consumed by the
// keywords.json generator (gen_keywords.go). The canonical spelling list is
// the Keywords map in token.go; this file maps each keyword Type to a
// category, and the generator refuses to run if any keyword lacks one.
package token

// Categories classifies every keyword token Type into a semantic group. Each
// Type reachable from the Keywords map in token.go must have an entry here;
// the generator (gen_keywords.go) fails loudly on any missing category.
var Categories = map[Type]string{
	// control — flow-control keywords
	IF:       "control",
	ELSE:     "control",
	WHILE:    "control",
	FOR:      "control",
	FROM:     "control",
	TO:       "control",
	STEP:     "control",
	IN:       "control",
	TRY:      "control",
	FINALLY:  "control",
	BREAK:    "control",
	CONTINUE: "control",

	// declaration — definitions, types, interfaces, inheritance, decorators
	DEF:        "declaration",
	CLASS:      "declaration",
	INTERF:     "declaration",
	EXTENDS:    "declaration",
	IMPLEMENTS: "declaration",
	DECOR:      "declaration",

	// copula — to-be verbs (باشد / نباشد / است)
	COP_POS: "copula",
	COP_NEG: "copula",

	// logical — boolean connectors
	AND: "logical",
	OR:  "logical",

	// scope — variable scoping declarations (v1.0)
	GLOBAL:   "scope",
	NONLOCAL: "scope",

	// concurrency — goroutines and channels (v0.6)
	GO:      "concurrency",
	CHANNEL: "concurrency",
	CLOSE:   "concurrency",
	CLOSED:  "concurrency",

	// verb — action verbs
	PRINT:     "verb",
	RETURN:    "verb",
	APPEND:    "verb",
	REMOVE:    "verb",
	RAISE:     "verb",
	IMPORT:    "verb",
	INPUT:     "verb",
	YIELD:     "verb",
	YIELDFROM: "verb",
	DEFER:     "verb",

	// literal — boolean / null literals
	TRUE:  "literal",
	FALSE: "literal",
	NONE:  "literal",

	// self_super — self / parent references
	SELF:  "self_super",
	SUPER: "self_super",

	// other — miscellaneous keywords
	AS:   "other",
	WITH: "other",
	SEP:  "other",
	BEH:  "other",
	PASS: "other",
}
