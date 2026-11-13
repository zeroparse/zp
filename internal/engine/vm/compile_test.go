package vm

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
)

// TestModuleDisassembly checks the disassembly of the match code of a module as the compiler
// makes it.
func TestModuleDisassembly(t *testing.T) {
	t.Parallel()
	got := compileModule(t, `type P = { K node[string]; N int }
rule main = k:key "=" v:[0-9]+ #atomic (";" / _) => P{K: $k, N: len($v) + 1}
rule key = [a-z]+ @error(message="expected a key") #atomic`).Disassemble()
	want := `rules:
  0 main entry=0 scope=[k v] action=0 memo,transient
  1 key entry=13 scope=[] memo,transient
code:
main:
     0  CALL       key min=0 keep=1
     1  CAPTURE    slot=0 pop
     2  STR        "=" build=0
     3  PUSHPOS    
     4  SCAN       "[0-9]" min=1 max=-1
     5  ATOMIC     build=1
     6  CAPTURE    slot=1 pop
     7  GUARD      ";" depth=0 -> 11
     8  CHOICE     -> 11
     9  STR        ";" build=0
    10  COMMIT     -> 12
    11  TOP        build=0
    12  RETURN     
key:
    13  PUSHPOS    
    14  LABEL      "expected a key"
    15  SCANSITE   "[a-z]" min=1 site=1
    16  ENDLABEL   
    17  ATOMIC     build=1
    18  RETURN     
terms:
main.action:
  term 0 `
	// The term code is the engine's translation of the action's Core IR, which the term tests pin.
	if !strings.HasPrefix(got, want) {
		t.Errorf("got\n%s", got)
	}
}

func TestDiagnosticLabelDisassembly(t *testing.T) {
	t.Parallel()
	module := compileModule(t, `rule main = "a" @error(message="explicit") / "b" @diagnostic(name="letter")`)
	text := module.Disassemble()
	if !strings.Contains(text, `LABEL      "explicit"`+"\n") {
		t.Errorf("explicit error label changed:\n%s", text)
	}
	if !strings.Contains(text, `LABEL      "letter" mode=diagnostic`+"\n") {
		t.Errorf("diagnostic label mode missing:\n%s", text)
	}
}

// TestLoosestLevelCall checks that a call of the loosest level of an Operators Rule is the plain
// call of the rule: the same instruction, with binding level 0, so the same memo entry. The cost
// view treats the two as the same expression.
func TestLoosestLevelCall(t *testing.T) {
	t.Parallel()
	text := compileModule(t, `rule main = level(e, sum) "x" / e "y" / level(e, mul) "z"
rule e = #operators {
  operand "n"
  level sum { infix left "+" }
  level mul { infix left "*" }
}`).Disassemble()
	var calls []string
	for _, line := range strings.Split(text, "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[1] == "CALL" {
			calls = append(calls, strings.Join(f[2:], " "))
		}
	}
	want := []string{"e min=0 keep=1", "e min=0 keep=1", "e min=1 keep=1"}
	if strings.Join(calls, "; ") != strings.Join(want, "; ") {
		t.Errorf("calls %q, want %q\n%s", calls, want, text)
	}
}

// TestMissingTokensModuleGuard checks that a grammar that uses #missing has no bytecode form.
func TestMissingTokensModuleGuard(t *testing.T) {
	t.Parallel()
	prog := compiletest.Program(t, `rule main = ";" #missing`, compile.Options{SourceInfo: true})
	if _, err := Compile(prog); err == nil || !strings.Contains(err.Error(), "bytecode module construction") {
		t.Fatal("silent bytecode lowering", err)
	}
}

// compileModule compiles src with source information, and the options opts if any, to a module.
func compileModule(t *testing.T, src string, opts ...compile.Options) *bytecode.Module {
	t.Helper()
	var o compile.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	o.SourceInfo = true
	m, err := Compile(compiletest.Program(t, src, o))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// varModuleSource declares variables of each type, reads them in predicates and rebinds them.
const varModuleSource = `var n int = 3
var s string = "x"
var b bool
rule main = {n := n + 1} {s := s + "y"} {n > 2 && s != "" && !b} item "a"
rule item = {n > 0} "b"?
`
