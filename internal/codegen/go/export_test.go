package gogen

import (
	"zeroparse.dev/zp/internal/ir/exec/lower"
	"zeroparse.dev/zp/internal/ir/exec/pass"
)

// The tests of package gogen_test compile grammars with the engine, which imports packages that
// the generator must not depend on, so they cannot live in package gogen. These declarations
// expose to them the unexported internals that they check.

type (
	Dvar         = dvar
	LiveRules    = liveRules
	RuleKey      = ruleKey
	ShakeUnit    = shakeUnit
	TypesPackage = typesPackage
)

const LibImportPath = libImportPath

// ExecVarsError returns the error that the check of the variables of a printed body gives, for a body
// whose text is text and whose locals dead are not declared.
func ExecVarsError(text string, dead ...string) error {
	g := &Generator{}
	res := &BodyResult{Text: text, Stores: &pass.Stores{Dead: map[string]bool{}}}
	for _, v := range dead {
		res.Stores.Dead[v] = true
	}
	(&flowPrinter{g: g}).execVars(&lower.Lowered{}, res)
	return g.FlowErr
}

var (
	ClassBits          = classBits
	DeclaredNames      = declaredNames
	DefaultPackageName = defaultPackageName
	FreeIdents         = freeIdents
	KeptUnits          = keptUnits
	LoadLib            = loadLib
	QualifyLib         = qualifyLib
	RuleTableLimits    = ruleTableLimits
	ShakableVar        = shakableVar
	ShakeVars          = shakeVars
	TrimComments       = trimComments
	VarLinesExcept     = varLinesExcept
)

// ReachFrom calls reachFrom.
func (l *LiveRules) ReachFrom(roots []RuleKey) { l.reachFrom(roots) }

// MissingBodies calls missingBodies.
func (l *LiveRules) MissingBodies(table string, printed map[RuleKey]bool) []RuleKey {
	return l.missingBodies(table, printed)
}

// LoweredTermNames returns the names of the library functions and methods that only lowered terms call.
func LoweredTermNames() []string { return loweredTermNames() }
