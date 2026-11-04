package compile

import (
	"strings"
	"testing"
)

// TestSeparatedRejected checks that #sep reports the grammars it cannot make sound.
func TestSeparatedRejected(t *testing.T) {
	const nums = "\nrule num = [0-9]+\n"
	for _, c := range []struct {
		name, src, want string
	}{
		{"not a repetition", `rule main = num #sep(",") $$` + nums, "repetition"},
		{"zero maximum", `rule main = num #repeat(0, 0) #sep(",") $$` + nums, "#sep"},
		{"duplicate", `rule main = num* #sep(",") #sep(";") $$` + nums, "#sep"},
		{"missing separator", `rule main = num* #sep() $$` + nums, "requires a separator"},
		{"empty separator", `rule main = num* #sep("") $$` + nums, "must not be empty"},
		{"capture in separator", `rule main = num* #sep(x:",") $$` + nums, "must not contain captures"},
		{"unknown argument", `rule main = num* #sep(",", leading="optional") $$` + nums, "leading"},
		{"bad policy", `rule main = num* #sep(",", trailing="maybe") $$` + nums, "trailing"},
		{"nullable element", `rule main = ([0-9]*)* #sep(",") $$`, "empty input"},
		{"nullable separator", `rule main = num* #sep(" "*) $$` + nums, "empty input"},
		{"element starts with the separator", `rule main = num* #sep("1") $$` + nums, "starts the separator"},
		{"separator rule overlap", `rule main = (num / ",")* #sep(",") $$` + nums, "starts the separator"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := compileError(t, c.src); !strings.Contains(got, c.want) {
				t.Errorf("error %q lacks %q", got, c.want)
			}
		})
	}
}
