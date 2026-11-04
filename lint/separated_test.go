package lint

import (
	"context"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// TestSeparatedPragmaDiagnostics checks that Run reports a #sep the compiler rejects, with the
// position of the pragma, and accepts a sound list without findings about the separator.
func TestSeparatedPragmaDiagnostics(t *testing.T) {
	const nums = "\nrule num = [0-9]+\n"
	for _, c := range []struct{ src, want string }{
		{`rule main = num #sep(",") $$` + nums, "repetition"},
		{`rule main = num* #sep("1") $$` + nums, "starts the separator"},
		{`rule main = num* #sep(",", trailing="maybe") $$` + nums, "trailing"},
	} {
		g, err := syntax.Parse(c.src)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Config{}.Run(context.Background(), g, "")
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.HasPrefix(err.Error(), "1:") {
			t.Errorf("%s: error %v, want position and %q", c.src, err, c.want)
		}
	}
	g, err := syntax.Parse(`rule main = num* #sep(",", trailing="optional") $$` + nums)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := Config{}.Run(context.Background(), starttest.MarkAll(g), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("unexpected finding %v", f)
	}
}
