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
//     left-recursive (a leader or member of a cycle), and contains none of the above;
//   - it is restartable (restartableRepeats): a repetition that no iteration can reach is walked
//     from each position at most once, so no walk can reuse another's record.
//
// The records of a run site are keyed by the environment, and a walk that reuses one takes its end
// environment, unless the site is free of the environment (envFree): then the rest of a walk from a
// position is the same in every environment and leaves the environment as it found it, so its
// records hold for every environment. Skip-run sites are free of it by their conditions.

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
	names := make([]string, len(prog.RuleList))
	for i, r := range prog.RuleList {
		names[i] = r.Name
	}
	restart := restartableRepeats(names, func(n string) grammar.Expr { return prog.ByName[n].Def.Expr }, calls)
	readsVars := func(n string) bool { r := prog.ByName[n]; return r == nil || len(r.Vars) > 0 }
	scanRules := prog.scanRules(control, defs, calls)
	for _, r := range prog.RuleList {
		if scanRules[r.Name] {
			walkExprControlled(r.Def.Expr, func(x grammar.Expr) {
				if rep, ok := x.(*grammar.Repeat); ok {
					restart[rep] = true
				}
			}, control)
		}
	}
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
			if !ok || prog.skipSites[rep] != 0 || prog.runSites[rep] != 0 || !restart[rep] || !eligible(rep) || isScan(rep) {
				return
			}
			prog.NumRunSites++
			site := prog.NumSkipSites + prog.NumRunSites
			prog.runSites[rep] = site
			if envFree(rep, readsVars) {
				prog.markEnvFree(site)
			}
		}, control)
	}
	prog.markScanSites(control, scanRules)
}

// EnvFree reports whether the skip-run or run site is free of the environment: its records are
// kept by position alone, and a walk that reuses one keeps its own environment. Otherwise the
// records are keyed by the environment the walk from a position starts in, and a walk that reuses
// one takes its end environment; a site of which nothing is known is keyed, which is always
// correct.
func (prog *Program) EnvFree(site int) bool { return site < len(prog.envFree) && prog.envFree[site] }

// markEnvFree marks the site free of the environment (EnvFree).
func (prog *Program) markEnvFree(site int) {
	for len(prog.envFree) <= site {
		prog.envFree = append(prog.envFree, false)
	}
	prog.envFree[site] = true
}

// EnvFreeMutation, when not zero, leaves a part out of envFree, for the mutation tests of the
// differential tests of run sites; nothing else sets it.
var EnvFreeMutation int

// The parts that EnvFreeMutation leaves out.
const (
	EnvFreeAll           = 1 + iota // every site is free of the environment
	EnvFreeCalleesIgnore            // the variables that the called rules read are left out
)

// envFree reports whether the run site e is free of the environment: neither its element nor a rule
// that the element calls, directly or indirectly, reads a variable, and the element binds none.
// readsVars reports whether a rule or the rules it calls read a variable (Rule.Vars). A binding
// made in a called rule is undone when the rule returns (Interaction With Memoization in
// spec/language/predicates.md), so only the element's own bindings count; one there would carry
// into the iterations after it and the environment after the walk. Then the rest of a walk from a
// position is a function of the position and the input, and ends in the environment it began in.
// Inline actions are lowered to calls of hidden rules before the sites are marked, so the calls
// stand for them.
func envFree(e *grammar.Repeat, readsVars func(rule string) bool) bool {
	if EnvFreeMutation == EnvFreeAll {
		return true
	}
	free := true
	WalkExpr(e.Expr, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Predicate:
			free = free && !readsOrBindsVariable(x.Term)
		case *grammar.Ref:
			free = free && (EnvFreeMutation == EnvFreeCalleesIgnore || !readsVars(x.Name))
		}
	})
	return free
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

// restartableRepeats returns the repetitions that an iteration of a repetition can reach: those in
// the element of another repetition that iterates (its maximum is not 0 or 1), and those in a rule
// that such an element calls, or that calls itself, directly or indirectly. Every other repetition is reached by a
// derivation once, from the start rule through sequences, choices and calls, so one walk of it
// starts at each position at most once per environment, and a record could not be reused.
func restartableRepeats(names []string, exprOf func(string) grammar.Expr, calls map[string][]string) map[*grammar.Repeat]bool {
	out := map[*grammar.Repeat]bool{}
	iterated := map[string]bool{}
	var queue []string
	mark := func(n string) {
		if !iterated[n] {
			iterated[n] = true
			queue = append(queue, n)
		}
	}
	for _, n := range names {
		WalkExpr(exprOf(n), func(x grammar.Expr) {
			r, ok := x.(*grammar.Repeat)
			if !ok || r.Max == 0 || r.Max == 1 {
				return
			}
			WalkExpr(r.Expr, func(y grammar.Expr) {
				switch y := y.(type) {
				case *grammar.Repeat:
					out[y] = true
				case *grammar.Ref:
					mark(y.Name)
				}
			})
		})
	}
	// A rule that calls itself, directly or through others, repeats as an iteration does: a right
	// recursion walks the repetitions in its rules from each position its alternatives advance to.
	for _, n := range names {
		seen := map[string]bool{}
		stack := append([]string(nil), calls[n]...)
		for len(stack) > 0 && !iterated[n] {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if c == n {
				mark(n)
			} else if !seen[c] {
				seen[c] = true
				stack = append(stack, calls[c]...)
			}
		}
	}
	for len(queue) > 0 {
		n := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, c := range calls[n] {
			mark(c)
		}
	}
	for _, n := range names {
		if !iterated[n] {
			continue
		}
		WalkExpr(exprOf(n), func(x grammar.Expr) {
			if r, ok := x.(*grammar.Repeat); ok {
				out[r] = true
			}
		})
	}
	return out
}
