//go:build zp_long

package gogen_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// engineObservationRun is a test that an engine mutant runs, and what the test must then report (want),
// or "" if it must pass.
type engineObservationRun struct{ pkg, test, want string }

// engineObservationMutant changes a file of the engine (file, relative to the module root) by replacing
// old with new, and runs tests with the change through the overlay of the go command.
type engineObservationMutant struct {
	name, file, old, new string
	runs                 []engineObservationRun
}

var (
	agreementRun     = engineObservationRun{"./internal/codegen/go", "TestEngineObservationsAgreeWithGenerated", ""}
	discardRun       = engineObservationRun{"./internal/engine", "TestProfileStreamMemory", ""}
	engineOptionsRun = engineObservationRun{"./engine", "TestTraceOptions", ""}
	// The generated parsers of the agreement use a copy of the library that the overlay does not
	// change; a mutant of the library changes that copy too (agreeLibPatchEnv).
	libOptionsRun = engineObservationRun{"./codegen/go/lib", "TestTraceOption(Selection|Composition)", ""}
)

func expecting(r engineObservationRun, want string) engineObservationRun { r.want = want; return r }

var engineObservationMutants = []engineObservationMutant{
	{name: "a memo hit records nothing", file: "internal/engine/state/call_exec.go",
		old: "\t\tp.touch(e.Examined)\n\t\tif e.From < e.Pos {", new: "\t\tif e.From < e.Pos {",
		runs: []engineObservationRun{expecting(agreementRun, "examined end of")}},
	{name: "lookahead flipped", file: "internal/engine/state/trace.go",
		old: "Pos: p.Pos, Lookahead: p.Silent > 0}", new: "Pos: p.Pos, Lookahead: p.Silent == 0}",
		runs: []engineObservationRun{expecting(agreementRun, "lookahead of the call of")}},
	{name: "profiles not told of discarded input", file: "internal/engine/state/stream_ops.go",
		old: "\t\tp.Tr.discard(pos)\n", new: "",
		runs: []engineObservationRun{expecting(discardRun, "heap")}},
	// The trace options: a producer that ignores the trace filter or the depth limit, keeps only the
	// last trace function or profile, or lets a Document's parses share what they add to its defaults.
	{name: "engine trace filter ignored", file: "internal/engine/state/trace.go",
		old: "selected := t.rules == nil || t.rules[r.Name] ||", new: "selected := len(t.rules) >= 0 || t.rules[r.Name] ||",
		runs: []engineObservationRun{expecting(agreementRun, "the trace filter"), expecting(engineOptionsRun, "filtered events")}},
	{name: "engine depth limit ignored", file: "internal/engine/state/trace.go",
		old: "deliver := t.fn != nil && selected && (t.maxDepth <= 0 || ev.Depth <= t.maxDepth)", new: "deliver := t.fn != nil && selected",
		runs: []engineObservationRun{expecting(agreementRun, "the trace filter"), expecting(engineOptionsRun, "filtered events")}},
	{name: "engine keeps the last trace function", file: "engine/engine.go",
		old: "o.Trace = engine.ComposeTrace(prev, f)", new: "o.Trace = f",
		runs: []engineObservationRun{expecting(engineOptionsRun, "the second trace function")}},
	{name: "engine keeps the last profile", file: "engine/engine.go",
		old: "o.Profiles = append(slices.Clip(o.Profiles), prof)", new: "o.Profiles = append(o.Profiles[:0:0], prof)",
		runs: []engineObservationRun{expecting(engineOptionsRun, "the second profile")}},
	{name: "generated trace filter ignored", file: "codegen/go/lib/trace.go",
		old: "return len(o.rules) == 0 || o.rules[rule] ||", new: "return len(o.rules) >= 0 || o.rules[rule] ||",
		runs: []engineObservationRun{expecting(agreementRun, "generated: the trace filter"), expecting(libOptionsRun, "pair and nested: got")}},
	{name: "generated depth limit ignored", file: "codegen/go/lib/trace.go",
		old: "if o.trace == nil || !selected || o.maxDepth > 0 && e.Depth > o.maxDepth {", new: "if o.trace == nil || !selected {",
		runs: []engineObservationRun{expecting(agreementRun, "generated: the trace filter"), expecting(libOptionsRun, "depth: got")}},
	{name: "generated keeps the last trace function", file: "codegen/go/lib/trace.go",
		old: "\t\t\t\tprev(e)\n", new: "\t\t\t\t_ = prev\n",
		runs: []engineObservationRun{expecting(agreementRun, "generated: the second trace function"), expecting(libOptionsRun, "trace functions received")}},
	{name: "generated keeps the last profile", file: "codegen/go/lib/trace.go",
		old: "x.more = append(slices.Clip(x.more), profile)", new: "x.more, o.observe.profile = slices.Clip(x.more), profile",
		runs: []engineObservationRun{expecting(agreementRun, "generated: the second profile"), expecting(libOptionsRun, "profiles recorded")}},
	{name: "generated Document parses share added profiles", file: "codegen/go/lib/trace.go",
		old: "x.more = append(slices.Clip(x.more), profile)", new: "_ = slices.Clip[[]int]\n\t\t\tx.more = append(x.more, profile)",
		runs: []engineObservationRun{expecting(libOptionsRun, "was replaced")}},
	// The control changes nothing that the tests observe: they must pass with it, so that what makes the
	// other mutants fail is their change, not the overlay.
	{name: "control", file: "internal/engine/state/trace.go",
		old: "\tt.exp = nil\n", new: "\tt.exp = nil\n\t_ = t\n",
		runs: []engineObservationRun{agreementRun, discardRun, engineOptionsRun, libOptionsRun}},
	{name: "library control", file: "codegen/go/lib/trace.go",
		old: "\tx := new(observeExtras)\n", new: "\tx := new(observeExtras)\n\t_ = x\n",
		runs: []engineObservationRun{agreementRun, libOptionsRun}},
}

// TestEngineObservationMutants checks that the agreement of the engine's observations with the generated
// parsers' and the bound on the profile of a stream notice an engine that leaves out the examined end of a
// memo hit, inverts Lookahead or keeps the profile's records of discarded input, and that the agreement and
// the tests of the trace options notice either producer getting one of them wrong. A mutant of the library
// also changes the copy of it that the agreement's generated parsers import.
func TestEngineObservationMutants(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds the engine again for each mutant")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range engineObservationMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, filepath.FromSlash(m.file))
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(src), m.old) != 1 {
				t.Fatalf("%q occurs %d times in %s", m.old, strings.Count(string(src), m.old), m.file)
			}
			dir := t.TempDir()
			mutated := filepath.Join(dir, filepath.Base(path))
			if err := os.WriteFile(mutated, []byte(strings.Replace(string(src), m.old, m.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]map[string]string{"Replace": {path: mutated}})
			overlayFile := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			for _, r := range m.runs {
				cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-count=1", "-overlay="+overlayFile, "-run", "^"+r.test+"$", r.pkg)
				cmd.Dir = root
				// The tests run in the module itself, as go test runs them here.
				cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
				if strings.HasPrefix(m.file, "codegen/go/lib/") {
					patch, _ := json.Marshal(struct{ File, Old, New string }{m.file, m.old, m.new})
					cmd.Env = append(cmd.Env, agreeLibPatchEnv+"="+string(patch))
				}
				out, err := cmd.CombinedOutput()
				switch {
				case r.want == "" && err != nil:
					t.Errorf("%s fails with the mutant: %v\n%s", r.test, err, out)
				case r.want != "" && err == nil:
					t.Errorf("%s passes with the mutant", r.test)
				case r.want != "" && !strings.Contains(string(out), r.want):
					t.Errorf("%s fails with the mutant without reporting %q:\n%s", r.test, r.want, out)
				}
			}
		})
	}
}
