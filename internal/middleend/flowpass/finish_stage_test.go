package flowpass

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/pass"
)

// TestFinishingStep checks Finish as a step of the pipeline: one required step that keeps the Flow
// stage and is not dumped.
func TestFinishingStep(t *testing.T) {
	want := []pass.Step{{Name: FinishName, Required: true, In: "flow", Out: "flow"}}
	if got := Finishing.Steps(); !slices.Equal(got, want) {
		t.Errorf("steps = %+v, want %+v", got, want)
	}
}
