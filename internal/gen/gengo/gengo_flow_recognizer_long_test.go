//go:build zp_long

package gengo_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestGeneratedFlowRecognizerNegativeControl checks that the parity test rejects each wrong recognizer
// of flowRecognizerMutants: the mutant child must fail with a recognition mismatch.
func TestGeneratedFlowRecognizerNegativeControl(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	for name := range flowRecognizerMutants {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(os.Args[0], "-test.run=^TestGeneratedFlowRecognizerMutantChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), "ZP_FLOW_MUTANT="+name)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the %s mutant passed the parity test:\n%s", name, out)
			}
			if !strings.Contains(string(out), "--- FAIL: TestGeneratedFlowRecognizerMutantChild") {
				t.Fatalf("the %s mutant child did not fail its test:\n%s", name, out)
			}
			// The rejection must come from the recognition comparison, not from a failure to build.
			if !strings.Contains(string(out), "recognize: ") {
				t.Fatalf("the %s mutant child failed without a recognition mismatch:\n%s", name, out)
			}
		})
	}
}
