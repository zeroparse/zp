package gogen

import (
	"fmt"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func (g *Generator) tracePlainWrapper(r *compile.Rule, name string) string {
	return g.traceCallWrapper(r, name, true)
}

func (g *Generator) traceCallWrapper(r *compile.Rule, name string, evaluated bool) string {
	body := name + "ObservedBody"
	eval := ""
	if evaluated {
		eval = "p.Observation.Evaluated()\n"
	}
	fmt.Fprintf(&g.Methods, `// %[1]s observes one actual direct rule call.
func %[1]s(p *%[2]s) (%[3]s, bool) {
 if p.Observation == nil { return %[4]s(p) }
 p.Observation.Enter(%[5]q, p.Pos, 0)
 h := p.ExaminedBegin()
 %[6]sv, ok := %[4]s(p)
 x := p.ExaminedEnd(h)
 p.Observation.Exit(ok, p.Pos, x)
 return v, ok
}

`, name, g.recv(), g.valType(), body, r.Name, eval)
	return body
}

// A guard may suppress a whole sequence or a chain of rule references. Report
// only its first identifiable rule call site, without inventing nested calls.
func (g *Generator) traceSkip(expr ast.Expr) string {
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
			return fmt.Sprintf("\np.Observation.Skipped(%q, p.Pos, 0)\n", e.Name)
		default:
			return ""
		}
	}
}
