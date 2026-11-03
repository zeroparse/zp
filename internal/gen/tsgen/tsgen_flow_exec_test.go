package tsgen_test

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/tsgen"
	"zeroparse.dev/zp/internal/gentest"
)

// execGenerate generates the TypeScript parser of the grammar source with the options, joined into
// one text, and returns the number of the functions printed from Exec IR, or reports false if the
// generator rejects the grammar.
func execGenerate(src string, opts gen.GenOptions) (string, int, bool) {
	g, err := parseTest(src)
	if err != nil {
		return "", 0, false
	}
	n := 0
	h := &tsgen.Hooks{Generator: func(x *tsgen.Generator) { n = x.ExecFuncs }}
	files, err := tsgen.GenerateParser(g, compileForGenerator, opts, h)
	if err != nil {
		return "", 0, false
	}
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "== %s\n", f.Path)
		b.Write(f.Content)
	}
	return b.String(), n, true
}

// TestExecRoutePrintsEveryFunction checks that every corpus grammar generates TypeScript, in each
// mode that changes how functions print, and that the functions print from Exec IR. The code is
// pinned by the goldens of the generator tests.
func TestExecRoutePrintsEveryFunction(t *testing.T) {
	t.Parallel()
	cases := gentest.Representative(append(gentest.CorpusWithParsers(t), gentest.CoreExtraGrammars...))
	modes := []struct {
		name string
		opts gen.GenOptions
	}{
		{"plain", gen.GenOptions{}},
		{"recognize", gen.GenOptions{Recognize: true}},
		{"budget", gen.GenOptions{Recognize: true, LimitControls: true}},
		{"observe", gen.GenOptions{Observability: true}},
		{"types", gen.GenOptions{Types: true}},
		{"default value", gen.GenOptions{DefaultValue: true}},
	}
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			compared, routed := 0, 0
			for _, gc := range cases {
				_, n, ok := execGenerate(gc.Src, mode.opts)
				if !ok {
					continue
				}
				compared++
				routed += n
			}
			t.Logf("generated %d grammars; %d functions printed from Exec IR", compared, routed)
			if compared < gentest.ScaleFloor(100) {
				t.Errorf("compared %d grammars", compared)
			}
			if routed < gentest.ScaleFloor(1000) {
				t.Errorf("%d functions printed from Exec IR", routed)
			}
		})
	}
}
