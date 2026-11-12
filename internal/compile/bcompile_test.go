package compile

import (
	"strings"
	"testing"
)

// TestModuleDisassembly checks the disassembly of a module as the compiler makes it: the engine makes
// the term code of its terms, so the term is empty.
func TestModuleDisassembly(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `type P = { K node[string]; N int }
rule main = k:key "=" v:[0-9]+ #atomic (";" / _) => P{K: $k, N: len($v) + 1}
rule key = [a-z]+ @error(message="expected a key") #atomic`)
	got := prog.BytecodeModule().Disassemble()
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
  term 0 regs=[0 0 0 0]
`
	if got != want {
		t.Errorf("got\n%s", got)
	}
}

func TestDiagnosticLabelDisassembly(t *testing.T) {
	t.Parallel()
	module := mustCompile(t, `rule main = "a" @error(message="explicit") / "b" @diagnostic(name="letter")`).BytecodeModule()
	text := module.Disassemble()
	if !strings.Contains(text, `LABEL      "explicit"`+"\n") {
		t.Errorf("explicit error label changed:\n%s", text)
	}
	if !strings.Contains(text, `LABEL      "letter" mode=diagnostic`+"\n") {
		t.Errorf("diagnostic label mode missing:\n%s", text)
	}
}
