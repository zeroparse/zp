package tsgen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// tsOperatorsGrammar exercises the paths of the Operators Rule loop: trivia that can fail after it
// consumes, operand lines with captures, actions and a cut, recovered errors inside operands and
// operators, prefix, postfix and infix operators of every associativity, operators that are
// prefixes of each other, a prefix operator that cuts, and operator parts that can match empty.
const tsOperatorsGrammar = `type Num = { V string }
type Bin = {
    L  E
    Op node[_]
    R  E
}
type Pow = {
    L  E
    Op node[_]
    R  E
    S  *node[string]
}
type Un = {
    Op node[_]
    X  E
}
type Pair = {
    L E
    R E
}
type Post = { X E }
type E ~ Num | Bin | Pair | Pow | Un | Post

@start
rule main E = e:expr ws => $e

rule expr E = #operators {
    skip (" "+ / "//" "x" / ("/" "!") #recover(skip="/" "x") "y") !"%"
    operand n:[12]+ => Num{V: text($n)}
    operand "(" e:expr ws ")" => $e
    operand "(" xs:[-!12]+ "." => Num{V: text($xs)}
    operand "{" e:expr "!}" => $e
    operand "x" #cut "y" => Num{V: "xy"}
    operand bad => Num{V: "bad"}
    operand "[" e:empty "]" => $e
    level {
        infix left "+" / "%" => Bin{L: $lhs, Op: $op, R: $rhs}
        infix left "+-" => Bin{L: $lhs, Op: $op, R: $rhs}
        infix left "|" #cut => Bin{L: $lhs, Op: $op, R: $rhs}
        infix left "/x" => Bin{L: $lhs, Op: $op, R: $rhs}
        infix left ("&" "!") #recover(skip="&" "a") => Pair{L: $lhs, R: $rhs}
    }
    level { infix none "=" => Bin{L: $lhs, Op: $op, R: $rhs} }
    level {
        prefix "-" => Un{Op: $op, X: $rhs}
        prefix "-" #cut "!" => Un{Op: $op, X: $rhs}
    }
    level { infix right ("^" / "%") s:"*"? => Pow{L: $lhs, Op: $op, R: $rhs, S: $s} }
    level {
        postfix "?" / "~"? => Post{X: $lhs}
        postfix "!" #cut "a" => Post{X: $lhs}
    }
}

rule empty E = #operators {
    operand "z"? => Num{V: "z"}
    level { infix left "#"? => Pair{L: $lhs, R: $rhs} }
}

rule bad = ("a" "!") #recover(skip="a" "?")
rule ws = " "*`

// TestTSOperatorsRuleMatchesReference parses random inputs with the Operators Rule methods printed
// from Exec IR and with the hand-written forms they replaced, kept in the test, with and without
// budget controls and with small nesting limits, and compares the values, the errors and the units
// charged.
func TestTSOperatorsRuleMatchesReference(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	g, err := parseTest(tsOperatorsGrammar)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	code = append(code, []byte(tsOperatorsReference)...)
	for file, data := range map[string][]byte{"parser.ts": code, "test.mjs": []byte(`import {testOperatorsReference} from "./parser.ts";
testOperatorsReference();
console.log("ok");
`)} {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(node, "test.mjs")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil || string(out) != "ok\n" {
		t.Fatalf("%v\n%s", err, out)
	}
}

// tsOperatorsReference holds the hand-written reset, skipTrivia, longest, apply and Operators Rule
// loop that the printed methods replaced, without the unit the loop charged per iteration, and the
// random comparison. It is appended to a bundle, which declares Parser, G, run and the rule tables.
const tsOperatorsReference = `
function refReset(p: Parser, pos: number, env: Env | null, trail: number, recovered: number): void {
  p.pos = pos;
  p.env = env;
  p.recovered.length = recovered;
  for (let i = p.trail.length - 1; i >= trail; i--) {
    p.spend();
    const u = p.trail[i]!;
    u.f[u.slot] = u.old;
  }
  p.trail.length = trail;
}
function refSkipTrivia(p: Parser, pr: OperatorsRule): void {
  if (pr.skip === null) {
    return;
  }
  const m0pos = p.pos;
  const m0env = p.env;
  const m0trail = p.trail.length;
  const m0rec = p.recovered.length;
  p.silent++;
  const v = pr.skip(p);
  p.silent--;
  if (v === undefined) {
    refReset(p, m0pos, m0env, m0trail, m0rec);
    return;
  }
  p.recovered.length = m0rec;
}
function refLongest(p: Parser, ops: OperatorsRuleOp[]): OperatorsRuleAttempt | null {
  const m0pos = p.pos;
  const m0env = p.env;
  const m0trail = p.trail.length;
  const m0rec = p.recovered.length;
  let best: OperatorsRuleAttempt | null = null;
  let cutFailed = false;
  for (const op of ops) {
    p.spend();
    const prevFrame = p.frame;
    const prevCut = p.cut;
    const f = p.newFrame(op.line.scope.length);
    p.frame = f;
    p.cut = false;
    const v = op.line.m(p);
    let ok = v !== undefined;
    const cut = p.cut;
    p.frame = prevFrame;
    p.cut = prevCut;
    if (ok && p.pos === m0pos && op.kind !== "infix") {
      ok = false; // an empty prefix or postfix match could be applied forever
    }
    if (ok && (best === null || p.pos > best.end)) {
      best = {
        op,
        frame: f,
        v: v as Node | null,
        start: m0pos,
        end: p.pos,
        env: p.env,
        cut,
        errs: p.recSince(m0rec),
      };
    }
    if (!ok && cut) {
      cutFailed = true;
    }
    if (f.length === 0) {
      refReset(p, m0pos, m0env, m0trail, m0rec);
      continue;
    }
    // reset undoes this candidate's captures; restore them. Nothing parses between saving
    // and restoring, so one buffer per parser suffices.
    p.saved.length = 0;
    for (const x of f) {
      p.spend();
      p.saved.push(x);
    }
    refReset(p, m0pos, m0env, m0trail, m0rec);
    for (let i = 0; i < f.length; i++) {
      p.spend();
      f[i] = p.saved[i]!;
    }
  }
  p.cutFailed = cutFailed;
  return best;
}
function refApply(p: Parser, a: OperatorsRuleAttempt): void {
  p.pos = a.end;
  p.env = a.env;
  p.recAdd(a.errs);
}
function refOperatorsRuleParse(p: Parser, r: Rule, min: number): R {
  if (++p.depth > p.maxDepth) {
    p.tooDeep();
  }
  const v = refOperatorsRuleExpr(p, r, min);
  p.depth--;
  return v;
}
// refNudBound is the binding level above which no operator can follow the start that the last
// refOperatorsRuleNud parsed.
let refNudBound = Infinity;
function refOperatorsRuleExpr(p: Parser, r: Rule, min: number): R {
  const pr = r.operatorsRule as OperatorsRule;
  // The right operand of an infix part that consumed no input does not apply an empty infix part
  // at its start, which would begin the same expression again without end.
  const guard = p.afterEmpty ? p.pos : -1;
  p.afterEmpty = false;
  let lhs = refOperatorsRuleNud(p, r);
  if (lhs === undefined) {
    return undefined;
  }
  if (pr.led.length === 0) {
    return lhs;
  }
  let lastNone = -1;
  let bound = refNudBound;
  for (;;) {
    const m0pos = p.pos;
    const m0env = p.env;
    const m0trail = p.trail.length;
    const m0rec = p.recovered.length;
    refSkipTrivia(p, pr);
    const a = refLongest(p, pr.led);
    if (a === null) {
      const cutFailed = p.cutFailed;
      refReset(p, m0pos, m0env, m0trail, m0rec);
      if (cutFailed) {
        return undefined;
      }
      break;
    }
    if (a.op.level <= min || a.op.level > bound || (a.op.assoc === "none" && a.op.level === lastNone)) {
      refReset(p, m0pos, m0env, m0trail, m0rec);
      break;
    }
    if (m0pos === guard && a.end === m0pos) {
      refReset(p, m0pos, m0env, m0trail, m0rec);
      break;
    }
    refApply(p, a);
    if (a.op.kind === "postfix") {
      lhs = p.operatorsRuleBuild(r, a, lhs, null);
      lastNone = -1;
      bound = Infinity;
      if (p.pos === m0pos) {
        break;
      }
      continue;
    }
    let rmin = a.op.level;
    if (a.op.assoc === "right") {
      rmin--;
    }
    p.afterEmpty = p.pos === m0pos;
    const rhs = refOperatorsRuleParse(p, r, rmin);
    if (rhs === undefined) {
      if (a.cut) {
        return undefined;
      }
      refReset(p, m0pos, m0env, m0trail, m0rec);
      break;
    }
    lhs = p.operatorsRuleBuild(r, a, lhs, rhs);
    lastNone = -1;
    if (a.op.assoc === "none") {
      lastNone = a.op.level;
    }
    // The right operand took the operators that bind tighter than rmin: the next one that does
    // was refused by it, and cannot apply to this application as a whole.
    bound = rmin;
    // An application that consumed no input ends the loop, as an empty iteration ends a
    // repetition: otherwise an operator that can match empty after an operand that can loops
    // forever.
    if (p.pos === m0pos) {
      break;
    }
  }
  return lhs;
}
function refOperatorsRuleNud(p: Parser, r: Rule): R {
  const pr = r.operatorsRule as OperatorsRule;
  refSkipTrivia(p, pr);
  const m0pos = p.pos;
  const m0env = p.env;
  const m0trail = p.trail.length;
  const m0rec = p.recovered.length;
  const a = refLongest(p, pr.prefix);
  if (a !== null) {
    refApply(p, a);
    const rhs = refOperatorsRuleParse(p, r, a.op.level);
    if (rhs !== undefined) {
      refNudBound = a.op.level;
      return p.operatorsRuleBuild(r, a, null, rhs);
    }
    if (a.cut) {
      return undefined;
    }
    refReset(p, m0pos, m0env, m0trail, m0rec);
  } else if (p.cutFailed) {
    return undefined;
  }
  for (const o of pr.operands) {
    p.spend();
    const prevFrame = p.frame;
    const prevCut = p.cut;
    const f = p.newFrame(o.scope.length);
    p.frame = f;
    p.cut = false;
    const v = o.m(p);
    const cut = p.cut;
    p.frame = prevFrame;
    p.cut = prevCut;
    if (v !== undefined) {
      refNudBound = Infinity;
      return p.lineResult(r, o, f, v, m0pos);
    }
    refReset(p, m0pos, m0env, m0trail, m0rec);
    if (cut) {
      return undefined;
    }
  }
  return undefined;
}
function refUsed(c: BudgetControl | null): number {
  return c === null ? 0 : (c as any).budgetUsed;
}

function refOutcome(input: string, maxDepth: number, controlled: boolean): string {
  const c = controlled ? new BudgetControl({signal: null, budget: 2 ** 40} as any) : null;
  const r = startIn(rules, ruleNames, starts, defaultStart, "") as Rule;
  let out: string;
  try {
    const res = run(G, r, nseen, input, CodePoints, maxDepth, c);
    out = (res.node === null ? "null" : marshal(res.node)) + " | " + (res.error === null ? "" : res.error.message);
  } catch (x) {
    out = "throw " + (x instanceof Fatal ? x.err.message : String(x));
  }
  return out + " | spent " + refUsed(c);
}

export function testOperatorsReference(): void {
  let seed = 11;
  const rnd = (n: number): number => {
    seed = (Math.imul(seed, 1103515245) + 12345) >>> 0;
    return (seed >>> 8) % n;
  };
  const alphabet = ["1", "2", "12", " ", "//", "/", "+", "+-", "%", "-", "-!", "=", "^", "^*", "*", "?", "~", "!", "!a",
    "(", ")", ".", "x", "xy", "a!", "a?", "&!", "&a", "[", "]", "z", "#", "{", "!}", "|", "/x", "/!y"];
  const pick = (xs: string[]): string => xs[rnd(xs.length)]!;
  const trivia = (): string => pick(["", "", "", " ", "  ", "//x", "//", "/!y", "/xy"]);
  // expr writes a random expression, mostly well formed, so that whole parses succeed.
  const expr = (d: number): string => {
    const k = d > 3 ? rnd(3) : rnd(10);
    switch (k) {
      case 3: return "(" + expr(d + 1) + trivia() + ")";
      case 4: return "{" + expr(d + 1) + "!}";
      case 5: return pick(["-", "-!", "-!-"]) + trivia() + expr(d + 1);
      case 6: return expr(d + 1) + trivia() + pick(["?", "~", "!a", "!"]);
      case 7: case 8: case 9:
        return expr(d + 1) + trivia() + pick(["+", "+-", "%", "=", "^", "^*", "&!", "&a", "&", "|", "/x"]) + trivia() + expr(d + 1);
      default: return pick(["1", "2", "12", "xy", "x", "a!", "a?", "a", "(-!1.", "[]", "[z]", "[#]", "[z#z]", "[##z]"]);
    }
  };
  const proto = Parser.prototype as any;
  const printed = {parse: proto.operatorsRuleParse};
  const ref = {parse: function (this: Parser, r: Rule, min: number): R { return refOperatorsRuleParse(this, r, min); }};
  let checked = 0, recovered = 0;
  // The fixed inputs reach paths that random ones reach rarely: a non-associative operator after its
  // own level, a skip that fails after recovering from an error, and a skip that fails after
  // consuming trivia (before "%").
  const fixed = ["1=2=1", "1=2+1=1", "1/x1", "1/xy1", "1/!y1", "1/xy/x1", "1 %1", "1 % 1", "1/!xy%1", "1/!x%1"];
  for (let round = 0; round < 4000 + fixed.length; round++) {
    let input = "";
    if (round >= 4000) {
      input = fixed[round - 4000]!;
    } else if (round % 2 === 0) {
      input = trivia() + expr(0) + trivia();
      // Noise makes part of the inputs fail late.
      if (rnd(3) === 0) {
        const at = rnd(input.length + 1);
        input = input.slice(0, at) + pick(alphabet) + input.slice(at);
      }
    } else {
      const n = rnd(12);
      for (let i = 0; i < n; i++) input += pick(alphabet);
    }
    const maxDepth = rnd(4) === 0 ? 1 + rnd(6) : G.maxDepth;
    for (const controlled of [false, true]) {
      proto.operatorsRuleParse = printed.parse;
      const got = refOutcome(input, maxDepth, controlled);
      proto.operatorsRuleParse = ref.parse;
      const want = refOutcome(input, maxDepth, controlled);
      proto.operatorsRuleParse = printed.parse;
      if (got !== want) {
        throw new Error("input " + JSON.stringify(input) + " depth " + maxDepth + " controlled " + controlled + ":\nprinted   " + got + "\nreference " + want);
      }
      if (!got.startsWith("null")) checked++;
      if (!got.startsWith("null") && got.includes("| spent") && !got.includes(" |  |")) recovered++;
    }
  }
  if (checked < 800 || recovered < 100) {
    throw new Error("only " + checked + " parses produced a value, " + recovered + " with recovered errors");
  }
}
`
