package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/desugar"
)

// Scan sites.
//
// A rule that calls no rule is not memoized (a transient rule), so it is walked again at every
// position where a failed alternative leads back, as a run of blanks is. A repetition of a single
// character (a character class or any character) without an upper bound in such a rule is a scan
// site: an iteration reads one character and nothing else, so the walk from a start ends at a
// position, examines the input up to a position, matches a number of characters and builds
// elements that depend on the start and the input alone, whatever the rule around it does, and a
// record kept at the start holds for every later walk from there (core/scan_sites.go). A
// repetition that is a skip-run site records its walks as one, and one that delivers the elements
// of a #stream is not a site.
//
// The rules whose other repetitions are made restartable (run_sites.go), pure scan rules, are
// narrower: they are also called from two places or more, have no attribute but @start, no type and
// no action, and their expressions are made of literals, character classes, any character,
// sequences, choices, groups, options, repetitions and discards.
//
// A site numbers two sites after the run sites, one for the walks that build a value and one for the
// walks that do not, whose records differ.

// NoScanSites disables the marking of scan sites when a program is compiled, for tests and
// measurements that compare parses with and without them.
var NoScanSites bool

// scanRules returns the names of the pure scan rules, whose other repetitions are restartable.
func (prog *Program) scanRules(control *cancel.Control, defs []*grammar.RuleDef, calls map[string][]string) map[string]bool {
	out := map[string]bool{}
	if NoScanSites || NoRunSites {
		return out
	}
	refs := map[string]int{}
	for _, cs := range calls {
		for _, c := range cs {
			refs[c]++
		}
	}
	for _, rd := range defs {
		control.Poll()
		if refs[rd.Name] >= 2 && !hasAttrsBesidesStart(rd) && rd.Type == nil && rd.Action == nil && len(calls[rd.Name]) == 0 && pureScan(rd.Expr, control) {
			out[rd.Name] = true
		}
	}
	return out
}

// markScanSites numbers the scan sites of the rules that call no rule.
func (prog *Program) markScanSites(control *cancel.Control, calls map[string][]string) {
	prog.scanRepeats = map[*grammar.Repeat]int{}
	if NoScanSites || NoRunSites {
		return
	}
	for _, r := range prog.RuleList {
		if len(calls[r.Name]) != 0 {
			continue
		}
		ScanSiteRepeats(r.Def.Expr, control, func(rep *grammar.Repeat, site bool) {
			if site && prog.skipSites[rep] == 0 {
				prog.scanRepeats[rep] = prog.NumSkipSites + prog.NumRunSites + 1 + 2*prog.NumScanSites
				prog.NumScanSites++
			}
		})
	}
}

// ScanSiteRepeats calls f for each repetition without an upper bound in e, the body of a rule that
// calls no rule, in order, with whether it is a scan site: it repeats a character class or any
// character and does not deliver the elements of a #stream. A repetition in a #recover skip is
// reported as a site; it is one unless it is a skip-run site, which records its walks as one.
func ScanSiteRepeats(e grammar.Expr, control *cancel.Control, f func(rep *grammar.Repeat, site bool)) {
	streamed := map[*grammar.Repeat]bool{}
	walkExprControlled(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Attributed:
			if rep, ok := x.Expr.(*grammar.Repeat); ok && IsStream(x) {
				streamed[rep] = true
			}
		case *grammar.Stream:
			if rep, ok := x.Expr.(*grammar.Repeat); ok {
				streamed[rep] = true
			}
		case *grammar.Repeat:
			if x.Max < 0 {
				f(x, IsScan(x) && !streamed[x])
			}
		}
	}, control)
}

// ScanSite returns the number of the scan site e, or 0 if e is not one; the walks that build a
// value use the next number.
func (prog *Program) ScanSite(e *grammar.Repeat) int { return prog.scanRepeats[e] }

// pureScan reports whether e is made only of the forms of the body of a scan site.
func pureScan(e grammar.Expr, control *cancel.Control) bool {
	ok := true
	walkExprControlled(e, func(x grammar.Expr) {
		switch x.(type) {
		case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Seq, *grammar.Choice,
			*grammar.Group, *grammar.Optional, *grammar.Repeat, *grammar.Discard:
		default:
			ok = false
		}
	}, control)
	return ok
}

// hasAttrsBesidesStart reports whether rd has an attribute or pragma other than @start and
// @linear, which only add checks.
func hasAttrsBesidesStart(rd *grammar.RuleDef) bool {
	for _, a := range rd.Attrs {
		if (a.Pragma || a.Name != "start") && !desugar.IsLinear(a) {
			return true
		}
	}
	return false
}
