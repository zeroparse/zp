package compile

import "zeroparse.dev/zp/grammar"

// Run sites in the classifier (see Run Sites in spec/runtime/parsing.md).
//
// A repetition without an upper bound whose walks keep records, as those of a skip-run site do, is
// not a scanning source: the records bound the evaluations of its element at each position by a
// constant. The engine marks the same repetitions (run_sites.go). The marking is off in the
// classification until the proofs of the bounds exist, so the classes do not move; the tests and
// measurements switch it on.
var runSitesOn = false

// runSite reports whether the repetition e is a run site: the records of its walks can be keyed by
// position, and its element costs a constant outside the calls it makes.
func (k *classifier) runSite(e *grammar.Repeat) bool {
	if !runSitesOn || e.Max >= 0 || escapingCut(e.Expr) {
		return false
	}
	if v, ok := k.runMemo[e]; ok {
		return v
	}
	k.badRules()
	ok := true
	WalkExpr(e.Expr, func(x grammar.Expr) {
		if !ok {
			return
		}
		switch x := x.(type) {
		case *grammar.Repeat:
			ok = x.Max >= 0 || k.runSite(x)
		case *grammar.OperatorsRule, *grammar.Stream:
			ok = false
		case *grammar.Attributed:
			ok = !blocksRunAttrs(x)
		case *grammar.Ref:
			ok = k.byName[x.Name] != nil && x.Level == "" && !k.runBad[x.Name]
		}
	})
	if k.runMemo == nil {
		k.runMemo = map[*grammar.Repeat]bool{}
	}
	k.runMemo[e] = ok
	return ok
}

// blocksRunAttrs reports whether the attributes of x keep a repetition that holds it from being a
// run site: #stream, #missing, #error and #diagnostic.
func blocksRunAttrs(x *grammar.Attributed) bool {
	for _, a := range x.Attrs {
		switch a.Name {
		case "stream", "missing", "error", "diagnostic":
			return true
		}
	}
	return false
}

// badRules computes the rules that a record cannot stand for: rules that are on a left-recursion
// cycle or contain an Operators Rule, #stream, #missing, #error or #diagnostic, directly or through
// calls.
func (k *classifier) badRules() {
	if k.runBad != nil {
		return
	}
	k.runBad = map[string]bool{}
	for _, r := range k.rules {
		if k.an.cyclic[r.Name] {
			k.runBad[r.Name] = true
		}
		WalkExpr(r.Expr, func(x grammar.Expr) {
			switch x := x.(type) {
			case *grammar.Stream:
				k.runBad[r.Name] = true
			case *grammar.Attributed:
				if blocksRunAttrs(x) {
					k.runBad[r.Name] = true
				}
			}
		})
	}
	for changed := true; changed; {
		changed = false
		for _, r := range k.rules {
			if k.runBad[r.Name] {
				continue
			}
			for _, c := range k.calls[r.Name] {
				if k.runBad[c] {
					k.runBad[r.Name] = true
					changed = true
					break
				}
			}
		}
	}
}
