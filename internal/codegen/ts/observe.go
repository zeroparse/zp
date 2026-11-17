package tsgen

import (
	"fmt"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func (g *Generator) tracePlainCall(r *compile.Rule, name string) string {
	if !g.Observability {
		return name
	}
	body := name + "ObservedBody"
	fmt.Fprintf(&g.Methods, `// Observes one actual generated rule call.
function %s(p: Parser): R {
  const o = p.observation;
  if (o === null) return %s(p);
  o.enter(%s, p.pos, 0);
  o.evaluated();
  const v = %s(p);
  o.exit(v !== undefined, p.pos);
  return v;
}

`, name, body, TsString(r.Name), body)
	return body
}

func (g *Generator) traceSkipped(expr ast.Expr) string {
	if !g.Observability {
		return ""
	}
	for {
		switch e := expr.(type) {
		case *ast.Seq:
			if len(e.Items) == 0 {
				return ""
			}
			expr = e.Items[0]
		case *ast.Capture:
			expr = e.Expr
		case *ast.Atomic:
			expr = e.Expr
		case *ast.Discard:
			expr = e.Expr
		case *ast.Ref:
			min := 0
			if e.Level != "" {
				for i, l := range g.Prog.ByName[e.Name].Def.Expr.(*ast.OperatorsRule).Levels {
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
