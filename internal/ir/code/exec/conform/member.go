package conform

import (
	"fmt"
	"strings"
)

// The member table: reads of the members of the values of one fixture, and what each read returns, in
// the canonical form of MemberCanon. Each target builds the fixture and runs the reads through its
// printed composition member; the tests that do are the ones the mutants of that code name.

// MemberStructs are the struct types of the fixture, with each field and the zero descriptor of its
// type (compile.Program.ZeroDesc), in declaration order: S has an int, a string, a list, a tuple and a
// struct field.
var MemberStructs = []struct {
	Name   string
	Fields [][2]string
}{
	{"Inner", [][2]string{{"N", "i"}}},
	{"S", [][2]string{{"A", "i"}, {"B", "s"}, {"L", "l"}, {"T", "(is)"}, {"In", "{Inner}"}}},
}

// The values of the fixture, by the name a case reads.
const (
	// MemberNil is nil.
	MemberNil = "nil"
	// MemberInt is the int 1.
	MemberInt = "1"
	// MemberNode is a tuple node with no elements and the range 2 to 5, whose capture y holds the int 7
	// and which has no capture z.
	MemberNode = "node"
	// MemberEmpty is a struct of type S with no field set.
	MemberEmpty = "S{}"
	// MemberSet is a struct of type S with A set to 3 and B to "x".
	MemberSet = `S{A: 3, B: "x"}`
	// MemberList is an empty list value.
	MemberList = "[]"
)

// MemberCase is a read of the member Member of the value Value of the fixture, which returns Want: a
// value in the form of MemberCanon, or "error: " and the text of the failure.
type MemberCase struct{ Value, Member, Want string }

// MemberCases are the reads of the member table: every kind of member, of a node, a struct and a list,
// set and unset fields, and each failure.
var MemberCases = []MemberCase{
	{MemberInt, "A", "error: cannot access .A of int"},
	{MemberNil, "$y", "error: cannot access .$y of nil"},
	{MemberNode, "Start", "2"},
	{MemberNode, "End", "5"},
	{MemberNode, "Value", "()"},
	{MemberNode, "$y", "7"},
	{MemberNode, "$z", "nil"},
	{MemberSet, "A", "3"},
	{MemberSet, "B", `"x"`},
	{MemberEmpty, "A", "0"},
	{MemberEmpty, "B", `""`},
	{MemberEmpty, "L", "[]"},
	{MemberEmpty, "T", `(0, "")`},
	{MemberEmpty, "In", "Inner"},
	// Start, End and Value are fields of a value that is not a node.
	{MemberEmpty, "Start", "error: S has no field Start"},
	{MemberList, "Start", "nil"},
	{MemberEmpty, "Q", "error: S has no field Q"},
	{MemberSet, "$A", "error: S has no capture A"},
}

// MemberCanon returns the canonical form of a value of a member read, given what kind of value it is:
// nil, an int, a string (quoted), a struct (its type name), and a list or a tuple of the canonical forms
// of its elements in brackets or parentheses.
func MemberCanon(kind string, scalar any, elems []string) string {
	switch kind {
	case "nil":
		return "nil"
	case "int", "bool":
		return fmt.Sprint(scalar)
	case "string":
		return fmt.Sprintf("%q", scalar)
	case "list":
		return "[" + strings.Join(elems, ", ") + "]"
	case "tuple":
		return "(" + strings.Join(elems, ", ") + ")"
	}
	return kind
}

// RunMembers runs every case of the member table on read, which returns the canonical result of a read
// of a target, and returns the cases whose result differs.
func RunMembers(target string, read func(value, member string) string) error {
	var bad []string
	for _, c := range MemberCases {
		if got := read(c.Value, c.Member); got != c.Want {
			bad = append(bad, fmt.Sprintf("%s: (%s).%s = %s, want %s", target, c.Value, c.Member, got, c.Want))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("member table:\n%s", strings.Join(bad, "\n"))
	}
	return nil
}
