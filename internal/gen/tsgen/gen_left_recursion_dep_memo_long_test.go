//go:build zp_long

package tsgen_test

import (
	"os/exec"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// depMemoTSMutations are the changes of the TypeScript runtime for the negative controls of
// TestGeneratedTSLeftRecursionDependencyMemoControls, each a list of replacements: a reuse that
// does not compare the identities of the seeds read, that does not check observations, a result in
// the memo reused whatever its present set, equal step results given the same identity without
// comparing them, a jump that uses observations whose outcomes read the growing seed, a reused
// result that replays the seeds its observations read where it was made, and one that passes on only
// those seeds of its observations with current identities.
var depMemoTSMutations = map[string][][2]string{
	"seed identities": {
		{"k += this.gamma[i]!.id + \",\";\n", "k += \"0,\";\n"},
		{"if (d.kind === depSeed) k += d.val + \",\";\n", "if (d.kind === depSeed) k += \"0,\";\n"},
	},
	"observations": {
		{"const v = this.obsValue(pos, gp, present, o);\n        if (v === obsNone) {\n", "const v = o * 0 - 2;\n        if (v === obsNone) {\n"},
		{"if (d.kind === depObs) k += d.val + \",\";\n", "if (d.kind === depObs) k += \"-2,\";\n"},
		{"if (o.v !== d.val || !o.free) {", "if (false) {"},
	},
	"present growths": {
		{"return !c.ctx || (c.present & ~present) === 0;\n", "return true;\n"},
	},
	"canonical seeds": {
		{"if (c.e.errs === null && this.nodesEqual(", "if (true || this.nodesEqual("},
	},
	"jumps over observations of the growing seed": {
		{"if (o.v !== d.val || !o.free) {", "if (o.v !== d.val) {"},
	},
	"replayed observations": {
		{"buf.push({ kind: d.kind, idx: i, d: depth, val: this.gamma[i]!.id });", "buf.push({ kind: d.kind, idx: i, d: depth, val: d.val });"},
		{"this.replayObs(buf, pos, gp, present, d.site, depth);\n", "\n"},
	},
	"observations replayed in the current context": {
		{"this.replayObs(buf, pos, gp, present, d.site, depth);\n", "\n"},
	},
}

// TestGeneratedTSLeftRecursionDependencyMemoControls checks that the grammars of
// TestGeneratedTSLeftRecursionDependencyMemo detect, in generated TypeScript parsers, each check of
// the dependency memo: with the check broken, some result differs from those of the runtime as it
// is.
func TestGeneratedTSLeftRecursionDependencyMemoControls(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	cases := depMemoTSCases(t, 120)
	opts := gen.GenOptions{SourceInfo: true}
	// The parsers are generated once; each mutation of their runtime runs as a parallel subtest.
	bundles := gentest.TSBundles(t, cases, opts)
	want := gentest.TSOutputs(t, node, cases, bundles, nil)
	for name, reps := range depMemoTSMutations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := gentest.TSOutputs(t, node, cases, bundles, func(src string) string {
				for _, r := range reps {
					if strings.Count(src, r[0]) != 1 {
						t.Fatalf("mutation %s: %q is not in the runtime once", name, r[0])
					}
					src = strings.Replace(src, r[0], r[1], 1)
				}
				return src
			})
			if got == want {
				t.Errorf("%s: the mutation went undetected", name)
			}
		})
	}
}
