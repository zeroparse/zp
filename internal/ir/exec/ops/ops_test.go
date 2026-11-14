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

// TestPilotGolden checks that the pilot declarations, and those with the transitional operations,
// verify and that their listings are testdata/pilot.golden and testdata/transitional.golden: a change
// of a contract, a precondition or a cost bound shows in review.
func TestPilotGolden(t *testing.T) {
	withTransitional := Pilot()
	Transitional(withTransitional)
	for golden, m := range map[string]*exec.Module{"testdata/pilot.golden": Pilot(), "testdata/transitional.golden": withTransitional} {
		if err := exec.Verify(m); err != nil {
			t.Fatal(err)
		}
		got := exec.String(m)
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
			t.Errorf("listing differs from %s; run go test -run TestPilotGolden -update", golden)
		}
	}
}

// TestCrossComponentWriters pins the transitional operations that write a component other than
// their own, which Verify allows only for transitional operations: the growth operations that
// record recovered errors again. Another writer needs a reason recorded here.
func TestCrossComponentWriters(t *testing.T) {
	m := Pilot()
	Transitional(m)
	var got []string
	for _, o := range m.Ops {
		for _, s := range o.Writes {
			if s != o.Component && isComponent(m, s) {
				got = append(got, o.Name+" writes "+s)
			}
		}
	}
	want := []string{"ctx.replay writes " + Rec, "grow.step writes " + Rec, "grow.end writes " + Rec}
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

// TestValueGolden checks that the declarations with the value primitives verify and that their listing
// is testdata/value.golden.
func TestValueGolden(t *testing.T) {
	m := ValueModule()
	if err := exec.Verify(m); err != nil {
		t.Fatal(err)
	}
	got := exec.String(m)
	const golden = "testdata/value.golden"
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
		t.Errorf("listing differs from %s; run go test -run TestValueGolden -update", golden)
	}
}

// TestTermCodeGolden checks that the declarations of applying a term verify, on the pilot and the value
// and transitional operations they extend, and what the listing says of them.
func TestTermCodeGolden(t *testing.T) {
	m := ValueModule()
	Transitional(m)
	AddTermCode(m)
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
		"host op term.ctx() Act\n  component host\n  reads host, pos\n  writes host\n  cost <= 0\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("listing lacks\n%s", want)
		}
	}
}
