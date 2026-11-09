package engine

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
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
