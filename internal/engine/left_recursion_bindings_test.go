package engine

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
)

// TestLeftRecursionGuardIgnoresVariables checks that a left-recursive re-entry at the same
// position gets the growing result even when its variable environment differs from that of
// the call being grown, on every backend and in Documents: the parse matches the same rule
// without the binding instead of nesting until the depth limit.
func TestLeftRecursionGuardIgnoresVariables(t *testing.T) {
	t.Parallel()
	inputs := []string{"x", "xxx", "xxxxxxxxxx", "", "y"}
	for _, options := range []compile.Options{{}, {DisableMemo: true}} {
		options.LeftRecursionBindings = true
		prog := mustCompile(t, codegentest.LRBindingSource, options)
		plain := mustCompile(t, codegentest.PlainLRSource, options)
		for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
				for _, recognize := range []bool{false, true} {
					// A compute limit makes the parse take the controlled memo lookups.
					for _, lim := range []core.Limits{{Depth: 400}, {Compute: 1 << 40, Depth: 400}} {
						opts := ParseOptions{Backend: backend, Unit: unit, Recognize: recognize, Limits: lim}
						for _, input := range inputs {
							got := codegentest.ResultJSON(prog.ParseWith("main", input, opts))
							want := codegentest.ResultJSON(plain.ParseWith("main", input, opts))
							if got != want {
								t.Fatalf("%s/%s recognize=%t DisableMemo=%t limits=%v %q:\n got  %s\n want %s", backend, unit, recognize, options.DisableMemo, lim, input, got, want)
							}
						}
					}
				}
				d, err := prog.NewDocumentWith("main", "xx", ParseOptions{Backend: backend, Unit: unit, Limits: core.Limits{Depth: 400}})
				if err != nil {
					t.Fatal(err)
				}
				for _, edit := range []struct {
					start, end int
					text       string
				}{{2, 2, "x"}, {0, 0, "xx"}, {1, 3, ""}, {0, 3, "y"}, {0, 1, "x"}} {
					if err := d.Edit(edit.start, edit.end, edit.text); err != nil {
						t.Fatal(err)
					}
					got := codegentest.ResultJSON(d.Parse())
					want := codegentest.ResultJSON(plain.ParseWith("main", d.Text(), ParseOptions{Backend: backend, Unit: unit}))
					if got != want {
						t.Fatalf("%s/%s Document %q:\n got  %s\n want %s", backend, unit, d.Text(), got, want)
					}
				}
			}
		}
	}
}
