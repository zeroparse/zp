package libgogen

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/core"
	"zeroparse.dev/zp/internal/ir/exec/spectest"
)

// specDir is the directory of the Exec IR specification, relative to this package.
const specDir = "../../../../spec/exec-ir"

// specializations returns what the Go library and the engine print: for each of the two in
// the table of specializations, and each composition, the feature sets of its specializations.
func specializations() map[string]map[string][]string {
	features := core.Build().Features
	out := map[string]map[string][]string{}
	const nodes, typed, engine = "Generated Go, nodes", "Generated Go, typed values", "The engine"
	for _, list := range [][]goSpec{goSpecs, goStreamSpecs, goOperatorSpecs, goTermSpecs} {
		for _, s := range list {
			if s.typed {
				spectest.AddSpecialization(out, features, typed, s.fn, s.on)
			} else {
				spectest.AddSpecialization(out, features, nodes, s.fn, s.on)
			}
		}
	}
	for _, list := range [][]goSpec{engineSpecs, engineOperatorSpecs, engineStreamSpecs, engineDocumentSpecs} {
		for _, s := range list {
			spectest.AddSpecialization(out, features, engine, s.fn, s.on)
		}
	}
	for _, f := range termFuncs {
		spectest.AddSpecialization(out, features, engine, f.fn, nil)
	}
	for _, fn := range engineBackendFuncs {
		spectest.AddSpecialization(out, features, engine, fn, nil)
	}
	return out
}

// TestSpecSpecializations checks the Go and engine columns of the table of specializations against
// what the Go library and the engine print.
func TestSpecSpecializations(t *testing.T) {
	for _, e := range spectest.SpecializationErrors(specializations(), spectest.Read(t, specDir)) {
		t.Error(e)
	}
}

// TestSpecSpecializationsNegative is the permanent negative control of TestSpecSpecializations.
func TestSpecSpecializationsNegative(t *testing.T) {
	want := specializations()
	want["The engine"]["grow"] = []string{"[typed]"}
	want["Generated Go, nodes"]["plantedFunc"] = []string{"[budget]"}
	errs := strings.Join(spectest.SpecializationErrors(want, spectest.Read(t, specDir)), "\n")
	for _, w := range []string{"Generated Go, nodes: specialization plantedFunc: in the module", "The engine: specialization grow: the specification has"} {
		if !strings.Contains(errs, w) {
			t.Errorf("no error %q in:\n%s", w, errs)
		}
	}
}
