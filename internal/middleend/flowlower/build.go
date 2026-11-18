package flowlower

import (
	"fmt"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
)

// The constructions of Code IR that the lowering writes: the operations of the Flow dialect, with
// their dynamic operands as arguments and their static data as the immediate, and the base
// statements and expressions they combine with.

// use reads the variable v.
func use(v string) code.Expr { return code.L(v) }

// null is the null value.
func null() code.Expr { return code.Null(code.THandle(flow.TValue)) }

// num is the integer n.
func num(n int) code.Expr { return code.Int(int64(n)) }

// pos reads the position register.
func pos() code.Expr { return code.R(flow.Pos) }

// call returns a call of the Flow operation name with the immediate imm and the arguments args.
func call(name string, imm any, args ...code.Expr) *code.Call {
	return &code.Call{Callee: name, Args: args, Imm: imm}
}

// do returns the statement that calls the Flow operation name for its effect.
func do(name string, imm any, args ...code.Expr) code.Stmt {
	return &code.Do{Call: call(name, imm, args...)}
}

// inc adds one to the variable v.
func inc(v string) *code.Set {
	return code.Assign(v, code.Add(use(v), num(1)))
}

// peek reads the next character into the variables ch, size and ok; an empty name discards the result.
func peek(ch, size, ok string) code.Stmt {
	return &code.Set{Dst: []code.Place{{Name: ch}, {Name: size}, {Name: ok}}, Values: []code.Expr{call(flow.OpPeek, nil)}}
}

// unshare copies each expression of the block b that the lowering placed more than once, such as a
// value that it both assigns and returns, so that every node of the function has one parent and a
// pass can rewrite one place without changing another.
func unshare(b *code.Block) {
	seen := map[code.Expr]bool{}
	var expr func(x code.Expr) code.Expr
	expr = func(x code.Expr) code.Expr {
		if x == nil {
			return nil
		}
		if seen[x] {
			x = clone(x)
		}
		seen[x] = true
		switch x := x.(type) {
		case *code.Call:
			for i, a := range x.Args {
				x.Args[i] = expr(a)
			}
		case *code.Unary:
			x.X = expr(x.X)
		case *code.Binary:
			x.X, x.Y = expr(x.X), expr(x.Y)
		}
		return x
	}
	var set func(s *code.Set)
	set = func(s *code.Set) {
		if s == nil {
			return
		}
		for i, v := range s.Values {
			s.Values[i] = expr(v)
		}
	}
	var block func(b *code.Block)
	block = func(b *code.Block) {
		if b == nil {
			return
		}
		for _, s := range b.Stmts {
			switch s := s.(type) {
			case *code.Block:
				block(s)
			case *code.Loop:
				set(s.Init)
				s.Cond = expr(s.Cond)
				set(s.Post)
				block(s.Body)
			case *code.If:
				set(s.Init)
				s.Cond = expr(s.Cond)
				block(s.Then)
				block(s.Else)
			case *code.Set:
				set(s)
			case *code.Do:
				s.Call = expr(s.Call).(*code.Call)
			case *code.Return:
				for i, v := range s.Values {
					s.Values[i] = expr(v)
				}
			case *code.Exit, *code.Break, *code.Continue, *code.Comment:
			default:
				panic(fmt.Sprintf("flowlower: statement %T", s))
			}
		}
	}
	block(b)
}

// clone returns a copy of the expression x, which unshare then walks.
func clone(x code.Expr) code.Expr {
	switch x := x.(type) {
	case *code.Local:
		c := *x
		return &c
	case *code.Reg:
		c := *x
		return &c
	case *code.Const:
		c := *x
		return &c
	case *code.Call:
		c := *x
		c.Args = append([]code.Expr(nil), x.Args...)
		return &c
	case *code.Unary:
		c := *x
		return &c
	case *code.Binary:
		c := *x
		return &c
	}
	panic(fmt.Sprintf("flowlower: expression %T", x))
}
