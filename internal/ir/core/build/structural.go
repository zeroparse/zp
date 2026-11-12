package build

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/check"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/types"
)

// Structural builds the untyped Core IR module of in: the control structure, the calls, the
// captures and the variable reads of its rules, with no types, shapes, twins or rule flags. Every
// rule builds its value, no repetition projects, and a cast checks nothing. The facts of the
// module (internal/ir/core/facts) read this structure only, so they are those of the typed module
// of the same rules. If origin is not nil, it records the expression each node is built from.
func Structural(in *Input, origin map[core.Node]grammar.Expr) (*core.Module, error) {
	return coreModule(structural{in}, origin, false)
}

// structural is the Program of a module built without a checker.
type structural struct{ in *Input }

func (s structural) CoreInput() *Input                    { return s.in }
func (structural) Checker() *check.Checker                { return nil }
func (structural) ResolveCheck(grammar.TypeExpr) types.Ty { return nil }
func (structural) CastPos(*grammar.Cast) (int, int)       { return 0, 0 }
func (structural) SkipSite(*grammar.Repeat) int           { return 0 }
