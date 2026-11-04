package engine

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/value"
)

// The three backends of the engine agree on every separated list, and a list rejects what its
// policy forbids.
func TestSeparatedListsAcrossBackends(t *testing.T) {
	t.Parallel()
	for _, fixture := range gentest.SeparatedCases() {
		t.Run(fixture.Name, func(t *testing.T) {
			program := mustCompile(t, fixture.Src)
			for _, input := range fixture.Inputs {
				baseline := gentest.ResultJSON(program.ParseWith("main", input, ParseOptions{Backend: Closure}))
				for _, backend := range []Backend{Bytecode, BytecodeIterative} {
					if actual := gentest.ResultJSON(program.ParseWith("main", input, ParseOptions{Backend: backend})); actual != baseline {
						t.Fatalf("%s input=%q: %s / %s", backend, input, actual, baseline)
					}
				}
			}
		})
	}
}

// A Document reparses a separated list after random edits as a fresh parse does.
func TestSeparatedListDocumentEdits(t *testing.T) {
	t.Parallel()
	for _, trailing := range []string{"forbidden", "optional", "required"} {
		src := "rule main = num* #sep(\",\", trailing=\"" + trailing + "\") $$\nrule num = [0-9]+\n"
		p := mustCompile(t, src)
		for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
			t.Run(trailing+"/"+back.String(), func(t *testing.T) {
				d, err := p.NewDocumentWith("main", "1,2,3", ParseOptions{Backend: back})
				if err != nil {
					t.Fatal(err)
				}
				rng := rand.New(rand.NewSource(7))
				alphabet := []string{"1", "22", ",", ",", " ", "", "3,"}
				for i := 0; i < 300; i++ {
					n := len([]rune(d.Text()))
					start := rng.Intn(n + 1)
					end := start + rng.Intn(min(3, n-start)+1)
					if err := d.Edit(start, end, alphabet[rng.Intn(len(alphabet))]); err != nil {
						t.Fatal(err)
					}
					got, err := d.Parse()
					want, werr := p.ParseWith("main", d.Text(), ParseOptions{Backend: back})
					if dump(t, got, err) != dump(t, want, werr) {
						t.Fatalf("edit %d: %q got%s want%s", i, d.Text(), dump(t, got, err), dump(t, want, werr))
					}
				}
			})
		}
	}
}

// A separated list of the start rule streams its elements, and a stream rejects what a batch parse
// rejects.
func TestSeparatedListStream(t *testing.T) {
	t.Parallel()
	for _, trailing := range []string{"forbidden", "optional", "required"} {
		src := "rule main = num* #sep(\",\", trailing=\"" + trailing + "\") #stream $$\nrule num = [0-9]+ #atomic\n"
		p := mustCompile(t, src)
		for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, input := range []string{"", "1", "1,2,3", "1,", "1,2,", ",", "1,,2", "1,2;", "12,345"} {
				var got []string
				err := p.ParseStreamWith("main", strings.NewReader(input), func(v *value.Node) error {
					got = append(got, fmt.Sprint(value.D(v).Start, "-", value.D(v).End))
					return nil
				}, ParseOptions{Backend: back})
				var want []string
				batch, berr := p.ParseWith("main", input, ParseOptions{Backend: back})
				if berr == nil {
					for _, kid := range value.D(value.D(batch).Children[0]).Children {
						want = append(want, fmt.Sprint(value.D(kid).Start, "-", value.D(kid).End))
					}
				}
				if (err == nil) != (berr == nil) || berr == nil && strings.Join(got, " ") != strings.Join(want, " ") {
					t.Errorf("%s %s %q: stream %v %v, batch %v %v", trailing, back, input, got, err, want, berr)
				}
			}
		}
	}
}
