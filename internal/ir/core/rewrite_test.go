package core

import "testing"

// TestRewriteReachesMissingBody checks that Rewrite visits the body of a #missing insertion, which
// can hold attributes such as @error around the literal, and installs what f returns for it.
func TestRewriteReachesMissingBody(t *testing.T) {
	lit := &Lit{Text: ";"}
	label := &Label{Body: lit}
	m := &Missing{Lit: lit, Body: label, Site: 1}
	repl := &Lit{Text: ";"}
	var seen []Node
	got := Rewrite(m, func(n Node) Node {
		seen = append(seen, n)
		if n == lit {
			return repl
		}
		return n
	})
	if got != m || len(seen) != 3 || seen[0] != lit || seen[1] != label || seen[2] != m {
		t.Fatalf("Rewrite visited %v, returned %v; want the literal, the label and the insertion", seen, got)
	}
	if m.Body != label || label.Body != repl {
		t.Fatalf("the rewritten body was not installed: %#v", m.Body)
	}
}
