// Package codepass holds the passes of Exec IR (design 065): transformations of functions that keep
// their behavior and that every backend prints the result of.
//
// Specialize makes the copy of a function for one set of features, in which every feature is a
// constant: the hand-written variants of one mechanism for each build (ordinary, budget-controlled,
// observed, node or typed values) become specializations of one composition. Simplify folds
// constants and removes code that cannot run, which is what makes a specialization lean.
package codepass

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/ir/code"
)

// Specialize returns a copy of the function f of the module m named name and specialized for the
// features on, which must be features of m: each Feature expression becomes the constant true if
// its feature is in on and false otherwise, and the copy is simplified (Simplify). The copy's
// Features are on, so that it may call the operations of those features and no others.
func Specialize(m *code.Module, f *code.Func, name string, on []string) (*code.Func, error) {
	for _, ft := range on {
		if !slices.Contains(m.Features, ft) {
			return nil, fmt.Errorf("specialize %s: undeclared feature %s", f.Name, ft)
		}
	}
	c := Clone(f)
	c.Name, c.Features = name, slices.Clone(on)
	c.Body = mapBlock(c.Body, func(x code.Expr) code.Expr {
		if ft, ok := x.(*code.Feature); ok {
			return &code.Const{Type: code.TBool, Value: slices.Contains(on, ft.Name)}
		}
		return x
	})
	Simplify(c)
	return c, nil
}

// Clone returns a deep copy of f.
func Clone(f *code.Func) *code.Func {
	c := *f
	c.Params = slices.Clone(f.Params)
	c.Results = slices.Clone(f.Results)
	c.Locals = slices.Clone(f.Locals)
	c.Features = slices.Clone(f.Features)
	c.Body = mapBlock(f.Body, func(x code.Expr) code.Expr { return x })
	return &c
}

// mapBlock returns a copy of b in which every expression is replaced by what fn returns for it,
// after its subexpressions are replaced. Immediate operands are shared, not copied.
func mapBlock(b *code.Block, fn func(code.Expr) code.Expr) *code.Block {
	if b == nil {
		return nil
	}
	var x func(code.Expr) code.Expr
	x = func(e code.Expr) code.Expr {
		switch e := e.(type) {
		case nil:
			return nil
		case *code.Const:
			c := *e
			return fn(&c)
		case *code.Local:
			c := *e
			return fn(&c)
		case *code.Reg:
			c := *e
			return fn(&c)
		case *code.Feature:
			c := *e
			return fn(&c)
		case *code.Unary:
			return fn(&code.Unary{Op: e.Op, X: x(e.X)})
		case *code.Binary:
			return fn(&code.Binary{Op: e.Op, X: x(e.X), Y: x(e.Y)})
		case *code.Convert:
			return fn(&code.Convert{To: e.To, X: x(e.X)})
		case *code.Call:
			return fn(call(e, x))
		}
		panic(fmt.Sprintf("codepass: unknown expression %T", e))
	}
	set := func(s *code.Set) *code.Set {
		if s == nil {
			return nil
		}
		c := &code.Set{Dst: slices.Clone(s.Dst)}
		for _, v := range s.Values {
			c.Values = append(c.Values, x(v))
		}
		return c
	}
	var blk func(*code.Block) *code.Block
	blk = func(b *code.Block) *code.Block {
		if b == nil {
			return nil
		}
		out := &code.Block{Label: b.Label}
		for _, st := range b.Stmts {
			var n code.Stmt
			switch st := st.(type) {
			case *code.Block:
				n = blk(st)
			case *code.Loop:
				n = &code.Loop{Label: st.Label, Init: set(st.Init), Cond: x(st.Cond), Post: set(st.Post), Body: blk(st.Body)}
			case *code.If:
				n = &code.If{Init: set(st.Init), Cond: x(st.Cond), Then: blk(st.Then), Else: blk(st.Else)}
			case *code.Protect:
				n = &code.Protect{Body: blk(st.Body), Finally: blk(st.Finally)}
			case *code.Let:
				n = &code.Let{Names: slices.Clone(st.Names), Value: x(st.Value)}
			case *code.Set:
				n = set(st)
			case *code.Do:
				n = &code.Do{Call: call(st.Call, x)}
			case *code.Return:
				r := &code.Return{}
				for _, v := range st.Values {
					r.Values = append(r.Values, x(v))
				}
				n = r
			case *code.Assert:
				n = &code.Assert{Cond: x(st.Cond), Msg: st.Msg}
			case *code.Exit:
				c := *st
				n = &c
			case *code.Break:
				c := *st
				n = &c
			case *code.Continue:
				c := *st
				n = &c
			case *code.Comment:
				c := *st
				n = &c
			default:
				panic(fmt.Sprintf("codepass: unknown statement %T", st))
			}
			out.Stmts = append(out.Stmts, n)
		}
		return out
	}
	return blk(b)
}

func call(c *code.Call, x func(code.Expr) code.Expr) *code.Call {
	n := &code.Call{Callee: c.Callee, Imm: c.Imm}
	if c.Fn != nil {
		n.Fn = x(c.Fn)
	}
	for _, a := range c.Args {
		n.Args = append(n.Args, x(a))
	}
	return n
}

// Simplify folds the constant expressions of f and removes the code that cannot run, in place:
//
//   - an operator on constants is replaced by its value (a division by zero is kept, so that it
//     fails where it ran), "!" of a constant, and "&&" and "||" with a constant operand that decides
//     them or drops out (an operand that is dropped must have no call);
//   - an If with a constant condition is replaced by its Init and the branch that runs; a Loop whose
//     condition is the constant false by its Init;
//   - statements after an Exit, Break, Continue or Return in the same block are removed;
//   - an unlabeled block that declares no local is spliced into its parent, and a block label that
//     no Exit names is removed.
func Simplify(f *code.Func) {
	f.Body = mapBlock(f.Body, fold)
	f.Body.Stmts = prune(f.Body.Stmts)
	used := map[string]bool{}
	code.Walk(f.Body, func(s code.Stmt) {
		if e, ok := s.(*code.Exit); ok {
			used[e.Label] = true
		}
	}, nil)
	code.Walk(f.Body, func(s code.Stmt) {
		if b, ok := s.(*code.Block); ok && !used[b.Label] {
			b.Label = ""
		}
	}, nil)
	f.Body.Stmts = splice(f.Body.Stmts)
}

// constant returns the value of x if it is a constant.
func constant(x code.Expr) (any, bool) {
	c, ok := x.(*code.Const)
	if !ok || c.Type.Kind == code.KHandle {
		return nil, false
	}
	return c.Value, true
}

// pure reports whether x has no call.
func pure(x code.Expr) bool {
	p := true
	code.Walk(&code.Block{Stmts: []code.Stmt{&code.Assert{Cond: x}}}, nil, func(e code.Expr) {
		if _, ok := e.(*code.Call); ok {
			p = false
		}
	})
	return p
}

func boolConst(v bool) code.Expr { return &code.Const{Type: code.TBool, Value: v} }

// fold folds the expression x, whose subexpressions are folded.
func fold(x code.Expr) code.Expr {
	switch x := x.(type) {
	case *code.Unary:
		v, ok := constant(x.X)
		if !ok {
			return x
		}
		if x.Op == "!" {
			return boolConst(!v.(bool))
		}
		c := x.X.(*code.Const)
		return &code.Const{Type: c.Type, Value: code.Apply("-", code.ConvertValue(int64(0), c.Type), v)}
	case *code.Binary:
		a, aok := constant(x.X)
		b, bok := constant(x.Y)
		switch x.Op {
		case "&&", "||":
			decide := x.Op == "||" // the value of an operand that decides the result
			switch {
			case aok && a.(bool) == decide:
				return boolConst(decide)
			case aok:
				return x.Y
			case bok && b.(bool) != decide:
				return x.X
			case bok && pure(x.X):
				return boolConst(decide)
			}
			return x
		}
		if !aok || !bok {
			return x
		}
		if (x.Op == "/" || x.Op == "%") && code.Apply("==", b, code.ConvertValue(int64(0), x.Y.(*code.Const).Type)).(bool) {
			return x
		}
		v := code.Apply(x.Op, a, b)
		t := x.X.(*code.Const).Type
		if _, isBool := v.(bool); isBool {
			t = code.TBool
		}
		return &code.Const{Type: t, Value: v}
	case *code.Convert:
		if v, ok := constant(x.X); ok {
			return &code.Const{Type: x.To, Value: code.ConvertValue(v, x.To)}
		}
	}
	return x
}

// prune replaces Ifs and Loops with constant conditions and removes unreachable statements, in a
// list of statements and its nested blocks.
func prune(ss []code.Stmt) []code.Stmt {
	var out []code.Stmt
	for _, st := range ss {
		switch s := st.(type) {
		case *code.Block:
			s.Stmts = prune(s.Stmts)
		case *code.Loop:
			if v, ok := constant(s.Cond); ok && !v.(bool) {
				if s.Init != nil {
					out = append(out, s.Init)
				}
				continue
			}
			if v, ok := constant(s.Cond); ok && v.(bool) {
				s.Cond = nil
			}
			s.Body.Stmts = prune(s.Body.Stmts)
		case *code.If:
			s.Then.Stmts = prune(s.Then.Stmts)
			if s.Else != nil {
				s.Else.Stmts = prune(s.Else.Stmts)
			}
			if v, ok := constant(s.Cond); ok {
				if s.Init != nil {
					out = append(out, s.Init)
				}
				if v.(bool) {
					out = append(out, s.Then)
				} else if s.Else != nil {
					out = append(out, s.Else)
				}
				continue
			}
		case *code.Protect:
			s.Body.Stmts = prune(s.Body.Stmts)
			s.Finally.Stmts = prune(s.Finally.Stmts)
		}
		out = append(out, st)
		switch st.(type) {
		case *code.Exit, *code.Break, *code.Continue, *code.Return:
			return out
		}
	}
	return out
}

// splice splices unlabeled blocks that declare no local into the list, in it and in its nested
// blocks, and removes empty unlabeled blocks.
func splice(ss []code.Stmt) []code.Stmt {
	var out []code.Stmt
	for _, st := range ss {
		switch s := st.(type) {
		case *code.Block:
			s.Stmts = splice(s.Stmts)
			if s.Label == "" && !declares(s) {
				out = append(out, s.Stmts...)
				continue
			}
		case *code.Loop:
			s.Body.Stmts = splice(s.Body.Stmts)
		case *code.If:
			s.Then.Stmts = splice(s.Then.Stmts)
			if s.Else != nil {
				s.Else.Stmts = splice(s.Else.Stmts)
				if len(s.Else.Stmts) == 0 {
					s.Else = nil
				}
			}
		case *code.Protect:
			s.Body.Stmts = splice(s.Body.Stmts)
			s.Finally.Stmts = splice(s.Finally.Stmts)
		}
		out = append(out, st)
	}
	return out
}

// declares reports whether the block declares a local directly.
func declares(b *code.Block) bool {
	for _, st := range b.Stmts {
		if _, ok := st.(*code.Let); ok {
			return true
		}
	}
	return false
}
