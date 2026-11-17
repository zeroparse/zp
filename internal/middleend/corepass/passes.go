package corepass

import "zeroparse.dev/zp/internal/ir/core"

// Passes rewrite Core IR. Each states its precondition; each leaves core.Analyze to be run again,
// after which core.Verify holds if it held before.

// MinTerminalLeaves is the smallest number of literals and classes for which Terminals makes a
// terminal region of an expression with a choice. Smaller ones stay as they are, which costs less
// to run than an interpreter's calls.
const MinTerminalLeaves = 16

// TerminalLeaves returns the number of literals and classes of n, and whether n is a terminal
// form (literals, classes, sequences, choices, options, negative lookaheads, Text and Drop)
// containing a choice. It returns ok false for any other expression.
func TerminalLeaves(n core.Node) (leaves int, choice, ok bool) {
	switch n := n.(type) {
	case *core.Lit:
		if n.Fold {
			// Terminal programs compare exact literals only, so a case-insensitive literal is
			// not a terminal leaf, as in the generator's own count.
			return 0, false, false
		}
		return 1, false, true
	case *core.ClassNode:
		return 1, false, true
	case *core.Seq:
		for _, it := range n.Items {
			k, c, ok := TerminalLeaves(it)
			if !ok {
				return 0, false, false
			}
			leaves, choice = leaves+k, choice || c
		}
		return leaves, choice, true
	case *core.Choice:
		for _, a := range n.Alts {
			k, _, ok := TerminalLeaves(a.Body)
			if !ok {
				return 0, false, false
			}
			leaves += k
		}
		return leaves, true, true
	case *core.Opt:
		return TerminalLeaves(n.Body)
	case *core.Look:
		if n.Neg {
			return TerminalLeaves(n.Body)
		}
	case *core.Drop:
		return TerminalLeaves(n.Body)
	case *core.Text:
		return TerminalLeaves(n.Body)
	}
	return 0, false, false
}

// Terminals makes a Terminal of each outermost value-free expression of the given rules that is a
// terminal form with a choice and at least MinTerminalLeaves literals and classes, and that is in
// a value-free context: a lookahead or a discarded expression where the expression around it
// builds a value is not one, though its body may be. A target runs a Terminal as one program,
// which consumes the same input and records the same expectations as the expression, and charges
// the steps of the expression's entry only. Precondition: Want is set, and the guards of the
// rules' choices are not.
func Terminals(m *core.Module, rule func(*core.Rule) bool) {
	for _, r := range m.Rules {
		m.Control.Poll()
		if r == nil || !rule(r) {
			continue
		}
		r.Body = terminals(r.Body, r.Want)
	}
}

// terminals makes the Terminals of n, whose context builds a value if build.
func terminals(n core.Node, build bool) core.Node {
	if n == nil {
		return nil
	}
	if !build && !n.Attrs().Want {
		if k, choice, ok := TerminalLeaves(n); ok && choice && k >= MinTerminalLeaves {
			return &core.Terminal{Body: n}
		}
	}
	// The context of a child: the value of a lookahead's or a discarded expression's body is
	// never used, and a captured body always builds; otherwise a body builds where it is wanted.
	want := func(x core.Node) bool { return x.Attrs().Want }
	switch x := n.(type) {
	case *core.Seq:
		for i, it := range x.Items {
			x.Items[i] = terminals(it, x.Attrs().Want)
		}
	case *core.Choice:
		for _, a := range x.Alts {
			a.Body = terminals(a.Body, x.Attrs().Want)
		}
	case *core.Opt:
		x.Body = terminals(x.Body, x.Attrs().Want)
	case *core.Repeat:
		x.Body = terminals(x.Body, want(x.Body))
	case *core.Stream:
		// The element of a #stream repetition is the repetition's body, as in any repetition.
		x.Repeat.Body = terminals(x.Repeat.Body, want(x.Repeat.Body))
	case *core.Look:
		x.Body = terminals(x.Body, want(x.Body))
	case *core.Nest:
		x.Body = terminals(x.Body, want(x.Body))
	case *core.Text:
		x.Body = terminals(x.Body, false)
	case *core.Drop:
		x.Body = terminals(x.Body, false)
	case *core.Capture:
		x.Body = terminals(x.Body, true)
	case *core.Label:
		x.Body = terminals(x.Body, x.Attrs().Want)
	case *core.Recover:
		x.Body = terminals(x.Body, want(x.Body))
		x.Skip = terminals(x.Skip, false)
	case *core.OperatorsRule:
		for _, l := range x.Lines() {
			l.Body = terminals(l.Body, want(l.Body))
		}
	}
	return n
}

// Guards sets the guard of every alternative of a choice outside Terminals that has a first
// terminal (Attrs.First) to it: the alternative is skipped when the next character cannot start
// it. With classes, an alternative that is a class, or a sequence whose first item is one, is
// marked GuardedClass: the guard's test of the class is the class's own, so the class only
// consumes the character the guard read. A Terminal's program does its own dispatch. Precondition:
// Analyze has run.
func Guards(m *core.Module, classes bool) {
	for _, r := range m.Rules {
		m.Control.Poll()
		if r != nil {
			guards(r.Body, classes)
		}
	}
}

func guards(n core.Node, classes bool) {
	switch x := n.(type) {
	case *core.Terminal:
		return
	case *core.OperatorsRule:
		for _, l := range x.Lines() {
			guards(l.Body, classes)
		}
		return
	case *core.Choice:
		for _, a := range x.Alts {
			a.Guard, a.GuardedClass = a.Body.Attrs().First, false
			if a.Guard != nil && classes {
				a.GuardedClass = leadingClassNode(a.Body) != nil
			}
		}
	}
	for _, c := range core.Children(n) {
		guards(c, classes)
	}
}

// leadingClassNode returns the class that n is, or that begins the sequence n.
func leadingClassNode(n core.Node) *core.ClassNode {
	if s, ok := n.(*core.Seq); ok && len(s.Items) > 0 {
		n = s.Items[0]
	}
	c, _ := n.(*core.ClassNode)
	return c
}

// Scans replaces every value-free repetition of a class or of any character by a Scan, except a
// skip-run site, which records its walks as any repetition does. A Stream that delivers no elements
// (no EffCommit: its rule is not a start rule) is only its repetition, so it is replaced as that
// repetition is; a Stream that may deliver elements keeps its repetition. A Scan of a scan site keeps
// its number.
func Scans(m *core.Module) {
	for _, r := range m.Rules {
		m.Control.Poll()
		if r == nil {
			continue
		}
		r.Body = core.Rewrite(r.Body, func(n core.Node) core.Node {
			switch x := n.(type) {
			case *core.Repeat:
				if s := scanOf(x); s != nil {
					return s
				}
			case *core.Stream:
				if x.Attrs().Eff.Set&core.EffCommit == 0 {
					if s := scanOf(x.Repeat); s != nil {
						return s
					}
				}
			}
			return n
		})
	}
}

// scanOf returns the Scan that replaces rep, or nil if rep is not a value-free repetition of a class
// or of any character, or is a skip-run site.
func scanOf(rep *core.Repeat) *core.Scan {
	if rep.Attrs().Want || rep.Project != "" || rep.Scope != nil || rep.Site != 0 {
		return nil
	}
	switch body := rep.Body.(type) {
	case *core.ClassNode:
		return scanWith(rep, &core.Scan{Class: body.Class, Min: rep.Min, Max: rep.Max, Site: rep.ScanSite})
	case *core.Any:
		return scanWith(rep, &core.Scan{Min: rep.Min, Max: rep.Max, Site: rep.ScanSite})
	}
	return nil
}

// scanWith gives s the attributes of the repetition rep that it replaces and returns it.
func scanWith(rep *core.Repeat, s *core.Scan) *core.Scan {
	*s.Attrs() = *rep.Attrs()
	return s
}

// ClassRuns replaces every run of two or more consecutive classes in a value-free sequence by a
// ClassRun, except inside Terminals and except the first class of an alternative marked
// GuardedClass, which only consumes the character its guard read. A ClassRun charges no steps on
// entry: the classes charge nothing for their own entries.
func ClassRuns(m *core.Module) {
	for _, r := range m.Rules {
		m.Control.Poll()
		if r == nil {
			continue
		}
		classRuns(r.Body, false)
	}
}

func classRuns(n core.Node, guarded bool) {
	switch x := n.(type) {
	case *core.Terminal:
		return
	case *core.Choice:
		for _, a := range x.Alts {
			classRuns(a.Body, a.GuardedClass)
		}
		return
	case *core.OperatorsRule:
		for _, l := range x.Lines() {
			classRuns(l.Body, false)
		}
		return
	case *core.Seq:
		for _, it := range x.Items {
			classRuns(it, false)
		}
		if x.Attrs().Want {
			return
		}
		var items []core.Node
		for i := 0; i < len(x.Items); i++ {
			run := 0
			for i+run < len(x.Items) {
				if _, ok := x.Items[i+run].(*core.ClassNode); !ok {
					break
				}
				run++
			}
			if run < 2 || i == 0 && guarded {
				items = append(items, x.Items[i])
				continue
			}
			cr := &core.ClassRun{}
			for _, it := range x.Items[i : i+run] {
				cr.Classes = append(cr.Classes, it.(*core.ClassNode).Class)
			}
			items = append(items, cr)
			i += run - 1
		}
		x.Items = items
		return
	}
	for _, c := range core.Children(n) {
		classRuns(c, false)
	}
}
