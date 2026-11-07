package tsgen_test

import (
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	genopt "zeroparse.dev/zp/internal/gen/opt"
	"zeroparse.dev/zp/internal/gentest"
)

// tsModes are the ways a TypeScript parser is generated, which take different routes.
var tsModes = []struct {
	name string
	opts gen.GenOptions
}{
	{"plain", gen.GenOptions{}},
	{"types", gen.GenOptions{Types: true}},
	{"observability", gen.GenOptions{Observability: true}},
	{"limits", gen.GenOptions{LimitControls: true}},
}

// TestOptimizationDisabledTSParity compares TypeScript parsers generated with an optimization off,
// or with the opt-in literal sharing on, with the engine, in every mode of tsModes.
func TestOptimizationDisabledTSParity(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	var all []string
	for _, f := range genopt.Flags {
		if f.Kind != genopt.Runtime {
			all = append(all, f.Name)
		}
	}
	sets := []struct {
		name  string
		noOpt []string
		set   map[string]int
	}{
		{"guards", []string{"guards"}, nil},
		{"scans", []string{"scans"}, nil},
		{"text-dispatch", []string{"text-dispatch"}, nil},
		{"method-sharing", []string{"method-sharing"}, nil},
		{"sparse-memo", []string{"sparse-memo"}, nil},
		{"all", all, nil},
		{"ts-literal-sharing", nil, map[string]int{"ts-literal-sharing": 1}},
	}
	for _, mode := range tsModes {
		for _, s := range sets {
			t.Run(mode.name+"/"+s.name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				o.NoOpt, o.Set = s.noOpt, s.set
				gentest.TSParsersCorpus(t, node, gentest.FlagParityCases(), o)
			})
		}
	}
}

// TestOptimizationDisabledTSNegativeControl shows that the comparison of the test above notices a
// parser that is wrong: with the guards off and a literal of the grammar changed in the generated
// code, the results differ from those of the parser as generated.
func TestOptimizationDisabledTSNegativeControl(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	for _, mode := range tsModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			o := mode.opts
			o.NoOpt = []string{"guards"}
			cases := gentest.FlagParityCases()
			var changed atomic.Bool
			mutate := func(src string) string {
				out := strings.ReplaceAll(src, `"while"`, `"whilx"`)
				if out != src {
					changed.Store(true)
				}
				return out
			}
			want := gentest.TSParsersOutputs(t, node, cases, o, nil)
			got := gentest.TSParsersOutputs(t, node, cases, o, mutate)
			if !changed.Load() {
				t.Fatal(`the generated code has no literal "while" to change: the control proves nothing`)
			}
			if got == want {
				t.Errorf("a parser with a changed literal gives the results of the parser as generated")
			}
		})
	}
}
