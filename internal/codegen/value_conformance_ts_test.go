package codegen_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	xexec "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The operator compositions of generated TypeScript parsers are the printed methods termBinary, termUnary
// and termLogic of the Parser class. Node runs them on every call of the value conformance at once, and
// the Go side compares the printed results with the model's: this is the seed of a conformance runner
// for TypeScript targets.

// valueScript runs the calls of cases.json on the library functions and prints one JSON result per
// call, in order.
const valueScript = `import { readFileSync } from "node:fs";
import * as p from "./parser.ts";
type Enc = { k: string; v?: string | boolean; items?: Enc[] };
const box = (v: p.Value): p.Node => {
  const b = new p.Node(p.typeBox, "", 0, 0, "", p.noNodes, p.noFields, false, false);
  b.box = v;
  return b;
};
const dec = (e: Enc): p.Value => {
  switch (e.k) {
    case "list": return new p.Node("list", "", 0, 0, "", (e.items ?? []).map((x) => (x.k === "nil" ? null : box(dec(x)))), p.noFields, false, false);
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
type Call = { op: string; code: number; args: Enc[] };
const P = Object.create(p.Parser.prototype);
const CONTROLLED = false;
P.control = CONTROLLED ? new p.BudgetControl({ signal: null, budget: 0 } as unknown as p.ResolvedParseOptions) : null;
const calls: Call[] = JSON.parse(readFileSync("cases.json", "utf8"));
const run = (c: Call) => {
  const a = c.args.map(dec);
  switch (c.op) {
    case "binary": {
      const [v, e] = P.termBinary(c.code, a[0], a[1]);
      return e === null ? { v: enc(v) } : { e };
    }
    case "unary": {
      const [v, e] = P.termUnary(c.code, a[0]);
      return e === null ? { v: enc(v) } : { e };
    }
  }
  const [v, e] = P.termLogic(c.code, a[0]);
  if (e === null && typeof v !== "boolean") return { e: "not a bool: " + String(v) };
  return e === null ? { v: { k: "bool", v } } : { e };
};
// An exception of the library is a result that the model does not have.
const out = calls.map((c) => {
  try {
    return run(c);
  } catch (x) {
    return { e: "exception: " + String(x) };
  }
});
console.log(JSON.stringify(out));
// A box counts as the value it holds, which the pool of the calls cannot show: it has no box.
const boxed = (op: number, l: p.Value, r: p.Value): string => {
  try {
    return String(P.termBinary(op, l, r)[1] === null ? P.termBinary(op, l, r)[0] : "error");
  } catch (x) {
    return "exception";
  }
};
console.log(JSON.stringify([boxed(5, box(1), 1), boxed(5, 1, box(1)), boxed(5, box("a"), "b"), boxed(6, box(true), true), boxed(5, box(null), null),
  boxed(5, box(1), null), boxed(0, box(1), 1)]));
`

type tsEnc struct {
	K     string  `json:"k"`
	V     any     `json:"v,omitempty"`
	Items []tsEnc `json:"items,omitempty"`
}

func encodeV(v ref.V) tsEnc {
	switch v.Kind {
	case ref.KList, ref.KTuple:
		e := tsEnc{K: "list"}
		for _, x := range v.Items {
			e.Items = append(e.Items, encodeV(x))
		}
		return e
	case ref.KInt:
		return tsEnc{K: "int", V: strconvI(v.I)}
	case ref.KStr:
		return tsEnc{K: "str", V: v.S}
	case ref.KBool:
		return tsEnc{K: "bool", V: v.B}
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

// tsValues is the TypeScript library's results for all calls.
type tsValues struct {
	t       *testing.T
	results map[string][]any
	boxed   []string
}

// wantBoxed are the results of the calls on boxed operands in valueScript.
var wantBoxed = []string{"true", "true", "false", "false", "true", "false", "error"}

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

// runTSValues runs every call of the value conformance on the TypeScript library, after mutate changed
// its source.
// runTSValues runs every call on the library after mutate changed its source; controlled sets a budget
// control on the parser, which makes the string operations take the paths of a limited parse.
func runTSValues(t *testing.T, controlled bool, mutate func(string) string) tsValues {
	t.Helper()
	node := codegentest.RequireNode(t)
	g, err := parseTest("@start\nrule main = \"a\"\n")
	if err != nil {
		t.Fatal(err)
	}
	code, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(code)
	if mutate != nil {
		src = mutate(src)
	}
	calls := conform.ValueCalls(conform.ValueOps)
	type call struct {
		Op   string  `json:"op"`
		Code int64   `json:"code"`
		Args []tsEnc `json:"args"`
	}
	var in []call
	for _, c := range calls {
		cc := call{Op: c.Op, Code: c.Code}
		for _, a := range c.Args {
			cc.Args = append(cc.Args, encodeV(a))
		}
		in = append(in, cc)
	}
	data, _ := json.Marshal(in)
	dir := t.TempDir()
	for name, content := range map[string]string{"package.json": `{"type": "module"}`, "parser.ts": src, "main.ts": strings.Replace(valueScript, "const CONTROLLED = false;", "const CONTROLLED = "+strconv.FormatBool(controlled)+";", 1), "cases.json": string(data)} {
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
		t.Fatalf("node: %v\n%s", err, codegentest.Clip(stderr.String(), 0))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("node printed %d lines, not 2", len(lines))
	}
	out = []byte(lines[0])
	var boxed []string
	if err := json.Unmarshal([]byte(lines[1]), &boxed); err != nil {
		t.Fatal(err)
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
	v := tsValues{t: t, results: map[string][]any{}, boxed: boxed}
	for i, c := range calls {
		r := results[i]
		errV := ref.Nil
		if r.E != nil {
			errV = ref.Str(*r.E)
		}
		var res []any
		switch {
		case r.V == nil: // a failure: the value is nil, and false for logic
			if c.Op == "logic" {
				res = []any{false, errV}
			} else {
				res = []any{ref.Nil, errV}
			}
		case c.Op == "logic":
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
	for _, controlled := range []bool{false, true} {
		impl := runTSValues(t, controlled, nil)
		s := conform.ValueSuite{Name: "typescript controlled=" + strconv.FormatBool(controlled), Impl: impl, Ops: conform.ValueOps}
		if err := conform.RunValues(s); err != nil {
			t.Error(err)
		}
		if !slices.Equal(impl.boxed, wantBoxed) {
			t.Errorf("controlled=%v: boxed operands: %v, want %v", controlled, impl.boxed, wantBoxed)
		}
	}
}

// TestValueConformanceDetectsBrokenTypeScript is the negative control: with the library functions
// changed in one way each, the conformance run fails.
func TestValueConformanceDetectsBrokenTypeScript(t *testing.T) {
	t.Parallel()
	mutants := map[string][2]string{
		"an addition does not wrap":    {"return intOf(big(l as number | bigint) + big(r as number | bigint));", "return Number(big(l as number | bigint) + big(r as number | bigint));"},
		"strings compare by code unit": {"compareStrings(l as string, r as string)", "((l as string) < (r as string) ? -1 : (l as string) > (r as string) ? 1 : 0)"},
		"nil equals nothing":           {"return [ka === kb, true];", "return [false, true];"},
	}
	for name, m := range mutants {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := conform.ValueSuite{Name: name, Impl: runTSValues(t, false, func(src string) string {
				if !strings.Contains(src, m[0]) {
					t.Fatalf("the library does not contain %q", m[0])
				}
				return strings.Replace(src, m[0], m[1], 1)
			}), Ops: conform.ValueOps}
			if err := conform.RunValues(s); err == nil {
				t.Error("the conformance run did not notice")
			}
		})
	}
}

// mutateTSFunc replaces old by new in the function or method sig of src, the text from sig to the
// closing brace at the same indentation, at the last occurrence if last is set.
func mutateTSFunc(src, sig, old, new string, last bool) (string, bool) {
	i := strings.Index(src, sig)
	if i < 0 {
		return src, false
	}
	// The newline and the indentation of the line of sig.
	line := strings.LastIndex(src[:i], "\n") + 1
	indent := "\n" + src[line:line+len(src[line:])-len(strings.TrimLeft(src[line:], " "))]
	end := strings.Index(src[i:], indent+"}")
	if end < 0 {
		return src, false
	}
	end += i
	body := src[i:end]
	at := strings.Index(body, old)
	if last {
		at = strings.LastIndex(body, old)
	}
	if at < 0 {
		return src, false
	}
	return src[:i] + body[:at] + new + body[at+len(old):] + src[end:], true
}

// TestPrintedOperatorMutantsTypeScript is the proof that the value conformance holds the printed code
// of the operators of terms in the TypeScript library: with the source of each mutant, it fails.
func TestPrintedOperatorMutantsTypeScript(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	t.Parallel()
	mutants := []struct {
		name, sig, old, new string
		last                bool
		controlled          bool
	}{
		{"swapped operands of -", "termBinary(op: number", "intSub(l, r)", "intSub(r, l)", false, false},
		{"a node is equal to a node", "termEqual(l: Value", "return [false, false];", "return [true, true];", true, false},
		{"an int equals a bool", "termEqual(l: Value", "return [false, false];", "return [false, true];", false, false},
		{"an index may be the length", "function intIndex(", "k >= n", "k > n", false, false},
		{"the least index is 1", "function intIndex(", "k < 0", "k < 1", false, false},
		{"a division by zero succeeds", "termBinary(op: number", "(r === 0)", "(false)", false, false},
		{"< is <=", "termBinary(op: number", "intCmp(l, r) < 0", "intCmp(l, r) <= 0", false, false},
		{"strings compare as ints", "termBinary(op: number", "this.strCmp(l, r) < 0", "this.strCmp(l, r) > 0", false, false},
		{"! of a bool does not negate", "termUnary(op: number", "!(x as boolean)", "(x as boolean)", false, false},
		{"a logical operand need not be a bool", "termLogic(op: number", "valKind(x) !== 3", "false", false, false},
		{"a box is not opened", "termEqual(l: Value", "const a = unboxValue(l);", "const a = l;", false, false},
		{"the surrogate fix-up is dropped from the controlled comparison", "function compareStringsSpend(", "x = x >= 0xe000 ? x - 0x800 : x + 0x2000;", "", false, true},
		{"the controlled comparison treats < as <=", "strCmp(l: Value", "compareStringsSpend(l as string, r as string, this.control)", "(compareStringsSpend(l as string, r as string, this.control) < 0 ? -1 : 1)", false, true},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			impl := runTSValues(t, m.controlled, func(src string) string {
				out, ok := mutateTSFunc(src, m.sig, m.old, m.new, m.last)
				if !ok {
					t.Fatalf("the library has no %s with %q", m.sig, m.old)
				}
				return out
			})
			s := conform.ValueSuite{Name: m.name, Impl: impl, Ops: conform.ValueOps}
			if err := conform.RunValues(s); err == nil && slices.Equal(impl.boxed, wantBoxed) {
				t.Error("the conformance run did not notice")
			}
		})
	}
}
