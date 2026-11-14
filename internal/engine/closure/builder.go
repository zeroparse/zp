package closure

import (
	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	ir "zeroparse.dev/zp/internal/ir/core"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/value"
)

// Builder builds the closure backend's form of a program from its Core IR (Build): a matcher per
// node, and the bodies, values and actions of the rules.
type Builder struct {
	prog *compile.Program
	m    *ir.Module
	// plainDispatch leaves out the budget charge of the loop iterations; without it the builder
	// builds the metered closure graph.
	plainDispatch bool
	// family is the rule set being built, which decides how the call sites call plain rules.
	family state.Family
	// static holds the static rules of the program, by ID: the rules and then their twins.
	static []*compile.Rule
	// rules holds the rules the parser runs, by ID (staticRule.ID). They are made when a body or a
	// call needs them and get the facts of their static rules in Build.
	rules []*state.Rule
	// descs maps the expectations of the module (ir.Module.Exps) to those of the program
	// (compile.Program.DescTable), which the parser records.
	descs []state.ExpID
	// accepts holds the membership test of each class of the module.
	accepts map[*ir.Class]func(*state.Parser, rune) bool
	// cur is the static rule whose body is being built, and scope the capture scope of the node
	// being built.
	cur   *compile.Rule
	scope *compile.Scope
	// err is the first internal error of the build.
	err error
}

// rule returns the rule the parser runs for the static rule of the ID id.
func (b *Builder) rule(id int) *state.Rule {
	for len(b.rules) <= id {
		b.rules = append(b.rules, nil)
	}
	if b.rules[id] == nil {
		b.rules[id] = &state.Rule{}
	}
	return b.rules[id]
}

// charges reports whether the loops charge the budget for each iteration. A rule call charges in
// state.Parser.Call and InvokePlain, whichever graph runs it.
func (b *Builder) charges() bool { return state.BudgetControlsEnabled && !b.plainDispatch }

// ruleBody completes the rule sr, whose Core IR is cr, with its body m.
func (b *Builder) ruleBody(sr *compile.Rule, cr *ir.Rule, m state.Matcher) error {
	r := b.rule(sr.ID)
	r.Kinds = coreimpl.NewRuleKinds(sr.Name)
	if sr.Splice {
		r.Value = func(_ *state.Parser, v *core.Node, _ int) *core.Node { return v }
	} else {
		r.Value = scopeValueOf(cr.Finish.Shape)
	}
	r.Body = func(p *state.Parser, _ int) (*core.Node, bool) { return m(p) }
	if cr.Finish.Kind == ir.FinishAction {
		t, err := b.term(cr.Finish.Action, projections(cr.Body))
		if err != nil {
			return err
		}
		r.Act = t.Eval
	}
	return nil
}

// projections returns the projected captures of a rule body, mapped to the field each gathers
// (ir.Capture.Project): the action reads the list of such a capture as the values of that field.
func projections(body ir.Node) map[string]string {
	var proj map[string]string
	ir.Walk(body, func(n ir.Node) {
		if c, ok := n.(*ir.Capture); ok && c.Project != "" {
			if proj == nil {
				proj = map[string]string{}
			}
			proj[c.Name] = c.Project
		}
	})
	return proj
}

// operatorsRule completes the Operators Rule sr, whose Core IR is cr, with its lines.
func (b *Builder) operatorsRule(sr *compile.Rule, cr *ir.Rule, o *ir.OperatorsRule) error {
	r := b.rule(sr.ID)
	r.Kinds = coreimpl.NewRuleKinds(sr.Name)
	// The lines build the rule's value.
	r.Value = func(_ *state.Parser, v *core.Node, _ int) *core.Node { return v }
	pr := &state.OperatorsRule{}
	r.OperatorsRule = pr
	if o.Skip != nil {
		b.scope = scopeOf(o.Skip.Scope)
		pr.Skip = b.node(o.Skip.Body)
	}
	for _, l := range o.Operands {
		pl, err := b.line(l)
		if err != nil {
			return err
		}
		pr.Operands = append(pr.Operands, pl)
	}
	for _, x := range o.Ops {
		pl, err := b.line(x.Line)
		if err != nil {
			return err
		}
		op := &state.OperatorsRuleOp{Id: x.ID, Kind: x.Kind, Assoc: x.Assoc, Level: x.Level, Line: pl}
		if x.Kind == grammar.Prefix {
			pr.Prefix = append(pr.Prefix, op)
		} else {
			pr.Led = append(pr.Led, op)
		}
	}
	r.Body = func(p *state.Parser, min int) (*core.Node, bool) { return p.OperatorsRuleParse(r, min) }
	if cr.Finish.Kind == ir.FinishAction {
		// The whole-rule action runs once per call, after the fold (state.Parser.Finish).
		b.scope = scopeOf(nil)
		t, err := b.term(cr.Finish.Action, nil)
		if err != nil {
			return err
		}
		r.Act = t.Eval
	}
	return nil
}

// line builds a line of an Operators Rule.
func (b *Builder) line(l *ir.OpLine) (*state.OperatorsRuleLine, error) {
	b.scope = scopeOf(l.Scope)
	pl := &state.OperatorsRuleLine{Scope: b.scope, M: b.node(l.Body), Key: l}
	pl.Value, pl.Single, pl.Shape = scopeValueOf(l.Shape), compile.SingleMode(l.Shape.Mode), shaper(l.Shape)
	if l.Action != nil {
		t, err := b.term(l.Action, nil)
		if err != nil {
			return nil, err
		}
		pl.Act = t.Eval
	}
	return pl, nil
}

func (b *Builder) literal(val string, fold bool, desc state.ExpID, build bool) state.Matcher {
	if fold {
		rs := compile.FoldedRunes([]rune(val))
		return func(p *state.Parser) (*core.Node, bool) {
			start := p.Pos
			if !p.MatchLiteralFold(rs) {
				p.Expect(start, desc)
				return nil, false
			}
			if !build {
				return nil, true
			}
			// The node holds the input as matched: its letters may differ in case from the literal.
			return p.NewNode(value.Data{Kind: coreimpl.MatchKind, Start: int32(start), End: int32(p.Pos), Text: p.Text(start, p.Pos), Terminal: true, Fresh: true}), true
		}
	}
	rs, bs := []rune(val), []byte(val)
	return func(p *state.Parser) (*core.Node, bool) {
		start := p.Pos
		if !p.MatchLiteral(rs, bs) {
			p.Expect(start, desc)
			return nil, false
		}
		if !build {
			return nil, true
		}
		return p.NewNode(value.Data{Kind: coreimpl.MatchKind, Start: int32(start), End: int32(p.Pos), Text: p.LiteralText(start, val), Terminal: true, Fresh: true}), true
	}
}

// call returns the matcher of a call of the rule of the ID id at binding level min. In the inlined
// family a call of a plain rule invokes it directly, which is what state.Parser.Call does for such a
// rule without its checks; every other call, and every call in the call-preserving family, goes
// through Call. The rule gets the facts of its static rule only at the end of Build, so the
// static rule decides (a Matcher of a test may call a rule its program lacks: not plain).
func (b *Builder) call(id, min int) state.Matcher {
	r := b.rule(id)
	if b.family == state.Inlined && id < len(b.static) && b.static[id].Plain {
		return func(p *state.Parser) (*core.Node, bool) { return p.InvokePlain(r, min) }
	}
	return func(p *state.Parser) (*core.Node, bool) { return p.Call(r, min) }
}

func (b *Builder) seq(ms []state.Matcher, flat, valued []bool, n int, build bool) state.Matcher {
	// The value is the tuple node of the elements, flattened through nested sequences.
	return func(p *state.Parser) (*core.Node, bool) {
		start := p.Pos
		var kids []*core.Node
		if build {
			kids = p.Nodes(n)[:0]
		}
		for i, m := range ms {
			v, ok := m(p)
			if !ok {
				return nil, false
			}
			if !build {
				continue
			}
			switch {
			case flat[i]:
				kids = append(kids, value.D(v).Children...)
			case valued[i]:
				kids = append(kids, v)
			}
		}
		if !build {
			return nil, true
		}
		return p.NewNode(value.Data{Kind: coreimpl.SeqKind, Start: int32(start), End: int32(p.Pos), Children: kids, Fresh: true}), true
	}
}

func (b *Builder) choice(alts []*ir.Alt, ms []state.Matcher, build bool) state.Matcher {
	guards := make([]firstGuard, len(alts))
	guarded := false
	for i, alt := range alts {
		if build {
			ms[i] = singleMatcher(alt.Body.Attrs().Shape, ms[i])
		}
		guards[i] = b.guard(alt.Guard)
		guarded = guarded || guards[i].accept != nil
	}
	if guarded {
		return func(p *state.Parser) (*core.Node, bool) {
			m0 := p.Mark()
			var ch rune
			more, peeked := false, false
			for i, m := range ms {
				if g := &guards[i]; g.accept != nil && p.Missing == nil {
					if !peeked {
						ch, _, more = p.Peek()
						peeked = true
					}
					if !(more && g.accept(p, ch)) && p.Depth+g.depth <= p.MaxDepth {
						p.Expect(p.Pos, g.desc) // what the alternative would record
						continue
					}
				}
				prevCut := p.Cut
				p.Cut = false
				v, ok := m(p)
				cut := p.Cut
				p.Cut = prevCut
				if ok {
					return v, true
				}
				p.Reset(m0)
				if cut {
					return nil, false
				}
			}
			return nil, false
		}
	}
	return func(p *state.Parser) (*core.Node, bool) {
		m0 := p.Mark()
		for _, m := range ms {
			prevCut := p.Cut
			p.Cut = false
			v, ok := m(p)
			cut := p.Cut
			p.Cut = prevCut
			if ok {
				return v, true
			}
			p.Reset(m0)
			if cut {
				return nil, false
			}
		}
		return nil, false
	}
}

// firstGuard tells when an alternative of a choice can be skipped (the guard of the alternative in
// Core IR).
type firstGuard struct {
	accept func(*state.Parser, rune) bool // nil if it cannot be
	desc   state.ExpID
	depth  int
}

// guard returns the test of the next character that the alternative guarded by f needs.
func (b *Builder) guard(f *ir.First) firstGuard {
	switch {
	case f == nil:
		return firstGuard{}
	case f.Lit && f.Fold:
		f0 := f.Rune
		return firstGuard{func(_ *state.Parser, r rune) bool { return compile.FoldASCIIRune(r) == f0 }, b.desc(f.Exp), f.Depth}
	case f.Lit:
		r0 := f.Rune
		return firstGuard{func(_ *state.Parser, r rune) bool { return r == r0 }, b.desc(f.Exp), f.Depth}
	}
	return firstGuard{b.accept(f.Class), b.desc(f.Exp), f.Depth}
}

func (b *Builder) optional(body ir.Node, m state.Matcher, build bool) state.Matcher {
	if build {
		m = singleMatcher(body.Attrs().Shape, m)
	}
	return func(p *state.Parser) (*core.Node, bool) {
		m0 := p.Mark()
		prevCut := p.Cut
		p.Cut = false
		v, ok := m(p)
		cut := p.Cut
		p.Cut = prevCut
		if ok {
			return v, true
		}
		p.Reset(m0)
		if cut {
			return nil, false
		}
		return nil, true
	}
}

func (b *Builder) and(m state.Matcher, site int) state.Matcher {
	if site == 0 {
		// A lookahead that leaves captures or bindings is no observation (Parser.LookBegin).
		site = -1
	}
	return func(p *state.Parser) (*core.Node, bool) {
		start, prevCut := p.Pos, p.Cut
		p.Silent++
		p.LookBegin(site)
		_, ok := m(p)
		p.LookEnd(ok)
		p.Silent--
		p.Cut = prevCut
		p.Pos = start
		return nil, ok
	}
}

func (b *Builder) not(m state.Matcher, site int) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		m0 := p.Mark()
		prevCut := p.Cut
		p.Silent++
		p.LookBegin(site)
		_, ok := m(p)
		p.LookEnd(ok)
		p.Silent--
		p.Cut = prevCut
		p.Reset(m0)
		return nil, !ok
	}
}

func (b *Builder) atomic(m state.Matcher, build bool) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		start := p.Pos
		if _, ok := m(p); !ok {
			return nil, false
		}
		if !build {
			return nil, true
		}
		return p.NewNode(value.Data{Kind: coreimpl.MatchKind, Start: int32(start), End: int32(p.Pos), Text: p.Text(start, p.Pos), Terminal: true, Fresh: true}), true
	}
}

func (b *Builder) discard(m state.Matcher) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		_, ok := m(p)
		return nil, ok
	}
}

func (b *Builder) capture(n *ir.Capture, m state.Matcher, build bool) state.Matcher {
	slot := n.Slot
	if n.Project == "" {
		m = captureMatcher(n.Body.Attrs().Shape, m)
	}
	return func(p *state.Parser) (*core.Node, bool) {
		v, ok := m(p)
		if ok {
			p.SetCapture(slot, v)
		}
		if !build {
			return nil, ok
		}
		return v, ok
	}
}

func (b *Builder) cut() state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		p.Cut = true
		return nil, true
	}
}

func (b *Builder) empty() state.Matcher {
	// _ matches the empty input and has no value.
	return func(p *state.Parser) (*core.Node, bool) { return nil, true }
}

func (b *Builder) fail() state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) { return nil, false }
}

func (b *Builder) anchor(kind ir.AnchorKind, desc state.ExpID) state.Matcher {
	switch kind {
	case ir.BeginInput:
		return anchor(func(p *state.Parser) bool { return p.AtInputStart() }, desc)
	case ir.EndInput:
		return anchor(func(p *state.Parser) bool { return p.AtEOF() }, desc)
	case ir.BeginLine:
		return anchor(func(p *state.Parser) bool { return p.AtLineStart() }, desc)
	}
	return anchor(func(p *state.Parser) bool {
		ch, _, ok := p.Peek()
		return !ok || ch == '\n' || ch == '\r'
	}, desc)
}

func anchor(cond func(p *state.Parser) bool, desc state.ExpID) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		if cond(p) {
			return nil, true
		}
		p.Expect(p.Pos, desc)
		return nil, false
	}
}

// single returns a matcher of one character that accept accepts.
func single(build bool, desc state.ExpID, accept func(*state.Parser, rune) bool) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		ch, size, ok := p.Peek()
		if !ok || !accept(p, ch) {
			p.Expect(p.Pos, desc)
			return nil, false
		}
		start := p.Pos
		p.Pos += size
		if !build {
			return nil, true
		}
		return p.NewNode(value.Data{Kind: coreimpl.MatchKind, Start: int32(start), End: int32(p.Pos), Text: p.Text(start, p.Pos), Terminal: true, Fresh: true}), true
	}
}

// ScanRepeat advances over the repetition without saving and restoring state per element.
// Expectations are recorded as for a general repetition (once, at the position that did not match
// last).
func (b *Builder) scanRepeat(n *ir.Scan) state.Matcher {
	accept, desc := func(*state.Parser, rune) bool { return true }, compile.IDAny
	if n.Class != nil {
		accept, desc = b.accept(n.Class), b.desc(n.Class.Exp)
	}
	min, max, scan := n.Min, n.Max, n.Site
	charge := b.charges()
	if scan != 0 && max < 0 {
		return b.scanSite(accept, desc, min, charge, scan)
	}
	return func(p *state.Parser) (*core.Node, bool) {
		count := 0
		for max < 0 || count < max {
			if charge {
				p.Spend()
			}
			ch, size, ok := p.Peek()
			if !ok || !accept(p, ch) {
				p.Expect(p.Pos, desc)
				break
			}
			p.Pos += size
			count++
		}
		return nil, count >= min
	}
}

// scanSite is ScanRepeat for the scan site scan (state/scan_sites.go): a walk that reaches its
// ScanHotSpan-th character looks for the record of its start, if the site is hot, and otherwise
// reports itself to the cold state of the site.
func (b *Builder) scanSite(accept func(*state.Parser, rune) bool, desc state.ExpID, min int, charge bool, site int) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		start, span := p.Pos, state.ScanHotSpan
		count := 0
		for count < span {
			if charge {
				p.Spend()
			}
			ch, size, ok := p.Peek()
			if !ok || !accept(p, ch) {
				p.Expect(p.Pos, desc)
				return nil, count >= min
			}
			p.Pos += size
			count++
		}
		hot, hit := p.ScanHot(site), false
		hw0 := p.Hw
		p.Hw = 0
		if hot {
			var rec state.ScanRecord
			if rec, hit = p.ScanLookup(site, start); hit {
				p.ScanJump(rec)
				count = rec.Count()
			}
		}
		for {
			if charge {
				p.Spend()
			}
			ch, size, ok := p.Peek()
			if !ok || !accept(p, ch) {
				p.Expect(p.Pos, desc)
				break
			}
			p.Pos += size
			count++
		}
		p.FinishScan(site, start, count, hot, hit, hw0)
		return nil, count >= min
	}
}

// scanSiteList is Repeat for the scan site scan whose walks build a list of the elements (see
// scanSite). The element is a single character, matched by m, so it captures nothing and cuts nothing.
func (b *Builder) scanSiteList(m state.Matcher, elemValue func(*state.Parser, *core.Node, int) *core.Node, rs *state.RunSite, min int, charge, build bool, site int) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		if rs != nil && p.Runs != nil && p.Silent == 0 {
			return p.ResumeRepeat(rs)
		}
		start, span := p.Pos, state.ScanHotSpan
		base := len(p.KidStack)
		var (
			hot, hit bool
			rec      state.ScanRecord
			hw0      int
		)
		count := 0
		for n := 0; ; n++ {
			if n == span {
				hot = p.ScanHot(site)
				hw0 = p.Hw
				p.Hw = 0
				if hot {
					if rec, hit = p.ScanLookup(site, start); hit {
						p.ScanJump(rec)
						count = rec.Count()
					}
				}
			}
			if charge {
				p.Spend()
			}
			m0 := p.Mark()
			v, ok := m(p)
			if !ok {
				p.Reset(m0)
				break
			}
			count++
			if build {
				p.KidStack = append(p.KidStack, elemValue(p, v, m0.Pos))
			}
			if p.Pos == m0.Pos {
				break
			}
		}
		if count < min {
			if count >= span {
				p.Hw = max(hw0, p.Hw)
			}
			p.Kids(base)
			return nil, false
		}
		if !build {
			if count >= span {
				p.FinishScan(site, start, count, hot, hit, hw0)
			}
			return nil, true
		}
		if count >= span {
			return p.FinishScanList(site, start, base, count, hot, hit, rec, hw0), true
		}
		return p.NewNode(value.Data{Kind: coreimpl.ListKind, Start: int32(start), End: int32(p.Pos), Children: p.Kids(base), Fresh: true}), true
	}
}

// SkipRepeat is Repeat for the skip-run site skip, value-free and without a scope, whose walks
// record the rest of them from each position and reuse the records (state/skip_runs.go).
func (b *Builder) skipRepeat(min int, m state.Matcher, skip int) state.Matcher {
	charge := b.charges()
	keyEnv := !b.prog.EnvFree(skip)
	return func(p *state.Parser) (*core.Node, bool) {
		p.SkipBegin(skip, keyEnv)
		count := 0
		for {
			if charge {
				p.Spend()
			}
			if count >= min && p.SkipHit() {
				break
			}
			p.SkipVisit()
			m0 := p.Mark()
			prevCut := p.Cut
			p.Cut = false
			_, ok := m(p)
			p.Cut = prevCut
			if !ok {
				p.Reset(m0)
				break
			}
			count++
			if p.Pos == m0.Pos {
				// Stop a repetition that consumes no input.
				break
			}
		}
		p.SkipEnd()
		return nil, count >= min
	}
}

// runRepeat is Repeat for the run site run (compile/run_sites.go): the walks of its iterations
// record the rest of them from each position and reuse the records (state/skip_runs.go). The
// iterations have a scope of their own where they capture; build says whether they have values,
// which the walk gathers in a list.
func (b *Builder) runRepeat(min int, m state.Matcher, elemValue func(*state.Parser, *core.Node, int) *core.Node, elemScope *compile.Scope, ownScope, build bool, run int) state.Matcher {
	charge := b.charges()
	keyEnv := !b.prog.EnvFree(run)
	return func(p *state.Parser) (*core.Node, bool) {
		start := p.Pos
		if build {
			p.SkipBeginList(run, keyEnv)
		} else {
			p.SkipBegin(run, keyEnv)
		}
		count := 0
		for {
			if charge {
				p.Spend()
			}
			if count >= min && p.SkipHit() {
				break
			}
			p.SkipVisit()
			m0 := p.Mark()
			prevCut, prevFrame := p.Cut, p.Frame
			p.Cut = false
			var f *state.Frame
			if ownScope {
				f = p.NewFrame(len(elemScope.Names))
				p.Frame = f
			}
			v, ok := m(p)
			p.Cut, p.Frame = prevCut, prevFrame
			if !ok {
				p.Reset(m0)
				break
			}
			if build {
				v = elemValue(p, v, m0.Pos)
				if ownScope {
					v = p.AttachCaptures(v, elemScope, f, m0.Pos, p.Pos)
				}
				p.SkipValue(v)
			}
			count++
			if p.Pos == m0.Pos {
				// Stop a repetition that consumes no input.
				break
			}
		}
		if count < min {
			p.SkipEndList(start, false, true)
			return nil, false
		}
		v := p.SkipEndList(start, build, true)
		return v, true
	}
}

func (b *Builder) repeat(n *ir.Repeat, m state.Matcher, elemScope *compile.Scope, ownScope, build, stream bool) state.Matcher {
	min, max, site, shiftable, in, run := n.Min, n.Max, n.Resume, n.Shiftable, b.cur, n.Run
	// A repetition that delivers stream elements is no scan site; one that builds values uses the
	// site's second number.
	scan := 0
	if !stream && n.ScanSite != 0 {
		scan = n.ScanSite
		if build {
			scan++
		}
	}
	elemValue := scopeValueOf(n.Body.Attrs().Shape)
	// A run site walks as a plain repetition until its walks overlap, and keeps records after.
	var hot state.Matcher
	if run != 0 {
		hot = b.runRepeat(min, m, elemValue, elemScope, ownScope, build, run)
	}
	var rs *state.RunSite
	if site != 0 {
		rs = &state.RunSite{Id: site, M: m, Scope: elemScope, OwnScope: ownScope, Build: build, Min: min, Max: max, Shiftable: shiftable, Value: elemValue}
	}
	charge := b.charges()
	inID, inRec := in.ID, in.Rec
	if scan != 0 && !stream && !ownScope && max < 0 && run == 0 {
		return b.scanSiteList(m, elemValue, rs, min, charge, build, scan)
	}
	return func(p *state.Parser) (*core.Node, bool) {
		if rs != nil && p.Runs != nil && p.Silent == 0 {
			return p.ResumeRepeat(rs)
		}
		if run != 0 && p.RunHot(run) {
			return hot(p)
		}
		start := p.Pos
		base := len(p.KidStack) // collect the element values
		if state.StreamPrefixIsolation && stream && p.StreamsIn(inID, inRec) {
			p.StartStreamChunks()
		}
		count := 0
		for n := 0; max < 0 || n < max; n++ {
			if charge {
				p.Spend()
			}
			m0 := p.Mark()
			prevCut, prevFrame := p.Cut, p.Frame
			p.Cut = false
			var f *state.Frame
			if ownScope {
				f = p.NewFrame(len(elemScope.Names))
				p.Frame = f
			}
			v, ok := m(p)
			cut := p.Cut
			p.Cut, p.Frame = prevCut, prevFrame
			if !ok {
				p.Reset(m0)
				if cut {
					p.Kids(base)
					return nil, false
				}
				break
			}
			if build {
				v = elemValue(p, v, m0.Pos)
				if ownScope {
					v = p.AttachCaptures(v, elemScope, f, m0.Pos, p.Pos)
				}
			}
			count++
			if stream && p.StreamsIn(inID, inRec) {
				// Elements of the start rule's #stream are passed on without being retained, and committed.
				p.EmitStream(v, p.Pos)
				if p.Pos == m0.Pos {
					break
				}
				continue
			}
			if build {
				p.KidStack = append(p.KidStack, v)
			}
			if p.Pos == m0.Pos {
				// Stop a repetition that consumes no input.
				break
			}
		}
		if count < min {
			p.Kids(base)
			return nil, false
		}
		if run != 0 && p.Pos-start >= state.RunHotSpan {
			p.RunWalked(run, start)
		}
		if !build {
			return nil, true
		}
		return p.NewNode(value.Data{Kind: coreimpl.ListKind, Start: int32(start), End: int32(p.Pos), Children: p.Kids(base), Fresh: true}), true
	}
}

// ProjectRepeat gathers the values of a capture of the elements, which share one capture frame:
// nothing refers to an element's frame once its value is taken.
func (b *Builder) projectRepeat(min, max int, m state.Matcher, slot, n int) state.Matcher {
	charge := b.charges()
	return func(p *state.Parser) (*core.Node, bool) {
		start := p.Pos
		base := len(p.KidStack)
		f := p.NewFrame(n)
		count := 0
		for i := 0; max < 0 || i < max; i++ {
			if charge {
				p.Spend()
			}
			state.ClearSpend(p, f.Vals)
			m0 := p.Mark()
			prevCut, prevFrame := p.Cut, p.Frame
			p.Cut, p.Frame = false, f
			_, ok := m(p)
			cut := p.Cut
			p.Cut, p.Frame = prevCut, prevFrame
			if !ok {
				p.Reset(m0)
				if cut {
					p.Kids(base)
					return nil, false
				}
				break
			}
			count++
			p.KidStack = append(p.KidStack, f.Vals[slot])
			if p.Pos == m0.Pos {
				break
			}
		}
		if count < min {
			p.Kids(base)
			return nil, false
		}
		return p.NewNode(value.Data{Kind: coreimpl.ListKind, Start: int32(start), End: int32(p.Pos), Children: p.Kids(base), Fresh: true}), true
	}
}

func (b *Builder) recover(body ir.Node, m, skip state.Matcher, build bool) state.Matcher {
	if build {
		m = captureMatcher(body.Attrs().Shape, m)
	}
	return recoverAttr(m, skip, build)
}

func errorAttr(m state.Matcher, msg state.ExpID) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		v, ok, far, expected := state.Tracked(p, m)
		if ok {
			p.MergeExpected(far, expected)
			return v, true
		}
		p.Expect(far, msg)
		return nil, false
	}
}

// diagnosticAttr names ordinary expectations of a failure at the expression boundary.
// Explicit messages and the tracked failure position remain unchanged.
func diagnosticAttr(m state.Matcher, name state.ExpID) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		v, ok, far, expected := state.Tracked(p, m)
		if ok {
			p.MergeExpected(far, expected)
			return v, true
		}
		p.MergeExpected(far, state.RenamedExpected(p, expected, name))
		return nil, false
	}
}

// recoverAttr records an error when the expression fails, skips the input matched by skip, and
// returns an Error node. If skip fails or skips no characters, it fails without recovering.
func recoverAttr(m, skip state.Matcher, build bool) state.Matcher {
	return func(p *state.Parser) (*core.Node, bool) {
		m0 := p.Mark()
		v, ok, far, expected := state.Tracked(p, m)
		if ok {
			p.MergeExpected(far, expected)
			return v, true
		}
		expected = p.Keep(expected) // keep them while skip runs
		p.Reset(m0)
		if _, ok := skip(p); !ok || p.Pos == m0.Pos {
			p.Reset(m0)
			p.MergeExpected(far, expected)
			return nil, false
		}
		e := p.MakeError(far, expected)
		p.PushRecovered(e)
		if !build {
			return nil, true
		}
		return p.NewNode(value.Data{Kind: coreimpl.ErrorKind, Start: int32(m0.Pos), End: int32(p.Pos), Fields: coreimpl.Fields{{Name: "message", Value: p.RecoveryErrorText(e)}}, Fresh: true, Terminal: true}), true
	}
}

// singleMatcher returns a matcher of the value of e where it takes one element (an alternative or
// the operand of ?), from the building matcher m of e. Without a value the result is nil.
// shaper returns the runtime's shaper of a value of the shape s.
func shaper(s ir.Shape) compile.Shaper {
	return compile.Shaper{Elems: s.Elems, Shared: s.Shared, K: s.K}
}

func singleMatcher(s ir.Shape, m state.Matcher) state.Matcher {
	mode, sh := compile.SingleMode(s.Mode), shaper(s)
	if mode == compile.SinglePass && !sh.Elems {
		return m
	}
	return func(p *state.Parser) (*core.Node, bool) {
		start := p.Pos
		v, ok := m(p)
		if !ok {
			return nil, false
		}
		v, _ = state.ShapeSingle(p, sh, mode, v, start)
		return v, true
	}
}

func captureMatcher(s ir.Shape, m state.Matcher) state.Matcher {
	mode, sh := compile.SingleMode(s.Mode), shaper(s)
	if mode == compile.SinglePass && !sh.Elems {
		return m
	}
	return func(p *state.Parser) (*core.Node, bool) {
		start := p.Pos
		v, ok := m(p)
		if !ok {
			return nil, false
		}
		if v, ok = state.ShapeSingle(p, sh, mode, v, start); !ok {
			v = p.NewNode(value.Data{Kind: coreimpl.SeqKind, Start: int32(start), End: int32(p.Pos), Fresh: true})
		}
		return v, true
	}
}

func scopeValueOf(s ir.Shape) func(p *state.Parser, v *core.Node, start int) *core.Node {
	sh := shaper(s)
	if s.Tuple {
		return func(p *state.Parser, v *core.Node, start int) *core.Node { return state.ShapeTuple(p, sh, v, start) }
	}
	return func(_ *state.Parser, v *core.Node, _ int) *core.Node { return sh.One(v) }
}
