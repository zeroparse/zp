package print

import (
	"fmt"
	"zeroparse.dev/zp/internal/backend/codegen/common"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func (g *Generator) tracePlainCall(r *common.Rule, name string) string {
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
