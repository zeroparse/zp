package core

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
)

// TestCompositionsDocumented checks that every composition has a doc and logs its Exec IR.
func TestCompositionsDocumented(t *testing.T) {
	for _, f := range Build().Funcs {
		if f.Doc == "" {
			t.Errorf("composition %s has no doc", f.Name)
		}
		t.Log(exec.FuncString(f))
	}
}
