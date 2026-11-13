package gents_test

import (
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
	node := gentest.RequireNode(t)
	var all []string
	for _, f := range genopt.Flags {
		if f.Kind != genopt.Runtime && !f.ChangesOutput {
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
				o.NoOpt, o.Opt = s.noOpt, s.set
				gentest.TSParsersCorpus(t, node, gentest.FlagParityCases(), o)
			})
		}
	}
}

// TestProfilesTSParity compares TypeScript parsers generated with each profile other than speed with
// the engine, in every mode of tsModes. The profiles set no flag that acts on TypeScript output
// today, but a profile selects flag values for every target, so the parsers must agree with the
// engine whatever the values are.
func TestProfilesTSParity(t *testing.T) {
	t.Parallel()
	node := gentest.RequireNode(t)
	for _, name := range genopt.Profiles()[1:] {
		for _, mode := range tsModes {
			t.Run(name+"/"+mode.name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				o.Optimize = name
				gentest.TSParsersCorpus(t, node, gentest.FlagParityCases(), o)
			})
		}
	}
}

// TestOptimizationDisabledTSNegativeControl shows that the comparison of the test above, the one
// of TSParsersCorpus with the engine, notices a parser that is wrong: with the guards off and a
// literal of the grammar changed in the generated code, results differ from the engine's, and
// without the change they do not.
func TestOptimizationDisabledTSNegativeControl(t *testing.T) {
	t.Parallel()
	node := gentest.RequireNode(t)
	for _, mode := range tsModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			o := mode.opts
			o.NoOpt = []string{"guards"}
			cases := gentest.FlagParityCases()
			if got := gentest.TSParsersMismatches(t, node, cases, o, nil); len(got) != 0 {
				t.Fatalf("the unchanged parsers differ from the engine's results:\n%s", got[0])
			}
			var changed atomic.Bool
			mutate := func(src string) string {
				out := strings.ReplaceAll(src, `"while"`, `"whilx"`)
				if out != src {
					changed.Store(true)
				}
				return out
			}
			got := gentest.TSParsersMismatches(t, node, cases, o, mutate)
			if !changed.Load() {
				t.Fatal(`the generated code has no literal "while" to change: the control proves nothing`)
			}
			if len(got) == 0 {
				t.Errorf("a parser with a changed literal gives the results of the engine")
			}
		})
	}
}
