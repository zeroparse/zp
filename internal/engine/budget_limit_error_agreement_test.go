//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/limits"
)

// A LimitError of kind LimitCompute reports the reservation that failed and the units reserved before
// it, and every backend reserves the examination of the input one unit at a time. These cases run
// under every compute limit from 1 to max and compare what each backend reports, as "used/requested"
// ("ok" for a parse that succeeds), with the closure backend at the limits of a window in which the
// backends make the same charges: there a backend that reports a run of units as one reservation, or
// splits one reservation into units, disagrees. Outside the windows the charges of the backends differ,
// as the specification allows.
type limitAgreementCase struct {
	name, grammar, input string
	max                  uint64
	// window is the limits at which the engine's backends and generated Go agree; tsWindow those at
	// which generated TypeScript agrees with them too (it charges the encoding of its input otherwise).
	window, tsWindow [2]uint64
	// units is a limit of the window at which the failing reservation is a unit of a run that a backend
	// may make at once: every backend must report the limit as used and one unit requested.
	units uint64
}

var limitAgreementCases = []limitAgreementCase{
	{
		// A literal of four code points, which generated Go reserves with one Take(4).
		name:    "literal",
		grammar: "@start\nrule main = \"abcd\"+",
		input:   "abcdabcdabcd",
		max:     80,
		window:  [2]uint64{1, 42}, tsWindow: [2]uint64{26, 42},
		units: 27,
	},
	{
		// The operators of a level, which the longest match charges as one run of units for each
		// attempt.
		name: "operators",
		grammar: `@start
rule main = #operators {
    operand [0-9]
    level {
        infix left "+"
        infix left "-"
        infix left "*"
        infix left "/"
        infix left "%"
        infix left "^"
    }
}`,
		input:  "1+2-3*4/5%6^7",
		max:    220,
		window: [2]uint64{1, 198}, tsWindow: [2]uint64{26, 198},
		units: 31,
	},
}

func limitReport(err error, used, requested uint64, isLimit bool) string {
	switch {
	case err == nil:
		return "ok"
	case isLimit:
		return fmt.Sprintf("%d/%d", used, requested)
	}
	return "error"
}

// engineLimitReports returns what the backend b reports at each limit from 1 to c.max.
func engineLimitReports(t *testing.T, c limitAgreementCase, b Backend) []string {
	prog := compileBare(t, c.grammar)
	out := make([]string, c.max+1)
	for l := uint64(1); l <= c.max; l++ {
		_, err := prog.ParseWith("main", c.input, ParseOptions{Backend: b, Limits: limits.Limits{Compute: l}})
		var le *limits.LimitError
		ok := errors.As(err, &le)
		if ok {
			out[l] = limitReport(err, le.Used, le.Requested, ok)
		} else {
			out[l] = limitReport(err, 0, 0, false)
		}
	}
	return out
}

// limitAgreementConsumer prints, for each limit, what the generated Parse and ParseValue report.
const limitAgreementConsumer = `package fixture

import (
	"errors"
	"fmt"
	"testing"
)

func report(err error) string {
	var le *LimitError
	switch {
	case err == nil:
		return "ok"
	case errors.As(err, &le):
		return fmt.Sprintf("%d/%d", le.Used, le.Requested)
	}
	return "error"
}

func TestReports(t *testing.T) {
	for l := uint64(1); l <= MAX; l++ {
		_, err := Parse(INPUT, WithLimits(Limits{Compute: l}))
		_, terr := ParseValue(INPUT, WithLimits(Limits{Compute: l}))
		fmt.Printf("REPORT %d %s %s\n", l, report(err), report(terr))
	}
}
`

// generatedGoLimitReports returns what the Parse and the ParseValue of the parser generated from c report
// at each limit from 1 to c.max, with the runtime in the module directory runtime.
func generatedGoLimitReports(t *testing.T, c limitAgreementCase, runtime string) (node, typed []string) {
	g, err := parseTest(c.grammar)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gentest.GenerateCode(t, g, gen.GenOptions{Package: "fixture", Start: "main", Types: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	consumer := strings.NewReplacer("MAX", fmt.Sprint(c.max), "INPUT", fmt.Sprintf("%q", c.input)).Replace(limitAgreementConsumer)
	for name, source := range map[string]string{
		"go.mod":         gentest.GoModAt("fixture", runtime),
		"parser.go":      string(code),
		"parser_test.go": consumer,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", "TestReports", "-v", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	node, typed = make([]string, c.max+1), make([]string, c.max+1)
	for _, line := range strings.Split(string(out), "\n") {
		var l uint64
		var n, ty string
		if _, err := fmt.Sscanf(line, "REPORT %d %s %s", &l, &n, &ty); err == nil && l <= c.max {
			node[l], typed[l] = n, ty
		}
	}
	return node, typed
}

const limitAgreementTSScript = `import { parseWithOptions } from "./parser.ts";
for (let l = 1; l <= MAX; l++) {
  const r = parseWithOptions(INPUT, { limits: { compute: l } });
  const e = r.error;
  const s = e == null ? "ok" : e.kind === "compute-limit" ? e.used + "/" + e.requested : "error";
  console.log("REPORT " + l + " " + s);
}
`

// generatedTSLimitReports returns what the generated TypeScript parser reports at each limit from 1
// to c.max, with patch applied to its bundle.
func generatedTSLimitReports(t *testing.T, c limitAgreementCase, patch func(string) string) []string {
	g, err := parseTest(c.grammar)
	if err != nil {
		t.Fatal(err)
	}
	script := strings.NewReplacer("MAX", fmt.Sprint(c.max), "INPUT", fmt.Sprintf("%q", c.input)).Replace(limitAgreementTSScript)
	bundle := gentest.GenerateTSBundle
	if patch != nil {
		bundle = func(g *grammar.Grammar, opts gen.GenOptions) ([]byte, error) {
			b, err := gentest.GenerateTSBundle(g, opts)
			return []byte(patch(string(b))), err
		}
	}
	out := gentest.RunTSGrammarScriptBundle(t, g, script, gen.GenOptions{Start: "main", LimitControls: true}, bundle)
	reports := make([]string, c.max+1)
	for _, line := range strings.Split(out, "\n") {
		var l uint64
		var s string
		if _, err := fmt.Sscanf(line, "REPORT %d %s", &l, &s); err == nil && l <= c.max {
			reports[l] = s
		}
	}
	return reports
}

// limitDisagreements returns the limits of the window w at which got differs from want.
func limitDisagreements(want, got []string, w [2]uint64) []string {
	var d []string
	for l := w[0]; l <= w[1]; l++ {
		if got[l] != want[l] {
			d = append(d, fmt.Sprintf("limit %d: %s, closure %s", l, got[l], want[l]))
		}
	}
	return d
}

func TestLimitErrorAgreement(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated parsers")
	}
	runtime := gentest.RuntimeCopy()
	for _, c := range limitAgreementCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gentest.NeedsBudgetControls(t)
			want := engineLimitReports(t, c, Closure)
			if c.units != 0 && want[c.units] != fmt.Sprintf("%d/1", c.units) {
				t.Fatalf("closure at limit %d reports %s, want a unit of a run", c.units, want[c.units])
			}
			reports := map[string][]string{
				"bytecode":           engineLimitReports(t, c, Bytecode),
				"bytecode-iterative": engineLimitReports(t, c, BytecodeIterative),
			}
			reports["generated Go"], reports["generated Go typed"] = generatedGoLimitReports(t, c, runtime)
			ts := generatedTSLimitReports(t, c, nil)
			if os.Getenv("ZP_LIMIT_AGREEMENT_DUMP") != "" {
				for l := uint64(1); l <= c.max; l++ {
					t.Logf("%d closure=%s bytecode=%s iterative=%s go=%s typed=%s ts=%s", l, want[l], reports["bytecode"][l], reports["bytecode-iterative"][l], reports["generated Go"][l], reports["generated Go typed"][l], ts[l])
				}
			}
			for name, got := range reports {
				for _, d := range limitDisagreements(want, got, c.window) {
					t.Errorf("%s: %s", name, d)
				}
			}
			for _, d := range limitDisagreements(want, ts, c.tsWindow) {
				t.Errorf("generated TypeScript: %s", d)
			}
		})
	}
}
