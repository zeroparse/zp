package conform

import (
	"fmt"
	"strings"
)

// The struct table: structs that a term makes of field values of one fixture (the composition cover over
// each field value and the composition made over the struct), and the range each takes and whether the
// context records it. Each target builds the fixture and makes the structs through its printed
// compositions; the tests that do are the ones the mutants of that code name.

// StructLate is a string node with the range 7 to 9, besides the values of the built-in fixture
// (MemberNil, MemberInt, BuiltinString, BuiltinText with the range 2 to 4 and BuiltinNode with the range
// 2 to 5).
const StructLate = "late"

// StructRuleStart and StructRuleEnd are the range of the rule in whose context the structs are made,
// which no value of the fixture has.
const (
	StructRuleStart = 6
	StructRuleEnd   = 8
)

// StructCase makes a struct whose field values are Fields, in order, and which takes the range Want: the
// start and the end, a dash between them.
type StructCase struct {
	Fields []string
	Want   string
}

// StructCases are the structs of the struct table: none and many field values, those that are not nodes,
// nil among them, which leave the range of the rule, and the nodes in each order, so that a start that is
// later and an end that is earlier than the one of another field leave the range as it is.
var StructCases = []StructCase{
	{nil, "6-8"},
	{[]string{MemberInt, BuiltinString}, "6-8"},
	{[]string{MemberNil}, "6-8"},
	{[]string{BuiltinText}, "2-4"},
	{[]string{BuiltinNode}, "2-5"},
	{[]string{StructLate}, "7-9"},
	{[]string{MemberInt, StructLate}, "7-9"},
	{[]string{StructLate, MemberNil, MemberInt}, "7-9"},
	{[]string{BuiltinText, BuiltinNode}, "2-5"},
	{[]string{BuiltinNode, BuiltinText}, "2-5"},
	{[]string{StructLate, BuiltinText}, "2-9"},
	{[]string{BuiltinText, StructLate}, "2-9"},
	{[]string{StructLate, BuiltinNode}, "2-9"},
	{[]string{BuiltinNode, StructLate}, "2-9"},
	{[]string{BuiltinText, BuiltinNode, StructLate, MemberNil}, "2-9"},
}

// StructRecorded is what a target appends to the range of a struct that its context recorded as made.
const StructRecorded = " made"

// RunStructs runs every case of the struct table on make, which makes the struct of the field values of a
// case in a target and returns its range, as the want of the case writes it, and StructRecorded if the
// context recorded the struct. It returns the cases whose result differs.
func RunStructs(target string, make func(fields []string) string) error {
	var bad []string
	for _, c := range StructCases {
		if got := make(c.Fields); got != c.Want+StructRecorded {
			bad = append(bad, fmt.Sprintf("%s: struct(%s) = %s, want %s", target, strings.Join(c.Fields, ", "), got, c.Want+StructRecorded))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("struct table:\n%s", strings.Join(bad, "\n"))
	}
	return nil
}
