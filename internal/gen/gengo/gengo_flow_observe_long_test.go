//go:build zp_long

package gengo_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// TestFlowNodeObservationMutants checks that the golden notices a Flow build that drops, moves or reorders
// an event: it changes the code of the build in each way and requires a digest that differs from the
// golden's for some case.
func TestFlowNodeObservationMutants(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	gentest.PinOrdinary(t)
	cases := append(gentest.GenCorpus(t), observeShapes...)
	golden := gentest.ReadGolden(t, observationGolden)
	changed := func(t *testing.T, lines []string) int {
		t.Helper()
		n := 0
		for k, v := range observationDigests(t, "corpus", cases, lines) {
			if w, ok := golden[k]; !ok || w != v {
				n++
			}
		}
		return n
	}
	if n := changed(t, runObserveProbes(t, cases, "main", nil, nil)); n != 0 {
		t.Fatalf("%d digests of the unmutated build differ from %s", n, observationGolden)
	}
	for _, m := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"dropped skipped event", func(code []byte) []byte {
			loc := skippedStmt.FindIndex(code)
			if loc == nil {
				return code
			}
			return append(append([]byte(nil), code[:loc[0]]...), code[loc[1]:]...)
		}},
		{"reordered skipped events", func(code []byte) []byte {
			// Exchange the rule names of the first two consecutive reports of a function.
			all := skippedStmt.FindAllSubmatchIndex(code, -1)
			for i := 0; i+1 < len(all); i++ {
				a, b := all[i], all[i+1]
				if string(code[a[2]:a[3]]) != string(code[b[2]:b[3]]) && !strings.Contains(string(code[a[1]:b[0]]), "\nfunc ") {
					na, nb := string(code[a[2]:a[3]]), string(code[b[2]:b[3]])
					out := string(code[:a[2]]) + nb + string(code[a[3]:b[2]]) + na + string(code[b[3]:])
					return []byte(out)
				}
			}
			return code
		}},
		{"shifted position of an event", func(code []byte) []byte {
			loc := skippedStmt.FindIndex(code)
			if loc == nil {
				return code
			}
			stmt := strings.Replace(string(code[loc[0]:loc[1]]), "p.Pos, 0)", "p.Pos+1, 0)", 1)
			return []byte(string(code[:loc[0]]) + stmt + string(code[loc[1]:]))
		}},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			if changed(t, runObserveProbes(t, cases, "main", nil, m.mutate)) == 0 {
				t.Errorf("the mutant %q changes no digest: the events are not compared", m.name)
			}
		})
	}
}
