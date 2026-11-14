package lower

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/core"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Terms of the action language lowered to Exec IR.
//
// A term (an action, a predicate or the value of a binding) becomes a function of Exec IR that calls
// the compositions of the operators of terms (binary, unary, logic, member, length, text, convert,
// conforms) of the parser VM, in the order the term evaluates its operands. Each call returns its value
// and a failure; the failure is checked right after the call. The function returns the term's value and
// a failure, as a composition does: a failure ends the evaluation and the function returns it, for its
// caller to handle (a predicate fails, an action ends the parse). The logical operators && and || are
// branches: the right operand is evaluated only if the left one does not decide the result.
//
// The leaves of a term (constants, captures, locals and variables), the context of the evaluation and
// the static operands of the compositions (a member's name, a node type, the checked type of a cast)
// are transitional operations whose immediate operand the target prints.
//
// A built-in function that takes a function (map, filter, foldl, foldr) is a loop over the elements of
// its list argument: each step charges the step of the element (seq.step), binds the parameters of the
// lambda to locals of the loop and evaluates its body in place.

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
	OpTermKeep       = "term.keep"
	OpTermCopyList   = "term.copyList"
	// The operations of the lowering options FieldCompare and TextDispatch.
	OpTermFieldOperand = "term.fieldOperand"
	OpTermOperand      = "term.operand"
	OpTermCompare      = "term.compare"
	OpTermUncontrolled = "term.uncontrolled"
	OpTermTextIn       = "term.textIn"
	// OpTermNew is the prefix of term.newN, the call of the constructor of a struct type with N
	// declared fields (TermOptions.StructCall).
	OpTermNew = "term.new"
)

// opTermKids is the operation of the parser VM that returns the base of the elements of a list or a
// tuple that starts being made: the number of elements gathered so far (act.push).
const opTermKids = "term.kids"

// HandleOperand is the handle type of an operand of term.compare.
const HandleOperand = "Operand"

// TermField is the immediate operand of term.field: the name of a member (Name) and the name of the
// field or capture that it reads (Field), which is Name without the $ of a capture.
type TermField struct{ Name, Field string }

func (f TermField) String() string { return f.Field + " of " + f.Name }

// TermNew is the immediate operand of term.newN: the struct literal.
type TermNew struct{ New *ir.New }

func (n TermNew) String() string { return n.New.Type }

// TermCompositions are the compositions of the parser VM that lowered terms call.
var TermCompositions = []string{"binary", "unary", "logic", "member", "length", "text", "convert", "conforms",
	"push", "pushItems", "hull", "cover", "made", "items", "itemAt", "filterKeep"}

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
	// the term's value to last, and a failure ends the parse by term.raise; otherwise the body returns
	// the value and the failure. A printer that prints the body without the function, in place of an
	// action, declares the local around it.
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
	// FieldCompare makes a comparison (==, !=, <, <=, >, >=) of which an operand reads a field a call
	// of term.compare on operands, reading such a field with term.fieldOperand, so that a string field
	// is compared without making a value of it.
	FieldCompare bool
	// TextDispatch makes a predicate that compares the text of one capture with string literals,
	// text($x) == "a" || text($x) == "b" ..., test the text against the literals at once with
	// term.textIn when the parse runs without limit controls, and evaluate the term as written
	// otherwise. It applies to a term that the function returns (Result is empty).
	TextDispatch bool
	// Proj are the projected captures of the rule of the term, by name, with the capture of an element
	// that each stands for (compile.RuleProjections). A call map($x, (e) => e.$f) of a projected capture
	// x whose elements stand for f takes the elements of x as they are (term.copyList).
	Proj map[string]string
}

// Term lowers the term t. It reports an error if t is not a term that the front end builds: a
// built-in other than those of the language, one with arguments of the wrong number, a lambda that
// is not the function argument of map, filter, foldl or foldr, or a parameter read outside the body
// of its lambda.
func Term(t ir.Term, o TermOptions) (*TermLowered, error) {
	if !wellFormed(t) {
		return nil, fmt.Errorf("lower: a term that the front end does not build: %s", ir.TermString(t))
	}
	m, err := termModule()
	if err != nil {
		return nil, err
	}
	nOps, nFuncs := len(m.Ops), len(m.Funcs)
	datum, failT := exec.THandle(ops.Datum), exec.THandle(ops.Fail)
	var results []exec.Type
	if o.Result == "" {
		results = []exec.Type{datum, failT}
	}
	l := &termLowerer{m: m, steps: o.Steps, structCall: o.StructCall, unbounded: o.Unbounded, proj: o.Proj,
		params: map[*ir.Lambda][]exec.Expr{}, inPlace: o.Result != "", fieldCompare: o.FieldCompare}
	fn := m.AddFunc("term", nil, results, nil, func(b *exec.Builder) {
		var res exec.Expr
		if o.Result != "" {
			res = b.Var(o.Result, datum)
		}
		if x, lits, ok := textMembership(t); ok && o.TextDispatch && res == nil {
			b.If(exec.C(OpTermUncontrolled), func(b *exec.Builder) {
				s := l.fallible(b, "ts", exec.C("text", imm(OpTermRead, TermRead{x})))
				var in exec.Expr = &exec.Call{Callee: OpTermTextIn, Args: []exec.Expr{s}, Imm: lits}
				if len(lits) > 1 {
					// A target may print the binding as one switch; one literal stays an equality.
					in = b.Let(l.local("tin"), in)
				}
				b.Return(exec.C("val.fromBool", in), exec.Null(failT))
			})
		}
		v := l.value(b, t)
		if res != nil {
			b.Set(o.Result, v)
		} else {
			b.Return(v, exec.Null(failT))
		}
	})
	// The module the term extends was checked once when it was built.
	if err := exec.VerifyExtension(m, nOps, nFuncs); err != nil {
		return nil, fmt.Errorf("lower term: %w", err)
	}
	return &TermLowered{Module: m, Func: fn, Result: o.Result}, nil
}

// wellFormed reports whether t is a term that Term lowers: the built-ins of the language with their
// arguments, lambdas only as their function arguments, and parameters only in the bodies of their
// lambdas.
func wellFormed(t ir.Term) bool {
	ok := true
	args := map[*ir.Lambda]bool{} // the lambdas that are the function arguments of built-ins
	ir.WalkTerm(t, func(x ir.Term) {
		switch x := x.(type) {
		case *ir.Const, *ir.SlotRef, *ir.VarRef, *ir.Member, *ir.Binary, *ir.Unary, *ir.Convert, *ir.Cast,
			*ir.Tuple, *ir.New, *ir.Local:
		case *ir.Lambda:
			// WalkTerm visits a built-in before its arguments; a parameter is read only in the body of
			// its lambda.
			ok = ok && args[x]
		case *ir.Builtin:
			switch x.Func {
			case "len", "text":
				ok = ok && len(x.Args) == 1 && plain(x.Args[0])
			case "list", "concat":
				for _, a := range x.Args {
					ok = ok && plain(a)
				}
			case "map", "filter":
				ok = ok && len(x.Args) == 2 && plain(x.Args[0]) && lambda(x.Args[1], 1)
			case "foldl", "foldr":
				ok = ok && len(x.Args) == 3 && plain(x.Args[0]) && plain(x.Args[1]) && lambda(x.Args[2], 2)
			default:
				ok = false
			}
			if n := len(x.Args); ok && n > 0 {
				if f, isLambda := x.Args[n-1].(*ir.Lambda); isLambda {
					args[f] = true
				}
			}
		default:
			ok = false
		}
	})
	return ok && scoped(t, nil)
}

// scoped reports whether every parameter that t reads is read in the body of its lambda; in holds
// the lambdas that enclose t.
func scoped(t ir.Term, in []*ir.Lambda) bool {
	switch x := t.(type) {
	case *ir.Local:
		return x.Kind != ir.LocalParam || slices.Contains(in, x.Lambda)
	case *ir.Lambda:
		in = append(in[:len(in):len(in)], x)
	}
	for _, c := range ir.TermChildren(t) {
		if !scoped(c, in) {
			return false
		}
	}
	return true
}

// plain reports whether t is not a lambda.
func plain(t ir.Term) bool {
	_, ok := t.(*ir.Lambda)
	return !ok
}

// lambda reports whether t is a lambda with n parameters.
func lambda(t ir.Term, n int) bool {
	f, ok := t.(*ir.Lambda)
	return ok && len(f.Params) == n
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
		base := core.Build()
		m := &exec.Module{Name: "term", Features: base.Features, Components: slices.Clip(base.Components),
			Registers: base.Registers, Handles: slices.Clip(base.Handles), Ops: slices.Clip(base.Ops)}
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
		if err := exec.Verify(m); err != nil {
			termBaseErr = fmt.Errorf("lower: the module of terms: %w", err)
			return
		}
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
		Contract: "Returns the value of the capture, the local or the variable in Imm, as the target reads a leaf of an action."})
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
		Contract: "Ends the parse with the failure f of a term evaluated in place of an action (TermOptions.Result), as the failure of an action does."})
	op(&exec.Op{Name: OpTermCastError, Params: []exec.Param{exec.P("v", datum)}, Results: []exec.Type{failT}, Reads: reads, Writes: reads,
		Imm: "the cast (TermCast)", Contract: "Returns the failure of the cast in Imm of the value v."})
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
	op(&exec.Op{Name: OpTermKeep, Params: []exec.Param{exec.P("items", seq), exec.P("i", exec.TInt)}, Reads: reads, Writes: reads, NoReturn: true,
		Contract: "Pushes element i of items, which items returned, as it is (a box stays a box) as the next element of the list that filter makes, charging nothing."})
	op(&exec.Op{Name: OpTermCopyList, Params: []exec.Param{exec.P("items", seq)}, Results: []exec.Type{datum}, Reads: reads, Writes: reads,
		Contract: "Returns a new list value of the elements items, which items returned, as they are, charging for copying them under budget controls."})
	m.Handles = append(m.Handles, &exec.HandleType{Name: HandleOperand, Component: Component,
		Doc: "An operand of a comparison: a value, or a string field read without making a value of it."})
	operand := exec.THandle(HandleOperand)
	op(&exec.Op{Name: OpTermFieldOperand, Params: []exec.Param{exec.P("x", datum)}, Results: []exec.Type{operand, failT}, Reads: reads, Writes: reads,
		Imm: "the name of the member (a string)", Contract: "Reads the field in Imm of x as member does, and returns it as an operand and the failure of the read; a string field stays a string, with no value made of it."})
	op(&exec.Op{Name: OpTermOperand, Params: []exec.Param{exec.P("v", datum)}, Results: []exec.Type{operand},
		Contract: "Returns the value v as an operand."})
	op(&exec.Op{Name: OpTermCompare, Params: []exec.Param{exec.P("l", operand), exec.P("r", operand)}, Results: []exec.Type{datum, failT}, Reads: reads, Writes: reads,
		Imm: "the comparison operator (a string)", Contract: "Applies the comparison in Imm to l and r as binary does, comparing two strings directly, and returns the value and the failure."})
	op(&exec.Op{Name: OpTermUncontrolled, Results: []exec.Type{exec.TBool}, Reads: reads,
		Contract: "Returns whether the parse runs without limit controls."})
	op(&exec.Op{Name: OpTermTextIn, Params: []exec.Param{exec.P("s", str)}, Results: []exec.Type{exec.TBool},
		Imm: "the literals (a []string of distinct strings)", Contract: "Returns whether s is one of the literals in Imm, charging nothing."})
}

type termLowerer struct {
	m          *exec.Module
	n          int
	steps      bool
	structCall bool
	unbounded  bool
	proj       map[string]string
	// params holds the values of the parameters of each lambda whose body is being lowered.
	params map[*ir.Lambda][]exec.Expr
	// inPlace is set when the term is evaluated in place of an action (TermOptions.Result).
	inPlace      bool
	fieldCompare bool
}

// raise ends the evaluation of the term with the failure f: the function returns it, or, in place of
// an action, term.raise ends the parse.
func (l *termLowerer) raise(b *exec.Builder, f exec.Expr) {
	if l.inPlace {
		b.Do(OpTermRaise, f)
		return
	}
	b.Return(exec.Null(exec.THandle(ops.Datum)), f)
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
// raises the failure (raise), and returns the value.
func (l *termLowerer) fallible(b *exec.Builder, prefix string, c *exec.Call) exec.Expr {
	n := l.local("")
	r := b.LetN([]string{prefix + n, "tf" + n}, c)
	b.If(exec.Bin("!=", r[1], exec.Null(exec.THandle(ops.Fail))), func(b *exec.Builder) { l.raise(b, r[1]) })
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
	case *ir.Local:
		if t.Kind == ir.LocalParam {
			// A parameter may hold a box, which stands for its value.
			return exec.C("val.unbox", l.params[t.Lambda][slices.Index(t.Lambda.Params, t.Name)])
		}
		return imm(OpTermRead, TermRead{t})
	case *ir.SlotRef:
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
		if l.fieldCompare && comparison(t.Op) && (fieldRead(t.L) || fieldRead(t.R)) {
			x := l.operand(b, t.L)
			y := l.operand(b, t.R)
			return l.fallible(b, "tv", &exec.Call{Callee: OpTermCompare, Args: []exec.Expr{x, y}, Imm: t.Op})
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
		case "map":
			if x, ok := l.projected(t); ok {
				items := l.fallible(b, "ti", exec.C("items", imm(OpTermName, "map"), l.value(b, x)))
				return b.Let(l.local("tv"), exec.C(OpTermCopyList, items))
			}
			list := l.value(b, t.Args[0])
			base := b.Let(l.local("tm"), exec.C(opTermKids))
			l.mapPush(b, list, t.Args[1].(*ir.Lambda))
			return l.hull(b, false, base)
		case "filter":
			list := l.value(b, t.Args[0])
			base := b.Let(l.local("tm"), exec.C(opTermKids))
			f := t.Args[1].(*ir.Lambda)
			items := l.fallible(b, "ti", exec.C("items", imm(OpTermName, "filter"), list))
			l.loop(b, items, func(b *exec.Builder, i exec.Expr) {
				v := l.call(b, f, exec.C("itemAt", items, i, exec.BoolC(false)))
				keep := l.fallible(b, "tb", exec.C("filterKeep", v))
				b.If(keep, func(b *exec.Builder) { b.Do(OpTermKeep, items, i) })
			})
			return l.hull(b, false, base)
		case "foldl", "foldr":
			init := l.value(b, t.Args[0])
			list := l.value(b, t.Args[1])
			f := t.Args[2].(*ir.Lambda)
			items := l.fallible(b, "ti", exec.C("items", imm(OpTermName, t.Func), list))
			acc := l.local("ta")
			b.Let(acc, init)
			l.loop(b, items, func(b *exec.Builder, i exec.Expr) {
				b.Set(acc, l.call(b, f, exec.L(acc), exec.C("itemAt", items, i, exec.BoolC(t.Func == "foldr"))))
			})
			return exec.L(acc)
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
		if t.Check == nil {
			// Every value fits the type.
			return l.value(b, t.X)
		}
		x := b.Let(l.local("tc"), l.value(b, t.X))
		b.If(exec.Not(exec.C("conforms", imm(OpTermCastType, TermCast{t}), x)), func(b *exec.Builder) {
			l.raise(b, &exec.Call{Callee: OpTermCastError, Args: []exec.Expr{x}, Imm: TermCast{t}})
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
	b.If(exec.Bin("!=", f, exec.Null(exec.THandle(ops.Fail))), func(b *exec.Builder) { l.raise(b, f) })
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
// as list pushes them, those of a map call as map pushes them, and those of a concat call by its own
// arguments. The calls themselves are not subterms that are evaluated, and charge no step for one.
func (l *termLowerer) concatParts(b *exec.Builder, t *ir.Builtin) {
	for _, a := range t.Args {
		call := a.(*ir.Builtin)
		switch call.Func {
		case "list":
			l.pushes(b, "list", l.values(b, call.Args))
		case "map":
			if x, ok := l.projected(call); ok {
				// The elements of the projected capture are pushed as concat pushes those of an argument.
				v := l.value(b, x)
				if l.steps {
					b.Do(OpTermSpend)
				}
				l.check(b, exec.C("pushItems", exec.C(OpTermCtx), imm(OpTermName, "concat"), v))
				break
			}
			l.mapPush(b, l.value(b, call.Args[0]), call.Args[1].(*ir.Lambda))
		case "concat":
			l.concatParts(b, call)
		default:
			panic("lower: concat part " + call.Func)
		}
	}
}

// fusable reports whether every argument of the concat call t is a call of list or map or a fusable
// concat, whose elements concatParts pushes as they are made, as compile.Fusable decides.
func fusable(t *ir.Builtin) bool {
	for _, a := range t.Args {
		call, ok := a.(*ir.Builtin)
		switch {
		case ok && call.Func == "list", ok && call.Func == "map" && len(call.Args) == 2:
		case ok && call.Func == "concat" && fusable(call):
		default:
			return false
		}
	}
	return true
}

// mapPush pushes the values that the lambda f returns for the elements of list as the elements of the
// list that map makes. Under Steps, a step is charged before the elements are read, as the target's
// map charges one.
func (l *termLowerer) mapPush(b *exec.Builder, list exec.Expr, f *ir.Lambda) {
	if l.steps {
		b.Do(OpTermSpend)
	}
	items := l.fallible(b, "ti", exec.C("items", imm(OpTermName, "map"), list))
	l.loop(b, items, func(b *exec.Builder, i exec.Expr) {
		v := l.call(b, f, exec.C("itemAt", items, i, exec.BoolC(false)))
		l.check(b, exec.C("push", exec.C(OpTermCtx), imm(OpTermName, "map"), v))
	})
}

// loop appends a loop over the elements items, which items returned, whose body charges the step of
// the element and then runs body with the index of the element.
func (l *termLowerer) loop(b *exec.Builder, items exec.Expr, body func(b *exec.Builder, i exec.Expr)) {
	i := l.local("tj")
	b.Let(i, exec.Int(0))
	b.For(l.local("each"), nil, exec.Lt(exec.L(i), exec.C("seq.len", items)), exec.Assign(i, exec.Add(exec.L(i), exec.Int(1))), func(b *exec.Builder) {
		l.step(b)
		body(b, exec.L(i))
	})
}

// call evaluates the body of the lambda f with its parameters bound to args, each let to a local, and
// returns its value.
func (l *termLowerer) call(b *exec.Builder, f *ir.Lambda, args ...exec.Expr) exec.Expr {
	vals := make([]exec.Expr, len(args))
	for i, a := range args {
		vals[i] = b.Let(l.local("tp"), a)
	}
	l.params[f] = vals
	v := b.Let(l.local("tv"), l.value(b, f.Body))
	delete(l.params, f)
	return v
}

// projected reports whether the map call t is map($x, (e) => e.$f) of a projected capture x (Proj)
// whose elements stand for the capture f, and returns the read of x.
func (l *termLowerer) projected(t *ir.Builtin) (*ir.SlotRef, bool) {
	if l.proj == nil || len(t.Args) != 2 {
		return nil, false
	}
	x, ok := t.Args[0].(*ir.SlotRef)
	if !ok || l.proj[x.Name] == "" {
		return nil, false
	}
	f, ok := t.Args[1].(*ir.Lambda)
	if !ok || len(f.Params) != 1 {
		return nil, false
	}
	m, ok := f.Body.(*ir.Member)
	if !ok {
		return nil, false
	}
	e, ok := m.X.(*ir.Local)
	if !ok || e.Kind != ir.LocalParam || e.Lambda != f {
		return nil, false
	}
	return x, m.Field == l.proj[x.Name]
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
	// another order than written: the call evaluates its arguments in the order declared. Without the
	// constructor, each value is used twice (set and covered), so a value that is not a local or a
	// constant is let once: a read is then made, and charged, once, as the term reads it. Otherwise
	// the values are used as they are; what a field leaves for its use to evaluate (a read, a
	// constant) does not depend on the fields after it.
	let := l.structCall && !inDeclaredOrder(t)
	vals := make([]exec.Expr, len(t.Fields))
	for i, f := range t.Fields {
		vals[i] = l.value(b, f.Value)
		if let || !l.structCall && !settled(vals[i]) {
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

// settled reports whether the expression e is a local or a constant, which can be used more than
// once without being evaluated again.
func settled(e exec.Expr) bool {
	switch e := e.(type) {
	case *exec.Local, *exec.Const:
		return true
	case *exec.Call:
		return e.Callee == OpTermConst
	}
	return false
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

// comparison reports whether op is a comparison operator.
func comparison(op string) bool {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	}
	return false
}

// fieldRead reports whether t reads a field of a value, x.F, not a capture.
func fieldRead(t ir.Term) bool {
	m, ok := t.(*ir.Member)
	return ok && (m.Name == "" || m.Name[0] != '$')
}

// operand lowers t, an operand of a comparison (FieldCompare): a field read is read by
// term.fieldOperand, any other term is a value.
func (l *termLowerer) operand(b *exec.Builder, t ir.Term) exec.Expr {
	if !fieldRead(t) {
		return exec.C(OpTermOperand, l.value(b, t))
	}
	if l.steps {
		b.Do(OpTermSpend)
	}
	m := t.(*ir.Member)
	x := l.value(b, m.X)
	return l.fallible(b, "to", &exec.Call{Callee: OpTermFieldOperand, Args: []exec.Expr{x}, Imm: m.Name})
}

// textMembership reports whether t compares the text of one capture with string literals,
// text($x) == "a" || ..., with the capture and the distinct literals in the order written.
func textMembership(t ir.Term) (ir.Term, []string, bool) {
	var x ir.Term
	var name string
	var lits []string
	var walk func(ir.Term) bool
	walk = func(t ir.Term) bool {
		b, ok := t.(*ir.Binary)
		if ok && b.Op == "||" {
			return walk(b.L) && walk(b.R)
		}
		if !ok || b.Op != "==" {
			return false
		}
		arg, ok := textArg(b.L)
		lit, isConst := b.R.(*ir.Const)
		if !ok || !isConst {
			arg, ok = textArg(b.R)
			lit, isConst = b.L.(*ir.Const)
		}
		if !ok || !isConst {
			return false
		}
		s, isStr := lit.Value.(string)
		var n string
		switch c := arg.(type) {
		case *ir.SlotRef:
			n = c.Name
		case *ir.Local:
			if c.Kind == ir.LocalParam {
				return false
			}
			n = c.Name
		default:
			return false
		}
		if !isStr || (name != "" && n != name) {
			return false
		}
		x, name = arg, n
		if !slices.Contains(lits, s) {
			lits = append(lits, s)
		}
		return true
	}
	if !walk(t) {
		return nil, nil, false
	}
	return x, lits, true
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
