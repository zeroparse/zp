package tsgen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestTSKeepAndMergeMatchReference runs random sequences of the operations on the expectation
// record of the TypeScript runtime through the methods printed from Exec IR (keep, mergeExpected)
// and through the hand-written forms they replaced, kept in the test, in parses that record
// expectations and parses that do not, with and without budget controls. After each operation it
// compares the record (farthest position, base, top and the stack below the top), the set kept
// last, whether a kept set is the one kept last, and the units charged.
func TestTSKeepAndMergeMatchReference(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	g, err := parseTest(`rule main = "a"*`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gentest.GenerateTSBundle(g, gen.GenOptions{Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	code = append(code, []byte(tsExpectedReference)...)
	for file, data := range map[string][]byte{"parser.ts": code, "test.mjs": []byte(`import {testExpectedReference} from "./parser.ts";
testExpectedReference();
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

// tsExpectedReference holds the hand-written keep and mergeExpected that the printed methods
// replaced, and the random comparison. It is appended to a bundle, which declares Parser, G,
// BudgetControl, noExp, expFar and copyIDsSpend.
const tsExpectedReference = `
function refKeep(p: Parser, from: number, to: number): number[] {
  if (from === to) {
    return noExp;
  }
  const last = p.lastKept;
  if (last.length === to - from) {
    let same = true;
    for (let i = 0; i < last.length; i++) {
      p.spend();
      if (last[i] !== p.exp[from + i]) {
        same = false;
        break;
      }
    }
    if (same) {
      return last;
    }
  }
  p.lastKept = p.control === null ? p.exp.slice(from, to) : copyIDsSpend(p.exp, from, to, p.control);
  return p.lastKept;
}

function refMergeExpected(p: Parser, far: number, from: number, to: number): void {
  for (let i = from; i < to; i++) {
    p.spend();
    p.expect(far, p.exp[i]!);
  }
}

function used(p: Parser): number {
  return p.control === null ? 0 : (p.control as any).budgetUsed;
}

export function testExpectedReference(): void {
  let seed = 7;
  const rnd = (n: number): number => {
    seed = (Math.imul(seed, 1103515245) + 12345) >>> 0;
    return (seed >>> 8) % n;
  };
  for (const recording of [false, true]) {
    for (const controlled of [false, true]) {
      for (let round = 0; round < 300; round++) {
        const mk = (): Parser => {
          const c = controlled ? new BudgetControl({signal: null, budget: 2 ** 40} as any) : null;
          const p = new Parser(G, "", CodePoints, 1, G.maxDepth, c);
          p.expectations = recording;
          return p;
        };
        const a = mk(), b = mk();
        const marks: number[][] = [];
        const check = (step: number, what: string): void => {
          let same = a.farthest === b.farthest && a.expBase === b.expBase && a.expTop === b.expTop &&
            a.silent === b.silent && used(a) === used(b) && a.lastKept.length === b.lastKept.length;
          for (let i = 0; same && i < a.expTop; i++) same = a.exp[i] === b.exp[i];
          for (let i = 0; same && i < a.lastKept.length; i++) same = a.lastKept[i] === b.lastKept[i];
          if (!same) {
            throw new Error("recording " + recording + " controlled " + controlled + " round " + round + " step " + step +
              " after " + what + ": far " + a.farthest + "/" + b.farthest + " base " + a.expBase + "/" + b.expBase +
              " top " + a.expTop + "/" + b.expTop + " exp " + a.exp.slice(0, a.expTop) + " / " + b.exp.slice(0, b.expTop) +
              " used " + used(a) + "/" + used(b) + " kept " + a.lastKept + " / " + b.lastKept);
          }
        };
        // end ends the isolation m of p as unisolateMerge does, merging with merge, or keeping the
        // set with keep and merging it then, as a memo entry does.
        const end = (p: Parser, m: number, keep: boolean, merge: (far: number, from: number, to: number) => void,
          kept: (from: number, to: number) => number[]): number[] | null => {
          const far = p.farthest, from = p.expBase, to = p.expTop;
          p.expTop = from;
          p.expBase = p.isoStack[m + 1]!;
          p.farthest = p.isoStack[m]!;
          p.isoStack.length = m;
          const k = keep ? kept(from, to) : null;
          merge(far, from, to);
          return k;
        };
        for (let step = 0; step < 60; step++) {
          const pos = rnd(6);
          const id = rnd(5) === 0 ? expFar : 1 + rnd(90);
          switch (rnd(7)) {
            case 0: case 1:
              a.expect(pos, id);
              b.expect(pos, id);
              check(step, "expect");
              break;
            case 2:
              if (marks.length < 4) {
                a.pos = pos;
                b.pos = pos;
                marks.push([a.isolate(), b.isolate()]);
              }
              check(step, "isolate");
              break;
            case 3: {
              const m = marks.pop();
              if (m !== undefined) {
                const keep = rnd(3) !== 0;
                const ka = end(a, m[0]!, keep, (f, x, y) => a.mergeExpected(f, x, y), (x, y) => a.keep(x, y));
                const kb = end(b, m[1]!, keep, (f, x, y) => refMergeExpected(b, f, x, y), (x, y) => refKeep(b, x, y));
                if (keep && (ka!.join() !== kb!.join() || (ka === a.lastKept) !== (kb === b.lastKept) ||
                  (ka === noExp) !== (kb === noExp))) {
                  throw new Error("keep differs at step " + step + ": " + ka + " / " + kb);
                }
              }
              check(step, "unisolate");
              break;
            }
            case 4: {
              // Keep the current record twice, so that the second keep finds the first.
              for (let i = 0; i < 2; i++) {
                const ka = a.keep(a.expBase, a.expTop), kb = refKeep(b, b.expBase, b.expTop);
                if (ka.join() !== kb.join() || (ka === a.lastKept) !== (kb === b.lastKept)) {
                  throw new Error("keep differs at step " + step);
                }
              }
              check(step, "keep");
              break;
            }
            case 5:
              a.silent = rnd(2);
              b.silent = a.silent;
              check(step, "silent");
              break;
            case 6: {
              // Merge a set held above the top of the stack, as a finished isolation leaves it.
              const ids = [id, 1 + rnd(90), expFar].slice(0, rnd(4));
              for (const p of [a, b]) {
                for (let i = 0; i < ids.length; i++) p.exp[p.expTop + 8 + i] = ids[i]!;
              }
              a.mergeExpected(pos, a.expTop + 8, a.expTop + 8 + ids.length);
              refMergeExpected(b, pos, b.expTop + 8, b.expTop + 8 + ids.length);
              check(step, "mergeExpected");
              break;
            }
          }
        }
      }
    }
  }
}
`
