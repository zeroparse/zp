package gogen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gogen"
	"zeroparse.dev/zp/internal/gentest"
)

// The tests of generated code catch a wrong decision of which variables are unread: one that treats
// every variable as unread, and one that ignores the reads of the assignments that stay.
func TestFlowVarsMutantFailsToCompile(t *testing.T) {
	t.Parallel()
	for name, h := range map[string]*gogen.Hooks{"every variable unread": {FlowVarsMutant: true}, "assignment reads ignored": {FlowVarsMutantUses: true}} {
		t.Run(name, func(t *testing.T) {
			gentest.SetGenHooks(t, h)
			testFlowVarsMutantFailsToCompile(t)
		})
	}
}

func testFlowVarsMutantFailsToCompile(t *testing.T) {
	g, err := parseTest("rule main = (kw / id)+ !.\nrule kw = \"select\"i / \"from\"i / \"x\"\nrule id = [a-z]+")
	if err != nil {
		t.Fatal(err)
	}
	files, err := gentest.Generate(t, g, gen.GenOptions{Package: "p", Start: "main", Runtime: gen.RuntimeImport, Module: "example.com/p", Types: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(localErrors(t, files)) == 0 {
		t.Error("no compile error with every variable treated as unread")
	}
}

// TestExecVarsCheck checks that a printed body that names a local that is not declared is an
// internal error, and that one whose names are declared, or that only a selector or a string
// names, is not.
func TestExecVarsCheck(t *testing.T) {
	t.Parallel()
	if err := gogen.ExecVarsError("x = v3 + 1\n", "v3"); err == nil || !strings.Contains(err.Error(), "v3") {
		t.Errorf("an undeclared local named by the body: error %v, want one that names v3", err)
	}
	if err := gogen.ExecVarsError("x = p.v3 + f(\"v3\")\n", "v3"); err != nil {
		t.Errorf("a selector and a string: error %v, want none", err)
	}
	if err := gogen.ExecVarsError("x = v3 + 1\n"); err != nil {
		t.Errorf("a declared local: error %v, want none", err)
	}
}

func TestFreeIdents(t *testing.T) {
	t.Parallel()
	got := strings.Join(gogen.FreeIdents("x = p.v3 + f(\"v4\") // v5\n/* v6 */ y := v7\n"), " ")
	if want := "x p f y v7"; got != want {
		t.Errorf("freeIdents = %q, want %q (a selector, a string and a comment do not count)", got, want)
	}
}
