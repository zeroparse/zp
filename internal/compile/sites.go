package compile

import (
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Sites numbers the skip-run, run and scan sites of a program's repetitions: the repetitions whose
// walks a parser records (see "Skip Runs", "Run Sites" and "Scan Sites" in
// spec/runtime/parsing.md). The numbering is an analysis of the Core IR module of the rules
// (internal/middleend/corepass, ComputeSites); the front end keys its result by the repetitions of
// the grammar that the module's repetitions were built from, which the Core IR of the program, its
// value-free twins included, is built from again.
type Sites struct {
	// Skip, Run and Scan number the skip-run sites from 1, the run sites after them and the scan
	// sites after those, two numbers each: one for the walks that build a value and the next for
	// those that do not.
	Skip, Run, Scan map[*ast.Repeat]int
	// NumSkip, NumRun and NumScan count them.
	NumSkip, NumRun, NumScan int
	// EnvFree[site] reports a skip-run or run site whose records are not keyed by the environment.
	EnvFree []bool
	// RunKeys holds, by site, the key of each run site that is not free of the environment.
	RunKeys map[int]RunKey
}

// RunKey holds the variables that key the records of a run site that is not free of the
// environment. Vars are the variables that its element reads, directly or in the rules it calls,
// and those that the element binds itself, Bind: a walk from a position is a function of the
// position, the input and the values of Vars, and changes no variable but those of Bind, since a
// binding made in a called rule ends when the rule returns. A walk that takes a record sets the
// variables of Bind to the values that the walk of the record ended with. Keying by the bound
// variables as well makes a binding that leaves a value as it was the same as no binding, which a
// walk cannot tell apart. Both lists are sorted.
type RunKey struct {
	Vars, Bind []string
}

// SetSites sets the site numbers of the program, which Check leaves unset.
func (prog *Program) SetSites(s Sites) {
	prog.sites = s
	prog.NumSkipSites, prog.NumRunSites, prog.NumScanSites = s.NumSkip, s.NumRun, s.NumScan
}

// SkipSite returns the number of the skip-run site e (from 1), or 0 if e is not one.
func (prog *Program) SkipSite(e *ast.Repeat) int { return prog.sites.Skip[e] }

// RunSite returns the number of the run site e, or 0 if e is not one. Numbers follow those of the
// skip-run sites, so a site's records share the parser's tables with them.
func (prog *Program) RunSite(e *ast.Repeat) int { return prog.sites.Run[e] }

// ScanSite returns the number of the scan site e, or 0 if e is not one; the walks that build a
// value use the next number.
func (prog *Program) ScanSite(e *ast.Repeat) int { return prog.sites.Scan[e] }

// EnvFree reports whether the skip-run or run site is free of the environment: its records are
// kept by position alone, and a walk that reuses one keeps its own environment. Otherwise the
// records are keyed by the environment too, and a walk that reuses one takes its end environment.
func (prog *Program) EnvFree(site int) bool {
	return site < len(prog.sites.EnvFree) && prog.sites.EnvFree[site]
}

// RunKey returns the variables that key the records of the run site (RunKey); a site free of the
// environment, or no run site, has none.
func (prog *Program) RunKey(site int) RunKey { return prog.sites.RunKeys[site] }

// RunKeys returns the keys of the run sites that are not free of the environment, by site.
func (prog *Program) RunKeys() map[int]RunKey { return prog.sites.RunKeys }

// SetRunKey sets the key of the run site: a program built from a compiled grammar has the keys of
// its module.
func (prog *Program) SetRunKey(site int, k RunKey) {
	if prog.sites.RunKeys == nil {
		prog.sites.RunKeys = map[int]RunKey{}
	}
	prog.sites.RunKeys[site] = k
}

// DeclaredBeyondScan reports whether the declaration of rd keeps it from being a pure scan rule
// (corepass.SiteRule.Declared): it has a type, an action, or an attribute or pragma other than
// @start and @linear, which only add checks.
func DeclaredBeyondScan(rd *ast.RuleDef) bool {
	if rd.Type != nil || rd.Action != nil {
		return true
	}
	for _, a := range rd.Attrs {
		if (a.Pragma || a.Name != "start") && !desugar.IsLinear(a) {
			return true
		}
	}
	return false
}
