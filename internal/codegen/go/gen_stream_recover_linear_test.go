package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// A generated stream parse recovers in time proportional to the work: the compute that it needs
// grows linearly with the input when each recovery skips a constant amount, as the batch parse
// does, and the same holds when the skip is a run that walks share. Counting the line of every recovered error from the start of the buffer made it grow
// with the size of the buffer.
func TestGeneratedStreamRecoverIsLinear(t *testing.T) {
	codegentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds a generated standalone stream consumer")
	}
	for name, src := range map[string]string{
		"constant skip": `rule main = (x / "a")* #stream $$
rule x = (y #recover(skip="a")) "c"
rule y = "b"`,
		"reused skip": `rule main = (x / "a")* #stream $$
rule x = (y #recover(skip=[a]+)) "c"
rule y = "b"`,
	} {
		t.Run(name, func(t *testing.T) { testGeneratedStreamRecoverIsLinear(t, src) })
	}
}

func testGeneratedStreamRecoverIsLinear(t *testing.T, src string) {
	t.Parallel()
	g, err := parseTest(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, source := range map[string]string{
		"go.mod":         codegentest.GoMod("fixture"),
		"parser.go":      string(code),
		"parser_test.go": generatedStreamRecoverLinearConsumer,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatal(err)
	}
}

const generatedStreamRecoverLinearConsumer = `package fixture

import (
	"errors"
	"strings"
	"testing"
)

// steps is the smallest compute limit with which the stream parse of n characters finishes.
func steps(t *testing.T, n int) uint64 {
	input := strings.Repeat("a", n)
	finishes := func(limit uint64) bool {
		err := ParseStream(strings.NewReader(input), func(*Node) error { return nil }, WithLimits(Limits{Compute: limit}))
		var le *LimitError
		if errors.As(err, &le) {
			return false
		}
		if err != nil {
			t.Fatal(err)
		}
		return true
	}
	lo, hi := uint64(0), uint64(1)<<34
	for hi-lo > 1 && hi-lo > hi/256 {
		mid := lo + (hi-lo)/2
		if finishes(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

func TestRecoverStepsAreLinear(t *testing.T) {
	var s [4]uint64
	for i := range s {
		s[i] = steps(t, 1000<<i)
	}
	t.Logf("steps at 1000, 2000, 4000, 8000: %v", s)
	// Linear growth doubles the steps with the input; a rescan of the buffer for each
	// recovery makes it grow with the buffer.
	for i := 1; i < len(s); i++ {
		if r := float64(s[i]) / float64(s[i-1]); r > 2.4 {
			t.Errorf("steps grew by %.2f when the input doubled to %d", r, 1000<<i)
		}
	}
}
`
