package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
)

// Scan sites.
//
// A rule that calls no rule and reads and writes nothing but the input is called again at a position
// where a failed alternative led back (a transient rule is not memoized), and walks its body again.
// A repetition of a single character in such a rule, without an upper bound, is a scan site: the
// walk from its start ends at a position, and examines the input up to a position, that depend on
// the start and the input alone, so a record kept at the start holds for every later walk from there
// (core/scan_sites.go). A rule is of this kind when
//   - it has no attribute but @start, no type and no action, and its expression is made of
//     literals, character classes, any character, sequences, choices, groups, options, repetitions
//     and discards (no call, capture, predicate, lookahead, cut, recovery, stream or position test);
//   - it is called from two or more places, where an alternative that fails can lead a later
//     call back to the same position.
// The other repetitions of such a rule are restartable (run_sites.go): the rule is walked again at
// positions its walks have reached.
//
// A site numbers two sites after the run sites, one for the walks that build a value and one for the
// walks that do not, whose records differ.

// NoScanSites disables the marking of scan sites when a program is compiled, for tests and
// measurements that compare parses with and without them.
var NoScanSites bool

// scanRules returns the names of the rules whose single-character repetitions are scan sites.
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

// markScanSites numbers the single-character repetitions of the rules in scanRules.
func (prog *Program) markScanSites(control *cancel.Control, scanRules map[string]bool) {
	prog.scanRepeats = map[*grammar.Repeat]int{}
	for _, r := range prog.RuleList {
		if !scanRules[r.Name] {
			continue
		}
		walkExprControlled(r.Def.Expr, func(x grammar.Expr) {
			if rep, ok := x.(*grammar.Repeat); ok && rep.Max < 0 && isScan(rep) {
				prog.scanRepeats[rep] = prog.NumSkipSites + prog.NumRunSites + 1 + 2*prog.NumScanSites
				prog.NumScanSites++
			}
		}, control)
	}
}

// ScanSite returns the number of the scan site e, or 0 if e is not one; the walks that build a
// value use the next number (Walker.scanSiteOf).
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

// hasAttrsBesidesStart reports whether rd has an attribute or pragma other than @start.
func hasAttrsBesidesStart(rd *grammar.RuleDef) bool {
	for _, a := range rd.Attrs {
		if a.Pragma || a.Name != "start" {
			return true
		}
	}
	return false
}
