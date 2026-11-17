//go:build zp_long

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestRunSitesCorpusMatchWithoutRecords checks that the grammars of testdata/grammars give, on their
// sample inputs and on prefixes of them that fail in the middle, byte-identical results with the
// records of run sites taken from the first walk on and with records off, on every backend: a walk
// that takes a record gives what one that evaluates its iterations gives.
func TestRunSitesCorpusMatchWithoutRecords(t *testing.T) {
	defer func(n, m int) {
		state.RunHotSpan, state.ScanHotSpan, state.SkipRecordsOff = n, m, false
	}(state.RunHotSpan, state.ScanHotSpan)
	grammars, _ := filepath.Glob("../../testdata/grammars/*/*.zp")
	if len(grammars) == 0 {
		t.Fatal("no corpus grammars")
	}
	turns := state.RunHotTurns.Load()
	for _, g := range grammars {
		src, err := os.ReadFile(g)
		if err != nil {
			t.Fatal(err)
		}
		prog := mustCompile(t, string(src))
		texts, _ := filepath.Glob(filepath.Join(filepath.Dir(g), "*.txt"))
		var inputs []string
		for _, f := range texts {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			in := string(b)
			inputs = append(inputs, in, in[:len(in)/2], in[:len(in)*3/4])
		}
		for _, in := range inputs {
			for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
				o := ParseOptions{Backend: b, Expectations: true, Unit: core.Bytes}
				state.RunHotSpan, state.ScanHotSpan, state.SkipRecordsOff = 1, 1, false
				n, err := prog.ParseWith(prog.DefaultStartRule, in, o)
				with := codegentest.ResultJSON(n, err)
				state.SkipRecordsOff = true
				n, err = prog.ParseWith(prog.DefaultStartRule, in, o)
				without := codegentest.ResultJSON(n, err)
				state.SkipRecordsOff = false
				if with != without {
					t.Errorf("%s, %s, input of %d bytes %q...: records change the result\n with    %.300s\n without %.300s",
						filepath.Base(g), b, len(in), strings.SplitN(in, "\n", 2)[0], with, without)
				}
			}
		}
	}
	if state.RunHotTurns.Load() == turns {
		t.Error("no run site turned hot: the records were never taken")
	}
}
