package tsgen_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/syntax"
)

// documentEdit is an edit of a Document in the script of TestTSDocumentMatchesFreshParse: [Start,End)
// in the position unit, replaced with the bytes B64, or with the string Str when IsStr is set.
type documentEdit struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	B64   string `json:"b64"`
	Str   string `json:"str"`
	IsStr bool   `json:"isStr"`
	// Bad marks an edit that the Document rejects, leaving the text as it was.
	Bad bool `json:"bad"`
}

// documentCase is the sequence of edits of one Document, from the text B64, in the unit Bytes.
type documentCase struct {
	B64   string         `json:"b64"`
	Bytes bool           `json:"bytes"`
	Edits []documentEdit `json:"edits"`
}

// documentFragments are the replacements of the random edits: text of the grammar, and bytes that
// join with their neighbors into characters or stay invalid UTF-8.
var documentFragments = []string{
	"a = 1+2*3\n", "@x\n", "a,b,c\n", "\n", "x", "1", "(", ")", "+", " = ", ",", "@", "-", "é", "😀",
	"\xc3", "\xa9", "\xf0\x9f", "\x98\x80", "\xff", "zz = (4-5)*6\n",
}

// documentLines are the lines of the grammar that the random edits insert at line starts.
var documentLines = []string{"a = 1+2*3\n", "@x\n", "a,b,c\n", "\n", "zz = (4-5)*6\n", "q = -(7)\n"}

// documentByteStart returns the byte offset in text of the position start in the unit.
func documentByteStart(text string, start int, bytes bool) int {
	if bytes {
		return start
	}
	return documentByteStarts(text)[start]
}

// documentStarts returns the positions in the unit at which a character of text starts, and its end.
func documentStarts(text string, bytes bool) []int {
	var out []int
	n := 0
	for off := range text {
		if bytes {
			out = append(out, off)
		} else {
			out = append(out, n)
		}
		n++
	}
	if bytes {
		return append(out, len(text))
	}
	return append(out, n)
}

// documentApply applies an edit of [start,end) in the unit to text.
func documentApply(text string, start, end int, repl string, bytes bool) string {
	if !bytes {
		offs := append([]int{}, documentByteStarts(text)...)
		start, end = offs[start], offs[end]
	}
	return text[:start] + repl + text[end:]
}

// documentByteStarts returns the byte offsets of the characters of text, and its length.
func documentByteStarts(text string) []int {
	var out []int
	for off := range text {
		out = append(out, off)
	}
	return append(out, len(text))
}

// documentInside reports whether the byte offset k is inside a valid character of text.
func documentInside(text string, k int) bool {
	for start := k - 1; start >= max(0, k-utf8.UTFMax+1); start-- {
		if utf8.RuneStart(text[start]) {
			r, size := utf8.DecodeRuneInString(text[start:])
			return !(r == utf8.RuneError && size == 1) && start+size > k
		}
	}
	return false
}

// TestTSDocumentMatchesFreshParse edits generated TypeScript Documents at random, in both units and
// with replacements that join invalid UTF-8 bytes into characters, and requires each parse to return
// what the engine returns for a parse of the whole text, each typed parse what parseAST returns, and
// the text to be the edited one. Rejected edits change nothing. The Documents must reuse results:
// over the script they evaluate fewer rules than a new Document of each text does, and a parse with
// no edit since the last one evaluates none. Only main is a start rule, so the other rules are
// entered only through the calls of the printed code. As a negative control, runtimes with a broken
// edit or a broken retention of results must fail the comparison or the reuse.
func TestTSDocumentMatchesFreshParse(t *testing.T) {
	src := strings.Replace(gentest.IncrementalGrammar, "rule main", "@start\nrule main", 1)
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	ref := referenceProgram(t, src, compile.Options{NoProjections: true})
	rng := rand.New(rand.NewSource(64))
	var cases []documentCase
	var wants []string
	for _, bytes := range []bool{false, true} {
		for c := 0; c < 6; c++ {
			text := "a = 1+2\n@pos\nx,y\n\nb = (3)*-4\n"
			dc := documentCase{B64: base64.StdEncoding.EncodeToString([]byte(text)), Bytes: bytes}
			// undo is the edit that undoes the last applied one, when the texts and the replacement
			// are valid UTF-8, and failed tells whether the text does not match.
			var undo *documentEdit
			failed := false
			for e := 0; e < 40; e++ {
				starts := documentStarts(text, bytes)
				var ed documentEdit
				var repl string
				switch {
				case failed && undo != nil && rng.Intn(4) != 0:
					// The text is repaired, so that the parses that follow reach the end of the text.
					ed, repl = *undo, undo.Str
				case utf8.ValidString(text) && rng.Intn(3) == 0:
					// A line inserted at a line start, or replacing the line there.
					var lines []int
					for k, off := range documentByteStarts(text) {
						if k == 0 || text[off-1] == '\n' {
							lines = append(lines, k)
						}
					}
					k := rng.Intn(len(lines))
					i, j := lines[k], lines[k]
					if k+1 < len(lines) && rng.Intn(2) == 0 {
						j = lines[k+1]
					}
					repl = documentLines[rng.Intn(len(documentLines))]
					ed = documentEdit{Start: starts[i], End: starts[j], Str: repl, IsStr: true}
				default:
					i := rng.Intn(len(starts))
					j := i + rng.Intn(min(3, len(starts)-i))
					repl = documentFragments[rng.Intn(len(documentFragments))]
					ed = documentEdit{Start: starts[i], End: starts[j]}
					if utf8.ValidString(repl) && rng.Intn(2) == 0 {
						ed.Str, ed.IsStr = repl, true
					}
					switch rng.Intn(8) {
					case 0:
						ed.Start, ed.End, ed.Bad = starts[j], starts[i]-1, true
					case 1:
						ed.End, ed.Bad = starts[len(starts)-1]+1, true
					case 2:
						if bytes {
							// A position inside a character, if the text has one.
							for k := 1; k < len(text); k++ {
								if documentInside(text, k) {
									ed.Start, ed.End, ed.Bad = k, k, true
									break
								}
							}
						}
					}
				}
				ed.B64 = base64.StdEncoding.EncodeToString([]byte(repl))
				if !ed.Bad {
					old := text
					text = documentApply(text, ed.Start, ed.End, repl, bytes)
					undo = nil
					if utf8.ValidString(old) && utf8.ValidString(text) && utf8.ValidString(repl) {
						removed := old[len(text[:documentByteStart(text, ed.Start, bytes)]):]
						removed = removed[:len(old)-len(text)+len(repl)]
						n := len(repl)
						if !bytes {
							n = utf8.RuneCountInString(repl)
						}
						undo = &documentEdit{Start: ed.Start, End: ed.Start + n, Str: removed, IsStr: true}
					}
				}
				dc.Edits = append(dc.Edits, ed)
				n, err := ref(text, bytes, false)
				failed = err != nil
				wants = append(wants, fmt.Sprintf("%s %s", base64.StdEncoding.EncodeToString([]byte(text)), gentest.DeepResultJSON(t, n, err)))
			}
			cases = append(cases, dc)
		}
	}
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	script := `import { createDocument, parse, parseAST, marshal, Bytes, CodePoints } from "./parser.ts";
const cases = ` + string(data) + `;
const b64 = (u) => Buffer.from(u).toString("base64");
// evaluated and reused sum the statistics of the Documents' parses, full the rule evaluations of a
// first parse of a new Document of each text, and again those of parses with no edit before them.
let evaluated = 0, reused = 0, full = 0, again = 0;
for (const c of cases) {
  const unit = c.bytes ? Bytes : CodePoints;
  const made = createDocument(new Uint8Array(Buffer.from(c.b64, "base64")), { unit, expectations: true });
  if (made.error !== null) throw made.error;
  const d = made.document;
  for (const e of c.edits) {
    try {
      const before = b64(d.bytes());
      const err = d.edit(e.start, e.end, e.isStr ? e.str : new Uint8Array(Buffer.from(e.b64, "base64")));
      if ((err !== null) !== e.bad) throw new Error("edit " + JSON.stringify(e) + ": " + err);
      if (err !== null && b64(d.bytes()) !== before) throw new Error("a rejected edit changed the text");
      const r = d.parse();
      const s = d.stats();
      evaluated += s.evaluated;
      reused += s.reused;
      const text = d.bytes();
      const n = createDocument(text, { unit, expectations: true }).document;
      n.parse();
      full += n.stats().evaluated;
      const fresh = parse(text, unit);
      if (marshal(fresh.node) !== marshal(r.node) || String(fresh.error) !== String(r.error)) throw new Error("Document parse differs from parse");
      const typed = d.parseAST();
      const freshTyped = parseAST(text, unit);
      if (JSON.stringify(typed.value) !== JSON.stringify(freshTyped.value) || String(typed.error) !== String(freshTyped.error)) throw new Error("Document parseAST differs from parseAST");
      const out = r.error === null ? '{"node":' + marshal(r.node) + "}" : '{"err":' + JSON.stringify(r.error.message) + ',"node":' + marshal(r.node) + "}";
      console.log(b64(text) + " " + out);
    } catch (err) {
      console.log("throw " + JSON.stringify(String(err)));
    }
  }
  d.parse();
  again += d.stats().evaluated;
}
console.log("stats " + JSON.stringify({ evaluated, reused, full, again }));
`
	opts := gen.GenOptions{Start: "main", Document: true, Types: true}
	// run runs the script on the runtime that mutate changes, and returns the first difference from
	// the engine.
	run := func(mutate func(string) string) error {
		out := gentest.RunTSGrammarScriptBundle(t, g, script, opts, func(g *grammar.Grammar, opts gen.GenOptions) ([]byte, error) {
			code, err := gentest.GenerateTSBundle(g, opts)
			if err != nil || mutate == nil {
				return code, err
			}
			return []byte(mutate(string(code))), nil
		})
		got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		last := got[len(got)-1]
		if !strings.HasPrefix(last, "stats ") {
			return fmt.Errorf("no statistics:\n%s", gentest.Clip(out, 0))
		}
		got = got[:len(got)-1]
		var stats struct{ Evaluated, Reused, Full, Again int }
		if err := json.Unmarshal([]byte(strings.TrimPrefix(last, "stats ")), &stats); err != nil {
			return err
		}
		if mutate == nil {
			t.Logf("statistics: %+v", stats)
		}
		for i := range wants {
			if i >= len(got) {
				return fmt.Errorf("got %d results, want %d:\n%s", len(got), len(wants), gentest.Clip(out, 0))
			}
			if got[i] != wants[i] {
				at := gentest.DiffAt(got[i], wants[i])
				return fmt.Errorf("parse %d:\n got %s\nwant %s", i, gentest.Clip(got[i], at), gentest.Clip(wants[i], at))
			}
		}
		if len(got) != len(wants) {
			return fmt.Errorf("got %d results, want %d:\n%s", len(got), len(wants), gentest.Clip(out, 0))
		}
		if stats.Reused == 0 || stats.Evaluated >= stats.Full || stats.Again != 0 {
			return fmt.Errorf("the Documents reuse too little: %+v", stats)
		}
		return nil
	}
	if err := run(nil); err != nil {
		t.Fatal(err)
	}
	for _, m := range documentMutants {
		mutate := func(src string) string {
			if strings.Count(src, m.old) != 1 {
				t.Fatalf("mutant %q: the runtime has %d copies of %q, want 1", m.name, strings.Count(src, m.old), m.old)
			}
			return strings.Replace(src, m.old, m.new, 1)
		}
		if run(mutate) == nil {
			t.Errorf("mutant %q passes the comparison", m.name)
		}
	}
}

// documentMutants are the broken edits of the negative control of TestTSDocumentMatchesFreshParse:
// each replaces old in the runtime with new.
var documentMutants = []struct{ name, old, new string }{
	// The replacement is dropped: the edit only deletes the range.
	{"dropped splice", "out.set(r, startByte);", ""},
	// A position in code points is taken as one byte per character after the start.
	{"mis-mapped unit offset", "endByte = documentByteOffset(b, end);", "endByte = startByte + (end - start);"},
	// No result is kept between parses.
	{"memo not kept", "d.slots = document.slots;", ""},
	// A result whose examined input reaches into the edit is kept in place.
	{"edit ignores examined input", "} else if (e.examined <= ed.start) {", "} else if (e.from !== noRecord) {"},
	// A result after the edit is kept at its old position.
	{"result not moved", "to = pos + ed.delta;", ""},
	// The value of a moved result keeps its old positions.
	{"value not moved", "if (shift === undefined) return true;", "return true;"},
}

// TestTSDocumentReuseRecordsExamined checks that a result reused inside a rule that is evaluated
// again adds what it examined to that rule's result: w looks past its end, p ends with w, and after
// p is evaluated again with w reused, an edit that w looked at must make p evaluated again. As a
// negative control, the runtime without that record must fail.
func TestTSDocumentReuseRecordsExamined(t *testing.T) {
	g, err := syntax.Parse("@start\nrule main = p \";\" [a-z?]*\nrule p = \"s\" w\nrule w = [0-9]+ !\";x\"")
	if err != nil {
		t.Fatal(err)
	}
	script := `import { createDocument, marshal } from "./parser.ts";
const d = createDocument("s12;?").document;
d.parse();
for (const [s, e, r] of [[0, 1, "s"], [4, 5, "x"]]) {
  d.edit(s, e, r);
  const a = d.parse();
  const f = createDocument(d.text()).document.parse();
  console.log(d.text(), marshal(a.node) === marshal(f.node) && String(a.error) === String(f.error), JSON.stringify(d.stats()));
}
`
	want := "s12;? true {\"evaluated\":2,\"reused\":1}\ns12;x true {\"evaluated\":3,\"reused\":0}\n"
	run := func(mutate func(string) string) string {
		return gentest.RunTSGrammarScriptBundle(t, g, script, gen.GenOptions{Start: "main", Document: true}, func(g *grammar.Grammar, opts gen.GenOptions) ([]byte, error) {
			code, err := gentest.GenerateTSBundle(g, opts)
			if err != nil || mutate == nil {
				return code, err
			}
			return []byte(mutate(string(code))), nil
		})
	}
	if got := run(nil); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	const old = "this.documentTouch(e!.examined);"
	got := run(func(src string) string {
		if strings.Count(src, old) != 1 {
			t.Fatalf("the runtime has %d copies of %q, want 1", strings.Count(src, old), old)
		}
		return strings.Replace(src, old, "", 1)
	})
	if got == want {
		t.Error("the runtime that does not record what a reused result examined passes")
	}
}

// TestTSDocumentOptions checks the options of a Document: invocation options apply after the
// defaults, an invocation cannot change the unit or the expectations (which the test parsers record
// by default), an invalid default fails
// construction, the input is copied, and a parse or an edit from a trace callback of a parse fails.
func TestTSDocumentOptions(t *testing.T) {
	g, err := parseTest(`rule main = [a-z]+ "!"`)
	if err != nil {
		t.Fatal(err)
	}
	script := `import { createDocument, Bytes, CodePoints } from "./parser.ts";
const input = new TextEncoder().encode("ab!");
const { document: d, error } = createDocument(input, { unit: Bytes, limits: { compute: 100000 } });
if (error !== null) throw error;
input[0] = 0x21;
console.log(d.text(), d.parse().error === null);
console.log(String(d.parse({ unit: CodePoints }).error));
console.log(String(d.parse({ expectations: false }).error));
console.log(String(d.parse({ limits: { input: 2 } }).error));
console.log(String(createDocument("abc!", { limits: { input: 2 } }).error));
console.log(String(createDocument("abc!", { bogus: 1 }).error));
let inner = null;
const traced = d.parse({ trace: () => { if (inner === null) inner = [String(d.parse().error), String(d.edit(0, 0, "x"))]; } });
console.log(traced.error === null, inner.join(" | "), d.text());
console.log(String(d.edit(0, 9, "x")), d.text());
`
	out := gentest.RunTSGrammarScriptWith(t, g, script, gen.GenOptions{Start: "main", Document: true, LimitControls: true, Observability: true})
	want := `ab! true
Error: document parse cannot change unit
Error: document parse cannot change expectations
ParseInterruptionError: input-limit exceeded: limit 2, length 3
ParseInterruptionError: input-limit exceeded: limit 2, length 4
Error: unknown parse option: bogus
true Error: document is already parsing | Error: document is already parsing ab!
Error: invalid range [0,9) for text of length 3 ab!
`
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
}
