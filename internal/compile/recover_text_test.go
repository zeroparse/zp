package compile_test

import (
	"errors"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// An error value that #recover produces is a node with a range and no value, so text(x) accepts a value
// that can be one and gives the input that the recovery skipped, and len(x) treats it as nil.
func TestTextOfErrorValues(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ src, in, want string }{
		{"@start\nrule main = x:(\"a\" #recover(skip=\"bb\")) => text($x)\n", "bb", `"bb"`},
		{"@start\nrule main = x:(\"a\" #recover(skip=\"bb\")) {text($x) == \"bb\"} \"c\" => 1\n", "bbc", "1"},
		{"@start\nrule main = x:(\"a\" #recover(skip=\"bb\")) => len(text($x))\n", "bb", "2"},
		{"@start\nrule main = x:(\"a\" #recover(skip=\"bb\")) => len($x)\n", "bb", "0"},
		{"@start\nrule main = x:(\"a\" #recover(skip=\"bb\")) => $x.Value == nil\n", "bb", "true"},
	} {
		p, err := engine.CompileSource(c.src, "main")
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		n, err := p.Parse(c.in)
		var recovered value.SyntaxErrors
		if !errors.As(err, &recovered) {
			t.Errorf("%q on %q: error %v, want the recovered syntax error", c.src, c.in, err)
			continue
		}
		if got := n.String(); got != c.want {
			t.Errorf("%q on %q = %s, want %s", c.src, c.in, got, c.want)
		}
	}
}
