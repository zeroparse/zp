package gogen_test

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine"
)

// TestConstructedFieldChecksAreSelective checks that a constructed field is never checked when it
// runs: a value statically known to fit needs nothing, a value of an unknown type is rejected
// when the grammar is compiled, and a cast is the only check, one cast in the term code for each cast in the grammar.
func TestConstructedFieldChecksAreSelective(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testConstructedFieldChecksAreSelective)
}

func testConstructedFieldChecksAreSelective(t *testing.T) {
	const decls = `type Int = { Text string }
type S = { Static Int; Var int; Dynamic Int; Elements []Int; Union *Int; Untyped _; Empty []Int; Joined []Int }
var x int
rule item Int = "i" => Int{Text: "i"}
rule val _ = "v" => list()
`
	const body = "rule main = {x := 1} a:item d:val l:item* o:item? => S{Static: $a, Var: x, Dynamic: %s, Elements: $l.Value, Union: $o, Untyped: x, Empty: filter(nil, (e) => true), Joined: concat(nil, $l.Value)}\n"
	if got := compileMessage(t, decls+fmt.Sprintf(body, "$d")); !strings.Contains(got, "cannot use _ as Int in field Dynamic of S: the type of the value is not known; convert it with a cast, as in value.(Int)") {
		t.Fatalf("unknown value: %q", got)
	}
	src := decls + fmt.Sprintf(body, "$d.(Int)")
	prog := mustCompile(t, src)
	m := prog.Module()
	if v := m.InstructionSetVersion(); v != bytecode.ISAVersion {
		t.Fatalf("instruction set %d", v)
	}
	if d := m.Disassemble(); fmt.Sprint(termCasts(d)) != `["Int" at=6:88]` {
		t.Fatalf("casts %q, want the one cast to Int:\n%s", termCasts(d), d)
	}
	plain := mustCompile(t, `type Int = { Text string }
type S = { Static Int }
rule main = a:item => S{Static: $a}
rule item Int = "i" => Int{Text: "i"}
`)
	if v := plain.Module().InstructionSetVersion(); v != bytecode.ISAVersion {
		t.Fatalf("module instruction set %d", v)
	}
	if d := plain.Module().Disassemble(); len(termCasts(d)) != 0 || !strings.Contains(d, "terms:") {
		t.Fatalf("a statically typed field is checked:\n%s", d)
	}
	if got := resultWith(prog, "iv", engine.ParseOptions{Expectations: true, Backend: engine.Bytecode}); !strings.Contains(got, "cannot cast []_ to Int") {
		t.Fatalf("dynamic list: %s", got)
	}
	// The recognizer keeps the same casts as its program.
	pred := mustCompile(t, `type Int = { Text string }
type S = { Static Int; Dynamic Int }
rule main = a:item d:val {S{Static: $a, Dynamic: $d.(Int)} != nil}
rule item Int = "i" => Int{Text: "i"}
rule val _ = "v"
`)
	rec, err := pred.Recognizer()
	if err != nil {
		t.Fatal(err)
	}
	if d := rec.Module().Disassemble(); fmt.Sprint(termCasts(d)) != `["Int" at=3:53]` {
		t.Fatalf("recognizer disassembly:\n%s", d)
	}
	g, err := parseTest(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: "p", Start: "main", Types: true, Recognize: true})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(code), "var fieldType"); n != 1 {
		t.Fatalf("generated %d field type variables", n)
	}
}

// termCasts returns the casts of the term code in the disassembly d, each once, in order of first
// appearance: the check and the failure of a cast name the same cast.
func termCasts(d string) []string {
	var casts []string
	seen := map[string]bool{}
	for _, line := range strings.Split(d, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "cast" {
			continue
		}
		c := strings.Join(f[2:], " ")
		if !seen[c] {
			seen[c] = true
			casts = append(casts, c)
		}
	}
	return casts
}
