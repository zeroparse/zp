package compile

import (
	"strings"
	"testing"
)

// TestCutLeftCalls checks that, in a cycle with more than one leader, a #cut passed after a left
// call and a lookahead that can make a left call are not classified, and that the same shapes
// with the cut before the call, in a cycle with one leader, or without the left call are.
func TestCutLeftCalls(t *testing.T) {
	t.Parallel()
	const head = "@start\nrule main = S $$\n"
	cases := []struct {
		name, src string
		class     ComplexityClass
		reason    string // substring of a reason, or "" for none of the two cut-left reasons
	}{
		{"s4w3 cut after a left call", `rule S = S "w" / R "z"
rule R = (R #cut "b" / S "y" / "c") / S "x"`, ClassUnknown, "passed after a left call"},
		{"s4w4 negative lookahead calling R", `rule S = S "w" / R "z"
rule R = R "b" / S "y" / !(!"q" R) "c" / S "x"`, ClassUnknown, "can make a left call"},
		{"s4w5 positive lookahead calling R", `rule S = S "w" / R "z"
rule R = R "b" / S "y" / &(!"q" R) S "x" / "c"`, ClassUnknown, "can make a left call"},
		{"s4d cut after a left call", `rule S = S "w" / R "z" / "c" Bs
rule Bs = "b" Bs / ""
rule R = (R #cut "b" / S "y" / "c") / S "x"`, ClassUnknown, "passed after a left call"},
		{"control: cut before the left call", `rule S = S "w" / R "z"
rule R = (#cut R "b" / S "y" / "c") / S "x"`, ClassLinear, ""},
		{"control: one leader in the cycle", `rule S = S "w" / "c" R "z"
rule R = (R #cut "b" / "c") / "x"`, ClassLinear, ""},
		{"control: lookahead without a left call", `rule S = S "w" / R "z"
rule R = R "b" / S "y" / !(!"q" "d") "c" / S "x"`, ClassUnknown, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := classifySource(t, head+tc.src)
			if c.Class != tc.class {
				t.Fatalf("class %v, want %v\nreasons %v", c.Class, tc.class, c.Reasons)
			}
			var got []string
			for _, r := range c.Reasons {
				s := r.Message
				if strings.Contains(s, "passed after a left call") || strings.Contains(s, "can make a left call") {
					got = append(got, s)
				}
			}
			if tc.reason == "" && len(got) > 0 {
				t.Errorf("unexpected reasons %v", got)
			}
			if tc.reason != "" && (len(got) == 0 || !strings.Contains(got[0], tc.reason)) {
				t.Errorf("reasons %v do not contain %q", got, tc.reason)
			}
		})
	}
}
