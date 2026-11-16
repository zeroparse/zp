package complexity

import (
	"strings"
	"testing"
)

// TestCutLeftCalls checks that, in a cycle with more than one leader, a #cut passed after a left
// call and a lookahead that can make a left call are not classified, and that the same shapes
// with the cut before the call, after input is consumed, in a cycle with one leader, or without
// the left call are.
func TestCutLeftCalls(t *testing.T) {
	t.Parallel()
	const head = "@start\nrule main = S $$\n"
	cases := []struct {
		name, src string
		class     ComplexityClass
		// reason is a substring of the first cut-left reason, or "" for none of the two
		// cut-left reasons.
		reason string
		// only says that the cut-left reasons are the only reasons, so that the class is unknown
		// because of them: without them the grammar would be linear.
		only bool
	}{
		{"s4w3 cut after a left call", `rule S = S "w" / R "z"
rule R = (R #cut "b" / S "y" / "c") / S "x"`, ClassUnknown, "passed after a left call", false},
		{"s4w4 negative lookahead calling R", `rule S = S "w" / R "z"
rule R = R "b" / S "y" / !(!"q" R) "c" / S "x"`, ClassUnknown, "can make a left call", false},
		{"s4w5 positive lookahead calling R", `rule S = S "w" / R "z"
rule R = R "b" / S "y" / &(!"q" R) S "x" / "c"`, ClassUnknown, "can make a left call", false},
		{"s4d cut after a left call", `rule S = S "w" / R "z" / "c" Bs
rule Bs = "b" Bs / ""
rule R = (R #cut "b" / S "y" / "c") / S "x"`, ClassUnknown, "passed after a left call", false},
		// The cut in a recovery commits the choice around it. On "c" q^n (zy)^n the cost grows
		// as n^2; without the #cut the grammar is linear.
		{"cut in the base of a recovery", `rule S = S "w" / R "z"
rule R = (R (#cut "b") #recover(skip="q") / "c") / S "y"`, ClassUnknown, "passed after a left call", true},
		{"cut in the skip of a recovery", `rule S = S "w" / R "z"
rule R = (R ("b") #recover(skip=#cut "q") / "c") / S "y"`, ClassUnknown, "passed after a left call", true},
		// The recovery attribute binds to the last item, so the group makes the left call part
		// of the recovery's own base: the skip must see it.
		{"cut in the skip of a recovery whose base makes the left call", `rule S = S "w" / R "z"
rule R = ((R "b") #recover(skip=#cut "q") / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		// A repetition that may match nothing leaves the next item at the start, so a call after
		// it is a left call.
		{"cut after a left call following a repetition", `rule S = S "w" / R "z"
rule R = ("x"* R #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		{"cut after a left call following a repetition in a capture", `rule S = S "w" / R "z"
rule R = (v:("x"* R) #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		{"cut after a left call in one choice", `rule S = S "w" / R "z"
rule R = (R #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", true},
		{"lookahead in the base of a recovery", `rule S = S "w" / R "z"
rule R = R "b" / S "y" / (!(!"q" R) "c") #recover(skip="q") / S "x"`, ClassUnknown, "can make a left call", false},
		// Left calls nested in captures, discards and #atomic after parts that consume no
		// input.
		{"left call after a lookahead in #atomic", `rule S = S "w" / R "z"
rule R = ((&"c" R) #atomic #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		{"left call after a lookahead in a capture", `rule S = S "w" / R "z"
rule R = (v:(&"c" R) #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		{"left call after an option in #atomic", `rule S = S "w" / R "z"
rule R = (("w"? R) #atomic #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		{"left call after an option in a capture", `rule S = S "w" / R "z"
rule R = (v:("w"? R) #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		{"left call after a nullable rule in a capture", `rule S = S "w" / R "z"
rule R = (v:(Q R) #cut "b" / "c") / S "y"
rule Q = "w"?`, ClassUnknown, "passed after a left call", false},
		{"left call after an option in a discard", `rule S = S "w" / R "z"
rule R = (-("w"? R) #cut "b" / "c") / S "y"`, ClassUnknown, "passed after a left call", false},
		{"control: cut before the left call", `rule S = S "w" / R "z"
rule R = (#cut R "b" / "c") / "d" / S "y"`, ClassLinear, "", false},
		{"control: cut after a call that is not a left call", `rule S = S "w" / "(" S #cut ")" / R "z"
rule R = (R "b" / "c") / S "y"`, ClassLinear, "", false},
		{"control: lookahead with a call that is not a left call", `rule S = S "w" / "(" &S S ")" / R "z"
rule R = (R "b" / "c") / S "y"`, ClassLinear, "", false},
		{"control: one leader in the cycle", `rule S = S "w" / "c" R "z"
rule R = (R #cut "b" / "c") / "x"`, ClassLinear, "", false},
		{"control: lookahead without a left call", `rule S = S "w" / R "z"
rule R = R "b" / S "y" / !(!"q" "d") "c" / S "x"`, ClassUnknown, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := classifySource(t, head+tc.src)
			if c.Class != tc.class {
				t.Fatalf("class %v, want %v\nreasons %v", c.Class, tc.class, c.Reasons)
			}
			var got, other []string
			for _, r := range c.Reasons {
				s := r.Message
				if strings.Contains(s, "passed after a left call") || strings.Contains(s, "can make a left call") {
					got = append(got, s)
				} else {
					other = append(other, s)
				}
			}
			if tc.reason == "" && len(got) > 0 {
				t.Errorf("unexpected reasons %v", got)
			}
			if tc.reason != "" && (len(got) == 0 || !strings.Contains(got[0], tc.reason)) {
				t.Errorf("reasons %v do not contain %q", got, tc.reason)
			}
			if tc.only && len(other) > 0 {
				t.Errorf("reasons other than the cut-left ones: %v", other)
			}
		})
	}
}
