package conform

import (
	"fmt"
	"strings"
)

// The built-in table: calls of the built-in functions len and text, and of the compositions element
// and items, which every list, tuple and list-taking built-in function applies to its arguments, on the
// values of one fixture, and what each returns in the canonical form of MemberCanon. Each target builds
// the fixture over the input BuiltinInput and runs the calls through its printed compositions; the
// tests that do are the ones the mutants of that code name.

// BuiltinInput is the input of the fixture, whose ranges the text of a node reads.
const BuiltinInput = "0123456789"

// The values of the fixture, by the name a case reads, besides MemberNil, MemberInt and MemberList.
const (
	// BuiltinString is the string "ab".
	BuiltinString = `"ab"`
	// BuiltinBool is the bool true.
	BuiltinBool = "true"
	// BuiltinNode is a node that is not a string node, with the range 2 to 5 and two children, the
	// string nodes BuiltinText.
	BuiltinNode = "node"
	// BuiltinText is a string node with the range 2 to 4 that holds the text "xyz", which is not the
	// input of its range: the text of a string node is the one it holds, and its length is that of its
	// range.
	BuiltinText = "text"
	// BuiltinPair is a list value of the ints 1 and 2.
	BuiltinPair = "[1, 2]"
	// BuiltinFunc is a function, which is not a value of a term.
	BuiltinFunc = "func"
)

// The calls of the table, by the name a case calls.
const (
	// CallLen is len(x).
	CallLen = "len"
	// CallText is text(x).
	CallText = "text"
	// CallElement is the composition element for the built-in function list, which returns x as an
	// element: the canonical form of a box is "box " and that of the value it holds.
	CallElement = "element"
	// CallItems is the composition items for the built-in function concat, which returns the elements
	// of x as a list.
	CallItems = "items"
)

// BuiltinCase is a call Call of the value Value of the fixture, which returns Want: a value in the form
// of MemberCanon, or "error: " and the text of the failure. A Want that ends with "got " is a prefix: the
// name of the type of a function differs between targets.
type BuiltinCase struct{ Call, Value, Want string }

// BuiltinCases are the calls of the built-in table.
var BuiltinCases = []BuiltinCase{
	{CallLen, MemberNil, "0"},
	{CallLen, BuiltinString, "2"},
	{CallLen, BuiltinText, "2"},
	{CallLen, BuiltinNode, "2"},
	{CallLen, BuiltinPair, "2"},
	{CallLen, MemberList, "0"},
	{CallLen, MemberInt, "error: len: invalid argument int"},
	{CallLen, BuiltinBool, "error: len: invalid argument bool"},
	{CallText, MemberNil, `""`},
	{CallText, BuiltinString, `"ab"`},
	{CallText, BuiltinText, `"xyz"`},
	{CallText, BuiltinNode, `"234"`},
	{CallText, MemberInt, "error: text: invalid argument int"},
	{CallElement, MemberNil, "nil"},
	{CallElement, MemberInt, "box 1"},
	{CallElement, BuiltinString, `box "ab"`},
	{CallElement, BuiltinBool, "box true"},
	{CallElement, BuiltinPair, "[1, 2]"},
	{CallElement, BuiltinFunc, "error: list: elements must be values, got "},
	{CallItems, MemberNil, "[]"},
	{CallItems, BuiltinPair, "[1, 2]"},
	{CallItems, MemberList, "[]"},
	{CallItems, MemberInt, "error: concat: expected a list, got int"},
	{CallItems, BuiltinString, "error: concat: expected a list, got string"},
}

// BuiltinFn is the name of the built-in function that a call of CallElement or CallItems passes to the
// composition, as the text of its failure names it.
func BuiltinFn(call string) string {
	if call == CallItems {
		return "concat"
	}
	return "list"
}

// RunBuiltins runs every case of the built-in table on call, which returns the canonical result of a
// call of a target, and returns the cases whose result differs.
func RunBuiltins(target string, call func(call, value string) string) error {
	var bad []string
	for _, c := range BuiltinCases {
		got := call(c.Call, c.Value)
		ok := got == c.Want
		if strings.HasSuffix(c.Want, "got ") {
			ok = strings.HasPrefix(got, c.Want) && len(got) > len(c.Want)
		}
		if !ok {
			bad = append(bad, fmt.Sprintf("%s: %s(%s) = %s, want %s", target, c.Call, c.Value, got, c.Want))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("built-in table:\n%s", strings.Join(bad, "\n"))
	}
	return nil
}
