//go:build zp_long

package gogen_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// depMemoMutations are the changes of the embedded library for the negative controls of
// TestGeneratedLeftRecursionDependencyMemoControls, each a list of replacements: a reuse that does
// not compare the identities of the seeds read, that does not check observations, a result in the
// memo reused whatever its present set, equal step results given the same identity without
// comparing them, a jump that uses observations whose outcomes read the growing seed, a reused
// result that replays the seeds its observations read where it was made, and one that passes on only
// those seeds of its observations with current identities, a result with more dependencies than a
// key holds that is not stored, and one identity for every present set of leaders without bits.
var depMemoMutations = map[string][][2]string{
	"seed identities": {
		{"rest = k.add(p.gamma[i].id, rest)\n", "rest = k.add(0, rest)\n"},
		{"if d.kind == depSeed {\n\t\t\trest = k.add(d.val, rest)\n", "if d.kind == depSeed {\n\t\t\trest = k.add(0, rest)\n"},
	},
	"observations": {
		{"v := p.obsValue(pos, gp, h.present, o)\n", "v := obsWide + 0*uint64(o)\n"},
		{"if d.kind == depObs {\n\t\t\trest = k.add(d.val, rest)\n", "if d.kind == depObs {\n\t\t\trest = k.add(obsWide, rest)\n"},
		{"if v, free := p.obsFree(g.start, g.gp, g.present, d.site, g.growKey()); v != d.val || !free {", "if false {"},
	},
	"present growths": {
		{"return !c.ctx || c.present&^present == 0\n", "return true\n"},
	},
	"canonical seeds": {
		{"if c.e.examined == e.examined && c.e.errs.none() && p.nodesEqual(", "if true || p.nodesEqual("},
	},
	"jumps over observations of the growing seed": {
		{"v != d.val || !free {", "v != d.val || !free && false {"},
	},
	"replayed observations": {
		{"buf = append(buf, dep{kind: d.kind, idx: int32(i), d: depth, val: p.gamma[i].id})", "buf = append(buf, dep{kind: d.kind, idx: int32(i), d: depth, val: d.val})"},
		{"buf = p.replayObs(buf, pos, gp, present, d.site, depth)\n", "\n"},
	},
	"observations replayed in the current context": {
		{"buf = p.replayObs(buf, pos, gp, present, d.site, depth)\n", "\n"},
	},
	"dependency cap": {
		{"\t\tif d.kind == depSeed {\n\t\t\tseeds = append(seeds, growKey{d.rule, d.min})", "\t\tif len(seeds)+len(obs) == maxCtxDeps {\n\t\t\treturn\n\t\t}\n\t\tif d.kind == depSeed {\n\t\t\tseeds = append(seeds, growKey{d.rule, d.min})"},
	},
	"wide present identity": {
		{"return presentWide | uint32(p.intern(key))", "return presentWide | 1 + 0*uint32(len(key))"},
	},
}

// TestGeneratedLeftRecursionDependencyMemoControls checks that the grammars of
// TestGeneratedLeftRecursionDependencyMemo detect, in generated Go parsers with and without the
// Exec IR compositions, each check of the dependency memo: with the check broken, some result
// differs from those of the library as it is.
func TestGeneratedLeftRecursionDependencyMemoControls(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	// The mutated libraries have no limit controls.
	gentest.PinOrdinary(t)
	pkgs, rt := depMemoPackages(t, depMemoGenCases(t, 120))
	// Every program is built by its own go command. They share one build cache, so the packages
	// that a mutation leaves alone are built once, and the mutations run concurrently up to a
	// bound that keeps the memory of their linkers in check. The library as it is runs first: its run time bounds those of the mutations, which may
	// make the parsers exponential (the dependency cap does). A run stopped at the bound counts
	// as a difference.
	const flags = "-mod=mod -trimpath"
	want, took, stopped, err := depMemoOutcomes(t, goBin, pkgs, rt, "", flags, 0)
	if err != nil || stopped {
		t.Fatalf("GOFLAGS %q: the library as it is: stopped %v: %v", flags, stopped, err)
	}
	limit := depMemoMutantLimit(took)
	t.Logf("the library as it is ran %v; each mutation runs at most %v", took, limit)
	sem := make(chan struct{}, 6)
	type run struct {
		mutation string
		out      string
		stopped  bool
		err      error
	}
	var runs []*run
	for name := range depMemoMutations {
		runs = append(runs, &run{mutation: name})
	}
	var wg sync.WaitGroup
	for _, r := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r.out, _, r.stopped, r.err = depMemoOutcomes(t, goBin, pkgs, rt, r.mutation, flags, limit)
		}()
	}
	wg.Wait()
	for _, r := range runs {
		switch {
		case r.err != nil:
			t.Errorf("GOFLAGS %q: mutation %q: %v", flags, r.mutation, r.err)
		case r.stopped:
			t.Logf("GOFLAGS %q: %s: detected: the run did not end within %v", flags, r.mutation, limit)
		case r.out == want:
			t.Errorf("GOFLAGS %q: %s: the mutation went undetected", flags, r.mutation)
		}
	}
}

// depMemoMutantLimit bounds the run of a mutated library of the dependency memo controls, given
// how long the library as it is took on the same inputs: far more than the slowdown that a
// loaded machine causes, and far less than an exponential run takes.
func depMemoMutantLimit(took time.Duration) time.Duration {
	return time.Minute + 20*took
}

// depMemoPackage is a generated package of a grammar of the dependency memo tests, without the
// embedded library, and the calls that parse its inputs.
type depMemoPackage struct {
	name  string
	files []codegen.GeneratedFile
	calls string
}

// depMemoPackages generates the parsers of cases, untyped and typed, with the embedded library.
// The library does not depend on the grammar, so the packages import one copy of it, at
// depmemo/internal/libgo, which depMemoOutcomes writes (changed by a mutation) once: the go
// command then builds the library once per program rather than once per grammar. The library files
// are the second result.
func depMemoPackages(t *testing.T, cases []gentest.Case) ([]depMemoPackage, []codegen.GeneratedFile) {
	t.Helper()
	var pkgs []depMemoPackage
	var rt []codegen.GeneratedFile
	for i, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		pkg := fmt.Sprintf("g%d", i)
		files, err := gentest.Generate(t, g, codegen.GenOptions{Module: "depmemo/" + pkg, Package: pkg, Start: "main", Types: true, SourceInfo: true, Lib: codegen.LibEmbed})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		p := depMemoPackage{name: pkg}
		var thisRT []codegen.GeneratedFile
		for _, f := range files {
			// The parser names the copy of the library by this package's path: all share one copy at
			// depmemo/internal/libgo.
			f.Content = bytes.ReplaceAll(f.Content, []byte(`"depmemo/`+pkg+`/internal/libgo"`), []byte(`"depmemo/internal/libgo"`))
			if strings.HasPrefix(f.Path, "internal/libgo/") {
				thisRT = append(thisRT, f)
				continue
			}
			p.files = append(p.files, f)
		}
		if rt == nil {
			rt = thisRT
		} else if len(rt) != len(thisRT) {
			t.Fatalf("%s: the embedded library has %d files, that of %s has %d", c.Name, len(thisRT), cases[0].Name, len(rt))
		} else {
			for k := range rt {
				if rt[k].Path != thisRT[k].Path || !bytes.Equal(rt[k].Content, thisRT[k].Content) {
					t.Fatalf("%s: the embedded library file %s differs from that of %s", c.Name, rt[k].Path, cases[0].Name)
				}
			}
		}
		var calls strings.Builder
		for _, in := range c.Inputs {
			fmt.Fprintf(&calls, "\tshow(%s.Parse(%q))\n\tshow(%[1]s.ParseValue(%[2]q))\n", pkg, in)
		}
		p.calls = calls.String()
		pkgs = append(pkgs, p)
	}
	return pkgs, rt
}

// depMemoOutcomes writes the packages of depMemoPackages, with the library changed by the mutation
// (none if ""), parses their inputs, untyped and typed, with a program built with the GOFLAGS flags
// and returns the outcomes. The values record the rules that produced them: two step results that
// differ only in that are different seeds. The program runs for at most limit (no bound if
// limit <= 0) and is killed before the test's deadline. A stop at the bound is reported as
// stopped, with the outcomes printed by then; a stop at the deadline is an error. The results also
// give how long it ran.
func depMemoOutcomes(t *testing.T, goBin string, pkgs []depMemoPackage, rt []codegen.GeneratedFile, mutation, flags string, limit time.Duration) (string, time.Duration, bool, error) {
	dir, err := os.MkdirTemp("", "depmemo-")
	if err != nil {
		return "", 0, false, err
	}
	defer os.RemoveAll(dir)
	write := func(name string, content []byte) error {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, content, 0o644)
	}
	if err := write("go.mod", []byte(gentest.GoMod("depmemo"))); err != nil {
		return "", 0, false, err
	}
	reps := depMemoMutations[mutation]
	applied := make([]bool, len(reps))
	for _, f := range rt {
		content := string(f.Content)
		if mutation != "" {
			for k, r := range reps {
				if n := strings.Count(content, r[0]); n > 1 {
					return "", 0, false, fmt.Errorf("mutation %s: %q is not unique", mutation, r[0])
				} else if n == 1 {
					content = strings.Replace(content, r[0], r[1], 1)
					applied[k] = true
				}
			}
		}
		if err := write(f.Path, []byte(content)); err != nil {
			return "", 0, false, err
		}
	}
	for k, ok := range applied {
		if !ok {
			return "", 0, false, fmt.Errorf("mutation %s: %q not in the library", mutation, reps[k][0])
		}
	}
	var imports, calls strings.Builder
	for _, p := range pkgs {
		for _, f := range p.files {
			if err := write(p.name+"/"+f.Path, f.Content); err != nil {
				return "", 0, false, err
			}
		}
		fmt.Fprintf(&imports, "\t%q\n", "depmemo/"+p.name)
		calls.WriteString(p.calls)
	}
	if err := write("main.go", []byte(`package main

import (
	"encoding/json"
	"fmt"
`+imports.String()+`)

func show(n any, err error) {
	j, _ := json.Marshal(n)
	fmt.Println(string(j), err)
}

func main() {
`+calls.String()+`}
`)); err != nil {
		return "", 0, false, err
	}
	// The program is built and then run, not run by go run, so that the process that a stop kills
	// is the program itself.
	buildCtx, cancelBuild := gentest.ChildContext(t, 0)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, goBin, "build", "-o", "depmemo.bin", ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOFLAGS="+flags, "GOTOOLCHAIN=local")
	build.WaitDelay = gentest.ChildWaitDelay
	if out, err := build.CombinedOutput(); err != nil {
		return "", 0, false, fmt.Errorf("go build: %v\n%s", err, out)
	}
	ctx, cancel := gentest.ChildContext(t, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(dir, "depmemo.bin"))
	cmd.Dir = dir
	cmd.WaitDelay = gentest.ChildWaitDelay
	start := time.Now()
	out, err := cmd.CombinedOutput()
	took := time.Since(start)
	if err != nil && gentest.ChildStoppedAtBound(ctx) {
		return string(out), took, true, nil
	}
	if err != nil && ctx.Err() != nil {
		return "", took, false, fmt.Errorf("stopped by the test's deadline, not at its bound (%v); raise -timeout: %v\n%s", limit, err, out)
	}
	if err != nil {
		return "", took, false, fmt.Errorf("%v\n%s", err, out)
	}
	return string(out), took, false, nil
}
