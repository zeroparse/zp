package engine

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/limits"
)

// TestUnsetFieldsReadZero pins the value of each field type that a constructor left unset, on every
// backend of the engine.
func TestUnsetFieldsReadZero(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, want string }{
		{"int", `0`},
		{"string", `""`},
		{"bool", `false`},
		{"list", `[]`},
		{"list length", `0`},
		{"list in a concat", `[1]`},
		{"optional", `null`},
		{"empty list type", `[]`},
		{"empty list type length", `0`},
		{"nil type", `null`},
		{"tuple", `[0,""]`},
		{"struct field", `0`},
		{"struct string field", `""`},
		{"top", `null`},
		{"arithmetic", `0`},
		{"comparison", `true`},
		{"set and unset", `8`},
	} {
		for _, c := range gentest.UnsetZeroCases() {
			if c.Name != tc.name {
				continue
			}
			program := mustCompile(t, c.Src)
			for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
				got := gentest.ResultJSON(program.ParseWith("main", c.Inputs[0], ParseOptions{Backend: backend}))
				if !strings.Contains(got, tc.want) {
					t.Errorf("%s %s: %s, want %s", tc.name, backend, got, tc.want)
				}
			}
		}
	}
}

// TestZeroValueOfDeepStructTypes pins the cost of the zero value of a struct type that holds another
// twice, many levels deep: it does not depend on the depth, neither when the program is compiled nor
// when a field of the zero value is read.
func TestZeroValueOfDeepStructTypes(t *testing.T) {
	t.Parallel()
	const depth = 24
	var src strings.Builder
	for i := range depth {
		fmt.Fprintf(&src, "type S%d = { X S%d; Y S%d }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&src, "type S%d = { V int }\ntype Top = { K node[string]; In S0 }\n", depth)
	src.WriteString("rule main Top = k:\"x\" => Top{K: $k}\nrule read = t:main $$ => $t.In" + strings.Repeat(".X", depth) + ".V\n")
	prog := mustCompile(t, src.String())
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		got := gentest.ResultJSON(prog.ParseWith("read", "x", ParseOptions{Backend: backend, Limits: limits.Limits{Compute: 1 << 12}}))
		if !strings.Contains(got, "0") || strings.Contains(got, "limit") {
			t.Errorf("%s: %s", backend, got)
		}
	}
}
