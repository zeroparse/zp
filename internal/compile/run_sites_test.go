package compile

import (
	"testing"

	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/parser"
)

// TestRunSitesNeedRecordedReach checks that a repetition is no run site when its element or a rule
// that its element calls holds a repetition without an upper bound that keeps no records, and that
// repetitions of a single character, skip-run sites and other run sites do not count; without the
// condition (NoRecordedReach), the negative control, the first grammars have a site.
func TestRunSitesNeedRecordedReach(t *testing.T) {
	const head = "@start\nrule main = (x \"!\" / [a-z;!,])* $$\n"
	for _, tc := range []struct {
		name, src string
		sites     int // run sites with the condition
		without   int // run sites without it
	}{
		{"a repetition under #cut in a called rule", head + "rule x = (y \";\")*\nrule y = (\"a\" #cut \"b\")*\n", 0, 1},
		{"a recovery whose skip keeps no records", head + "rule x = (y \";\")*\nrule y = (\"a\" \"b\") #recover(skip=(\"a\" #cut \"b\")*)\n", 0, 1},
		{"a repetition of a single character", head + "rule x = (y \";\")*\nrule y = [ab]*\n", 1, 1},
		{"nested run sites", head + "rule x = (y \";\")*\nrule y = (\"ab\" \",\"?)*\n", 2, 2},
	} {
		for _, without := range []bool{false, true} {
			NoRecordedReach = without
			g, err := parser.Parse(tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			prog, err := Check(g, Options{}, nil, desugar.InlineNames{}, nil)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			want := tc.sites
			if without {
				want = tc.without
			}
			if prog.NumRunSites != want {
				t.Errorf("%s (without the condition: %v): %d run sites, want %d", tc.name, without, prog.NumRunSites, want)
			}
		}
	}
	NoRecordedReach = false
}
