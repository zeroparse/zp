package compile

import (
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// ValidateModule is program.ValidateModule.
func ValidateModule(m *bytecode.Module) error { return program.ValidateModule(m) }

// ModuleTypeNames is program.ModuleTypeNames.
func ModuleTypeNames(m *bytecode.Module) map[string]bool { return program.ModuleTypeNames(m) }

// ResolveCheck is check.Checker.ResolveCheck.
func (prog *Program) ResolveCheck(t ast.TypeExpr) types.Ty { return prog.Checker().ResolveCheck(t) }

// CastType is check.Checker.CastType.
func (prog *Program) CastType(t *ast.Cast) *types.FieldType { return prog.Checker().CastType(t) }

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
