package gen_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	xexec "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The value primitives of generated TypeScript parsers are the runtime functions rtBinary, rtUnary and
// rtBool. Node runs them on every call of the value conformance at once, and the Go side compares the
// printed results with the model's: this is the seed of a conformance runner for TypeScript targets.

// valueScript runs the calls of cases.json on the runtime functions and prints one JSON result per
// call, in order.
const valueScript = `import { readFileSync } from "node:fs";
import * as p from "./parser.ts";
type Enc = { k: string; v?: string | boolean };
const dec = (e: Enc): p.Value => {
  switch (e.k) {
    case "nil": return null;
    case "int": return p.intOf(BigInt(e.v as string));
    case "str": return e.v as string;
  }
  return e.v as boolean;
};
const enc = (v: p.Value): Enc => {
  if (v === null || v === undefined) return { k: "nil" };
  if (typeof v === "number" || typeof v === "bigint") return { k: "int", v: String(v) };
  if (typeof v === "string") return { k: "str", v };
  if (typeof v === "boolean") return { k: "bool", v };
  throw new Error("not a scalar");
};
type Call = { op: string; operator: string; args: Enc[] };
const calls: Call[] = JSON.parse(readFileSync("cases.json", "utf8"));
const out = calls.map((c) => {
  const a = c.args.map(dec);
  try {
    switch (c.op) {
      case "val.binary": return { v: enc(p.rtBinary(c.operator, a[0], a[1])) };
      case "val.unary": return { v: enc(p.rtUnary(c.operator, a[0])) };
    }
    return { v: { k: "bool", v: p.rtBool(c.operator, a[0]) } };
  } catch (e) {
    if (e instanceof p.EvalError) return { e: e.msg };
    throw e;
  }
});
console.log(JSON.stringify(out));
`

type tsEnc struct {
	K string `json:"k"`
	V any    `json:"v,omitempty"`
}

func encodeV(v ref.V) tsEnc {
	switch v.Kind {
	case ref.KInt:
		return tsEnc{"int", strconvI(v.I)}
	case ref.KStr:
		return tsEnc{"str", v.S}
	case ref.KBool:
		return tsEnc{"bool", v.B}
	}
	return tsEnc{K: "nil"}
}

func strconvI(i int64) string { return strconv.FormatInt(i, 10) }

func decodeV(e tsEnc) ref.V {
	switch e.K {
	case "int":
		i, err := strconv.ParseInt(e.V.(string), 10, 64)
		if err != nil {
			// Not an int of 64 bits: a value that equals none of the model's.
			return ref.V{Kind: ref.KInt, S: "not an int64: " + e.V.(string)}
		}
		return ref.Int(i)
	case "str":
		return ref.Str(e.V.(string))
	case "bool":
		return ref.Bool(e.V.(bool))
	}
	return ref.Nil
}

// tsValues is the TypeScript runtime's results for all calls.
type tsValues struct {
	t       *testing.T
	results map[string][]any
}

func (tsValues) Datum(v ref.V) any { return v }
func (tsValues) Canon(x any) ref.V { return x.(ref.V) }

func (v tsValues) Call(o *xexec.Op, args []any) []any {
	c := conform.ValueCall{Op: o.Name, Code: args[0].(int64)}
	for _, a := range args[1:] {
		c.Args = append(c.Args, a.(ref.V))
	}
	res, ok := v.results[c.Key()]
	if !ok {
		v.t.Fatalf("no result for %s", c.Key())
	}
	return res
}

// runTSValues runs every call of the value conformance on the TypeScript runtime, after mutate changed
// its source.
func runTSValues(t *testing.T, mutate func(string) string) tsValues {
	t.Helper()
	node := gentest.RequireNode(t)
	g, err := parseTest("@start\nrule main = \"a\"\n")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gentest.GenerateTSBundle(g, gen.GenOptions{Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(code)
	if mutate != nil {
		src = mutate(src)
	}
	calls := conform.ValueCalls(conform.ValueOps)
	type call struct {
		Op       string  `json:"op"`
		Operator string  `json:"operator"`
		Args     []tsEnc `json:"args"`
	}
	var in []call
	for _, c := range calls {
		cc := call{Op: c.Op, Operator: c.Operator()}
		for _, a := range c.Args {
			cc.Args = append(cc.Args, encodeV(a))
		}
		in = append(in, cc)
	}
	data, _ := json.Marshal(in)
	dir := t.TempDir()
	for name, content := range map[string]string{"package.json": `{"type": "module"}`, "parser.ts": src, "main.ts": valueScript, "cases.json": string(data)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(node, "main.ts")
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, gentest.Clip(stderr.String(), 0))
	}
	var results []struct {
		V *tsEnc  `json:"v"`
		E *string `json:"e"`
	}
	if err := json.Unmarshal(out, &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != len(calls) {
		t.Fatalf("%d results for %d calls", len(results), len(calls))
	}
	v := tsValues{t: t, results: map[string][]any{}}
	for i, c := range calls {
		r := results[i]
		errV := ref.Nil
		if r.E != nil {
			errV = ref.Str(*r.E)
		}
		var res []any
		switch {
		case r.V == nil: // a failure: the value is nil, and false for val.bool
			if c.Op == "val.bool" {
				res = []any{false, errV}
			} else {
				res = []any{ref.Nil, errV}
			}
		case c.Op == "val.bool":
			res = []any{decodeV(*r.V).B, errV}
		default:
			res = []any{decodeV(*r.V), errV}
		}
		v.results[c.Key()] = res
	}
	return v
}

func TestValueConformanceTypeScript(t *testing.T) {
	t.Parallel()
	s := conform.ValueSuite{Name: "typescript", Impl: runTSValues(t, nil), Ops: conform.ValueOps}
	if err := conform.RunValues(s); err != nil {
		t.Fatal(err)
	}
}

// TestValueConformanceDetectsBrokenTypeScript is the negative control: with the runtime functions
// changed in one way each, the conformance run fails.
func TestValueConformanceDetectsBrokenTypeScript(t *testing.T) {
	t.Parallel()
	mutants := map[string][2]string{
		"an addition does not wrap":    {"return intOf(big(l) + big(r));", "return Number(big(l) + big(r));"},
		"strings compare by code unit": {"return compareStrings(l, r) < 0;", "return l < r;"},
		"nil equals nothing":           {"return (a === null || a === undefined) && (b === null || b === undefined);", "return false;"},
	}
	for name, m := range mutants {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := conform.ValueSuite{Name: name, Impl: runTSValues(t, func(src string) string {
				if !strings.Contains(src, m[0]) {
					t.Fatalf("the runtime does not contain %q", m[0])
				}
				return strings.Replace(src, m[0], m[1], 1)
			}), Ops: conform.ValueOps}
			if err := conform.RunValues(s); err == nil {
				t.Error("the conformance run did not notice")
			}
		})
	}
}
