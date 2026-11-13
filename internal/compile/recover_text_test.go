package compile_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/engine"
)

// An error value that #recover produces carries a range and a message, not matched text, so
// text(x) rejects a value that can be one at compile time: an action or predicate never reads the
// text of an error node.
func TestTextRejectsErrorValues(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"rule main = x:(\"a\" #recover(skip=\"b\")) => text($x)\n",
		"rule main = x:(\"a\" #recover(skip=\"b\")) {text($x) == \"b\"} \"c\"\n",
		"rule main = x:(\"a\" #recover(skip=\"b\")) => len(text($x))\n",
	} {
		_, err := engine.CompileSource(src, "main")
		if err == nil || !strings.Contains(err.Error(), "text: invalid argument") {
			t.Errorf("%q: error %v, want text: invalid argument", src, err)
		}
	}
}
