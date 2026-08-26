package eval

import (
	"strings"

	"github.com/faralidev/kolang/internal/parser"
)

// --- string interpolation ---

// interpolate expands the {expr} placeholders of a string literal at runtime.
// «{{» and «}}» escape literal braces; the embedded expression is scanned with
// brace-depth tracking (ignoring braces inside nested «...» strings) and
// evaluated via evalInterpExpr, then stringified into the result.
func (e *Eval) interpolate(raw string, env *Env, line int) (Value, error) {
	var sb strings.Builder
	i := 0
	for i < len(raw) {
		if raw[i] == '{' {
			if i+1 < len(raw) && raw[i+1] == '{' {
				sb.WriteByte('{')
				i += 2
				continue
			}
			depth := 1
			strDepth := 0 // nesting of «...» strings inside the interpolation
			j := i + 1
			for j < len(raw) && depth > 0 {
				// « and » are multi-byte runes; match them as prefixes so a
				// trailing byte can never be mistaken for one. Inside a
				// «...» string a literal '}' does not close the interpolation.
				if strings.HasPrefix(raw[j:], "«") {
					strDepth++
					j++
					continue
				}
				if strings.HasPrefix(raw[j:], "»") {
					if strDepth > 0 {
						strDepth--
					}
					j++
					continue
				}
				switch raw[j] {
				case '{':
					if strDepth == 0 {
						depth++
					}
				case '}':
					if strDepth == 0 {
						depth--
					}
				}
				if depth > 0 {
					j++
				}
			}
			if depth != 0 {
				return nil, &RuntimeError{Line: line, Msg: "عبارت درون متن تمام نشده است"}
			}
			inner := raw[i+1 : j]
			v, err := e.evalInterpExpr(inner, env, line)
			if err != nil {
				return nil, err
			}
			sb.WriteString(Stringify(v))
			i = j + 1
		} else if raw[i] == '}' {
			if i+1 < len(raw) && raw[i+1] == '}' {
				sb.WriteByte('}')
				i += 2
				continue
			}
			sb.WriteByte('}')
			i++
		} else {
			sb.WriteByte(raw[i])
			i++
		}
	}
	return sb.String(), nil
}

// evalInterpExpr parses and evaluates the inner text of an interpolation
// placeholder as a single expression.
func (e *Eval) evalInterpExpr(src string, env *Env, line int) (Value, error) {
	expr, err := parser.ParseSingleExpr(src)
	if err != nil {
		return nil, &RuntimeError{Line: line, Msg: "عبارت درون متن نامعتبر است: " + err.Error()}
	}
	return e.evalExpr(expr, env)
}
