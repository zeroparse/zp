//go:build zp_long

package gogen_test

import (
	"bytes"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// TestTermChargeGoldensDetectBrokenCharges checks that the term charge goldens notice generated Go that
// charges other than the library for a lowered term. Each mutant edits the generated parsers of a case set
// and must occur in them, and the Go entries of its golden must change:
//   - a member read of a capture passes the bare capture name instead of the name cut from the member
//     name, so that the controlled comparison with the names of the captures can take its constant path
//     for the same string data;
//   - the elements that a tuple, a list, a concat or a struct literal makes charge no step.
func TestTermChargeGoldensDetectBrokenCharges(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cutName := regexp.MustCompile(`"\$(\w+)"\[1:\]`)
	for _, m := range []struct {
		name, golden, prefix string
		cases                func() []gentest.TermCase
		mutate               func([]byte) ([]byte, int)
	}{
		{"a capture member passes the bare name", "testdata/term_charges_member.golden", "termchargesmember", gentest.MemberTermCases,
			func(src []byte) ([]byte, int) {
				return cutName.ReplaceAll(src, []byte(`"$1"`)), len(cutName.FindAllIndex(src, -1))
			}},
		{"struct fields charge no step", "testdata/term_charges_struct.golden", "termchargesstruct", gentest.StructTermCases, dropElementSteps},
		{"pushed elements charge no step", "testdata/term_charges_push.golden", "termchargespush", gentest.PushTermCases, dropElementSteps},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			var hits atomic.Int64
			got := goTermCharges(t, m.prefix, termChargeCases(t, m.cases()), func(src []byte) []byte {
				out, n := m.mutate(src)
				hits.Add(int64(n))
				return out
			})
			if hits.Load() == 0 {
				t.Fatalf("the mutant %q matches no generated parser", m.name)
			}
			want := gentest.ReadGolden(t, m.golden)
			differ := 0
			for k, v := range got {
				w, ok := want[k]
				if !ok {
					t.Fatalf("%s has no entry for %q", m.golden, k)
				}
				if w != v {
					differ++
					if differ <= 5 {
						t.Logf("%s: %s, golden %s", k, v, w)
					}
				}
			}
			if differ == 0 {
				t.Errorf("the mutant %q (%d edits) changes no Go entry of %s", m.name, hits.Load(), m.golden)
			}
		})
	}
}

// dropElementSteps removes the step that generated Go charges for each element a lowered term makes
// (c.P.Spend()), returning the code and the number of steps removed.
func dropElementSteps(src []byte) ([]byte, int) {
	const step = "c.P.Spend()\n"
	n := bytes.Count(src, []byte(step))
	return []byte(strings.ReplaceAll(string(src), step, "\n")), n
}
