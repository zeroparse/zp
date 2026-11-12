package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/check"
)

// The variable table lives in internal/check; these aliases keep the packages that name it through
// compile building and go with the other aliases.

type (
	VarDecl  = check.VarDecl
	VarTable = check.VarTable
)

// ModuleVarTable is check.ModuleVarTable.
func ModuleVarTable(m *bytecode.Module) *VarTable { return check.ModuleVarTable(m) }

// constantValue returns the value of an initializer: an int, a string or a bool literal, or an
// int literal with a minus sign. ok is false for any other expression.
func constantValue(t grammar.Term) (v any, ok bool) {
	switch t := t.(type) {
	case *grammar.TermGroup:
		return constantValue(t.Term)
	case *grammar.IntLit:
		return t.Value, true
	case *grammar.StringLit:
		return t.Value, true
	case *grammar.BoolLit:
		return t.Value, true
	case *grammar.Unary:
		if lit, isInt := t.X.(*grammar.IntLit); isInt && t.Op == "-" {
			return -lit.Value, true
		}
	}
	return nil, false
}

// declareVars records the variables of the grammar and reports the invalid declarations.
func (c *Compiler) declareVars(g *grammar.Grammar) {
	for _, d := range g.File.Decls {
		c.Control.Poll()
		vd, ok := d.(*grammar.VarDef)
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
			decl = &VarDecl{Name: vd.Name, Typ: TyInt, Init: 0}
		}
		c.Prog.Vars.Add(decl)
	}
}

// varDecl returns the declaration of vd, or nil after reporting why it is invalid.
func (c *Compiler) varDecl(vd *grammar.VarDef) *VarDecl {
	var typ BasicTy
	if vd.Type != nil {
		ref, ok := vd.Type.(*grammar.TypeRef)
		if ok {
			switch ref.Name {
			case "int":
				typ = TyInt
			case "string":
				typ = TyString
			case "bool":
				typ = TyBool
			default:
				ok = false
			}
		}
		if !ok {
			if ref, isRef := vd.Type.(*grammar.TypeRef); isRef {
				if to, retired := retiredTypeNames[ref.Name]; retired {
					c.errorf(vd.Diagnostic(), "%s is not a type; write %s", ref.Name, to)
					return nil
				}
			}
			c.errorf(vd.Diagnostic(), "variable %s must have type int, string, or bool, got %s", vd.Name, grammar.FormatType(vd.Type))
			return nil
		}
	}
	if vd.Value == nil {
		if vd.Type == nil {
			c.errorf(vd.Diagnostic(), "variable %s needs a type or an initial value", vd.Name)
			return nil
		}
		return &VarDecl{Name: vd.Name, Typ: typ, Init: check.ZeroValue(typ)}
	}
	if _, isNil := vd.Value.(*grammar.NilLit); isNil {
		c.errorf(termPos(vd.Value, vd.Diagnostic()), "variable %s cannot be nil; its type is int, string, or bool", vd.Name)
		return nil
	}
	v, ok := constantValue(vd.Value)
	if !ok {
		c.errorf(termPos(vd.Value, vd.Diagnostic()), "the initial value of variable %s must be an int, string, or bool literal", vd.Name)
		return nil
	}
	if vd.Type != nil && check.ValueType(v) != typ {
		c.errorf(termPos(vd.Value, vd.Diagnostic()), "cannot use %s as %s in the initial value of variable %s", check.ValueType(v), typ, vd.Name)
		return nil
	}
	return &VarDecl{Name: vd.Name, Typ: check.ValueType(v), Init: v}
}
