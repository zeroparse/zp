package engine

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
	ir "zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// parseTest parses ZeroParse source for a test whose subject is not start rules: unless the source
// has declaration attributes, every rule becomes a start rule, the first one the default.
func parseTest(src string) (*grammar.Grammar, error) {
	g, err := syntax.Parse(src)
	return starttest.MarkAll(g), err
}

// mustCompile compiles ZeroParse source with source information, so that results show rule names and
// capture labels.
func mustCompile(t *testing.T, src string, opts ...compile.Options) *Program {
	t.Helper()
	var o compile.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	o.SourceInfo = true
	return compileBare(t, src, o)
}

// compileBare compiles ZeroParse source with the options as given: without source information unless
// they request it.
func compileBare(t *testing.T, src string, opts ...compile.Options) *Program {
	t.Helper()
	g, err := parseTest(src)
	if err != nil {
		t.Fatalf("syntax error: %v\n%s", err, src)
	}
	var o compile.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	prog, err := Compile(g, o)
	if err != nil {
		t.Fatalf("compile error: %v\n%s", err, src)
	}
	return prog
}

// compileError returns the compile error message.
func compileError(t *testing.T, src string) string {
	t.Helper()
	g, err := parseTest(src)
	if err != nil {
		t.Fatalf("syntax error: %v\n%s", err, src)
	}
	_, err = Compile(g, compile.Options{})
	if err == nil {
		t.Fatalf("expected a compile error\n%s", src)
	}
	return err.Error()
}

// result returns the parse result as an S-expression, or "error: ..." on error.
func result(prog *Program, input string) string {
	return resultWith(prog, input, ParseOptions{Expectations: true})
}

func resultWith(prog *Program, input string, o ParseOptions) string {
	n, err := prog.ParseWith("main", input, o)
	if err != nil {
		return "error: " + err.Error()
	}
	return n.String()
}

type parseCase struct {
	input, want string
}

// check checks pairs of inputs and expected results against the grammar src. It also checks
// that the results are the same with and without memoization, on every backend, and with and
// without tracing.
func check(t *testing.T, src string, cases ...parseCase) {
	t.Helper()
	prog := mustCompile(t, src)
	noMemo := mustCompile(t, src, compile.Options{DisableMemo: true})
	for _, c := range cases {
		got := result(prog, c.input)
		if got != c.want && !(strings.HasPrefix(c.want, "error: ") && strings.Contains(got, c.want[len("error: "):]) && strings.HasPrefix(got, "error: ")) {
			t.Errorf("input %q\n got  %s\n want %s", c.input, got, c.want)
		}
		if got2 := result(noMemo, c.input); got2 != got {
			t.Errorf("input %q: result differs without memoization\n memo    %s\n no memo %s", c.input, got, got2)
		}
		checkBackends(t, prog, "main", c.input)
		checkBackends(t, noMemo, "main", c.input)
		checkUnits(t, prog, "main", c.input, Closure)
		checkUnits(t, prog, "main", c.input, Bytecode)
		checkUnits(t, prog, "main", c.input, BytecodeIterative)
		checkTrace(t, prog, "main", c.input, ParseOptions{Expectations: true})
		checkTrace(t, noMemo, "main", c.input, ParseOptions{Expectations: true})
	}
}

func ok(input, want string) parseCase { return parseCase{input, want} }

// fails expects a syntax error. want is a string contained in the error message.
func fails(input, want string) parseCase {
	return parseCase{input, "error: " + want}
}

func TestMain(m *testing.M) {
	// Tests build generated programs in fresh temporary directories, with a build cache private
	// to the test process (SetGoCache) so that their builds, which no later run reuses, do not
	// accumulate in the shared cache. -trimpath makes the entries independent of the directory,
	// so the tests of the package share the builds of the standard library and of this module;
	// commands that set GOFLAGS add it themselves. -mod=mod lets the temporary modules that
	// replace this module resolve it without a go.sum.
	gentest.SetGoFlags()
	removeGoCache := gentest.SetGoCache()
	code := m.Run()
	removeGoCache()
	if names := gentest.UnwrappedFixtureTests(); len(names) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: tests build generated fixtures without choosing their form: %s\nWrap the test body in gentest.EachMode, or call gentest.NeedsBudgetControls, or pin the form with gentest.PinOrdinary\n", strings.Join(names, ", "))
		code = 1
	}
	os.Exit(code)
}

func lines(s ...string) string { return strings.Join(s, "\n") }

func mustParse(src string) *grammar.Grammar {
	g, err := parseTest(src)
	if err != nil {
		panic(err)
	}
	return g
}

// checkUnits checks that the result of parsing in bytes, with positions converted to code
// points, matches the result of parsing in code points.
func checkUnits(t *testing.T, prog *Program, start, input string, b Backend) {
	t.Helper()
	n1, err1 := prog.ParseWith(start, input, ParseOptions{Expectations: true, Unit: parse.CodePoints, Backend: b})
	n2, err2 := prog.ParseWith(start, input, ParseOptions{Expectations: true, Unit: parse.Bytes, Backend: b})
	n2, err2 = toCodePoints(input, n2, err2)
	if a, b := gentest.ResultJSON(n1, err1), gentest.ResultJSON(n2, err2); a != b {
		t.Errorf("input %q: results differ between units\n codepoints %s\n bytes      %s", input, a, b)
	}
}

// checkBackends checks that every backend produces the same result (value, positions, errors).
func checkBackends(t *testing.T, prog *Program, start, input string) {
	t.Helper()
	for _, u := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
		n1, err1 := prog.ParseWith(start, input, ParseOptions{Expectations: true, Unit: u})
		want := gentest.ResultJSON(n1, err1)
		checkRecognize(t, prog, start, input, u, err1)
		// The default budget runs the controlled form; Unlimited runs the uncontrolled one, which must
		// give the same result on inputs within the default.
		if n0, err0 := prog.ParseWith(start, input, ParseOptions{Expectations: true, Unit: u, Limits: parse.Limits{Compute: parse.Unlimited}}); !errors.Is(err1, parse.ErrLimit) {
			if got := gentest.ResultJSON(n0, err0); got != want {
				t.Errorf("input %q (unit %v): Unlimited differs from the default budget\n default   %s\n unlimited %s", input, u, want, got)
			}
		}
		for _, b := range []Backend{Bytecode, BytecodeIterative} {
			n2, err2 := prog.ParseWith(start, input, ParseOptions{Expectations: true, Unit: u, Backend: b})
			if got := gentest.ResultJSON(n2, err2); got != want {
				t.Errorf("input %q (unit %v): %v differs from closure\n closure %s\n %v %s", input, u, b, want, b, got)
			}
		}
	}
}

// checkRecognize checks that a parse that builds no tree (recognition) returns the same syntax
// error as an ordinary parse (or succeeds if it does). Results are not compared when the
// ordinary parse fails with a runtime error in an action (recognition does not evaluate
// actions).
func checkRecognize(t *testing.T, prog *Program, start, input string, u parse.PositionUnit, want error) {
	t.Helper()
	switch want.(type) {
	case nil, *parse.SyntaxError, parse.SyntaxErrors:
	default:
		return
	}
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		n, err := prog.ParseWith(start, input, ParseOptions{Expectations: true, Unit: u, Backend: b, Recognize: true})
		if n != nil || fmt.Sprint(err) != fmt.Sprint(want) {
			t.Errorf("input %q (unit %v, %v): recognition gives %v, %v; want %v", input, u, b, n, err, want)
		}
	}
}

// toCodePoints converts the positions in a result parsed in bytes to code points.
func toCodePoints(input string, n *parse.Node, err error) (*parse.Node, error) {
	// Map byte offsets to code point positions (a byte in the middle of a character maps to that
	// character's position).
	cp := make([]int, len(input)+1)
	i := 0
	for b := 0; b < len(input); i++ {
		_, size := utf8.DecodeRuneInString(input[b:])
		for k := 0; k < size; k++ {
			cp[b+k] = i
		}
		b += size
	}
	cp[len(input)] = i
	runes := []rune(input)
	lineCol := func(pos int) (int, int) {
		line, col := 1, 1
		for _, r := range runes[:min(pos, len(runes))] {
			if r == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
		return line, col
	}
	// Convert the column (in bytes) of "line:col: ..." to code points.
	fixMessage := func(msg string) string {
		var line, col int
		var rest string
		if k, _ := fmt.Sscanf(msg, "%d:%d:", &line, &col); k != 2 {
			return msg
		}
		rest = msg[strings.Index(msg[strings.Index(msg, ":")+1:], ":")+strings.Index(msg, ":")+1:]
		off := 0
		for l := 1; l < line; l++ {
			off += strings.Index(input[off:], "\n") + 1
		}
		_, c := lineCol(cp[off+col-1])
		return fmt.Sprintf("%d:%d%s", line, c, rest)
	}
	seen := map[*parse.Node]*parse.Node{}
	var conv func(*parse.Node) *parse.Node
	conv = func(n *parse.Node) *parse.Node {
		if n == nil {
			return nil
		}
		if c, ok := seen[n]; ok {
			return c
		}
		c := *n
		seen[n] = &c
		value.D(&c).Start, value.D(&c).End = int32(cp[value.D(n).Start]), int32(cp[value.D(n).End])
		if value.D(n).Children != nil {
			value.D(&c).Children = make([]*parse.Node, len(value.D(n).Children))
			for i, ch := range value.D(n).Children {
				value.D(&c).Children[i] = conv(ch)
			}
		}
		if value.D(n).Fields != nil {
			value.D(&c).Fields = make(noderep.Fields, len(value.D(n).Fields))
			for i, f := range value.D(n).Fields {
				switch v := f.Value.(type) {
				case *parse.Node:
					f.Value = conv(v)
				case string:
					if n.Type() == noderep.TypeError && f.Name == "message" {
						f.Value = fixMessage(v)
					}
				}
				value.D(&c).Fields[i] = f
			}
		}
		return &c
	}
	convErr := func(e *parse.SyntaxError) *parse.SyntaxError {
		c := *e
		c.Pos = cp[e.Pos]
		c.Line, c.Col = lineCol(c.Pos)
		return &c
	}
	switch e := err.(type) {
	case *parse.SyntaxError:
		err = convErr(e)
	case parse.SyntaxErrors:
		l := make(parse.SyntaxErrors, len(e))
		for i, x := range e {
			l[i] = convErr(x)
		}
		err = l
	}
	return conv(n), err
}

// closureExprMatcher returns the closure that the closure backend builds for the expression e of
// the program prog, from the program's Core IR after the closure pipeline. The expression must be
// one that a node of Core IR is built from.
func closureExprMatcher(t *testing.T, prog *compile.Program, e grammar.Expr, plainDispatch bool) core.Matcher {
	t.Helper()
	m, found := closureExprCore(t, prog, e)
	f, err := closure.Matcher(prog, m, found, nil, plainDispatch)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// closureExprNode returns the Core IR node, after the passes, that closureExprMatcher builds for e.
func closureExprNode(t *testing.T, prog *compile.Program, e grammar.Expr) ir.Node {
	t.Helper()
	_, n := closureExprCore(t, prog, e)
	return n
}

func closureExprCore(t *testing.T, prog *compile.Program, e grammar.Expr) (*ir.Module, ir.Node) {
	t.Helper()
	origin := map[ir.Node]grammar.Expr{}
	m, err := irbuild.CoreFrom(prog, origin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ir.Run(m, &ir.Env{}, ir.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	var found ir.Node
	for _, r := range m.Rules {
		ir.Walk(r.Body, func(n ir.Node) {
			if found == nil && origin[n] == e {
				found = n
			}
		})
	}
	if found == nil {
		t.Fatal("no Core IR node of the expression")
	}
	return m, found
}

// inlinedRules returns the rules that the closure backend runs in the inlined family, by ID.
func inlinedRules(t testing.TB, prog *Program) []*core.Rule {
	t.Helper()
	rules, err := prog.familyRules(closure.Inlined, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}
