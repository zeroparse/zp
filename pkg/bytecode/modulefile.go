package bytecode

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Reading and writing modules and validating loaded modules.

func (w *Encoder) ints(xs []int) {
	w.Uint(len(xs))
	for _, x := range xs {
		w.Int(x)
	}
}

func (w *Encoder) code(code []Instr) {
	w.Uint(len(code))
	for _, in := range code {
		w.Byte(byte(in.Op))
		w.Int(int(in.A))
		w.Int(int(in.B))
		w.Int(int(in.C))
	}
}

func (w *Encoder) line(l *LineInfo) {
	w.Int(l.Entry)
	w.ints(l.Scope)
	w.Int(l.Action)
	w.Bool(l.IsSeq)
}

// WriteModule writes m.
func (w *Encoder) WriteModule(m *Module) {
	w.Uint(len(m.Strings))
	w.Bool(m.SourceInfo)
	w.Bool(m.DefaultValue)
	w.Uint(len(m.Classes))
	for _, c := range m.Classes {
		w.Bool(c.Negated)
		w.Int(c.Desc)
		w.Uint(len(c.Ranges) / 2)
		for _, r := range c.Ranges {
			w.Int(int(r))
		}
	}
	w.Uint(len(m.Types))
	for _, t := range m.Types {
		w.Int(t.Name)
		w.ints(t.Fields)
		w.ints(t.Zeros)
		w.Int(t.Node)
	}
	w.Uint(len(m.Scopes))
	for _, l := range m.Scopes {
		w.ints(l)
	}
	w.Uint(len(m.Vars))
	for _, v := range m.Vars {
		w.Int(v.Name)
		w.Int(v.Type)
		w.Int(v.Int)
		w.Int(v.Str)
	}
	w.Uint(len(m.Rules))
	for _, r := range m.Rules {
		w.Int(r.Name)
		w.Int(r.Entry)
		w.ints(r.Scope)
		w.Int(r.Action)
		w.Int(r.NodeType)
		w.Bool(r.Memo)
		w.Bool(r.Leader)
		w.Bool(r.Member)
		w.Int(r.Cycle)
		w.Bool(r.Positional)
		w.Int(r.OperatorsRule)
		w.Bool(r.NoValue)
		w.Bool(r.Transient)
		w.Bool(r.Rec)
		w.Int(r.Where)
		w.Bool(r.Splice)
		w.ints(r.Vars)
	}
	w.ints(m.Starts)
	w.Int(m.DefaultStart)
	w.Uint(len(m.OperatorsRules))
	for _, p := range m.OperatorsRules {
		w.Int(p.Skip)
		w.Uint(len(p.Operands))
		for i := range p.Operands {
			w.line(&p.Operands[i])
		}
		for _, ops := range [][]OpInfo{p.Prefix, p.Led} {
			w.Uint(len(ops))
			for i := range ops {
				o := &ops[i]
				w.Int(o.ID)
				w.Int(o.Kind)
				w.Int(o.Assoc)
				w.Int(o.Level)
				w.line(&o.Line)
			}
		}
	}
	w.code(m.Code)
	w.Uint(len(m.Ints))
	for _, x := range m.Ints {
		w.Body.Write(binary.AppendVarint(nil, x))
	}
	w.Uint(len(m.Terms))
	for i := range m.Terms {
		w.term(&m.Terms[i])
	}
	w.Uint(len(m.RunKeys))
	for _, k := range m.RunKeys {
		w.Int(k.Site)
		w.ints(k.Vars)
		w.ints(k.Bind)
	}
	rules := slices.Sorted(maps.Keys(m.Flows))
	w.Uint(len(rules))
	for _, rule := range rules {
		w.Int(rule)
		w.flow(m.Flows[rule])
	}
}

// flow writes the Flow body f: its number of registers and its instructions. An instruction is its
// operation (1 byte), then its destination and argument registers, its target and its immediate as
// far as the operation's shape (FlowShapes) has them.
func (w *Encoder) flow(f *Flow) {
	w.Uint(f.Regs)
	w.Uint(len(f.Code))
	for i := range f.Code {
		in := &f.Code[i]
		s, ok := in.Op.Shape()
		if !ok || len(in.Dst) != s.Dst || len(in.Args) != s.Args {
			panic(fmt.Sprintf("internal error: bytecode: flow instruction %d: %v with %d destinations and %d arguments", i, in.Op, len(in.Dst), len(in.Args)))
		}
		w.Byte(byte(in.Op))
		for _, d := range in.Dst {
			w.Int(d)
		}
		for _, a := range in.Args {
			w.Int(a)
		}
		if s.Target {
			w.Int(in.Target)
		}
		w.flowImm(s.Imm, in.Imm)
	}
}

// flowImm writes the immediate v of kind k. A constant is a kind (1 byte: ConstNil, ConstInt or
// ConstBool) and a signed integer (0 for nil, 0 or 1 for a bool); a guard is (literal, rune,
// case-folded, class, depth); an expectation (ID, message); a call (rule, level).
func (w *Encoder) flowImm(k FlowImm, v any) {
	bad := func() { panic(fmt.Sprintf("internal error: bytecode: flow immediate %#v of kind %d", v, k)) }
	switch k {
	case FlowImmNone:
		if v != nil {
			bad()
		}
	case FlowImmConst:
		switch x := v.(type) {
		case nil:
			w.Byte(ConstNil)
			w.Int(0)
		case int:
			w.Byte(ConstInt)
			w.Int(x)
		case bool:
			w.Byte(ConstBool)
			if x {
				w.Int(1)
			} else {
				w.Int(0)
			}
		default:
			bad()
		}
	case FlowImmGuard:
		x, ok := v.(FlowGuardOf)
		if !ok {
			bad()
		}
		w.Bool(x.Lit)
		w.Int(int(x.Rune))
		w.Bool(x.Fold)
		w.Int(x.Class)
		w.Int(x.Depth)
	case FlowImmExpect:
		x, ok := v.(FlowExpectOf)
		if !ok {
			bad()
		}
		w.Int(x.Exp)
		w.Bool(x.Message)
	case FlowImmCall:
		x, ok := v.(FlowCallOf)
		if !ok {
			bad()
		}
		w.Int(x.Rule)
		w.Int(x.Level)
	case FlowImmClass, FlowImmAnchor:
		x, ok := v.(int)
		if !ok {
			bad()
		}
		w.Int(x)
	case FlowImmLit:
		x, ok := v.(FlowLitOf)
		if !ok {
			bad()
		}
		w.Int(x.Str)
		w.Int(x.Exp)
		w.Bool(x.Fold)
	case FlowImmSilence:
		x, ok := v.(FlowSilenceOf)
		if !ok {
			bad()
		}
		w.Int(x.Delta)
		w.Int(x.Site)
		w.Bool(x.Ok)
	case FlowImmRunWalk:
		x, ok := v.(FlowRunWalkOf)
		if !ok {
			bad()
		}
		w.Int(x.Site)
		w.Bool(x.Build)
		w.Bool(x.KeyEnv)
	case FlowImmCount, FlowImmStr, FlowImmSlot, FlowImmScope, FlowImmRunSite, FlowImmScanSite, FlowImmTerm:
		x, ok := v.(int)
		if !ok {
			bad()
		}
		w.Int(x)
	case FlowImmShape:
		x, ok := v.(FlowShapeOf)
		if !ok {
			bad()
		}
		w.Bool(x.Elems)
		w.Bool(x.Shared)
		w.Bool(x.Tuple)
		w.Int(x.K)
		w.Int(x.Mode)
	default:
		bad()
	}
}

func (w *Encoder) term(t *Term) {
	for _, n := range t.Regs {
		w.Uint(n)
	}
	// Most terms use few of their pools: a byte whose bits 0 to 5 mark the pools that are not empty
	// (constants, names, member reads, node types, casts and struct literals), then those pools.
	lens := t.poolLens()
	var mask byte
	for i, n := range lens {
		if n != 0 {
			mask |= 1 << i
		}
	}
	w.Byte(mask)
	if lens[0] != 0 {
		w.Uint(len(t.Consts))
		for _, c := range t.Consts {
			w.Byte(byte(c.Kind))
			w.Body.Write(binary.AppendVarint(nil, c.Int))
		}
	}
	if lens[1] != 0 {
		w.ints(t.Names)
	}
	if lens[2] != 0 {
		w.Uint(len(t.Members))
		for _, x := range t.Members {
			w.Int(x.Kind)
			w.Int(x.Name)
			w.Int(x.Field)
		}
	}
	if lens[3] != 0 {
		w.ints(t.NodeTypes)
	}
	if lens[4] != 0 {
		w.Uint(len(t.Casts))
		for _, x := range t.Casts {
			w.Int(x.Type)
			w.Int(x.Line)
			w.Int(x.Col)
		}
	}
	if lens[5] != 0 {
		w.Uint(len(t.Structs))
		for _, x := range t.Structs {
			w.Int(x.Type)
			w.Int(x.Fields)
		}
	}
	w.Uint(len(t.Code))
	for _, in := range t.Code {
		// Most operands of term code are zero or unused: an instruction writes its opcode, a byte
		// whose bits 0 to 3 mark the operands D, A, B and C that are not zero, and those operands.
		ops := [4]int32{in.Dst, in.A, in.B, in.C}
		var mask byte
		for i, x := range ops {
			if x != 0 {
				mask |= 1 << i
			}
		}
		w.Byte(in.Op)
		w.Byte(mask)
		for _, x := range ops {
			if x != 0 {
				w.Int(int(x))
			}
		}
	}
}

// int32 reads an instruction operand.
func (r *Decoder) int32() int32 {
	if r.Err != nil {
		return 0
	}
	n, k := binary.Varint(r.Data[r.Pos:])
	if k <= 0 {
		r.Fail("invalid integer")
		return 0
	}
	r.Pos += k
	if n < -1<<31 || n >= 1<<31 {
		r.Fail("integer %d out of range", n)
		return 0
	}
	return int32(n)
}

func (r *Decoder) ints() []int {
	n := r.Count()
	xs := make([]int, 0, n)
	for i := 0; i < n && r.Err == nil; i++ {
		xs = append(xs, int(r.int32()))
	}
	return xs
}

func (r *Decoder) code() []Instr {
	n := r.Count()
	code := make([]Instr, 0, n)
	for i := 0; i < n && r.Err == nil; i++ {
		code = append(code, Instr{Op: Op(r.Byte()), A: r.int32(), B: r.int32(), C: r.int32()})
	}
	return code
}

func (r *Decoder) line() LineInfo {
	return LineInfo{Entry: int(r.int32()), Scope: r.ints(), Action: int(r.int32()), IsSeq: r.Bool()}
}

// ReadModule reads a module.
func (r *Decoder) ReadModule() *Module {
	m := &Module{}
	n := r.Uint()
	if r.Err == nil && n > len(r.Strs) {
		r.Fail("module has %d strings, file has %d", n, len(r.Strs))
	}
	if r.Err != nil {
		return m
	}
	m.Strings = r.Strs[:n]
	m.SourceInfo = r.Bool()
	m.DefaultValue = r.Bool()
	n = r.Count()
	m.Classes = sized[Class](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		c := Class{Negated: r.Bool(), Desc: int(r.int32())}
		k := r.Count()
		for j := 0; j < 2*k && r.Err == nil; j++ {
			c.Ranges = append(c.Ranges, rune(r.int32()))
		}
		m.Classes = append(m.Classes, c)
	}
	n = r.Count()
	m.Types = sized[TypeInfo](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		m.Types = append(m.Types, TypeInfo{Name: int(r.int32()), Fields: r.ints(), Zeros: r.ints(), Node: int(r.int32())})
	}
	n = r.Count()
	m.Scopes = sized[[]int](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		m.Scopes = append(m.Scopes, r.ints())
	}
	n = r.Count()
	m.Vars = sized[VarInfo](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		m.Vars = append(m.Vars, VarInfo{Name: int(r.int32()), Type: int(r.int32()), Int: r.Int(), Str: int(r.int32())})
	}
	n = r.Count()
	m.Rules = sized[RuleInfo](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		ri := RuleInfo{
			Name: int(r.int32()), Entry: int(r.int32()), Scope: r.ints(), Action: int(r.int32()),
			NodeType: int(r.int32()),
			Memo:     r.Bool(), Leader: r.Bool(), Member: r.Bool(), Cycle: int(r.int32()), Positional: r.Bool(), OperatorsRule: int(r.int32()), NoValue: r.Bool(), Transient: r.Bool(), Rec: r.Bool(), Where: int(r.int32()), Splice: r.Bool(), Vars: r.ints(),
		}
		m.Rules = append(m.Rules, ri)
	}
	m.Starts = r.ints()
	m.DefaultStart = int(r.int32())
	n = r.Count()
	m.OperatorsRules = sized[OperatorsRuleInfo](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		p := OperatorsRuleInfo{Skip: int(r.int32())}
		k := r.Count()
		for j := 0; j < k && r.Err == nil; j++ {
			p.Operands = append(p.Operands, r.line())
		}
		for _, ops := range []*[]OpInfo{&p.Prefix, &p.Led} {
			k = r.Count()
			for j := 0; j < k && r.Err == nil; j++ {
				o := OpInfo{ID: int(r.int32()), Kind: int(r.int32()), Assoc: int(r.int32()), Level: int(r.int32())}
				o.Line = r.line()
				*ops = append(*ops, o)
			}
		}
		m.OperatorsRules = append(m.OperatorsRules, p)
	}
	m.Code = r.code()
	n = r.Count()
	m.Ints = sized[int64](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		m.Ints = append(m.Ints, r.int64())
	}
	n = r.Count()
	m.Terms = sized[Term](r, n, 6)
	for i := 0; i < n && r.Err == nil; i++ {
		m.Terms = append(m.Terms, r.term())
	}
	n = r.Count()
	m.RunKeys = sized[RunKeyInfo](r, n, 3)
	for i := 0; i < n && r.Err == nil; i++ {
		m.RunKeys = append(m.RunKeys, RunKeyInfo{Site: int(r.int32()), Vars: r.ints(), Bind: r.ints()})
	}
	n = r.Count()
	for i := 0; i < n && r.Err == nil; i++ {
		rule := int(r.int32())
		f := r.flow()
		if r.Err != nil {
			break
		}
		if _, dup := m.Flows[rule]; dup {
			r.Fail("rule %d has two Flow bodies", rule)
			break
		}
		if m.Flows == nil {
			m.Flows = map[int]*Flow{}
		}
		m.Flows[rule] = f
	}
	return m
}

// flow reads a Flow body. The operation of each instruction says how many registers it names and
// whether a target and an immediate follow; an unknown operation is invalid. ValidateModule checks
// the registers, targets and immediates.
func (r *Decoder) flow() *Flow {
	f := &Flow{Regs: r.Uint()}
	n := r.Count()
	f.Code = sized[FlowInstr](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		in := FlowInstr{Op: FlowOp(r.Byte())}
		s, ok := in.Op.Shape()
		if r.Err == nil && !ok {
			r.Fail("invalid flow operation %d", in.Op)
			break
		}
		regs := func(k int) []int {
			if k == 0 {
				return nil
			}
			xs := make([]int, k)
			for j := range xs {
				xs[j] = int(r.int32())
			}
			return xs
		}
		in.Dst, in.Args = regs(s.Dst), regs(s.Args)
		if s.Target {
			in.Target = int(r.int32())
		}
		in.Imm = r.flowImm(s.Imm)
		f.Code = append(f.Code, in)
	}
	return f
}

// flowImm reads an immediate of kind k.
func (r *Decoder) flowImm(k FlowImm) any {
	switch k {
	case FlowImmConst:
		kind, x := r.Byte(), r.int64()
		if r.Err != nil {
			return nil
		}
		switch {
		case kind == ConstNil && x == 0:
			return nil
		case kind == ConstInt && int64(int(x)) == x:
			return int(x)
		case kind == ConstBool && (x == 0 || x == 1):
			return x == 1
		}
		r.Fail("invalid flow constant %d of kind %d", x, kind)
	case FlowImmGuard:
		return FlowGuardOf{Lit: r.Bool(), Rune: r.int32(), Fold: r.Bool(), Class: int(r.int32()), Depth: int(r.int32())}
	case FlowImmExpect:
		return FlowExpectOf{Exp: int(r.int32()), Message: r.Bool()}
	case FlowImmCall:
		return FlowCallOf{Rule: int(r.int32()), Level: int(r.int32())}
	case FlowImmClass, FlowImmAnchor:
		return int(r.int32())
	case FlowImmLit:
		return FlowLitOf{Str: int(r.int32()), Exp: int(r.int32()), Fold: r.Bool()}
	case FlowImmSilence:
		return FlowSilenceOf{Delta: int(r.int32()), Site: int(r.int32()), Ok: r.Bool()}
	case FlowImmRunWalk:
		return FlowRunWalkOf{Site: int(r.int32()), Build: r.Bool(), KeyEnv: r.Bool()}
	case FlowImmCount, FlowImmStr, FlowImmSlot, FlowImmScope, FlowImmRunSite, FlowImmScanSite, FlowImmTerm:
		return int(r.int32())
	case FlowImmShape:
		x := FlowShapeOf{Elems: r.Bool(), Shared: r.Bool(), Tuple: r.Bool()}
		x.K = int(r.int32())
		x.Mode = int(r.int32())
		return x
	}
	return nil
}

// int64 reads a signed integer of 64 bits.
func (r *Decoder) int64() int64 {
	if r.Err != nil {
		return 0
	}
	n, k := binary.Varint(r.Data[r.Pos:])
	if k <= 0 {
		r.Fail("invalid integer")
		return 0
	}
	r.Pos += k
	return n
}

// sized returns an empty slice with room for the n elements that follow, or nil for none. Each
// element takes at least least bytes of the data, so a valid count fits the bytes left; the room is
// bounded by them, so that a forged count reserves no more elements than the rest of the data could
// hold.
func sized[T any](r *Decoder, n, least int) []T {
	if left := (len(r.Data) - r.Pos) / least; n > left {
		n = left
	}
	if n <= 0 {
		return nil
	}
	return make([]T, 0, n)
}

// poolLens returns the lengths of the pools of t, in the order a module writes them.
func (t *Term) poolLens() [6]int {
	return [6]int{len(t.Consts), len(t.Names), len(t.Members), len(t.NodeTypes), len(t.Casts), len(t.Structs)}
}

func (r *Decoder) term() Term {
	var t Term
	for i := range t.Regs {
		t.Regs[i] = r.Uint()
	}
	mask := r.Byte()
	if r.Err == nil && mask > 0x3f {
		r.Fail("invalid term pool mask %#x", mask)
	}
	// count reads the length of pool i: zero if the mask leaves it out, else not zero, so that each
	// term has one encoding.
	count := func(i int) int {
		if mask&(1<<i) == 0 || r.Err != nil {
			return 0
		}
		n := r.Count()
		if r.Err == nil && n == 0 {
			r.Fail("term pool marked but empty")
		}
		return n
	}
	n := count(0)
	t.Consts = sized[TermConst](r, n, 2)
	for i := 0; i < n && r.Err == nil; i++ {
		t.Consts = append(t.Consts, TermConst{Kind: int(r.Byte()), Int: r.int64()})
	}
	n = count(1)
	t.Names = sized[int](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		t.Names = append(t.Names, int(r.int32()))
	}
	n = count(2)
	t.Members = sized[TermMember](r, n, 3)
	for i := 0; i < n && r.Err == nil; i++ {
		t.Members = append(t.Members, TermMember{Kind: int(r.int32()), Name: int(r.int32()), Field: int(r.int32())})
	}
	n = count(3)
	t.NodeTypes = sized[int](r, n, 1)
	for i := 0; i < n && r.Err == nil; i++ {
		t.NodeTypes = append(t.NodeTypes, int(r.int32()))
	}
	n = count(4)
	t.Casts = sized[TermCast](r, n, 3)
	for i := 0; i < n && r.Err == nil; i++ {
		t.Casts = append(t.Casts, TermCast{Type: int(r.int32()), Line: int(r.int32()), Col: int(r.int32())})
	}
	n = count(5)
	t.Structs = sized[TermStruct](r, n, 2)
	for i := 0; i < n && r.Err == nil; i++ {
		t.Structs = append(t.Structs, TermStruct{Type: int(r.int32()), Fields: int(r.int32())})
	}
	n = r.Count()
	t.Code = sized[TermInstr](r, n, 2)
	for i := 0; i < n && r.Err == nil; i++ {
		t.Code = append(t.Code, r.termInstr())
	}
	return t
}

// termInstr reads an instruction of term code: its opcode, the mask of its operands that are not
// zero, and those operands. A mask with other bits, or a marked operand that is zero, is invalid,
// so that each instruction has one encoding.
func (r *Decoder) termInstr() TermInstr {
	op, mask := r.Byte(), r.Byte()
	if r.Err == nil && mask > 0xf {
		r.Fail("invalid term operand mask %#x", mask)
	}
	var ops [4]int32
	for i := range ops {
		if mask&(1<<i) == 0 || r.Err != nil {
			continue
		}
		if ops[i] = r.int32(); r.Err == nil && ops[i] == 0 {
			r.Fail("term operand marked but zero")
		}
	}
	return TermInstr{Op: op, Dst: ops[0], A: ops[1], B: ops[2], C: ops[3]}
}

// ValidateModule checks that every table and instruction reference in the module is in range
// and that jumps go forward (only the NEXT of a repetition jumps backward).
// Recovery regions must not enclose a stream commitment. Other value-stack
// and entry-stack invariants are not checked. fieldType checks the text of a field type, which
// the type system above the module defines.
func ValidateModule(m *Module, fieldType func(text string) error) error {
	var err error
	fail := func(format string, args ...any) {
		if err == nil {
			err = fmt.Errorf(format, args...)
		}
	}
	in := func(what string, i, n int) {
		if i < 0 || i >= n {
			fail("%s %d out of range", what, i)
		}
	}
	opt := func(what string, i, n int) {
		if i != -1 {
			in(what, i, n)
		}
	}
	str := func(i int) { in("string", i, len(m.Strings)) }
	strs := func(xs []int) {
		for _, x := range xs {
			str(x)
		}
	}
	nonneg := func(i int32) {
		if i < 0 {
			fail("negative operand %d", i)
		}
	}
	for _, c := range m.Classes {
		if len(c.Ranges)%2 != 0 {
			fail("invalid class")
		}
		str(c.Desc)
	}
	for _, t := range m.Types {
		str(t.Name)
		strs(t.Fields)
		strs(t.Zeros)
		if err == nil && len(t.Zeros) != len(t.Fields) {
			fail("type %q has %d zero descriptors for %d fields", m.Strings[t.Name], len(t.Zeros), len(t.Fields))
		}
		opt("string", t.Node, len(m.Strings))
		if err == nil && t.Node >= 0 && len(t.Fields) != 0 {
			fail("node type %q has fields", m.Strings[t.Name])
		}
	}
	if err == nil {
		err = validateZeros(m)
	}
	for _, l := range m.Scopes {
		strs(l)
	}
	// typeErr parses the field type at string index i once no error is recorded. Callers format
	// the location only on failure, so a valid module allocates no messages.
	typeErr := func(i int) error {
		if err != nil {
			return nil
		}
		return fieldType(m.Strings[i])
	}
	for _, t := range m.Types {
		if t.Node >= 0 && err == nil {
			if perr := typeErr(t.Node); perr != nil {
				fail("%v in node type %s", perr, m.Strings[t.Name])
			}
			if err == nil && !strings.HasPrefix(m.Strings[t.Node], "node[") {
				fail("node type %q is not over a node type", m.Strings[t.Name])
			}
		}
	}
	// nodeType reports whether name is a declared named node type.
	nodeType := func(name string) bool {
		for _, t := range m.Types {
			if m.Strings[t.Name] == name {
				return t.Node >= 0
			}
		}
		return false
	}
	// structType reports whether name is a declared struct type.
	structType := func(name string) bool {
		for _, t := range m.Types {
			if m.Strings[t.Name] == name {
				return t.Node < 0
			}
		}
		return false
	}
	line := func(l *LineInfo) {
		in("entry", l.Entry, len(m.Code))
		strs(l.Scope)
		opt("action", l.Action, len(m.Terms))
	}
	seenVars := map[string]bool{}
	for _, v := range m.Vars {
		str(v.Name)
		in("variable type", v.Type, 3)
		opt("string", v.Str, len(m.Strings))
		if err == nil {
			name := m.Strings[v.Name]
			switch {
			case seenVars[name]:
				fail("variable %q is defined twice", name)
			case (v.Type == VarString) != (v.Str >= 0):
				fail("invalid initial value of variable %q", name)
			case v.Type == VarBool && v.Int != 0 && v.Int != 1:
				fail("invalid initial value of variable %q", name)
			case v.Type == VarString && v.Int != 0:
				fail("invalid initial value of variable %q", name)
			}
			seenVars[name] = true
		}
	}
	// declared checks that a string index names a declared variable.
	declared := func(i int) {
		str(i)
		if err == nil && !seenVars[m.Strings[i]] {
			fail("variable %q is not declared", m.Strings[i])
		}
	}
	for _, r := range m.Rules {
		str(r.Name)
		in("entry", r.Entry, len(m.Code))
		strs(r.Scope)
		opt("action", r.Action, len(m.Terms))
		opt("string", r.NodeType, len(m.Strings))
		if err == nil && r.NodeType >= 0 && !nodeType(m.Strings[r.NodeType]) {
			fail("type %q is not a named node type", m.Strings[r.NodeType])
		}
		opt("string", r.Where, len(m.Strings))
		opt("operatorsRule", r.OperatorsRule, len(m.OperatorsRules))
		for _, v := range r.Vars {
			declared(v)
		}
	}
	for i, k := range m.RunKeys {
		if err == nil && (k.Site <= 0 || i > 0 && k.Site <= m.RunKeys[i-1].Site) {
			fail("invalid run key site %d", k.Site)
		}
		for _, v := range k.Vars {
			declared(v)
		}
		for _, v := range k.Bind {
			declared(v)
			if err == nil && !slices.Contains(k.Vars, v) {
				fail("run key of site %d binds %q, which it does not hold", k.Site, m.Strings[v])
			}
		}
	}
	if len(m.Starts) == 0 {
		fail("no start rule")
	}
	startNames := map[int]bool{}
	for i, x := range m.Starts {
		in("start rule", x, len(m.Rules))
		if err == nil && (i > 0 && x <= m.Starts[i-1] || startNames[m.Rules[x].Name]) {
			fail("invalid start rule %d", x)
		}
		if err == nil {
			startNames[m.Rules[x].Name] = true
		}
	}
	if err == nil && !slices.Contains(m.Starts, m.DefaultStart) {
		fail("default start rule %d is not a start rule", m.DefaultStart)
	}
	for _, p := range m.OperatorsRules {
		opt("entry", p.Skip, len(m.Code))
		for i := range p.Operands {
			line(&p.Operands[i])
		}
		for _, ops := range [][]OpInfo{p.Prefix, p.Led} {
			for i := range ops {
				o := &ops[i]
				in("operator kind", o.Kind, 3)
				in("associativity", o.Assoc, 3)
				line(&o.Line)
			}
		}
	}
	forward := func(ip int, l int32, n int) {
		if int(l) <= ip || int(l) >= n {
			fail("invalid jump from %d to %d", ip, l)
		}
	}
	flag := func(b int32, max int32) {
		if b < 0 || b > max {
			fail("invalid flag %d", b)
		}
	}
	bound := func(index int32) int64 {
		in("wide integer", int(index), len(m.Ints))
		if index < 0 || int(index) >= len(m.Ints) {
			return 0
		}
		n := m.Ints[index]
		if int64(int(n)) != n {
			fail("repetition bound %d exceeds the implementation int range", n)
		}
		return n
	}
	wideBounds := func(minIndex, maxIndex int32) {
		min, max := bound(minIndex), bound(maxIndex)
		if min < 0 {
			fail("invalid minimum %d", min)
		}
		if max < -1 || max >= 0 && max < min {
			fail("invalid maximum %d for minimum %d", max, min)
		}
	}
	keys := make(map[int]bool, len(m.RunKeys))
	for _, k := range m.RunKeys {
		keys[k.Site] = true
	}
	keyed := func(site int32) {
		if err == nil && !keys[int(site)] {
			fail("run site %d is keyed by the variables but has no run key", site)
		}
	}
	for ip, x := range m.Code {
		n := len(m.Code)
		switch x.Op {
		case OpStr, OpStrFold:
			str(int(x.A))
			str(int(x.B))
			flag(x.C, 1)
		case OpClass:
			in("class", int(x.A), len(m.Classes))
			str(int(x.B))
			flag(x.C, 1)
		case OpAny, OpTop, OpAtomic, OpEndRepeat, OpEndLook, OpEndSkip:
			flag(x.A, 1)
		case OpAssert:
			flag(x.A, 3)
		case OpFail, OpCut, OpPushPos, OpPushNil, OpEndLabel, OpReturn, OpEnd:
		case OpIter:
			forward(ip, x.A, n)
			flag(x.B, 7)
			if x.C == 0 && x.B != 0 {
				fail("run site flags %d without a run site", x.B)
			}
			if x.B&4 != 0 && x.B&2 != 0 {
				fail("scan site with the flags %d of a run site", x.B)
			}
			if x.B&4 != 0 {
				if x.C < 1 || x.C > scanSiteLimit(n) {
					fail("invalid scan site %d", x.C)
				}
			} else if x.C < 0 || int(x.C) > n {
				fail("invalid run site %d", x.C)
			} else if x.C > 0 && x.B&2 == 0 {
				keyed(x.C)
			}
		case OpJump, OpChoice, OpCommit, OpRecover, OpEndRecover:
			forward(ip, x.A, n)
		case OpLook:
			flag(x.B, 1)
			if x.B == 1 { // a positive lookahead does not use the jump target
				forward(ip, x.A, n)
				flag(x.C, 0)
			} else {
				flag(x.C, 1)
			}
		case OpNext:
			if x.A < 0 || int(x.A) >= ip {
				fail("invalid loop from %d to %d", ip, x.A)
			}
			flag(x.B, 3)
			nonneg(x.C)
		case OpSeq:
			nonneg(x.A)
		case OpSpread:
			nonneg(x.A)
			flag(x.B, 1)
		case OpCapture:
			nonneg(x.A)
			flag(x.B, 1)
		case OpRepeat:
			nonneg(x.A)
			if x.B < -1 {
				fail("invalid maximum %d", x.B)
			}
			opt("scope", int(x.C), len(m.Scopes))
		case OpRepeatWide:
			wideBounds(x.A, x.B)
			opt("scope", int(x.C), len(m.Scopes))
		case OpSkipRepeat:
			flag(x.C, 3)
			if x.C&1 == 1 {
				if min := bound(x.A); min < 0 {
					fail("invalid minimum %d", min)
				}
			} else {
				nonneg(x.A)
			}
			if x.B < 1 || int(x.B) > n {
				fail("invalid skip-run site %d", x.B)
			} else if x.C&2 == 0 {
				keyed(x.B)
			}
		case OpCall:
			in("rule", int(x.A), len(m.Rules))
			nonneg(x.B)
			flag(x.C, 1)
		case OpOperatorsRule:
			in("operatorsRule", int(x.A), len(m.OperatorsRules))
		case OpPred:
			in("term", int(x.A), len(m.Terms))
		case OpBind:
			in("variable", int(x.A), len(m.Vars))
			in("term", int(x.B), len(m.Terms))
			flag(x.C, 0)
		case OpLabel:
			str(int(x.A))
			flag(x.B, 1)
		case OpGuard:
			in("class", int(x.A), len(m.Classes))
			nonneg(x.B)
			forward(ip, x.C, n)
		case OpScan:
			opt("class", int(x.A), len(m.Classes))
			nonneg(x.B)
			if x.C < -1 {
				fail("invalid maximum %d", x.C)
			}
		case OpScanWide:
			opt("class", int(x.A), len(m.Classes))
			wideBounds(x.B, x.C)
		case OpScanSite:
			opt("class", int(x.A), len(m.Classes))
			nonneg(x.B)
			if x.C < 1 || x.C > scanSiteLimit(n) {
				fail("invalid scan site %d", x.C)
			}
		default:
			fail("invalid instruction %d at %d", x.Op, ip)
		}
	}
	for i := range m.Terms {
		t := &m.Terms[i]
		for _, c := range t.Consts {
			switch c.Kind {
			case ConstNil, ConstInt:
			case ConstString:
				str(int(c.Int))
			case ConstBool:
				if c.Int != 0 && c.Int != 1 {
					fail("invalid boolean constant %d in term %d", c.Int, i)
				}
			default:
				fail("invalid constant kind %d in term %d", c.Kind, i)
			}
		}
		strs(t.Names)
		for _, x := range t.Members {
			str(x.Name)
			str(x.Field)
		}
		for _, x := range t.NodeTypes {
			str(x)
			if err == nil && !nodeType(m.Strings[x]) {
				fail("type %q of a conversion is not a named node type in term %d", m.Strings[x], i)
			}
		}
		for _, x := range t.Casts {
			opt("string", x.Type, len(m.Strings))
			if x.Type >= 0 {
				if perr := typeErr(x.Type); perr != nil {
					fail("%v in term %d", perr, i)
				}
			}
			if x.Line < 0 || x.Col < 0 {
				fail("invalid cast position %d:%d in term %d", x.Line, x.Col, i)
			}
		}
		for _, x := range t.Structs {
			str(x.Type)
			if err == nil && !structType(m.Strings[x.Type]) {
				fail("type %q of a struct literal is not a struct type in term %d", m.Strings[x.Type], i)
			}
			if x.Fields < 0 {
				fail("invalid field count %d in term %d", x.Fields, i)
			}
		}
	}
	if err != nil {
		return err
	}
	if err := validateFlows(m); err != nil {
		return err
	}
	return ValidateStreamRecovery(m.Code)
}

// validateFlows checks the Flow bodies of m: each belongs to a rule of the rule table; each
// instruction has a known operation with the registers, target and immediate of its shape; every
// register is within the frame (a destination may be -1, which discards); every target is an
// instruction of the body; every immediate refers to a class, rule, expectation or scope in range,
// and every capture slot to a slot of the largest frame the body may run with (its rule's scope or a
// scope of its NEWFRAME); and the body ends with JUMP or RETURN, so that running it cannot pass its
// end. As for match code, a runtime does not check that registers hold what an instruction reads;
// it checks that a slot is in the frame it indexes.
func validateFlows(m *Module) error {
	for _, rule := range slices.Sorted(maps.Keys(m.Flows)) {
		f := m.Flows[rule]
		if rule < 0 || rule >= len(m.Rules) {
			return fmt.Errorf("flow body of rule %d out of range", rule)
		}
		if err := validateFlow(m, rule, f); err != nil {
			return fmt.Errorf("flow body of rule %s: %w", m.Strings[m.Rules[rule].Name], err)
		}
	}
	return nil
}

func validateFlow(m *Module, rule int, f *Flow) error {
	if f == nil || f.Regs < 0 {
		return fmt.Errorf("invalid frame")
	}
	if n := len(f.Code); n == 0 || f.Code[n-1].Op != FlowJump && f.Code[n-1].Op != FlowReturn {
		return fmt.Errorf("the body does not end with JUMP or RETURN")
	}
	slots := len(m.Rules[rule].Scope) // the number of slots of the largest frame the body may run with
	for _, in := range f.Code {
		if sc, ok := in.Imm.(int); ok && in.Op == FlowNewFrame && sc >= 0 && sc < len(m.Scopes) {
			slots = max(slots, len(m.Scopes[sc]))
		}
	}
	for pc := range f.Code {
		in := &f.Code[pc]
		s, ok := in.Op.Shape()
		if !ok {
			return fmt.Errorf("invalid flow operation %d at %d", in.Op, pc)
		}
		if len(in.Dst) != s.Dst || len(in.Args) != s.Args {
			return fmt.Errorf("%v at %d with %d destinations and %d arguments", in.Op, pc, len(in.Dst), len(in.Args))
		}
		for _, d := range in.Dst {
			if d < -1 || d >= f.Regs {
				return fmt.Errorf("register %d out of range at %d", d, pc)
			}
		}
		for _, a := range in.Args {
			if a < 0 || a >= f.Regs {
				return fmt.Errorf("register %d out of range at %d", a, pc)
			}
		}
		if s.Target && (in.Target < 0 || in.Target >= len(f.Code)) || !s.Target && in.Target != 0 {
			return fmt.Errorf("invalid jump from %d to %d", pc, in.Target)
		}
		if err := validateFlowImm(m, slots, s.Imm, in.Imm); err != nil {
			return fmt.Errorf("%v at %d: %w", in.Op, pc, err)
		}
	}
	return nil
}

// validateFlowImm checks the immediate v of kind k of an instruction of a body whose largest frame
// has the number of slots slots.
func validateFlowImm(m *Module, slots int, k FlowImm, v any) error {
	ok := false
	switch k {
	case FlowImmNone:
		ok = v == nil
	case FlowImmConst:
		switch v.(type) {
		case nil, bool, int:
			ok = true
		}
	case FlowImmGuard:
		var x FlowGuardOf
		if x, ok = v.(FlowGuardOf); !ok {
			break
		}
		switch {
		case x.Lit && x.Class != -1, !x.Lit && (x.Class < 0 || x.Class >= len(m.Classes) || x.Fold):
			return fmt.Errorf("invalid guard class %d", x.Class)
		case x.Depth < 0:
			return fmt.Errorf("invalid guard depth %d", x.Depth)
		}
	case FlowImmExpect:
		var x FlowExpectOf
		if x, ok = v.(FlowExpectOf); ok && (x.Exp < 0 || x.Exp >= FixedExps+len(m.Strings)) {
			return fmt.Errorf("expectation %d out of range", x.Exp)
		}
	case FlowImmCall:
		var x FlowCallOf
		if x, ok = v.(FlowCallOf); !ok {
			break
		}
		if x.Rule < 0 || x.Rule >= len(m.Rules) {
			return fmt.Errorf("rule %d out of range", x.Rule)
		}
		if x.Level < 0 {
			return fmt.Errorf("invalid binding level %d", x.Level)
		}
	case FlowImmClass:
		var x int
		if x, ok = v.(int); ok && (x < 0 || x >= len(m.Classes)) {
			return fmt.Errorf("class %d out of range", x)
		}
	case FlowImmAnchor:
		var x int
		if x, ok = v.(int); ok && (x < 0 || x > 3) {
			return fmt.Errorf("invalid anchor %d", x)
		}
	case FlowImmLit:
		var x FlowLitOf
		if x, ok = v.(FlowLitOf); !ok {
			break
		}
		if x.Str < 0 || x.Str >= len(m.Strings) {
			return fmt.Errorf("literal %d out of range", x.Str)
		}
		if x.Exp < 0 || x.Exp >= FixedExps+len(m.Strings) {
			return fmt.Errorf("expectation %d out of range", x.Exp)
		}
	case FlowImmCount:
		var x int
		if x, ok = v.(int); ok && x < 0 {
			return fmt.Errorf("invalid count %d", x)
		}
	case FlowImmStr:
		var x int
		if x, ok = v.(int); ok && (x < 0 || x >= len(m.Strings)) {
			return fmt.Errorf("string %d out of range", x)
		}
	case FlowImmSlot:
		var x int
		if x, ok = v.(int); ok && (x < 0 || x >= slots) {
			return fmt.Errorf("capture slot %d out of range", x)
		}
	case FlowImmScope:
		var x int
		if x, ok = v.(int); ok && (x < 0 || x >= len(m.Scopes)) {
			return fmt.Errorf("scope %d out of range", x)
		}
	case FlowImmTerm:
		var x int
		if x, ok = v.(int); ok && (x < 0 || x >= len(m.Terms)) {
			return fmt.Errorf("term %d out of range", x)
		}
	case FlowImmRunSite:
		var x int
		if x, ok = v.(int); ok && (x < 1 || x > len(m.Code)) {
			return fmt.Errorf("invalid run site %d", x)
		}
	case FlowImmRunWalk:
		var x FlowRunWalkOf
		if x, ok = v.(FlowRunWalkOf); !ok {
			break
		}
		if x.Site < 1 || x.Site > len(m.Code) {
			return fmt.Errorf("invalid run site %d", x.Site)
		}
		if x.KeyEnv && !slices.ContainsFunc(m.RunKeys, func(k RunKeyInfo) bool { return k.Site == x.Site }) {
			return fmt.Errorf("run site %d is keyed by the variables but has no run key", x.Site)
		}
	case FlowImmScanSite:
		var x int
		if x, ok = v.(int); ok && (x < 1 || int64(x) > int64(scanSiteLimit(len(m.Code)))) {
			return fmt.Errorf("invalid scan site %d", x)
		}
	case FlowImmShape:
		var x FlowShapeOf
		if x, ok = v.(FlowShapeOf); ok && (x.K < 0 || x.Mode < 0 || x.Mode > 2) {
			return fmt.Errorf("invalid shape %+v", x)
		}
	case FlowImmSilence:
		var x FlowSilenceOf
		if x, ok = v.(FlowSilenceOf); ok && (x.Delta != 1 && x.Delta != -1 || x.Site < -1 || x.Delta == 1 && x.Ok) {
			return fmt.Errorf("invalid lookahead %+v", x)
		}
	}
	if !ok {
		return fmt.Errorf("invalid immediate %#v", v)
	}
	return nil
}

// ValidateStreamRecovery checks recovery boundaries in lexical bytecode order.
// Both the protected body and its skip region must finish before a stream NEXT:
// recovery restores the saved input position, but streaming discards that input.
// Element-local recovery ends before the enclosing repetition's NEXT and is safe.
func ValidateStreamRecovery(code []Instr) error {
	type region struct{ skip, end int }
	var regions []region
	for ip, in := range code {
		for len(regions) > 0 && regions[len(regions)-1].end == ip {
			regions = regions[:len(regions)-1]
		}
		if in.Op == OpRecover {
			skip := int(in.A)
			if skip <= ip+1 || skip >= len(code) || code[skip-1].Op != OpEndRecover {
				return fmt.Errorf("invalid recovery body at %d", ip)
			}
			end := int(code[skip-1].A)
			if end <= skip || end >= len(code) || code[end-1].Op != OpEndSkip {
				return fmt.Errorf("invalid recovery skip at %d", ip)
			}
			if len(regions) > 0 {
				parent := regions[len(regions)-1]
				limit := parent.end
				if ip < parent.skip {
					limit = parent.skip
				}
				if end >= limit {
					return fmt.Errorf("crossing recovery regions at %d", ip)
				}
			}
			regions = append(regions, region{skip, end})
		} else if in.StreamsValue() && len(regions) > 0 {
			return fmt.Errorf("recovery region encloses a stream commitment at %d", ip)
		}
	}
	return nil
}

// scanSiteLimit is the largest scan site number that code of n instructions can name: the sites of
// the skip runs and run sites come first, at most one for each instruction, and then two for each
// scan site.
func scanSiteLimit(n int) int32 { return int32(min(3*n+2, 1<<30)) }

// validateZeros checks the zero descriptors of the struct types of m (TypeInfo.Zeros): each is one
// descriptor of the grammar in ZeroDesc, every {Name} names a struct type of the module, and no struct
// type reaches itself through the {Name} of its descriptors, which a compiled module never has since
// such a type has no zero value. The string table indices are already checked.
func validateZeros(m *Module) error {
	structs := map[string][]string{} // struct type -> struct types its descriptors refer to
	for _, t := range m.Types {
		if t.Node < 0 {
			structs[m.Strings[t.Name]] = nil
		}
	}
	for _, t := range m.Types {
		if t.Node >= 0 {
			continue
		}
		name := m.Strings[t.Name]
		for i, z := range t.Zeros {
			refs, ok := zeroRefs(m.Strings[z])
			if !ok {
				return fmt.Errorf("type %q has the malformed zero descriptor %q for field %q", name, m.Strings[z], m.Strings[t.Fields[i]])
			}
			for _, r := range refs {
				if _, ok := structs[r]; !ok {
					return fmt.Errorf("type %q has the zero descriptor %q for field %q, which names %q, not a struct type", name, m.Strings[z], m.Strings[t.Fields[i]], r)
				}
			}
			structs[name] = append(structs[name], refs...)
		}
	}
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var cyclic func(name string) bool
	cyclic = func(name string) bool {
		switch state[name] {
		case visiting:
			return true
		case done:
			return false
		}
		state[name] = visiting
		for _, r := range structs[name] {
			if cyclic(r) {
				return true
			}
		}
		state[name] = done
		return false
	}
	for _, t := range m.Types {
		if t.Node < 0 && cyclic(m.Strings[t.Name]) {
			return fmt.Errorf("type %q reaches itself through its zero descriptors", m.Strings[t.Name])
		}
	}
	return nil
}

// zeroRefs parses d as one zero descriptor and returns the struct type names it refers to, or false
// if d is not exactly one descriptor: i, s, b, n, l or - alone, a tuple of descriptors in parentheses,
// or a struct type name in braces. A - stands for a whole descriptor, never an element of a tuple.
func zeroRefs(d string) ([]string, bool) {
	var refs []string
	var one func(i int, inTuple bool) (int, bool)
	one = func(i int, inTuple bool) (int, bool) {
		if i >= len(d) {
			return i, false
		}
		switch d[i] {
		case 'i', 's', 'b', 'n', 'l':
			return i + 1, true
		case '-':
			return i + 1, !inTuple
		case '(':
			i++
			for i < len(d) && d[i] != ')' {
				var ok bool
				if i, ok = one(i, true); !ok {
					return i, false
				}
			}
			return i + 1, i < len(d)
		case '{':
			end := strings.IndexByte(d[i:], '}')
			if end <= 1 {
				return i, false
			}
			refs = append(refs, d[i+1:i+end])
			return i + end + 1, true
		}
		return i, false
	}
	end, ok := one(0, false)
	return refs, ok && end == len(d)
}
