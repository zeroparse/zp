package complexity

import (
	"maps"
	"slices"
	"unicode/utf8"
	"zeroparse.dev/zp/internal/check"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/grammaranalysis"

	"zeroparse.dev/zp/grammar"
)

// maxFiniteValues caps the number of values that finiteVars computes for one variable. The
// classifier evaluates every binding of the variable at each value, so the cap bounds the work of
// the classification, not the cost of a parse; a variable with more values has no finite values.
const maxFiniteValues = 1 << 16

// maxCombinations caps the product of the numbers of values of the variables with finite values,
// which the class Linear reports as its bound on K (see Complexity.Combinations). The cap keeps
// the product exact in an int; a bound past it would promise nothing that a parse can use.
const maxCombinations = 1 << 32

// maxLenOperand caps the length bound of a capture whose length a binding of an int variable reads,
// and maxOperandCombinations the combinations of such lengths that one binding reads.
const (
	maxLenOperand          = 256
	maxOperandCombinations = 4096
)

// finiteDomains is what finiteVars finds: the variables with finite values, the product of the
// sizes of their value sets, and the variables whose value sets are finite but that the cap on the
// product leaves out.
type finiteDomains struct {
	finite  map[string]bool
	product int
	capped  map[string]int
}

// finiteVars returns the variables whose values in every parse lie in a set bounded by a constant
// of the grammar: every bool variable (see finiteBool), every int variable whose initial value and
// bindings reach a finite set (see finiteInt), and every variable whose initial value and bindings
// are all constants (see constantValues), each with at most maxFiniteValues values. The value
// sets are taken from the smallest up while the product of their sizes stays at most
// maxCombinations; a variable whose set would take it past the cap has no finite values. No value
// set depends on another variable, so a variable without finite values leaves the others as they
// are.
func (k *classifier) finiteVars() finiteDomains {
	byVar := map[string][]*binding{}
	for _, b := range k.binds {
		byVar[b.b.Name] = append(byVar[b.b.Name], b)
	}
	type sized struct {
		name string
		n    int
	}
	var sets []sized
	for _, v := range k.g.Vars() {
		n, ok := 0, false
		switch k.varType[v.Name] {
		case "bool":
			n, ok = finiteBool(v, byVar[v.Name]), true
		case "int":
			n, ok = k.finiteInt(v, byVar[v.Name])
		}
		if c, isConst := constantValues(v, byVar[v.Name]); isConst && (!ok || c < n) {
			n, ok = c, true
		}
		if ok && n <= maxFiniteValues {
			sets = append(sets, sized{v.Name, n})
		}
	}
	slices.SortStableFunc(sets, func(a, b sized) int { return a.n - b.n })
	d := finiteDomains{finite: map[string]bool{}, product: 1, capped: map[string]int{}}
	for _, s := range sets {
		if d.product*s.n > maxCombinations {
			d.capped[s.name] = s.n
			continue
		}
		d.product *= s.n
		d.finite[s.name] = true
	}
	return d
}

// constantValues returns a bound on the number of values of the variable v when its initial value
// and every binding of it are constants, and whether they are: each constant term gives one value,
// or none when its evaluation is an error, so the distinct terms bound the values.
func constantValues(v *grammar.VarDef, binds []*binding) (int, bool) {
	terms := map[string]bool{}
	if v.Value == nil {
		terms[""] = true
	} else {
		terms[grammar.FormatTerm(check.UngroupTerm(v.Value))] = true
	}
	for _, b := range binds {
		if !constant(b.b.Value) {
			return 0, false
		}
		terms[grammar.FormatTerm(check.UngroupTerm(b.b.Value))] = true
	}
	return len(terms), true
}

// finiteBool returns the number of values of the bool variable v: one when no binding of v can
// bind a value other than its initial value, and two otherwise. A binding counts by its value
// when that is a constant, binds nothing when it fails to evaluate, and counts as both values
// otherwise.
func finiteBool(v *grammar.VarDef, binds []*binding) int {
	set := map[bool]bool{}
	init := any(false)
	if v.Value != nil {
		var res evalResult
		if init, res = eval(v.Value, domainEnv{}); res != evalOK {
			return 2
		}
	}
	x, isBool := init.(bool)
	if !isBool {
		return 2
	}
	set[x] = true
	for _, b := range binds {
		r, res := eval(b.b.Value, domainEnv{})
		y, isBool := r.(bool)
		switch {
		case res == evalFailed:
		case res != evalOK || !isBool:
			return 2
		default:
			set[y] = true
		}
	}
	return len(set)
}

// valueSet is a set of values of an int variable.
type valueSet map[int]bool

// lenOperand is an operand len($c) of a binding, with the bound on the length of the value of c.
type lenOperand struct {
	call *grammar.Call
	max  int
}

// intDomain computes the values of one int variable.
type intDomain struct {
	k      *classifier
	v      string
	set    valueSet // the values found so far
	base   valueSet // the values of set that the current pass starts from
	over   bool     // the values exceed maxFiniteValues, or one is not known
	lens   map[*binding][]lenOperand
	opaque map[*binding]bool // bindings that read what eval does not know (see lenOperands)
	guards map[*binding]guard
	binds  map[*grammar.Bind]*binding
}

// finiteInt reports whether the values of the int variable v form a set of at most
// maxFiniteValues values: the least set S that holds the initial value and, for each binding of
// v, the values that the binding leaves bound from the values that can reach it (see outputs).
// Those are the values of S after the earlier elements of the sequences around the binding (see
// inputs). A binding may read v, literals, and len($c) of a capture c whose value has a length
// bounded by a constant (see valueLen), which it reads at every length up to the bound. A binding
// that fails to evaluate, as by a division by zero, binds nothing. A binding that reads anything
// else binds the values that its guard bounds (see guardOf); one whose values have no such bound
// leaves the set unbounded.
func (k *classifier) finiteInt(v *grammar.VarDef, binds []*binding) (int, bool) {
	init := 0
	if v.Value != nil {
		r, res := eval(v.Value, domainEnv{v: v.Name})
		x, isInt := r.(int)
		if res != evalOK || !isInt {
			return 0, false
		}
		init = x
	}
	d := &intDomain{k: k, v: v.Name, set: valueSet{init: true}, lens: map[*binding][]lenOperand{},
		opaque: map[*binding]bool{}, guards: map[*binding]guard{}, binds: map[*grammar.Bind]*binding{}}
	for _, b := range binds {
		ops, ok := k.lenOperands(b)
		d.lens[b], d.opaque[b] = ops, !ok
		d.guards[b] = guardOf(b)
		d.binds[b.b] = b
	}
	// Each pass evaluates the bindings only from the values that the previous pass added: what
	// reaches a binding from a set is the union of what reaches it from each value of the set, so
	// the earlier values give nothing new.
	for d.base = d.set; len(d.base) > 0; {
		added := valueSet{}
		for _, b := range binds {
			for y := range d.outputs(b, d.inputs(b)) {
				if !d.set[y] && !added[y] {
					if len(d.set)+len(added) == maxFiniteValues {
						return 0, false
					}
					added[y] = true
				}
			}
			if d.over {
				return 0, false
			}
		}
		maps.Copy(d.set, added)
		d.base = added
	}
	return len(d.set), true
}

// outputs returns the values that the binding b leaves bound when its variable holds a value of
// xs before it: the values that it binds from them and that pass its guard. The values of a
// binding that eval cannot evaluate are those in the range of its value (see termRange) within
// the bounds of the guard; past maxFiniteValues of them, or without a bound, the set is
// unbounded.
func (d *intDomain) outputs(b *binding, xs valueSet) valueSet {
	out := valueSet{}
	if len(xs) == 0 {
		return out
	}
	g := d.guards[b]
	if !d.opaque[b] {
		known := true
		for x := range xs {
			ys, ok := d.apply(b, x)
			if !ok {
				known = false
				break
			}
			for _, y := range ys {
				if g.passes(d.v, y) {
					out[y] = true
				}
			}
		}
		if known {
			return out
		}
		clear(out)
	}
	r := termRange(b.b.Value, d.v, xs).meet(g.bounds)
	if r.size() > maxFiniteValues {
		d.over = true
		return nil
	}
	for y := r.lo; y <= r.hi; y++ {
		if g.passes(d.v, y) {
			out[y] = true
		}
	}
	return out
}

// apply returns the values that the binding b binds when its variable holds x, at every
// combination of the lengths that it reads, and whether eval knows them all.
func (d *intDomain) apply(b *binding, x int) ([]int, bool) {
	ops := d.lens[b]
	lens := make([]int, len(ops))
	var out []int
	for {
		env := domainEnv{v: d.v, x: x, lens: map[*grammar.Call]int{}}
		for i, op := range ops {
			env.lens[op.call] = lens[i]
		}
		r, res := eval(b.b.Value, env)
		y, isInt := r.(int)
		switch {
		case res == evalFailed:
		case res != evalOK || !isInt:
			return nil, false
		default:
			out = append(out, y)
		}
		i := 0
		for i < len(ops) && lens[i] == ops[i].max {
			lens[i] = 0
			i++
		}
		if i == len(ops) {
			return out, true
		}
		lens[i]++
	}
}

// lenOperands returns the operands len($c) of the binding b, and whether b reads only its
// variable, literals and such operands.
func (k *classifier) lenOperands(b *binding) ([]lenOperand, bool) {
	var ops []lenOperand
	combos := 1
	ok := readsOnly(b.b.Value, b.b.Name, func(c *grammar.Call) bool {
		r, isRef := check.UngroupTerm(c.Args[0]).(*grammar.CaptureRef)
		if !isRef || r.Param {
			return false
		}
		exprs := k.captures[b.rule][r.Name]
		if len(exprs) == 0 {
			return false
		}
		m := 0
		for _, e := range exprs {
			n, ok := k.valueLen(e, map[string]bool{})
			if !ok {
				return false
			}
			m = max(m, n)
		}
		if m > maxLenOperand || combos*(m+1) > maxOperandCombinations {
			return false
		}
		combos *= m + 1
		ops = append(ops, lenOperand{call: c, max: m})
		return true
	})
	return ops, ok
}

// inputs returns the values that can reach the binding b: the values of the set, after the
// earlier elements of the sequences around b from the outermost one that b is in through groups,
// options, choices, captures, discards and atomic expressions only, and after the earlier
// statements of the block that holds b.
func (d *intDomain) inputs(b *binding) valueSet {
	x := d.base
	frames := b.c.seqs[b.c.strictFrom:]
	for _, f := range frames {
		for _, it := range f.seq.Items[:f.index] {
			x = d.after(it, x)
		}
	}
	if len(frames) == 0 {
		return x
	}
	f := frames[len(frames)-1]
	stmts, found := blockBefore(f.seq.Items[f.index], b.b)
	if !found {
		// b lies deeper in the terms of a predicate than a statement of its block.
		return d.base
	}
	for _, s := range stmts {
		x = d.afterTerm(s, x)
	}
	return x
}

// blockBefore returns the statements before the binding b in the block of the predicate in e that
// holds it as a statement or as its term, and whether e has such a predicate.
func blockBefore(e grammar.Expr, b *grammar.Bind) ([]grammar.Term, bool) {
	var out []grammar.Term
	found := false
	grammaranalysis.WalkExpr(e, func(x grammar.Expr) {
		p, ok := x.(*grammar.Predicate)
		if !ok || found {
			return
		}
		if p.Term == grammar.Term(b) {
			found = true
			return
		}
		if blk, ok := p.Term.(*grammar.Block); ok {
			if i := slices.Index(blk.Statements, grammar.Term(b)); i >= 0 {
				out, found = blk.Statements[:i], true
			}
		}
	})
	return out, found
}

// after returns the values that the variable can hold after e matches when it holds a value of x
// before. A binding applies, a test filters, a choice or an option joins its paths, @ attributes
// change nothing, and a call or a terminal keeps the values, since bindings in a called rule are
// not visible to its caller. Any other expression that binds the variable can leave it with any
// value of the set.
func (d *intDomain) after(e grammar.Expr, x valueSet) valueSet {
	switch e := e.(type) {
	case *grammar.Predicate:
		return d.afterTerm(e.Term, x)
	case *grammar.Group:
		return d.after(e.Expr, x)
	case *grammar.Capture:
		return d.after(e.Expr, x)
	case *grammar.Discard:
		return d.after(e.Expr, x)
	case *grammar.Atomic:
		return d.after(e.Expr, x)
	case *grammar.Attributed:
		if labelsOnly(e) {
			return d.after(e.Expr, x)
		}
	case *grammar.Seq:
		for _, it := range e.Items {
			x = d.after(it, x)
		}
		return x
	case *grammar.Choice:
		out := valueSet{}
		for _, alt := range e.Alts {
			maps.Copy(out, d.after(alt, x))
		}
		return out
	case *grammar.Optional:
		out := maps.Clone(x)
		maps.Copy(out, d.after(e.Expr, x))
		return out
	case *grammar.Not:
		// A negative lookahead undoes its bindings.
		return x
	}
	if d.bindsVar(e) {
		return d.base
	}
	return x
}

// bindsVar reports whether e binds the variable in its own body, outside the rules that it calls.
func (d *intDomain) bindsVar(e grammar.Expr) bool {
	found := false
	grammaranalysis.WalkExpr(e, func(x grammar.Expr) {
		if p, ok := x.(*grammar.Predicate); ok {
			grammaranalysis.WalkTerm(p.Term, func(t grammar.Term) {
				if bd, ok := t.(*grammar.Bind); ok && bd.Name == d.v {
					found = true
				}
			})
		}
	})
	return found
}

// afterTerm returns the values that the variable can hold after the predicate term t succeeds.
func (d *intDomain) afterTerm(t grammar.Term, x valueSet) valueSet {
	switch t := t.(type) {
	case *grammar.Block:
		for _, s := range t.Statements {
			x = d.afterTerm(s, x)
		}
		return x
	case *grammar.Bind:
		if t.Name != d.v {
			return x
		}
		b := d.binds[t]
		if b == nil {
			return d.base
		}
		return d.outputs(b, x)
	}
	if !readsOnly(t, d.v, nil) {
		return x
	}
	out := valueSet{}
	for v := range x {
		r, res := eval(t, domainEnv{v: d.v, x: v})
		if b, isBool := r.(bool); res == evalUnknown || res == evalOK && (!isBool || b) {
			out[v] = true
		}
	}
	return out
}

// valueLen returns a bound on what len gives for the value of the captured expression e, and
// whether it finds one: the length of the matched text in any position unit for a string node,
// the number of elements for a tuple or list node or a list value, and 0 for nil.
func (k *classifier) valueLen(e grammar.Expr, seen map[string]bool) (int, bool) {
	switch e := e.(type) {
	case *grammar.Literal, *grammar.CharClass, *grammar.Any:
		return textLen(e, k, seen)
	case *grammar.Group:
		return k.valueLen(e.Expr, seen)
	case *grammar.Capture:
		return k.valueLen(e.Expr, seen)
	case *grammar.Optional:
		return k.valueLen(e.Expr, seen)
	case *grammar.And:
		return k.valueLen(e.Expr, seen)
	case *grammar.Atomic:
		return textLen(e.Expr, k, seen)
	case *grammar.Repeat:
		if e.Max >= 0 {
			return e.Max, true
		}
	case *grammar.Choice:
		m := 0
		for _, alt := range e.Alts {
			n, ok := k.valueLen(alt, seen)
			if !ok {
				return 0, false
			}
			m = max(m, n)
		}
		return m, true
	case *grammar.Seq:
		// A tuple with at most one element for each item, where a plain group of a sequence
		// and a call of a splice rule splice their elements into the tuple, or the value of a
		// single item.
		m, ok := k.splicedLen(e, map[string]bool{})
		if !ok {
			return 0, false
		}
		for _, it := range e.Items {
			n, ok := k.valueLen(it, seen)
			if !ok {
				return 0, false
			}
			m = max(m, n)
		}
		return m, true
	case *grammar.Discard, *grammar.Predicate, *grammar.Not:
		return 0, true
	case *grammar.Ref:
		// A rule on a recursion can have a bounded value, as one whose action reads only a
		// bounded capture; a value that reaches its own rule again has no bound.
		r := k.byName[e.Name]
		if r == nil || seen[e.Name] {
			return 0, false
		}
		seen[e.Name] = true
		defer delete(seen, e.Name)
		if r.Action != nil {
			return k.termLen(r.Action, e.Name, seen)
		}
		if isOperators(r.Expr) {
			return 0, false
		}
		return k.valueLen(r.Expr, seen)
	}
	return 0, false
}

// splicedLen returns the number of items of the tuple of the sequence e: a plain group of a
// sequence adds its own items, a call of a splice rule whose body is a sequence adds the items of
// that body, and every other item, such as a capture, an option, a choice, a repetition or any
// other call, adds one. It reports false if the items have no bound: a splice rule that calls
// itself among its own items.
func (k *classifier) splicedLen(e *grammar.Seq, seen map[string]bool) (int, bool) {
	n := 0
	for _, it := range e.Items {
		m, ok := k.itemLen(it, seen)
		if !ok {
			return 0, false
		}
		n += m
	}
	return n, true
}

// itemLen returns the number of tuple items that the sequence item it adds, as splicedLen counts.
func (k *classifier) itemLen(it grammar.Expr, seen map[string]bool) (int, bool) {
	for {
		switch x := check.UngroupExpr(it).(type) {
		case *grammar.Seq:
			return k.splicedLen(x, seen)
		case *grammar.Attributed:
			// An attribute other than recovery only labels its expression (isSeqOf).
			if check.Recovers(x) {
				return 1, true
			}
			it = x.Expr
		case *grammar.Ref:
			if !check.IsSeqOf(k.spliceBody, x) {
				return 1, true
			}
			if seen[x.Name] {
				return 0, false
			}
			body, _ := k.spliceBody(x.Name)
			seen[x.Name] = true
			n, ok := k.itemLen(body, seen)
			delete(seen, x.Name)
			return n, ok
		default:
			return 1, true
		}
	}
}

// spliceBody returns the body of the rule name if it is a splice rule (Program.SpliceBody), as the
// grammar writes it.
func (k *classifier) spliceBody(name string) (grammar.Expr, bool) {
	if rd := k.byName[name]; rd != nil && compile.SpliceRule(rd) {
		return rd.Expr, true
	}
	return nil, false
}

// termLen bounds what len gives for the value of the action term t of rule.
func (k *classifier) termLen(t grammar.Term, rule string, seen map[string]bool) (int, bool) {
	switch t := t.(type) {
	case *grammar.TermGroup:
		return k.termLen(t.Term, rule, seen)
	case *grammar.StringLit:
		return len(t.Value), true
	case *grammar.NilLit:
		return 0, true
	case *grammar.CaptureRef:
		exprs := k.captures[rule][t.Name]
		if t.Param || len(exprs) == 0 {
			return 0, false
		}
		m := 0
		for _, e := range exprs {
			n, ok := k.valueLen(e, seen)
			if !ok {
				return 0, false
			}
			m = max(m, n)
		}
		return m, true
	case *grammar.Call:
		if t.Func == "list" {
			return len(t.Args), true
		}
	}
	return 0, false
}

// textLen bounds the length of the text that e matches, in any position unit: no unit counts more
// positions for a character than the bytes of its UTF-8 encoding.
func textLen(e grammar.Expr, k *classifier, seen map[string]bool) (int, bool) {
	switch e := e.(type) {
	case *grammar.Literal:
		if e.IgnoreCase {
			return utf8.UTFMax * utf8.RuneCountInString(e.Value), true
		}
		return len(e.Value), true
	case *grammar.CharClass, *grammar.Any:
		return utf8.UTFMax, true
	case *grammar.Predicate, *grammar.And, *grammar.Not, *grammar.Cut, *grammar.BeginInput, *grammar.EndInput,
		*grammar.BeginLine, *grammar.EndLine:
		return 0, true
	case *grammar.Group:
		return textLen(e.Expr, k, seen)
	case *grammar.Capture:
		return textLen(e.Expr, k, seen)
	case *grammar.Discard:
		return textLen(e.Expr, k, seen)
	case *grammar.Atomic:
		return textLen(e.Expr, k, seen)
	case *grammar.Optional:
		return textLen(e.Expr, k, seen)
	case *grammar.Repeat:
		if e.Max >= 0 {
			n, ok := textLen(e.Expr, k, seen)
			return e.Max * n, ok
		}
	case *grammar.Seq:
		sum := 0
		for _, it := range e.Items {
			n, ok := textLen(it, k, seen)
			if !ok {
				return 0, false
			}
			sum += n
		}
		return sum, true
	case *grammar.Choice:
		m := 0
		for _, alt := range e.Alts {
			n, ok := textLen(alt, k, seen)
			if !ok {
				return 0, false
			}
			m = max(m, n)
		}
		return m, true
	case *grammar.Ref:
		r := k.byName[e.Name]
		if r == nil || seen[e.Name] || k.recursive[e.Name] || isOperators(r.Expr) {
			return 0, false
		}
		seen[e.Name] = true
		defer delete(seen, e.Name)
		return textLen(r.Expr, k, seen)
	}
	return 0, false
}

// readsOnly reports whether t is built from the variable v, int and bool literals, the operators
// that eval knows, and the calls of len that operand accepts (none when operand is nil).
func readsOnly(t grammar.Term, v string, operand func(*grammar.Call) bool) bool {
	switch t := t.(type) {
	case *grammar.IntLit, *grammar.BoolLit:
		return true
	case *grammar.VarRef:
		return t.Name == v
	case *grammar.TermGroup:
		return readsOnly(t.Term, v, operand)
	case *grammar.Unary:
		return (t.Op == "-" || t.Op == "!") && readsOnly(t.X, v, operand)
	case *grammar.Binary:
		return intOps[t.Op] && readsOnly(t.L, v, operand) && readsOnly(t.R, v, operand)
	case *grammar.Call:
		return operand != nil && t.Func == "len" && len(t.Args) == 1 && operand(t)
	}
	return false
}

var intOps = map[string]bool{"+": true, "-": true, "*": true, "/": true, "%": true,
	"==": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true, "&&": true, "||": true}

// domainEnv binds the variable v to x and each operand len($c) to a length.
type domainEnv struct {
	v    string
	x    int
	lens map[*grammar.Call]int
}

// evalResult is the outcome of eval.
type evalResult uint8

const (
	evalOK      evalResult = iota
	evalFailed             // an error of the action language, such as a division by zero
	evalUnknown            // an int beyond what every backend computes alike, or a term eval does not know
)

// maxExact bounds the ints that eval computes: every backend computes them alike.
const maxExact = 1 << 53

// eval evaluates t, which readsOnly accepts, in env, by the rules of the action
// language: division truncates toward zero, the remainder has the sign of the dividend, and
// dividing by zero is an error.
func eval(t grammar.Term, env domainEnv) (any, evalResult) {
	switch t := t.(type) {
	case *grammar.IntLit:
		return exact(t.Value)
	case *grammar.BoolLit:
		return t.Value, evalOK
	case *grammar.VarRef:
		if t.Name == env.v {
			return env.x, evalOK
		}
	case *grammar.Call:
		if n, ok := env.lens[t]; ok {
			return n, evalOK
		}
	case *grammar.TermGroup:
		return eval(t.Term, env)
	case *grammar.Unary:
		r, res := eval(t.X, env)
		if res != evalOK {
			return nil, res
		}
		switch r := r.(type) {
		case int:
			if t.Op == "-" {
				return exact(-r)
			}
		case bool:
			if t.Op == "!" {
				return !r, evalOK
			}
		}
	case *grammar.Binary:
		l, res := eval(t.L, env)
		if res != evalOK {
			return nil, res
		}
		if lb, isBool := l.(bool); isBool && (t.Op == "&&" || t.Op == "||") {
			if lb == (t.Op == "||") {
				return lb, evalOK
			}
			r, res := eval(t.R, env)
			if _, isBool := r.(bool); res == evalOK && !isBool {
				return nil, evalUnknown
			}
			return r, res
		}
		r, res := eval(t.R, env)
		if res != evalOK {
			return nil, res
		}
		return binaryOp(t.Op, l, r)
	}
	return nil, evalUnknown
}

func binaryOp(op string, l, r any) (any, evalResult) {
	if lb, isBool := l.(bool); isBool {
		if rb, isBool := r.(bool); isBool {
			switch op {
			case "==":
				return lb == rb, evalOK
			case "!=":
				return lb != rb, evalOK
			}
		}
		return nil, evalUnknown
	}
	li, lInt := l.(int)
	ri, rInt := r.(int)
	if !lInt || !rInt {
		return nil, evalUnknown
	}
	switch op {
	case "+":
		return exact(li + ri)
	case "-":
		return exact(li - ri)
	case "*":
		if li != 0 && (abs(ri) > maxExact/abs(li)) {
			return nil, evalUnknown
		}
		return exact(li * ri)
	case "/", "%":
		if ri == 0 {
			return nil, evalFailed
		}
		if op == "/" {
			return li / ri, evalOK
		}
		return li % ri, evalOK
	case "==":
		return li == ri, evalOK
	case "!=":
		return li != ri, evalOK
	case "<":
		return li < ri, evalOK
	case "<=":
		return li <= ri, evalOK
	case ">":
		return li > ri, evalOK
	case ">=":
		return li >= ri, evalOK
	}
	return nil, evalUnknown
}

func exact(n int) (any, evalResult) {
	if abs(n) > maxExact {
		return nil, evalUnknown
	}
	return n, evalOK
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
