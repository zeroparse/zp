package program

import "zeroparse.dev/zp/pkg/bytecode"

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
