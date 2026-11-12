package engine

import (
	"strings"
	"testing"
	"time"
)

// emptyInfixRule is an Operators Rule whose operand and infix operator part both match the empty
// string, with the associativity and the levels that %s gives.
const emptyInfixRule = `rule e = #operators { operand "x"; operand ""; %s }`

// emptyInfixCases are the grammars of TestOperatorsEmptyInfixTerminates, with the results that
// some inputs must give on every backend. The last grammar parses an expression after the right
// operand of an empty operator: that expression is no right operand, so it applies the operator.
var emptyInfixCases = []struct {
	levels string
	start  string
	want   map[string]string
}{
	{levels: `level { infix right "" }`, want: map[string]string{
		"x":  `(lhs:"x"@e op:"" rhs:""@e)@e`,
		"xx": `(lhs:"x"@e op:"" rhs:(lhs:"x"@e op:"" rhs:""@e)@e)@e`,
	}},
	{levels: `level { infix left "" }`, want: map[string]string{
		"xx": `(lhs:(lhs:"x"@e op:"" rhs:"x"@e)@e op:"" rhs:""@e)@e`,
	}},
	{levels: `level { infix none "" }`, want: map[string]string{
		"xx": `(lhs:"x"@e op:"" rhs:"x"@e)@e`,
	}},
	{levels: `level { infix right "" }; level { infix right "" }`},
	{levels: `level { infix right "" }`, start: `rule s = e "," e` + "\n", want: map[string]string{
		"x,": `((lhs:"x"@e op:"" rhs:""@e)@e "," (lhs:""@e op:"" rhs:""@e)@e)@s`,
	}},
}

// TestOperatorsEmptyInfixTerminates checks on every backend that an Operators Rule whose operand and
// infix operator part both match the empty string parses within the default limits, with the same
// result on every backend: the right operand of an operator applied without consuming input does
// not apply an empty operator again before it consumes input. Each parse runs under a time limit,
// so that a parse that nests without end fails the test instead of hanging it.
func TestOperatorsEmptyInfixTerminates(t *testing.T) {
	t.Parallel()
	for _, c := range emptyInfixCases {
		src := "@start\n" + c.start + strings.Replace(emptyInfixRule, "%s", c.levels, 1)
		prog := mustCompile(t, src)
		for _, in := range []string{"x", "", "xx", "xxx", "x,", ","} {
			first := ""
			for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
				done := make(chan string, 1)
				go func() {
					n, err := prog.ParseWith("", in, ParseOptions{Backend: b})
					if err != nil {
						done <- "error: " + err.Error()
						return
					}
					done <- n.String()
				}()
				var got string
				select {
				case got = <-done:
				case <-time.After(10 * time.Second):
					t.Fatalf("%s\non %q with %v: no result within 10s", src, in, b)
				}
				if first == "" {
					first = got
				}
				want, ok := c.want[in]
				switch {
				case strings.Contains(got, "nesting") || strings.Contains(got, "limit"):
					t.Errorf("%s\non %q with %v: %s", src, in, b, got)
				case ok && got != want:
					t.Errorf("%s\non %q with %v: %s, want %s", src, in, b, got, want)
				case got != first:
					t.Errorf("%s\non %q with %v: %s, but %s on %v", src, in, b, got, first, Closure)
				}
			}
		}
	}
}
