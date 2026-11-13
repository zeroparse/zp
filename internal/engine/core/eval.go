package core

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// Values of actions and predicates are *Node, int, string, bool, nil, or *closure.
// Nodes created by actions are not fresh, so they get no rule name.

// Function is a function (lambda) passed to a built-in function: *closure when evaluating the
// AST, *vmFunc in bytecode.
type Function interface {
	Arity() int
	// Apply calls the function with the arguments x and y; y is ignored by a function of one
	// parameter. (Arguments are not passed as a slice, which would escape through the interface.)
	Apply(x, y any) (any, error)
}

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
	Scope      *compile.Scope
	Frame      *Frame
	Items      []*parse.Node // elements referenced by $n
	Start, End int           // range matched by the rule
	Locals     *Local        // lambda arguments and OperatorsRule's $lhs, $rhs, $op
	Cbase      int           // start, in p.Created, of the nodes created by this evaluation
}

// runAction evaluates the rule's action.
func (p *Parser) runAction(r *Rule, f *Frame, locals *Local, start, end int) *parse.Node {
	ctx := p.UseCtx(EvalCtx{P: p, Scope: r.Scope, Frame: f, Locals: locals, Start: start, End: end, Cbase: len(p.Created)})
	return p.ActionResult(ctx, r.Act, r.WhereName())
}

func (p *Parser) ActionResult(ctx *EvalCtx, act Evaluator, where string) *parse.Node {
	v, err := act(ctx)
	n, ok := v.(*parse.Node)
	if err == nil && n != nil {
		for _, c := range p.Created[ctx.Cbase:] {
			p.Spend()
			if c == n {
				value.D(n).Start, value.D(n).End = int32(ctx.Start), int32(ctx.End)
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
		if _, isFn := v.(Function); isFn {
			p.FailAction(where, p.ActionError("result must be a value, got a function"))
		}
		// A value other than a node travels boxed, with the range of the action's input.
		return p.NewNode(value.Data{Kind: noderep.BoxKind, Box: v, Start: int32(ctx.Start), End: int32(ctx.End)})
	}
	if n != nil {
		// The value of an action is final: a node it returns from the rule body (=> $1) is not
		// labeled by the rules that receive it, which might share it through the memo.
		value.D(n).Fresh = false
	}
	return n
}

// DropCreated releases tracking references without changing the created values.
func (p *Parser) DropCreated(base int) {
	ClearSpend(p, p.Created[base:])
	p.Created = p.Created[:base]
}

// IsFalse is the operation term.isFalse.
func IsFalse(v any) bool {
	b, ok := v.(bool)
	return ok && !b
}

func TypeName(v any) string {
	switch v := v.(type) {
	case nil:
		return "nil"
	case *parse.Node:
		if v == nil {
			return "nil"
		}
		if value.D(v).Kind == noderep.BoxKind {
			return TypeName(value.D(v).Box)
		}
		if value.D(v).Kind.IsNodeType() {
			return v.Type()
		}
		return value.ValueTypeName(value.ShapeOf(v), v.Type(), len(value.D(v).Children))
	case int:
		return "int"
	case string:
		return "string"
	case bool:
		return "bool"
	case Function:
		return "function"
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
func (p *Parser) memberZero(c *EvalCtx, n *parse.Node, name string) (any, bool) {
	if !p.HasValueField(p.Prog.Types[n.Type()].(*grammar.TypeDef), name) {
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
func (c *EvalCtx) NewStruct(typ string, names []string, vals []any) (*parse.Node, error) {
	n := c.P.NewNode(value.Data{Kind: c.P.Prog.TypeKind(typ), Fields: c.P.fields(len(names))})
	first := true
	for i, name := range names {
		c.P.Spend()
		v := vals[i]
		if _, isFn := v.(Function); isFn {
			return nil, c.P.ActionError("field ", name, ": cannot store a function")
		}
		c.P.SetValueField(&value.D(n).Fields, name, v)
		first = c.P.termCover(n, v, first)
	}
	c.P.termMade(c, n, first)
	return n, nil
}

func (c *EvalCtx) NewList(items []*parse.Node) *parse.Node {
	kids := c.P.Nodes(len(items))
	CopySpend(c.P, kids, items)
	return c.ListNode(kids)
}

// ListNode returns a list value with the child slice kids (which it takes over). Its range,
// which the value model does not show, covers the elements that are nodes.
func (c *EvalCtx) ListNode(kids []*parse.Node) *parse.Node {
	return c.P.termHull(c, false, kids)
}

// TupleNode is ListNode for a tuple value.
func (c *EvalCtx) TupleNode(kids []*parse.Node) *parse.Node {
	return c.P.termHull(c, true, kids)
}

// Builtin calls the built-in function fn with evaluated arguments.
func (c *EvalCtx) Builtin(fn string, args []any) (any, error) {
	switch fn {
	case "len":
		return c.P.termLength(args[0])
	case "text":
		s, err := c.TextString(args[0])
		if err != nil {
			return nil, err
		}
		return s, nil
	case "foldl", "foldr":
		f, ok := args[2].(Function)
		if !ok || f.Arity() != 2 {
			return nil, c.P.ActionError(fn, ": third argument must be a function of two parameters")
		}
		items, err := c.P.ValueListItems(fn, args[1])
		if err != nil {
			return nil, err
		}
		acc := args[0]
		right := fn == "foldr"
		for i := range items {
			c.P.Spend()
			if acc, err = f.Apply(acc, c.P.termItemAt(items, i, right)); err != nil {
				return nil, err
			}
		}
		return acc, nil
	case "map", "list", "concat", "filter":
		// The elements are gathered on KidStack; lambdas that build lists push above them.
		base := len(c.P.KidStack)
		if err := c.PushElems(fn, args); err != nil {
			c.P.DropKids(base)
			return nil, err
		}
		return c.ListNode(c.P.Kids(base)), nil
	}
	return nil, c.P.ActionError("unknown function ", fn)
}

// TextString returns text(v) (the built-in) as a string.
func (c *EvalCtx) TextString(v any) (string, error) {
	return c.P.termText(v)
}

// PushElems pushes onto KidStack the elements of the list that the list built-in fn (map, list
// or concat) makes from args.
func (c *EvalCtx) PushElems(fn string, args []any) error {
	switch fn {
	case "filter":
		return c.pushFiltered(args)
	case "map":
		f, ok := args[1].(Function)
		if !ok || f.Arity() != 1 {
			return c.P.ActionError("map: second argument must be a function of one parameter")
		}
		items, err := c.P.ValueListItems("map", args[0])
		if err != nil {
			return err
		}
		for i := range items {
			c.P.Spend()
			v, err := f.Apply(c.P.termItemAt(items, i, false), nil)
			if err != nil {
				return err
			}
			if err := c.Push("map", v); err != nil {
				return err
			}
		}
	case "list":
		for _, a := range args {
			c.P.Spend()
			if err := c.Push("list", a); err != nil {
				return err
			}
		}
	case "concat":
		for _, a := range args {
			c.P.Spend()
			if err := c.P.termPushItems(c, "concat", a); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValueListItems returns the items of the list node v that the built-in fn reads: none for nil, and
// an error for a value that is not a list.
func (p *Parser) ValueListItems(fn string, v any) ([]*parse.Node, error) {
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
