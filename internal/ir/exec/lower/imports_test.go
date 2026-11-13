package lower_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestLoweringDoesNotDependOnTheCompiler checks the direction of the layers: the lowering of Flow IR
// reads the Core IR and the Exec IR, and neither it nor anything it imports depends on the compiler
// (the Core IR carries what the lowering needs from the grammar).
func TestLoweringDoesNotDependOnTheCompiler(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, p := range strings.Fields(string(out)) {
		if strings.HasSuffix(p, "/internal/compile") {
			t.Errorf("the lowering depends on %s", p)
		}
	}
}
