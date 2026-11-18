package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/frontend/desugar"
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
	Skip, Run, Scan map[*grammar.Repeat]int
	// NumSkip, NumRun and NumScan count them.
	NumSkip, NumRun, NumScan int
	// EnvFree[site] reports a skip-run or run site whose records are not keyed by the environment.
	EnvFree []bool
}

// SetSites sets the site numbers of the program, which Check leaves unset.
func (prog *Program) SetSites(s Sites) {
	prog.sites = s
	prog.NumSkipSites, prog.NumRunSites, prog.NumScanSites = s.NumSkip, s.NumRun, s.NumScan
}

// SkipSite returns the number of the skip-run site e (from 1), or 0 if e is not one.
func (prog *Program) SkipSite(e *grammar.Repeat) int { return prog.sites.Skip[e] }

// RunSite returns the number of the run site e, or 0 if e is not one. Numbers follow those of the
// skip-run sites, so a site's records share the parser's tables with them.
func (prog *Program) RunSite(e *grammar.Repeat) int { return prog.sites.Run[e] }

// ScanSite returns the number of the scan site e, or 0 if e is not one; the walks that build a
// value use the next number.
func (prog *Program) ScanSite(e *grammar.Repeat) int { return prog.sites.Scan[e] }

// EnvFree reports whether the skip-run or run site is free of the environment: its records are
// kept by position alone, and a walk that reuses one keeps its own environment. Otherwise the
// records are keyed by the environment too, and a walk that reuses one takes its end environment.
func (prog *Program) EnvFree(site int) bool {
	return site < len(prog.sites.EnvFree) && prog.sites.EnvFree[site]
}

// DeclaredBeyondScan reports whether the declaration of rd keeps it from being a pure scan rule
// (corepass.SiteRule.Declared): it has a type, an action, or an attribute or pragma other than
// @start and @linear, which only add checks.
func DeclaredBeyondScan(rd *grammar.RuleDef) bool {
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
