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
	{"stream on a cycle in a rule that is not a start rule", "@start\nrule main = r $$\nrule r = (r / \"y\")+ #stream"},
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

// TestRunRejectsStreamOnCycleInAnyRule requires Run to reject a #stream on a left-recursion cycle
// in a rule that is not a start rule, with the message and position given here rather than the
// compiler's, so that the error is not lost when the compiler stops reporting the errors that
// depend on the facts. TestStructuralErrorsMatchCompile requires the facts alone to report it.
func TestRunRejectsStreamOnCycleInAnyRule(t *testing.T) {
	t.Parallel()
	g, err := compiletest.Parse("@start\nrule main = r $$\nrule r = (r / \"y\")+ #stream")
	if err != nil {
		t.Fatal(err)
	}
	const want = "3:21: #stream cannot be used in rule r, which is on a left-recursion cycle; growing the recursion would re-evaluate elements already delivered"
	if _, err := Run(g, compile.Options{}, nil); err == nil || err.Error() != want {
		t.Fatalf("Run error %v, want %s", err, want)
	}
}

// TestRunRejectsFactErrors requires Run, and the facts of the untyped module alone, to reject every
// kind of error that depends on the facts with the message and position given here rather than the
// compiler's, so that none is lost when the compiler stops reporting them. Run then has to look
// for them also when checking succeeds.
func TestRunRejectsFactErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ src, want string }{
		{"var k int\n@start\nrule main = R $$\nrule R = {k := k + 1} R \"x\" / \"x\"",
			"4:11: rule R binds variable k before its left-recursive call of R; the left recursion reads k"},
		{"var k int\n@start\nrule main = A $$\nrule A = {k := k + 1} B \"x\" / \"x\"\nrule B = A \"y\"",
			"4:11: rule A binds variable k before its left-recursive call of B; the left recursion reads k"},
		{"@start\nrule main = (main / \"y\")+ #stream",
			"2:27: #stream cannot be used in rule main, which is on a left-recursion cycle; growing the recursion would re-evaluate elements already delivered"},
		{"@start\nrule main = _s\nrule _s = \"x\" _s",
			"3:1: rule _s contains itself in its sequence of elements, so it has no fixed number of them"},
	} {
		g, err := compiletest.Parse(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Run(g, compile.Options{}, nil); err == nil || err.Error() != tc.want {
			t.Errorf("Run error %v, want %s", err, tc.want)
		}
		if got := structuralErrors(g, compile.Options{}, desugar.InlineNames{}, nil); got.Error() != tc.want {
			t.Errorf("structural errors %v, want %s", got, tc.want)
		}
	}
}
