package tsgen

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
)

func (g *Generator) tracePlainCall(r *compile.Rule, name string) string {
	if !g.Observability {
		return name
	}
	body := name + "ObservedBody"
	fmt.Fprintf(&g.Methods, `// Observes one actual generated rule call.
function %s(p: Parser): R {
  if (p.observation === null) return %s(p);
  return (p as ObservedParser).observePlain(%s, %s);
}

`, name, body, TsString(r.Name), body)
	return body
}

func (g *Generator) traceSkipped(expr grammar.Expr) string {
	if !g.Observability {
		return ""
	}
	for {
		switch e := expr.(type) {
		case *grammar.Seq:
			if len(e.Items) == 0 {
				return ""
			}
			expr = e.Items[0]
		case *grammar.Capture:
			expr = e.Expr
		case *grammar.Atomic:
			expr = e.Expr
		case *grammar.Discard:
			expr = e.Expr
		case *grammar.Ref:
			min := 0
			if e.Level != "" {
				for i, l := range g.Prog.ByName[e.Name].Def.Expr.(*grammar.OperatorsRule).Levels {
					if l.Name == e.Level {
						min = i
					}
				}
			}
			return fmt.Sprintf("    p.observation?.skipped(%s, p.pos, %d);\n", TsString(e.Name), min)
		default:
			return ""
		}
	}
}
