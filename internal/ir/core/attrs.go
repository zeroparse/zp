package core

import "unicode/utf8"

// Analyze computes Nullable, NullableIns, First and Eff of every node of the module from the
// rules' bodies. Want and Shape are left as built. The rule references of the module must be valid.
//
// Nullable, NullableIns and the effects of calls are the least fixed point over the rules. First follows the
// first item of sequences, the bodies of captures, Text, Drop, Nest and Terminal, the first class
// of a class run and calls of rules that are neither left-recursion leaders nor Operators Rules,
// counting one level for each call of a recursive rule on the way (a Nest is of a rule that is not
// recursive).
func Analyze(m *Module) {
	a := &analyzer{m: m, null: make([]bool, len(m.Rules)), ins: make([]bool, len(m.Rules)), eff: make([]Effect, len(m.Rules)),
		start: make([]bool, len(m.Rules))}
	for _, s := range m.Starts {
		a.start[s] = true
	}
	for changed := true; changed; {
		changed = false
		for _, r := range m.Rules {
			if r == nil {
				continue
			}
			a.cur = r.ID
			n, in, e := a.node(r.Body)
			if r.Finish.Kind == FinishAction && Raises(r.Finish.Action) {
				e.Set |= EffRaise
			}
			s := e.Set &^ (EffCapture | EffBind | EffCut)
			if n != a.null[r.ID] || in != a.ins[r.ID] || s != a.eff[r.ID] {
				a.null[r.ID], a.ins[r.ID], a.eff[r.ID] = n, in, s
				changed = true
			}
		}
	}
	a.first = make([]*First, len(m.Rules))
	a.state = make([]uint8, len(m.Rules))
	for _, r := range m.Rules {
		if r != nil {
			a.firsts(r.Body)
		}
	}
}

type analyzer struct {
	m     *Module
	null  []bool
	ins   []bool
	eff   []Effect
	first []*First
	// state is 0 for a rule whose First is not computed, 1 while it is computed, 2 after.
	state []uint8
	// start holds the start rules, and cur is the rule whose body is analyzed.
	start []bool
	cur   int
}

// node sets the Nullable, NullableIns and Eff attributes of n and its subexpressions and returns
// them. Only Missing makes the two nullabilities differ.
func (a *analyzer) node(n Node) (bool, bool, Effects) {
	if n == nil {
		return true, true, Effects{}
	}
	var null, ins bool
	var e Effects
	const local = EffCapture | EffBind | EffCut
	switch n := n.(type) {
	case *Lit:
		null = n.Text == ""
		ins = null
		if !null {
			e.Set = EffConsume | EffFail | EffExpect
		}
	case *ClassNode, *Any:
		e.Set = EffConsume | EffFail | EffExpect
	case *Anchor:
		null, ins = true, true
		e.Set = EffFail | EffExpect
	case *Seq:
		null, ins = true, true
		for _, it := range n.Items {
			nn, ni, ee := a.node(it)
			null, ins = null && nn, ins && ni
			e = union(e, ee)
		}
	case *Choice:
		for _, alt := range n.Alts {
			nn, ni, ee := a.node(alt.Body)
			null, ins = null || nn, ins || ni
			e = union(e, ee)
		}
		e.Set &^= EffCut
	case *Opt:
		_, _, ee := a.node(n.Body)
		null, ins = true, true
		e = ee
		// An option fails only when its body fails after a cut that commits it.
		if ee.Set&EffCut == 0 {
			e.Set &^= EffFail
		}
		e.Set &^= EffCut
	case *Repeat:
		nn, ni, ee := a.node(n.Body)
		null = n.Min == 0 || n.Min == 1 && nn
		ins = n.Min == 0 || n.Min == 1 && ni
		e = ee
		if n.Scope != nil {
			// The element's captures are its own; its bindings are the rule's, as everywhere
			// outside a rule call.
			e = Effects{Set: e.Set &^ EffCapture}
		}
		// A repetition without a minimum fails only when an iteration fails after a cut that
		// commits it.
		if n.Min == 0 && ee.Set&EffCut == 0 {
			e.Set &^= EffFail
		}
		e.Set &^= EffCut
	case *Look:
		_, _, ee := a.node(n.Body)
		null, ins = true, true
		e = ee
		// The end of a lookahead discards the errors recovered inside it.
		e.Set &^= EffConsume | EffExpect | EffCut | EffRecover
		e.Set |= EffFail
	case *Cut:
		null, ins = true, true
		e.Set = EffCut
	case *Succeed:
		null, ins = true, true
	case *FailNode:
		e.Set = EffFail
	case *Call:
		// A call of a recursive rule can exceed the nesting limit.
		null, ins = a.null[n.Rule], a.ins[n.Rule]
		e.Set = a.eff[n.Rule] | EffCall
		if a.m.Rules[n.Rule].Policy.Rec {
			e.Set |= EffRaise
		}
	case *Nest:
		nn, ni, ee := a.node(n.Body)
		null, ins = nn, ni
		e.Set = ee.Set&^local | EffCall
	case *OperatorsRule:
		for _, l := range n.Lines() {
			_, _, ee := a.node(l.Body)
			e.Set |= ee.Set &^ local
			if l.Action != nil && Raises(l.Action) {
				e.Set |= EffRaise
			}
		}
		for _, l := range n.Operands {
			at := l.Body.Attrs()
			null, ins = null || at.Nullable, ins || at.NullableIns
		}
		e.Set |= EffConsume | EffFail | EffExpect | EffCall | EffRaise
	case *Text:
		null, ins, e = a.node(n.Body)
	case *Drop:
		null, ins, e = a.node(n.Body)
	case *Capture:
		null, ins, e = a.node(n.Body)
		e = union(Effects{Set: EffCapture, Slots: []int{n.Slot}}, e)
	case *Pred:
		null, ins = true, true
		e.Set = EffFail
		if Raises(n.Term) {
			e.Set |= EffRaise
		}
	case *BindNode:
		null, ins = true, true
		e.Set = EffBind
		if Raises(n.Term) {
			e.Set |= EffRaise
		}
	case *Label:
		null, ins, e = a.node(n.Body)
	case *Recover:
		// A recovery that skips nothing fails, so it succeeds without consuming input only as its
		// body does.
		var eb, es Effects
		null, ins, eb = a.node(n.Body)
		nullSkip, _, es := a.node(n.Skip)
		e = union(eb, es)
		// A recovery fails when its body fails and its skip fails or consumes nothing.
		if eb.Set&EffFail == 0 || es.Set&EffFail == 0 && !nullSkip {
			e.Set &^= EffFail
		}
		e.Set |= EffRecover
	case *Missing:
		// An insertion consumes no input; without insertion the literal must match, and with it the
		// literal fails where it was already inserted at the same position. An insertion can exceed
		// the limit of missing tokens.
		null, _, _ = a.node(n.Lit)
		ins = true
		e.Set = EffConsume | EffFail | EffExpect | EffRecover | EffRaise
	case *Stream:
		null, ins, e = a.node(n.Repeat)
		// Only the invocation of a start rule that starts a stream parse delivers elements.
		if a.start[a.cur] {
			e.Set |= EffCommit
		}
	case *Scan:
		// A scan records the expectation of the character where it stops.
		null = n.Min == 0
		ins = null
		e.Set = EffConsume | EffExpect
		if n.Min > 0 {
			e.Set |= EffFail
		}
	case *ClassRun:
		e.Set = EffConsume | EffFail | EffExpect
	case *Terminal:
		null, ins, e = a.node(n.Body)
	}
	at := n.Attrs()
	at.Nullable, at.NullableIns, at.Eff = null, ins, e
	return null, ins, e
}

// union returns the union of two effect summaries, with the slots of x first.
func union(x, y Effects) Effects {
	s := Effects{Set: x.Set | y.Set, Slots: x.Slots}
	for _, v := range y.Slots {
		if !hasSlot(s.Slots, v) {
			s.Slots = append(s.Slots[:len(s.Slots):len(s.Slots)], v)
		}
	}
	return s
}

func hasSlot(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// firsts sets the First attribute of n and its subexpressions.
func (a *analyzer) firsts(n Node) {
	if n == nil {
		return
	}
	if o, ok := n.(*OperatorsRule); ok {
		for _, l := range o.Lines() {
			a.firsts(l.Body)
		}
	}
	for _, c := range Children(n) {
		a.firsts(c)
	}
	n.Attrs().First = a.head(n)
}

// head returns the First of n, following only the expressions n must begin with, so that a rule
// is entered only where its call can be the first thing a body matches.
func (a *analyzer) head(n Node) *First {
	switch n := n.(type) {
	case *Lit:
		if n.Text != "" {
			r, _ := utf8.DecodeRuneInString(n.Text)
			if n.Fold && ('A' <= r && r <= 'Z' || 'a' <= r && r <= 'z') {
				// Only ASCII letters fold: the first character is either case of r.
				return &First{Lit: true, Fold: true, Rune: r | 0x20, Exp: n.Exp}
			}
			return &First{Lit: true, Rune: r, Exp: n.Exp}
		}
	case *ClassNode:
		return &First{Class: n.Class, Exp: n.Class.Exp}
	case *Seq:
		if len(n.Items) > 0 {
			return a.head(n.Items[0])
		}
	case *Capture:
		return a.head(n.Body)
	case *Text:
		return a.head(n.Body)
	case *Drop:
		return a.head(n.Body)
	case *Call:
		f := a.ruleFirst(n.Rule)
		if a.m.Rules[n.Rule].Policy.Rec {
			f = deeper(f)
		}
		return f
	case *Nest:
		return a.head(n.Body)
	case *Terminal:
		return a.head(n.Body)
	case *ClassRun:
		return &First{Class: n.Classes[0], Exp: n.Classes[0].Exp}
	}
	return nil
}

// ruleFirst returns the First of a rule's body, nil for a leader, an Operators Rule or a rule
// whose First is being computed.
func (a *analyzer) ruleFirst(id int) *First {
	r := a.m.Rules[id]
	if r.Policy.Leader {
		return nil
	}
	if _, ok := r.Body.(*OperatorsRule); ok {
		return nil
	}
	switch a.state[id] {
	case 1:
		return nil
	case 2:
		return a.first[id]
	}
	a.state[id] = 1
	a.first[id] = a.head(r.Body)
	a.state[id] = 2
	return a.first[id]
}

func deeper(f *First) *First {
	if f == nil {
		return nil
	}
	g := *f
	g.Depth++
	return &g
}
