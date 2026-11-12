package frontend

import (
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/diag"
)

// rejected are grammars that the compiler rejects for the errors that depend on the facts and on
// nothing else: the untyped module must report the same errors, at the same positions.
var rejected = []struct{ name, src string }{
	{"binding", "var k int\n@start\nrule main = R $$\nrule R = {k := k + 1} R \"x\" / \"x\""},
	{"binding through a rule", "var k int\n@start\nrule main = A $$\nrule A = {k := k + 1} B \"x\" / \"x\"\nrule B = A \"y\""},
	{"binding in a repetition", "var k int\n@start\nrule main = R $$\nrule R = ({k := k + 1} \"b\"?)* R \"x\" / \"x\""},
	{"stream on a cycle", "@start\nrule main = (main / \"y\")+ #stream"},
	{"splice cycle", "@start\nrule main = _s\nrule _s = \"x\" _s"},
}

func TestStructuralErrorsMatchCompile(t *testing.T) {
	t.Parallel()
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, err := compiletest.Parse(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, cerr := compiletest.Compile(g, compile.Options{})
			want, ok := cerr.(diag.ErrorList)
			if !ok {
				t.Fatalf("compile error %v is not a list", cerr)
			}
			got := structuralErrors(g, compile.Options{}, desugar.InlineNames{}, nil)
			if len(got) == 0 || got.Error() != want.Error() {
				t.Fatalf("structural errors\n%s\ncompile errors\n%s", got, want)
			}
		})
	}
}

// TestRunErrorsMatchCompile requires Run to report the compiler's errors, in its order, for
// grammars where the compiler reports an error that depends on the facts together with errors
// that do not: a second #stream in a cyclic rule is reported once, as the compiler does, and so is
// a #stream on a cycle after one that is not at the top of the body.
func TestRunErrorsMatchCompile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, src string }{
		{"second stream on a cycle", "@start\nrule main = (main / \"y\")+ #stream (\"z\")+ #stream"},
		{"stream on a cycle after a nested one", "@start\nrule main = (main / \"y\")+ (\"x\"+ #stream)? (\"z\")+ #stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, err := compiletest.Parse(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, cerr := compiletest.Compile(g, compile.Options{})
			if cerr == nil {
				t.Fatal("compile accepted the grammar")
			}
			_, rerr := Run(g, compile.Options{}, nil)
			if rerr == nil || rerr.Error() != cerr.Error() {
				t.Fatalf("Run errors\n%v\ncompile errors\n%v", rerr, cerr)
			}
		})
	}
}

// TestMergeErrors requires a fact error the compiler did not report to be added in position order,
// and the compiler's error to be returned as is if it has them all.
func TestMergeErrors(t *testing.T) {
	t.Parallel()
	pos := func(line int) *diag.Error { return &diag.Error{Pos: grammarPos(line), Msg: "m"} }
	a, b, c := pos(1), pos(2), pos(3)
	if got := mergeErrors(diag.ErrorList{a, c}, diag.ErrorList{c}); got.(diag.ErrorList)[1] != c || len(got.(diag.ErrorList)) != 2 {
		t.Fatalf("all reported: %v", got)
	}
	got := mergeErrors(diag.ErrorList{a, c}, diag.ErrorList{b})
	if l := got.(diag.ErrorList); len(l) != 3 || l[0] != a || l[1] != b || l[2] != c {
		t.Fatalf("merged: %v", got)
	}
	// The compiler's order is kept even where it is not the order of the positions.
	d := pos(4)
	got = mergeErrors(diag.ErrorList{d, a, c}, diag.ErrorList{b})
	if l := got.(diag.ErrorList); len(l) != 4 || l[0] != b || l[1] != d || l[2] != a || l[3] != c {
		t.Fatalf("merged into an unsorted list: %v", got)
	}
	got = mergeErrors(diag.ErrorList{c, a}, diag.ErrorList{d})
	if l := got.(diag.ErrorList); len(l) != 3 || l[0] != c || l[1] != a || l[2] != d {
		t.Fatalf("appended: %v", got)
	}
}

func grammarPos(line int) grammar.Pos { return grammar.Pos{Line: line, Col: 1} }
