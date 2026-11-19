package libgo

// The probes of the conformance test of the Flow operations (flow_conform_test.go), added to a copy
// of the library: generated code calls ConfPre before each call of a primitive operation and one of
// the pops after it, which record the call with the state of the parser around it.

import (
	"fmt"
	"reflect"
)

// ConfEvent is a call of a primitive operation. K identifies the call in the generated code; Used
// counts the units of budget charged, and Nested those charged by the probed calls inside the call.
type ConfEvent struct {
	K                   int
	Pos, PosAfter       int
	Used, UsedAfter     uint64
	Nested              uint64
	Kids, KidsAfter     int
	Rec, RecAfter       int
	Silent, SilentAfter int
	Args                map[int]any
	Results             []any
	Done                bool
	Parent              int
	p                   any
}

// ConfEvents are the calls in the order they began; ConfReset clears them.
var (
	ConfEvents []*ConfEvent
	confStack  []*ConfEvent
)

// ConfReset clears the record for a new parse.
func ConfReset() { ConfEvents, confStack = nil, nil }

func confState(p any) (pos int, used uint64, kids, rec, silent int) {
	var q *Parser
	switch x := p.(type) {
	case *Parser:
		q, kids = x, len(x.KidStack)
	case *TParser:
		q, kids = x.Parser, len(x.KidStack)
	default:
		panic(fmt.Sprintf("probe: parser %T", p))
	}
	if q.Control != nil {
		used = q.Control.budgetUsed()
	}
	return q.Pos, used, kids, len(q.Recovered), q.Silent
}

// ConfPre begins the call k on the parser p and returns p.
func ConfPre[P any](p P, k int) P {
	e := &ConfEvent{K: k, Args: map[int]any{}, p: p, Parent: -1}
	e.Pos, e.Used, e.Kids, e.Rec, e.Silent = confState(p)
	if n := len(confStack); n > 0 {
		e.Parent = confStack[n-1].K
	}
	ConfEvents = append(ConfEvents, e)
	confStack = append(confStack, e)
	return p
}

// ConfArg records x as the argument i of the call in progress and returns it.
func ConfArg[T any](i int, x T) T {
	confStack[len(confStack)-1].Args[i] = confVal(x)
	return x
}

// ConfPost ends the call in progress, with the results xs.
func ConfPost(xs ...any) {
	n := len(confStack) - 1
	e := confStack[n]
	confStack = confStack[:n]
	e.PosAfter, e.UsedAfter, e.KidsAfter, e.RecAfter, e.SilentAfter = confState(e.p)
	for _, x := range xs {
		e.Results = append(e.Results, confVal(x))
	}
	e.Done = true
	if n > 0 {
		confStack[n-1].Nested += e.UsedAfter - e.Used
	}
}

// Conf1 ends the call in progress, which ConfPre began in its first argument, with the result x.
func Conf1[T any](_ any, x T) T {
	ConfPost(x)
	return x
}

// ConfNot ends the call in progress, whose negation x is, and returns x.
func ConfNot(_ any, x bool) bool {
	ConfPost(!x)
	return x
}

// ConfPop2 and ConfPop3 end the call in progress with its results.
func ConfPop2[A, B any](a A, b B) (A, B) {
	ConfPost(a, b)
	return a, b
}

func ConfPop3[A, B, C any](a A, b B, c C) (A, B, C) {
	ConfPost(a, b, c)
	return a, b, c
}

// confVal returns the recorded form of a value: integers, runes and booleans as they are, a string
// as "s:" and its text, a list as "len:" and its length, a mark as "mark:" and its position, an error
// as "err:" and its message, and any other value as its type, or nil.
func confVal(x any) any {
	switch v := x.(type) {
	case nil:
		return nil
	case bool:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case string:
		return "s:" + v
	case Mark:
		return fmt.Sprintf("mark:%d", v.Pos)
	case error:
		if r := reflect.ValueOf(x); r.Kind() == reflect.Pointer && r.IsNil() {
			return nil
		}
		return "err:" + v.Error()
	}
	r := reflect.ValueOf(x)
	switch r.Kind() {
	case reflect.Slice:
		return fmt.Sprintf("len:%d", r.Len())
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Func:
		if r.IsNil() {
			return nil
		}
	}
	return fmt.Sprintf("%T", x)
}
