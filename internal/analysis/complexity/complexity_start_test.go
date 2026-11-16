package complexity

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/syntax"
)

// TestClassifyComplexityStartRule checks that an empty start name selects the default start rule
// and that a grammar without one is an error, not an empty reach classified as linear.
func TestClassifyComplexityStartRule(t *testing.T) {
	parse := func(t *testing.T, src string) *Complexity {
		t.Helper()
		g, err := syntax.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		c, err := ClassifyComplexityControlled(g, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	t.Run("no start rule", func(t *testing.T) {
		g, err := syntax.Parse("rule main = \"a\"\n")
		if err != nil {
			t.Fatal(err)
		}
		if c, err := ClassifyComplexityControlled(g, "", nil); err == nil || c != nil || !strings.Contains(err.Error(), "no start rule") {
			t.Errorf("ClassifyComplexity = %v, %v, want an error that names the missing start rule", c, err)
		}
		if b, err := GrowingBindingsControlled(g, "", nil); err == nil || b != nil {
			t.Errorf("GrowingBindings = %v, %v, want an error", b, err)
		}
	})
	t.Run("default start rule", func(t *testing.T) {
		const body = "rule main = (a / \"a\")* $$\nrule a = \"a\"* \"c\"\n"
		marked := parse(t, "@start\n"+body)
		g, err := syntax.Parse("@start\n" + body)
		if err != nil {
			t.Fatal(err)
		}
		named, err := ClassifyComplexityControlled(g, "main", nil)
		if err != nil {
			t.Fatal(err)
		}
		if marked.Class != named.Class || marked.Degree != named.Degree {
			t.Errorf("start \"\" gives %v degree %d, start \"main\" gives %v degree %d", marked.Class, marked.Degree, named.Class, named.Degree)
		}
	})
	// A start name is accepted or rejected as the entry to a parse of the compiled grammar
	// rejects or accepts it, with the same message.
	t.Run("agrees with parsing", func(t *testing.T) {
		const src = "@start(default)\nrule main = \"a\"\n@start\nrule other = \"b\"\nrule inner = \"c\"\nrule _hidden = \"d\"\n"
		g, err := syntax.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, err := compile.Check(g, compile.Options{}, nil, desugar.InlineNames{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"", "main", "other", "inner", "_hidden", "nosuch"} {
			want := prog.CheckStart(name)
			_, got := ClassifyComplexityControlled(g, name, nil)
			if (want == nil) != (got == nil) || want != nil && want.Error() != got.Error() {
				t.Errorf("start %q: parsing gives %v, classification gives %v", name, want, got)
			}
			_, got = GrowingBindingsControlled(g, name, nil)
			if (want == nil) != (got == nil) || want != nil && want.Error() != got.Error() {
				t.Errorf("start %q: parsing gives %v, GrowingBindings gives %v", name, want, got)
			}
		}
	})
}
