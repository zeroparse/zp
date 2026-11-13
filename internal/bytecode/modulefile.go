package bytecode

import (
	"encoding/binary"
	"fmt"
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
	return m
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
	return ValidateStreamRecovery(m.Code)
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
