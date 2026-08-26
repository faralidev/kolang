//go:build ignore

// Command gen_keywords generates the machine-readable keywords.json from the
// canonical keyword source of truth: the Keywords map in token.go, annotated
// with the Categories map in categories.go.
//
// It is excluded from the regular build via the "ignore" build tag and is only
// invoked through `go generate ./internal/token/`.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/faralidev/kolang/internal/token"
)

// typeName maps a token Type back to its exported Go constant name, so the
// generated JSON references the identifier (DEF, IF, ...) rather than the raw
// Persian spelling.
var typeName = map[token.Type]string{
	token.DEF:        "DEF",
	token.RETURN:     "RETURN",
	token.IF:         "IF",
	token.ELSE:       "ELSE",
	token.WHILE:      "WHILE",
	token.FOR:        "FOR",
	token.IN:         "IN",
	token.FROM:       "FROM",
	token.TO:         "TO",
	token.STEP:       "STEP",
	token.BREAK:      "BREAK",
	token.CONTINUE:   "CONTINUE",
	token.PRINT:      "PRINT",
	token.INPUT:      "INPUT",
	token.PASS:       "PASS",
	token.IMPORT:     "IMPORT",
	token.APPEND:     "APPEND",
	token.REMOVE:     "REMOVE",
	token.RAISE:      "RAISE",
	token.TRUE:       "TRUE",
	token.FALSE:      "FALSE",
	token.NONE:       "NONE",
	token.SELF:       "SELF",
	token.SUPER:      "SUPER",
	token.AS:         "AS",
	token.WITH:       "WITH",
	token.SEP:        "SEP",
	token.BEH:        "BEH",
	token.AND:        "AND",
	token.OR:         "OR",
	token.COP_POS:    "COP_POS",
	token.COP_NEG:    "COP_NEG",
	token.GO:         "GO",
	token.CHANNEL:    "CHANNEL",
	token.CLOSE:      "CLOSE",
	token.CLOSED:     "CLOSED",
	token.GLOBAL:     "GLOBAL",
	token.NONLOCAL:   "NONLOCAL",
	token.CLASS:      "CLASS",
	token.INTERF:     "INTERF",
	token.TRY:        "TRY",
	token.FINALLY:    "FINALLY",
	token.DEFER:      "DEFER",
	token.YIELD:      "YIELD",
	token.YIELDFROM:  "YIELDFROM",
	token.DECOR:      "DECOR",
	token.IMPLEMENTS: "IMPLEMENTS",
	token.EXTENDS:    "EXTENDS",
}

// entry is a single keyword's metadata in the generated JSON.
type entry struct {
	Type     string `json:"type"`
	Category string `json:"category"`
}

// doc is the top-level structure written to keywords.json.
type doc struct {
	Comment   string           `json:"_comment"`
	Version   string           `json:"_version"`
	Source    string           `json:"_source"`
	Generator string           `json:"_generator"`
	Keywords  map[string]entry `json:"keywords"`
}

func main() {
	out := flag.String("o", "../../keywords.json", "output path, relative to internal/token/")
	flag.Parse()

	keywords := make(map[string]entry, len(token.Keywords))
	var missing []string
	for spelling, typ := range token.Keywords {
		cat, ok := token.Categories[typ]
		if !ok {
			missing = append(missing, fmt.Sprintf("%q (%s)", spelling, typ))
			continue
		}
		name, ok := typeName[typ]
		if !ok {
			fmt.Fprintf(os.Stderr, "gen_keywords: no typeName entry for token type %q (add it to the typeName map)\n", typ)
			os.Exit(1)
		}
		keywords[spelling] = entry{Type: name, Category: cat}
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "gen_keywords: no category for type(s): %v\n", missing)
		os.Exit(1)
	}

	d := doc{
		Comment:   "کلمات کلیدی زبان کلنگ — تولیدشده از internal/token/token.go. دستی ویرایش نکنید.",
		Version:   "0.0.1",
		Source:    "internal/token/token.go",
		Generator: "internal/token/gen_keywords.go (go generate)",
		Keywords:  keywords,
	}

	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen_keywords: marshal: %v\n", err)
		os.Exit(1)
	}
	raw = append(raw, '\n')

	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gen_keywords: write %s: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Printf("gen_keywords: wrote %d keyword spellings to %s\n", len(keywords), *out)
}
