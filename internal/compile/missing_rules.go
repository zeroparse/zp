package compile

import (
	"fmt"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// markMissingRules records transitive insertion dependencies before matchers or
// value-free twins are compiled. Unrelated rules retain completed memo reuse.
func (prog *Program) markMissingRules(control *cancel.Control) {
	var usesMissingType func(ast.TypeExpr) bool
	usesMissingType = func(t ast.TypeExpr) bool {
		control.Poll()
		switch t := t.(type) {
		case *ast.TypeRef:
			return t.Name == coreimpl.TypeMissing
		case *ast.ListType:
			return usesMissingType(t.Elem)
		case *ast.OptionalType:
			return usesMissingType(t.Elem)
		case *ast.TypeGroup:
			return usesMissingType(t.Type)
		case *ast.JoinType:
			for _, t := range t.Types {
				if usesMissingType(t) {
					return true
				}
			}
		case *ast.MeetType:
			for _, t := range t.Types {
				if usesMissingType(t) {
					return true
				}
			}
		case *ast.TupleType:
			for _, t := range t.Types {
				if usesMissingType(t) {
					return true
				}
			}
		case *ast.TypeApp:
			for _, t := range t.Args {
				if usesMissingType(t) {
					return true
				}
			}
		case *ast.StructType:
			for _, f := range t.Fields {
				if usesMissingType(f.Type) {
					return true
				}
			}
		}
		return false
	}
	for _, td := range control.Types(prog.Grammar) {
		switch t := td.(type) {
		case *ast.TypeDef:
			for _, f := range check.DefFields(t) {
				if usesMissingType(f.Type) {
					prog.HasMissing = true
				}
			}
		case *ast.TypeAlias:
			if usesMissingType(t.Type) {
				prog.HasMissing = true
			}
		}
	}
	refs := make(map[*Rule][]string)
	for _, r := range prog.RuleList {
		if usesMissingType(r.Def.Type) {
			prog.HasMissing = true
		}
		walkExprControlled(r.Def.Expr, func(e ast.Expr) {
			control.Poll()
			switch e := e.(type) {
			case *ast.Attributed:
				for _, a := range e.Attrs {
					if a.Name == "missing" {
						r.Missing = true
						prog.HasMissing = true
					}
				}
			case *ast.Ref:
				refs[r] = append(refs[r], e.Name)
			}
		}, control)
	}
	for changed := true; changed; {
		changed = false
		for _, r := range prog.RuleList {
			control.Poll()
			if r.Missing {
				continue
			}
			for _, name := range refs[r] {
				control.Poll()
				if callee := prog.ByName[name]; callee != nil && callee.Missing {
					r.Missing = true
					changed = true
					break
				}
			}
		}
	}
}

// MissingUnsupported returns the error of an operation that does not support missing-token
// recovery.
func MissingUnsupported(operation string) error {
	return fmt.Errorf("missing-token recovery is not supported for %s; use Closure whole-input parsing or recognition", operation)
}
