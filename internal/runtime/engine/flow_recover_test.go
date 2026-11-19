package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/bytecode"
)

// TestFlowBodyRecoveries runs grammars with #recover from Flow bodies (checkFlowGrammars). A recovery
// whose body fails keeps the body's expectations while its skip runs (KEEP), makes the syntax error
// (MAKEERROR), records it (APPENDERROR) and, when it has a value, builds its error node
// (NEWERRORNODE). The grammars recover with and without a value, in a repetition, nested, in a
// choice and a lookahead that drop the errors recovered inside them, in a rule that fails after
// recovering, with a skip that matches nothing, under @error, and with a body that succeeds, so the
// recovered errors and the expectations of every outcome must be those of the match code.
func TestFlowBodyRecoveries(t *testing.T) {
	checkFlowGrammars(t, []flowGrammar{
		{"value", `rule main = x:("a" ";") #recover(skip=[^;]* ";") $$ => $x`, []string{"a;", "b;", "bb;", ";", "a", "", "b"}, true},
		{"no value", `rule main = ("a" ";") #recover(skip=[^;]* ";") $$`, []string{"a;", "b;", ";", ""}, true},
		{"repetition", `rule main = stmt* $$
rule stmt = ([a-z]+ ";") #recover(skip=[^;]+ ";")`, []string{"a;b;", "a;1;b;", "1;2;", "a;1", "", ";"}, true},
		{"nested", `rule main = (("a" "b") #recover(skip="x") ";") #recover(skip=[^;]* ";") $$`, []string{"ab;", "x;", "ax;", "ab", "q;", "xq;"}, true},
		{"choice", `rule main = ("a" "b") #recover(skip="x" "y") "!" $$ / "x" "y" "?" $$`, []string{"ab!", "xy!", "xy?", "x?", ""}, true},
		{"lookahead", `rule main = &(("a" "b") #recover(skip="x")) "x" $$ / "ab" $$`, []string{"x", "ab", "a", ""}, true},
		{"rule fails after", `rule main = item "!" $$ / "q" "z" $$
rule item = ("a" "b") #recover(skip="q")`, []string{"ab!", "q!", "qz", "q", ""}, true},
		{"empty skip", `rule main = ("a" "b") #recover(skip=[x]*) $$`, []string{"ab", "xx", "c", ""}, true},
		{"error label", `rule main = item* $$
rule item = ("a" @error(message="want a") ";") #recover(skip=[^;]+ ";")`, []string{"a;", "b;a;", "ab;", ""}, true},
		{"captures", `rule main = xs:(k:[a-z] v:([0-9]+ ";") #recover(skip=[^;]* ";"))* $$ => $xs`, []string{"a1;b2;", "a1;bz;", "az;", ""}, true},
	}, bytecode.FlowKeep)
}

// TestFlowBodyRecoveryCorpora runs the recovering grammars of testdata/recovery over their corpora
// of invalid inputs, and the valid inputs they were made from, from Flow bodies (checkFlowGrammars).
func TestFlowBodyRecoveryCorpora(t *testing.T) {
	for _, name := range []string{"json", "cel"} {
		dir := "../../../testdata/recovery"
		src, err := os.ReadFile(filepath.Join(dir, name+".zp"))
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, name+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		var inputs []string
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var c struct{ Input, Base string }
			if err := json.Unmarshal([]byte(line), &c); err != nil {
				t.Fatal(err)
			}
			inputs = append(inputs, c.Input, c.Base)
		}
		checkFlowGrammars(t, []flowGrammar{{name, string(src), inputs, true}}, bytecode.FlowKeep)
	}
}
