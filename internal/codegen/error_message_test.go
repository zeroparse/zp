package codegen_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine"
)

// errorMessageCases are grammars whose parses of some inputs recover from syntax errors. Typed
// marks the cases that also generate a typed Go parser (ParseValue).
var errorMessageCases = []struct {
	codegentest.Case
	Typed bool
}{
	{codegentest.Case{Name: "recover in a tuple", Src: `rule main = x:"a" e:("b" #recover(skip=";")) $$`,
		Inputs: []string{"a;", "ab", "a"}}, false},
	{codegentest.Case{Name: "messages of @error and several recoveries", Src: `rule main = stmt* $$
rule stmt = (name ";") #recover(skip=[^;]*";")
rule name = [a-z]+ @error(message="a name")`, Inputs: []string{"ab;", "1;cd;", "1;2;", "ab;1"}}, false},
	{codegentest.RecoveredValueCases[1], true},
	{codegentest.RecoveredValueCases[2], true},
}

// errorMessageModes are the three ways a parse records expectations: a parser that was not
// generated to record them (off), one generated to record them whose parse does not ask (avail),
// and one whose parse asks (on). The engine's modes are with and without WithExpectations.
var errorMessageModes = []string{"off", "avail", "on"}

// errorMessages is what one producer reports for one parse: the messages of its syntax errors, and
// those of the error values in its result, in order.
type errorMessages struct {
	Errs []string `json:"errs"`
	Vals []string `json:"vals"`
}

var (
	textErrorValue = regexp.MustCompile(`error\(("(?:[^"\\]|\\.)*")`)
	jsonErrorValue = regexp.MustCompile(`"Message":("(?:[^"\\]|\\.)*")`)
)

// valueMessages returns the messages of the error values in text, which matches re, unquoted.
func valueMessages(t *testing.T, re *regexp.Regexp, text string) []string {
	t.Helper()
	out := []string{}
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		s, err := strconv.Unquote(m[1])
		if err != nil {
			t.Fatalf("message %s: %v", m[1], err)
		}
		out = append(out, s)
	}
	return out
}

func engineMessages(t *testing.T, n *core.Node, err error) errorMessages {
	t.Helper()
	m := errorMessages{Errs: []string{}, Vals: []string{}}
	var list core.SyntaxErrors
	var one *core.SyntaxError
	switch {
	case errors.As(err, &list):
		for _, e := range list {
			m.Errs = append(m.Errs, e.Error())
		}
	case errors.As(err, &one):
		m.Errs = append(m.Errs, one.Error())
	case err != nil:
		m.Errs = append(m.Errs, "other "+err.Error())
	}
	if n != nil {
		m.Vals = valueMessages(t, textErrorValue, n.String())
	}
	return m
}

// TestErrorMessagesAgree checks that the message of a syntax error, and of the error value that a
// recovery puts in the result, is the one text on every producer: the engine's closure, bytecode
// and iterative backends, generated Go (nodes and typed values) and generated TypeScript. Without
// recorded expectations it is `syntax error` after the position; with them the expected items follow.
func TestErrorMessagesAgree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds and runs generated code")
	}
	node := codegentest.RequireNode(t)
	codegentest.EachMode(t, func(t *testing.T) { testErrorMessagesAgree(t, node) })
}

func testErrorMessagesAgree(t *testing.T, node string) {
	// want[mode][case][input] is the closure backend's messages, with and without expectations.
	want := map[string]map[string]errorMessages{}
	for _, mode := range []string{"off", "on"} {
		for _, c := range errorMessageCases {
			g, err := parseTest(c.Src)
			if err != nil {
				t.Fatal(err)
			}
			prog, err := engine.Compile(g, compile.Options{})
			if err != nil {
				t.Fatalf("%s: %v", c.Name, err)
			}
			for _, in := range c.Inputs {
				var first errorMessages
				for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
					n, err := prog.ParseWith("main", in, engine.ParseOptions{Backend: b, Expectations: mode == "on"})
					got := engineMessages(t, n, err)
					if b == engine.Closure {
						first = got
						if want[mode] == nil {
							want[mode] = map[string]errorMessages{}
						}
						want[mode][c.Name+"\x00"+in] = got
					} else if !reflect.DeepEqual(got, first) {
						t.Errorf("%s %q (%s): backend %d reports %v, closure reports %v", c.Name, in, mode, b, got, first)
					}
				}
				if len(first.Errs) == 0 {
					continue
				}
				for _, m := range append(append([]string{}, first.Errs...), first.Vals...) {
					if has := strings.Contains(m, "expected"); has != (mode == "on") && !strings.Contains(c.Src, "@error") {
						t.Errorf("%s %q (%s): the message %q %s expectations", c.Name, in, mode, m, map[bool]string{true: "lists", false: "lacks"}[mode == "on"])
					}
				}
			}
		}
	}
	// The messages with expectations must differ from those without: otherwise the comparison below
	// could not tell the modes apart.
	differs := false
	for k, off := range want["off"] {
		differs = differs || !reflect.DeepEqual(off, want["on"][k])
	}
	if !differs {
		t.Fatal("no input reports different messages with and without expectations")
	}

	goOut := runGoMessages(t)
	tsOut := runTSMessages(t, node)
	checked := 0
	for producer, byMode := range mergeMessages(goOut, tsOut) {
		for mode, byInput := range byMode {
			engineMode := "off"
			if mode == "on" {
				engineMode = "on"
			}
			for key, got := range byInput {
				if w := want[engineMode][key]; !reflect.DeepEqual(got, w) {
					t.Errorf("%s (%s) %q:\n got  %v\n want %v", producer, mode, strings.ReplaceAll(key, "\x00", " / "), got, w)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no generated result compared")
	}

	// Negative control: the generated results of an "on" parse are not those of the engine without
	// expectations, so the comparison above reports a producer that gives the other mode's messages.
	reported := 0
	for _, byMode := range mergeMessages(goOut, tsOut) {
		for key, got := range byMode["on"] {
			if !reflect.DeepEqual(got, want["off"][key]) {
				reported++
			}
		}
	}
	if reported == 0 {
		t.Error("the comparison does not report generated messages that list expectations against the engine's that do not")
	}
}

type producerResults map[string]map[string]map[string]errorMessages // producer, mode, case+input

func mergeMessages(rs ...producerResults) producerResults {
	out := producerResults{}
	for _, r := range rs {
		for p, byMode := range r {
			out[p] = byMode
		}
	}
	return out
}

// runGoMessages generates a Go parser for every case, mode and form, runs them on the inputs and
// returns their messages.
func runGoMessages(t *testing.T) producerResults {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoMod("codegentest")), 0o644); err != nil {
		t.Fatal(err)
	}
	var imports, table strings.Builder
	for i, c := range errorMessageCases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range errorMessageModes {
			for _, typed := range []bool{false, true} {
				if typed && !c.Typed {
					continue
				}
				pkg := fmt.Sprintf("p%d%s", i, mode)
				name, call := "go node", "Parse"
				if typed {
					pkg += "t"
					name, call = "go typed", "ParseValue"
				}
				codegentest.GeneratePackageRaw(t, dir, pkg, g, codegen.GenOptions{Start: "main", Types: typed, Expectations: mode != "off"})
				ask := ""
				if mode == "on" {
					ask = ", " + pkg + ".WithExpectations()"
				}
				fmt.Fprintf(&imports, "\t%s \"codegentest/%s\"\n", pkg, pkg)
				fmt.Fprintf(&table, "\t{%q, %q, %q, func(in string) (any, error) { return %s.%s(in%s) },\n\t\tfunc(v any, err error) []string { var l %[7]s.SyntaxErrors; var e *%[7]s.SyntaxError; var out []string; switch { case errors.As(err, &l): for _, x := range l { out = append(out, x.Error()) }; case errors.As(err, &e): out = append(out, e.Error()); case err != nil: out = append(out, \"other \"+err.Error()) }; return out }},\n",
					name, mode, c.Name, pkg, call, ask, pkg)
			}
		}
	}
	var inputs strings.Builder
	for _, c := range errorMessageCases {
		fmt.Fprintf(&inputs, "{%q, []string{%s}},", c.Name, quoteAll(c.Inputs))
	}
	main := `package main

import (
	"encoding/json"
	"errors"
	"fmt"
` + imports.String() + `)

var table = []struct {
	producer, mode, kase string
	parse func(string) (any, error)
	errs func(any, error) []string
}{
` + table.String() + `}

var inputs = []struct{ name string; in []string }{` + inputs.String() + `}

func main() {
	for _, c := range table {
		for _, k := range inputs {
			if k.name != c.kase {
				continue
			}
			for _, in := range k.in {
				v, err := c.parse(in)
				val := ""
				if c.producer == "go typed" {
					b, _ := json.Marshal(v)
					val = string(b)
				} else {
					val = fmt.Sprint(v)
				}
				errs := c.errs(v, err)
				if errs == nil { errs = []string{} }
				out, _ := json.Marshal(map[string]any{"producer": c.producer, "mode": c.mode, "case": c.kase, "in": in, "errs": errs, "val": val})
				fmt.Println(string(out))
			}
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), codegentest.GoFlagsEnv, "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	res := producerResults{}
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		var r struct {
			Producer, Mode, Case, In, Val string
			Errs                          []string
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		re := textErrorValue
		if r.Producer == "go typed" {
			re = jsonErrorValue
		}
		put(res, r.Producer, r.Mode, r.Case+"\x00"+r.In, errorMessages{Errs: r.Errs, Vals: valueMessages(t, re, r.Val)})
	}
	return res
}

func put(res producerResults, producer, mode, key string, m errorMessages) {
	if res[producer] == nil {
		res[producer] = map[string]map[string]errorMessages{}
	}
	if res[producer][mode] == nil {
		res[producer][mode] = map[string]errorMessages{}
	}
	res[producer][mode][key] = m
}

func quoteAll(ss []string) string {
	var b strings.Builder
	for _, s := range ss {
		fmt.Fprintf(&b, "%q,", s)
	}
	return b.String()
}

// runTSMessages generates a TypeScript parser for every case and mode, runs them on the inputs and
// returns their messages.
func runTSMessages(t *testing.T, _ string) producerResults {
	t.Helper()
	const script = `import { parseWithOptions } from "./parser.ts";
for (const s of INPUTS) {
  const r = parseWithOptions(s, OPTIONS);
  const errs = r.error === null ? [] : "errors" in r.error ? r.error.errors : [r.error];
  console.log(JSON.stringify({ s, node: r.node === null ? "" : String(r.node), errs: errs.map((e: any) => e.message) }));
}
`
	res := producerResults{}
	for _, c := range errorMessageCases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range errorMessageModes {
			options := "undefined"
			if mode == "on" {
				options = "{ expectations: true }"
			}
			js, _ := json.Marshal(c.Inputs)
			s := strings.Replace(strings.Replace(script, "OPTIONS", options, 1), "INPUTS", string(js), 1)
			out := codegentest.RunTSGrammarScriptRaw(t, g, s, codegen.GenOptions{Start: "main", Expectations: mode != "off"})
			lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			if len(lines) != len(c.Inputs) {
				t.Fatalf("%s: %d results for %d inputs\n%s", c.Name, len(lines), len(c.Inputs), out)
			}
			for _, line := range lines {
				var r struct {
					S, Node string
					Errs    []string
				}
				if err := json.Unmarshal([]byte(line), &r); err != nil {
					t.Fatalf("%v: %s", err, line)
				}
				if r.Errs == nil {
					r.Errs = []string{}
				}
				put(res, "ts", mode, c.Name+"\x00"+r.S, errorMessages{Errs: r.Errs, Vals: valueMessages(t, textErrorValue, r.Node)})
			}
		}
	}
	return res
}
