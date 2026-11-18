package compile

import (
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// constantValue returns the value of an initializer: an int, a string or a bool literal, or an
// int literal with a minus sign. ok is false for any other expression.
func constantValue(t ast.Term) (v any, ok bool) {
	switch t := t.(type) {
	case *ast.TermGroup:
		return constantValue(t.Term)
	case *ast.IntLit:
		return t.Value, true
	case *ast.StringLit:
		return t.Value, true
	case *ast.BoolLit:
		return t.Value, true
	case *ast.Unary:
		if lit, isInt := t.X.(*ast.IntLit); isInt && t.Op == "-" {
			return -lit.Value, true
		}
	}
	return nil, false
}

// declareVars records the variables of the grammar and reports the invalid declarations.
func (c *Compiler) declareVars(g *ast.Grammar) {
	for _, d := range g.File.Decls {
		c.Control.Poll()
		vd, ok := d.(*ast.VarDef)
		if !ok {
			continue
		}
		if c.Prog.Vars == nil {
			c.Prog.Vars = check.NewVarTable()
		}
		if c.Prog.Vars.Lookup(vd.Name) != nil {
			c.errorf(vd.Diagnostic(), "variable %s is already defined", vd.Name)
			continue
		}
		decl := c.varDecl(vd)
		if decl == nil {
			// Keep the name declared, so that its uses do not add errors of their own.
			decl = &check.VarDecl{Name: vd.Name, Typ: types.TyInt, Init: 0}
		}
		c.Prog.Vars.Add(decl)
	}
}

// varDecl returns the declaration of vd, or nil after reporting why it is invalid.
func (c *Compiler) varDecl(vd *ast.VarDef) *check.VarDecl {
	var typ types.BasicTy
	if vd.Type != nil {
		ref, ok := vd.Type.(*ast.TypeRef)
		if ok {
			switch ref.Name {
			case "int":
				typ = types.TyInt
			case "string":
				typ = types.TyString
			case "bool":
				typ = types.TyBool
			default:
				ok = false
			}
		}
		if !ok {
			if ref, isRef := vd.Type.(*ast.TypeRef); isRef {
				if to, retired := check.RetiredTypeNames[ref.Name]; retired {
					c.errorf(vd.Diagnostic(), "%s is not a type; write %s", ref.Name, to)
					return nil
				}
			}
			c.errorf(vd.Diagnostic(), "variable %s must have type int, string, or bool, got %s", vd.Name, fmt.FormatType(vd.Type))
			return nil
		}
	}
	if vd.Value == nil {
		if vd.Type == nil {
			c.errorf(vd.Diagnostic(), "variable %s needs a type or an initial value", vd.Name)
			return nil
		}
		return &check.VarDecl{Name: vd.Name, Typ: typ, Init: check.ZeroValue(typ)}
	}
	if _, isNil := vd.Value.(*ast.NilLit); isNil {
		c.errorf(check.TermPos(vd.Value, vd.Diagnostic()), "variable %s cannot be nil; its type is int, string, or bool", vd.Name)
		return nil
	}
	v, ok := constantValue(vd.Value)
	if !ok {
		c.errorf(check.TermPos(vd.Value, vd.Diagnostic()), "the initial value of variable %s must be an int, string, or bool literal", vd.Name)
		return nil
	}
	if vd.Type != nil && check.ValueType(v) != typ {
		c.errorf(check.TermPos(vd.Value, vd.Diagnostic()), "cannot use %s as %s in the initial value of variable %s", check.ValueType(v), typ, vd.Name)
		return nil
	}
	return &check.VarDecl{Name: vd.Name, Typ: check.ValueType(v), Init: v}
}
