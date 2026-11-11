package compile

import "zeroparse.dev/zp/grammar"

// scanOnly reports whether a call of the rule name pays at most a constant when it is evaluated
// again at a position where it was evaluated before: the rule reads no variable and calls no rule,
// so each of its evaluations at a position is the same; every repetition without an upper bound in
// its body as written is a scan site (ScanSiteRepeats); its body has no #recover, Operators Rule or
// #stream; and every scanning source that the walk found in it is such a repetition (no copying
// operation). The records of scan sites then answer each walk that starts again at a position after
// it matched ScanHotSpan characters, with a constant number of steps ("Scan Sites" in
// spec/runtime/parsing.md), so the evaluations that repeat the call at one position pay a constant
// each, besides at most three full walks of each walk there in each mode.
//
// The test reads the rules as written, not their view: the view merges choices of single
// characters into classes and unrolls repetitions, which the runtime does not, so a repetition of a
// class in the view need not be a scan site.
//
// It is false for every rule until the sources are settled (computeScanOnly).
func (k *classifier) scanOnly(name string) bool { return k.scanOnlyRules[name] }

// computeScanOnly finds the rules for scanOnly, once the scanning sources of the walk are settled.
func (k *classifier) computeScanOnly() {
	out := map[string]bool{}
	for _, r := range k.rules {
		if k.reach[r.Name] && k.scanOnlyRule(r.Name) {
			out[r.Name] = true
		}
	}
	k.scanOnlyRules = out
}

func (k *classifier) scanOnlyRule(name string) bool {
	if NoScanSites || NoRunSites {
		return false
	}
	var def *grammar.RuleDef
	rules := k.base
	if rules == nil {
		rules = k.rules
	}
	for _, rd := range rules {
		if rd.Name == name {
			def = rd
		}
	}
	if def == nil || len(k.vars[name]) > 0 || len(allCallsControlled(def.Expr, nil, k.control)) != 0 {
		return false
	}
	ok := true
	walkExprControlled(def.Expr, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Recovery, *grammar.OperatorsRule, *grammar.Stream:
			ok = false
		case *grammar.Attributed:
			ok = ok && !IsStream(x)
		}
	}, k.control)
	ScanSiteRepeats(def.Expr, k.control, func(_ *grammar.Repeat, site bool) { ok = ok && site })
	if !ok {
		return false
	}
	for _, s := range k.walked[name] {
		if (s.scan == nil || k.srcExprs[s.scan]) && (s.scan == nil || !isScan(s.scan)) {
			return false
		}
	}
	return true
}
