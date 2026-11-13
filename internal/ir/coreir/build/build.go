// Package build builds Core IR (internal/ir/coreir) from a compiled program.
//
// Core makes the module of the program variant whose rule bodies build values where the
// compiled rules do: a rule body builds its value unless the rule is lean (its action reads only
// captures, or it is a value-free twin); a capture, and the sequence, choice, optional and
// repetition around a built expression, build theirs; text (#atomic), drop (-), negative
// lookaheads and recovery skips build none; a positive lookahead builds only when it contains
// captures; the element of a projected repetition builds none. A call where no value is wanted
// calls the rule's value-free twin, if it has one.
//
// Capture slots follow the frame layouts of the compiled rules: the scope of a rule body, a
// repetition element with captures of its own (elementScoped) or an Operators Rule line lists
// every capture written in it, including unreachable ones, in the order they are first met
// (coreLayout).
//
// A type-checked program gives the module its checked types (check.ValueRecord): those of the rules'
// values, of the capture slots, of every term and of the named types and variables. The
// expectations of the module begin with the fixed ones of the runtime (coreir.FixedDescs), so that Any
// and the anchors, which name none, record those of the same index. A #stream repetition at the
// top of a rule body is a Stream if it is the first #stream of the body, in every rule, whether
// or not it starts a parse; elsewhere it is built as its repetition. A body the compiler accepts
// has at most one #stream; for one it rejects, the module has a Stream exactly where the compiler
// checks a #stream against left recursion, so that the facts report that error where the
// compiler does.
package build

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/check"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/grammaranalysis"
	"zeroparse.dev/zp/internal/ir/coreir"
	"zeroparse.dev/zp/internal/types"
	"zeroparse.dev/zp/internal/value"
)

// CoreFrom builds the Core IR of the program: its rules and then its value-free twins, each at
// the position of its ID. Shapes are those of the value model (internal/compile/value_shape.go);
// Analyze has not run. If origin is not nil, it records the expression each node is built from
// (the innermost one, for nodes built from a chain of expressions).
func CoreFrom(prog Program, origin map[coreir.Node]grammar.Expr) (*coreir.Module, error) {
	return CoreProjecting(prog, origin, true)
}

// CoreProjecting is CoreFrom; if project is false, no repetition gathers the values of a
// capture (Rule.Projections is ignored), the shape of the values the recognizer's table
// writes its actions for (the Go generator's projections).
func CoreProjecting(prog Program, origin map[coreir.Node]grammar.Expr, project bool) (*coreir.Module, error) {
	return coreModule(prog, origin, project)
}

// coreModule builds the module of prog. A program without a checker (see Structural) builds an
// untyped module.
func coreModule(prog Program, origin map[coreir.Node]grammar.Expr, project bool) (*coreir.Module, error) {
	in := prog.CoreInput()
	all := in.Rules
	b := &coreBuilder{prog: prog, in: in, chk: prog.Checker(), m: &coreir.Module{Rules: make([]*coreir.Rule, len(all))}, origin: origin,
		exps: map[string]coreir.ExpID{}, classes: map[string]*coreir.Class{}}
	b.noProject = !project
	if b.chk != nil {
		b.rec = b.chk.ValueRecord()
	}
	for _, d := range coreir.FixedDescs {
		b.exp(d)
	}
	for _, r := range in.Starts {
		b.m.Starts = append(b.m.Starts, r.ID)
	}
	for i, r := range all {
		if r.ID != i {
			return nil, fmt.Errorf("internal error: rule %s has ID %d at position %d", r.Name, r.ID, i)
		}
		cr, err := b.rule(r)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", r.Name, err)
		}
		b.m.Rules[i] = cr
	}
	for _, r := range all {
		if r.Twin != nil {
			b.m.Rules[r.ID].Twin = b.m.Rules[r.Twin.ID]
		}
	}
	if b.rec != nil {
		b.m.Typed = true
		b.m.Types = map[string]coreir.Type{}
		k := &check.TypeResolver{Prog: b.chk}
		for name := range in.Types {
			b.m.Types[name] = k.ResolveNamed(name, grammar.Pos{})
		}
	}
	if in.Vars != nil {
		for _, d := range in.Vars.List {
			b.m.Vars = append(b.m.Vars, &coreir.VarDecl{Name: d.Name, Type: d.Typ, Init: d.Init})
		}
	}
	coreir.CallGraph(b.m)
	return b.m, nil
}

type coreBuilder struct {
	prog Program
	in   *Input
	chk  *check.Checker
	m    *coreir.Module
	cur  *Rule
	// noProject builds no projected repetition: proj stays empty.
	noProject bool
	// proj holds the projected repetition captures of the rule being built (ruleProjections).
	proj map[string]string
	// final is the action whose value is the result of the rule or line being built.
	final grammar.Term
	// origin, if not nil, records the expression each node is built from.
	origin map[coreir.Node]grammar.Expr
	// rec holds the checked types of the program, nil if it has none.
	rec *check.ValueRecord
	// exps and classes intern the expectations and classes of the module.
	exps    map[string]coreir.ExpID
	classes map[string]*coreir.Class
	// stream is the #stream of the rule being built that makes a Stream: its first #stream in
	// walk order, if that one is at the top of the body (check.StreamTops), else nil.
	stream grammar.Expr
	// looks is the number of lookaheads built (coreir.Look.Site).
	looks int
}

// callLevel returns the minimum binding level of a call of the rule r (coreir.Call.Level).
func callLevel(e *grammar.Ref, r *Rule) (int, error) {
	if e.Level == "" {
		return 0, nil
	}
	o, ok := r.Def.Expr.(*grammar.OperatorsRule)
	if !ok {
		return 0, fmt.Errorf("rule %s has no Operators Rule levels", e.Name)
	}
	for i, l := range o.Levels {
		if l.Name == e.Level {
			return i, nil
		}
	}
	return 0, fmt.Errorf("rule %s has no level %s", e.Name, e.Level)
}

// exp returns the ID of the expectation d, interning it.
func (b *coreBuilder) exp(d string) coreir.ExpID {
	id, ok := b.exps[d]
	if !ok {
		id = coreir.ExpID(len(b.m.Exps))
		b.exps[d] = id
		b.m.Exps = append(b.m.Exps, d)
	}
	return id
}

// class returns the class of a character class expression, interning it.
func (b *coreBuilder) class(e *grammar.CharClass) *coreir.Class {
	d := coreir.CharClassString(e)
	key := fmt.Sprint(e.Negated, e.Ranges, d)
	if c, ok := b.classes[key]; ok {
		return c
	}
	c := &coreir.Class{Negated: e.Negated, Exp: b.exp(d)}
	for _, r := range e.Ranges {
		c.Ranges = append(c.Ranges, coreir.Range{Lo: r.Lo, Hi: r.Hi})
	}
	b.classes[key] = c
	b.m.Classes = append(b.m.Classes, c)
	return c
}

// coreScope is a capture scope being built: the names of its slots and the checked types of the
// captures of its body.
type coreScope struct {
	names []string
	types map[string]types.Ty
}

// scope returns a scope with the slots names for the scope body e.
func (b *coreBuilder) scope(names []string, e grammar.Expr) *coreScope {
	s := &coreScope{names: names}
	if b.rec != nil {
		s.types = b.rec.Scopes[e]
	}
	return s
}

// slots returns the slots of the scope s, each with the checked type of its capture.
func (s *coreScope) slots() []coreir.Slot {
	if s.names == nil {
		return nil
	}
	out := make([]coreir.Slot, len(s.names))
	for i, n := range s.names {
		out[i] = coreir.Slot{Name: n}
		if t, ok := s.types[n]; ok {
			out[i].Type = t
		}
	}
	return out
}

func (s *coreScope) slot(name string) int {
	for i, n := range s.names {
		if n == name {
			return i
		}
	}
	s.names = append(s.names, name)
	return len(s.names) - 1
}

func (s *coreScope) lookup(name string) (int, bool) {
	for i, n := range s.names {
		if n == name {
			return i, true
		}
	}
	return 0, false
}

func (b *coreBuilder) rule(r *Rule) (*coreir.Rule, error) {
	b.cur = r
	cr := &coreir.Rule{ID: r.ID, Name: r.Name, Where: r.WhereName(), Want: !r.Lean}
	if b.rec != nil && !r.Novalue {
		if t, ok := b.rec.Rules[r.Def.Name]; ok {
			cr.Type = t
		}
	}
	cr.Policy = coreir.Policy{Memo: r.Memo, Transient: r.Transient, Plain: r.Plain, Rec: r.Rec, Leader: r.Leader, Splice: r.Splice,
		NoValue: r.Novalue, Seen: r.Seen, Vars: r.Vars}
	if o, ok := r.Def.Expr.(*grammar.OperatorsRule); ok {
		b.proj = nil
		body, err := b.operatorsRule(o)
		if err != nil {
			return nil, err
		}
		cr.Body = body
		cr.Finish = coreir.Finish{Kind: coreir.FinishSplice}
		if r.Action != nil && !r.Novalue {
			// The whole-rule action runs once per call on the folded value, $value.
			env := map[string]*coreir.Local{check.OperatorsRuleValue: {Kind: coreir.LocalValue, Name: check.OperatorsRuleValue}}
			b.final = r.Action
			cr.Finish.Action, err = b.term(r.Action, b.scope(nil, o), env)
			b.final = nil
			if err != nil {
				return nil, err
			}
			cr.Finish.Kind, cr.Finish.NodeType = coreir.FinishAction, r.NodeType
		}
		return cr, nil
	}
	if !b.noProject {
		b.proj = r.Projections()
	}
	defer func() { b.proj = nil }()
	body := r.Body
	b.stream = firstStream(body)
	s := b.scope(coreLayout(r, b.proj, body, !r.Lean).names, body)
	n, err := b.expr(body, s, !r.Lean)
	if err != nil {
		return nil, err
	}
	cr.Body = n
	cr.Finish.NodeType = r.NodeType
	cr.Finish.Shape = coreShape(b.chk, body)
	switch {
	case r.Novalue:
		cr.Finish.Kind = coreir.FinishNone
		cr.Finish.NodeType = ""
	case r.Action != nil:
		cr.Finish.Kind = coreir.FinishAction
		b.final = r.Action
		cr.Finish.Action, err = b.term(r.Action, s, nil)
		b.final = nil
		if err != nil {
			return nil, err
		}
	case r.Splice:
		cr.Finish.Kind = coreir.FinishSplice
	default:
		cr.Finish.Kind = coreir.FinishScope
	}
	cr.Scope = s.slots()
	return cr, nil
}

func (b *coreBuilder) operatorsRule(e *grammar.OperatorsRule) (*coreir.OperatorsRule, error) {
	o := &coreir.OperatorsRule{}
	var err error
	if e.Skip != nil {
		if o.Skip, err = b.opLine(e.Skip, nil, false, true); err != nil {
			return nil, err
		}
	}
	for _, x := range e.Operands {
		l, err := b.opLine(x.Expr, x.Action, false, false)
		if err != nil {
			return nil, err
		}
		o.Operands = append(o.Operands, l)
	}
	id := 0
	for i, level := range e.Levels {
		for _, x := range level.Operators {
			l, err := b.opLine(x.Expr, x.Action, true, false)
			if err != nil {
				return nil, err
			}
			o.Ops = append(o.Ops, &coreir.Operator{ID: id, Kind: x.Kind, Assoc: x.Assoc, Level: i + 1, Line: l})
			id++
		}
	}
	return o, nil
}

// opLine builds an Operators Rule line, the skip line if skip. Lines whose value is unused are
// matched without values, as are the skip line and all lines of a value-free rule.
func (b *coreBuilder) opLine(e grammar.Expr, action grammar.Term, operator, skip bool) (*coreir.OpLine, error) {
	var locals []string
	if operator {
		locals = []string{"lhs", "rhs", "op"}
	}
	build := !skip && !b.cur.Novalue && !check.LeanLine(action, locals)
	s := b.scope(coreLayout(b.cur, nil, e, build).names, e)
	body, err := b.expr(e, s, build)
	if err != nil {
		return nil, err
	}
	l := &coreir.OpLine{Body: body, Shape: coreShape(b.chk, e)}
	if action != nil && !b.cur.Novalue {
		env := map[string]*coreir.Local{}
		if operator {
			env["lhs"] = &coreir.Local{Kind: coreir.LocalLhs, Name: "lhs"}
			env["rhs"] = &coreir.Local{Kind: coreir.LocalRhs, Name: "rhs"}
			env["op"] = &coreir.Local{Kind: coreir.LocalOp, Name: "op"}
		}
		b.final = action
		l.Action, err = b.term(action, s, env)
		b.final = nil
		if err != nil {
			return nil, err
		}
	}
	l.Scope = s.slots()
	return l, nil
}

// coreShape returns the static shape of the value of e.
func coreShape(chk *check.Checker, e grammar.Expr) coreir.Shape {
	if chk == nil {
		return coreir.Shape{} // an untyped module has no shapes
	}
	elems := check.IsElems(chk, e)
	return coreir.Shape{Elems: elems, K: check.ValuedCount(chk, e), Mode: coreir.SingleMode(check.SingleModeOf(chk, e)), Tuple: check.ScopeTuple(chk, e),
		Shared: elems && check.SharesTuple(chk, e)}
}

// coreLayout returns the frame layout of a scope with the body e: the captures written in it in
// the order they are first met, unreachable ones included, following the build policy of the
// rule r with the projections proj.
func coreLayout(r *Rule, proj map[string]string, e grammar.Expr, build bool) *coreScope {
	s := &coreScope{}
	var visit func(grammar.Expr, bool)
	visit = func(e grammar.Expr, build bool) {
		switch e := e.(type) {
		case *grammar.Capture:
			if !build && r.PredCaps != nil && !r.PredCaps.Records(e) {
				if r.PredCaps.Slotted(e) {
					s.slot(e.Name)
				}
				visit(e.Expr, false)
				return
			}
			s.slot(e.Name)
			if proj[e.Name] == "" {
				visit(e.Expr, true)
			}
		case *grammar.Seq:
			for _, it := range e.Items {
				visit(it, build)
			}
		case *grammar.Choice:
			for _, alt := range e.Alts {
				visit(alt, build)
			}
		case *grammar.Repeat:
			if !check.ElementScoped(e.Expr) {
				visit(e.Expr, build)
			}
		case *grammar.Optional:
			visit(e.Expr, build)
		case *grammar.And:
			visit(e.Expr, build && check.HasCaptures(e.Expr))
		case *grammar.Not:
			visit(e.Expr, false)
		case *grammar.Atomic:
			visit(e.Expr, false)
		case *grammar.Discard:
			visit(e.Expr, false)
		case *grammar.Attributed:
			visit(e.Expr, build)
			for _, a := range e.Attrs {
				if a.Name == "recover" {
					skip, _ := a.Arg("skip")
					visit(skip, false)
				}
			}
		}
	}
	visit(e, build)
	return s
}

// set sets the attributes of the node n built from the expression e that the builder sets: whether
// its value is wanted, and its shape.
func (b *coreBuilder) set(n coreir.Node, e grammar.Expr, want bool) coreir.Node {
	a := n.Attrs()
	a.Want, a.Shape = want, coreShape(b.chk, e)
	return n
}

// expr builds the expression e in the scope s; build reports whether its value is wanted.
func (b *coreBuilder) expr(e grammar.Expr, s *coreScope, build bool) (coreir.Node, error) {
	n, err := b.node(e, s, build)
	if err == nil && b.origin != nil {
		if _, ok := b.origin[n]; !ok {
			b.origin[n] = e
		}
	}
	return n, err
}

func (b *coreBuilder) node(e grammar.Expr, s *coreScope, build bool) (coreir.Node, error) {
	switch e := e.(type) {
	case *grammar.Literal:
		n := &coreir.Lit{Text: value.LiteralValue(e.Value), Fold: e.IgnoreCase}
		if n.Text != "" {
			n.Exp = b.exp(coreir.LiteralDesc(e))
		}
		return b.set(n, e, build), nil
	case *grammar.CharClass:
		return b.set(&coreir.ClassNode{Class: b.class(e)}, e, build), nil
	case *grammar.Any:
		return b.set(&coreir.Any{}, e, build), nil
	case *grammar.Ref:
		r := b.in.ByName[e.Name]
		if r == nil {
			return nil, fmt.Errorf("undefined rule %s", e.Name)
		}
		level, err := callLevel(e, r)
		if err != nil {
			return nil, err
		}
		if !build && r.Twin != nil {
			r = r.Twin
		}
		return b.set(&coreir.Call{Rule: r.ID, Level: level}, e, build), nil
	case *grammar.Seq:
		n := &coreir.Seq{}
		for _, it := range e.Items {
			x, err := b.expr(it, s, build)
			if err != nil {
				return nil, err
			}
			n.Items = append(n.Items, x)
		}
		return b.set(n, e, build), nil
	case *grammar.Choice:
		n := &coreir.Choice{}
		for _, alt := range e.Alts {
			x, err := b.expr(alt, s, build)
			if err != nil {
				return nil, err
			}
			n.Alts = append(n.Alts, &coreir.Alt{Body: x})
		}
		return b.set(n, e, build), nil
	case *grammar.Repeat:
		return b.repeat(e, s, build, "")
	case *grammar.Optional:
		x, err := b.expr(e.Expr, s, build)
		if err != nil {
			return nil, err
		}
		return b.set(&coreir.Opt{Body: x}, e, build), nil
	case *grammar.And:
		x, err := b.expr(e.Expr, s, build && check.HasCaptures(e.Expr))
		if err != nil {
			return nil, err
		}
		if check.LookKeepsState(e.Expr) {
			// Its captures and bindings outlast it: no observation (check.LookKeepsState).
			return b.set(&coreir.Look{Body: x}, e, false), nil
		}
		b.looks++
		return b.set(&coreir.Look{Body: x, Site: b.looks}, e, false), nil
	case *grammar.Not:
		x, err := b.expr(e.Expr, s, false)
		if err != nil {
			return nil, err
		}
		b.looks++
		return b.set(&coreir.Look{Body: x, Neg: true, Site: b.looks}, e, false), nil
	case *grammar.Atomic:
		x, err := b.expr(e.Expr, s, false)
		if err != nil {
			return nil, err
		}
		return b.set(&coreir.Text{Body: x}, e, build), nil
	case *grammar.Discard:
		x, err := b.expr(e.Expr, s, false)
		if err != nil {
			return nil, err
		}
		return b.set(&coreir.Drop{Body: x}, e, false), nil
	case *grammar.Capture:
		if pc := b.cur.PredCaps; !build && pc != nil && !pc.Records(e) {
			// Captures that no predicate can read build nothing where no value is wanted; one of a
			// name that a predicate reads keeps its slot, which it leaves unset.
			if pc.Slotted(e) {
				s.slot(e.Name)
			}
			return b.expr(e.Expr, s, false)
		}
		slot := s.slot(e.Name)
		n := &coreir.Capture{Slot: slot, Name: e.Name}
		if field := b.proj[e.Name]; field != "" {
			rep, ok := e.Expr.(*grammar.Repeat)
			if !ok {
				return nil, fmt.Errorf("projected capture %s is not a repetition", e.Name)
			}
			x, err := b.repeat(rep, s, true, field)
			if err != nil {
				return nil, err
			}
			if b.origin != nil {
				b.origin[x] = rep
			}
			n.Body, n.Project = x, field
		} else {
			x, err := b.expr(e.Expr, s, true)
			if err != nil {
				return nil, err
			}
			n.Body = x
		}
		return b.set(n, e, build), nil
	case *grammar.Cut:
		return b.set(&coreir.Cut{}, e, build), nil
	case *grammar.Top:
		return b.set(&coreir.Succeed{}, e, build), nil
	case *grammar.Bottom:
		return b.set(&coreir.FailNode{}, e, build), nil
	case *grammar.BeginInput:
		return b.set(&coreir.Anchor{Kind: coreir.BeginInput}, e, build), nil
	case *grammar.EndInput:
		return b.set(&coreir.Anchor{Kind: coreir.EndInput}, e, build), nil
	case *grammar.BeginLine:
		return b.set(&coreir.Anchor{Kind: coreir.BeginLine}, e, build), nil
	case *grammar.EndLine:
		return b.set(&coreir.Anchor{Kind: coreir.EndLine}, e, build), nil
	case *grammar.Predicate:
		if a, ok := e.Term.(*grammar.Bind); ok {
			t, err := b.term(a.Value, s, nil)
			if err != nil {
				return nil, err
			}
			return b.set(&coreir.BindNode{Var: a.Name, Term: t}, e, build), nil
		}
		t, err := b.term(e.Term, s, nil)
		if err != nil {
			return nil, err
		}
		return b.set(&coreir.Pred{Term: t}, e, build), nil
	case *grammar.Attributed:
		return b.attributed(e, len(e.Attrs)-1, s, build)
	}
	return nil, fmt.Errorf("unsupported expression %T", e)
}

// firstStream returns the first #stream of body in walk order if it is at the top of body, else
// nil.
func firstStream(body grammar.Expr) grammar.Expr {
	var first grammar.Expr
	grammaranalysis.WalkExpr(body, func(e grammar.Expr) {
		if first == nil && check.IsStream(e) {
			first = e
		}
	})
	if first == nil || !check.StreamTops(body)[first] {
		return nil
	}
	return first
}

// attributed builds e under its attributes Attrs[:i+1], the last one outermost.
func (b *coreBuilder) attributed(e *grammar.Attributed, i int, s *coreScope, build bool) (coreir.Node, error) {
	if i < 0 {
		if rep, ok := e.Expr.(*grammar.Repeat); ok && e == b.stream {
			x, err := b.expr(rep, s, build)
			if err != nil {
				return nil, err
			}
			n := &coreir.Stream{Repeat: x.(*coreir.Repeat)}
			n.Attrs().Want, n.Attrs().Shape = x.Attrs().Want, x.Attrs().Shape
			return n, nil
		}
		return b.expr(e.Expr, s, build)
	}
	a := e.Attrs[i]
	switch a.Name {
	case "error", "diagnostic":
		x, err := b.attributed(e, i-1, s, build)
		if err != nil {
			return nil, err
		}
		n := &coreir.Label{Body: x, Diagnostic: a.Name == "diagnostic"}
		if n.Diagnostic {
			n.Exp = b.exp(desugar.AttributeDiagnosticName(a))
		} else {
			n.Exp = b.exp(desugar.AttributeMessage(a))
		}
		// The value of a label is that of its operand: the attributed expression once it recovers.
		shaped := grammar.Expr(e.Expr)
		if i == len(e.Attrs)-1 {
			shaped = e
		}
		for _, inner := range e.Attrs[:i] {
			if inner.Name == "recover" {
				shaped = e
			}
		}
		return b.set(n, shaped, build), nil
	case "recover":
		x, err := b.attributed(e, i-1, s, build)
		if err != nil {
			return nil, err
		}
		skipExpr, _ := a.Arg("skip")
		skip, err := b.expr(skipExpr, s, false)
		if err != nil {
			return nil, err
		}
		n := &coreir.Recover{Body: x, Skip: skip}
		if build {
			// What recovers is one element: the value of the expression, or the error. Its shape
			// gives an expression without a value no element, which its users then leave out.
			n.Attrs().Want, n.Attrs().Shape = true, coreShape(b.chk, e)
		}
		return n, nil
	case "stream":
		// The innermost attribute applies to the repetition itself (attributed).
		return b.attributed(e, i-1, s, build)
	case "missing":
		x, err := b.attributed(e, i-1, s, build)
		if err != nil {
			return nil, err
		}
		l, ok := x.(*coreir.Lit)
		if !ok || l.Text == "" {
			return nil, fmt.Errorf("#missing on %T, not a nonempty literal", x)
		}
		n := &coreir.Missing{Lit: l}
		n.Attrs().Want, n.Attrs().Shape = l.Attrs().Want, l.Attrs().Shape
		return n, nil
	}
	return nil, fmt.Errorf("unknown attribute %s", a.Name)
}

// repeat builds a repetition; with field, a projected repetition gathering the values of the
// capture field of its elements.
func (b *coreBuilder) repeat(e *grammar.Repeat, s *coreScope, build bool, field string) (coreir.Node, error) {
	n := &coreir.Repeat{Min: e.Min, Max: e.Max, Project: field, Site: b.prog.SkipSite(e), ScanSite: b.prog.ScanSite(e)}
	es := s
	if field != "" || check.ElementScoped(e.Expr) {
		es = b.scope(coreLayout(b.cur, b.proj, e.Expr, build && field == "").names, e.Expr)
	}
	x, err := b.expr(e.Expr, es, build && field == "")
	if err != nil {
		return nil, err
	}
	n.Body = x
	if es != s {
		n.Scope = es.slots()
		if n.Scope == nil {
			n.Scope = []coreir.Slot{}
		}
		if field != "" {
			if _, ok := es.lookup(field); !ok {
				return nil, fmt.Errorf("projected field %s is not captured in the element", field)
			}
		}
	}
	return b.set(n, e, build), nil
}

// term builds the term t in the scope s, with its checked type; env maps the names of lambda
// parameters and Operators Rule locals in scope to their locals.
func (b *coreBuilder) term(t grammar.Term, s *coreScope, env map[string]*coreir.Local) (coreir.Term, error) {
	if g, ok := t.(*grammar.TermGroup); ok {
		return b.term(g.Term, s, env)
	}
	x, err := b.termNode(t, s, env)
	if err != nil {
		return nil, err
	}
	if b.rec != nil {
		if typ, ok := b.rec.Terms[t]; ok {
			coreir.SetType(x, typ)
		}
	}
	return x, nil
}

func (b *coreBuilder) termNode(t grammar.Term, s *coreScope, env map[string]*coreir.Local) (coreir.Term, error) {
	sub := func(x grammar.Term) (coreir.Term, error) { return b.term(x, s, env) }
	switch t := t.(type) {
	case *grammar.IntLit:
		return &coreir.Const{Value: t.Value}, nil
	case *grammar.StringLit:
		return &coreir.Const{Value: t.Value}, nil
	case *grammar.BoolLit:
		return &coreir.Const{Value: t.Value}, nil
	case *grammar.NilLit:
		return &coreir.Const{Value: nil}, nil
	case *grammar.CaptureRef:
		if l, ok := env[t.Name]; ok {
			ref := *l
			return &ref, nil
		}
		slot, ok := s.lookup(t.Name)
		if !ok {
			return nil, fmt.Errorf("capture %s is not in scope %v", t.Name, s.names)
		}
		return &coreir.SlotRef{Slot: slot, Name: t.Name}, nil
	case *grammar.VarRef:
		return &coreir.VarRef{Name: t.Name}, nil
	case *grammar.Member:
		x, err := sub(t.X)
		if err != nil {
			return nil, err
		}
		return &coreir.Member{X: x, Name: t.Name}, nil
	case *grammar.New:
		if len(t.Args) != 0 {
			x, err := sub(t.Args[0])
			if err != nil {
				return nil, err
			}
			if b.chk == nil {
				return &coreir.Convert{NodeType: t.Type, X: x}, nil
			}
			n, ok := b.chk.NodeTypeOf(t.Type) // the type's own name, also for an alias of it
			if !ok {
				return nil, fmt.Errorf("%s is not a named node type", t.Type)
			}
			proven := b.rec != nil && b.rec.Proven[t]
			return &coreir.Convert{NodeType: n.Name, X: x, Proven: proven}, nil
		}
		name := t.Type
		var spec *grammar.TypeDef
		if b.chk != nil {
			name, spec = b.chk.StructType(t.Type)
		}
		n := &coreir.New{Type: name, Final: grammar.Term(t) == b.final}
		if b.chk != nil {
			for _, f := range check.DefFields(spec) {
				n.Declared = append(n.Declared, f.Name)
			}
		}
		for _, f := range t.Fields {
			v, err := sub(f.Value)
			if err != nil {
				return nil, err
			}
			n.Fields = append(n.Fields, coreir.FieldInit{Name: f.Name, Value: v})
		}
		return n, nil
	case *grammar.ListConstructor:
		n := &coreir.Builtin{Func: "list"}
		for _, x := range t.Items {
			v, err := sub(x)
			if err != nil {
				return nil, err
			}
			n.Args = append(n.Args, v)
		}
		return n, nil
	case *grammar.TupleLit:
		n := &coreir.Tuple{}
		for _, x := range t.Elems {
			v, err := sub(x)
			if err != nil {
				return nil, err
			}
			n.Elems = append(n.Elems, v)
		}
		return n, nil
	case *grammar.Call:
		n := &coreir.Builtin{Func: t.Func}
		for _, a := range t.Args {
			v, err := sub(a)
			if err != nil {
				return nil, err
			}
			n.Args = append(n.Args, v)
		}
		return n, nil
	case *grammar.Lambda:
		l := &coreir.Lambda{Params: t.Params}
		inner := make(map[string]*coreir.Local, len(env)+len(t.Params))
		for k, v := range env {
			inner[k] = v
		}
		for _, p := range t.Params {
			inner[p] = &coreir.Local{Kind: coreir.LocalParam, Name: p, Lambda: l}
		}
		body, err := b.term(t.Body, s, inner)
		if err != nil {
			return nil, err
		}
		l.Body = body
		return l, nil
	case *grammar.Binary:
		l, err := sub(t.L)
		if err != nil {
			return nil, err
		}
		r, err := sub(t.R)
		if err != nil {
			return nil, err
		}
		return &coreir.Binary{Op: t.Op, L: l, R: r}, nil
	case *grammar.Unary:
		x, err := sub(t.X)
		if err != nil {
			return nil, err
		}
		return &coreir.Unary{Op: t.Op, X: x}, nil
	case *grammar.Cast:
		x, err := sub(t.X)
		if err != nil {
			return nil, err
		}
		line, col := b.prog.CastPos(t)
		// A cast to a type that every value fits has no check (Check nil); it stays a term, which a
		// target may charge for.
		var check any
		if c := b.prog.ResolveCheck(t.Type); c != nil {
			check = c
		}
		return &coreir.Cast{X: x, Check: check, Key: t, Line: line, Col: col}, nil
	}
	return nil, fmt.Errorf("unsupported term %T", t)
}

// FollowRepeats returns functions that, once the scan pass has run, give the scan that took the
// place of each repetition with an origin the origin of the repetition. A scan that took the place
// of a Stream gets the origin of the Stream's repetition.
func FollowRepeats(m *coreir.Module, origin map[coreir.Node]grammar.Expr) []func() {
	var out []func()
	visit := func(at func() coreir.Node) {
		rep, ok := at().(*coreir.Repeat)
		if st, isStream := at().(*coreir.Stream); isStream {
			rep, ok = st.Repeat, true
		}
		if ok {
			if e, ok := origin[rep]; ok {
				out = append(out, func() {
					if s, ok := at().(*coreir.Scan); ok {
						origin[s] = e
					}
				})
			}
		}
	}
	for _, r := range m.Rules {
		EachSlot(r, visit)
	}
	return out
}

// EachSlot calls f with an accessor of every place of the rule's body where a node is: the body,
// the bodies of Operators Rule lines and the children of nodes.
func EachSlot(r *coreir.Rule, f func(at func() coreir.Node)) {
	var walk func(at func() coreir.Node)
	walk = func(at func() coreir.Node) {
		f(at)
		n := at()
		if o, ok := n.(*coreir.OperatorsRule); ok {
			for _, l := range o.Lines() {
				walk(func() coreir.Node { return l.Body })
			}
			return
		}
		for i := range coreir.Children(n) {
			walk(func() coreir.Node { return coreir.Children(n)[i] })
		}
	}
	walk(func() coreir.Node { return r.Body })
}
