package lint

import (
	"context"
	"errors"
	"testing"

	"zeroparse.dev/zp/internal/syntax"
)

// TestRewritesErrors requires Rewrites to return an error and no rewrites for a grammar that does
// not compile and for a start rule the grammar does not have, and the rewrites of the findings
// otherwise.
func TestRewritesErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bad, err := syntax.Parse("@start\nrule main = missing")
	if err != nil {
		t.Fatal(err)
	}
	if rs, err := Rewrites(ctx, bad, "main", nil); err == nil || rs != nil {
		t.Errorf("grammar that does not compile: %v, %v", rs, err)
	}
	g, err := syntax.Parse("@start\nrule main = [a-cb-d]\nrule unused = \"x\"")
	if err != nil {
		t.Fatal(err)
	}
	if rs, err := Rewrites(ctx, g, "unused", nil); err == nil || rs != nil {
		t.Errorf("start rule that is not a start rule: %v, %v", rs, err)
	}
	if rs, err := Rewrites(ctx, g, "nothing", nil); err == nil || rs != nil {
		t.Errorf("start rule that does not exist: %v, %v", rs, err)
	}
	findings, err := (Config{}).Run(ctx, g, "main")
	if err != nil {
		t.Fatal(err)
	}
	rs, err := Rewrites(ctx, g, "main", findings)
	if err != nil || len(rs) != len(findings) || len(findings) == 0 {
		t.Fatalf("Rewrites = %v, %v for findings %v", rs, err, findings)
	}
	for i, r := range rs {
		if !r.IsValid() {
			t.Errorf("finding %v has no rewrite", findings[i])
		}
	}
}

// TestCheckGrammarCancelledMidCompile cancels the front end at every context check after the
// first, which checkGrammar makes before the front end starts, until the front end finishes. Each
// cancelled run returns the context's error and no result, and the grammar then checks and lints
// as before.
func TestCheckGrammarCancelledMidCompile(t *testing.T) {
	t.Parallel()
	g, err := syntax.Parse("@start\nrule main = (a / \"a\")* \"b\"\nrule a = \"a\" a / \"x\"\nrule unused = \"x\"")
	if err != nil {
		t.Fatal(err)
	}
	cancelled := 0
	for at := 2; ; at++ {
		ctx := lintTestContext(t, at)
		res, err := checkGrammar(ctx, g)
		if err == nil {
			if res == nil {
				t.Fatal("no result and no error")
			}
			break
		}
		if !errors.Is(err, context.Canceled) || res != nil {
			t.Fatalf("cancelled at check %d: %v, %v", at, res, err)
		}
		cancelled++
	}
	if cancelled == 0 {
		t.Fatal("the front end checked the context only once")
	}
	findings, err := (Config{}).Run(context.Background(), g, "main")
	if err != nil || len(findings) == 0 {
		t.Fatalf("after cancellation: %v, %v", findings, err)
	}
	if _, err := Rewrites(context.Background(), g, "main", findings); err != nil {
		t.Fatalf("Rewrites after cancellation: %v", err)
	}
	t.Logf("cancelled at %d context checks", cancelled)
}
