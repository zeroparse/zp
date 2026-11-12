package engine

import (
	"testing"

	"zeroparse.dev/zp/internal/gen/gogen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/coreir"
	irbuild "zeroparse.dev/zp/internal/ir/coreir/build"
)

// coreBreak is a negative control: a pass broken on purpose, which the comparison with the frozen
// decisions must catch in every module where it changes something.
type coreBreak struct {
	name string
	// pass is the pass after which the module is broken.
	pass string
	// aspects are the aspects of the differences it causes (compareCoreDecisions).
	aspects []string
	// apply breaks the module and reports whether it changed something.
	apply func(c *coreGenCase, m *coreir.Module) bool
}

// checkCoreAspect compares the aspect of the Core IR with the frozen decisions over the corpus in
// every mode, and runs the negative controls in the lean mode.
func checkCoreAspect(t *testing.T, aspect string, breaks []coreBreak) {
	t.Helper()
	for _, mode := range coreGenModes {
		var n coreDecisionCounts
		for _, c := range coreGenCases(t, mode, nil) {
			for _, err := range coreAspectErrors(compareCoreDecisions(c, &n), aspect) {
				t.Errorf("%s: %s: %s", mode.name, c.name, err)
			}
		}
	}
	for _, b := range breaks {
		broken, caught := 0, 0
		cases := coreGenCases(t, coreGenModes[0], func(c *coreGenCase, pass string, m *coreir.Module) {
			if pass == b.pass && b.apply(c, m) {
				c.broken = true
			}
		})
		for _, c := range cases {
			if !c.broken {
				continue
			}
			broken++
			errs := compareCoreDecisions(c, &coreDecisionCounts{})
			found := false
			for _, a := range b.aspects {
				found = found || len(coreAspectErrors(errs, a)) > 0
			}
			if found {
				caught++
			} else {
				t.Errorf("%s: %s: not caught", b.name, c.name)
			}
		}
		if broken == 0 {
			t.Errorf("%s: no module to break", b.name)
		}
		t.Logf("%s: %d of %d broken modules caught", b.name, caught, broken)
	}
}

// firstNode returns the first node of the module for which f reports true, with an accessor of
// its place (eachSlot).
func firstNode(m *coreir.Module, f func(coreir.Node) bool) (coreir.Node, func(coreir.Node)) {
	for _, r := range m.Rules {
		var found coreir.Node
		var set func(coreir.Node)
		irbuild.EachSlot(r, func(at func() coreir.Node) {
			if found == nil && f(at()) {
				found = at()
				set = placeSetter(r, at)
			}
		})
		if found != nil {
			return found, set
		}
	}
	return nil, nil
}

// placeSetter returns a function that replaces the node at the place at of the rule r.
func placeSetter(r *coreir.Rule, at func() coreir.Node) func(coreir.Node) {
	old := at()
	return func(n coreir.Node) {
		replace := func(x coreir.Node) coreir.Node {
			if x == old {
				return n
			}
			return x
		}
		if r.Body == old {
			r.Body = n
			return
		}
		irbuild.EachSlot(r, func(p func() coreir.Node) {
			switch x := p().(type) {
			case *coreir.Seq:
				for i := range x.Items {
					x.Items[i] = replace(x.Items[i])
				}
			case *coreir.Choice:
				for _, a := range x.Alts {
					a.Body = replace(a.Body)
				}
			case *coreir.Opt:
				x.Body = replace(x.Body)
			case *coreir.Repeat:
				x.Body = replace(x.Body)
			case *coreir.Look:
				x.Body = replace(x.Body)
			case *coreir.Nest:
				x.Body = replace(x.Body)
			case *coreir.Text:
				x.Body = replace(x.Body)
			case *coreir.Drop:
				x.Body = replace(x.Body)
			case *coreir.Capture:
				x.Body = replace(x.Body)
			case *coreir.Label:
				x.Body = replace(x.Body)
			case *coreir.Recover:
				x.Body, x.Skip = replace(x.Body), replace(x.Skip)
			case *coreir.Terminal:
				x.Body = replace(x.Body)
			case *coreir.OperatorsRule:
				for _, l := range x.Lines() {
					l.Body = replace(l.Body)
				}
			}
		})
	}
}

// firstAlt returns the first alternative of a choice of the module that has an origin and for which f
// reports true.
func firstAlt(c *coreGenCase, m *coreir.Module, f func(*coreir.Alt) bool) *coreir.Alt {
	var found *coreir.Alt
	for _, r := range m.Rules {
		coreir.Walk(r.Body, func(n coreir.Node) {
			if ch, ok := n.(*coreir.Choice); ok && found == nil {
				for i, a := range ch.Alts {
					_, ok := c.altOrigin(ch, i)
					if found == nil && ok && f(a) {
						found = a
					}
				}
			}
		})
	}
	return found
}

// TestCoreGuardsFrozen compares the guards and guarded classes of the Core passes with the
// frozen ones, and checks that a guard pass that drops a guard or a guarded class is caught.
func TestCoreGuardsFrozen(t *testing.T) {
	checkCoreAspect(t, "guard", []coreBreak{
		{"guard dropped", "guards", []string{"guard"}, func(c *coreGenCase, m *coreir.Module) bool {
			a := firstAlt(c, m, func(a *coreir.Alt) bool { return a.Guard != nil && !a.GuardedClass })
			if a == nil {
				return false
			}
			a.Guard = nil
			return true
		}},
		{"wrong guard expectation", "guards", []string{"guard"}, func(c *coreGenCase, m *coreir.Module) bool {
			// The literal that begins the alternative, and so its guard, record another
			// expectation: the module stays valid, with a guard of the wrong expectation.
			var lit *coreir.Lit
			a := firstAlt(c, m, func(a *coreir.Alt) bool {
				lit = nil
				if a.Guard != nil && a.Guard.Lit && a.Guard.Depth == 0 {
					lit = leadingLit(a.Body)
				}
				return lit != nil
			})
			if a == nil {
				return false
			}
			g := *a.Guard
			lit.Exp, g.Exp = 0, 0 // any character, which no literal records
			a.Guard = &g
			return true
		}},
		{"guarded class dropped", "guards", []string{"guard"}, func(c *coreGenCase, m *coreir.Module) bool {
			a := firstAlt(c, m, func(a *coreir.Alt) bool { return a.GuardedClass })
			if a == nil {
				return false
			}
			a.GuardedClass = false
			return true
		}},
	})
}

// TestCoreTerminalsFrozen compares the Terminals of the Core passes with the frozen ones,
// and checks that a terminal pass that leaves one out is
// caught.
func TestCoreTerminalsFrozen(t *testing.T) {
	checkCoreAspect(t, "terminal", []coreBreak{
		{"terminal left out", "terminals", []string{"terminal"}, func(c *coreGenCase, m *coreir.Module) bool {
			n, set := firstNode(m, func(n coreir.Node) bool { _, ok := n.(*coreir.Terminal); return ok })
			if n == nil {
				return false
			}
			set(n.(*coreir.Terminal).Body)
			return true
		}},
		{"spurious terminal", "terminals", []string{"terminal"}, func(c *coreGenCase, m *coreir.Module) bool {
			for _, r := range m.Rules {
				if !gogen.CoreLocalRule(r) {
					continue
				}
				if seq := valueFreeSeq(r.Body, func(n coreir.Node) bool {
					_, ok := c.origin[n]
					return ok
				}); seq != nil {
					seq.Items[0] = &coreir.Terminal{Body: seq.Items[0]}
					return true
				}
			}
			return false
		}},
	})
}

// leadingLit returns the literal that n is or that begins the sequence n.
func leadingLit(n coreir.Node) *coreir.Lit {
	if s, ok := n.(*coreir.Seq); ok && len(s.Items) > 0 {
		n = s.Items[0]
	}
	l, _ := n.(*coreir.Lit)
	return l
}

// valueFreeSeq returns the first value-free sequence of n outside Terminals that begins with a
// literal for which written reports true.
func valueFreeSeq(n coreir.Node, written func(coreir.Node) bool) *coreir.Seq {
	if _, ok := n.(*coreir.Terminal); ok {
		return nil
	}
	if s, ok := n.(*coreir.Seq); ok && !s.Attrs().Want && len(s.Items) > 0 {
		if _, ok := s.Items[0].(*coreir.Lit); ok && written(s.Items[0]) {
			return s
		}
	}
	for _, c := range coreir.Children(n) {
		if s := valueFreeSeq(c, written); s != nil {
			return s
		}
	}
	return nil
}

// TestCoreClassRunsFrozen compares the class runs of the Core passes with the frozen ones,
// and checks that a class-run pass that leaves one out is caught.
func TestCoreClassRunsFrozen(t *testing.T) {
	checkCoreAspect(t, "class runs", []coreBreak{
		{"class run left out", "class-runs", []string{"class runs"}, func(c *coreGenCase, m *coreir.Module) bool {
			var seq *coreir.Seq
			at := -1
			for _, r := range m.Rules {
				coreir.Walk(r.Body, func(n coreir.Node) {
					if s, ok := n.(*coreir.Seq); ok && seq == nil {
						for i, it := range s.Items {
							if _, ok := it.(*coreir.ClassRun); ok && at < 0 {
								seq, at = s, i
							}
						}
					}
				})
			}
			if seq == nil {
				return false
			}
			var items []coreir.Node
			for _, cl := range seq.Items[at].(*coreir.ClassRun).Classes {
				items = append(items, &coreir.ClassNode{Class: cl})
			}
			seq.Items = append(seq.Items[:at:at], append(items, seq.Items[at+1:]...)...)
			return true
		}},
	})
}

// TestCoreScansFrozen compares the scans of the Core passes with the frozen ones, and
// checks that a scan pass that leaves one out is caught.
func TestCoreScansFrozen(t *testing.T) {
	checkCoreAspect(t, "scan", []coreBreak{
		{"scan left out", "scans", []string{"scan"}, func(c *coreGenCase, m *coreir.Module) bool {
			n, set := firstNode(m, func(n coreir.Node) bool { _, ok := n.(*coreir.Scan); return ok })
			if n == nil {
				return false
			}
			s := n.(*coreir.Scan)
			var body coreir.Node = &coreir.Any{}
			if s.Class != nil {
				body = &coreir.ClassNode{Class: s.Class}
			}
			rep := &coreir.Repeat{Min: s.Min, Max: s.Max, Body: body}
			c.origin[rep] = c.origin[s]
			set(rep)
			return true
		}},
	})
}

// TestCoreScopesFrozen compares the scopes of repetition elements and Operators Rule lines
// with the frozen ones, and checks that a scope with a slot too many is caught.
func TestCoreScopesFrozen(t *testing.T) {
	checkCoreAspect(t, "scope", []coreBreak{
		{"element slot added", "build", []string{"scope"}, func(c *coreGenCase, m *coreir.Module) bool {
			n, _ := firstNode(m, func(n coreir.Node) bool { r, ok := n.(*coreir.Repeat); return ok && r.Scope != nil })
			if n == nil {
				return false
			}
			r := n.(*coreir.Repeat)
			r.Scope = append(r.Scope, coreir.Slot{Name: "extra", Type: "extra"})
			return true
		}},
		{"line slot added", "build", []string{"scope"}, func(c *coreGenCase, m *coreir.Module) bool {
			n, _ := firstNode(m, func(n coreir.Node) bool { _, ok := n.(*coreir.OperatorsRule); return ok })
			if n == nil {
				return false
			}
			l := n.(*coreir.OperatorsRule).Lines()[0]
			l.Scope = append(l.Scope, coreir.Slot{Name: "extra", Type: "extra"})
			return true
		}},
	})
}

func TestCoreScans(t *testing.T) {
	t.Parallel()
	src := `rule main = [a-z]* "-"#repeat(2) .#repeat(1, 3) x:[0-9]* $$ => $x
`
	prog, ok := coreCorpusProgram(t, gentest.Case{Src: src})
	if !ok {
		t.Fatal("does not compile")
	}
	g := &gogen.Generator{Prog: prog.Static(), Table: "trules"}
	m, err := g.TypedCore(nil)
	if err != nil {
		t.Fatal(err)
	}
	got := coreir.PrintNode(m, m.Rules[0].Body, false)
	want := `seq
  scan [a-z] 0 -1 scan-site=1
  repeat 2 2
    lit "-"
  scan any 1 3
  capture 0 x
    repeat 0 -1 scan-site=3
      class [0-9]
  anchor end-input
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
