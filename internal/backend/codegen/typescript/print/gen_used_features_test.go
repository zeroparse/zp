package print_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/internal/testing/compiletest"
)

// TestUsedFeaturesTSParity compares parsers that carry their own copy of the library, narrowed to
// their features, with the engine on codegentest.UsedFeatureCases, plain, with limit controls,
// with types and with Documents, and does the same with the optimization used-features off, whose
// copy is the whole library. Each narrowed copy is type-checked with the strictest options (when
// tsc is found), so this also checks that narrowing leaves no dangling name and no unused local.
// With limit controls, the narrowed copy must also charge the units that the whole library charges
// for each input (the subtest limits/charges). The default tier runs the narrowed copies, the long
// tier the whole library too.
func TestUsedFeaturesTSParity(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	modes := []struct {
		name string
		opts common.GenOptions
	}{
		{"plain", common.GenOptions{}},
		{"limits", common.GenOptions{LimitControls: true}},
		{"types", common.GenOptions{Types: true}},
		{"document", common.GenOptions{Document: true}},
	}
	for _, mode := range modes {
		for _, off := range []bool{false, true} {
			if off && !codegentest.Long() {
				continue
			}
			name := mode.name
			if off {
				name += "/no-used-features"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				if off {
					o.NoOpt = []string{common.UsedFeatures}
				}
				codegentest.TSParsersCorpus(t, node, codegentest.UsedFeatureCases(), o)
			})
		}
	}
	t.Run("limits/charges", func(t *testing.T) {
		t.Parallel()
		cases := codegentest.UsedFeatureCases()
		narrowed := tsSpentUnits(t, node, cases, common.GenOptions{LimitControls: true})
		whole := tsSpentUnits(t, node, cases, common.GenOptions{LimitControls: true, NoOpt: []string{common.UsedFeatures}})
		if len(narrowed) != len(whole) {
			t.Fatalf("got %d results narrowed, %d whole", len(narrowed), len(whole))
		}
		charged := false
		for i := range narrowed {
			charged = charged || !strings.Contains(whole[i], ": spent 0 ")
			if narrowed[i] != whole[i] {
				t.Errorf("narrowed copy %s\nwhole library  %s", narrowed[i], whole[i])
			}
		}
		if !charged {
			t.Fatal("no parse charged a unit: the comparison checks nothing")
		}
	})
}

// tsSpentUnits generates the TypeScript parser of each case with opts, which must have limit
// controls, parses each input that is valid UTF-8 as a string in both units under a compute limit
// that no input reaches, and returns, per parse, the case, input, unit and the compute units that
// the parse charged (the sum of the units passed to BudgetControl.charge and chargeUnits).
func tsSpentUnits(t *testing.T, node string, cases []codegentest.Case, opts common.GenOptions) []string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"type": "module"}`+"\n")
	var inputs [][]string
	for i, c := range cases {
		g, err := compiletest.Parse(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		o := opts
		o.Start = "main"
		code, err := codegentest.GenerateTSBundle(g, o)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		write(fmt.Sprintf("g%d.ts", i), string(code))
		var in []string
		for _, s := range c.Inputs {
			if utf8.ValidString(s) {
				in = append(in, s)
			}
		}
		inputs = append(inputs, in)
	}
	data, _ := json.Marshal(inputs)
	write("inputs.json", string(data))
	write("spent.mjs", tsSpentHarness)
	cmd := exec.Command(node, "spent.mjs", "inputs.json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if e, ok := err.(*exec.ExitError); ok {
			stderr = e.Stderr
		}
		t.Fatalf("node: %v\n%s", err, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	var res []string
	k := 0
	for i, c := range cases {
		for _, s := range inputs[i] {
			for _, unit := range []string{"codepoints", "bytes"} {
				if k >= len(lines) {
					t.Fatalf("got %d results, want more", len(lines))
				}
				res = append(res, fmt.Sprintf("%s: input %q, %s: %s", c.Name, s, unit, lines[k]))
				k++
			}
		}
	}
	if k != len(lines) {
		t.Fatalf("got %d results, want %d", len(lines), k)
	}
	return res
}

// tsSpentHarness parses the inputs of inputs.json with the parsers g<i>.ts in a worker with a large
// stack and prints, per parse, the compute units charged and the error, if any. The worker hands
// its output to the main thread, which writes it and lets the process end by itself: ending it
// with process.exit when the worker exits can cut off output that a slow reader has not yet drained.
const tsSpentHarness = tsWorkerMain + `
  const inputs = JSON.parse(readFileSync(workerData.inputs, "utf8"));
  const lines = [];
  for (let i = 0; i < inputs.length; i++) {
    const m = await import("./g" + i + ".ts");
    const proto = m.BudgetControl.prototype;
    let spent = 0;
    for (const name of ["charge", "chargeUnits"]) {
      const f = proto[name];
      proto[name] = function (n) { spent += n; return f.call(this, n); };
    }
    for (const input of inputs[i]) {
      for (const unit of [m.CodePoints, m.Bytes]) {
        spent = 0;
        let err;
        try {
          const r = m.parseWithOptions(input, { unit, limits: { compute: 2 ** 40 } });
          err = r.error === null ? "" : r.error.message;
        } catch (x) {
          err = "throw " + String(x);
        }
        lines.push("spent " + spent + " " + JSON.stringify(err));
      }
    }
  }
  parentPort.postMessage(lines.join("\n") + "\n");
}
`

// tsWorkerMain is the start of a harness that runs its body in a worker thread with a large stack:
// the main thread writes what the worker posts to the standard output, and sets the exit status
// instead of calling process.exit, so that the output is written out before the process ends. The
// body continues the else branch and ends it.
const tsWorkerMain = `import { Worker, isMainThread, workerData, parentPort } from "node:worker_threads";
import { readFileSync } from "node:fs";

if (isMainThread) {
  const w = new Worker(new URL(import.meta.url), { workerData: { inputs: process.argv[2] }, resourceLimits: { stackSizeMb: 1024 } });
  w.on("message", (s) => process.stdout.write(s));
  w.on("error", (e) => { console.error(e); process.exitCode = 1; });
  w.on("exit", (code) => { if (code !== 0) process.exitCode = code; });
} else {`

// TestTSWorkerMainDrainsSlowReader runs a worker that posts about 400 KB to a reader that starts
// reading late, far beyond the capacity of a pipe, and checks that every line arrives.
func TestTSWorkerMainDrainsSlowReader(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	const lines = 20000
	dir := t.TempDir()
	harness := tsWorkerMain + `
  const lines = [];
  for (let i = 0; i < ` + fmt.Sprint(lines) + `; i++) lines.push("spent " + i + " \"\"");
  parentPort.postMessage(lines.join("\n") + "\n");
}
`
	if err := os.WriteFile(filepath.Join(dir, "spent.mjs"), []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "spent.mjs", "unused.json")
	cmd.Dir = dir
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	data, err := io.ReadAll(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("node: %v", err)
	}
	if got := strings.Count(string(data), "\n"); got != lines {
		t.Errorf("got %d lines from a reader that started late, want %d", got, lines)
	}
}
