package conform

import (
	"fmt"
	"strings"
)

// The iteration table: the built-in functions foldl, foldr, map and filter over values of the fixture of the
// built-in table, each with a function of the table's own, and what each returns in the canonical form of
// MemberCanon. They walk the list through the compositions itemAt and filterKeep. Each target builds the
// fixture and calls the methods that a generated parser or a backend calls; the tests that do are the ones
// the mutants of that code name.

// The calls of the iteration table, by the name a case calls.
const (
	// CallFoldl is foldl(nil, x, f) where f returns the element it is passed: the last element that foldl
	// passes, and nil if it passes none.
	CallFoldl = "foldl"
	// CallFoldr is CallFoldl with foldr, which passes the first element last.
	CallFoldr = "foldr"
	// CallMap is map(x, f) where f returns the element it is passed.
	CallMap = "map"
	// CallFilter is filter(x, f) where f returns whether the element it is passed is the int 1.
	CallFilter = "filter"
	// CallFilterSelf is filter(x, f) where f returns the element it is passed, which is not a bool.
	CallFilterSelf = "filterSelf"
)

// IterCase is a call Call of the value Value of the fixture, which returns Want: a value in the form of
// MemberCanon, or "error: " and the text of the failure. Steps is the units that a target that bounds the work
// charges for the call, or -1 where the table does not check them: a fold charges one step for each element it
// passes, and the function of the table charges none.
type IterCase struct {
	Call, Value, Want string
	Steps             int
}

// IterCases are the calls of the iteration table. The list BuiltinPair holds the ints 1 and 2, which a target
// whose elements are nodes holds in boxes: the function is passed the ints themselves.
var IterCases = []IterCase{
	{CallFoldl, BuiltinPair, "2", 2},
	{CallFoldr, BuiltinPair, "1", 2},
	{CallFoldl, MemberNil, "nil", 0},
	{CallFoldr, MemberList, "nil", 0},
	{CallFoldl, MemberInt, "error: foldl: expected a list, got int", -1},
	{CallFoldr, BuiltinString, "error: foldr: expected a list, got string", -1},
	{CallMap, BuiltinPair, "[1, 2]", -1},
	{CallMap, MemberNil, "[]", -1},
	{CallMap, MemberInt, "error: map: expected a list, got int", -1},
	{CallFilter, BuiltinPair, "[1]", -1},
	{CallFilter, MemberList, "[]", -1},
	{CallFilter, BuiltinBool, "error: filter: expected a list, got bool", -1},
	{CallFilterSelf, BuiltinPair, "error: filter: predicate must produce a bool, got int", -1},
}

// RunIters runs every case of the iteration table on call, which returns the canonical result of a call of a
// target and the units it charged; a target that charges none passes controlled false, and its units are not
// checked. It returns the cases whose result differs.
func RunIters(target string, controlled bool, call func(call, value string) (string, int)) error {
	var bad []string
	for _, c := range IterCases {
		got, steps := call(c.Call, c.Value)
		if got != c.Want || (controlled && c.Steps >= 0 && steps != c.Steps) {
			bad = append(bad, fmt.Sprintf("%s: %s(%s) = %s with %d steps, want %s with %d", target, c.Call, c.Value, got, steps, c.Want, c.Steps))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("iteration table:\n%s", strings.Join(bad, "\n"))
	}
	return nil
}
