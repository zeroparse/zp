package compile

import (
	"context"
	"strings"
	"testing"
	"time"

	"zeroparse.dev/zp/internal/syntax"
)

// TestAnalysisEntryPointsReportDuplicateRule checks that the entry points that analyze a grammar
// without compiling it report a redeclared rule, and do not loop on it.
func TestAnalysisEntryPointsReportDuplicateRule(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"@start\nrule main = main \"a\" / \"a\"\nrule main = main \"b\" / \"b\"", "3:1: rule main is already defined"},
		{"@start\nrule main = a\nrule a = \"a\" main?\nrule main = a \"b\"", "4:1: rule main is already defined"},
	} {
		g, err := syntax.Parse(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		control := &Control{Ctx: ctx}
		_, errClassify := ClassifyComplexity(g, "", control)
		_, errGrowing := GrowingBindings(g, "", control)
		_, errView := NormalView(g, false, control)
		_, errMatching := NormalView(g, true, control)
		cancel()
		for name, err := range map[string]error{"ClassifyComplexity": errClassify, "GrowingBindings": errGrowing, "NormalView": errView, "NormalView matching": errMatching} {
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s(%q) error = %v, want %s", name, tc.src, err, tc.want)
			}
		}
	}
}
