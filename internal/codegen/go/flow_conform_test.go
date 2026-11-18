package gogen_test

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	gogen "zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec/ref"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// The conformance of the Go target to the Flow operations. Each call of a primitive operation that
// the Go printer prints is wrapped in a probe (testdata/flowconform/probe.go, added to a copy of
// the library) that records the parser state around it: the position, the units of budget charged,
// the height of the stack of gathered values, the recovered errors and the silent depth, with the
// arguments and results that the checks read. The generated parsers of flowConformCases run on their
// inputs, and every recorded call is checked against its operation's declaration:
//
//   - an operation that does not write the position leaves it unchanged;
//   - the units it charges itself, those of the probed calls inside it aside, stay within its cost
//     bound (ops.go), the quantities taken from its immediate; the operations of flowExactCharges
//     charge exactly their bound;
//   - the check of its case (flowConformChecks) holds: the reads of the input agree with the input
//     model (ref.Input), and the operations on the stack, the recovered errors, the marks and the
//     silent depth change the state as their contracts say.
//
// Every Flow operation has a case: a call of it in the generated parsers of flowConformCases that
// ran, or for flow.declareNames, which has no Go form, its printing as nothing. The operations of
// flowOtherTargets are not printed for Go, which the test checks instead.

// flowConformCase is a grammar, the options it is generated with and the inputs its parser runs on,
// through each of its entries: Go statements that parse in with the options opts and set err, by
// default flowConformParse.
type flowConformCase struct {
	name    string
	src     string
	opts    codegen.GenOptions
	inputs  []string
	entries []string
	// noLimits generates the parser without limit controls: the ordinary form, whose calls charge
	// nothing.
	noLimits bool
}

const (
	flowConformParse    = "_, err = Parse(in, opts...)"
	flowConformOrdinary = "_, err = Parse(in, ordinary...)"
)

const flowConformLong = "abcdefghijklmnopqrstuvwxyzabcdefghij"

var flowConformCases = []flowConformCase{
	{name: "plain", src: `@start
rule main = ^ item ("," item)* ","? &$ $$
rule item = [a-z]+ / "x" &"y" "y" / !"q" [0-9] / "if"i ws / "\u{fffd}x"
rule ws = [ \n]* $
`, inputs: []string{"abc,x", "ab,7,IF", flowConformLong + ",b", "a,q", "", "a,,", "\ufffdx"},
		entries: []string{flowConformParse, flowConformOrdinary}},
	{name: "typed", src: `type P = { Name string; N int }
@start
rule main = xs:(p:pair ";"?)* $$ => map($xs, (e) => e.$p)
rule pair = n:name "=" v:num {len(text($n)) > 0} => P{Name: text($n), N: len(text($v))}
rule name = [a-z]+ #atomic
rule num = d:[0-9]+ => text($d)
`, opts: codegen.GenOptions{Types: true}, inputs: []string{"a=1;bc=23", "a=", flowConformLong + "=1", ""},
		entries: []string{flowConformParse, "_, err = ParseValue(in, opts...)"}},
	{name: "lean", src: `type Item = { Key Word; Vals []int; Sub Sub }
type Sub = { N int }
type Word = node[string]
@start
rule main = items:(it:item -";"?)* tail:(t:num "!")* $$ => map($items, (e) => e.$it)
rule item = (k:key "=" vs:(v:num -","?)+ {len(text($k)) < 40} => Item{Key: $k, Vals: map($vs, (e) => e.$v), Sub: Sub{N: 1}}) #recover(skip=(![;] .)*)
rule key Word = [a-zA-Z0-9_+*/%.]+ #atomic
rule num int = d:[0-9]+ => len(text($d))
`, opts: codegen.GenOptions{Types: true, TypedOnly: true, Recognize: true}, inputs: []string{"a=1,2;b.c=3", "a=;b=1", flowConformLong + "=1,", "", "a=1;1!22!"},
		entries: []string{"_, err = ParseValue(in, opts...)", "err = Recognize(in, opts...)", "_, err = ParseValue(in, ordinary...)",
			"err = Recognize(in, ordinary...)"}},
	{name: "ordinary", src: `@start
rule main = (stmt ";")* $$
rule stmt = -([a-z] [0-9] [a-z]) "!" / (c:[a-zA-Z0-9_+*/%.] "#" => text($c)) / -kw "(" / -kw "[" / !kw id
rule kw = ([aA] ([bB] ([oO] [rR] [tT] / [sS] [oO] [lL] [uU] [tT] [eE]) / [cC] [tT] [iI] [oO] [nN] / [lL] [lL]) / [cC] ([aA] [lL] [lL] ([eE] [dD])? / [hH] [eE] [cC] [kK]) / [dD] [oO] / [a-e] [xX] / [eE] [nN] [dD] / [fF] [oO] [rR] / "q"? [zZ] [qQ]) ![a-z_]
rule id = [a-z_]+ #atomic
`, noLimits: true, opts: codegen.GenOptions{Types: true, TypedOnly: true, Recognize: true, Opt: map[string]int{"class-tables": 2}},
		inputs:  []string{"abort(;ab;a1b!;x#;do[;", "call(;", "a1;", ""},
		entries: []string{"_, err = ParseValue(in, ordinary...)", "err = Recognize(in, ordinary...)"}},
	{name: "bind", src: `var k int
@start
rule main = {k := 0} ({k := 1} "w")? !({k := 2} "q") item* end $$
rule item = "x" {k := k + 1} "!" / "x" {k := k + 1} / "y" {k < 3} / ("z" "z") @diagnostic(name="zz")
rule end = ";"?
`, inputs: []string{"xxy;", "wx!xxxy", "zz", "z", "w"},
		opts: codegen.GenOptions{Types: true}, entries: []string{flowConformParse, "_, err = ParseValue(in, opts...)"}},
	{name: "stream", src: `@start
rule main = (item ";")* #stream $$
rule item = [a-z]+
`, opts: codegen.GenOptions{Stream: true}, inputs: []string{"ab;c;", "a;b", ""},
		entries: []string{flowConformParse, "err = ParseStream(strings.NewReader(in), func(*Node) error { return nil }, opts...)"}},
	{name: "recover", src: `@start
rule main = stmt* $$
rule stmt = ("let" ws name "=" [0-9]+ ";") #recover(skip=(![;] .)* ";") ws
rule name = [a-z]+
rule ws = [ ]*
`, inputs: []string{"let a=1;let b=;let c=2;", "let=;", "let x=1;"}},
	{name: "operators", src: `@start
rule main = expr $$
rule expr = #operators {
    operand [0-9]+
    level {
        infix left "+"
    }
    level {
        prefix "-"
    }
}
`, inputs: []string{"1+2+-3", "1+", "-"}},
	{name: "leftrec", src: `@start
rule main = sum $$
rule sum = sum "+" n:[0-9] ps:(a:[a-z] b:[0-9])* / [0-9]
`, opts: codegen.GenOptions{Types: true}, inputs: []string{"1+2+3", "1+", "1+2a1b2+3"},
		entries: []string{flowConformParse, "_, err = ParseValue(in, opts...)"}},
	{name: "observe", src: `@start
rule main = (kw / id)* $$
rule kw = "if" / "else"
rule id = [a-z]+ ";"
`, opts: codegen.GenOptions{Observability: true}, inputs: []string{"if;else", "abc;if"}},
}

// flowOtherTargets are the operations that the Go printer never prints, with the reason; the test
// checks that none of its cases prints them.
var flowOtherTargets = map[string]string{
	flow.OpCallOutline: "only the TypeScript generator outlines nodes (flowlower.Options.Outline)",
}

// flowExactCharges are the operations whose charges equal their cost bound.
var flowExactCharges = []string{flow.OpStep}

// flowProbe wraps the calls of primitive operations in probes and records what each one calls.
type flowProbe struct {
	mu    sync.Mutex
	calls []*code.Call
	// printed are the operations whose Go form is nothing.
	empty map[string]bool
}

// flowBodyProbes matches the first line of each generated function that takes the parser: the body of
// a rule, a frame, a line or an outlined node. A probe of its own (K -1) wraps it, so that the units of
// a body count as the body's, not as those of the call that runs it.
var flowBodyProbes = regexp.MustCompile(`(?m)^func \w+\(p \*rt\.T?Parser[^)]*\) [^{\n]*\{\n`)

// firstParser matches the first use of the parser p in a printed call.
var firstParser = regexp.MustCompile(`(^|[^.\w])p\b`)

// flowArgOps are the operations whose arguments the probes record, for their checks. The arguments of
// others are not wrapped: flow.append assigns to its first.
var flowArgOps = []string{flow.OpRestore, flow.OpDropKids, flow.OpKids, flow.OpInClass, flow.OpGuardFails, flow.OpText,
	flow.OpLiteralText, flow.OpReset, flow.OpMarkPos, flow.OpMergeExpected, flow.OpMergeRenamed, flow.OpKeep, flow.OpMakeError,
	flow.OpErrorValue, flow.OpNewErrorNode}

func (pr *flowProbe) add(c *code.Call) int {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.calls = append(pr.calls, c)
	return len(pr.calls) - 1
}

func (pr *flowProbe) target(t *gogen.Target, pre *string) *gogen.Target {
	w := *t
	ops := flowOps()
	wrapArgs := func(c *code.Call, args []string) []string {
		if !slices.Contains(flowArgOps, execlower.FlowOp(c.Callee)) {
			return args
		}
		out := make([]string, len(args))
		for i, a := range args {
			out[i] = fmt.Sprintf("rt.ConfArg(%d, %s)", i, a)
		}
		return out
	}
	w.Op = func(c *code.Call, args []string) string {
		name := execlower.FlowOp(c.Callee)
		o := ops[name]
		if o == nil {
			return t.Op(c, args)
		}
		k := pr.add(c)
		mark := len(*pre)
		s := t.Op(c, wrapArgs(c, args))
		switch {
		case s == "":
			pr.mu.Lock()
			pr.empty[name] = true
			pr.mu.Unlock()
			return ""
		case len(o.Results) == 0:
			return fmt.Sprintf("rt.ConfPre(p, %d)\n%s\nrt.ConfPost()", k, s)
		case len(o.Results) == 1 && len(*pre) > mark:
			// The statements of the term run inside the call.
			*pre = (*pre)[:mark] + fmt.Sprintf("rt.ConfPre(p, %d)\n", k) + (*pre)[mark:]
			return fmt.Sprintf("rt.Conf1(nil, %s)", s)
		case len(o.Results) == 1:
			return fmt.Sprintf("rt.Conf1(rt.ConfPre(p, %d), %s)", k, s)
		}
		loc := firstParser.FindStringSubmatchIndex(s)
		if loc == nil {
			panic("flow probe: no parser in the form of " + name + ": " + s)
		}
		at := loc[3]
		s = s[:at] + fmt.Sprintf("rt.ConfPre(p, %d)", k) + s[at+1:]
		return fmt.Sprintf("rt.ConfPop%d(%s)", len(o.Results), s)
	}
	w.Negate = func(c *code.Call, args []string) (string, bool) {
		o := ops[execlower.FlowOp(c.Callee)]
		if o == nil || t.Negate == nil {
			if t.Negate == nil {
				return "", false
			}
			return t.Negate(c, args)
		}
		k := pr.add(c)
		s, ok := t.Negate(c, wrapArgs(c, args))
		if !ok {
			return "", false
		}
		return fmt.Sprintf("rt.ConfNot(rt.ConfPre(p, %d), %s)", k, s), true
	}
	w.Stmt = func(c *code.Call, args []string, dst []string, define bool) (string, bool) {
		if ops[execlower.FlowOp(c.Callee)] == nil || t.Stmt == nil {
			if t.Stmt == nil {
				return "", false
			}
			return t.Stmt(c, args, dst, define)
		}
		k := pr.add(c)
		s, ok := t.Stmt(c, wrapArgs(c, args), dst, define)
		if !ok || s == "" {
			return s, ok
		}
		res := make([]string, len(dst))
		for i, d := range dst {
			res[i] = d
			if d == "_" || strings.HasPrefix(s, "{") {
				res[i] = "nil"
			}
		}
		return fmt.Sprintf("rt.ConfPre(p, %d)\n%s\nrt.ConfPost(%s)", k, s, strings.Join(res, ", ")), true
	}
	return &w
}

var flowOps = sync.OnceValue(func() map[string]*code.Op {
	m := map[string]*code.Op{}
	for _, o := range flow.Ops() {
		m[o.Name] = o
	}
	return m
})

// flowEvent is a call that a probe recorded (testdata/flowconform/probe.go).
type flowEvent struct {
	K                   int
	Pos, PosAfter       int
	Used, UsedAfter     uint64
	Nested              uint64
	Kids, KidsAfter     int
	Rec, RecAfter       int
	Silent, SilentAfter int
	Args                map[int]any
	Results             []any
	Done                bool
}

// flowRun is the record of a parse of an input.
type flowRun struct {
	Input  string
	Err    string
	Events []flowEvent
}

func TestFlowOperationsConform(t *testing.T) {
	codegentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated parsers")
	}
	probeSrc, err := os.ReadFile(filepath.Join("testdata", "flowconform", "probe.go"))
	if err != nil {
		t.Fatal(err)
	}
	lib := codegentest.LibCopyWith(t, nil, map[string]string{"zz_probe.go": string(probeSrc)})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoModAt("flowconf", lib)), 0o644); err != nil {
		t.Fatal(err)
	}
	probes := map[string]*flowProbe{}
	for _, c := range flowConformCases {
		g, err := parseTest(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		pr := &flowProbe{empty: map[string]bool{}}
		probes[c.name] = pr
		opts := c.opts
		opts.Lib, opts.LimitControls, opts.KeepComments, opts.Expectations = codegen.LibImport, !c.noLimits, true, true
		opts.Package, opts.Module = c.name, "flowconf/"+c.name
		files, err := gogen.GenerateParser(g, codegentest.Compile, codegentest.WithLib(opts), &gogen.Hooks{FlowTarget: pr.target})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, f := range files {
			path := filepath.Join(dir, c.name, filepath.FromSlash(f.Path))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			content := f.Content
			if f.Path == "parser.go" {
				content = flowBodyProbes.ReplaceAll(content, []byte("$0\trt.ConfPre(p, -1)\n\tdefer rt.ConfPost()\n"))
			}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		inputs, _ := json.Marshal(c.inputs)
		entries := c.entries
		if entries == nil {
			entries = []string{flowConformParse}
		}
		var cases strings.Builder
		for i, e := range entries {
			fmt.Fprintf(&cases, "\t\t\tcase %d:\n\t\t\t\t%s\n", i, e)
		}
		test := fmt.Sprintf(`package %s

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	rt "zeroparse.dev/zp/codegen/go/lib"
	"zeroparse.dev/zp/core"
)

func TestProbe(t *testing.T) {
	var inputs []string
	if err := json.Unmarshal([]byte(%q), &inputs); err != nil {
		t.Fatal(err)
	}
	type run struct {
		Input  string
		Err    string
		Events []*rt.ConfEvent
	}
	var runs []run
	opts := []ParseOption{rt.WithExpectations(), rt.WithLimits(core.Limits{Compute: 1 << 40})}
	ordinary := []ParseOption{rt.WithExpectations()}
	for _, in := range inputs {
		for entry := range %d {
			rt.ConfReset()
			var err error
			switch entry {
%s			}
			r := run{Input: in, Events: rt.ConfEvents}
			if err != nil {
				r.Err = err.Error()
			}
			runs = append(runs, r)
		}
	}
	_, _, _ = strings.NewReader, opts, ordinary
	data, err := json.Marshal(runs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(%q, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
`, c.name, inputs, len(entries), cases.String(), filepath.Join(dir, c.name+".json"))
		if err := os.WriteFile(filepath.Join(dir, c.name, "probe_test.go"), []byte(test), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("probed parsers: %v\n%s", err, out)
	}
	covered := map[string]int{}
	for _, c := range flowConformCases {
		pr := probes[c.name]
		for name := range pr.empty {
			covered[name]++
		}
		for _, call := range pr.calls {
			if name := execlower.FlowOp(call.Callee); flowOtherTargets[name] != "" {
				covered[name]++
			}
		}
		data, err := os.ReadFile(filepath.Join(dir, c.name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var runs []flowRun
		if err := json.Unmarshal(data, &runs); err != nil {
			t.Fatal(err)
		}
		for _, r := range runs {
			in := ref.NewInput(r.Input)
			for _, e := range r.Events {
				if e.K < 0 {
					continue
				}
				call := pr.calls[e.K]
				name := execlower.FlowOp(call.Callee)
				if !e.Done {
					continue
				}
				covered[name]++
				for _, msg := range checkFlowCall(in, flowOps()[name], call, &e) {
					t.Errorf("%s %q: %s at %d: %s", c.name, r.Input, name, e.Pos, msg)
				}
			}
		}
	}
	for _, o := range flow.Ops() {
		if why, other := flowOtherTargets[o.Name]; other {
			if covered[o.Name] != 0 {
				t.Errorf("the Go printer printed %s: %s", o.Name, why)
			}
			continue
		}
		if covered[o.Name] == 0 {
			t.Errorf("no case calls %s", o.Name)
		}
	}
}

// flowConformExps bounds the expectations of the grammars of flowConformCases, which bounds the
// entries that recording one expectation compares at the farthest failure.
const flowConformExps = 16

// flowUnbounded is the value of a quantity that the bound does not check.
const flowUnbounded = 1 << 30

// flowBound returns the cost bound of the recorded call e of the operation o, the call c on the input
// in, with the values of its quantities:
//
//   - lit, classes: of the immediate;
//   - body: none for the rules and outlined nodes that a call runs, whose bodies are probed calls of
//     their own, and for the term code of a predicate, binding or action, which the probes do not
//     wrap, any (term_charges_test.go checks the charges of terms);
//   - terminal: any, checked by the case of flow.skipTerminal;
//   - merge: each expectation that the call records or is passed, at most once more for each entry
//     recorded at the farthest failure that it compares;
//   - call: the constant of the first-call record (seen.first of the Exec stage), the growth of the
//     memo's slots up to the position and the expectations that the memo keeps and merges;
//   - vals: the length of the list that the call returns;
//   - error: each expectation, the steps of sorting them and finding the line, and the input once;
//   - error_text: the bytes of the message and one for each of its parts;
//   - ranges: of the immediate; captures: its names, and capture_fields their square;
//   - stream: four for each code point of the input and a constant;
//   - text: the length of the result.
func flowBound(in *ref.Input, o *code.Op, c *code.Call, e *flowEvent) int {
	exps := 1
	for _, i := range []int{0, 1} {
		if n, ok := listLen(e.Args[i]); ok {
			exps = n
		}
	}
	n := o.Cost.Const
	for _, t := range o.Cost.Terms {
		q := 0
		switch t.Quantity {
		case flow.QLit:
			q = len([]rune(c.Imm.(*core.Lit).Text))
		case flow.QClasses:
			q = len(c.Imm.(*core.ClassRun).Classes)
		case flow.QTerminal:
			q = flowUnbounded
		case flow.QBody:
			if o.Name != flow.OpCallRule && o.Name != flow.OpCallOutline {
				q = flowUnbounded
			}
		case flow.QMerge:
			q = exps * (1 + flowConformExps)
		case flow.QCall:
			q = 66 + 2*(e.Pos+1) + 2*(1+flowConformExps)*flowConformExps
		case flow.QVals:
			q, _ = listLen(firstResult(e))
		case flow.QError:
			q = exps*(2+2*bits.Len(uint(exps))) + in.Len() + bits.Len(uint(in.Len())) + 1
		case flow.QRanges:
			q = len(c.Imm.(*core.Class).Ranges)
		case flow.QCaptures:
			q = len(c.Imm.(flow.Attach).Names.List)
		case flow.QCaptureFields:
			q = len(c.Imm.(flow.Attach).Names.List)
			q *= q
		case flow.QStream:
			q = 4*(in.Len()+1) + 64
		case flow.QText:
			if start, ok := argInt(e, 0); ok {
				q = e.Pos - start
			}
		case flow.QErrorText:
			if msg, ok := e.Args[1].(string); ok {
				q = len(msg) + strings.Count(msg, ", ") + 4
			}
		default:
			panic("flow conformance: no value of the quantity " + t.Quantity)
		}
		n += t.Coef * q
	}
	return n
}

// checkFlowCall returns what the recorded call e of the operation o, the call c, breaks.
func checkFlowCall(in *ref.Input, o *code.Op, c *code.Call, e *flowEvent) []string {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	if !slices.Contains(o.Writes, flow.Pos) && e.PosAfter != e.Pos {
		fail("moved the position to %d without writing it", e.PosAfter)
	}
	self := int(e.UsedAfter-e.Used) - int(e.Nested)
	if bound := flowBound(in, o, c, e); self > bound {
		fail("charged %d units, more than its bound %d", self, bound)
	} else if slices.Contains(flowExactCharges, o.Name) && self != bound {
		fail("charged %d units, not its bound %d", self, bound)
	}
	if check := flowConformChecks[o.Name]; check != nil {
		check(in, c, e, fail)
	}
	return errs
}

func argInt(e *flowEvent, i int) (int, bool) {
	v, ok := e.Args[i].(float64)
	return int(v), ok
}

func resBool(e *flowEvent, i int) (bool, bool) {
	if i >= len(e.Results) {
		return false, false
	}
	v, ok := e.Results[i].(bool)
	return v, ok
}

func resInt(e *flowEvent, i int) (int, bool) {
	if i >= len(e.Results) {
		return 0, false
	}
	v, ok := e.Results[i].(float64)
	return int(v), ok
}

func markPos(x any) (int, bool) {
	s, ok := x.(string)
	if !ok {
		return 0, false
	}
	var n int
	_, err := fmt.Sscanf(s, "mark:%d", &n)
	return n, err == nil
}

func listLen(x any) (int, bool) {
	s, ok := x.(string)
	if !ok {
		return 0, false
	}
	var n int
	_, err := fmt.Sscanf(s, "len:%d", &n)
	return n, err == nil
}

func classOf(c *core.Class) ref.Ranges {
	r := ref.Ranges{Negated: c.Negated}
	for _, x := range c.Ranges {
		r.Ranges = append(r.Ranges, [2]rune{x.Lo, x.Hi})
	}
	return r
}

type failFunc func(format string, args ...any)

// flowConformChecks are the checks of the Flow operations beyond their position and charges, by
// operation.
var flowConformChecks = map[string]func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc){
	flow.OpPeek: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		r, size, ok := in.At(e.Pos)
		if got, known := resBool(e, 2); known && got != ok {
			fail("reports a code point %v, the input %v", got, ok)
		}
		if !ok {
			return
		}
		if got, known := resInt(e, 0); known && rune(got) != r {
			fail("read %q, the input has %q", rune(got), r)
		}
		if got, known := resInt(e, 1); known && got != size {
			fail("read size %d, want %d", got, size)
		}
	},
	flow.OpMatchLit: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		lit := c.Imm.(*core.Lit)
		want := in.HasPrefix(e.Pos, lit.Text, lit.Fold)
		if got, known := resBool(e, 1); known && got != want {
			fail("matched %v, the input %v", got, want)
		}
		end := e.Pos
		if want {
			end += len([]rune(lit.Text))
		}
		if e.PosAfter != end {
			fail("ended at %d, want %d", e.PosAfter, end)
		}
	},
	flow.OpHasLit: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		lit := c.Imm.(*core.Lit)
		if got, known := resBool(e, 0); known && got && !in.HasPrefix(e.Pos, lit.Text, lit.Fold) {
			fail("holds where the input does not begin with %q", lit.Text)
		}
	},
	flow.OpMatchClass: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		want := in.MatchClass(e.Pos, classOf(c.Imm.(*core.Class)))
		if got, known := resBool(e, 0); known && got != want {
			fail("matched %v, the input %v", got, want)
		}
		end := e.Pos
		if want {
			end++
		}
		if e.PosAfter != end {
			fail("ended at %d, want %d", e.PosAfter, end)
		}
	},
	flow.OpInClass: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		ch, ok := argInt(e, 0)
		got, known := resBool(e, 0)
		if ok && known && got != classOf(c.Imm.(*core.Class)).Contains(rune(ch)) {
			fail("reports %v for %q", got, rune(ch))
		}
	},
	flow.OpGuardFails: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		g := c.Imm.(*core.First)
		got, known := resBool(e, 0)
		more, mok := e.Args[1].(bool)
		if !known || !mok {
			return
		}
		admits := false
		if ch, ok := argInt(e, 0); more && ok {
			r := rune(ch)
			if g.Lit {
				admits = r == g.Rune || g.Fold && r == g.Rune-('a'-'A')
			} else {
				admits = classOf(g.Class).Contains(r)
			}
		} else if more {
			return
		}
		if got && admits {
			fail("skips an alternative that the character can begin")
		}
		if !got && !admits && g.Depth == 0 {
			fail("keeps an alternative that the character cannot begin")
		}
	},
	flow.OpAtAnchor: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		want := in.AtAnchor(e.Pos, int(c.Imm.(core.AnchorKind)))
		if got, known := resBool(e, 0); known && got != want {
			fail("reports %v, the input %v", got, want)
		}
	},
	flow.OpSkipClasses: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		run := c.Imm.(*core.ClassRun)
		matched := 0
		for matched < len(run.Classes) && in.MatchClass(e.Pos+matched, classOf(run.Classes[matched])) {
			matched++
		}
		want := matched == len(run.Classes)
		if got, known := resBool(e, 0); known && got != want {
			fail("matched %v, the input %v", got, want)
		}
		// On failure the position is at the class that did not match.
		end := e.Pos + matched
		if e.PosAfter != end {
			fail("ended at %d, want %d", e.PosAfter, end)
		}
	},
	flow.OpSkipTerminal: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if ok, known := resBool(e, 0); known && !ok && e.PosAfter != e.Pos {
			fail("failed and moved the position")
		}
		// The terminal program charges a unit for each code point it reads, at most one past its match.
		if self := int(e.UsedAfter-e.Used) - int(e.Nested); self > e.PosAfter-e.Pos+1 {
			fail("charged %d units for a match of %d", self, e.PosAfter-e.Pos)
		}
	},
	flow.OpText: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		start, ok := argInt(e, 0)
		if s, isStr := firstString(e); ok && isStr && s != in.Slice(start, e.Pos) {
			fail("returned %q, the input %q", s, in.Slice(start, e.Pos))
		}
	},
	flow.OpLiteralText: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		start, ok := argInt(e, 0)
		if s, isStr := firstString(e); ok && isStr && s != in.Slice(start, e.Pos) {
			fail("returned %q, the input %q", s, in.Slice(start, e.Pos))
		}
	},
	flow.OpStr: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if s, isStr := firstString(e); isStr && s != string(c.Imm.(flow.Chars)) {
			fail("returned %q", s)
		}
	},
	flow.OpAdvance: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if n, ok := c.Args[0].(*code.Const); ok && e.PosAfter-e.Pos != int(n.Value.(int64)) {
			fail("moved by %d, not %v", e.PosAfter-e.Pos, n.Value)
		}
		if e.PosAfter < e.Pos || e.PosAfter > in.Len() {
			fail("moved to %d", e.PosAfter)
		}
	},
	flow.OpRestore: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if pos, ok := argInt(e, 0); ok && e.PosAfter != pos {
			fail("moved to %d, not %d", e.PosAfter, pos)
		}
		if rec, ok := argInt(e, 1); ok && e.RecAfter != min(rec, e.Rec) {
			fail("kept %d recovered errors of %d, not the first %d", e.RecAfter, e.Rec, rec)
		}
	},
	flow.OpMarkState: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if len(e.Results) > 0 {
			if pos, ok := markPos(e.Results[0]); ok && pos != e.Pos {
				fail("marked %d", pos)
			}
		}
	},
	flow.OpMarkPos: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		pos, ok := markPos(e.Args[0])
		if got, known := resInt(e, 0); ok && known && got != pos {
			fail("returned %d for a mark at %d", got, pos)
		}
	},
	flow.OpReset: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if pos, ok := markPos(e.Args[0]); ok && e.PosAfter != pos {
			fail("returned to %d for a mark at %d", e.PosAfter, pos)
		}
	},
	flow.OpPushKid: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if e.KidsAfter != e.Kids+1 {
			fail("left %d gathered values of %d", e.KidsAfter, e.Kids)
		}
	},
	flow.OpDropKids: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if base, ok := argInt(e, 0); ok && e.KidsAfter != base {
			fail("left %d gathered values, not %d", e.KidsAfter, base)
		}
	},
	flow.OpKidsLen: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if got, known := resInt(e, 0); known && got != e.Kids {
			fail("returned %d of %d", got, e.Kids)
		}
	},
	flow.OpKids: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		base, ok := argInt(e, 0)
		if !ok {
			return
		}
		if e.KidsAfter != base {
			fail("left %d gathered values, not %d", e.KidsAfter, base)
		}
		if n, ok := listLen(firstResult(e)); ok && n != e.Kids-base {
			fail("returned %d values of %d above %d", n, e.Kids, base)
		}
	},
	flow.OpNewVals: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if n, ok := listLen(firstResult(e)); ok && n != 0 {
			fail("returned %d values", n)
		}
	},
	flow.OpRecLen: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if got, known := resInt(e, 0); known && got != e.Rec {
			fail("returned %d of %d", got, e.Rec)
		}
	},
	flow.OpAppendError: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if e.RecAfter != e.Rec+1 {
			fail("left %d recovered errors of %d", e.RecAfter, e.Rec)
		}
	},
	flow.OpSilent: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if d := c.Imm.(flow.Silence).Delta; e.SilentAfter-e.Silent != d {
			fail("changed the silent depth by %d, not %d", e.SilentAfter-e.Silent, d)
		}
	},
	flow.OpCallRule: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if ok, known := resBool(e, 1); known && !ok && e.PosAfter != e.Pos {
			fail("failed and moved the position")
		}
	},
	flow.OpCallOutline: func(in *ref.Input, c *code.Call, e *flowEvent, fail failFunc) {
		if ok, known := resBool(e, 1); known && !ok && e.PosAfter != e.Pos {
			fail("failed and moved the position")
		}
	},
}

func firstResult(e *flowEvent) any {
	if len(e.Results) == 0 {
		return nil
	}
	return e.Results[0]
}

func firstString(e *flowEvent) (string, bool) {
	s, ok := firstResult(e).(string)
	if !ok || !strings.HasPrefix(s, "s:") {
		return "", false
	}
	return s[2:], true
}
