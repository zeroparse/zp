package compile

import (
	"slices"
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/desugar"
)

// BytecodeModule is Module.
func (prog *Program) BytecodeModule() *bytecode.Module {
	if prog.HasMissing {
		panic(MissingUnsupported("bytecode module construction"))
	}
	prog.BytecodeOnce.Do(func() {
		c := &bcompiler{prog: prog, m: &bytecode.Module{SourceInfo: prog.RecordsSourceInfo, DefaultValue: prog.MakesDefaultValue}, strs: map[string]int{}}
		c.compile()
		prog.Bytecode, prog.BytecodeTerms = c.m, c.terms
	})
	return prog.Bytecode
}

// bcompiler compiles a program to a bytecode module (Program.BytecodeModule): m is the module
// it fills and strs indexes the module's string table.
type bcompiler struct {
	prog *Program
	cur  *Rule             // rule being compiled
	proj map[string]string // projected captures of its body (ruleProjections)
	m    *bytecode.Module
	strs map[string]int
	// terms are the terms of the module, whose code the engine makes (Program.BytecodeTerms).
	terms []TermSite
}

// TermSite is a term of a bytecode module: the action, predicate or binding value Term, which reads
// the frame of Scope, with the projected captures Proj of the rule whose action it is (nil for
// other terms). The compiler leaves the term code of each site (Module.Terms) empty, because term
// code is the engine's translation of the term's Core IR, which this package cannot make; the engine
// fills it before a VM runs the module or a file saves it.
type TermSite struct {
	Term  grammar.Term
	Scope *Scope
	Proj  map[string]string
}

func (c *bcompiler) str(s string) int32 {
	if i, ok := c.strs[s]; ok {
		return int32(i)
	}
	c.strs[s] = len(c.m.Strings)
	c.m.Strings = append(c.m.Strings, s)
	return int32(len(c.m.Strings) - 1)
}

func (c *bcompiler) strs2(names []string) []int {
	out := make([]int, len(names))
	for i, n := range names {
		out[i] = int(c.str(n))
	}
	return out
}

func (c *bcompiler) emit(op bytecode.Op, a, b, cc int32) int {
	c.m.Code = append(c.m.Code, bytecode.Instr{Op: op, A: a, B: b, C: cc})
	return len(c.m.Code) - 1
}

func (c *bcompiler) here() int32 { return int32(len(c.m.Code)) }

// integer adds a wide integer to the module and returns its index.
func (c *bcompiler) integer(value int) int32 {
	c.m.Ints = append(c.m.Ints, int64(value))
	return int32(len(c.m.Ints) - 1)
}

func (c *bcompiler) repetition(op bytecode.Op, min, max int, other int32) {
	// Public ASTs allow any negative maximum. Canonicalize before narrowing
	// so large negative values cannot wrap into finite bytecode bounds.
	if max < 0 {
		max = -1
	}
	if int64(min) <= 1<<31-1 && int64(max) <= 1<<31-1 {
		if op == bytecode.OpScan {
			c.emit(op, other, int32(min), int32(max))
		} else {
			c.emit(op, int32(min), int32(max), other)
		}
		return
	}
	lo, hi := c.integer(min), c.integer(max)
	if op == bytecode.OpScan {
		c.emit(bytecode.OpScanWide, other, lo, hi)
	} else {
		c.emit(bytecode.OpRepeatWide, lo, hi, other)
	}
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// compile fills M with the type table, the rule table and the code of the rules of Prog.
func (c *bcompiler) compile() {
	// Type table
	for _, td := range c.prog.Grammar.Types() {
		switch spec := td.(type) {
		case *grammar.TypeDef:
			ti := bytecode.TypeInfo{Name: int(c.str(td.DeclName())), Node: -1}
			if n, ok := c.prog.NodeTypeOf(td.DeclName()); ok {
				ti.Node = int(c.str(NewFieldType(n.Underlying()).Text))
			}
			for _, f := range DefFields(spec) {
				ti.Fields = append(ti.Fields, int(c.str(f.Name)))
			}
			if z := c.prog.StructZeroOf(td.DeclName()); z != nil {
				for _, f := range z.Names {
					ti.Zeros = append(ti.Zeros, int(c.str(z.Zeros[f])))
				}
			}
			c.m.Types = append(c.m.Types, ti)
		}
	}
	if t := c.prog.Vars; t != nil {
		for _, d := range t.List {
			vi := bytecode.VarInfo{Name: int(c.str(d.Name)), Str: -1}
			switch v := d.Init.(type) {
			case int:
				vi.Type, vi.Int = bytecode.VarInt, v
			case string:
				vi.Type, vi.Str = bytecode.VarString, int(c.str(v))
			case bool:
				vi.Type, vi.Int = bytecode.VarBool, int(b2i(v))
			}
			c.m.Vars = append(c.m.Vars, vi)
		}
	}
	// Rule table (body positions are filled in later)
	all := append(append([]*Rule(nil), c.prog.RuleList...), c.prog.Twins...)
	for _, r := range all {
		ri := bytecode.RuleInfo{Name: int(c.str(r.Name)), Entry: -1, Action: -1, NodeType: -1, OperatorsRule: -1,
			Memo: r.Memo, Leader: r.Leader, Member: r.Member, Cycle: r.Cycle, Positional: r.Positional, NoValue: r.Novalue, Transient: r.Transient, Rec: r.Rec, Where: -1, Splice: r.Splice, Vars: c.strs2(r.Vars)}
		if r.Where != "" && r.Where != r.Name {
			ri.Where = int(c.str(r.Where))
		}
		if r.NodeType != "" {
			ri.NodeType = int(c.str(r.NodeType))
		} else if r.OpNodeType != "" {
			ri.NodeType = int(c.str(r.OpNodeType)) // that of the operator nodes of an Operators Rule
		}
		c.m.Rules = append(c.m.Rules, ri)
	}
	for i, r := range c.prog.RuleList {
		if slices.Contains(c.prog.StartRules, r.Name) {
			c.m.Starts = append(c.m.Starts, i)
		}
		if r.Name == c.prog.DefaultStartRule {
			c.m.DefaultStart = i
		}
	}
	for i, r := range all {
		c.cur = r
		ri := &c.m.Rules[i]
		s := NewScope()
		if pr, ok := r.Def.Expr.(*grammar.OperatorsRule); ok {
			ri.OperatorsRule = c.operatorsRule(pr)
			ri.Entry = int(c.here())
			c.emit(bytecode.OpOperatorsRule, int32(ri.OperatorsRule), 0, 0)
			c.emit(bytecode.OpReturn, 0, 0, 0)
		} else {
			ri.Entry = int(c.here())
			c.proj = RuleProjections(c.prog, r)
			if r.Lean {
				c.match(r.BodyExpr(), s, false)
			} else if r.Splice {
				c.spliceScope(r.BodyExpr(), s)
			} else {
				c.scope(r.BodyExpr(), s)
			}
			c.proj = nil
			c.emit(bytecode.OpReturn, 0, 0, 0)
		}
		ri.Scope = c.strs2(s.Names)
		if r.Action != nil {
			ri.Action = c.expr(r.Action, s, RuleProjections(c.prog, r))
		}
	}
}

// The value shapes of value_shape.go: each pushes values as the closure backend builds them.

// tuple emits code that pushes the tuple node of the elements of e.
func (c *bcompiler) tuple(e grammar.Expr, s *Scope) {
	c.emit(bytecode.OpPushPos, 0, 0, 0)
	c.elems(e, s)
	c.emit(bytecode.OpSeq, int32(ValuedCount(c.prog, e)), 0, 0)
}

// single emits code that pushes the value of e where it takes one element: nil without a value.
func (c *bcompiler) single(e grammar.Expr, s *Scope) {
	switch SingleModeOf(c.prog, e) {
	case singleNone:
		c.elems(e, s)
		c.emit(bytecode.OpPushNil, 0, 0, 0)
	case SinglePass:
		c.elems(e, s)
	default:
		c.tuple(e, s)
	}
}

// captured emits code that pushes the value of e where it is captured or recovered: the unit
// node of its range without a value.
func (c *bcompiler) captured(e grammar.Expr, s *Scope) {
	if SingleModeOf(c.prog, e) == SinglePass {
		c.elems(e, s)
		return
	}
	c.tuple(e, s)
}

// scope emits code that pushes the value of a scope with the body e.
func (c *bcompiler) scope(e grammar.Expr, s *Scope) {
	if ScopeTuple(c.prog, e) {
		c.tuple(e, s)
		return
	}
	c.elems(e, s)
}

// spliceScope emits code that pushes the value of the body e of a splice rule: the tuple node of
// its elements if e builds one, else its one element or nil, as OpSpread takes it.
func (c *bcompiler) spliceScope(e grammar.Expr, s *Scope) {
	switch {
	case IsElems(c.prog, e):
		c.tuple(e, s)
	case ValuedCount(c.prog, e) == 1:
		c.elems(e, s)
	default:
		c.elems(e, s)
		c.emit(bytecode.OpPushNil, 0, 0, 0)
	}
}

// class adds the character class to the character class table and returns its index.
// guard emits a GUARD for an alternative of a choice that must begin with a given terminal
// (firstTerminal) and returns its index, or -1. Its jump target is set by the caller.
func (c *bcompiler) guard(alt grammar.Expr) int {
	t, depth := firstTerminal(c.prog, alt, 0, map[string]bool{})
	var cl int32
	switch t := t.(type) {
	case *grammar.CharClass:
		cl = c.class(t)
	case *grammar.Literal:
		r, _ := utf8.DecodeRuneInString(t.Value)
		ranges := []rune{r, r}
		if t.IgnoreCase {
			// A case-insensitive literal's first character matches either case of an ASCII letter.
			f := FoldASCIIRune(r)
			ranges = []rune{f, f}
			if 'a' <= f && f <= 'z' {
				upper := f - ('a' - 'A')
				ranges = []rune{upper, upper, f, f}
			}
		}
		c.m.Classes = append(c.m.Classes, bytecode.Class{Ranges: ranges, Desc: int(c.str(LiteralDesc(t)))})
		cl = int32(len(c.m.Classes) - 1)
	default:
		return -1
	}
	return c.emit(bytecode.OpGuard, cl, int32(depth), 0)
}

func (c *bcompiler) class(e *grammar.CharClass) int32 {
	cl := bytecode.Class{Negated: e.Negated, Desc: int(c.str(CharClassString(e)))}
	for _, r := range e.Ranges {
		cl.Ranges = append(cl.Ranges, r.Lo, r.Hi)
	}
	c.m.Classes = append(c.m.Classes, cl)
	return int32(len(c.m.Classes) - 1)
}

// match emits code for the expression. If build is true, it pushes the elements of the expression
// (elems); otherwise it pushes nothing.
func (c *bcompiler) match(e grammar.Expr, s *Scope, build bool) {
	if build {
		c.elems(e, s)
		return
	}
	c.void(e, s)
}

// elems emits code that pushes the valuedCount(c.prog, e) elements of e, flattened through sequences.
func (c *bcompiler) elems(e grammar.Expr, s *Scope) {
	switch e := e.(type) {
	case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Ref, *grammar.Atomic:
		c.leaf(e, s, true)
	case *grammar.Seq:
		for _, it := range e.Items {
			c.elems(it, s)
		}
	case *grammar.Choice:
		valued := ValuedCount(c.prog, e) == 1
		var ends []int
		for i, alt := range e.Alts {
			emit := func() {
				if valued {
					c.single(alt, s)
				} else {
					c.elems(alt, s)
				}
			}
			if i == len(e.Alts)-1 {
				emit()
				break
			}
			guard := c.guard(alt)
			ch := c.emit(bytecode.OpChoice, 0, 0, 0)
			emit()
			ends = append(ends, c.emit(bytecode.OpCommit, 0, 0, 0))
			c.m.Code[ch].A = c.here()
			if guard >= 0 {
				c.m.Code[guard].C = c.here()
			}
		}
		for _, j := range ends {
			c.m.Code[j].A = c.here()
		}
	case *grammar.Repeat:
		c.repeat(e, s, true, false)
	case *grammar.Optional:
		valued := ValuedCount(c.prog, e) == 1
		ch := c.emit(bytecode.OpChoice, 0, 0, 0)
		if valued {
			c.single(e.Expr, s)
		} else {
			c.elems(e.Expr, s)
		}
		end := c.emit(bytecode.OpCommit, 0, 0, 0)
		c.m.Code[ch].A = c.here()
		if valued {
			c.emit(bytecode.OpPushNil, 0, 0, 0)
		}
		c.m.Code[end].A = c.here()
	case *grammar.Capture:
		c.capture(e, s, true)
	case *grammar.Attributed:
		c.attrElems(e, len(e.Attrs)-1, s)
	default:
		// The other expressions have no value: lookaheads keep only their captures.
		c.void(e, s)
	}
}

// leaf emits a literal, a character class, ., a rule call or #atomic, pushing its value if build.
func (c *bcompiler) leaf(e grammar.Expr, s *Scope, build bool) {
	switch e := e.(type) {
	case *grammar.Literal:
		if e.IgnoreCase {
			c.emit(bytecode.OpStrFold, c.str(string(FoldedRunes([]rune(e.Value)))), c.str(LiteralDesc(e)), b2i(build))
		} else {
			c.emit(bytecode.OpStr, c.str(e.Value), c.str(quote(e.Value)), b2i(build))
		}
	case *grammar.CharClass:
		c.emit(bytecode.OpClass, c.class(e), c.str(CharClassString(e)), b2i(build))
	case *grammar.Any:
		c.emit(bytecode.OpAny, b2i(build), 0, 0)
	case *grammar.Ref:
		r := c.prog.ByName[e.Name]
		min := 0
		if e.Level != "" {
			for i, l := range r.Def.Expr.(*grammar.OperatorsRule).Levels {
				if l.Name == e.Level {
					min = i
				}
			}
		}
		splice := r.Splice
		if !build && r.Twin != nil {
			r = r.Twin
		}
		c.emit(bytecode.OpCall, int32(r.ID), int32(min), b2i(build))
		if build && splice {
			// The elements of the rule are elements of the sequence that calls it.
			c.emit(bytecode.OpSpread, int32(ValuedCount(c.prog, e)), b2i(IsElems(c.prog, e)), 0)
		}
	case *grammar.Atomic:
		if build {
			c.emit(bytecode.OpPushPos, 0, 0, 0)
		}
		c.void(e.Expr, s)
		if build {
			c.emit(bytecode.OpAtomic, 1, 0, 0)
		}
	}
}

// void emits code for the expression without its value.
func (c *bcompiler) void(e grammar.Expr, s *Scope) {
	switch e := e.(type) {
	case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Ref, *grammar.Atomic:
		c.leaf(e, s, false)
	case *grammar.Seq:
		for _, it := range e.Items {
			c.void(it, s)
		}
	case *grammar.Choice:
		var ends []int
		for i, alt := range e.Alts {
			if i == len(e.Alts)-1 {
				c.void(alt, s)
				break
			}
			guard := c.guard(alt)
			ch := c.emit(bytecode.OpChoice, 0, 0, 0)
			c.void(alt, s)
			ends = append(ends, c.emit(bytecode.OpCommit, 0, 0, 0))
			c.m.Code[ch].A = c.here()
			if guard >= 0 {
				c.m.Code[guard].C = c.here()
			}
		}
		for _, j := range ends {
			c.m.Code[j].A = c.here()
		}
	case *grammar.Repeat:
		c.repeat(e, s, false, false)
	case *grammar.Optional:
		ch := c.emit(bytecode.OpChoice, 0, 0, 0)
		c.void(e.Expr, s)
		end := c.emit(bytecode.OpCommit, 0, 0, 0)
		c.m.Code[ch].A = c.here()
		c.m.Code[end].A = c.here()
	case *grammar.And:
		// The values inside are dropped by ENDLOOK; the captures are kept. C marks a lookahead that
		// can leave captures or bindings (LookKeepsState).
		keeps := int32(0)
		if LookKeepsState(e.Expr) {
			keeps = 1
		}
		c.emit(bytecode.OpLook, 0, 0, keeps)
		c.void(e.Expr, s)
		c.emit(bytecode.OpEndLook, 0, 0, 0)
	case *grammar.Not:
		look := c.emit(bytecode.OpLook, 0, 1, 0)
		c.void(e.Expr, s)
		c.emit(bytecode.OpEndLook, 1, 0, 0)
		c.m.Code[look].A = c.here()
	case *grammar.Discard:
		c.void(e.Expr, s)
	case *grammar.Capture:
		c.capture(e, s, false)
	case *grammar.Cut:
		c.emit(bytecode.OpCut, 0, 0, 0)
	case *grammar.Top:
		c.emit(bytecode.OpTop, 0, 0, 0)
	case *grammar.Bottom:
		c.emit(bytecode.OpFail, 0, 0, 0)
	case *grammar.BeginInput:
		c.emit(bytecode.OpAssert, 0, 0, 0)
	case *grammar.EndInput:
		c.emit(bytecode.OpAssert, 1, 0, 0)
	case *grammar.BeginLine:
		c.emit(bytecode.OpAssert, 2, 0, 0)
	case *grammar.EndLine:
		c.emit(bytecode.OpAssert, 3, 0, 0)
	case *grammar.Predicate:
		if a, ok := e.Term.(*grammar.Bind); ok {
			c.emit(bytecode.OpBind, int32(c.prog.Vars.Index(a.Name)), int32(c.expr(a.Value, s, nil)), 0)
		} else {
			c.emit(bytecode.OpPred, int32(c.expr(e.Term, s, nil)), 0, 0)
		}
	case *grammar.Attributed:
		c.attributed(e, len(e.Attrs)-1, s)
	}
}

// capture emits a capture, leaving its value pushed if keep.
func (c *bcompiler) capture(e *grammar.Capture, s *Scope, keep bool) {
	if pc := c.cur.PredCaps; !keep && pc != nil && !pc.Records(e) {
		// A capture that no predicate can read, in a program that builds no tree.
		if pc.Slotted(e) {
			s.Slot(e.Name)
		}
		c.void(e.Expr, s)
		return
	}
	slot := s.Slot(e.Name)
	if field := c.proj[e.Name]; field != "" {
		c.projectRepeat(e.Expr.(*grammar.Repeat), field)
	} else {
		c.captured(e.Expr, s)
	}
	c.emit(bytecode.OpCapture, int32(slot), b2i(!keep), 0)
}

func (c *bcompiler) repeat(e *grammar.Repeat, s *Scope, build, stream bool) {
	if site := c.prog.SkipSite(e); site != 0 { // as Walker.repeat: a site is neither a scan nor resumed
		free := b2i(c.prog.EnvFree(site)) << 1
		if int64(e.Min) <= 1<<31-1 {
			c.emit(bytecode.OpSkipRepeat, int32(e.Min), int32(site), free)
		} else {
			c.emit(bytecode.OpSkipRepeat, c.integer(e.Min), int32(site), 1|free)
		}
		loop := c.here()
		iter := c.emit(bytecode.OpIter, 0, 0, 0)
		c.void(e.Expr, s)
		c.emit(bytecode.OpNext, loop, 0, 0)
		c.m.Code[iter].A = c.here()
		c.emit(bytecode.OpEndRepeat, 0, 0, 0)
		return
	}
	if !build && !stream { // value-free repetition of a single character (same as Walker.scanRepeat)
		site := int32(0)
		if int64(e.Min) <= 1<<31-1 {
			site = int32(c.prog.ScanSite(e))
		}
		switch x := e.Expr.(type) {
		case *grammar.CharClass:
			if site != 0 {
				c.emit(bytecode.OpScanSite, c.class(x), int32(e.Min), site)
				return
			}
			c.repetition(bytecode.OpScan, e.Min, e.Max, c.class(x))
			return
		case *grammar.Any:
			if site != 0 {
				c.emit(bytecode.OpScanSite, -1, int32(e.Min), site)
				return
			}
			c.repetition(bytecode.OpScan, e.Min, e.Max, -1)
			return
		}
	}
	elemScope := s
	sc := int32(-1)
	if ElementScoped(e.Expr) {
		elemScope = NewScope()
		c.m.Scopes = append(c.m.Scopes, nil)
		sc = int32(len(c.m.Scopes) - 1)
	}
	if build {
		c.emit(bytecode.OpPushPos, 0, 0, 0)
	}
	c.repetition(bytecode.OpRepeat, e.Min, e.Max, sc)
	loop := c.here()
	run, runBuild := int32(0), int32(0)
	if !stream {
		if run = int32(c.prog.RunSite(e)); run != 0 {
			runBuild = b2i(build) | b2i(c.prog.EnvFree(int(run)))<<1
		}
	}
	if scan := c.prog.ScanSite(e); scan != 0 && !stream && run == 0 && sc < 0 {
		// A scan site (as Walker.plainRepeat): its walks list their elements if they build values.
		runBuild, run = 4|b2i(build), int32(scan)+b2i(build)
	}
	iter := c.emit(bytecode.OpIter, 0, runBuild, run)
	if build {
		c.scope(e.Expr, elemScope)
	} else {
		c.void(e.Expr, elemScope)
	}
	flag := b2i(build)
	if stream && build {
		flag = 2
	}
	c.emit(bytecode.OpNext, loop, flag, 0)
	c.m.Code[iter].A = c.here()
	c.emit(bytecode.OpEndRepeat, b2i(build), 0, 0)
	if sc >= 0 {
		c.m.Scopes[sc] = c.strs2(elemScope.Names)
	}
}

// attrElems emits code that pushes the elements of the expression with attributes up to i applied
// (inner attributes are applied first). What recovers is one element: its value or the error.
func (c *bcompiler) attrElems(e *grammar.Attributed, i int, s *Scope) {
	if i < 0 {
		if rep, ok := e.Expr.(*grammar.Repeat); ok && IsStream(e) {
			c.repeat(rep, s, true, true)
		} else {
			c.elems(e.Expr, s)
		}
		return
	}
	a := e.Attrs[i]
	switch a.Name {
	case "error", "diagnostic":
		c.label(a)
		c.attrElems(e, i-1, s)
		c.emit(bytecode.OpEndLabel, 0, 0, 0)
	case "recover":
		if ValuedCount(c.prog, e) == 0 {
			// An expression without a value stays without one: its error is only recorded.
			c.attributed(e, i, s)
			return
		}
		skip, _ := a.Arg("skip")
		rec := c.emit(bytecode.OpRecover, 0, 0, 0)
		// The value of the inner expression where it is captured (captured), as in captureMatcher.
		mode := SingleModeOf(c.prog, e.Expr)
		if hasRecover(e.Attrs[:i]) {
			mode = SinglePass // an inner recovery is one element already
		}
		if mode != SinglePass {
			c.emit(bytecode.OpPushPos, 0, 0, 0)
		}
		c.attrElems(e, i-1, s)
		if mode != SinglePass {
			n := 0
			if mode == SingleTuple {
				n = ValuedCount(c.prog, e.Expr)
			}
			c.emit(bytecode.OpSeq, int32(n), 0, 0)
		}
		end := c.emit(bytecode.OpEndRecover, 0, 0, 0)
		c.m.Code[rec].A = c.here()
		c.void(skip, s)
		c.emit(bytecode.OpEndSkip, 1, 0, 0)
		c.m.Code[end].A = c.here()
	default: // #stream is handled by the repetition
		c.attrElems(e, i-1, s)
	}
}

// hasRecover reports whether attrs has #recover.
func hasRecover(attrs []*grammar.Attribute) bool {
	for _, a := range attrs {
		if a.Name == "recover" {
			return true
		}
	}
	return false
}

// label emits the LABEL of @error or @diagnostic.
func (c *bcompiler) label(a *grammar.Attribute) {
	if a.Name == "error" {
		c.emit(bytecode.OpLabel, c.str(desugar.AttributeMessage(a)), 0, 0)
	} else {
		c.emit(bytecode.OpLabel, c.str(desugar.AttributeDiagnosticName(a)), 1, 0)
	}
}

// attributed emits code for the expression with attributes up to i applied (inner attributes
// are applied first), without its value.
func (c *bcompiler) attributed(e *grammar.Attributed, i int, s *Scope) {
	if i < 0 {
		if rep, ok := e.Expr.(*grammar.Repeat); ok && IsStream(e) {
			c.repeat(rep, s, false, true)
		} else {
			c.void(e.Expr, s)
		}
		return
	}
	a := e.Attrs[i]
	switch a.Name {
	case "error", "diagnostic":
		c.label(a)
		c.attributed(e, i-1, s)
		c.emit(bytecode.OpEndLabel, 0, 0, 0)
	case "recover":
		skip, _ := a.Arg("skip")
		rec := c.emit(bytecode.OpRecover, 0, 0, 0)
		c.attributed(e, i-1, s)
		end := c.emit(bytecode.OpEndRecover, 0, 0, 0)
		c.m.Code[rec].A = c.here()
		c.void(skip, s)
		c.emit(bytecode.OpEndSkip, 0, 0, 0)
		c.m.Code[end].A = c.here()
	default: // #stream is handled by the repetition
		c.attributed(e, i-1, s)
	}
}

func (c *bcompiler) operatorsRule(e *grammar.OperatorsRule) int {
	pi := bytecode.OperatorsRuleInfo{Skip: -1}
	if e.Skip != nil {
		pi.Skip = int(c.here())
		c.match(e.Skip, NewScope(), false)
		c.emit(bytecode.OpEnd, 0, 0, 0)
	}
	for _, o := range e.Operands {
		pi.Operands = append(pi.Operands, c.line(o.Expr, o.Action, nil, o))
	}
	id := 0
	for i, l := range e.Levels {
		for _, op := range l.Operators {
			oi := bytecode.OpInfo{ID: id, Level: i + 1}
			id++
			var locals []string
			switch op.Kind {
			case grammar.Prefix:
				oi.Kind = bytecode.OperatorPrefix
			case grammar.Postfix:
				oi.Kind = bytecode.OperatorPostfix
			default:
				oi.Kind = bytecode.OperatorInfix
			}
			switch op.Assoc {
			case grammar.AssocRight:
				oi.Assoc = bytecode.AssocRight
			case grammar.AssocNone:
				oi.Assoc = bytecode.AssocNone
			}
			// Locals of an operator action: 0 $lhs, 1 $rhs, 2 $op
			locals = []string{"lhs", "rhs", "op"}
			oi.Line = c.line(op.Expr, op.Action, locals, op)
			if oi.Kind == bytecode.OperatorPrefix {
				pi.Prefix = append(pi.Prefix, oi)
			} else {
				pi.Led = append(pi.Led, oi)
			}
		}
	}
	c.m.OperatorsRules = append(c.m.OperatorsRules, pi)
	return len(c.m.OperatorsRules) - 1
}

func (c *bcompiler) line(e grammar.Expr, action grammar.Term, locals []string, key any) bytecode.LineInfo {
	s := NewScope()
	li := bytecode.LineInfo{Entry: int(c.here()), Action: -1}
	switch build := !c.cur.Novalue && !LeanLine(action, locals); {
	case !build:
		c.void(e, s)
	case locals == nil: // an operand line: the value of its scope
		c.scope(e, s)
	default: // an operator line: the operator's value, the node of its tuple if IsSeq
		li.IsSeq = SingleModeOf(c.prog, e) == SingleTuple
		c.captured(e, s)
	}
	c.emit(bytecode.OpEnd, 0, 0, 0)
	li.Scope = c.strs2(s.Names)
	if action != nil && !c.cur.Novalue {
		li.Action = c.expr(action, s, nil)
	}
	return li
}

// expr adds the term t, which reads the frame of the scope s, to the module and returns its index.
func (c *bcompiler) expr(t grammar.Term, s *Scope, proj map[string]string) int {
	c.terms = append(c.terms, TermSite{Term: t, Scope: s, Proj: proj})
	c.m.Terms = append(c.m.Terms, bytecode.Term{})
	return len(c.m.Terms) - 1
}

// projectRepeat emits a repetition that gathers the values of the capture field of its elements
// (project.go): the elements are matched without values, and NEXT pushes the field's slot.
func (c *bcompiler) projectRepeat(e *grammar.Repeat, field string) {
	elemScope := NewScope()
	c.m.Scopes = append(c.m.Scopes, nil)
	sc := int32(len(c.m.Scopes) - 1)
	c.emit(bytecode.OpPushPos, 0, 0, 0)
	c.repetition(bytecode.OpRepeat, e.Min, e.Max, sc)
	loop := c.here()
	iter := c.emit(bytecode.OpIter, 0, 0, 0)
	c.match(e.Expr, elemScope, false)
	c.emit(bytecode.OpNext, loop, 3, int32(elemScope.Slots[field]))
	c.m.Code[iter].A = c.here()
	c.emit(bytecode.OpEndRepeat, 1, 0, 0)
	c.m.Scopes[sc] = c.strs2(elemScope.Names)
}
