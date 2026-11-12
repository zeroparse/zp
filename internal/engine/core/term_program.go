package core

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/lower"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
)

// Terms of the engine lowered from Exec IR.
//
// The engine evaluates an action, a predicate or the value of a binding as the function of Exec IR
// that lower.Term makes of its Core IR term, as generated parsers do: the same compositions of the
// operators of terms (term_exec.go), in the same order, with the same failures. The function is
// translated once, when the program is built, into the instructions of a TermProgram, which
// TermProgram.Eval runs. Instructions read and write registers of four files, by the Exec IR type
// of the local they hold: values (Datum, Obj), integers (int and bool), element slices (Seq) and
// strings (Str), so that no integer, slice or string is boxed to be held.
//
// Every composition that can fail is followed in the lowered function by the return of its
// failure; the translation folds the two into the one instruction, whose failure ends Eval.

// TermProgram is a term of the action language compiled for the engine. It is immutable and may be
// shared by parses that run concurrently.
type TermProgram struct {
	code []termInstr
	// The sizes of the register files.
	na, nn, ns, nt int32
	consts         []any
	names          []string // term.name and the names of Operators Rule locals and struct fields
	members        []termMember
	nodeTypes      []*value.NodeTypeKinds
	casts          []termCast
	structs        []termStruct
}

// TermEnv is what the leaves of a term resolve against when it is compiled.
type TermEnv struct {
	Prog *compile.Program
	// Scope holds the capture slots of the frame the term reads.
	Scope *compile.Scope
	// Proj are the projected captures of the rule whose action the term is (lower.TermOptions.Proj),
	// nil for every other term.
	Proj map[string]string
}

type termMember struct {
	kind        int
	name, field string
}

type termCast struct {
	t         *compile.FieldType
	line, col int
}

type termStruct struct {
	kind   *noderep.Kind
	fields int
}

type termOp uint8

const (
	tNop termOp = iota
	// Moves and constants.
	tConst // a[dst] = consts[a]
	tIntK  // n[dst] = a
	tMoveA // a[dst] = a[a]
	tMoveN // n[dst] = n[a]
	tMoveS // s[dst] = s[a]
	tMoveT // t[dst] = t[a]
	tCap   // a[dst] = the capture in slot a of the frame
	tLocal // a[dst] = the Operators Rule local names[a]
	tVar   // a[dst] = the variable in slot a
	tUnbox // a[dst] = unboxed a[a]
	tFromB // a[dst] = bool n[a]
	tFromT // a[dst] = string t[a]
	tNot   // n[dst] = !n[a]
	tLt    // n[dst] = n[a] < n[b]
	tAdd   // n[dst] = n[a] + n[b]
	tEqN   // n[dst] = n[a] == n[b]
	tIsNil // n[dst] = a[a] == nil
	// Compositions; those marked fallible end Eval with their failure.
	tMember   // fallible: a[dst] = member(kind, a[a]) of members[b]
	tUnary    // fallible: a[dst] = unary(b, a[a])
	tBinary   // fallible: a[dst] = binary(c, a[a], a[b])
	tLogic    // fallible: n[dst] = logic(b, a[a])
	tText     // fallible: t[dst] = text(a[a])
	tTextEq   // n[dst] = t[a] == t[b]
	tLength   // fallible: a[dst] = length(a[a])
	tConvert  // fallible: a[dst] = convert(nodeTypes[b], a[a])
	tConforms // n[dst] = conforms(casts[b], a[a])
	tCastErr  // a[dst] = the failure of the cast casts[b] of a[a]
	tKids     // n[dst] = the number of gathered elements
	tPush     // fallible: push(names[b], a[a])
	tPushAll  // fallible: pushItems(names[b], a[a])
	tAbove    // s[dst] = the elements gathered above n[a], taken off
	tHull     // a[dst] = hull(b != 0, s[a])
	tItems    // fallible: s[dst] = items(names[b], a[a])
	tItemAt   // a[dst] = itemAt(s[a], n[b], c != 0)
	tSeqLen   // n[dst] = len(s[a])
	tStep     // charge the step of an element
	tKeep     // keep element n[b] of s[a]
	tFilter   // fallible: n[dst] = filterKeep(a[a])
	tCopy     // a[dst] = a new list of the elements s[a]
	tStruct   // a[dst] = a new struct structs[a]
	tSetField // set field names[c] of the struct a[a] to a[b]
	tCover    // n[dst] = cover(a[a], a[b], n[c])
	tMade     // made(a[a], n[b])
	// Control.
	tJump   // continue at dst
	tJumpF  // continue at dst if n[a] is false
	tRet    // return a[a]
	tRetErr // return the failure a[a]
)

type termInstr struct {
	op      termOp
	dst     int32
	a, b, c int32
}

// The register files.
const (
	fileA = iota // values: Datum, Obj, Fail
	fileN        // int, bool
	fileS        // Seq
	fileT        // Str
)

type termReg struct {
	file int
	r    int32
}

// CompileTerm compiles the Core IR term t, which reads the frame of env.Scope: it lowers t to Exec IR
// and translates the function into the instructions of a TermProgram. An error is an internal
// error: a term that the front end does not build, or a lowering this translation does not know.
func CompileTerm(t ir.Term, env TermEnv) (*TermProgram, error) {
	l, err := lower.Term(t, lower.TermOptions{Proj: env.Proj})
	if err != nil {
		return nil, err
	}
	c := &termCompiler{env: env, m: l.Module, locals: map[string]termReg{}, prog: &TermProgram{}}
	for _, p := range l.Func.Locals {
		c.declare(p.Name, p.Type)
	}
	if err := c.block(l.Func.Body); err != nil {
		return nil, fmt.Errorf("core: term %s: %w", ir.TermString(t), err)
	}
	return c.prog, nil
}

type termCompiler struct {
	env    TermEnv
	m      *exec.Module
	prog   *TermProgram
	locals map[string]termReg
	// loops holds the start and the pending exits of the loops being translated.
}

func fileOf(t exec.Type) (int, bool) {
	switch t.Kind {
	case exec.KBool, exec.KInt:
		return fileN, true
	case exec.KHandle:
		switch t.Handle {
		case ops.Datum, ops.Obj, ops.Fail:
			return fileA, true
		case ops.Seq:
			return fileS, true
		case ops.Str:
			return fileT, true
		}
	}
	return 0, false
}

func (c *termCompiler) reg(file int) termReg {
	p := c.prog
	var n *int32
	switch file {
	case fileA:
		n = &p.na
	case fileN:
		n = &p.nn
	case fileS:
		n = &p.ns
	default:
		n = &p.nt
	}
	r := termReg{file, *n}
	*n++
	return r
}

func (c *termCompiler) declare(name string, t exec.Type) (termReg, error) {
	if r, ok := c.locals[name]; ok {
		return r, nil
	}
	f, ok := fileOf(t)
	if !ok {
		return termReg{}, fmt.Errorf("local %s of type %s", name, t)
	}
	r := c.reg(f)
	c.locals[name] = r
	return r, nil
}

func (c *termCompiler) emit(in termInstr) int {
	c.prog.code = append(c.prog.code, in)
	return len(c.prog.code) - 1
}

// results returns the result types of the operation or function callee.
func (c *termCompiler) results(callee string) []exec.Type {
	if o := c.m.Op(callee); o != nil {
		return o.Results
	}
	if f := c.m.Func(callee); f != nil {
		return f.Results
	}
	return nil
}

func (c *termCompiler) block(b *exec.Block) error {
	for i := 0; i < len(b.Stmts); i++ {
		next := func() exec.Stmt {
			if i+1 < len(b.Stmts) {
				return b.Stmts[i+1]
			}
			return nil
		}
		switch s := b.Stmts[i].(type) {
		case *exec.Let:
			call, isCall := s.Value.(*exec.Call)
			if isCall {
				res := c.results(call.Callee)
				if len(res) == len(s.Names) && len(res) > 0 && res[len(res)-1] == exec.THandle(ops.Fail) {
					// A composition that can fail: its failure must be raised right after it.
					fail := s.Names[len(s.Names)-1]
					if !raises(next(), fail) {
						return fmt.Errorf("the failure %s of %s is not returned right after it", fail, call.Callee)
					}
					i++
					var dst termReg
					if len(s.Names) == 2 {
						var err error
						if dst, err = c.declare(s.Names[0], res[0]); err != nil {
							return err
						}
					}
					if err := c.fallible(call, dst); err != nil {
						return err
					}
					continue
				}
			}
			if len(s.Names) != 1 {
				return fmt.Errorf("let of %d names", len(s.Names))
			}
			t, err := c.typeOf(s.Value)
			if err != nil {
				return err
			}
			dst, err := c.declare(s.Names[0], t)
			if err != nil {
				return err
			}
			if err := c.exprInto(s.Value, dst); err != nil {
				return err
			}
		case *exec.Set:
			if len(s.Dst) != 1 || len(s.Values) != 1 || s.Dst[0].Reg {
				return fmt.Errorf("set of %d places", len(s.Dst))
			}
			dst, ok := c.locals[s.Dst[0].Name]
			if !ok {
				return fmt.Errorf("set of undeclared %s", s.Dst[0].Name)
			}
			if err := c.exprInto(s.Values[0], dst); err != nil {
				return err
			}
		case *exec.Do:
			res := c.results(s.Call.Callee)
			if len(res) == 1 && res[0] == exec.THandle(ops.Fail) {
				return fmt.Errorf("the failure of %s is discarded", s.Call.Callee)
			}
			if err := c.effect(s.Call); err != nil {
				return err
			}
		case *exec.If:
			if s.Init != nil {
				return fmt.Errorf("if with an init")
			}
			cond, err := c.expr(s.Cond)
			if err != nil {
				return err
			}
			jf := c.emit(termInstr{op: tJumpF, a: cond.r})
			if err := c.block(s.Then); err != nil {
				return err
			}
			if s.Else != nil {
				j := c.emit(termInstr{op: tJump})
				c.prog.code[jf].dst = int32(len(c.prog.code))
				if err := c.block(s.Else); err != nil {
					return err
				}
				c.prog.code[j].dst = int32(len(c.prog.code))
			} else {
				c.prog.code[jf].dst = int32(len(c.prog.code))
			}
		case *exec.Loop:
			if s.Init != nil || s.Cond == nil {
				return fmt.Errorf("loop without the form of a for loop")
			}
			top := int32(len(c.prog.code))
			cond, err := c.expr(s.Cond)
			if err != nil {
				return err
			}
			jf := c.emit(termInstr{op: tJumpF, a: cond.r})
			if err := c.block(s.Body); err != nil {
				return err
			}
			if s.Post != nil {
				if err := c.block(&exec.Block{Stmts: []exec.Stmt{s.Post}}); err != nil {
					return err
				}
			}
			c.emit(termInstr{op: tJump, dst: top})
			c.prog.code[jf].dst = int32(len(c.prog.code))
		case *exec.Return:
			if len(s.Values) != 2 {
				return fmt.Errorf("return of %d values", len(s.Values))
			}
			if isNull(s.Values[1]) {
				v, err := c.expr(s.Values[0])
				if err != nil {
					return err
				}
				c.emit(termInstr{op: tRet, a: v.r})
				continue
			}
			if !isNull(s.Values[0]) {
				return fmt.Errorf("return of a value and a failure")
			}
			f, err := c.expr(s.Values[1])
			if err != nil {
				return err
			}
			c.emit(termInstr{op: tRetErr, a: f.r})
		case *exec.Comment:
		default:
			return fmt.Errorf("statement %T", s)
		}
	}
	return nil
}

func isNull(e exec.Expr) bool {
	k, ok := e.(*exec.Const)
	return ok && k.Type.Kind == exec.KHandle && k.Value == nil
}

// raises reports whether s is the return of the failure in the local fail when it is not null, as
// the lowering of terms follows a call that can fail.
func raises(s exec.Stmt, fail string) bool {
	f, ok := s.(*exec.If)
	if !ok || f.Init != nil || f.Else != nil || len(f.Then.Stmts) != 1 {
		return false
	}
	cond, ok := f.Cond.(*exec.Binary)
	if !ok || cond.Op != "!=" || !isLocal(cond.X, fail) || !isNull(cond.Y) {
		return false
	}
	r, ok := f.Then.Stmts[0].(*exec.Return)
	return ok && len(r.Values) == 2 && isNull(r.Values[0]) && isLocal(r.Values[1], fail)
}

func isLocal(e exec.Expr, name string) bool {
	l, ok := e.(*exec.Local)
	return ok && l.Name == name
}

// typeOf returns the type of the expression e of one value.
func (c *termCompiler) typeOf(e exec.Expr) (exec.Type, error) {
	switch e := e.(type) {
	case *exec.Const:
		return e.Type, nil
	case *exec.Local:
		r, ok := c.locals[e.Name]
		if !ok {
			return exec.Type{}, fmt.Errorf("undeclared local %s", e.Name)
		}
		switch r.file {
		case fileA:
			return exec.THandle(ops.Datum), nil
		case fileN:
			return exec.TInt, nil
		case fileS:
			return exec.THandle(ops.Seq), nil
		default:
			return exec.THandle(ops.Str), nil
		}
	case *exec.Unary:
		return exec.TBool, nil
	case *exec.Binary:
		if e.Op == "+" {
			return exec.TInt, nil
		}
		return exec.TBool, nil
	case *exec.Call:
		res := c.results(e.Callee)
		if len(res) != 1 {
			return exec.Type{}, fmt.Errorf("%s has %d results", e.Callee, len(res))
		}
		return res[0], nil
	}
	return exec.Type{}, fmt.Errorf("expression %T", e)
}

// expr translates e and returns the register that holds its value.
func (c *termCompiler) expr(e exec.Expr) (termReg, error) {
	if l, ok := e.(*exec.Local); ok {
		r, ok := c.locals[l.Name]
		if !ok {
			return termReg{}, fmt.Errorf("undeclared local %s", l.Name)
		}
		return r, nil
	}
	t, err := c.typeOf(e)
	if err != nil {
		return termReg{}, err
	}
	f, ok := fileOf(t)
	if !ok {
		return termReg{}, fmt.Errorf("value of type %s", t)
	}
	dst := c.reg(f)
	return dst, c.exprInto(e, dst)
}

// exprInto translates e into the register dst.
func (c *termCompiler) exprInto(e exec.Expr, dst termReg) error {
	switch e := e.(type) {
	case *exec.Const:
		switch e.Type.Kind {
		case exec.KBool:
			v := int32(0)
			if e.Value.(bool) {
				v = 1
			}
			c.emit(termInstr{op: tIntK, dst: dst.r, a: v})
		case exec.KInt:
			c.emit(termInstr{op: tIntK, dst: dst.r, a: int32(e.Value.(int64))})
		case exec.KHandle:
			if e.Value != nil || dst.file != fileA {
				return fmt.Errorf("constant of type %s", e.Type)
			}
			c.emit(termInstr{op: tConst, dst: dst.r, a: c.constant(nil)})
		default:
			return fmt.Errorf("constant of type %s", e.Type)
		}
		return nil
	case *exec.Local:
		src, ok := c.locals[e.Name]
		if !ok {
			return fmt.Errorf("undeclared local %s", e.Name)
		}
		if src.file != dst.file {
			return fmt.Errorf("move between register files")
		}
		if src == dst {
			return nil
		}
		op := [...]termOp{fileA: tMoveA, fileN: tMoveN, fileS: tMoveS, fileT: tMoveT}[src.file]
		c.emit(termInstr{op: op, dst: dst.r, a: src.r})
		return nil
	case *exec.Unary:
		if e.Op != "!" {
			return fmt.Errorf("unary %s", e.Op)
		}
		x, err := c.expr(e.X)
		if err != nil {
			return err
		}
		c.emit(termInstr{op: tNot, dst: dst.r, a: x.r})
		return nil
	case *exec.Binary:
		x, err := c.expr(e.X)
		if err != nil {
			return err
		}
		if x.file == fileA && isNull(e.Y) && (e.Op == "==" || e.Op == "!=") {
			c.emit(termInstr{op: tIsNil, dst: dst.r, a: x.r})
			if e.Op == "!=" {
				c.emit(termInstr{op: tNot, dst: dst.r, a: dst.r})
			}
			return nil
		}
		y, err := c.expr(e.Y)
		if err != nil {
			return err
		}
		if x.file != fileN || y.file != fileN {
			return fmt.Errorf("binary %s of handles", e.Op)
		}
		switch e.Op {
		case "<":
			c.emit(termInstr{op: tLt, dst: dst.r, a: x.r, b: y.r})
		case "+":
			c.emit(termInstr{op: tAdd, dst: dst.r, a: x.r, b: y.r})
		case "==", "!=":
			c.emit(termInstr{op: tEqN, dst: dst.r, a: x.r, b: y.r})
			if e.Op == "!=" {
				c.emit(termInstr{op: tNot, dst: dst.r, a: dst.r})
			}
		default:
			return fmt.Errorf("binary %s", e.Op)
		}
		return nil
	case *exec.Call:
		return c.call(e, dst)
	}
	return fmt.Errorf("expression %T", e)
}

func (c *termCompiler) constant(v any) int32 {
	c.prog.consts = append(c.prog.consts, v)
	return int32(len(c.prog.consts) - 1)
}

func (c *termCompiler) name(s string) int32 {
	for i, n := range c.prog.names {
		if n == s {
			return int32(i)
		}
	}
	c.prog.names = append(c.prog.names, s)
	return int32(len(c.prog.names) - 1)
}

// args translates the arguments of the call e at the positions idx and returns their registers.
func (c *termCompiler) args(e *exec.Call, idx ...int) ([]int32, error) {
	rs := make([]int32, len(idx))
	for i, j := range idx {
		if j >= len(e.Args) {
			return nil, fmt.Errorf("%s has %d arguments", e.Callee, len(e.Args))
		}
		r, err := c.expr(e.Args[j])
		if err != nil {
			return nil, err
		}
		rs[i] = r.r
	}
	return rs, nil
}

// intImm returns the integer constant that is argument i of the call e.
func intImm(e *exec.Call, i int) (int32, error) {
	if i < len(e.Args) {
		if k, ok := e.Args[i].(*exec.Const); ok {
			switch v := k.Value.(type) {
			case int64:
				return int32(v), nil
			case bool:
				if v {
					return 1, nil
				}
				return 0, nil
			}
		}
	}
	return 0, fmt.Errorf("argument %d of %s is not a constant", i, e.Callee)
}

// immArg returns the immediate operand of the call that is argument i of e, which must be of the
// operation op.
func immArg(e *exec.Call, i int, op string) (any, error) {
	if i < len(e.Args) {
		if a, ok := e.Args[i].(*exec.Call); ok && a.Callee == op {
			return a.Imm, nil
		}
	}
	return nil, fmt.Errorf("argument %d of %s is not %s", i, e.Callee, op)
}

// fallible translates the call e of a composition that returns its failure, with its value, if it
// has one, into dst.
func (c *termCompiler) fallible(e *exec.Call, dst termReg) error {
	in := termInstr{dst: dst.r}
	switch e.Callee {
	case "member":
		kind, err := intImm(e, 1)
		if err != nil {
			return err
		}
		name, err := immArg(e, 3, lower.OpTermName)
		if err != nil {
			return err
		}
		field, err := immArg(e, 4, lower.OpTermField)
		if err != nil {
			return err
		}
		rs, err := c.args(e, 2)
		if err != nil {
			return err
		}
		c.prog.members = append(c.prog.members, termMember{kind: int(kind), name: name.(string), field: field.(lower.TermField).Field})
		in.op, in.a, in.b = tMember, rs[0], int32(len(c.prog.members)-1)
	case "unary", "logic":
		op, err := intImm(e, 0)
		if err != nil {
			return err
		}
		rs, err := c.args(e, 1)
		if err != nil {
			return err
		}
		in.op, in.a, in.b = tUnary, rs[0], op
		if e.Callee == "logic" {
			in.op = tLogic
		}
	case "binary":
		op, err := intImm(e, 0)
		if err != nil {
			return err
		}
		rs, err := c.args(e, 1, 2)
		if err != nil {
			return err
		}
		in.op, in.a, in.b, in.c = tBinary, rs[0], rs[1], op
	case "text", "length", "filterKeep":
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a = map[string]termOp{"text": tText, "length": tLength, "filterKeep": tFilter}[e.Callee], rs[0]
	case "convert":
		nt, err := immArg(e, 0, lower.OpTermNodeType)
		if err != nil {
			return err
		}
		kinds := c.env.Prog.NodeKinds[nt.(string)]
		if kinds == nil {
			return fmt.Errorf("no node type %s", nt)
		}
		rs, err := c.args(e, 1)
		if err != nil {
			return err
		}
		c.prog.nodeTypes = append(c.prog.nodeTypes, kinds)
		in.op, in.a, in.b = tConvert, rs[0], int32(len(c.prog.nodeTypes)-1)
	case "push", "pushItems", "items":
		at := 2
		nameAt := 1
		if e.Callee == "items" {
			at, nameAt = 1, 0
		}
		name, err := immArg(e, nameAt, lower.OpTermName)
		if err != nil {
			return err
		}
		rs, err := c.args(e, at)
		if err != nil {
			return err
		}
		in.op, in.a, in.b = map[string]termOp{"push": tPush, "pushItems": tPushAll, "items": tItems}[e.Callee], rs[0], c.name(name.(string))
	default:
		return fmt.Errorf("composition %s", e.Callee)
	}
	c.emit(in)
	return nil
}

// effect translates the call e, whose results are discarded.
func (c *termCompiler) effect(e *exec.Call) error {
	switch e.Callee {
	case "seq.step":
		c.emit(termInstr{op: tStep})
	case lower.OpTermKeep:
		rs, err := c.args(e, 0, 1)
		if err != nil {
			return err
		}
		c.emit(termInstr{op: tKeep, a: rs[0], b: rs[1]})
	case lower.OpTermSetField:
		rs, err := c.args(e, 0, 1)
		if err != nil {
			return err
		}
		c.emit(termInstr{op: tSetField, a: rs[0], b: rs[1], c: c.name(e.Imm.(string))})
	case "made":
		rs, err := c.args(e, 1, 2)
		if err != nil {
			return err
		}
		c.emit(termInstr{op: tMade, a: rs[0], b: rs[1]})
	default:
		return fmt.Errorf("operation %s for its effect", e.Callee)
	}
	return nil
}

// call translates the call e of one result into dst.
func (c *termCompiler) call(e *exec.Call, dst termReg) error {
	in := termInstr{dst: dst.r}
	switch e.Callee {
	case lower.OpTermConst:
		in.op, in.a = tConst, c.constant(e.Imm.(lower.TermConst).Value)
	case lower.OpTermRead:
		switch x := e.Imm.(lower.TermRead).Term.(type) {
		case *ir.SlotRef:
			slot, ok := c.env.Scope.Slots[x.Name]
			if !ok {
				return fmt.Errorf("capture %s is not in scope", x.Name)
			}
			in.op, in.a = tCap, int32(slot)
		case *ir.Local:
			in.op, in.a = tLocal, c.name(x.Name)
		case *ir.VarRef:
			slot := c.env.Prog.Vars.Index(x.Name)
			if slot < 0 {
				return fmt.Errorf("variable %s is not declared", x.Name)
			}
			in.op, in.a = tVar, int32(slot)
		default:
			return fmt.Errorf("read of %T", x)
		}
	case "val.unbox":
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a = tUnbox, rs[0]
	case "val.fromBool":
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a = tFromB, rs[0]
	case lower.OpTermFromStr:
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a = tFromT, rs[0]
	case lower.OpTermFromObj:
		// A struct, list or tuple is held as the value it is.
		return c.exprInto(e.Args[0], dst)
	case lower.OpTermTextEqual:
		rs, err := c.args(e, 0, 1)
		if err != nil {
			return err
		}
		in.op, in.a, in.b = tTextEq, rs[0], rs[1]
	case "conforms":
		ct, err := immArg(e, 0, lower.OpTermCastType)
		if err != nil {
			return err
		}
		rs, err := c.args(e, 1)
		if err != nil {
			return err
		}
		in.op, in.a, in.b = tConforms, rs[0], c.cast(ct.(lower.TermCast).Cast)
	case lower.OpTermCastError:
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a, in.b = tCastErr, rs[0], c.cast(e.Imm.(lower.TermCast).Cast)
	case "term.kids":
		in.op = tKids
	case lower.OpTermElemsAbove:
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a = tAbove, rs[0]
	case "hull":
		tuple, err := intImm(e, 1)
		if err != nil {
			return err
		}
		rs, err := c.args(e, 2)
		if err != nil {
			return err
		}
		in.op, in.a, in.b = tHull, rs[0], tuple
	case "itemAt":
		right, err := intImm(e, 2)
		if err != nil {
			return err
		}
		rs, err := c.args(e, 0, 1)
		if err != nil {
			return err
		}
		in.op, in.a, in.b, in.c = tItemAt, rs[0], rs[1], right
	case "seq.len":
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a = tSeqLen, rs[0]
	case lower.OpTermCopyList:
		rs, err := c.args(e, 0)
		if err != nil {
			return err
		}
		in.op, in.a = tCopy, rs[0]
	case lower.OpTermStruct:
		t := e.Imm.(lower.TermNew).New
		c.prog.structs = append(c.prog.structs, termStruct{kind: c.env.Prog.TypeKind(t.Type), fields: len(t.Fields)})
		in.op, in.a = tStruct, int32(len(c.prog.structs)-1)
	case "cover":
		rs, err := c.args(e, 0, 1, 2)
		if err != nil {
			return err
		}
		in.op, in.a, in.b, in.c = tCover, rs[0], rs[1], rs[2]
	default:
		return fmt.Errorf("operation %s", e.Callee)
	}
	c.emit(in)
	return nil
}

func (c *termCompiler) cast(x *ir.Cast) int32 {
	c.prog.casts = append(c.prog.casts, termCast{t: c.env.Prog.CastType(x.Key.(*grammar.Cast)), line: x.Line, col: x.Col})
	return int32(len(c.prog.casts) - 1)
}
