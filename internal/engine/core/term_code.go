package core

import (
	"fmt"

	"zeroparse.dev/zp/core/noderep"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/value"
)

// Term code: a TermProgram as a bytecode module stores it (bytecode.Term), so that a saved program
// runs the terms a compiled one runs. The instructions are stored as they are; the pools that hold
// what the engine resolved from the program (node types, struct kinds, cast types) are stored as the
// names and type texts they were resolved from, and resolved again when the module is loaded.

// Export returns the term code of t, adding the strings it refers to with str, which returns the
// index of a string in the module's string table.
func (t *TermProgram) Export(str func(string) int) (bytecode.Term, error) {
	x := bytecode.Term{Regs: [4]int{int(t.na), int(t.nn), int(t.ns), int(t.nt)}}
	x.Code = make([]bytecode.TermInstr, len(t.code))
	for i, in := range t.code {
		x.Code[i] = bytecode.TermInstr{Op: uint8(in.op), Dst: in.dst, A: in.a, B: in.b, C: in.c}
	}
	for _, v := range t.consts {
		var c bytecode.TermConst
		switch v := v.(type) {
		case nil:
			c.Kind = bytecode.ConstNil
		case int:
			c.Kind, c.Int = bytecode.ConstInt, int64(v)
		case string:
			c.Kind, c.Int = bytecode.ConstString, int64(str(v))
		case bool:
			c.Kind, c.Int = bytecode.ConstBool, int64(b2i(v))
		default:
			return x, fmt.Errorf("core: term constant of type %T", v)
		}
		x.Consts = append(x.Consts, c)
	}
	for _, n := range t.names {
		x.Names = append(x.Names, str(n))
	}
	for _, m := range t.members {
		x.Members = append(x.Members, bytecode.TermMember{Kind: m.kind, Name: str(m.name), Field: str(m.field)})
	}
	for _, k := range t.nodeTypes {
		x.NodeTypes = append(x.NodeTypes, str(k[noderep.BaseList].Type))
	}
	for _, c := range t.casts {
		typ := -1
		if c.t != nil {
			typ = str(c.t.Text)
		}
		x.Casts = append(x.Casts, bytecode.TermCast{Type: typ, Line: c.line, Col: c.col})
	}
	for _, s := range t.structs {
		x.Structs = append(x.Structs, bytecode.TermStruct{Type: str(s.kind.Type), Fields: s.fields})
	}
	return x, nil
}

// TermTypes resolves the names and type texts of term code when a module is loaded.
type TermTypes struct {
	// NodeType returns the kinds of the named node type name, or nil if there is none.
	NodeType func(name string) *value.NodeTypeKinds
	// Struct returns the kind of the nodes of the struct type name, or nil if there is none.
	Struct func(name string) *noderep.Kind
	// Cast returns the type of the resolved type text of a cast, or an error.
	Cast func(text string) (*compile.FieldType, error)
}

// Kinds of the operands of term instructions, for the checks of LoadTerm.
const (
	oNone   = iota // not read
	oA             // a register of the value file
	oN             // a register of the integer file
	oS             // a register of the sequence file
	oT             // a register of the string file
	oJump          // a jump target
	oConst         // an index of Consts
	oName          // an index of Names
	oMember        // an index of Members
	oNode          // an index of NodeTypes
	oCast          // an index of Casts
	oStruct        // an index of Structs
	oSlot          // a capture slot: not negative
	oVar           // a variable of the module
	oUnary         // a unary operator (ops.UnaryOps)
	oBinary        // a binary operator (ops.BinaryOps)
	oLogic         // a logical operator (ops.LogicalOps)
	oFlag          // 0 or 1
	oImm           // any integer
)

// termOperands holds, for each instruction, the kinds of its operands dst, a, b and c.
var termOperands = map[termOp][4]uint8{
	tNop:      {},
	tConst:    {oA, oConst},
	tIntK:     {oN, oImm},
	tMoveA:    {oA, oA},
	tMoveN:    {oN, oN},
	tMoveS:    {oS, oS},
	tMoveT:    {oT, oT},
	tCap:      {oA, oSlot},
	tLocal:    {oA, oName},
	tVar:      {oA, oVar},
	tUnbox:    {oA, oA},
	tFromB:    {oA, oN},
	tFromT:    {oA, oT},
	tNot:      {oN, oN},
	tLt:       {oN, oN, oN},
	tAdd:      {oN, oN, oN},
	tEqN:      {oN, oN, oN},
	tIsNil:    {oN, oA},
	tMember:   {oA, oA, oMember},
	tUnary:    {oA, oA, oUnary},
	tBinary:   {oA, oA, oA, oBinary},
	tLogic:    {oN, oA, oLogic},
	tText:     {oT, oA},
	tTextEq:   {oN, oT, oT},
	tLength:   {oA, oA},
	tConvert:  {oA, oA, oNode},
	tConforms: {oN, oA, oCast},
	tCastErr:  {oA, oA, oCast},
	tKids:     {oN},
	tPush:     {oNone, oA, oName},
	tPushAll:  {oNone, oA, oName},
	tAbove:    {oS, oN},
	tHull:     {oA, oS, oFlag},
	tItems:    {oS, oA, oName},
	tItemAt:   {oA, oS, oN, oFlag},
	tSeqLen:   {oN, oS},
	tStep:     {},
	tKeep:     {oNone, oS, oN},
	tFilter:   {oN, oA},
	tCopy:     {oA, oS},
	tStruct:   {oA, oStruct},
	tSetField: {oNone, oA, oA, oName},
	tCover:    {oN, oA, oA, oN},
	tMade:     {oNone, oA, oN},
	tJump:     {oJump},
	tJumpF:    {oJump, oN},
	tRet:      {oNone, oA},
	tRetErr:   {oNone, oA},
	tField:    {oN, oA, oA, oName},
}

// LoadTerm makes the TermProgram of the term code x of a module with the string table strs and
// vars variables, resolving its types with types. It checks that every operand is in range for its
// kind (a register of its file, a pool index, a jump target in the code, a variable, an operator)
// and that the code ends with a return or a jump, so that running it cannot leave the code.
func LoadTerm(x *bytecode.Term, strs []string, vars int, types TermTypes) (*TermProgram, error) {
	t := new(TermProgram)
	if err := loadTerm(t, x, strs, vars, types); err != nil {
		return nil, err
	}
	return t, nil
}

// LoadTerms makes the TermPrograms of the term codes xs as LoadTerm does, allocating them together.
func LoadTerms(xs []bytecode.Term, strs []string, vars int, types TermTypes) ([]*TermProgram, error) {
	progs := make([]TermProgram, len(xs))
	terms := make([]*TermProgram, len(xs))
	for i := range xs {
		if err := loadTerm(&progs[i], &xs[i], strs, vars, types); err != nil {
			return nil, fmt.Errorf("term %d: %w", i, err)
		}
		terms[i] = &progs[i]
	}
	return terms, nil
}

// loadTerm fills t from x for LoadTerm. Each pool gets its exact capacity up front.
func loadTerm(t *TermProgram, x *bytecode.Term, strs []string, vars int, types TermTypes) error {
	*t = TermProgram{
		na: int32(x.Regs[0]), nn: int32(x.Regs[1]), ns: int32(x.Regs[2]), nt: int32(x.Regs[3]),
		consts:    make([]any, 0, len(x.Consts)),
		names:     make([]string, 0, len(x.Names)),
		members:   make([]termMember, 0, len(x.Members)),
		nodeTypes: make([]*value.NodeTypeKinds, 0, len(x.NodeTypes)),
		casts:     make([]termCast, 0, len(x.Casts)),
		structs:   make([]termStruct, 0, len(x.Structs)),
	}
	for _, n := range x.Regs {
		if n < 0 || n > 1<<20 {
			return fmt.Errorf("invalid register count %d", n)
		}
	}
	str := func(i int) (string, error) {
		if i < 0 || i >= len(strs) {
			return "", fmt.Errorf("invalid string index %d", i)
		}
		return strs[i], nil
	}
	for _, c := range x.Consts {
		switch c.Kind {
		case bytecode.ConstNil:
			t.consts = append(t.consts, nil)
		case bytecode.ConstInt:
			if int64(int(c.Int)) != c.Int {
				return fmt.Errorf("term constant %d exceeds the implementation int range", c.Int)
			}
			t.consts = append(t.consts, int(c.Int))
		case bytecode.ConstString:
			s, err := str(int(c.Int))
			if err != nil {
				return err
			}
			t.consts = append(t.consts, s)
		case bytecode.ConstBool:
			if c.Int != 0 && c.Int != 1 {
				return fmt.Errorf("invalid boolean constant %d", c.Int)
			}
			t.consts = append(t.consts, c.Int == 1)
		default:
			return fmt.Errorf("invalid constant kind %d", c.Kind)
		}
	}
	for _, i := range x.Names {
		s, err := str(i)
		if err != nil {
			return err
		}
		t.names = append(t.names, s)
	}
	for _, m := range x.Members {
		name, err := str(m.Name)
		if err != nil {
			return err
		}
		field, err := str(m.Field)
		if err != nil {
			return err
		}
		if m.Kind < 0 || m.Kind >= len(ops.MemberKinds) {
			return fmt.Errorf("invalid member kind %d", m.Kind)
		}
		t.members = append(t.members, termMember{kind: m.Kind, name: name, field: field})
	}
	for _, i := range x.NodeTypes {
		s, err := str(i)
		if err != nil {
			return err
		}
		k := types.NodeType(s)
		if k == nil {
			return fmt.Errorf("term conversion to %q, which is not a named node type", s)
		}
		t.nodeTypes = append(t.nodeTypes, k)
	}
	for _, c := range x.Casts {
		if c.Line < 0 || c.Col < 0 {
			return fmt.Errorf("invalid cast position %d:%d", c.Line, c.Col)
		}
		var ft *compile.FieldType
		if c.Type >= 0 {
			s, err := str(c.Type)
			if err != nil {
				return err
			}
			if ft, err = types.Cast(s); err != nil {
				return err
			}
		}
		t.casts = append(t.casts, termCast{t: ft, line: c.Line, col: c.Col})
	}
	for _, s := range x.Structs {
		name, err := str(s.Type)
		if err != nil {
			return err
		}
		k := types.Struct(name)
		if k == nil {
			return fmt.Errorf("term struct literal of %q, which is not a struct type", name)
		}
		if s.Fields < 0 {
			return fmt.Errorf("invalid field count %d", s.Fields)
		}
		t.structs = append(t.structs, termStruct{kind: k, fields: s.Fields})
	}
	n := len(x.Code)
	if n == 0 {
		return fmt.Errorf("empty term code")
	}
	limit := func(kind uint8) (int, bool) {
		switch kind {
		case oA:
			return int(t.na), true
		case oN:
			return int(t.nn), true
		case oS:
			return int(t.ns), true
		case oT:
			return int(t.nt), true
		case oJump:
			return n, true
		case oConst:
			return len(t.consts), true
		case oName:
			return len(t.names), true
		case oMember:
			return len(t.members), true
		case oNode:
			return len(t.nodeTypes), true
		case oCast:
			return len(t.casts), true
		case oStruct:
			return len(t.structs), true
		case oVar:
			return vars, true
		case oUnary:
			return len(ops.UnaryOps), true
		case oBinary:
			return len(ops.BinaryOps), true
		case oLogic:
			return len(ops.LogicalOps), true
		case oFlag:
			return 2, true
		}
		return 0, false
	}
	t.code = make([]termInstr, n)
	for pc, in := range x.Code {
		kinds, ok := termOperands[termOp(in.Op)]
		if !ok {
			return fmt.Errorf("invalid term instruction %d at %d", in.Op, pc)
		}
		for k, v := range [4]int32{in.Dst, in.A, in.B, in.C} {
			if kinds[k] == oSlot && v < 0 {
				return fmt.Errorf("invalid slot %d at %d", v, pc)
			}
			if max, ok := limit(kinds[k]); ok && (v < 0 || int(v) >= max) {
				return fmt.Errorf("invalid operand %d of term instruction %d at %d", v, in.Op, pc)
			}
		}
		t.code[pc] = termInstr{op: termOp(in.Op), dst: in.Dst, a: in.A, b: in.B, c: in.C}
	}
	switch t.code[n-1].op {
	case tRet, tRetErr, tJump:
	default:
		return fmt.Errorf("term code does not end with a return or a jump")
	}
	return nil
}
