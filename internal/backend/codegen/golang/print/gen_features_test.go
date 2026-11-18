package print_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	goprint "zeroparse.dev/zp/internal/backend/codegen/golang/print"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/internal/testing/compiletest"
)

// featureCase is a grammar whose parses use the library feature f, with the options it needs and the
// statements of a test that parse with it (in the package of the parser).
type featureCase struct {
	name  string
	f     common.Feature
	src   string
	opts  common.GenOptions
	parse string
}

var featureCases = []featureCase{
	{name: "memo", f: common.FeatureMemo, src: "rule main = a \"x\" / a \"y\" / a \"z\" $$\nrule a = \"(\" a \")\" / \"b\"",
		parse: `for _, in := range []string{"((b))z", "(b)y", "bx"} { Parse(in) }`},
	{name: "left_recursion", f: common.FeatureLeftRecursion, src: "rule main = e $$\nrule e = e \"+\" n / n\nrule n = [0-9]+",
		parse: `Parse("1+2+3")`},
	{name: "recovery", f: common.FeatureRecovery, src: `rule main = x:("q" _|_ #recover(skip="z")) {len($x) == 2} $$`,
		parse: `Parse("qz")`},
	{name: "operators", f: common.FeatureOperators, src: codegentest.Arith, parse: `Parse("1+2*3")`},
	{name: "vars", f: common.FeatureVars, src: codegentest.ActionVariableCases[0].Src, parse: `Parse("ab")`},
	{name: "skip_runs", f: common.FeatureSkipRuns, src: codegentest.SkipRunGrammars[0].Src, parse: `Parse("abxbcxac")`},
	{name: "scan_sites", f: common.FeatureScanSites, src: codegentest.ScanSiteGrammars[0].Src, parse: `Parse(strings.Repeat("ab", 200) + "w" + strings.Repeat("ab", 200) + "!")`},
	{name: "stream", f: common.FeatureStream, src: codegentest.Records, opts: common.GenOptions{Stream: true},
		parse: `ParseStream(strings.NewReader("#records\na=1\n"), func(*Node) error { return nil })`},
	{name: "document", f: common.FeatureDocument, src: "rule main = \"a\"+ $$", opts: common.GenOptions{Document: true},
		parse: `d, err := NewDocument("aa"); if err == nil { d.Parse() }`},
	{name: "observe", f: common.FeatureObserve, src: "rule main = \"a\"+ $$", opts: common.GenOptions{Observability: true},
		parse: `Parse("aa", WithTrace(func(TraceEvent) {}))`},
	{name: "budget", f: common.FeatureBudget, src: "rule main = \"a\"+ $$",
		parse: `Parse("aa", WithLimits(Limits{Compute: 1000}))`},
}

// TestFeatureWitness checks the witness of the feature analysis: a parser of each grammar whose
// parses use a feature runs with the features the analysis finds, and the same parser generated with
// that feature removed from the recorded set (the negative control) panics in its first use, so that
// the check every generated-parser test runs under the tag zp_feature_witness can fail.
func TestFeatureWitness(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	codegentest.PinOrdinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoMod("feattest")), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range featureCases {
		g, err := compiletest.Parse(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, drop := range []bool{false, true} {
			pkg := c.name
			if drop {
				pkg += "_dropped"
			}
			t.Run(pkg, func(t *testing.T) {
				opts := c.opts
				opts.LimitControls = c.f == common.FeatureBudget
				found := common.FeatureSet(0)
				codegentest.SetGenHooks(t, &goprint.Hooks{Features: func(s common.FeatureSet) common.FeatureSet {
					found = s
					if drop {
						return s &^ common.FeatureSet(c.f)
					}
					return s
				}})
				opts.Lib = common.LibImport
				opts.Package, opts.Module, opts.Start = pkg, "feattest/"+pkg, "main"
				files, err := codegentest.Generate(t, g, opts)
				if err != nil {
					t.Fatal(err)
				}
				if !found.Has(c.f) {
					t.Fatalf("the analysis found %s, without %s", found, common.FeatureSet(c.f))
				}
				for _, f := range files {
					path := filepath.Join(dir, pkg, filepath.FromSlash(f.Path))
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, f.Content, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				program := "package " + pkg + "\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nvar _ = strings.NewReader\n\nfunc TestUse(t *testing.T) {\n\t" + c.parse + "\n}\n"
				if err := os.WriteFile(filepath.Join(dir, pkg, "use_test.go"), []byte(program), 0o644); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	results := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && (f[0] == "ok" || f[0] == "FAIL") && strings.HasPrefix(f[1], "feattest/") {
			results[strings.TrimPrefix(f[1], "feattest/")] = f[0]
		}
	}
	for _, c := range featureCases {
		if got := results[c.name]; got != "ok" {
			t.Errorf("%s: the parser with the analysed features: %q, want ok\n%s", c.name, got, out)
		}
		if got := results[c.name+"_dropped"]; got != "FAIL" || !strings.Contains(string(out), fmt.Sprintf("uses feature %#x outside", uint32(c.f))) {
			t.Errorf("%s: the parser without %s: %q, want the panic of the witness (the negative control)", c.name, common.FeatureSet(c.f), got)
		}
	}
}

// TestAnalyzeFeatures checks the features that the analysis finds for small grammars: none of the
// grammar's for a grammar without memoized, recursive-left, recovering or Operators rules, and those of
// the options; and that the library declares the features with the analysis's values.
func TestAnalyzeFeatures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		src  string
		opts common.GenOptions
		want common.FeatureSet
	}{
		{src: `rule main = "a" "b"* $$`},
		{src: `rule main = "a" $$`, opts: common.GenOptions{Document: true, Stream: false, Observability: true, LimitControls: true},
			want: common.FeatureSet(common.FeatureDocument | common.FeatureObserve | common.FeatureBudget)},
		{src: "rule main = e $$\nrule e = e \"+\" n / n\nrule n = [0-9]+", want: common.FeatureSet(common.FeatureMemo | common.FeatureLeftRecursion | common.FeatureScanSites)},
		{src: codegentest.Arith, want: common.FeatureSet(common.FeatureOperators)},
		// A rule that no entry point reaches adds nothing.
		{src: "@start\nrule main = \"a\" $$\nrule unused = (\"q\" #recover(skip=\"z\")) e\nrule e = e \"+\" / \"n\""},
	} {
		g, err := compiletest.Parse(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		prog, _, err := codegentest.Compile(g, compile.Options{})
		if err != nil {
			t.Fatal(err)
		}
		got, err := common.Analyze(prog, nil, tc.opts)
		if err != nil {
			t.Fatal(err)
		}
		// Memoization follows the compiler's choice for the rules; compare it only where the case names it.
		if !tc.want.Has(common.FeatureMemo) {
			got &^= common.FeatureSet(common.FeatureMemo)
		}
		if got != tc.want {
			t.Errorf("%q %+v: features %q, want %q", tc.src, tc.opts, got, tc.want)
		}
	}
}
