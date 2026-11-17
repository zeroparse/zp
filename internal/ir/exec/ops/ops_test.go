package ops

import (
	"flag"
	"os"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
)

var update = flag.Bool("update", false, "update golden files")

// TestParserGolden checks that the declarations of the whole parser VM verify and that their listing
// is testdata/parser.golden: a change of a contract, a precondition or a cost bound shows in review.
func TestParserGolden(t *testing.T) {
	m := Parser()
	if err := exec.Verify(m); err != nil {
		t.Fatal(err)
	}
	got := exec.String(m)
	const golden = "testdata/parser.golden"
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("listing differs from %s; run go test -run TestParserGolden -update", golden)
	}
}

// TestCrossComponentWriters pins the transitional operations of the whole module (Parser) that write
// a component other than their own, which Verify allows only for transitional operations: the growth
// operations that record recovered errors again. Another writer needs a reason recorded here. The
// leaves of terms, which the lowering declares, are pinned there (TestTermLeavesWriteOwnComponent).
func TestCrossComponentWriters(t *testing.T) {
	m := Parser()
	var got []string
	for _, o := range m.Ops {
		for _, s := range o.Writes {
			if s != o.Component && isComponent(m, s) {
				got = append(got, o.Name+" writes "+s)
			}
		}
	}
	want := []string{"grow.step writes " + Rec, "grow.end writes " + Rec}
	if !slices.Equal(got, want) {
		t.Errorf("cross-component writers:\n got %q\nwant %q", got, want)
	}
}

func isComponent(m *exec.Module, name string) bool {
	for _, c := range m.Components {
		if c.Name == name {
			return true
		}
	}
	return false
}

// TestTermCodeGolden checks that the declarations of applying a term verify in the whole module, and
// what the listing says of them.
func TestTermCodeGolden(t *testing.T) {
	m := Parser()
	if err := exec.Verify(m); err != nil {
		t.Fatal(err)
	}
	h := m.HandleType(Term)
	if h == nil || h.Callable == nil || len(h.Callable.Params) != 1 || h.Callable.Params[0] != exec.THandle(Act) ||
		len(h.Callable.Results) != 2 || h.Callable.Results[0] != exec.THandle(Datum) || h.Callable.Results[1] != exec.THandle(Fail) {
		t.Fatalf("handle Term = %+v, want code of an Act with a Datum and a Fail", h)
	}
	for _, o := range m.Ops {
		if o.Name == "term.ctx" && (!o.Host || o.Transitional || o.NoReturn || len(o.Cost.Terms) != 0 || o.Cost.Const != 0) {
			t.Errorf("term.ctx = %+v, want a host operation that charges nothing and does not end the parse", o)
		}
	}
	got := exec.String(m)
	for _, want := range []string{
		"handle Term of host callable (Act) (Datum, Fail)\n",
		"host op term.ctx() Act\n  component host\n  reads host, pos, made\n  writes host\n  cost <= 0\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("listing lacks\n%s", want)
		}
	}
}
