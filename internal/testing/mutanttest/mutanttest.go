// Package mutanttest lets a test that runs mutants, or another slow negative control, skip itself when what it
// depends on has not changed since a base revision.
//
// A mutant runner rebuilds and reruns tests for every mutant of one set of source files, so its verdict can
// change only when those files, the tests it runs against, or the runner itself change. The gate
// (hack/gate.sh) exports ZP_MUTANTS_BASE=<its base revision>; a runner then calls SkipUnlessChanged with the
// repository paths it depends on, and skips with a message that names the base when none of them differs from
// it. With ZP_MUTANTS_BASE unset, as in a plain `go test -tags zp_long`, nothing skips and every mutant runs.
//
// The rule fails closed: whenever the answer is uncertain (git fails, the base is not a revision, or a path
// is not given) the runner runs.
package mutanttest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// BaseEnv names the environment variable that holds the base revision.
const BaseEnv = "ZP_MUTANTS_BASE"

// always are the paths that every runner depends on: the module files that select the toolchain and the
// dependencies, and this package.
var always = []string{"go.mod", "go.sum", "go.work", "internal/testing/mutanttest"}

// SkipUnlessChanged skips t when ZP_MUTANTS_BASE is set and no file under any of paths, which are slash-separated
// paths from the repository root (a directory stands for everything under it), differs between that revision
// and the working tree, counting untracked files. Call it first in the test, before t.Parallel.
func SkipUnlessChanged(t testing.TB, paths ...string) {
	t.Helper()
	base := os.Getenv(BaseEnv)
	if base == "" {
		return
	}
	changed, err := Changed(base, paths...)
	if err != nil {
		t.Logf("%s=%s: cannot tell what changed (%v); running the mutants", BaseEnv, base, err)
		return
	}
	if len(changed) > 0 {
		t.Logf("%s=%s: running the mutants; changed: %s", BaseEnv, base, strings.Join(changed, " "))
		return
	}
	t.Skipf("%s=%s: none of %s changed since the base, so the mutants' verdict cannot differ", BaseEnv, base, strings.Join(paths, " "))
}

// Changed returns the files under paths (and under the paths every runner depends on) that differ between base
// and the working tree or that are untracked and not ignored. An error means the answer is unknown.
func Changed(base string, paths ...string) ([]string, error) {
	if len(paths) == 0 {
		return nil, os.ErrInvalid
	}
	topOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, err
	}
	top := strings.TrimSpace(string(topOut))
	spec := append(append([]string{}, paths...), always...)
	var files []string
	for _, args := range [][]string{
		{"diff", "--name-only", base, "--"},
		{"ls-files", "--others", "--exclude-standard", "--"},
	} {
		cmd := exec.Command("git", append([]string{"-C", top}, append(args, spec...)...)...)
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		files = append(files, strings.Fields(string(out))...)
	}
	return files, nil
}

// Pipeline are the paths that the code generators and the engine, and so the runners of their mutants, depend
// on: every module package that their tests import (go list -deps -test), and the grammars and other test
// data. TestPipelineCoversImports checks that no import leaves it. A change elsewhere (cmd, lint, the language
// server, the documentation) cannot change the verdict of such a runner.
var Pipeline = []string{
	"codegen", "core", "engine", "grammar", "sample", "schema", "testdata",
	"internal/analysis", "internal/backend", "internal/bench", "internal/bytecode", "internal/cancel", "internal/compile",
	"internal/diagnostic", "internal/driver", "internal/engine", "internal/frontend", "internal/ir", "internal/limits",
	"internal/middleend", "internal/pass", "internal/testing", "internal/value",
}
