package state

import (
	"fmt"
	"strings"
	"zeroparse.dev/zp/internal/runtime/program"

	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/pkg/runtime/value"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// Values of actions and predicates are *Node, int, string, bool or nil.
// Nodes created by actions are not fresh, so they get no rule name.

// Evaluator evaluates an action expression (it differs per backend).
type Evaluator func(ctx *EvalCtx) (any, error)

type Local struct {
	Name string
	Val  any
	Next *Local
}

// EvalCtx is the context needed to evaluate value expressions.
type EvalCtx struct {
	P          *Parser
	Scope      *program.Scope
	Frame      *Frame
	Items      []*value.Node // elements referenced by $n
	Start, End int           // range matched by the rule
	Locals     *Local        // the locals of an OperatorsRule action ($value, $lhs, $rhs, $op)
	Cbase      int           // start, in p.Created, of the nodes created by this evaluation
}

// runAction evaluates the rule's action.
func (p *Parser) runAction(r *Rule, f *Frame, locals *Local, start, end int) *value.Node {
	ctx := p.UseCtx(EvalCtx{P: p, Scope: r.Scope, Frame: f, Locals: locals, Start: start, End: end, Cbase: len(p.Created)})
	return p.ActionResult(ctx, r.Act, r.WhereName())
}

func (p *Parser) ActionResult(ctx *EvalCtx, act Evaluator, where string) *value.Node {
	v, err := act(ctx)
	n, ok := v.(*value.Node)
	if err == nil && n != nil {
		for _, c := range p.Created[ctx.Cbase:] {
			p.Spend()
			if c == n {
				nodes.D(n).Start, nodes.D(n).End = int32(ctx.Start), int32(ctx.End)
				break
			}
		}
	}
	p.DropCreated(ctx.Cbase)
	if err != nil {
		p.FailAction(where, err)
	}
	if v == nil {
		return nil
	}
	if !ok {
		// A value other than a node travels boxed, with the range of the action's input.
		return p.NewNode(nodes.Data{Kind: valueimpl.BoxKind, Box: v, Start: int32(ctx.Start), End: int32(ctx.End)})
	}
	if n != nil {
		// The value of an action is final: a node it returns from the rule body (=> $1) is not
		// labeled by the rules that receive it, which might share it through the memo.
		nodes.D(n).Fresh = false
	}
	return n
}

// DropCreated releases tracking references without changing the created values.
func (p *Parser) DropCreated(base int) {
	ClearSpend(p, p.Created[base:])
	p.Created = p.Created[:base]
}

// ValKind is the operation val.kind, for the term compositions that the engine's backends print into
// their own packages.
func ValKind(x any) int { return valKind(x) }

func TypeName(v any) string {
	switch v := v.(type) {
	case nil:
		return "nil"
	case *value.Node:
		if v == nil {
			return "nil"
		}
		if nodes.D(v).Kind == valueimpl.BoxKind {
			return TypeName(nodes.D(v).Box)
		}
		if nodes.D(v).Kind.IsNodeType() {
			return v.Type()
		}
		return nodes.ValueTypeName(nodes.ShapeOf(v), v.Type(), len(nodes.D(v).Children))
	case int:
		return "int"
	case string:
		return "string"
	case bool:
		return "bool"
	}
	return fmt.Sprintf("%T", v)
}

// Member reads x.name, as the composition member (term_exec.go) does.
func (c *EvalCtx) Member(x any, name string) (any, error) {
	kind, field := memberKind(name)
	return c.P.termMember(c, kind, x, name, field)
}

// memberZero is the operation node.zero: the zero value of the declared type of the field name of the
// struct n, and false if its type declares no such field. Under budget controls it charges for the
// declared fields it scans, and for the values it makes.
func (p *Parser) memberZero(c *EvalCtx, n *value.Node, name string) (any, bool) {
	if !p.HasValueField(p.Prog.Types[n.Type()], name) {
		return nil, false
	}
	v, err := c.zeroValue(p.Prog.StructZeroOf(n.Type()).Zeros[name])
	if err != nil {
		// Making a zero value fails only on a function, which no zero value holds.
		panic("core: zero value of " + n.Type() + "." + name + ": " + err.Error())
	}
	return v, true
}

// zeroValue builds the zero value of the descriptor d (compile.Program.ZeroDesc).
func (c *EvalCtx) zeroValue(d string) (any, error) {
	v, _, err := c.zeroFrom(d)
	return v, err
}

// zeroFrom builds the zero value that d starts with and returns it with the rest of d.
func (c *EvalCtx) zeroFrom(d string) (any, string, error) {
	if d == "" {
		return nil, d, nil
	}
	switch d[0] {
	case 'i':
		return 0, d[1:], nil
	case 's':
		return "", d[1:], nil
	case 'b':
		return false, d[1:], nil
	case 'l':
		return c.ListNode(c.P.Kids(len(c.P.KidStack))), d[1:], nil
	case '(':
		p := c.P
		kb := len(p.KidStack)
		d = d[1:]
		for d != "" && d[0] != ')' {
			var e any
			var err error
			e, d, err = c.zeroFrom(d)
			if err == nil {
				p.Spend()
				err = c.Push("tuple", e)
			}
			if err != nil {
				p.DropKids(kb)
				return nil, d, err
			}
		}
		return c.TupleNode(p.Kids(kb)), strings.TrimPrefix(d, ")"), nil
	case '{':
		// The zero value of a struct type is a struct with no field set, whose fields read as their own
		// zero values, so that making it costs the same whatever the type holds.
		name, rest, _ := strings.Cut(d[1:], "}")
		n, err := c.NewStruct(name, nil, nil)
		if err != nil {
			return nil, rest, err
		}
		return n, rest, nil
	}
	return nil, d[1:], nil
}

// NewStruct creates a struct node. Its range covers the ranges of the fields whose values are
// nodes, or is the rule's range if there are none.
func (c *EvalCtx) NewStruct(typ string, names []string, vals []any) (*value.Node, error) {
	n := c.P.NewNode(nodes.Data{Kind: c.P.Prog.TypeKind(typ), Fields: c.P.fields(len(names))})
	first := true
	for i, name := range names {
		c.P.Spend()
		v := vals[i]
		c.P.SetValueField(&nodes.D(n).Fields, name, v)
		first = c.P.termCover(n, v, first)
	}
	c.P.termMade(c, n, first)
	return n, nil
}

func (c *EvalCtx) NewList(items []*value.Node) *value.Node {
	kids := c.P.Nodes(len(items))
	CopySpend(c.P, kids, items)
	return c.ListNode(kids)
}

// ListNode returns a list value with the child slice kids (which it takes over). Its range,
// which the value model does not show, covers the elements that are nodes.
func (c *EvalCtx) ListNode(kids []*value.Node) *value.Node {
	return c.P.termHull(c, false, kids)
}

// TupleNode is ListNode for a tuple value.
func (c *EvalCtx) TupleNode(kids []*value.Node) *value.Node {
	return c.P.termHull(c, true, kids)
}

// ValueListItems returns the items of the list node v that the built-in fn reads: none for nil, and
// an error for a value that is not a list.
func (p *Parser) ValueListItems(fn string, v any) ([]*value.Node, error) {
	return p.termItems(fn, v)
}

// Push pushes v onto KidStack as the next element of the list or the tuple that the built-in or literal
// fn makes, as the composition push does. An int, string or bool travels boxed, with the range of the
// action's input, as the result of an action does.
func (c *EvalCtx) Push(fn string, v any) error {
	return c.P.termPush(c, fn, v)
}

// Keep returns c, or a copy of c on the heap if it belongs to a lambda call (closure.apply),
// whose context and parameters are freed when the call returns while a lambda created in it can
// be returned from it.
func (c *EvalCtx) Keep() *EvalCtx {
	if c == &c.P.ectx {
		return c
	}
	k := *c
	var head *Local
	for l, link := c.Locals, &head; l != nil; l = l.Next {
		c.P.Spend()
		*link = &Local{Name: l.Name, Val: l.Val}
		link = &(*link).Next
	}
	k.Locals = head
	return &k
}
