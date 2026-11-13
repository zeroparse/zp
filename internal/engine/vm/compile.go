package vm

import (
	"fmt"
	"slices"
	"strconv"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
)

// Compile compiles the program prog to a bytecode module: it builds the program's Core IR, runs
// the bytecode pipeline on it (the choice guards and scans; the VMs have no instruction for class
// runs or terminal regions) and emits the match code of the module's rules and the term code of
// its terms from the result. A failure is an internal error: the program compiled, so its Core IR
// and its terms build. A program that uses #missing has no bytecode form.
func Compile(prog *compile.Program) (*bytecode.Module, error) {
	if prog.HasMissing {
		return nil, compile.MissingUnsupported("bytecode module construction")
	}
	m, err := build.CoreControlled(prog, nil)
	if err != nil {
		return nil, fmt.Errorf("internal error: Core IR: %w", err)
	}
	if _, err := ir.Run(m, &ir.Env{}, ir.RunOptions{}); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	return CompileCore(prog, m)
}

// CompileCore compiles the program prog, whose Core IR after the bytecode pipeline is m, to a
// bytecode module (Compile).
func CompileCore(prog *compile.Program, m *ir.Module) (*bytecode.Module, error) {
	c := &compiler{prog: prog, core: m, m: &bytecode.Module{SourceInfo: prog.RecordsSourceInfo, DefaultValue: prog.MakesDefaultValue}, strs: map[string]int{}}
	c.compile()
	if c.err != nil {
		return nil, c.err
	}
	// The term code comes after the match code, so the strings the terms add follow those of the
	// rules.
	for i, s := range c.terms {
		tp, err := state.CompileTerm(s.term, state.TermEnv{Prog: prog, Scope: scopeOf(s.scope), Proj: s.proj})
		if err != nil {
			return nil, fmt.Errorf("internal error: %w", err)
		}
		if c.m.Terms[i], err = tp.Export(func(s string) int { return int(c.str(s)) }); err != nil {
			return nil, fmt.Errorf("internal error: %w", err)
		}
	}
	return c.m, nil
}

// compiler emits a bytecode module from Core IR (CompileCore): m is the module it fills and strs
// indexes the module's string table.
type compiler struct {
	prog *compile.Program
	core *ir.Module
	m    *bytecode.Module
	strs map[string]int
	// terms are the terms of the module, by index of Module.Terms, whose code CompileCore makes
	// once the match code is emitted.
	terms []termSite
	// scope holds the capture slots of the frame the code being emitted writes.
	scope []ir.Slot
	err   error
}

// termSite is a term of the module: the Core IR term, the slots of the frame it reads, and the
// projected captures of the rule whose action it is (nil for every other term).
type termSite struct {
	term  ir.Term
	scope []ir.Slot
	proj  map[string]string
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

func (c *compiler) failf(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf("internal error: "+format, args...)
	}
}

func (c *compiler) str(s string) int32 {
	if i, ok := c.strs[s]; ok {
		return int32(i)
	}
	c.strs[s] = len(c.m.Strings)
	c.m.Strings = append(c.m.Strings, s)
	return int32(len(c.m.Strings) - 1)
}

func (c *compiler) strList(names []string) []int {
	out := make([]int, len(names))
	for i, n := range names {
		out[i] = int(c.str(n))
	}
	return out
}

// exp returns the string of the expectation id of the Core IR module.
func (c *compiler) exp(id ir.ExpID) int32 { return c.str(c.core.Exps[id]) }

func (c *compiler) emit(op bytecode.Op, a, b, cc int32) int {
	c.m.Code = append(c.m.Code, bytecode.Instr{Op: op, A: a, B: b, C: cc})
	return len(c.m.Code) - 1
}

func (c *compiler) here() int32 { return int32(len(c.m.Code)) }

// integer adds a wide integer to the module and returns its index.
func (c *compiler) integer(value int) int32 {
	c.m.Ints = append(c.m.Ints, int64(value))
	return int32(len(c.m.Ints) - 1)
}

// fits reports that n is an operand of an instruction.
func fits(n int) bool { return int64(n) <= 1<<31-1 }

func (c *compiler) repetition(op bytecode.Op, min, max int, other int32) {
	// Any negative maximum is unbounded. Canonicalize before narrowing so that large negative
	// values cannot wrap into finite bounds.
	if max < 0 {
		max = -1
	}
	if fits(min) && fits(max) {
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

// compile fills the module with the type, variable and rule tables and the code of the rules.
func (c *compiler) compile() {
	for _, td := range c.prog.Grammar.Types() {
		spec, ok := td.(*grammar.TypeDef)
		if !ok {
			continue
		}
		ti := bytecode.TypeInfo{Name: int(c.str(td.DeclName())), Node: -1}
		if n, ok := c.prog.NodeTypeOf(td.DeclName()); ok {
			ti.Node = int(c.str(compile.NewFieldType(n.Underlying()).Text))
		}
		for _, f := range compile.DefFields(spec) {
			ti.Fields = append(ti.Fields, int(c.str(f.Name)))
		}
		if z := c.prog.StructZeroOf(td.DeclName()); z != nil {
			for _, f := range z.Names {
				ti.Zeros = append(ti.Zeros, int(c.str(z.Zeros[f])))
			}
		}
		c.m.Types = append(c.m.Types, ti)
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
	// The rule table; the entries of the bodies are filled in below.
	all := append(append([]*compile.Rule(nil), c.prog.RuleList...), c.prog.Twins...)
	if len(all) != len(c.core.Rules) {
		c.failf("%d rules in Core IR, %d in the program", len(c.core.Rules), len(all))
		return
	}
	for _, r := range all {
		ri := bytecode.RuleInfo{Name: int(c.str(r.Name)), Entry: -1, Action: -1, NodeType: -1, OperatorsRule: -1,
			Memo: r.Memo, Leader: r.Leader, Member: r.Member, Cycle: r.Cycle, Positional: r.Positional, NoValue: r.Novalue,
			Transient: r.Transient, Rec: r.Rec, Where: -1, Splice: r.Splice, Vars: c.strList(r.Vars)}
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
	for i, cr := range c.core.Rules {
		ri := &c.m.Rules[i]
		if o, ok := cr.Body.(*ir.OperatorsRule); ok {
			ri.OperatorsRule = c.operatorsRule(o)
			ri.Entry = int(c.here())
			c.emit(bytecode.OpOperatorsRule, int32(ri.OperatorsRule), 0, 0)
			c.emit(bytecode.OpReturn, 0, 0, 0)
			c.scope = nil
		} else {
			ri.Entry = int(c.here())
			c.scope = cr.Scope
			switch {
			case !cr.Want:
				c.void(cr.Body)
			case cr.Policy.Splice:
				c.spliceScope(cr.Body, cr.Finish.Shape)
			default:
				c.scopeValue(cr.Body, cr.Finish.Shape)
			}
			c.emit(bytecode.OpReturn, 0, 0, 0)
		}
		ri.Scope = c.strList(ir.SlotNames(cr.Scope))
		if cr.Finish.Kind == ir.FinishAction {
			var proj map[string]string
			if _, ok := cr.Body.(*ir.OperatorsRule); !ok {
				proj = projections(cr.Body)
			}
			ri.Action = c.term(cr.Finish.Action, proj)
		}
	}
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

// term adds the term t, which reads the frame of the current scope, to the module and returns its
// index; proj are the projected captures of the rule whose action t is.
func (c *compiler) term(t ir.Term, proj map[string]string) int {
	c.terms = append(c.terms, termSite{term: t, scope: c.scope, proj: proj})
	c.m.Terms = append(c.m.Terms, bytecode.Term{})
	return len(c.m.Terms) - 1
}

// The value shapes: each pushes values as the closure backend builds them.

// tuple emits code that pushes the tuple node of the elements of n, which have the shape sh.
func (c *compiler) tuple(n ir.Node, sh ir.Shape) {
	c.emit(bytecode.OpPushPos, 0, 0, 0)
	c.elems(n)
	c.emit(bytecode.OpSeq, int32(sh.K), 0, 0)
}

// single emits code that pushes the value of n where it takes one element: nil without a value.
func (c *compiler) single(n ir.Node) {
	sh := n.Attrs().Shape
	switch sh.Mode {
	case ir.SingleNone:
		c.elems(n)
		c.emit(bytecode.OpPushNil, 0, 0, 0)
	case ir.SinglePass:
		c.elems(n)
	default:
		c.tuple(n, sh)
	}
}

// captured emits code that pushes the value of n where it is captured or recovered: the unit node
// of its range without a value.
func (c *compiler) captured(n ir.Node) {
	sh := n.Attrs().Shape
	if sh.Mode == ir.SinglePass {
		c.elems(n)
		return
	}
	c.tuple(n, sh)
}

// scopeValue emits code that pushes the value of a scope with the body n of the shape sh.
func (c *compiler) scopeValue(n ir.Node, sh ir.Shape) {
	if sh.Tuple {
		c.tuple(n, sh)
		return
	}
	c.elems(n)
}

// spliceScope emits code that pushes the value of the body n, of the shape sh, of a splice rule:
// the tuple node of its elements if it builds one, else its one element or nil, as SPREAD takes
// it.
func (c *compiler) spliceScope(n ir.Node, sh ir.Shape) {
	switch {
	case sh.Elems:
		c.tuple(n, sh)
	case sh.K == 1:
		c.elems(n)
	default:
		c.elems(n)
		c.emit(bytecode.OpPushNil, 0, 0, 0)
	}
}

// class adds the character class to the character class table and returns its index.
func (c *compiler) class(cl *ir.Class) int32 {
	bc := bytecode.Class{Negated: cl.Negated, Desc: int(c.exp(cl.Exp))}
	for _, r := range cl.Ranges {
		bc.Ranges = append(bc.Ranges, r.Lo, r.Hi)
	}
	c.m.Classes = append(c.m.Classes, bc)
	return int32(len(c.m.Classes) - 1)
}

// guard emits the GUARD of an alternative whose first terminal is f and returns its index, or -1
// for an alternative without one. Its jump target is set by the caller.
func (c *compiler) guard(f *ir.First) int {
	if f == nil {
		return -1
	}
	var cl int32
	if f.Lit {
		ranges := []rune{f.Rune, f.Rune}
		if f.Fold {
			// The first character of a case-insensitive literal is either case of the letter.
			upper := f.Rune - ('a' - 'A')
			ranges = []rune{upper, upper, f.Rune, f.Rune}
		}
		c.m.Classes = append(c.m.Classes, bytecode.Class{Ranges: ranges, Desc: int(c.exp(f.Exp))})
		cl = int32(len(c.m.Classes) - 1)
	} else {
		cl = c.class(f.Class)
	}
	return c.emit(bytecode.OpGuard, cl, int32(f.Depth), 0)
}

// elems emits code that pushes the valued elements of n, flattened through sequences.
func (c *compiler) elems(n ir.Node) {
	switch n := n.(type) {
	case *ir.Lit, *ir.ClassNode, *ir.Any, *ir.Call, *ir.Text:
		c.leaf(n, true)
	case *ir.Seq:
		for _, it := range n.Items {
			c.elems(it)
		}
	case *ir.Choice:
		c.choice(n, true)
	case *ir.Repeat:
		c.repeat(n, true, false)
	case *ir.Stream:
		c.repeat(n.Repeat, true, true)
	case *ir.Opt:
		valued := n.Attrs().Shape.K == 1
		ch := c.emit(bytecode.OpChoice, 0, 0, 0)
		if valued {
			c.single(n.Body)
		} else {
			c.elems(n.Body)
		}
		end := c.emit(bytecode.OpCommit, 0, 0, 0)
		c.m.Code[ch].A = c.here()
		if valued {
			c.emit(bytecode.OpPushNil, 0, 0, 0)
		}
		c.m.Code[end].A = c.here()
	case *ir.Capture:
		c.capture(n, true)
	case *ir.Label:
		c.label(n)
		c.elems(n.Body)
		c.emit(bytecode.OpEndLabel, 0, 0, 0)
	case *ir.Recover:
		if !n.Attrs().Want || n.Attrs().Shape.K == 0 {
			// An expression without a value stays without one: its error is only recorded.
			c.void(n)
			return
		}
		rec := c.emit(bytecode.OpRecover, 0, 0, 0)
		// The value of the inner expression where it is captured, as captured makes it.
		sh := n.Body.Attrs().Shape
		if sh.Mode != ir.SinglePass {
			c.emit(bytecode.OpPushPos, 0, 0, 0)
		}
		c.elems(n.Body)
		if sh.Mode != ir.SinglePass {
			k := 0
			if sh.Mode == ir.SingleTuple {
				k = sh.K
			}
			c.emit(bytecode.OpSeq, int32(k), 0, 0)
		}
		end := c.emit(bytecode.OpEndRecover, 0, 0, 0)
		c.m.Code[rec].A = c.here()
		c.void(n.Skip)
		c.emit(bytecode.OpEndSkip, 1, 0, 0)
		c.m.Code[end].A = c.here()
	default:
		// The other expressions have no value: lookaheads keep only their captures.
		c.void(n)
	}
}

// choice emits a choice, pushing the value of the alternative that matches if build.
func (c *compiler) choice(n *ir.Choice, build bool) {
	valued := build && n.Attrs().Shape.K == 1
	var ends []int
	for i, alt := range n.Alts {
		body := func() {
			switch {
			case valued:
				c.single(alt.Body)
			case build:
				c.elems(alt.Body)
			default:
				c.void(alt.Body)
			}
		}
		if i == len(n.Alts)-1 {
			body()
			break
		}
		guard := c.guard(alt.Guard)
		ch := c.emit(bytecode.OpChoice, 0, 0, 0)
		body()
		ends = append(ends, c.emit(bytecode.OpCommit, 0, 0, 0))
		c.m.Code[ch].A = c.here()
		if guard >= 0 {
			c.m.Code[guard].C = c.here()
		}
	}
	for _, j := range ends {
		c.m.Code[j].A = c.here()
	}
}

// leaf emits a literal, a character class, ., a rule call or #atomic, pushing its value if build.
func (c *compiler) leaf(n ir.Node, build bool) {
	switch n := n.(type) {
	case *ir.Lit:
		// The operands are interned in order: the text, then the expectation.
		if n.Fold {
			c.emit(bytecode.OpStrFold, c.str(string(compile.FoldedRunes([]rune(n.Text)))), c.litDesc(n), b2i(build))
		} else {
			c.emit(bytecode.OpStr, c.str(n.Text), c.litDesc(n), b2i(build))
		}
	case *ir.ClassNode:
		c.emit(bytecode.OpClass, c.class(n.Class), c.exp(n.Class.Exp), b2i(build))
	case *ir.Any:
		c.emit(bytecode.OpAny, b2i(build), 0, 0)
	case *ir.Call:
		callee := c.core.Rules[n.Rule]
		if !build && callee.Twin != nil {
			// A call whose value is dropped runs the value-free twin. Core IR keeps the rule where
			// the value of an enclosing expression is wanted only for its captures, as in a
			// lookahead that captures.
			callee = callee.Twin
		}
		c.emit(bytecode.OpCall, int32(callee.ID), int32(n.Level), b2i(build))
		if build && callee.Policy.Splice {
			// The elements of the rule are elements of the sequence that calls it.
			sh := n.Attrs().Shape
			c.emit(bytecode.OpSpread, int32(sh.K), b2i(sh.Elems), 0)
		}
	case *ir.Text:
		if build {
			c.emit(bytecode.OpPushPos, 0, 0, 0)
		}
		c.void(n.Body)
		if build {
			c.emit(bytecode.OpAtomic, 1, 0, 0)
		}
	}
}

// litDesc returns the string of the expectation of the literal n. The empty literal, which
// cannot fail, records none in Core IR; its operand is the quoted empty text.
func (c *compiler) litDesc(n *ir.Lit) int32 {
	if n.Text == "" {
		return c.str(strconv.Quote(""))
	}
	return c.exp(n.Exp)
}

// void emits code for the expression without its value.
func (c *compiler) void(n ir.Node) {
	switch n := n.(type) {
	case *ir.Lit, *ir.ClassNode, *ir.Any, *ir.Call, *ir.Text:
		c.leaf(n, false)
	case *ir.Seq:
		for _, it := range n.Items {
			c.void(it)
		}
	case *ir.Choice:
		c.choice(n, false)
	case *ir.Repeat:
		c.repeat(n, false, false)
	case *ir.Stream:
		c.repeat(n.Repeat, false, true)
	case *ir.Scan:
		c.scan(n)
	case *ir.Opt:
		ch := c.emit(bytecode.OpChoice, 0, 0, 0)
		c.void(n.Body)
		end := c.emit(bytecode.OpCommit, 0, 0, 0)
		c.m.Code[ch].A = c.here()
		c.m.Code[end].A = c.here()
	case *ir.Look:
		if n.Neg {
			look := c.emit(bytecode.OpLook, 0, 1, 0)
			c.void(n.Body)
			c.emit(bytecode.OpEndLook, 1, 0, 0)
			c.m.Code[look].A = c.here()
			return
		}
		// The values inside are dropped by ENDLOOK; the captures are kept. C marks a lookahead that
		// can leave captures or bindings, which records no observation (Site 0).
		c.emit(bytecode.OpLook, 0, 0, b2i(n.Site == 0))
		c.void(n.Body)
		c.emit(bytecode.OpEndLook, 0, 0, 0)
	case *ir.Drop:
		c.void(n.Body)
	case *ir.Capture:
		c.capture(n, false)
	case *ir.Cut:
		c.emit(bytecode.OpCut, 0, 0, 0)
	case *ir.Succeed:
		c.emit(bytecode.OpTop, 0, 0, 0)
	case *ir.FailNode:
		c.emit(bytecode.OpFail, 0, 0, 0)
	case *ir.Anchor:
		c.emit(bytecode.OpAssert, int32(n.Kind), 0, 0)
	case *ir.Pred:
		c.emit(bytecode.OpPred, int32(c.term(n.Term, nil)), 0, 0)
	case *ir.BindNode:
		c.emit(bytecode.OpBind, int32(c.prog.Vars.Index(n.Var)), int32(c.term(n.Term, nil)), 0)
	case *ir.Label:
		c.label(n)
		c.void(n.Body)
		c.emit(bytecode.OpEndLabel, 0, 0, 0)
	case *ir.Recover:
		rec := c.emit(bytecode.OpRecover, 0, 0, 0)
		c.void(n.Body)
		end := c.emit(bytecode.OpEndRecover, 0, 0, 0)
		c.m.Code[rec].A = c.here()
		c.void(n.Skip)
		c.emit(bytecode.OpEndSkip, 0, 0, 0)
		c.m.Code[end].A = c.here()
	default:
		c.failf("Core IR node %T in a bytecode program", n)
	}
}

// label emits the LABEL of @error or @diagnostic.
func (c *compiler) label(n *ir.Label) {
	c.emit(bytecode.OpLabel, c.exp(n.Exp), b2i(n.Diagnostic), 0)
}

// capture emits a capture, leaving its value pushed if keep.
func (c *compiler) capture(n *ir.Capture, keep bool) {
	if n.Project != "" {
		rep, ok := n.Body.(*ir.Repeat)
		if !ok {
			c.failf("projected capture %s of %T", n.Name, n.Body)
			return
		}
		c.projectRepeat(rep, n.Project)
	} else {
		c.captured(n.Body)
	}
	c.emit(bytecode.OpCapture, int32(n.Slot), b2i(!keep), 0)
}

// scan emits a value-free repetition of one character.
func (c *compiler) scan(n *ir.Scan) {
	cl := int32(-1)
	if n.Class != nil {
		cl = c.class(n.Class)
	}
	if n.Site != 0 && fits(n.Min) {
		c.emit(bytecode.OpScanSite, cl, int32(n.Min), int32(n.Site))
		return
	}
	c.repetition(bytecode.OpScan, n.Min, n.Max, cl)
}

// repeat emits a repetition, pushing its value if build; stream marks the repetition of #stream.
func (c *compiler) repeat(n *ir.Repeat, build, stream bool) {
	if n.Site != 0 { // a skip-run site: neither a scan nor resumed
		free := b2i(c.prog.EnvFree(n.Site)) << 1
		if fits(n.Min) {
			c.emit(bytecode.OpSkipRepeat, int32(n.Min), int32(n.Site), free)
		} else {
			c.emit(bytecode.OpSkipRepeat, c.integer(n.Min), int32(n.Site), 1|free)
		}
		loop := c.here()
		iter := c.emit(bytecode.OpIter, 0, 0, 0)
		c.void(n.Body)
		c.emit(bytecode.OpNext, loop, 0, 0)
		c.m.Code[iter].A = c.here()
		c.emit(bytecode.OpEndRepeat, 0, 0, 0)
		return
	}
	outer := c.scope
	sc := int32(-1)
	if n.Scope != nil {
		c.m.Scopes = append(c.m.Scopes, nil)
		sc = int32(len(c.m.Scopes) - 1)
		c.scope = n.Scope
	}
	if build {
		c.emit(bytecode.OpPushPos, 0, 0, 0)
	}
	c.repetition(bytecode.OpRepeat, n.Min, n.Max, sc)
	loop := c.here()
	run, runBuild := int32(0), int32(0)
	if !stream {
		if run = int32(n.Run); run != 0 {
			runBuild = b2i(build) | b2i(c.prog.EnvFree(n.Run))<<1
		}
	}
	if n.ScanSite != 0 && !stream && run == 0 && sc < 0 {
		// A scan site: its walks list their elements if they build values.
		runBuild, run = 4|b2i(build), int32(n.ScanSite)+b2i(build)
	}
	iter := c.emit(bytecode.OpIter, 0, runBuild, run)
	if build {
		c.scopeValue(n.Body, n.Body.Attrs().Shape)
	} else {
		c.void(n.Body)
	}
	flag := b2i(build)
	if stream && build {
		flag = 2
	}
	c.emit(bytecode.OpNext, loop, flag, 0)
	c.m.Code[iter].A = c.here()
	c.emit(bytecode.OpEndRepeat, b2i(build), 0, 0)
	if sc >= 0 {
		c.m.Scopes[sc] = c.strList(ir.SlotNames(n.Scope))
		c.scope = outer
	}
}

// projectRepeat emits a repetition that gathers the values of the capture field of its elements:
// the elements are matched without values, and NEXT pushes the field's slot.
func (c *compiler) projectRepeat(n *ir.Repeat, field string) {
	c.m.Scopes = append(c.m.Scopes, nil)
	sc := int32(len(c.m.Scopes) - 1)
	slot := slices.IndexFunc(n.Scope, func(s ir.Slot) bool { return s.Name == field })
	if slot < 0 {
		c.failf("projected field %s is not captured", field)
		return
	}
	outer := c.scope
	c.scope = n.Scope
	c.emit(bytecode.OpPushPos, 0, 0, 0)
	c.repetition(bytecode.OpRepeat, n.Min, n.Max, sc)
	loop := c.here()
	iter := c.emit(bytecode.OpIter, 0, 0, 0)
	c.void(n.Body)
	c.emit(bytecode.OpNext, loop, 3, int32(slot))
	c.m.Code[iter].A = c.here()
	c.emit(bytecode.OpEndRepeat, 1, 0, 0)
	c.m.Scopes[sc] = c.strList(ir.SlotNames(n.Scope))
	c.scope = outer
}

func (c *compiler) operatorsRule(o *ir.OperatorsRule) int {
	pi := bytecode.OperatorsRuleInfo{Skip: -1}
	if o.Skip != nil {
		pi.Skip = int(c.here())
		c.scope = o.Skip.Scope
		c.void(o.Skip.Body)
		c.emit(bytecode.OpEnd, 0, 0, 0)
	}
	for _, l := range o.Operands {
		pi.Operands = append(pi.Operands, c.line(l, false))
	}
	for _, op := range o.Ops {
		oi := bytecode.OpInfo{ID: op.ID, Level: op.Level}
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
		oi.Line = c.line(op.Line, true)
		if oi.Kind == bytecode.OperatorPrefix {
			pi.Prefix = append(pi.Prefix, oi)
		} else {
			pi.Led = append(pi.Led, oi)
		}
	}
	c.m.OperatorsRules = append(c.m.OperatorsRules, pi)
	return len(c.m.OperatorsRules) - 1
}

// line emits a line of an Operators Rule, an operator line if operator.
func (c *compiler) line(l *ir.OpLine, operator bool) bytecode.LineInfo {
	c.scope = l.Scope
	li := bytecode.LineInfo{Entry: int(c.here()), Action: -1}
	switch {
	case !l.Body.Attrs().Want:
		c.void(l.Body)
	case !operator: // an operand line: the value of its scope
		c.scopeValue(l.Body, l.Shape)
	default: // an operator line: the operator's value, the node of its tuple if IsSeq
		li.IsSeq = l.Shape.Mode == ir.SingleTuple
		c.captured(l.Body)
	}
	c.emit(bytecode.OpEnd, 0, 0, 0)
	li.Scope = c.strList(ir.SlotNames(l.Scope))
	if l.Action != nil {
		li.Action = c.term(l.Action, nil)
	}
	return li
}
