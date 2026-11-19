package code

import "slices"

// AttrsCloner is implemented by the attributes of a function (Func.Attrs) that refer to its
// statements or hold anything a pass changes: CloneAttrs returns a copy whose references go to the
// statements of the copy of the function, through c. Attributes that do not implement it are
// shared by the copy.
type AttrsCloner interface {
	CloneAttrs(c *Cloner) any
}

// Cloner copies the functions of a module, each statement and expression once, so that the
// attributes of a function can follow a reference to a statement to its copy (AttrsCloner).
type Cloner struct {
	stmts map[Stmt]Stmt
	exprs map[Expr]Expr
}

// Clone returns a copy of m that a pass may change without changing m: new functions with new
// statements, expressions and attributes (AttrsCloner), and new lists of the declarations. What no
// pass changes in place is shared: the declarations themselves (components, registers, handle
// types, operations) and the immediate operands of calls.
func (m *Module) Clone() *Module {
	c := &Cloner{stmts: map[Stmt]Stmt{}, exprs: map[Expr]Expr{}}
	out := *m
	out.Dialects = slices.Clone(m.Dialects)
	out.Features = slices.Clone(m.Features)
	out.Components = slices.Clone(m.Components)
	out.Registers = slices.Clone(m.Registers)
	out.Handles = slices.Clone(m.Handles)
	out.Ops = slices.Clone(m.Ops)
	out.Funcs = make([]*Func, len(m.Funcs))
	for i, f := range m.Funcs {
		out.Funcs[i] = c.Func(f)
	}
	return &out
}

// Func returns the copy of f.
func (c *Cloner) Func(f *Func) *Func {
	if f == nil {
		return nil
	}
	nf := *f
	nf.Params = slices.Clone(f.Params)
	nf.Results = slices.Clone(f.Results)
	nf.Locals = slices.Clone(f.Locals)
	nf.Features = slices.Clone(f.Features)
	nf.Body = c.Block(f.Body)
	if a, ok := f.Attrs.(AttrsCloner); ok {
		nf.Attrs = a.CloneAttrs(c)
	}
	return &nf
}

// Block returns the copy of b.
func (c *Cloner) Block(b *Block) *Block {
	if b == nil {
		return nil
	}
	return c.Stmt(b).(*Block)
}

// Set returns the copy of s.
func (c *Cloner) Set(s *Set) *Set {
	if s == nil {
		return nil
	}
	return c.Stmt(s).(*Set)
}

// Stmt returns the copy of s, made the first time it is asked for.
func (c *Cloner) Stmt(s Stmt) Stmt {
	if s == nil {
		return nil
	}
	if x, ok := c.stmts[s]; ok {
		return x
	}
	var out Stmt
	switch s := s.(type) {
	case *Block:
		y := &Block{Label: s.Label, Stmts: make([]Stmt, len(s.Stmts))}
		c.stmts[s] = y
		for i, x := range s.Stmts {
			y.Stmts[i] = c.Stmt(x)
		}
		return y
	case *Loop:
		y := *s
		y.Init, y.Cond, y.Post, y.Body = c.Set(s.Init), c.Expr(s.Cond), c.Set(s.Post), c.Block(s.Body)
		out = &y
	case *If:
		y := *s
		y.Init, y.Cond, y.Then, y.Else = c.Set(s.Init), c.Expr(s.Cond), c.Block(s.Then), c.Block(s.Else)
		out = &y
	case *Let:
		out = &Let{Names: slices.Clone(s.Names), Value: c.Expr(s.Value)}
	case *Set:
		out = &Set{Dst: slices.Clone(s.Dst), Values: c.exprList(s.Values)}
	case *Do:
		out = &Do{Call: c.call(s.Call)}
	case *Exit:
		y := *s
		out = &y
	case *Break:
		y := *s
		out = &y
	case *Continue:
		y := *s
		out = &y
	case *Return:
		out = &Return{Values: c.exprList(s.Values)}
	case *Assert:
		out = &Assert{Cond: c.Expr(s.Cond), Msg: s.Msg}
	case *Protect:
		out = &Protect{Body: c.Block(s.Body), Finally: c.Block(s.Finally)}
	case *Comment:
		y := *s
		out = &y
	default:
		panic("code: Clone of an unknown statement")
	}
	c.stmts[s] = out
	return out
}

// Expr returns the copy of e, made the first time it is asked for.
func (c *Cloner) Expr(e Expr) Expr {
	if e == nil {
		return nil
	}
	if x, ok := c.exprs[e]; ok {
		return x
	}
	var out Expr
	switch e := e.(type) {
	case *Const:
		y := *e
		out = &y
	case *Local:
		y := *e
		out = &y
	case *Reg:
		y := *e
		out = &y
	case *Feature:
		y := *e
		out = &y
	case *Unary:
		out = &Unary{Op: e.Op, X: c.Expr(e.X)}
	case *Binary:
		out = &Binary{Op: e.Op, X: c.Expr(e.X), Y: c.Expr(e.Y)}
	case *Convert:
		out = &Convert{To: e.To, X: c.Expr(e.X)}
	case *Call:
		return c.call(e)
	default:
		panic("code: Clone of an unknown expression")
	}
	c.exprs[e] = out
	return out
}

func (c *Cloner) call(e *Call) *Call {
	if e == nil {
		return nil
	}
	if x, ok := c.exprs[e]; ok {
		return x.(*Call)
	}
	y := &Call{Callee: e.Callee, Imm: e.Imm}
	c.exprs[e] = y
	y.Args, y.Fn = c.exprList(e.Args), c.Expr(e.Fn)
	return y
}

func (c *Cloner) exprList(es []Expr) []Expr {
	if es == nil {
		return nil
	}
	out := make([]Expr, len(es))
	for i, e := range es {
		out[i] = c.Expr(e)
	}
	return out
}
