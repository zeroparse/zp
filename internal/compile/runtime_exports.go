package compile

import (
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Every declaration of this file exists only for the runtime side, which reads the checked
// program through this package and does not import the front end, and leaves when the runtime
// tables move to internal/runtime/program. Compiler-side code names internal/frontend/check and
// internal/frontend/types directly.

// Scope is check.Scope. It exists only for the runtime side and leaves when the runtime tables
// move to internal/runtime/program.
type Scope = check.Scope

// NewScope is check.NewScope. It exists only for the runtime side and leaves when the runtime
// tables move to internal/runtime/program.
func NewScope() *Scope { return check.NewScope() }

// NewProgram returns a program of the grammar g with no rules or types yet; g is nil for a
// program loaded from a compiled grammar. It exists only for the runtime side, which builds such
// a program without naming the semantic model, and leaves when the runtime tables move to
// internal/runtime/program.
func NewProgram(g *ast.Grammar) *Program {
	return &Program{Model: check.NewModel(g, nil), ByName: map[string]*Rule{}}
}

// These are the value shapes of check (check.Shaper, check.SingleMode and its modes). They exist
// only for the runtime side and leave when the runtime tables move to internal/runtime/program.
type (
	Shaper     = check.Shaper
	SingleMode = check.SingleMode
)

// The modes of SingleMode; see SingleMode.
const (
	SinglePass  = check.SinglePass
	SingleTuple = check.SingleTuple
)

// ValueInference is check.ValueInference. It exists only for the runtime side and leaves when the
// runtime tables move to internal/runtime/program.
type ValueInference = check.ValueInference

// VarTable is check.VarTable. It exists only for the runtime side and leaves when the runtime
// tables move to internal/runtime/program.
type VarTable = check.VarTable

// ModuleVarTable is check.ModuleVarTable. It exists only for the runtime side and leaves when the
// runtime tables move to internal/runtime/program.
func ModuleVarTable(m *bytecode.Module) *VarTable { return check.ModuleVarTable(m) }

// FieldOf is check.FieldOf. It exists only for the runtime side and leaves when the runtime tables
// move to internal/runtime/program.
func FieldOf(st *ast.TypeDef, name string) *ast.Field { return check.FieldOf(st, name) }

// DefFields is check.DefFields. It exists only for the runtime side and leaves when the runtime
// tables move to internal/runtime/program.
func DefFields(spec *ast.TypeDef) []*ast.Field { return check.DefFields(spec) }

// CapType is check.CapType. It exists only for the runtime side and leaves when the runtime tables
// move to internal/runtime/program.
func CapType(caps []types.CapIndex, name string) (types.Ty, bool) { return check.CapType(caps, name) }

// NodeTy is types.NodeTy. It exists only for the runtime side and leaves when the runtime tables
// move to internal/runtime/program.
type NodeTy = types.NodeTy

// FieldType is types.FieldType. It exists only for the runtime side and leaves when the runtime
// tables move to internal/runtime/program.
type FieldType = types.FieldType

// NewFieldType is types.NewFieldType. It exists only for the runtime side and leaves when the
// runtime tables move to internal/runtime/program.
func NewFieldType(t types.Ty) *FieldType { return types.NewFieldType(t) }

// ParseFieldType is types.ParseFieldType. It exists only for the runtime side and leaves when the
// runtime tables move to internal/runtime/program.
func ParseFieldType(text string, declared map[string]bool) (*FieldType, error) {
	return types.ParseFieldType(text, declared)
}
