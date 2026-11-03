package gogen_test

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gogen"
	"zeroparse.dev/zp/internal/gentest"
)

// execGenerate generates the parser of the grammar source with the options, joined into one text,
// and returns the number of the functions printed from Exec IR by table, the functions that the
// generators counted as printed from Exec IR (Generator.ExecFuncs) less the rules and framed
// functions that they printed (Generator.FlowRules and FlowFrames), which is zero because every
// printed function is printed from Exec IR once, or reports false if the generator rejects the
// grammar.
func execGenerate(src string, opts gen.GenOptions) (string, map[string]int, int, bool) {
	g, err := parseTest(src)
	if err != nil {
		return "", nil, 0, false
	}
	var last *gogen.Generator
	var all []*gogen.Generator
	h := &gogen.Hooks{Generator: func(x *gogen.Generator) { last = x; all = append(all, x) }}
	opts.Module, opts.KeepComments = "example.com/p", true
	files, err := gogen.GenerateParser(g, compileForGenerator, opts, h)
	if err != nil {
		return "", nil, 0, false
	}
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "== %s\n", f.Path)
		b.Write(f.Content)
	}
	var n map[string]int
	if last != nil {
		n = last.ExecTables
	}
	extra := 0
	for _, x := range all {
		extra += x.ExecFuncs - x.FlowRules - x.FlowFrames
	}
	return b.String(), n, extra, true
}

// execModes are the generator options under which the rules of the tables (want) print from Exec IR: the modes of TestFlowPrintsTypedRules, and those of the Node runtime alone, in the
// ordinary form and with budget controls, with and without a Document. A stream mode runs the
// grammars with a #stream repetition (streamCases), too few for counts by table.
var execModes = func() []execMode {
	var modes []execMode
	for _, m := range flowModes {
		want := []string{"trules"}
		if m.opts.Recognize {
			want = append(want, "recRules")
		}
		modes = append(modes, execMode{m.name, m.opts, want})
	}
	return append(modes,
		execMode{"nodes", gen.GenOptions{}, []string{"rules"}},
		execMode{"recognizer", gen.GenOptions{Recognize: true}, []string{"rules", "recRules"}},
		execMode{"stream", gen.GenOptions{Stream: true}, nil},
		execMode{"typed stream", gen.GenOptions{Types: true, Stream: true}, nil},
		execMode{"source info", gen.GenOptions{SourceInfo: true}, []string{"rules"}},
		execMode{"default value", gen.GenOptions{DefaultValue: true}, []string{"rules"}},
		execMode{"observable nodes", gen.GenOptions{Observability: true}, []string{"rules"}},
		execMode{"controlled nodes", gen.GenOptions{LimitControls: true}, []string{"rules"}},
		execMode{"controlled nodes recognizer", gen.GenOptions{Recognize: true, LimitControls: true}, []string{"rules", "recRules"}},
		execMode{"controlled stream", gen.GenOptions{Stream: true, LimitControls: true}, nil},
		execMode{"controlled observable nodes", gen.GenOptions{Observability: true, LimitControls: true}, []string{"rules"}},
		execMode{"nodes Document", gen.GenOptions{Document: true}, []string{"rules"}},
		execMode{"recognizer Document", gen.GenOptions{Recognize: true, Document: true}, []string{"rules", "recRules"}},
		execMode{"observable nodes Document", gen.GenOptions{Observability: true, Document: true}, []string{"rules"}},
		execMode{"controlled nodes Document", gen.GenOptions{LimitControls: true, Document: true}, []string{"rules"}},
		execMode{"Document stream", gen.GenOptions{Types: true, Document: true, Stream: true}, nil},
	)
}()

var flowModeNames = func() map[string]bool {
	m := map[string]bool{}
	for _, f := range flowModes {
		m[f.name] = true
	}
	return m
}()

type execMode struct {
	name string
	opts gen.GenOptions
	want []string
}

// TestExecRoutePrintsEveryFunction checks that every corpus grammar generates in the modes of
// execModes, that every function prints from Exec IR once, and that the rules of each table that the
// mode prints print from it. The code is pinned by the goldens of the generator tests.
func TestExecRoutePrintsEveryFunction(t *testing.T) {
	t.Parallel()
	corpus := gentest.Representative(append(gentest.CorpusWithParsers(t), gentest.CoreExtraGrammars...))
	streams := streamCases(t)
	for _, mode := range execModes {
		// The modes after the flowModes each add one option to the ordinary route; the default
		// tier runs a third of those by name.
		if !gentest.Sampled(mode.name) && !flowModeNames[mode.name] {
			continue
		}
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			cases, least := corpus, gentest.ScaleFloor(100)
			if mode.opts.Stream {
				cases, least = streams, len(streams)*3/4
			}
			compared, routed := 0, map[string]int{}
			for _, gc := range cases {
				_, n, extra, ok := execGenerate(gc.Src, mode.opts)
				if !ok {
					continue
				}
				if extra != 0 {
					t.Errorf("%s: %d more functions counted as printed from Exec IR than printed", gc.Name, extra)
				}
				compared++
				for table, k := range n {
					routed[table] += k
				}
			}
			t.Logf("generated %d grammars; functions printed from Exec IR by table: %v", compared, routed)
			if compared < least {
				t.Errorf("generated %d grammars", compared)
			}
			for _, table := range mode.want {
				if routed[table] < gentest.ScaleFloor(1000) {
					t.Errorf("%d functions of table %s printed from Exec IR", routed[table], table)
				}
			}
		})
	}
}
