package closure

import (
	"fmt"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	ecore "zeroparse.dev/zp/internal/engine/core"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/value"
)

// Core builds the Core IR of the program prog and optimizes it with the closure pipeline (guards
// and scans), for Build. It runs when a parse first needs a family, outside the compilation: the
// build and the passes poll control, the parse's (nil for none), and a cancellation unwinds as
// cancel.Cancellation. The module returned does not keep control. A failure is an internal error:
// the program compiled, so its Core IR builds.
func Core(prog *compile.Program, control *cancel.Control) (*ir.Module, error) {
	m, err := build.CoreControlled(prog, control)
	if err != nil {
		return nil, fmt.Errorf("internal error: Core IR: %w", err)
	}
	if _, err := ir.Run(m, &ir.Env{}, ir.RunOptions{}); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	m.Control = nil
	return m, nil
}

// Build builds the closure backend's form of the program prog from its Core IR m (Core), and
// returns the rules of the family f that the parser runs, by ID. plainDispatch leaves out the
// budget charge of the loop iterations. Build only reads m, so the families of a program can be
// built from the same module, at different times. A failure is an internal error: the program
// compiled, so its terms build. The build runs when a parse first needs the family, outside the
// compilation, so no control interrupts it.
func Build(prog *compile.Program, m *ir.Module, plainDispatch bool, f ecore.Family) ([]*ecore.Rule, error) {
	b, err := newBuilder(prog, m, plainDispatch)
	if err != nil {
		return nil, err
	}
	b.family = f
	if len(b.static) != len(m.Rules) {
		return nil, fmt.Errorf("internal error: %d rules in Core IR, %d in the program", len(m.Rules), len(b.static))
	}
	for _, cr := range m.Rules {
		sr := b.static[cr.ID]
		b.cur = sr
		if o, ok := cr.Body.(*ir.OperatorsRule); ok {
			err = b.operatorsRule(sr, cr, o)
		} else {
			b.scope = scopeOf(cr.Scope)
			err = b.ruleBody(sr, cr, b.node(cr.Body))
		}
		if err == nil {
			err = b.err
		}
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", sr.Name, err)
		}
		r := b.rule(sr.ID)
		*ecore.StaticRuleOf(r) = *sr
		// The frame of the rule's captures has the slots of its Core IR scope.
		ecore.StaticRuleOf(r).Scope = scopeOf(cr.Scope)
		r.InitVals = prog.Vars.ValuesOf(sr.Vars)
		r.VarSlots = prog.Vars.Slots(sr.Vars)
		if sr.NodeType != "" {
			r.NodeKinds = value.NewNodeTypeKinds(sr.NodeType)
		}
		if sr.OpNodeType != "" {
			r.OpKinds = value.NewNodeTypeKinds(sr.OpNodeType)
		}
	}
	return b.rules, nil
}

// newBuilder returns the builder of the closure form of the module m of the program prog.
func newBuilder(prog *compile.Program, m *ir.Module, plainDispatch bool) (*Builder, error) {
	b := &Builder{prog: prog, m: m, plainDispatch: plainDispatch, accepts: map[*ir.Class]func(*ecore.Parser, rune) bool{}}
	b.static = append(append([]*compile.Rule(nil), prog.RuleList...), prog.Twins...)
	ids := make(map[string]ecore.ExpID, len(prog.DescTable))
	for i, d := range prog.DescTable {
		if _, ok := ids[d]; !ok {
			ids[d] = ecore.ExpID(i)
		}
	}
	for _, d := range m.Exps {
		id, ok := ids[d]
		if !ok {
			return nil, fmt.Errorf("internal error: expectation %q is not in the program", d)
		}
		b.descs = append(b.descs, id)
	}
	return b, nil
}

// Matcher returns the matcher of the node n, in the capture scope scope, of the Core IR module m
// of the program prog, in the inlined family. It is for the tests of package engine, which check
// single closures.
func Matcher(prog *compile.Program, m *ir.Module, n ir.Node, scope []ir.Slot, plainDispatch bool) (ecore.Matcher, error) {
	b, err := newBuilder(prog, m, plainDispatch)
	if err != nil {
		return nil, err
	}
	b.cur, b.scope = &compile.Rule{}, scopeOf(scope)
	f := b.node(n)
	return f, b.err
}

// scopeOf returns the capture scope of the Core IR slots s.
func scopeOf(s []ir.Slot) *compile.Scope {
	sc := compile.NewScope()
	for i, x := range s {
		sc.Names = append(sc.Names, x.Name)
		sc.Slots[x.Name] = i
	}
	return sc
}

// desc returns the expectation of the program that the module's expectation id is.
func (b *Builder) desc(id ir.ExpID) ecore.ExpID { return b.descs[id] }

// accept returns the membership test of the class c.
func (b *Builder) accept(c *ir.Class) func(*ecore.Parser, rune) bool {
	if f, ok := b.accepts[c]; ok {
		return f
	}
	cc := &grammar.CharClass{Negated: c.Negated}
	for _, r := range c.Ranges {
		cc.Ranges = append(cc.Ranges, grammar.CharRange{Lo: r.Lo, Hi: r.Hi})
	}
	f := ClassAccept(cc)
	b.accepts[c] = f
	return f
}

// fail records the first internal error of the build.
func (b *Builder) failf(format string, args ...any) ecore.Matcher {
	if b.err == nil {
		b.err = fmt.Errorf("internal error: "+format, args...)
	}
	return b.fail()
}

// node builds the matcher of the Core IR node n in the current scope.
func (b *Builder) node(n ir.Node) ecore.Matcher {
	want := n.Attrs().Want
	switch n := n.(type) {
	case *ir.Lit:
		return b.literal(n.Text, n.Fold, b.desc(n.Exp), want)
	case *ir.ClassNode:
		return single(want, b.desc(n.Class.Exp), b.accept(n.Class))
	case *ir.Any:
		return single(want, compile.IDAny, func(*ecore.Parser, rune) bool { return true })
	case *ir.Call:
		return b.call(n.Rule, n.Level)
	case *ir.Seq:
		ms := make([]ecore.Matcher, len(n.Items))
		flat := make([]bool, len(n.Items))
		valued := make([]bool, len(n.Items))
		for i, it := range n.Items {
			ms[i] = b.node(it)
			sh := it.Attrs().Shape
			flat[i] = sh.Elems
			valued[i] = !flat[i] && sh.K == 1
		}
		return b.seq(ms, flat, valued, n.Attrs().Shape.K, want)
	case *ir.Choice:
		ms := make([]ecore.Matcher, len(n.Alts))
		for i, a := range n.Alts {
			ms[i] = b.node(a.Body)
		}
		return b.choice(n.Alts, ms, want)
	case *ir.Opt:
		return b.optional(n.Body, b.node(n.Body), want)
	case *ir.Repeat:
		return b.repeatNode(n, false)
	case *ir.Stream:
		return b.repeatNode(n.Repeat, true)
	case *ir.Scan:
		return b.scanRepeat(n)
	case *ir.Look:
		m := b.node(n.Body)
		if n.Neg {
			return b.not(m, n.Site)
		}
		return b.and(m, n.Site)
	case *ir.Text:
		return b.atomic(b.node(n.Body), want)
	case *ir.Drop:
		return b.discard(b.node(n.Body))
	case *ir.Capture:
		if n.Project == "" {
			return b.capture(n, b.node(n.Body), want)
		}
		rep, ok := n.Body.(*ir.Repeat)
		if !ok {
			return b.failf("projected capture %s of %T", n.Name, n.Body)
		}
		outer := b.scope
		b.scope = scopeOf(rep.Scope)
		slot, ok := b.scope.Slots[n.Project]
		m := b.node(rep.Body)
		b.scope = outer
		if !ok {
			return b.failf("projected field %s is not captured", n.Project)
		}
		return b.capture(n, b.projectRepeat(rep.Min, rep.Max, m, slot, len(rep.Scope)), want)
	case *ir.Cut:
		return b.cut()
	case *ir.Succeed:
		return b.empty()
	case *ir.FailNode:
		return b.fail()
	case *ir.Anchor:
		switch n.Kind {
		case ir.BeginInput:
			return b.anchor(n.Kind, compile.IDBeginInput)
		case ir.EndInput:
			return b.anchor(n.Kind, compile.IDEndInput)
		case ir.BeginLine:
			return b.anchor(n.Kind, compile.IDBeginLine)
		}
		return b.anchor(n.Kind, compile.IDEndLine)
	case *ir.Pred:
		t := b.termSite(n.Term)
		return func(p *ecore.Parser) (*core.Node, bool) { return nil, predicate(p, t) }
	case *ir.BindNode:
		slot, t := b.prog.Vars.Index(n.Var), b.termSite(n.Term)
		return func(p *ecore.Parser) (*core.Node, bool) { return nil, bind(p, slot, t) }
	case *ir.Label:
		m := b.node(n.Body)
		if n.Diagnostic {
			return diagnosticAttr(m, b.desc(n.Exp))
		}
		msg := ecore.MsgBit | b.desc(n.Exp)
		m = errorAttr(m, msg)
		if n.MissingScope {
			m = ecore.MissingErrorScope(m, msg)
		}
		return m
	case *ir.Recover:
		return b.recover(n.Body, b.node(n.Body), b.node(n.Skip), want && n.Attrs().Shape.K > 0)
	case *ir.Missing:
		return ecore.MissingLiteral(b.node(n.Body), n.Site, n.Lit.Text, want)
	}
	return b.failf("Core IR node %T in a closure program", n)
}

// termSite makes the term program of a predicate or binding.
func (b *Builder) termSite(t ir.Term) *termSite {
	prog, err := b.term(t, nil)
	if err != nil {
		if b.err == nil {
			b.err = err
		}
		return &termSite{}
	}
	return &termSite{term: prog}
}

// repeatNode builds the repetition n; stream reports that it delivers stream elements.
func (b *Builder) repeatNode(n *ir.Repeat, stream bool) ecore.Matcher {
	if n.Site != 0 {
		return b.skipRepeat(n.Min, b.node(n.Body), n.Site)
	}
	outer, elem := b.scope, b.scope
	if n.Scope != nil {
		elem = scopeOf(n.Scope)
	}
	b.scope = elem
	m := b.node(n.Body)
	b.scope = outer
	return b.repeat(n, m, elem, n.Scope != nil, n.Attrs().Want, stream)
}
