package conform

import (
	"fmt"
	"strings"
)

// The push table: the compositions push and pushItems, which make the elements of a list or a tuple and push
// them on the stack that gathers them, on the values of the fixture of the built-in table, and what they push
// in the canonical form of MemberCanon. Each target builds the fixture and calls its printed compositions; the
// tests that do are the ones the mutants of that code name.

// The calls of the push table, by the name a case calls.
const (
	// CallPush is the composition push for the built-in function list, which pushes x as an element: the
	// canonical form of a box is "box " and that of the value it holds.
	CallPush = "push"
	// CallPushItems is the composition pushItems for the built-in function concat, which pushes the elements of
	// x; the canonical form is that of a list of the elements it pushed.
	CallPushItems = "pushItems"
)

// PushCase is a call Call of the value Value of the fixture, which pushes Pushed elements and returns Want: the
// canonical form of the element that CallPush pushed or of the list of those CallPushItems pushed, or "error: "
// and the text of the failure, which pushes none. A Want that ends with "..." is a prefix of a longer result.
// Units is what a target that bounds the work charges for the call, or -1 where the table does not check it:
// pushItems charges one unit for each element it pushes.
type PushCase struct {
	Call, Value, Want string
	Pushed, Units     int
}

// PushCases are the calls of the push table.
var PushCases = []PushCase{
	{CallPush, MemberNil, "nil", 1, 0},
	{CallPush, MemberInt, "box 1", 1, -1},
	{CallPush, BuiltinString, `box "ab"`, 1, -1},
	{CallPush, BuiltinBool, "box true", 1, -1},
	{CallPush, BuiltinPair, "[1, 2]", 1, 0},
	{CallPush, BuiltinFunc, "error: list: elements must be values, got ...", 0, -1},
	{CallPushItems, MemberNil, "[]", 0, 0},
	{CallPushItems, MemberList, "[]", 0, 0},
	{CallPushItems, BuiltinPair, "[1, 2]", 2, 2},
	{CallPushItems, MemberInt, "error: concat: expected a list, got int", 0, -1},
	{CallPushItems, BuiltinString, "error: concat: expected a list, got string", 0, -1},
}

// PushFn is the name of the built-in function that a call of the push table passes to the composition, as the
// text of its failure names it.
func PushFn(call string) string {
	if call == CallPushItems {
		return "concat"
	}
	return "list"
}

// RunPushes runs every case of the push table on call, which returns the canonical result of a call of a target,
// the number of elements it pushed and the units it charged; a target that charges none passes controlled false,
// and its units are not checked. It returns the cases whose result differs.
func RunPushes(target string, controlled bool, call func(call, value string) (string, int, int)) error {
	var bad []string
	for _, c := range PushCases {
		got, pushed, units := call(c.Call, c.Value)
		ok := got == c.Want
		if prefix, cut := strings.CutSuffix(c.Want, "..."); cut {
			ok = strings.HasPrefix(got, prefix) && len(got) > len(prefix)
		}
		if !ok || pushed != c.Pushed || (controlled && c.Units >= 0 && units != c.Units) {
			bad = append(bad, fmt.Sprintf("%s: %s(%s) = %s pushing %d with %d units, want %s pushing %d with %d",
				target, c.Call, c.Value, got, pushed, units, c.Want, c.Pushed, c.Units))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("push table:\n%s", strings.Join(bad, "\n"))
	}
	return nil
}
