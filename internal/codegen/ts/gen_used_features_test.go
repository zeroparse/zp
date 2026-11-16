package tsgen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestUsedFeaturesTSParity compares parsers that carry their own copy of the library, narrowed to
// their features, with the engine on codegentest.UsedFeatureCases, plain, with limit controls,
// with types and with Documents, and does the same with the optimization used-features off, whose
// copy is the whole library. Each narrowed copy is type-checked with the strictest options (when
// tsc is found), so this also checks that narrowing leaves no dangling name and no unused local.
// The default tier runs the narrowed copies, the long tier the whole library too.
func TestUsedFeaturesTSParity(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	modes := []struct {
		name string
		opts codegen.GenOptions
	}{
		{"plain", codegen.GenOptions{}},
		{"limits", codegen.GenOptions{LimitControls: true}},
		{"types", codegen.GenOptions{Types: true}},
		{"document", codegen.GenOptions{Document: true}},
	}
	for _, mode := range modes {
		for _, off := range []bool{false, true} {
			if off && !codegentest.Long() {
				continue
			}
			name := mode.name
			if off {
				name += "/no-used-features"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				if off {
					o.NoOpt = []string{codegen.UsedFeatures}
				}
				codegentest.TSParsersCorpus(t, node, codegentest.UsedFeatureCases(), o)
			})
		}
	}
}
