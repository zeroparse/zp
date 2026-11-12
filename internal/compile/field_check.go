package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/types"
)

// The field types live in internal/types; these aliases keep the packages that name them
// through compile building and go with the other type aliases.

type FieldType = types.FieldType

const (
	FtInt        = types.FtInt
	FtString     = types.FtString
	FtBool       = types.FtBool
	FtNil        = types.FtNil
	FtNode       = types.FtNode
	FtNamed      = types.FtNamed
	FtStringNode = types.FtStringNode
	FtError      = types.FtError
	FtMissing    = types.FtMissing
	FtList       = types.FtList
	FtOptional   = types.FtOptional
	FtJoin       = types.FtJoin
	FtNodeOf     = types.FtNodeOf
	FtTuple      = types.FtTuple
	FtTop        = types.FtTop
)

// NewFieldType is types.NewFieldType.
func NewFieldType(t Ty) *FieldType { return types.NewFieldType(t) }

// ParseFieldType is types.ParseFieldType.
func ParseFieldType(text string, declared map[string]bool) (*FieldType, error) {
	return types.ParseFieldType(text, declared)
}

// ValidateModule checks the module m (bytecode.ValidateModule), with the field types parsed against
// the types that m declares.
func ValidateModule(m *bytecode.Module) error {
	var typeNames map[string]bool
	return bytecode.ValidateModule(m, func(text string) error {
		if typeNames == nil {
			typeNames = ModuleTypeNames(m)
		}
		_, err := ParseFieldType(text, typeNames)
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
func (prog *Program) ResolveCheck(t grammar.TypeExpr) Ty {
	if t == nil {
		return nil
	}
	if r := (&TypeResolver{Prog: prog.Checker()}).Resolve(t); NewFieldType(r) != nil {
		return r
	}
	return nil
}

// CastType returns the type cast t checks its value against, or nil if every value fits it.
func (prog *Program) CastType(t *grammar.Cast) *FieldType {
	if v, ok := prog.fieldTypes.Load(t); ok {
		return v.(*FieldType)
	}
	var ft *FieldType
	if r := prog.ResolveCheck(t.Type); r != nil {
		ft = NewFieldType(r)
	}
	v, _ := prog.fieldTypes.LoadOrStore(t, ft)
	return v.(*FieldType)
}

// CastPos returns the grammar position of the cast t as line and column, or 0, 0 when the program
// records no source information or the position is unknown.
func (prog *Program) CastPos(t *grammar.Cast) (int, int) {
	if !prog.RecordsSourceInfo {
		return 0, 0
	}
	if pos := t.Diagnostic(); pos.IsValid() {
		return pos.Line, pos.Col
	}
	return 0, 0
}
