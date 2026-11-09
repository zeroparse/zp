package engine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/syntax"
)

// duplicateRuleGrammars declare a rule twice where the rules call rules; the analyses once
// looped on them, since only compilation reported the redeclaration.
var duplicateRuleGrammars = []struct{ src, want string }{
	{"@start\nrule main = main \"a\" / \"a\"\nrule main = main \"b\" / \"b\"", "3:1: rule main is already defined"},
	{"@start\nrule main = a\nrule a = \"a\" main?\nrule main = a \"b\"", "4:1: rule main is already defined"},
}

func TestClassifyDuplicateRule(t *testing.T) {
	for _, tc := range duplicateRuleGrammars {
		g, err := syntax.Parse(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = engine.ClassifyComplexity(ctx, g, "")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ClassifyComplexity(%q) error = %v, want %s", tc.src, err, tc.want)
		}
		_, err = engine.GrowingBindings(ctx, g, "main")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("GrowingBindings(%q) error = %v, want %s", tc.src, err, tc.want)
		}
		cancel()
	}
}
