package conform

import (
	"fmt"
	"strings"
)

// The hull table: tuple values and list values that a term makes of elements of one fixture (the composition
// hull over the elements), and the range each takes. Each target builds the fixture and makes the values
// through its printed composition; the tests that do are the ones the mutants of that code name. The fixture
// and the rule are those of the struct table (StructLate, StructRuleStart and StructRuleEnd).

// HullCase makes a value of the elements Elems, in order, which takes the range Want: the start and the end,
// a dash between them.
type HullCase struct {
	Elems []string
	Want  string
}

// HullCases are the values of the hull table: none and many elements; those that add no range (nil), those
// that add the range of the rule (an int and a string, which a target whose elements are nodes boxes in a
// node of that range), and the nodes in each order, alone and among the others, so that a start that is later
// and an end that is earlier than the one of another element leave the range as it is.
var HullCases = []HullCase{
	{nil, "6-6"},
	{[]string{MemberNil}, "6-6"},
	{[]string{MemberNil, MemberNil}, "6-6"},
	{[]string{MemberInt}, "6-8"},
	{[]string{BuiltinString}, "6-8"},
	{[]string{MemberNil, MemberInt, BuiltinString}, "6-8"},
	{[]string{BuiltinText}, "2-4"},
	{[]string{BuiltinNode}, "2-5"},
	{[]string{StructLate}, "7-9"},
	{[]string{MemberNil, StructLate, MemberNil}, "7-9"},
	{[]string{BuiltinText, BuiltinNode}, "2-5"},
	{[]string{BuiltinNode, BuiltinText}, "2-5"},
	{[]string{StructLate, BuiltinText}, "2-9"},
	{[]string{BuiltinText, StructLate}, "2-9"},
	{[]string{StructLate, BuiltinNode}, "2-9"},
	{[]string{BuiltinNode, StructLate}, "2-9"},
	{[]string{MemberInt, BuiltinText}, "2-8"},
	{[]string{BuiltinText, MemberInt}, "2-8"},
	{[]string{MemberInt, StructLate}, "6-9"},
	{[]string{StructLate, MemberInt}, "6-9"},
	{[]string{BuiltinNode, StructLate, MemberInt, MemberNil}, "2-9"},
}

// HullKinds are the kinds of values that a target makes of the elements of each case: a tuple and a list.
var HullKinds = []string{"tuple", "list"}

// RunHulls runs every case of the hull table on make, which makes a value of the kind kind of the elements of
// a case in a target and returns its range, as the want of the case writes it, and the units that the
// composition charged when it ran again (every element costs one step, whatever it adds, besides what the
// making of the value costs, which the case with no element shows); a
// target that charges none passes controlled false, and its steps are not checked. It returns the cases whose
// result differs.
func RunHulls(target string, controlled bool, make func(kind string, elems []string) (string, int)) error {
	var bad []string
	for _, kind := range HullKinds {
		_, made := make(kind, nil)
		for _, c := range HullCases {
			got, steps := make(kind, c.Elems)
			if got != c.Want || (controlled && steps != made+len(c.Elems)) {
				bad = append(bad, fmt.Sprintf("%s: %s(%s) = %s with %d steps, want %s with %d", target, kind, strings.Join(c.Elems, ", "), got, steps, c.Want, made+len(c.Elems)))
			}
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("hull table:\n%s", strings.Join(bad, "\n"))
	}
	return nil
}
