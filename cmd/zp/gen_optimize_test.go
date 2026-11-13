package main

import (
	"os"
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
	def, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-lib", "import")
	if err != nil {
		t.Fatal(err)
	}
	speed, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-lib", "import", "-optimize", "speed")
	if err != nil || speed != def {
		t.Fatalf("-optimize speed differs from the default: %v", err)
	}
	spaced, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-lib", "import", "-no-opt", "guards, scans")
	if err != nil {
		t.Fatalf("-no-opt with spaces after the comma: %v", err)
	}
	if plain, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-lib", "import", "-no-opt", "guards,scans"); err != nil || plain != spaced {
		t.Errorf("-no-opt \"guards, scans\" differs from \"guards,scans\": %v", err)
	}
	for _, name := range []string{"guards", "scans", "class-runs", "guards,scans"} {
		without, err := runGenGo(t, "-g", g, "-types", "-typed-only", "-lib", "import", "-no-opt", name)
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
		{[]string{"-optimize", "nosuch"}, `-optimize: unknown optimization profile "nosuch" (valid: speed, memory, size)`},
		{[]string{"-no-opt", "nosuch"}, `-no-opt: unknown optimization "nosuch" (valid: terminals, text-dispatch,`},
		{[]string{"-no-opt", "guards,nosuch"}, `unknown optimization "nosuch"`},
		{[]string{"-print-after", "nosuch"}, `-print-after: unknown pass "nosuch" to print after (valid: build, terminals,`},
	} {
		_, err := runGenGo(t, append([]string{"-g", g}, c.args...)...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want an error with %q", c.args, err, c.want)
		}
	}
	// The generation options do not apply to the library alone.
	for _, name := range []string{"-optimize", "-no-opt", "-print-after"} {
		_, err := runCLI(t, "", "gen", "-lib", "only", "-o", t.TempDir(), name, "speed")
		if err == nil || !strings.Contains(err.Error(), "cannot be combined with -lib only") {
			t.Errorf("%s with -lib only: %v", name, err)
		}
	}
}

// TestGenPrintAfter checks that -print-after writes the Core IR after the named passes, and only
// those, to the diagnostic stream, once for each Core module of the generation, and the Flow IR of
// each function printed from it after flow.
func TestGenPrintAfter(t *testing.T) {
	g := writeFile(t, "opt.zp", optimizeGrammar)
	run := func(list string, extra ...string) string {
		t.Helper()
		var out strings.Builder
		old := genDiagnostics
		genDiagnostics = &out
		defer func() { genDiagnostics = old }()
		if _, err := runGenGo(t, append([]string{"-g", g, "-types", "-typed-only", "-lib", "import", "-print-after", list}, extra...)...); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	all := run("all")
	for _, want := range []string{"table after build --", "table after guards --", "table after scans --", "table after class-runs --"} {
		if !strings.Contains(all, want) {
			t.Errorf("-print-after all lacks %q", want)
		}
	}
	// A parser with limit controls reads the input in ways the lean passes do not preserve, so they do not run.
	controlled := run("all", "-limits")
	if !strings.Contains(controlled, "after scans --") || strings.Contains(controlled, "after terminals --") || strings.Contains(controlled, "after class-runs --") {
		t.Errorf("-print-after all with -limits prints the passes that apply to a lean parser")
	}
	// A pass that does not apply prints nothing, however it is asked for.
	if got := run("terminals", "-limits"); got != "" {
		t.Errorf("-print-after terminals with -limits printed:\n%.300s", got)
	}
	guards := run("guards")
	if strings.Count(guards, "-- Core IR of the") != strings.Count(all, "after guards --") || !strings.Contains(guards, "after guards --") || strings.Contains(guards, "after scans --") {
		t.Errorf("-print-after guards prints %d sections:\n%.300s", strings.Count(guards, "-- Core IR"), guards)
	}
	if got := run("build,scans"); !strings.Contains(got, "after build --") || !strings.Contains(got, "after scans --") || strings.Contains(got, "after guards --") || strings.Contains(got, "-- Flow IR") {
		t.Errorf("-print-after build,scans prints the wrong sections")
	}
	// flow prints each function that the parser is printed from, as the lowering made it, and no Core IR.
	flowIR := run("flow")
	if !strings.Contains(flowIR, "-- Flow IR of ") || !strings.Contains(flowIR, " in the typed table after flow --\nfunc ") || strings.Contains(flowIR, "-- Core IR") {
		t.Errorf("-print-after flow prints the wrong sections:\n%.300s", flowIR)
	}
	if n := strings.Count(all, " table after flow --"); n == 0 || n != strings.Count(flowIR, "-- Flow IR of ") {
		t.Errorf("-print-after all prints %d lowered Flow functions, flow %d", n, strings.Count(flowIR, "-- Flow IR of "))
	}
	// exec prints each function printed from Exec IR, as lowered from Flow IR, and nothing else.
	execIR := run("exec")
	if !strings.Contains(execIR, "-- Exec IR of ") || !strings.Contains(execIR, " in the typed table after exec --\n") || strings.Contains(execIR, "-- Core IR") || strings.Contains(execIR, "-- Flow IR") {
		t.Errorf("-print-after exec prints the wrong sections:\n%.300s", execIR)
	}
	if n := strings.Count(all, " table after exec --"); n == 0 || n != strings.Count(execIR, "-- Exec IR of ") {
		t.Errorf("-print-after all prints %d lowered Exec functions, exec %d", n, strings.Count(execIR, "-- Exec IR of "))
	}
}

// TestGenOpt checks -opt: it gives a value to an optimization after the profile and before -no-opt,
// and a name that does not exist, a count that is not a number and a value the optimization does
// not take are usage errors.
func TestGenOpt(t *testing.T) {
	g := writeFile(t, "opt.zp", optimizeGrammar)
	base := []string{"-g", g, "-types", "-typed-only", "-lib", "import"}
	gen := func(extra ...string) string {
		t.Helper()
		out, err := runGenGo(t, append(append([]string{}, base...), extra...)...)
		if err != nil {
			t.Fatalf("%v: %v", extra, err)
		}
		return out
	}
	def := gen()
	// A value that is the default changes nothing; a different one changes the parser.
	if got := gen("-opt", "class-tables=0, inline-literals=4"); got != def {
		t.Errorf("-opt with the default values changed the parser")
	}
	off := gen("-opt", "inline-literals=0")
	if off == def {
		t.Errorf("-opt inline-literals=0 generated the same parser")
	}
	// -no-opt is applied after -opt and wins.
	if got := gen("-opt", "inline-literals=2", "-no-opt", "inline-literals"); got != off {
		t.Errorf("-no-opt did not win over -opt")
	}
	// A name alone turns an optimization on; the value 0 turns it off, as -no-opt does.
	if got := gen("-opt", "guards=0"); got != gen("-no-opt", "guards") {
		t.Errorf("-opt guards=0 differs from -no-opt guards")
	}
	if got := gen("-opt", "guards"); got != def {
		t.Errorf("-opt guards changed an optimization that is on")
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-opt", "nosuch"}, `-opt: unknown optimization "nosuch" (valid: terminals, text-dispatch,`},
		{[]string{"-opt", "guards,nosuch=2"}, `-opt: unknown optimization "nosuch"`},
		{[]string{"-opt", "class-tables=x"}, `-opt:`},
		{[]string{"-opt", "guards=2"}, `-opt: optimization guards does not take the value 2`},
		{[]string{"-opt", "class-tables=-1"}, `-opt: optimization class-tables does not take the value -1`},
		{[]string{"-opt", "guards,guards"}, `-opt:`},
		{[]string{"-opt", "guards,,scans"}, `-opt: empty optimization name`},
	} {
		_, err := runGenGo(t, append([]string{"-g", g}, c.args...)...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want an error with %q", c.args, err, c.want)
		}
	}
	_, err := runCLI(t, "", "gen", "-lib", "only", "-o", t.TempDir(), "-opt", "guards")
	if err == nil || !strings.Contains(err.Error(), "cannot be combined with -lib only") {
		t.Errorf("-opt with -lib only: %v", err)
	}
}

// TestGenOptTypeScript checks that -opt reaches the TypeScript generator.
func TestGenOptTypeScript(t *testing.T) {
	g := writeFile(t, "lit.zp", "@start\nrule main = (\"alpha\" / \"beta\" / \"gamma\" / \"delta\" / \"alpha\" \"beta\" / \"gamma\" \"delta\")+ $$\n")
	read := func(extra ...string) string {
		t.Helper()
		dir, err := runGenTS(t, append([]string{"-g", g}, extra...)...)
		if err != nil {
			t.Fatalf("%v: %v", extra, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return readTS(t, dir, names...)
	}
	def := read()
	shared := read("-opt", "ts-literal-sharing")
	if shared == def {
		t.Errorf("-opt ts-literal-sharing generated the same TypeScript parser")
	}
	if got := read("-opt", "ts-literal-sharing=0"); got != def {
		t.Errorf("-opt ts-literal-sharing=0 differs from the default")
	}
	if got := read("-opt", "ts-literal-sharing", "-no-opt", "ts-literal-sharing"); got != def {
		t.Errorf("-no-opt did not win over -opt")
	}
}
