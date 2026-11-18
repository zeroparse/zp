package print_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestUsedFeaturesEmbedParity compares parsers that carry their own copy of the library, narrowed to
// their features, with the engine on codegentest.UsedFeatureCases, in every mode of nodeRouteModes and in the
// Document route, and does the same with the optimization used-features off, whose copy is the whole
// library. Each copy must build, so this also checks that pruning leaves no dangling name. The
// default tier runs the narrowed copies of the plain and Document routes, the long tier
// every combination.
func TestUsedFeaturesEmbedParity(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	modes := append(nodeRouteModes[:len(nodeRouteModes):len(nodeRouteModes)], struct {
		name string
		opts common.GenOptions
	}{"document", common.GenOptions{Document: true}})
	for _, mode := range modes {
		for _, off := range []bool{false, true} {
			name := mode.name
			if off {
				name += "/no-used-features"
			}
			if !codegentest.Long() && (off || mode.name != "plain" && mode.name != "document") {
				continue
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				if off {
					o.NoOpt = []string{common.UsedFeatures}
				}
				codegentest.GeneratedParsersEmbedCorpus(t, goBin, codegentest.UsedFeatureCases(), o)
			})
		}
	}
}
