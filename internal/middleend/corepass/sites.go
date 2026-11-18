package corepass

import (
	"slices"

	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/ir/core"
)

// Site numbering.
//
// A site is a repetition whose walks a parser records, so that a later walk from a recorded
// position takes the rest of it from the record (see "Skip Runs", "Run Sites" and "Scan Sites" in
// spec/runtime/parsing.md). ComputeSites numbers the sites of a module: the skip-run sites from 1,
// then the run sites, then two numbers for each scan site, so that the records of every site share
// the parser's tables.
//
// Skip-run sites. A #recover skip runs at every position where its body fails, and a repetition in
// it walks forward from there; when recovery keeps failing in a loop, each walk covers the input
// the walks before it covered. A repetition is a skip-run site when it is written in a skip, not
// inside another repetition of that skip that is a site, and
//   - it has no maximum, and no cut of its element applies to it;
//   - its element has no predicate that reads or binds a variable (a skip has no captures);
//   - neither its element nor a rule the element calls, directly or not, reads a variable,
//     contains #recover, or is a leader or member of a left-recursion cycle.
//
// Run sites. A repetition whose iterations a failed alternative restarts walks the same input
// again. A repetition is a run site when
//   - it has no maximum, and no cut of its element applies to it;
//   - its element has no Operators Rule, #stream, #missing, @error or @diagnostic;
//   - every unbounded repetition in its element, or in a rule its element calls, directly or not,
//     is a run site, a skip-run site, or repeats a single character, skips included
//     (dropUnrecordedReach);
//   - it does not repeat a single character itself (a scan walks it);
//   - every rule its element calls, directly or not, is called with no level, is not a leader or
//     member of a cycle, and contains no #stream, #missing, @error or @diagnostic; it may be an
//     Operators Rule, which, called with no level, parses all of its levels as a function of the
//     position, the variables and the input;
//   - it is restartable (restartable): a repetition that no iteration can reach is walked from
//     each position at most once, so no walk could reuse another's record.
//
// The records of a run site are keyed by the environment unless the site is free of it (EnvFree):
// neither its element's predicates nor the rules it calls read or bind a variable. Skip-run sites
// are free of it by their conditions. The records of a run site that is not free of it are keyed
// by the values of the variables of its run key (RunKey).
//
// Scan sites. A rule that calls no rule is transient, so it is walked again at every position
// where a failed alternative leads back. A repetition of a single character without a maximum in
// such a rule, outside a #stream and not a skip-run site, is a scan site. The pure scan rules,
// those also called at two places or more, whose declaration has no type, action or attribute
// besides @start and @linear (SiteRule.Declared), and whose bodies are made of literals, classes,
// any character, sequences, choices, options, repetitions and discards, make their other
// repetitions restartable.

// NoRunSites disables the run sites and the scan sites, and NoScanSites the scan sites, for tests
// and measurements that compare parses with and without them.
var NoRunSites, NoScanSites bool

// EnvFreeMutation, when not zero, leaves a part out of the test of EnvFree, for the mutation tests
// of the run-site records.
var EnvFreeMutation int

// The parts that EnvFreeMutation leaves out.
const (
	EnvFreeAll           = 1 + iota // every site is free of the environment
	EnvFreeCalleesIgnore            // the variables that the called rules read are left out
)

// NoRecordedReach, when set, leaves out the condition of dropUnrecordedReach, for the negative
// control of its tests; nothing else sets it.
var NoRecordedReach bool

// RunKeyMutation, when not zero, leaves a part out of the keys of run sites (runKey), for the
// mutation tests of the run-site records; nothing else sets it.
var RunKeyMutation int

// The parts that RunKeyMutation leaves out.
const (
	RunKeyNoCallees = 1 + iota // the variables that the called rules read
)

// SiteRule is what the site numbering reads of a rule besides its body: the rule flags of the
// program the sites are numbered for, which may differ from the module's facts (a program built
// with the flags of token insertion).
type SiteRule struct {
	// LeftRec reports a leader or member of a left-recursion cycle.
	LeftRec bool
	// Vars are the variables that the rule or a rule it calls reads.
	Vars []string
	// Declared reports a declaration with a type, an action, or an attribute other than @start
	// and @linear: such a rule is not a pure scan rule.
	Declared bool
}

// Sites numbers the sites of a module.
type Sites struct {
	// Skip, Run and Scan number the skip-run, run and scan sites; a scan site's walks that build a
	// value use the number after its own.
	Skip, Run, Scan map[*core.Repeat]int
	// NumSkip, NumRun and NumScan count them.
	NumSkip, NumRun, NumScan int
	// envFree[site] reports a skip-run or run site free of the environment.
	envFree []bool
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

// EnvFree reports whether the skip-run or run site is free of the environment: its records are
// kept by position alone.
func (s *Sites) EnvFree(site int) bool { return site < len(s.envFree) && s.envFree[site] }

// EnvFreeSites returns, by site number, whether each skip-run or run site is free of the
// environment.
func (s *Sites) EnvFreeSites() []bool { return s.envFree }

func (s *Sites) markEnvFree(site int) {
	for len(s.envFree) <= site {
		s.envFree = append(s.envFree, false)
	}
	s.envFree[site] = true
}

// ComputeSites numbers the sites of the rules of m, which has no value-free twins; rules[i] is
// what the numbering reads of rule i. A non-nil control polls for cancellation.
func ComputeSites(m *core.Module, rules []SiteRule, control *cancel.Control) *Sites {
	s := &sites{m: m, rules: rules, control: control, calls: make([][]int, len(m.Rules)),
		out: &Sites{Skip: map[*core.Repeat]int{}, Run: map[*core.Repeat]int{}, Scan: map[*core.Repeat]int{}, RunKeys: map[int]RunKey{}}}
	for _, r := range m.Rules {
		control.Poll()
		if r == nil {
			continue
		}
		s.order = append(s.order, r.ID)
		core.Walk(r.Body, func(n core.Node) {
			if k, ok := calleeOf(n); ok {
				s.calls[r.ID] = append(s.calls[r.ID], k)
			}
		})
	}
	s.skipSites()
	s.runSites()
	s.scanSites()
	return s.out
}

type sites struct {
	m       *core.Module
	rules   []SiteRule
	control *cancel.Control
	order   []int
	// calls holds, for each rule, the rules its body calls, at each call in order.
	calls [][]int
	// recursive holds the leaders and members of cycles and the rules that call one.
	recursive []bool
	out       *Sites
}

// calleeOf returns the rule that n calls, if n is a call.
func calleeOf(n core.Node) (int, bool) {
	switch n := n.(type) {
	case *core.Call:
		return n.Rule, true
	case *core.Nest:
		return n.Rule, true
	}
	return 0, false
}

// propagate adds to set every rule that calls a rule of set, directly or not.
func (s *sites) propagate(set []bool) {
	for changed := true; changed; {
		s.control.Poll()
		changed = false
		for _, id := range s.order {
			if set[id] {
				continue
			}
			for _, k := range s.calls[id] {
				if set[k] {
					set[id], changed = true, true
					break
				}
			}
		}
	}
}

// contains reports whether the body n has a node that f accepts.
func contains(n core.Node, f func(core.Node) bool) bool {
	found := false
	core.Walk(n, func(x core.Node) { found = found || f(x) })
	return found
}

// readsOrBindsVariable reports a predicate that reads a variable, or a binding.
func readsOrBindsVariable(n core.Node) bool {
	switch n := n.(type) {
	case *core.BindNode:
		return true
	case *core.Pred:
		found := false
		core.WalkActionExpr(n.Term, func(t core.ActionExpr) {
			if _, ok := t.(*core.VarRef); ok {
				found = true
			}
		})
		return found
	}
	return false
}

func (s *sites) skipSites() {
	n := len(s.m.Rules)
	recovers := make([]bool, n)
	s.recursive = make([]bool, n)
	for _, id := range s.order {
		s.recursive[id] = s.rules[id].LeftRec
		recovers[id] = contains(s.m.Rules[id].Body, func(x core.Node) bool { _, ok := x.(*core.Recover); return ok })
	}
	s.propagate(recovers)
	s.propagate(s.recursive)
	eligible := func(e *core.Repeat) bool {
		if e.Max >= 0 || looseCut(e.Body) {
			return false
		}
		return !contains(e.Body, func(x core.Node) bool {
			if k, ok := calleeOf(x); ok {
				return len(s.rules[k].Vars) > 0 || recovers[k] || s.recursive[k]
			}
			_, recover := x.(*core.Recover)
			return recover || readsOrBindsVariable(x)
		})
	}
	inSite := map[*core.Repeat]bool{}
	// mark numbers the outermost eligible repetitions of the skip.
	mark := func(skip core.Node) {
		core.Walk(skip, func(x core.Node) {
			rep, ok := x.(*core.Repeat)
			if !ok || s.out.Skip[rep] != 0 || inSite[rep] || !eligible(rep) {
				return
			}
			s.out.NumSkip++
			s.out.Skip[rep] = s.out.NumSkip
			s.out.markEnvFree(s.out.NumSkip)
			core.Walk(rep.Body, func(y core.Node) {
				if r, ok := y.(*core.Repeat); ok {
					inSite[r] = true
				}
			})
		})
	}
	for _, id := range s.order {
		s.control.Poll()
		core.Walk(s.m.Rules[id].Body, func(x core.Node) {
			if r, ok := x.(*core.Recover); ok {
				mark(r.Skip)
			}
		})
	}
}

// blocksCallee reports a node that keeps a repetition that calls its rule from being a run site.
func blocksCallee(x core.Node) bool {
	switch x.(type) {
	case *core.Stream, *core.Missing, *core.Label:
		return true
	}
	return false
}

// isScan reports a repetition of a single character, which a scan walks without a record.
func isScan(e *core.Repeat) bool {
	switch e.Body.(type) {
	case *core.ClassNode, *core.Any:
		return true
	}
	return false
}

func (s *sites) runSites() {
	if NoRunSites {
		return
	}
	bad := make([]bool, len(s.m.Rules))
	for _, id := range s.order {
		bad[id] = s.recursive[id] || contains(s.m.Rules[id].Body, blocksCallee)
	}
	s.propagate(bad)
	restart := s.restartable()
	for _, id := range s.scanRules() {
		core.Walk(s.m.Rules[id].Body, func(x core.Node) {
			if rep, ok := x.(*core.Repeat); ok {
				restart[rep] = true
			}
		})
	}
	memo := map[*core.Repeat]bool{}
	var eligible func(e *core.Repeat) bool
	eligible = func(e *core.Repeat) bool {
		if ok, done := memo[e]; done {
			return ok
		}
		memo[e] = false
		if e.Max >= 0 || looseCut(e.Body) {
			return false
		}
		ok := true
		core.Walk(e.Body, func(x core.Node) {
			if !ok {
				return
			}
			switch x := x.(type) {
			case *core.Repeat:
				ok = x.Max >= 0 || eligible(x)
			case *core.Call:
				ok = x.Level == 0 && !bad[x.Rule]
			case *core.Nest:
				ok = !bad[x.Rule]
			case *core.OperatorsRule:
				ok = false
			default:
				ok = !blocksCallee(x)
			}
		})
		memo[e] = ok
		return ok
	}
	var candidates []*core.Repeat
	candidate := map[*core.Repeat]bool{}
	for _, id := range s.order {
		s.control.Poll()
		core.Walk(s.m.Rules[id].Body, func(x core.Node) {
			rep, ok := x.(*core.Repeat)
			if !ok || s.out.Skip[rep] != 0 || candidate[rep] || !restart[rep] || !eligible(rep) || isScan(rep) {
				return
			}
			candidate[rep] = true
			candidates = append(candidates, rep)
		})
	}
	s.dropUnrecordedReach(candidates, candidate)
	for _, rep := range candidates {
		if !candidate[rep] {
			continue
		}
		s.out.NumRun++
		site := s.out.NumSkip + s.out.NumRun
		s.out.Run[rep] = site
		if s.envFree(rep) {
			s.out.markEnvFree(site)
		} else {
			s.out.RunKeys[site] = s.runKey(rep)
		}
	}
}

// dropUnrecordedReach removes from candidate the repetitions whose element, or a rule that the
// element calls, directly or not, holds a repetition without a maximum that is neither a
// candidate, a skip-run site nor a repetition of a single character: a walk of such a site would
// repeat work that no record bounds each time it evaluates the element. Skips count, so a
// #recover whose skip repeats without a record is such work too. The removal repeats until no
// candidate holds one, so the candidates that stay are the greatest set closed under the
// condition: a site may reach another that reaches it back through calls that consume input.
func (s *sites) dropUnrecordedReach(candidates []*core.Repeat, candidate map[*core.Repeat]bool) {
	if NoRecordedReach {
		return
	}
	// holds lists the repetitions without a maximum in n that need a record.
	holds := func(n core.Node) []*core.Repeat {
		var out []*core.Repeat
		core.Walk(n, func(x core.Node) {
			if r, ok := x.(*core.Repeat); ok && r.Max < 0 && s.out.Skip[r] == 0 && !isScan(r) {
				out = append(out, r)
			}
		})
		return out
	}
	ruleHolds := make([][]*core.Repeat, len(s.m.Rules))
	for _, id := range s.order {
		s.control.Poll()
		ruleHolds[id] = holds(s.m.Rules[id].Body)
	}
	reach := make(map[*core.Repeat][]*core.Repeat, len(candidates))
	for _, rep := range candidates {
		s.control.Poll()
		var inner []*core.Repeat
		for _, r := range holds(rep.Body) {
			if r != rep {
				inner = append(inner, r)
			}
		}
		seen := map[int]bool{}
		var stack []int
		core.Walk(rep.Body, func(x core.Node) {
			if k, ok := calleeOf(x); ok {
				stack = append(stack, k)
			}
		})
		for len(stack) > 0 {
			k := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[k] {
				continue
			}
			seen[k] = true
			inner = append(inner, ruleHolds[k]...)
			stack = append(stack, s.calls[k]...)
		}
		reach[rep] = inner
	}
	for changed := true; changed; {
		s.control.Poll()
		changed = false
		for _, rep := range candidates {
			if !candidate[rep] {
				continue
			}
			for _, r := range reach[rep] {
				if !candidate[r] {
					candidate[rep], changed = false, true
					break
				}
			}
		}
	}
}

// runKey returns the RunKey of the run site e.
func (s *sites) runKey(e *core.Repeat) RunKey {
	vars, bind := map[string]bool{}, map[string]bool{}
	readVars := func(t core.ActionExpr) {
		core.WalkActionExpr(t, func(x core.ActionExpr) {
			if v, ok := x.(*core.VarRef); ok {
				vars[v.Name] = true
			}
		})
	}
	core.Walk(e.Body, func(x core.Node) {
		switch x := x.(type) {
		case *core.Pred:
			readVars(x.Term)
		case *core.BindNode:
			vars[x.Var], bind[x.Var] = true, true
			readVars(x.Term)
		default:
			if k, ok := calleeOf(x); ok && RunKeyMutation != RunKeyNoCallees {
				for _, v := range s.rules[k].Vars {
					vars[v] = true
				}
			}
		}
	})
	return RunKey{Vars: sortedNames(vars), Bind: sortedNames(bind)}
}

func sortedNames(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// envFree reports whether the run site e is free of the environment.
func (s *sites) envFree(e *core.Repeat) bool {
	if EnvFreeMutation == EnvFreeAll {
		return true
	}
	return !contains(e.Body, func(x core.Node) bool {
		if k, ok := calleeOf(x); ok {
			return EnvFreeMutation != EnvFreeCalleesIgnore && len(s.rules[k].Vars) > 0
		}
		return readsOrBindsVariable(x)
	})
}

// restartable returns the repetitions that an iteration of a repetition can reach: those in the
// element of a repetition that iterates (its maximum is not 0 or 1), and those in a rule that such
// an element calls, or that calls itself, directly or not. Every other repetition is reached by a
// derivation once, from the start rule through sequences, choices and calls, so one walk of it
// starts at each position at most once per environment.
func (s *sites) restartable() map[*core.Repeat]bool {
	out := map[*core.Repeat]bool{}
	iterated := make([]bool, len(s.m.Rules))
	var queue []int
	mark := func(id int) {
		if !iterated[id] {
			iterated[id] = true
			queue = append(queue, id)
		}
	}
	for _, id := range s.order {
		core.Walk(s.m.Rules[id].Body, func(x core.Node) {
			r, ok := x.(*core.Repeat)
			if !ok || r.Max == 0 || r.Max == 1 {
				return
			}
			core.Walk(r.Body, func(y core.Node) {
				if rep, ok := y.(*core.Repeat); ok {
					out[rep] = true
				} else if k, ok := calleeOf(y); ok {
					mark(k)
				}
			})
		})
	}
	// A rule that calls itself, directly or through others, repeats as an iteration does: a right
	// recursion walks the repetitions of its rules from each position its alternatives advance to.
	for _, id := range s.order {
		seen := map[int]bool{}
		stack := append([]int(nil), s.calls[id]...)
		for len(stack) > 0 && !iterated[id] {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if c == id {
				mark(id)
			} else if !seen[c] {
				seen[c] = true
				stack = append(stack, s.calls[c]...)
			}
		}
	}
	for len(queue) > 0 {
		id := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, c := range s.calls[id] {
			mark(c)
		}
	}
	for _, id := range s.order {
		if iterated[id] {
			core.Walk(s.m.Rules[id].Body, func(x core.Node) {
				if r, ok := x.(*core.Repeat); ok {
					out[r] = true
				}
			})
		}
	}
	return out
}

// scanRules returns the pure scan rules, in order.
func (s *sites) scanRules() []int {
	if NoScanSites || NoRunSites {
		return nil
	}
	refs := make([]int, len(s.m.Rules))
	for _, id := range s.order {
		for _, k := range s.calls[id] {
			refs[k]++
		}
	}
	var out []int
	for _, id := range s.order {
		s.control.Poll()
		if refs[id] >= 2 && !s.rules[id].Declared && len(s.calls[id]) == 0 && pureScan(s.m.Rules[id].Body) {
			out = append(out, id)
		}
	}
	return out
}

// pureScan reports whether n is made only of the forms of the body of a pure scan rule.
func pureScan(n core.Node) bool {
	return !contains(n, func(x core.Node) bool {
		switch x.(type) {
		case *core.Lit, *core.ClassNode, *core.Any, *core.Seq, *core.Choice, *core.Opt, *core.Repeat, *core.Drop:
			return false
		}
		return true
	})
}

// scanSites numbers the scan sites of the rules that call no rule.
func (s *sites) scanSites() {
	if NoScanSites || NoRunSites {
		return
	}
	for _, id := range s.order {
		if len(s.calls[id]) != 0 {
			continue
		}
		s.control.Poll()
		streamed := map[*core.Repeat]bool{}
		body := s.m.Rules[id].Body
		core.Walk(body, func(x core.Node) {
			if st, ok := x.(*core.Stream); ok {
				streamed[st.Repeat] = true
			}
		})
		core.Walk(body, func(x core.Node) {
			rep, ok := x.(*core.Repeat)
			if ok && rep.Max < 0 && isScan(rep) && !streamed[rep] && s.out.Skip[rep] == 0 {
				s.out.Scan[rep] = s.out.NumSkip + s.out.NumRun + 1 + 2*s.out.NumScan
				s.out.NumScan++
			}
		})
	}
}
