package engine

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
)

func TestMemberAccessOnJoinTypeErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ src, want string }{
		// No member declares the field.
		{codegentest.JoinMemberRules + "rule main = x:u => $x.Nope", "has no field Nope"},
		{codegentest.JoinMemberRules + "rule main = x:(n / w) => $x.F", "has no field F"},
		// Nested: no member of the join of the first access declares the field.
		{codegentest.JoinMemberRules + "rule main = x:v => $x.In.Nope", "has no field Nope"},
		// A field that only some members declare is rejected, node members included.
		{codegentest.JoinMemberRules + "rule main = x:(n / a) => $x.Start", "A lacks it"},
		{codegentest.JoinMemberRules + "rule main = x:(n / a) => $x.Value", "A lacks it"},
		{codegentest.JoinMemberRules + "rule main = x:u => $x.X", "B lacks it"},
		{codegentest.JoinMemberRules + "rule main = x:(a / n) => $x.F", "N lacks it"},
		{codegentest.JoinMemberRules + "rule main = x:u => $x.X", "A | B has no field X in every member"},
		{codegentest.JoinMemberRules + "rule main = x:u => $x.X", "x.(T).X"},
		// A member that may be nil is rejected.
		{codegentest.JoinMemberRules + "rule main = x:u? => $x.F", "may be nil"},
	} {
		g, err := parseTest(tc.src)
		if err != nil {
			t.Fatalf("%q: %v", tc.src, err)
		}
		if _, err := Compile(g, compile.Options{}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v, want %q", tc.src, err, tc.want)
		}
	}
}

func TestMemberAccessOnJoinAcrossBackends(t *testing.T) {
	t.Parallel()
	for _, fixture := range codegentest.JoinMemberCases() {
		t.Run(fixture.Name, func(t *testing.T) {
			program := mustCompile(t, fixture.Src)
			for _, input := range fixture.Inputs {
				baseline := codegentest.ResultJSON(program.ParseWith("main", input, ParseOptions{Backend: Closure}))
				for _, backend := range []Backend{Bytecode, BytecodeIterative} {
					if actual := codegentest.ResultJSON(program.ParseWith("main", input, ParseOptions{Backend: backend})); actual != baseline {
						t.Fatalf("%s input=%q: %s / %s", backend, input, actual, baseline)
					}
				}
			}
		})
	}
}

func TestMemberAccessOnJoinValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ src, input, want string }{
		{codegentest.JoinMemberRules + "rule main = x:s $$ => $x.F\n", "c", `"c"`},
		{codegentest.JoinMemberRules + "rule main = x:u $$ => $x.F\n", "b", `2`},
		{codegentest.JoinMemberRules + "rule main = x:u $$ => $x.F\n", "a", `"a"`},
		{codegentest.JoinMemberRules + "rule main = x:v $$ => $x.In.F\n", "d", `4`},
		{codegentest.JoinMemberRules + "rule main = x:v $$ => $x.In.F\n", "c", `"ca"`},
		{codegentest.JoinMemberRules + "rule main = x:u $$ => $x.(A).X\n", "a", `1`},
		{codegentest.JoinMemberRules + "rule main = x:u $$ => $x.(A).X\n", "a", `1`},
		{codegentest.JoinMemberRules + "rule main = x:u $$ => $x.(A).X\n", "b", `cannot cast`},
	} {
		program := mustCompile(t, tc.src)
		for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
			if got := codegentest.ResultJSON(program.ParseWith("main", tc.input, ParseOptions{Backend: backend})); !strings.Contains(got, tc.want) {
				t.Errorf("%s %q: %s, want %s", backend, tc.input, got, tc.want)
			}
		}
	}
}
