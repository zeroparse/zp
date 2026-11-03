package compile

import "zeroparse.dev/zp/grammar"

// Run sites.
//
// A repetition whose iterations the linear-cost classifier restarts (see "Classification" in
// spec/runtime/parsing.md) walks the same input again each time a failed alternative leads back to
// its start. A run site records what its walk from each position did, as a skip-run site does
// (skip_sites.go), and a later walk that reaches a recorded position in the same environment takes
// the rest from the record: where the walk ended and in which environment, the elements it
// collected, the errors it recovered, its farthest failure and the input it examined.
//
// The record holds if the rest of the walk is a function of the position, the variables and the
// input, and replaying it leaves the parse as the walk would. A repetition is a run site when
//   - it has no maximum, and no #cut applies to it from its element;
//   - its element has no Operators Rule, #stream, #missing, #error or #diagnostic (#recover, captures,
//     actions and predicates are allowed: a capture lives in the iteration, and a predicate reads
//     the variables, which are in the key);
//   - every unbounded repetition in its element is a run site, or repeats a single character;
//   - it does not repeat a single character itself (a scan walks it);
//   - every rule its element calls, directly or indirectly, is called without a level, is not
//     left-recursive (a leader or member of a cycle), and contains none of the above.

// NoRunSites disables the marking of run sites when a program is compiled, for tests and
// measurements that compare parses with and without them.
var NoRunSites = false

func (prog *Program) markRunSites(control *Control, defs []*grammar.RuleDef, calls map[string][]string, recursive map[string]bool) {
	prog.runSites = map[*grammar.Repeat]int{}
	if NoRunSites {
		return
	}
	bad := map[string]bool{}
	for name := range recursive {
		bad[name] = true
	}
	for _, r := range prog.RuleList {
		control.poll()
		walkExprControlled(r.Def.Expr, func(x grammar.Expr) {
			if blocksCallee(x) {
				bad[r.Name] = true
			}
		}, control)
	}
	propagate(defs, calls, bad, control)
	memo := map[*grammar.Repeat]bool{}
	var eligible func(e *grammar.Repeat) bool
	eligible = func(e *grammar.Repeat) bool {
		if ok, done := memo[e]; done {
			return ok
		}
		memo[e] = false
		if e.Max >= 0 || escapingCut(e.Expr) {
			return false
		}
		ok := true
		walkExprControlled(e.Expr, func(x grammar.Expr) {
			if !ok {
				return
			}
			switch x := x.(type) {
			case *grammar.Repeat:
				ok = x.Max >= 0 || eligible(x)
			case *grammar.Ref:
				r := prog.ByName[x.Name]
				ok = r != nil && x.Level == "" && !bad[x.Name]
			default:
				ok = !blocksRun(x)
			}
		}, control)
		memo[e] = ok
		return ok
	}
	for _, r := range prog.RuleList {
		control.poll()
		walkExprControlled(r.Def.Expr, func(x grammar.Expr) {
			rep, ok := x.(*grammar.Repeat)
			if !ok || prog.skipSites[rep] != 0 || prog.runSites[rep] != 0 || !eligible(rep) || isScan(rep) {
				return
			}
			prog.NumRunSites++
			prog.runSites[rep] = prog.NumSkipSites + prog.NumRunSites
		}, control)
	}
}

// isScan reports whether e repeats a single character, which a scan walks without a record.
func isScan(e *grammar.Repeat) bool {
	switch e.Expr.(type) {
	case *grammar.CharClass, *grammar.Any:
		return true
	}
	return false
}

// blocksRun reports whether the expression x keeps a repetition that holds it from being a run
// site, besides the repetitions and calls in it.
func blocksRun(x grammar.Expr) bool {
	if _, ok := x.(*grammar.OperatorsRule); ok {
		return true
	}
	return blocksCallee(x)
}

// blocksCallee reports whether the expression x in a rule keeps a repetition that calls the rule
// from being a run site: #stream, #missing, #error and #diagnostic.
func blocksCallee(x grammar.Expr) bool {
	switch x := x.(type) {
	case *grammar.Stream:
		return true
	case *grammar.Attributed:
		return blocksRunAttrs(x)
	}
	return false
}

// RunSite returns the number of the run site e, or 0 if e is not one. Numbers follow those of the
// skip-run sites, so a site's records share the parser's tables with them.
func (prog *Program) RunSite(e *grammar.Repeat) int { return prog.runSites[e] }
