package execlower

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/execcore"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Terms of the action language lowered to Exec IR.
//
// A term (an action, a predicate or the value of a binding) becomes a function of Exec IR that calls
// the compositions of the operators of terms (binary, unary, logic, member, length, text, convert,
// conforms) of the parser VM, in the order the term evaluates its operands. Each call returns its value
// and a failure; the failure is checked right after the call and raised by term.raise, which ends the
// evaluation of the term as the target's evaluation error does. The logical operators && and || are
// branches: the right operand is evaluated only if the left one does not decide the result.
//
// The leaves of a term (constants, captures, locals and variables), the context of the evaluation and
// the static operands of the compositions (a member's name, a node type, the checked type of a cast)
// are transitional operations whose immediate operand the target prints, as its term printer does.

// ErrUnsupported reports a term that holds a kind of term that Term does not lower yet; the caller
// prints the whole term the way it did before.
var ErrUnsupported = errors.New("lower: term kind not lowered")

// TermConst is the immediate operand of term.const: an int, a string, a bool or nil.
type TermConst struct{ Value any }

func (c TermConst) String() string {
	if s, ok := c.Value.(string); ok {
		return strconv.Quote(s)
	}
	return fmt.Sprint(c.Value)
}

// TermRead is the immediate operand of term.read: a ir.SlotRef, ir.Local or ir.VarRef.
type TermRead struct{ Term ir.Term }

func (r TermRead) String() string {
	switch t := r.Term.(type) {
	case *ir.SlotRef:
		return "$" + t.Name
	case *ir.Local:
		return "$" + t.Name
	case *ir.VarRef:
		return "var " + t.Name
	}
	return fmt.Sprintf("%T", r.Term)
}

// TermCast is the immediate operand of term.castType and term.castError: the cast.
type TermCast struct{ Cast *ir.Cast }

func (c TermCast) String() string { return fmt.Sprintf("cast %d:%d", c.Cast.Line, c.Cast.Col) }

// Names of the operations that lowered terms call besides the compositions.
const (
	OpTermConst      = "term.const"
	OpTermRead       = "term.read"
	OpTermCtx        = "term.ctx"
	OpTermName       = "term.name"
	OpTermField      = "term.field"
	OpTermNodeType   = "term.nodeType"
	OpTermCastType   = "term.castType"
	OpTermRaise      = "term.raise"
	OpTermCastError  = "term.castError"
	OpTermTextEqual  = "term.textEqual"
	OpTermFromStr    = "term.fromStr"
	OpTermSpend      = "term.spend"
	OpTermElemsAbove = "term.elemsAbove"
	OpTermStruct     = "term.struct"
	OpTermSetField   = "term.setField"
	OpTermFromObj    = "term.fromObj"
	// OpTermNew is the prefix of term.newN, the call of the constructor of a struct type with N
	// declared fields (TermOptions.StructCall).
	OpTermNew = "term.new"
)

// opTermKids is the operation of the parser VM that returns the base of the elements of a list or a
// tuple that starts being made: the number of elements gathered so far (act.push).
const opTermKids = "term.kids"

// TermField is the immediate operand of term.field: the name of a member (Name) and the name of the
// field or capture that it reads (Field), which is Name without the $ of a capture.
type TermField struct{ Name, Field string }

func (f TermField) String() string { return f.Field + " of " + f.Name }

// TermNew is the immediate operand of term.newN: the struct literal.
type TermNew struct{ New *ir.New }

func (n TermNew) String() string { return n.New.Type }

// TermCompositions are the compositions of the parser VM that lowered terms call.
var TermCompositions = []string{"binary", "unary", "logic", "member", "length", "text", "convert", "conforms",
	"push", "pushItems", "hull", "cover", "made"}

// TermLowered is a term lowered to Exec IR.
type TermLowered struct {
	// Module declares what Func calls: the compositions of the operators of terms with what they call,
	// and the operations of the leaves.
	Module *exec.Module
	Func   *exec.Func
	// Result is the local of Func (Func.Locals) that holds the term's value at the end of Func's body,
	// or "" if the body returns it.
	Result string
}

// TermOptions configures Term.
type TermOptions struct {
	// Result, if not empty, names a local of the lowered function (Func.Locals) that its body assigns
	// the term's value to last; otherwise the body returns the value. A printer that prints the body
	// without the function declares the local around it.
	Result string
	// Steps charges one step of the budget for each subterm, before its operands, as the target's
	// evaluation of terms does under budget controls (term.spend).
	Steps bool
	// StructCall makes a struct literal one call of the target's constructor of its type (term.newN),
	// with the field values evaluated in the order written and passed in declaration order, nil for a
	// field not given. Otherwise the struct is made of its fields by term.struct, term.setField, cover
	// and made.
	StructCall bool
	// Unbounded leaves out the steps that elements charge (seq.step), for a target whose parser
	// does not bound the work and prints no charge.
	Unbounded bool
}

// Term lowers the term t, or returns ErrUnsupported if t holds a kind of term that it does not lower
// (a built-in that takes a function, a lambda or a lambda parameter).
func Term(t ir.Term, o TermOptions) (*TermLowered, error) {
	if !TermSupported(t) {
		return nil, ErrUnsupported
	}
	m, err := termModule()
	if err != nil {
		return nil, err
	}
	datum := exec.THandle(ops.Datum)
	var results []exec.Type
	if o.Result == "" {
		results = []exec.Type{datum}
	}
	l := &termLowerer{m: m, steps: o.Steps, structCall: o.StructCall, unbounded: o.Unbounded}
	fn := m.AddFunc("term", nil, results, nil, func(b *exec.Builder) {
		var res exec.Expr
		if o.Result != "" {
			res = b.Var(o.Result, datum)
		}
		v := l.value(b, t)
		if res != nil {
			b.Set(o.Result, v)
		} else {
			b.Return(v)
		}
	})
	if err := exec.Verify(m); err != nil {
		return nil, fmt.Errorf("lower term: %w", err)
	}
	return &TermLowered{Module: m, Func: fn, Result: o.Result}, nil
}

// TermSupported reports whether Term lowers t.
func TermSupported(t ir.Term) bool {
	ok := true
	ir.WalkTerm(t, func(x ir.Term) {
		switch x := x.(type) {
		case *ir.Const, *ir.SlotRef, *ir.VarRef, *ir.Member, *ir.Binary, *ir.Unary, *ir.Convert, *ir.Cast,
			*ir.Tuple, *ir.New:
		case *ir.Local:
			ok = ok && x.Kind != ir.LocalParam
		case *ir.Builtin:
			switch x.Func {
			case "len", "text":
				ok = ok && len(x.Args) == 1
			case "list", "concat":
			default:
				ok = false
			}
		default:
			ok = false
		}
	})
	return ok
}

var (
	termBaseOnce sync.Once
	termBase     *exec.Module
	termBaseErr  error
)

// termModule returns a new module for one lowered term: the declarations of the parser VM, the
// operations of the leaves, and the compositions of the operators of terms with every function they
// call.
func termModule() (*exec.Module, error) {
	termBaseOnce.Do(func() {
		base := execcore.Build()
		m := &exec.Module{Name: "term", Features: base.Features, Components: slices.Clip(base.Components),
			Registers: base.Registers, Handles: base.Handles, Ops: slices.Clip(base.Ops)}
		m.AddComponent(Component, "The state of the hand-written runtime.")
		addTermOps(m)
		seen := map[string]bool{}
		var add func(name string)
		add = func(name string) {
			f := base.Func(name)
			if f == nil || seen[name] {
				return
			}
			seen[name] = true
			exec.Walk(f.Body, nil, func(e exec.Expr) {
				if c, ok := e.(*exec.Call); ok {
					add(c.Callee)
				}
			})
			m.Funcs = append(m.Funcs, f)
		}
		for _, name := range TermCompositions {
			if base.Func(name) == nil {
				termBaseErr = fmt.Errorf("lower: no composition %s", name)
				return
			}
			add(name)
		}
		m.Funcs = slices.Clip(m.Funcs)
		termBase = m
	})
	if termBaseErr != nil {
		return nil, termBaseErr
	}
	// The slices are clipped, so that what a term adds to its module (term.newN, its function) does not
	// write into those of the base, which other terms share.
	m := *termBase
	m.Ops, m.Funcs = slices.Clip(m.Ops), slices.Clip(m.Funcs)
	return &m, nil
}

// addTermOps declares the operations of the leaves of terms.
func addTermOps(m *exec.Module) {
	datum, failT := exec.THandle(ops.Datum), exec.THandle(ops.Fail)
	str := exec.THandle(ops.Str)
	reads := []string{Component}
	op := func(o *exec.Op) {
		o.Component = Component
		o.Transitional = true
		m.AddOp(o)
	}
	op(&exec.Op{Name: OpTermConst, Results: []exec.Type{datum}, Imm: "the constant (TermConst)",
		Contract: "Returns the constant in Imm."})
	op(&exec.Op{Name: OpTermRead, Results: []exec.Type{datum}, Reads: reads, Imm: "the capture, local or variable (TermRead)",
		Contract: "Returns the value of the capture, the local or the variable in Imm, as the term printer reads it."})
	op(&exec.Op{Name: OpTermCtx, Results: []exec.Type{exec.THandle(ops.Act)}, Reads: reads,
		Contract: "Returns the context of the evaluation of the term."})
	op(&exec.Op{Name: OpTermName, Results: []exec.Type{exec.THandle(ops.Name)}, Imm: "the name (a string)",
		Contract: "Returns the name in Imm."})
	op(&exec.Op{Name: OpTermField, Results: []exec.Type{exec.THandle(ops.Name)}, Imm: "the member and the name it reads (TermField)",
		Contract: "Returns the name of the field or the capture that the member in Imm reads. A target whose comparisons of names charge less for the same string data than for equal bytes cuts it from the name of the member, as its evaluation of terms does, so that the charge does not depend on whether equal literals share their data."})
	op(&exec.Op{Name: OpTermNodeType, Results: []exec.Type{exec.THandle(ops.NodeType)}, Reads: reads, Imm: "the name of the node type (a string)",
		Contract: "Returns the named node type in Imm."})
	op(&exec.Op{Name: OpTermCastType, Results: []exec.Type{exec.THandle(ops.FType)}, Imm: "the cast (TermCast)",
		Contract: "Returns the checked type of the cast in Imm."})
	op(&exec.Op{Name: OpTermRaise, Params: []exec.Param{exec.P("f", failT)}, Reads: reads, Writes: reads, NoReturn: true,
		Contract: "Ends the evaluation of the term with the failure f."})
	op(&exec.Op{Name: OpTermCastError, Params: []exec.Param{exec.P("v", datum)}, Reads: reads, Writes: reads, NoReturn: true,
		Imm: "the cast (TermCast)", Contract: "Ends the evaluation of the term with the failure of the cast in Imm of the value v."})
	op(&exec.Op{Name: OpTermTextEqual, Params: []exec.Param{exec.P("l", str), exec.P("r", str)}, Results: []exec.Type{exec.TBool},
		Reads: reads, Writes: reads, Contract: "Returns whether the strings l and r are equal, comparing them as == compares two strings."})
	op(&exec.Op{Name: OpTermSpend, Reads: reads, Writes: reads, NoReturn: true, Cost: exec.Cost{Const: 1},
		Contract: "Charges one step of the budget for a subterm, under budget controls."})
	op(&exec.Op{Name: OpTermFromStr, Params: []exec.Param{exec.P("s", str)}, Results: []exec.Type{datum},
		Contract: "Returns the string s as a value."})
	obj, seq := exec.THandle(ops.Obj), exec.THandle(ops.Seq)
	op(&exec.Op{Name: OpTermFromObj, Params: []exec.Param{exec.P("n", obj)}, Results: []exec.Type{datum},
		Contract: "Returns the struct, list value or tuple value n as a value."})
	op(&exec.Op{Name: OpTermElemsAbove, Params: []exec.Param{exec.P("base", exec.TInt)}, Results: []exec.Type{seq}, Reads: reads, Writes: reads,
		Contract: "Takes the elements gathered above base, which term.kids returned, off the gathered elements and returns them, for hull to take over."})
	op(&exec.Op{Name: OpTermStruct, Results: []exec.Type{obj}, Reads: reads, Writes: reads, NoReturn: true, Imm: "the struct literal (TermNew)",
		Contract: "Returns a new struct of the type of the literal in Imm with no field set and an empty range, with room for the fields the literal gives."})
	op(&exec.Op{Name: OpTermSetField, Params: []exec.Param{exec.P("n", obj), exec.P("v", datum)}, Reads: reads, Writes: reads, NoReturn: true,
		Imm:      "the name of the field (a string)",
		Contract: "Sets the field in Imm of the struct n, which a term is making, to v. A target that bounds the work charges for the fields it scans and the bytes of the names it compares."})
}

type termLowerer struct {
	m          *exec.Module
	n          int
	steps      bool
	structCall bool
	unbounded  bool
}

// step charges the step of an element (seq.step).
func (l *termLowerer) step(b *exec.Builder) {
	if !l.unbounded {
		b.Do("seq.step")
	}
}

// local returns a fresh name of a local with the prefix p.
func (l *termLowerer) local(p string) string {
	l.n++
	return p + strconv.Itoa(l.n)
}

// fallible lets the results of the call c of a composition that returns a value and a failure,
// raises the failure, and returns the value.
func (l *termLowerer) fallible(b *exec.Builder, prefix string, c *exec.Call) exec.Expr {
	n := l.local("")
	r := b.LetN([]string{prefix + n, "tf" + n}, c)
	b.If(exec.Bin("!=", r[1], exec.Null(exec.THandle(ops.Fail))), func(b *exec.Builder) { b.Do(OpTermRaise, r[1]) })
	return r[0]
}

func imm(callee string, x any) *exec.Call { return &exec.Call{Callee: callee, Imm: x} }

// code returns the code of the operator sym in the list of operators.
func code(list []string, sym string) exec.Expr {
	i := slices.Index(list, sym)
	if i < 0 {
		panic("lower: unknown operator " + sym)
	}
	return exec.Int(int64(i))
}

// value lowers t, appending the statements that compute its operands to b, and returns its value: a
// local or the call of a leaf.
func (l *termLowerer) value(b *exec.Builder, t ir.Term) exec.Expr {
	if l.steps {
		b.Do(OpTermSpend)
	}
	switch t := t.(type) {
	case *ir.Const:
		return imm(OpTermConst, TermConst{t.Value})
	case *ir.SlotRef, *ir.Local:
		return imm(OpTermRead, TermRead{t})
	case *ir.VarRef:
		// Reading a variable charges the budget, so it is read where the term reads it, before the
		// operands that follow.
		return b.Let(l.local("tv"), imm(OpTermRead, TermRead{t}))
	case *ir.Member:
		x := l.value(b, t.X)
		kind, field := memberKind(t.Name)
		return l.fallible(b, "tv", exec.C("member", exec.C(OpTermCtx), exec.Int(int64(kind)), x,
			imm(OpTermName, t.Name), imm(OpTermField, TermField{t.Name, field})))
	case *ir.Unary:
		x := l.value(b, t.X)
		return l.fallible(b, "tv", exec.C("unary", code(ops.UnaryOps, t.Op), x))
	case *ir.Binary:
		switch t.Op {
		case "&&", "||":
			return l.logic(b, t)
		case "==", "!=":
			if lx, ok := textArg(t.L); ok {
				if rx, ok := textArg(t.R); ok {
					// Two texts compare as strings, without making a value of either.
					ls := l.fallible(b, "ts", exec.C("text", l.value(b, lx)))
					rs := l.fallible(b, "ts", exec.C("text", l.value(b, rx)))
					var eq exec.Expr = exec.C(OpTermTextEqual, ls, rs)
					if t.Op == "!=" {
						eq = exec.Not(eq)
					}
					return exec.C("val.fromBool", eq)
				}
			}
		}
		x := l.value(b, t.L)
		y := l.value(b, t.R)
		return l.fallible(b, "tv", exec.C("binary", code(ops.BinaryOps, t.Op), x, y))
	case *ir.Tuple:
		return l.sequence(b, true, "tuple", l.values(b, t.Elems))
	case *ir.New:
		return l.structLiteral(b, t)
	case *ir.Builtin:
		switch t.Func {
		case "list":
			return l.sequence(b, false, "list", l.values(b, t.Args))
		case "concat":
			if fusable(t) {
				// The elements of the arguments are pushed as they are made, without making the lists of
				// the arguments.
				base := b.Let(l.local("tm"), exec.C(opTermKids))
				l.concatParts(b, t)
				return l.hull(b, false, base)
			}
			vals := l.values(b, t.Args)
			base := b.Let(l.local("tm"), exec.C(opTermKids))
			for _, v := range vals {
				l.step(b)
				if l.steps {
					b.Do(OpTermSpend)
				}
				l.check(b, exec.C("pushItems", exec.C(OpTermCtx), imm(OpTermName, "concat"), v))
			}
			return l.hull(b, false, base)
		}
		x := l.value(b, t.Args[0])
		if t.Func == "len" {
			return l.fallible(b, "tv", exec.C("length", x))
		}
		return exec.C(OpTermFromStr, l.fallible(b, "ts", exec.C("text", x)))
	case *ir.Convert:
		x := l.value(b, t.X)
		return l.fallible(b, "tv", exec.C("convert", imm(OpTermNodeType, t.NodeType), x))
	case *ir.Cast:
		x := b.Let(l.local("tc"), l.value(b, t.X))
		b.If(exec.Not(exec.C("conforms", imm(OpTermCastType, TermCast{t}), x)), func(b *exec.Builder) {
			b.Append(&exec.Do{Call: &exec.Call{Callee: OpTermCastError, Args: []exec.Expr{x}, Imm: TermCast{t}}})
		})
		return x
	}
	panic(fmt.Sprintf("lower: term %T", t))
}

// values lowers the terms ts in order and returns their values.
func (l *termLowerer) values(b *exec.Builder, ts []ir.Term) []exec.Expr {
	vals := make([]exec.Expr, len(ts))
	for i, t := range ts {
		vals[i] = l.value(b, t)
	}
	return vals
}

// check lets the failure that the call c of a composition returns and raises it.
func (l *termLowerer) check(b *exec.Builder, c *exec.Call) {
	f := b.Let(l.local("tf"), c)
	b.If(exec.Bin("!=", f, exec.Null(exec.THandle(ops.Fail))), func(b *exec.Builder) { b.Do(OpTermRaise, f) })
}

// sequence makes the tuple value, if tuple, or the list value of vals, the values of the elements that
// the literal or the built-in function fn gives: it pushes each, charging a step for it, and the hull
// takes them over.
func (l *termLowerer) sequence(b *exec.Builder, tuple bool, fn string, vals []exec.Expr) exec.Expr {
	base := b.Let(l.local("tm"), exec.C(opTermKids))
	l.pushes(b, fn, vals)
	return l.hull(b, tuple, base)
}

// pushes pushes vals as elements of the list or the tuple that fn makes, charging a step for each.
// Under Steps, a step is charged before them, as the target's evaluation of terms charges one for
// gathering them.
func (l *termLowerer) pushes(b *exec.Builder, fn string, vals []exec.Expr) {
	if l.steps {
		b.Do(OpTermSpend)
	}
	for _, v := range vals {
		l.step(b)
		l.check(b, exec.C("push", exec.C(OpTermCtx), imm(OpTermName, fn), v))
	}
}

// hull returns the tuple value, if tuple, or the list value of the elements pushed since base.
func (l *termLowerer) hull(b *exec.Builder, tuple bool, base exec.Expr) exec.Expr {
	kids := b.Let(l.local("tk"), exec.C(OpTermElemsAbove, base))
	return exec.C(OpTermFromObj, b.Let(l.local("tn"), exec.C("hull", exec.C(OpTermCtx), exec.BoolC(tuple), kids)))
}

// concatParts pushes the elements of the arguments of the fusable concat call t: those of a list call
// as list pushes them, and those of a concat call by its own arguments. The calls themselves are not
// subterms that are evaluated, and charge no step for one.
func (l *termLowerer) concatParts(b *exec.Builder, t *ir.Builtin) {
	for _, a := range t.Args {
		call := a.(*ir.Builtin)
		switch call.Func {
		case "list":
			l.pushes(b, "list", l.values(b, call.Args))
		case "concat":
			l.concatParts(b, call)
		default:
			panic("lower: concat part " + call.Func)
		}
	}
}

// fusable reports whether every argument of the concat call t is a call of list or a fusable concat,
// whose elements concatParts pushes as they are made. compile.Fusable, which the term printer follows,
// also fuses a call of map with a function; TermSupported rejects terms with functions, so no such
// call reaches the lowering, and fusable admits only the parts that concatParts lowers.
func fusable(t *ir.Builtin) bool {
	for _, a := range t.Args {
		call, ok := a.(*ir.Builtin)
		switch {
		case ok && call.Func == "list":
		case ok && call.Func == "concat" && fusable(call):
		default:
			return false
		}
	}
	return true
}

// inDeclaredOrder reports whether the struct literal t gives its fields in the order its type declares
// them.
func inDeclaredOrder(t *ir.New) bool {
	next := 0
	for _, f := range t.Fields {
		for next < len(t.Declared) && t.Declared[next] != f.Name {
			next++
		}
		if next == len(t.Declared) {
			return false
		}
		next++
	}
	return true
}

// structLiteral lowers the struct literal t. The field values are evaluated in the order written. The
// struct is then one call of the constructor of its type (StructCall), or made of its fields in that
// order: a step charged for each, the field set and its value covered by the range of the struct.
func (l *termLowerer) structLiteral(b *exec.Builder, t *ir.New) exec.Expr {
	// A value is let before the next one is evaluated when the constructor call takes the values in
	// another order than written: the call evaluates its arguments in the order declared. Otherwise the
	// values are used as they are; what a field leaves for its use to evaluate (a read, a constant)
	// does not depend on the fields after it.
	let := l.structCall && !inDeclaredOrder(t)
	vals := make([]exec.Expr, len(t.Fields))
	for i, f := range t.Fields {
		vals[i] = l.value(b, f.Value)
		if let {
			vals[i] = b.Let(l.local("tf"), vals[i])
		}
	}
	if l.structCall {
		name := OpTermNew + strconv.Itoa(len(t.Declared))
		if l.m.Op(name) == nil {
			params := make([]exec.Param, len(t.Declared))
			for i := range params {
				params[i] = exec.P("f"+strconv.Itoa(i), exec.THandle(ops.Datum))
			}
			l.m.AddOp(&exec.Op{Name: name, Component: Component, Transitional: true, Params: params,
				Results: []exec.Type{exec.THandle(ops.Datum)}, Reads: []string{Component}, Writes: []string{Component},
				Imm: "the struct literal (TermNew)", Contract: "Returns the struct that the literal in Imm makes of the field values, given in the order its type declares them; nil for a field the literal does not give."})
		}
		args := make([]exec.Expr, len(t.Declared))
		for i, d := range t.Declared {
			args[i] = imm(OpTermConst, TermConst{nil})
			for j, f := range t.Fields {
				if f.Name == d {
					args[i] = vals[j]
				}
			}
		}
		return b.Let(l.local("tv"), &exec.Call{Callee: name, Args: args, Imm: TermNew{t}})
	}
	n := b.Let(l.local("tn"), &exec.Call{Callee: OpTermStruct, Imm: TermNew{t}})
	first := l.local("tfirst")
	b.Let(first, exec.BoolC(true))
	for i, f := range t.Fields {
		l.step(b)
		b.Append(&exec.Do{Call: &exec.Call{Callee: OpTermSetField, Args: []exec.Expr{n, vals[i]}, Imm: f.Name}})
		b.Set(first, exec.C("cover", n, vals[i], exec.L(first)))
	}
	b.Do("made", exec.C(OpTermCtx), n, exec.L(first))
	return exec.C(OpTermFromObj, n)
}

// logic lowers l && r and l || r: logic checks each operand that is evaluated, and the right one is
// evaluated only if the left one does not decide the result.
func (l *termLowerer) logic(b *exec.Builder, t *ir.Binary) exec.Expr {
	op := code(ops.LogicalOps, t.Op)
	left := l.fallible(b, "tb", exec.C("logic", op, l.value(b, t.L)))
	and := t.Op == "&&"
	r := b.Let(l.local("tv"), exec.C("val.fromBool", exec.BoolC(!and)))
	cond := left
	if !and {
		cond = exec.Not(left)
	}
	b.If(cond, func(b *exec.Builder) {
		right := l.fallible(b, "tb", exec.C("logic", op, l.value(b, t.R)))
		b.Set(r.(*exec.Local).Name, exec.C("val.fromBool", right))
	})
	return r
}

// textArg returns the argument of t if t is a call text(x).
func textArg(t ir.Term) (ir.Term, bool) {
	if c, ok := t.(*ir.Builtin); ok && c.Func == "text" && len(c.Args) == 1 {
		return c.Args[0], true
	}
	return nil, false
}

// memberKind returns the kind of the member read name (ops.MemberKinds) and the name of the field or
// capture that it reads.
func memberKind(name string) (int, string) {
	prefix := -1
	for code, k := range ops.MemberKinds {
		switch {
		case code == ops.MemberCapture:
			prefix = code
		case k.Spelling != "" && name == k.Spelling:
			return code, name
		}
	}
	if prefix >= 0 {
		if field, ok := strings.CutPrefix(name, ops.MemberKinds[prefix].Spelling); ok {
			return prefix, field
		}
	}
	return ops.MemberField, name
}
