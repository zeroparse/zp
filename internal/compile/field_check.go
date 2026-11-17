package compile

import (
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// ValidateModule checks the module m (bytecode.ValidateModule), with the field types parsed against
// the types that m declares.
func ValidateModule(m *bytecode.Module) error {
	var typeNames map[string]bool
	return bytecode.ValidateModule(m, func(text string) error {
		if typeNames == nil {
			typeNames = ModuleTypeNames(m)
		}
		_, err := types.ParseFieldType(text, typeNames)
		return err
	})
}

// ModuleTypeNames returns the set of the names of a module's declared types.
func ModuleTypeNames(m *bytecode.Module) map[string]bool {
	names := make(map[string]bool, len(m.Types))
	for _, t := range m.Types {
		if t.Name >= 0 && t.Name < len(m.Strings) {
			names[m.Strings[t.Name]] = true
		}
	}
	return names
}

// ResolveCheck resolves a type for a cast, as a value type. It returns nil for _, which every
// value fits.
func (prog *Program) ResolveCheck(t ast.TypeExpr) types.Ty {
	if t == nil {
		return nil
	}
	if r := (&check.TypeResolver{Prog: prog.Checker()}).Resolve(t); types.NewFieldType(r) != nil {
		return r
	}
	return nil
}

// CastType returns the type cast t checks its value against, or nil if every value fits it.
func (prog *Program) CastType(t *ast.Cast) *types.FieldType {
	if v, ok := prog.fieldTypes.Load(t); ok {
		return v.(*types.FieldType)
	}
	var ft *types.FieldType
	if r := prog.ResolveCheck(t.Type); r != nil {
		ft = types.NewFieldType(r)
	}
	v, _ := prog.fieldTypes.LoadOrStore(t, ft)
	return v.(*types.FieldType)
}

// CastPos returns the grammar position of the cast t as line and column, or 0, 0 when the program
// records no source information or the position is unknown.
func (prog *Program) CastPos(t *ast.Cast) (int, int) {
	if !prog.RecordsSourceInfo {
		return 0, 0
	}
	if pos := t.Diagnostic(); pos.IsValid() {
		return pos.Line, pos.Col
	}
	return 0, 0
}
