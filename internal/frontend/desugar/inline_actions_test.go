package desugar

import "testing"

// The messages describe an inline action by the rule that holds it.
func TestInlineActionNamesInMessages(t *testing.T) {
	t.Parallel()
	names := InlineNames{"_main_1": {rule: "main", n: 1}, "_main_10": {rule: "main", n: 10}}
	for _, c := range []struct{ in, want string }{
		{"rule _main_1 produces int", "the inline action in rule main produces int"},
		{"result of rule _main_10: _main_1", "result of the inline action in rule main: the inline action in rule main"},
		{"_main_1_ is not _main_1", "_main_1_ is not the inline action in rule main"},
	} {
		if got := names.Text(c.in); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
	if got, ok := names.describe("main"); ok {
		t.Errorf("describes a declared rule: %q", got)
	}
}
