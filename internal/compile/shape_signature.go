package compile

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// ShapeSignature is ShapeSignature.
func (p *Program) ShapeSignature(name string, poll func()) (string, bool) {
	if p == nil || p.Typed == nil {
		return "", false
	}
	r := p.ByName[name]
	if r == nil || r.Def == nil {
		return "", false
	}
	step := func() {
		if poll != nil {
			poll()
		}
	}
	// The subexpressions of lookaheads are left out.
	inside := map[ast.Expr]bool{}
	WalkExpr(r.Def.Expr, func(e ast.Expr) {
		step()
		var body ast.Expr
		switch x := e.(type) {
		case *ast.And:
			body = x.Expr
		case *ast.Not:
			body = x.Expr
		}
		if body != nil {
			inside[e] = true
			WalkExpr(body, func(x ast.Expr) { step(); inside[x] = true })
		}
	})
	var b strings.Builder
	WalkExpr(r.Def.Expr, func(e ast.Expr) {
		step()
		if inside[e] {
			return
		}
		fmt.Fprintf(&b, "%T{", e)
		for _, el := range shapeElems(p, e) {
			step()
			if el.Type() != nil {
				b.WriteString(el.Type().String())
			}
			for _, c := range el.Captures() {
				fmt.Fprintf(&b, "<%s %v %v %v>", c.Name, c.Paths, c.Side, c.Maybe)
				if c.T != nil {
					b.WriteString(c.T.String())
				}
			}
			b.WriteByte(';')
		}
		b.WriteString("}")
	})
	return b.String(), true
}
