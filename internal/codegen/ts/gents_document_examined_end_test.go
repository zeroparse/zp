package tsgen_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// documentScanTypeScriptTest drives the library's Document directly with rules written by hand: main
// walks one scan site from the same start four times, the site turns hot at the second walk, the
// third records itself and the fourth, the rule last, takes the record. A walk matches the a
// characters, each iteration also reading the two characters after the one it matches, so that it
// examines two positions past the character where it ends. The entry of last must record that end,
// and an edit of the position before it must drop the entry.
const documentScanTypeScriptTest = `

let dsHit = false;
const dsWalk = (p: Parser): R => {
  const start = p.pos;
  let count = 0;
  for (;;) {
    const c = p.peek();
    if (c !== 97) break;
    const at = p.pos;
    p.pos = at + 1;
    p.peek();
    p.pos = at + 2;
    p.peek();
    p.pos = at + 1;
    count++;
    if (count === SCAN_HOT_SPAN) count = p.scanReach(1, start, count);
  }
  if (count >= SCAN_HOT_SPAN) p.scanEnd(1, start, count);
  return null;
};
const dsPlain = mkRule(1, "plain", [], 0, false, false, -1, true, []);
dsPlain.body = (p: Parser) => dsWalk(p);
const dsLast = mkRule(2, "last", [], 0, true, false, -1, true, []);
dsLast.docMemo = true;
dsLast.body = (p: Parser) => {
  const v = dsWalk(p);
  dsHit = (p as any).scanHit === true;
  return v;
};
const dsMain = mkRule(0, "main", [], 0, true, false, -1, true, []);
dsMain.docMemo = true;
dsMain.body = (p: Parser) => {
  for (let i = 0; i < 3; i++) {
    p.pos = 0;
    if (p.call(dsPlain, 0) === undefined) return undefined;
  }
  p.pos = 0;
  const v = p.call(dsLast, 0);
  if (v === undefined) return undefined;
  while (p.peek() >= 0) p.pos += p.psize;
  return v;
};

function dsExamined(d: any, rule: number): number | null {
  for (let e = d.memo.slots[0] ?? null; e; e = e.next) if (e.rule === rule) return e.examined;
  return null;
}

const runLen = 20;
for (const unit of [CodePoints, Bytes]) {
  const g = mkGrammar(1000, false, false, false);
  const bytes = encodeUTF8("a".repeat(runLen) + "bcd");
  const o = resolveOptions(g, { unit }, bytes);
  if (o instanceof Error) throw o;
  const d = new DocumentBase(g, dsMain, 0, bytes, { unit }, o);
  dsHit = false;
  const first = d.parse();
  if (first.error !== null) throw first.error;
  if (!dsHit) throw new Error("unit " + unit + ": the last walk did not take a record");
  const want = runLen + 2;
  const got = dsExamined(d, 2);
  if (got !== want) throw new Error("unit " + unit + ": the entry examined " + got + ", want " + want);
  if (d.edit(runLen + 1, runLen + 2, "x") !== null) throw new Error("edit failed");
  if (dsExamined(d, 2) !== null) throw new Error("unit " + unit + ": the entry survived an edit inside its examined range");
  const again = d.parse();
  const fresh = new DocumentBase(g, dsMain, 0, encodeUTF8(d.text()), { unit }, o).parse();
  if (marshal(again.node) !== marshal(fresh.node) || String(again.error) !== String(fresh.error)) throw new Error("unit " + unit + ": the edited parse differs from a fresh one");
}

// The end that decoding a character examined (decodedEnd), and the end that comparing a literal
// examined, in the entry of the call that made the comparison.
const decodedCases: [string, number, number][] = DECODED_CASES;
for (const [hex, pos, end] of decodedCases) {
  const b = new Uint8Array(Buffer.from(hex, "hex"));
  decodeRune(b, pos, b.length);
  const got = decodedEnd(b, pos, b.length, decodedSize);
  if (got !== end) throw new Error("decodedEnd(" + hex + ", " + pos + ") = " + got + ", want " + end);
}
const literalCases: [string, string, number, number, number][] = LITERAL_CASES;
for (const unit of [CodePoints, Bytes]) {
  for (const [literal, input, skip, wantCP, wantBytes] of literalCases) {
    const dsLit = mkRule(1, "lit", [], 0, true, false, -1, true, []);
    dsLit.docMemo = true;
    const cps = Array.from(literal).map((c) => c.codePointAt(0)!);
    const lit: Lit = { text: literal, cps, bytes: Array.from(encodeUTF8(literal)) };
    dsLit.body = (p: Parser) => p.matchLiteral(lit, 0, true);
    const root = mkRule(0, "main", [], 0, true, false, -1, true, []);
    root.docMemo = true;
    root.body = (p: Parser) => {
      for (let i = 0; i < skip; i++) {
        p.peek();
        p.pos += p.psize;
      }
      p.call(dsLit, 0);
      return undefined;
    };
    const g = mkGrammar(1000, false, false, false);
    const bytes = encodeUTF8(input);
    const o = resolveOptions(g, { unit }, bytes);
    if (o instanceof Error) throw o;
    const d = new DocumentBase(g, root, 0, bytes, { unit }, o);
    d.parse();
    let at = skip;
    if (unit === Bytes) at = encodeUTF8(Array.from(input).slice(0, skip).join("")).length;
    let got: number | null = null;
    for (let e = (d as any).memo.slots[at] ?? null; e; e = e.next) if (e.rule === 1) got = e.examined;
    const want = unit === Bytes ? wantBytes : wantCP;
    if (got !== want) throw new Error("literal " + JSON.stringify(literal) + " in " + JSON.stringify(input) + " unit " + unit + ": examined " + got + ", want " + want);
  }
}
console.log("ok");
`

// TestTSDocumentExaminedEnds checks that, in a Document parse, a scan walk that takes the
// record of an earlier walk of its start examines what the record's walk examined. The test code is
// appended to a generated parser, whose library it drives. As a negative control, the library whose
// hit does not touch the record's end must fail.
func TestTSDocumentExaminedEnds(t *testing.T) {
	g, err := parseTest(`rule main = "a"*`)
	if err != nil {
		t.Fatal(err)
	}
	opts := codegen.GenOptions{Start: "main", Document: true}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	// run runs the test code on the library that mutate changes, and returns what it prints and
	// whether it succeeded.
	run := func(mutate func(string) string) (string, bool) {
		code, err := codegentest.GenerateTSBundle(g, opts)
		if err != nil {
			t.Fatal(err)
		}
		src := string(code)
		if mutate != nil {
			src = mutate(src)
		}
		dir := t.TempDir()
		for name, content := range map[string]string{"package.json": `{"type": "module"}`, "parser.ts": src + documentScanScript(), "main.ts": `import "./parser.ts";` + "\n"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(node, "main.ts")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err == nil
	}
	if out, ok := run(nil); !ok || strings.TrimSpace(out) != "ok" {
		t.Fatalf("got %q, want ok", codegentest.Clip(out, 0))
	}
	const touch = "    this.documentTouch(rec.examined);\n"
	if out, ok := run(func(src string) string {
		if strings.Count(src, touch) != 1 {
			t.Fatalf("the library has %d copies of %q, want 1", strings.Count(src, touch), touch)
		}
		return strings.Replace(src, touch, "", 1)
	}); ok || !strings.Contains(out, "the entry examined") {
		t.Errorf("the library whose scan hit does not touch the record's end must fail by the recorded end: ok=%v\n%s", ok, codegentest.Clip(out, 0))
	}
}

// documentScanScript returns the test code with the tables of cases.
func documentScanScript() string {
	decoded, err := json.Marshal(decodedEndTable)
	if err != nil {
		panic(err)
	}
	literals, err := json.Marshal(literalEndTable)
	if err != nil {
		panic(err)
	}
	return strings.NewReplacer("DECODED_CASES", string(decoded), "LITERAL_CASES", string(literals)).Replace(documentScanTypeScriptTest)
}

// decodedEndTable are the cases of the end that decoding a character examined: the input in
// hexadecimal, the byte position of the character and the end. The same cases are those of the
// library of generated Go parsers and of the engine.
var decodedEndTable = [][3]any{
	{"61", 0, 1}, {"c3a9", 0, 2}, {"e282ac", 0, 3}, {"f09f9880", 0, 4}, {"6162c3a9", 2, 4},
	{"ff", 0, 1}, {"80", 0, 1}, {"c080", 0, 1}, {"c1bf", 0, 1}, {"f5808080", 0, 1}, {"61ff62", 1, 2},
	{"c328", 0, 2}, {"e228", 0, 2}, {"f028", 0, 2}, {"e08080", 0, 2}, {"eda080", 0, 2},
	{"f0808080", 0, 2}, {"f4908080", 0, 2},
	{"e28228", 0, 3}, {"f09f28", 0, 3}, {"f09f9828", 0, 4}, {"7878f09f9828", 2, 6},
	{"c3", 0, 2}, {"e2", 0, 2}, {"e282", 0, 3}, {"f0", 0, 2}, {"f09f", 0, 3}, {"f09f98", 0, 4},
	{"61c3", 1, 3}, {"6162e282", 2, 5}, {"6162f09f98", 2, 6},
}

// literalEndTable are the cases of the end that comparing a literal examined, as in the tests of
// the library of generated Go parsers: the literal, the input, the characters to skip, and the end in
// code points and in bytes.
var literalEndTable = [][5]any{
	{"abc", "abc", 0, 3, 3}, {"abc", "abd", 0, 3, 3}, {"abc", "xbc", 0, 1, 1}, {"abc", "ab", 0, 3, 3},
	{"abc", "a", 0, 2, 2}, {"abc", "", 0, 1, 1}, {"abc", "zzabx", 2, 5, 5}, {"abc", "zzab", 2, 5, 5},
	{"abc", "zz", 2, 3, 3}, {"abc", "zzabc", 2, 5, 5},
	{"éb", "éx", 0, 2, 3}, {"éb", "éb", 0, 2, 3}, {"éb", "é", 0, 2, 3}, {"éb", "ex", 0, 1, 1},
	{"ab", "éé", 1, 2, 3}, {"ab", "ééa", 2, 4, 6}, {"b", "é", 0, 1, 1},
}
