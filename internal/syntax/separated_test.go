package syntax

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
)

// TestSeparatedFormat checks that the formatter and the JSON form keep the #sep pragma.
func TestSeparatedFormat(t *testing.T) {
	for _, src := range []string{
		"rule main = num* #sep(\",\") $$\nrule num = [0-9]+\n",
		"rule main = num+ #sep(\",\", trailing=\"optional\") $$\nrule num = [0-9]+\n",
		"rule main = num* #sep(comma, trailing=\"required\") $$\nrule comma = \" \"* \",\" \" \"*\nrule num = [0-9]+\n",
	} {
		out := checkFormat(t, src)
		if !strings.Contains(out, "#sep(") {
			t.Errorf("formatted source lost the pragma:\n%s", out)
		}
		g, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		data, err := grammar.MarshalJSON(g)
		if err != nil {
			t.Fatal(err)
		}
		back, err := grammar.UnmarshalJSON(data)
		if err != nil {
			t.Fatal(err)
		}
		again, err := grammar.MarshalJSON(back)
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(data) || !strings.Contains(string(data), `"sep"`) {
			t.Errorf("JSON round trip changed the grammar:\n%s", data)
		}
	}
}
