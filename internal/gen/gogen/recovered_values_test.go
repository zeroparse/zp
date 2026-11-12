package gogen_test

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedRecoveredValues checks that generated Go, as nodes and as typed values, gives the
// engine's value for a parse that recovers from a syntax error.
func TestGeneratedRecoveredValues(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.EachMode(t, func(t *testing.T) {
		gentest.GeneratedParsersCorpus(t, goBin, gentest.RecoveredValueCases, gen.GenOptions{})
		gentest.GeneratedTypesCorpus(t, goBin, gentest.RecoveredValueCases, gen.GenOptions{}, 2)
	})
}

// TestGeneratedRecoveredValuesDetectsMutation is the negative control of
// TestGeneratedRecoveredValues: a typed parser that builds a wrong struct is reported for an input
// whose parse recovers, so the typed values of recovered parses are compared.
func TestGeneratedRecoveredValuesDetectsMutation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	gentest.EachMode(t, func(t *testing.T) {
		goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
		cases := gentest.RecoveredValueCases[1:2] // struct in a list
		mutated := 0
		mutate := func(code []byte) []byte {
			out := bytes.ReplaceAll(code, []byte("v.A = astInt(f0)"), []byte("v.A = astInt(f0) + 1"))
			if !bytes.Equal(out, code) {
				mutated++
			}
			return out
		}
		problems := gentest.GeneratedTypesMismatches(t, goBin, cases, gen.GenOptions{}, mutate, 1)
		if mutated == 0 {
			t.Fatal("the mutation did not change the typed parser")
		}
		found := false
		for _, p := range problems {
			if strings.Contains(p, `input "1a;2!;"`) {
				found = true
			}
		}
		if !found {
			t.Errorf("the mutated typed parser is not reported for a recovered parse; problems: %q", problems)
		}
	})
}
