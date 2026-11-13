package gogen_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedRecoverBottom checks that a generated Go parser gives the recovery of an expression
// of type _|_ an error value, and an expression without a value none: each parse succeeds or fails
// as codegentest.RecoverBottomCases states, and agrees with the engine.
func TestGeneratedRecoverBottom(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cases := codegentest.RecoverBottomGenCases()
	codegentest.EachMode(t, func(t *testing.T) {
		got, want := codegentest.GeneratedParsersOutputs(t, goBin, cases, codegen.GenOptions{}, nil)
		k := 0
		for _, c := range codegentest.RecoverBottomCases {
			for i, in := range c.Inputs {
				for _, unit := range []string{"codepoints", "bytes"} {
					head, _, _ := strings.Cut(got[k], " views: ")
					var r struct{ Node json.RawMessage }
					if err := json.Unmarshal([]byte(head), &r); err != nil {
						t.Fatalf("%s: %v", got[k], err)
					}
					if accepted := string(r.Node) != "null"; accepted != c.Accept[i] {
						t.Errorf("%s (%s), %q: accepted %v, want %v\n%s", c.Name, unit, in, accepted, c.Accept[i], got[k])
					}
					if got[k] != want[k] {
						t.Errorf("%s (%s), %q:\n generated %s\n engine    %s", c.Name, unit, in, got[k], want[k])
					}
					k++
				}
			}
		}
		codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{})
	})
}
