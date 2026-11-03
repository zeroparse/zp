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

func TestFreeIdents(t *testing.T) {
	t.Parallel()
	got := strings.Join(gogen.FreeIdents("x = p.v3 + f(\"v4\") // v5\n/* v6 */ y := v7\n"), " ")
	if want := "x p f y v7"; got != want {
		t.Errorf("freeIdents = %q, want %q (a selector, a string and a comment do not count)", got, want)
	}
}
