package codegen_test

import (
	"encoding/json"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/syntax"
)

// documentStatsScript is the text of a Document and the edits of TestDocumentStatsAcrossTargets, in
// code points: [start, end, replacement].
type documentStatsScript struct {
	Text  string
	Edits [][3]any
}

// TestDocumentStatsAcrossTargets runs the same edits on the Documents of the generated Go and
// TypeScript parsers of a grammar and requires the same statistics after each parse, with the same
// end of the result and the same success. Only main is a start rule, and the grammars have direct and
// indirect left recursion, an Operators Rule, line-start anchors and lookahead, whose results the
// Documents reuse by the same rules.
func TestDocumentStatsAcrossTargets(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	codegentest.RequireNode(t)
	if testing.Short() {
		t.Skip("builds a generated Go program")
	}
	for _, c := range []struct {
		name, src, text string
		frags           []string
	}{
		{"incremental", strings.Replace(codegentest.IncrementalGrammar, "rule main", "@start\nrule main", 1),
			"a = 1+2\n@pos\nx,y\n\nb = (3)*-4\nc = 5\n",
			[]string{"a = 1+2*3\n", "@x\n", "a,b,c\n", "\n", "x", "1", "(", ")", "+", " = ", ",", "@", "-", "é", "zz = (4-5)*6\n"}},
		{"left recursion", `@start
rule main = (stmt ";")* $$
rule stmt = id "=" e / e
rule e = f "+" t / f "-" t / t
rule f = e
rule t = t "*" n / n
rule n = !"0" [0-9]+ / "0" / "(" e ")" / id
rule id = [a-z]+ #atomic`,
			"x=1+2;3-(4+5)*6;0;y=x*x-1;",
			[]string{"1", "+", "-", "*", "(", ")", ";", "x", "=", "0", "7*8;", "a=(1);", ""}},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, err := syntax.Parse(c.src)
			if err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewSource(7))
			script := documentStatsScript{Text: c.text}
			ref := compileBare(t, c.src)
			text := []rune(c.text)
			// undo holds the edits that undo the edits made since the text last matched. While the
			// text does not match, edits mostly undo the last of them, so that most parses reach the
			// end of the text.
			var undo [][3]any
			matched := 0
			for range 60 {
				i := rng.Intn(len(text) + 1)
				j := i + rng.Intn(min(3, len(text)-i)+1)
				r := c.frags[rng.Intn(len(c.frags))]
				if _, err := ref.ParseWith("main", string(text), engine.ParseOptions{}); err == nil {
					matched++
					undo = undo[:0]
				}
				if n := len(undo); n > 0 && rng.Intn(4) != 0 {
					i, j, r = undo[n-1][0].(int), undo[n-1][1].(int), undo[n-1][2].(string)
					undo = undo[:n-1]
				} else {
					undo = append(undo, [3]any{i, i + len([]rune(r)), string(text[i:j])})
				}
				script.Edits = append(script.Edits, [3]any{i, j, r})
				text = append(append(append([]rune{}, text[:i]...), []rune(r)...), text[j:]...)
			}
			if matched < 20 {
				t.Fatalf("only %d of the texts before the edits match", matched)
			}
			data, err := json.Marshal(script)
			if err != nil {
				t.Fatal(err)
			}
			goOut := documentStatsGo(t, g, string(data))
			tsOut := codegentest.RunTSGrammarScriptWith(t, g, `import { createDocument } from "./parser.ts";
const c = `+string(data)+`;
const d = createDocument(c.Text).document;
const show = () => { const r = d.parse(); const s = d.stats(); console.log(s.evaluated, s.reused, r.node === null ? -1 : r.node.end, r.error !== null); };
show();
for (const e of c.Edits) {
  const err = d.edit(e[0], e[1], e[2]);
  if (err !== null) console.log("edit", String(err));
  show();
}
`, codegen.GenOptions{Start: "main", Document: true})
			if goOut != tsOut {
				gl, tl := strings.Split(goOut, "\n"), strings.Split(tsOut, "\n")
				for i := range min(len(gl), len(tl)) {
					if gl[i] != tl[i] {
						t.Fatalf("parse %d: Go %q, TypeScript %q (evaluated reused end failed)", i, gl[i], tl[i])
					}
				}
				t.Fatalf("Go has %d lines, TypeScript %d", len(gl), len(tl))
			}
			t.Logf("parses: %d, texts that match before the edits: %d", strings.Count(goOut, "\n"), matched)
		})
	}
}

// documentStatsGo runs the script data on the Document of the generated Go parser of g and returns
// what it prints for each parse.
func documentStatsGo(t *testing.T, g *grammar.Grammar, data string) string {
	t.Helper()
	code, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main", Document: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, src := range map[string]string{
		"go.mod":       codegentest.GoMod("fixture"),
		"parser.go":    string(code),
		"main/main.go": strings.Replace(documentStatsMain, "SCRIPT", strconv.Quote(data), 1),
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", "./main")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	return string(out)
}

// documentStatsMain is the program that runs a script on the Document of a generated Go parser.
const documentStatsMain = `package main

import (
	"encoding/json"
	"fmt"

	p "fixture"
)

const data = SCRIPT

func main() {
	var c struct {
		Text  string
		Edits [][3]any
	}
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		panic(err)
	}
	d, err := p.NewDocument(c.Text)
	if err != nil {
		panic(err)
	}
	show := func() {
		n, err := d.Parse()
		end := -1
		if n != nil {
			end = n.End()
		}
		s := d.Stats()
		fmt.Println(s.Evaluated, s.Reused, end, err != nil)
	}
	show()
	for _, e := range c.Edits {
		if err := d.Edit(int(e[0].(float64)), int(e[1].(float64)), e[2].(string)); err != nil {
			fmt.Println("edit", err)
		}
		show()
	}
}
`
