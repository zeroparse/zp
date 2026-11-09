//go:build zp_long

package complexitytest

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/starttest"
)

// TestClassGolden pins the cost class and degree of every corpus grammar (internal/factstest)
// from its default start rule. A grammar without start rules is classified from its first rule.
func TestClassGolden(t *testing.T) {
	lines := make([]string, len(factstest.Corpus))
	t.Run("corpus", func(t *testing.T) {
		for i, path := range factstest.Corpus {
			t.Run(path, func(t *testing.T) {
				t.Parallel()
				g, err := zp.ParseGrammar(factstest.Read(t, path))
				if err != nil {
					t.Fatal(err)
				}
				c, err := engine.ClassifyComplexity(context.Background(), starttest.MarkAll(g), "")
				if err != nil {
					t.Fatal(err)
				}
				lines[i] = fmt.Sprintf("%s %s degree=%d\n", path, c.Class, c.Degree)
			})
		}
	})
	if !t.Failed() {
		factstest.GoldenFile(t, "class.txt", strings.Join(lines, ""))
	}
}
