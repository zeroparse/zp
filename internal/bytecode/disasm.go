package bytecode

import (
	"fmt"
	"strconv"
	"strings"
)

// Disassemble renders the module as human-readable text (for debugging).
func (m *Module) Disassemble() string {
	var b strings.Builder
	str := func(i int) string {
		if i < 0 || i >= len(m.Strings) {
			return fmt.Sprintf("<%d>", i)
		}
		return strconv.Quote(m.Strings[i])
	}
	names := func(ids []int) string {
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = m.Strings[id]
		}
		return "[" + strings.Join(parts, " ") + "]"
	}
	// Label the start positions of rules and Operators Rule sections.
	labels := map[int][]string{}
	tlabels := map[int][]string{}
	for _, r := range m.Rules {
		name := m.Strings[r.Name]
		labels[r.Entry] = append(labels[r.Entry], name+":")
		if r.Action >= 0 {
			tlabels[r.Action] = append(tlabels[r.Action], name+".action:")
		}
	}
	line := func(l LineInfo, name string) {
		labels[l.Entry] = append(labels[l.Entry], name+":")
		if l.Action >= 0 {
			tlabels[l.Action] = append(tlabels[l.Action], name+".action:")
		}
	}
	for i, p := range m.OperatorsRules {
		if p.Skip >= 0 {
			labels[p.Skip] = append(labels[p.Skip], fmt.Sprintf("operatorsRule%d.skip:", i))
		}
		for k, o := range p.Operands {
			line(o, fmt.Sprintf("operatorsRule%d.operand%d", i, k))
		}
		for _, o := range append(append([]OpInfo{}, p.Prefix...), p.Led...) {
			line(o.Line, fmt.Sprintf("operatorsRule%d.op%d", i, o.ID))
		}
	}

	b.WriteString("rules:\n")
	for i, r := range m.Rules {
		fmt.Fprintf(&b, "  %d %s entry=%d scope=%s", i, m.Strings[r.Name], r.Entry, names(r.Scope))
		if r.Action >= 0 {
			fmt.Fprintf(&b, " action=%d", r.Action)
		}
		if r.NodeType >= 0 {
			fmt.Fprintf(&b, " type=%s", m.Strings[r.NodeType])
		}
		if r.OperatorsRule >= 0 {
			fmt.Fprintf(&b, " operatorsRule=%d", r.OperatorsRule)
		}
		if r.Where >= 0 {
			fmt.Fprintf(&b, " where=%q", m.Strings[r.Where])
		}
		var flags []string
		for _, f := range []struct {
			on   bool
			name string
		}{{r.Memo, "memo"}, {r.Leader, "leader"}, {r.Member, "member"}, {r.Positional, "positional"}, {r.NoValue, "novalue"}, {r.Transient, "transient"}, {r.Rec, "rec"}, {r.Splice, "splice"}} {
			if f.on {
				flags = append(flags, f.name)
			}
		}
		if len(flags) > 0 {
			fmt.Fprintf(&b, " %s", strings.Join(flags, ","))
		}
		b.WriteString("\n")
	}
	for i, p := range m.OperatorsRules {
		fmt.Fprintf(&b, "operatorsRule %d: skip=%d operands=%d\n", i, p.Skip, len(p.Operands))
		for _, o := range append(append([]OpInfo{}, p.Prefix...), p.Led...) {
			kind := []string{"prefix", "postfix", "infix"}[o.Kind]
			assoc := []string{"left", "right", "none"}[o.Assoc]
			fmt.Fprintf(&b, "  op%d %s %s level=%d entry=%d action=%d scope=%s", o.ID, kind, assoc, o.Level, o.Line.Entry, o.Line.Action, names(o.Line.Scope))
			b.WriteString("\n")
		}
	}
	b.WriteString("code:\n")
	for i, in := range m.Code {
		for _, l := range labels[i] {
			fmt.Fprintf(&b, "%s\n", l)
		}
		fmt.Fprintf(&b, "  %4d  %-10s %s\n", i, in.Op, m.operands(in, str, names))
	}
	if len(m.Ints) > 0 {
		fmt.Fprintf(&b, "ints: %v\n", m.Ints)
	}
	b.WriteString("terms:\n")
	for i, t := range m.Terms {
		for _, l := range tlabels[i] {
			fmt.Fprintf(&b, "%s\n", l)
		}
		fmt.Fprintf(&b, "  term %d regs=%v\n", i, t.Regs)
		m.disassembleTerm(&b, &t, str)
	}
	return b.String()
}

func (m *Module) operands(in Instr, str func(int) string, names func([]int) string) string {
	a, bb, c := int(in.A), int(in.B), int(in.C)
	switch in.Op {
	case OpStr, OpStrFold:
		return fmt.Sprintf("%s build=%d", str(a), c)
	case OpClass:
		return fmt.Sprintf("%s build=%d", str(bb), c)
	case OpAny, OpTop, OpAtomic, OpEndSkip, OpEndRepeat:
		return fmt.Sprintf("build=%d", a)
	case OpAssert:
		return []string{"begin-input", "end-input", "begin-line", "end-line"}[a]
	case OpIter:
		if c > 0 && bb&4 != 0 {
			return fmt.Sprintf("-> %d scan=%d build=%d", a, c, bb&1)
		}
		if c > 0 {
			return fmt.Sprintf("-> %d run=%d build=%d%s", a, c, bb&1, envFreeNote(bb))
		}
		return fmt.Sprintf("-> %d", a)
	case OpJump, OpChoice, OpCommit, OpRecover, OpEndRecover:
		return fmt.Sprintf("-> %d", a)
	case OpSeq:
		return fmt.Sprintf("n=%d", a)
	case OpSpread:
		return fmt.Sprintf("n=%d tuple=%d", a, bb)
	case OpCapture:
		if bb == 1 {
			return fmt.Sprintf("slot=%d pop", a)
		}
		return fmt.Sprintf("slot=%d", a)
	case OpGuard:
		return fmt.Sprintf("%s depth=%d -> %d", m.Strings[m.Classes[a].Desc], bb, c) // the expectation as written
	case OpScan, OpScanWide:
		if in.Op == OpScanWide {
			bb, c = int(m.Ints[bb]), int(m.Ints[c])
		}
		cl := "any"
		if a >= 0 {
			cl = str(m.Classes[a].Desc)
		}
		return fmt.Sprintf("%s min=%d max=%d", cl, bb, c)
	case OpScanSite:
		cl := "any"
		if a >= 0 {
			cl = str(m.Classes[a].Desc)
		}
		return fmt.Sprintf("%s min=%d site=%d", cl, bb, c)
	case OpRepeat, OpRepeatWide:
		if in.Op == OpRepeatWide {
			a, bb = int(m.Ints[a]), int(m.Ints[bb])
		}
		sc := "-"
		if c >= 0 {
			sc = names(m.Scopes[c])
		}
		return fmt.Sprintf("min=%d max=%d scope=%s", a, bb, sc)
	case OpSkipRepeat:
		if c&1 == 1 {
			a = int(m.Ints[a])
		}
		return fmt.Sprintf("min=%d site=%d%s", a, bb, envFreeNote(c))
	case OpNext:
		if bb == 3 {
			return fmt.Sprintf("-> %d mode=field slot=%d", a, c)
		}
		return fmt.Sprintf("-> %d mode=%s", a, []string{"drop", "keep", "stream"}[bb])
	case OpLook:
		if bb == 1 {
			return fmt.Sprintf("not -> %d", a)
		}
		if c == 1 {
			return "and keeps"
		}
		return "and"
	case OpEndLook:
		if a == 1 {
			return "not"
		}
		return "and"
	case OpCall:
		return fmt.Sprintf("%s min=%d keep=%d", m.Strings[m.Rules[a].Name], bb, c)
	case OpOperatorsRule:
		return fmt.Sprintf("operatorsRule%d", a)
	case OpPred:
		return fmt.Sprintf("term=%d", a)
	case OpBind:
		return fmt.Sprintf("%s term=%d", varName(m, a), bb)
	case OpLabel:
		if bb == 1 {
			return str(a) + " mode=diagnostic"
		}
		return str(a)
	}
	return ""
}

// varName returns the name of the variable in slot i.
func varName(m *Module, i int) string {
	if i < 0 || i >= len(m.Vars) {
		return fmt.Sprintf("var%d", i)
	}
	return m.Strings[m.Vars[i].Name]
}

// envFreeNote describes the flag of a site instruction (bit 1 of its flags) that the site is free
// of the environment.
func envFreeNote(flags int) string {
	if flags&2 != 0 {
		return " env-free"
	}
	return ""
}

// disassembleTerm renders the pools and the instructions of the term t. The opcodes of term code
// belong to the engine, so instructions show their numbers.
func (m *Module) disassembleTerm(b *strings.Builder, t *Term, str func(int) string) {
	for i, c := range t.Consts {
		v := "nil"
		switch c.Kind {
		case ConstInt:
			v = strconv.FormatInt(c.Int, 10)
		case ConstString:
			v = str(int(c.Int))
		case ConstBool:
			v = strconv.FormatBool(c.Int == 1)
		}
		fmt.Fprintf(b, "    const %d %s\n", i, v)
	}
	for i, n := range t.Names {
		fmt.Fprintf(b, "    name %d %s\n", i, str(n))
	}
	for i, x := range t.Members {
		fmt.Fprintf(b, "    member %d kind=%d %s %s\n", i, x.Kind, str(x.Name), str(x.Field))
	}
	for i, n := range t.NodeTypes {
		fmt.Fprintf(b, "    node %d %s\n", i, str(n))
	}
	for i, x := range t.Casts {
		fmt.Fprintf(b, "    cast %d %s at=%d:%d\n", i, str(x.Type), x.Line, x.Col)
	}
	for i, x := range t.Structs {
		fmt.Fprintf(b, "    struct %d %s fields=%d\n", i, str(x.Type), x.Fields)
	}
	for pc, in := range t.Code {
		fmt.Fprintf(b, "  %4d  T%-3d %d %d %d %d\n", pc, in.Op, in.Dst, in.A, in.B, in.C)
	}
}
