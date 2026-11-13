package gents

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/core"
	"zeroparse.dev/zp/internal/ir/exec/spectest"
)

// execSpecDir is the directory of the Exec IR specification, relative to this package.
const execSpecDir = "../../../spec/exec-ir"

// execSpecializations returns what the TypeScript runtime prints: for each composition, the feature
// sets of its specializations, under the TypeScript column of the table of specializations.
func execSpecializations() map[string]map[string][]string {
	features := core.Build().Features
	out := map[string]map[string][]string{}
	for _, s := range tsSpecs {
		spectest.AddSpecialization(out, features, "TypeScript", s.fn, s.on)
	}
	return out
}

// TestExecSpecSpecializations checks the TypeScript column of the Exec IR table of specializations
// against what the TypeScript runtime prints.
func TestExecSpecSpecializations(t *testing.T) {
	for _, e := range spectest.SpecializationErrors(execSpecializations(), spectest.Read(t, execSpecDir)) {
		t.Error(e)
	}
}

// TestExecSpecSpecializationsNegative is the permanent negative control of
// TestExecSpecSpecializations.
func TestExecSpecSpecializationsNegative(t *testing.T) {
	want := execSpecializations()
	want["TypeScript"]["plantedFunc"] = []string{"[budget]"}
	errs := strings.Join(spectest.SpecializationErrors(want, spectest.Read(t, execSpecDir)), "\n")
	if w := "TypeScript: specialization plantedFunc: in the module"; !strings.Contains(errs, w) {
		t.Errorf("no error %q in:\n%s", w, errs)
	}
}
