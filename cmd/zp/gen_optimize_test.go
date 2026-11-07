package main

import (
	"strings"
	"testing"
)

// optimizeGrammar has a choice whose alternatives start with different characters, so the guards
// pass changes the generated parser.
const optimizeGrammar = "@start\nrule main = (word / num / -\" \")* $$\nrule word = \"if\" / \"else\" / [a-z]+\nrule num = [0-9]+\n"

// TestGenOptimize checks the optimization options of zp gen: speed is the default, an optimization that
// is turned off changes the parser, and every unknown name is a usage error that names the valid ones.
func TestGenOptimize(t *testing.T) {
	g := writeFile(t, "opt.zp", optimizeGrammar)
	def, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-runtime", "import")
	if err != nil {
		t.Fatal(err)
	}
	speed, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-runtime", "import", "-optimize", "speed")
	if err != nil || speed != def {
		t.Fatalf("-optimize speed differs from the default: %v", err)
	}
	for _, name := range []string{"guards", "scans", "class-runs", "guards,scans"} {
		without, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-runtime", "import", "-no-opt", name)
		if err != nil {
			t.Fatalf("-no-opt %s: %v", name, err)
		}
		if name == "guards" && without == def {
			t.Errorf("-no-opt guards generated the same parser")
		}
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-optimize", "memory"}, `-optimize: unknown optimization profile "memory" (valid: speed)`},
		{[]string{"-no-opt", "nosuch"}, `-no-opt: unknown optimization "nosuch" (valid: terminals, term-rewrites,`},
		{[]string{"-no-opt", "guards,nosuch"}, `unknown optimization "nosuch"`},
		{[]string{"-print-after", "nosuch"}, `-print-after: unknown pass "nosuch" to print after (valid: build, terminals,`},
	} {
		_, err := runGenGo(t, append([]string{"-g", g}, c.args...)...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want an error with %q", c.args, err, c.want)
		}
	}
	// The generation options do not apply to the runtime alone.
	for _, name := range []string{"-optimize", "-no-opt", "-print-after"} {
		_, err := runCLI(t, "", "gen", "-runtime", "only", "-o", t.TempDir(), name, "speed")
		if err == nil || !strings.Contains(err.Error(), "cannot be combined with -runtime only") {
			t.Errorf("%s with -runtime only: %v", name, err)
		}
	}
}

// TestGenPrintAfter checks that -print-after writes the Core IR after the named passes, and only
// those, to the diagnostic stream, once for each Core module of the generation.
func TestGenPrintAfter(t *testing.T) {
	g := writeFile(t, "opt.zp", optimizeGrammar)
	run := func(list string, extra ...string) string {
		t.Helper()
		var out strings.Builder
		old := genDiagnostics
		genDiagnostics = &out
		defer func() { genDiagnostics = old }()
		if _, err := runGenGo(t, append([]string{"-g", g, "-types", "-typed-only", "-runtime", "import", "-print-after", list}, extra...)...); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	all := run("all")
	for _, want := range []string{"table after build --", "table after term-rewrites --", "table after guards --", "table after scans --", "table after class-runs --"} {
		if !strings.Contains(all, want) {
			t.Errorf("-print-after all lacks %q", want)
		}
	}
	// A parser with limit controls reads the input in ways the lean passes do not preserve, so they do not run.
	controlled := run("all", "-limits")
	if !strings.Contains(controlled, "after scans --") || strings.Contains(controlled, "after terminals --") || strings.Contains(controlled, "after class-runs --") {
		t.Errorf("-print-after all with -limits prints the passes that apply to a lean parser")
	}
	guards := run("guards")
	if strings.Count(guards, "-- Core IR of the") != strings.Count(all, "after guards --") || !strings.Contains(guards, "after guards --") || strings.Contains(guards, "after scans --") {
		t.Errorf("-print-after guards prints %d sections:\n%.300s", strings.Count(guards, "-- Core IR"), guards)
	}
	if got := run("build,scans"); !strings.Contains(got, "after build --") || !strings.Contains(got, "after scans --") || strings.Contains(got, "after guards --") {
		t.Errorf("-print-after build,scans prints the wrong sections")
	}
}
