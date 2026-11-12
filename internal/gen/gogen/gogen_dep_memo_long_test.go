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

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// depMemoMutations are the changes of the embedded runtime for the negative controls of
// TestGeneratedLeftRecursionDependencyMemoControls, each a list of replacements: a reuse that does
// not compare the identities of the seeds read, that does not check observations, a result in the
// memo reused whatever its present set, equal step results given the same identity without
// comparing them, a jump that uses observations whose outcomes read the growing seed, a reused
// result that replays the seeds its observations read where it was made, and one that passes on only
// those seeds of its observations with current identities.
var depMemoMutations = map[string][][2]string{
	"seed identities": {
		{"rest = k.add(p.gamma[i].id, rest)\n", "rest = k.add(0, rest)\n"},
		{"if d.kind == depSeed {\n\t\t\trest = k.add(d.val, rest)\n", "if d.kind == depSeed {\n\t\t\trest = k.add(0, rest)\n"},
	},
	"observations": {
		{"v := p.obsValue(pos, gp, h.present, o)\n", "v := obsUnique + 0*uint64(o)\n"},
		{"if d.kind == depObs {\n\t\t\trest = k.add(d.val, rest)\n", "if d.kind == depObs {\n\t\t\trest = k.add(obsUnique, rest)\n"},
		{"if v, free := p.obsFree(g.start, g.gp, g.present, d.site, g.growKey()); v != d.val || !free {", "if false {"},
	},
	"present growths": {
		{"return !c.ctx || c.known && c.present&^present == 0\n", "return true\n"},
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
}

// TestGeneratedLeftRecursionDependencyMemoControls checks that the grammars of
// TestGeneratedLeftRecursionDependencyMemo detect, in generated Go parsers with and without the
// Exec IR compositions, each check of the dependency memo: with the check broken, some result
// differs from those of the runtime as it is.
func TestGeneratedLeftRecursionDependencyMemoControls(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	// The mutated runtimes have no limit controls.
	gentest.PinOrdinary(t)
	pkgs, rt := depMemoPackages(t, depMemoGenCases(t, 120))
	// Every program is built and run by its own go command. They share one build cache, so the
	// packages that a mutation leaves alone are built once, and run concurrently up to a bound
	// that keeps the memory of their linkers in check.
	sem := make(chan struct{}, 6)
	type run struct {
		flags, mutation string
		out             string
		err             error
	}
	var runs []*run
	for _, flags := range []string{"-mod=mod -trimpath"} {
		runs = append(runs, &run{flags: flags})
		for name := range depMemoMutations {
			runs = append(runs, &run{flags: flags, mutation: name})
		}
	}
	var wg sync.WaitGroup
	for _, r := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r.out, r.err = depMemoOutcomes(goBin, pkgs, rt, r.mutation, r.flags)
		}()
	}
	wg.Wait()
	want := map[string]string{}
	for _, r := range runs {
		if r.err != nil {
			t.Fatalf("GOFLAGS %q: mutation %q: %v", r.flags, r.mutation, r.err)
		}
		if r.mutation == "" {
			want[r.flags] = r.out
		}
	}
	for _, r := range runs {
		if r.mutation != "" && r.out == want[r.flags] {
			t.Errorf("GOFLAGS %q: %s: the mutation went undetected", r.flags, r.mutation)
		}
	}
}

// depMemoPackage is a generated package of a grammar of the dependency memo tests, without the
// embedded runtime, and the calls that parse its inputs.
type depMemoPackage struct {
	name  string
	files []gen.GeneratedFile
	calls string
}

// depMemoPackages generates the parsers of cases, untyped and typed, with the embedded runtime.
// The runtime does not depend on the grammar, so the packages import one copy of it, at
// depmemo/internal/runtime, which depMemoOutcomes writes (changed by a mutation) once: the go
// command then builds the runtime once per program rather than once per grammar. The runtime files
// are the second result.
func depMemoPackages(t *testing.T, cases []gentest.Case) ([]depMemoPackage, []gen.GeneratedFile) {
	t.Helper()
	var pkgs []depMemoPackage
	var rt []gen.GeneratedFile
	for i, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		pkg := fmt.Sprintf("g%d", i)
		files, err := gentest.Generate(t, g, gen.GenOptions{Module: "depmemo/" + pkg, Package: pkg, Start: "main", Types: true, SourceInfo: true, Runtime: gen.RuntimeEmbed})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		p := depMemoPackage{name: pkg}
		var thisRT []gen.GeneratedFile
		for _, f := range files {
			if strings.HasPrefix(f.Path, "internal/runtime/") {
				thisRT = append(thisRT, f)
				continue
			}
			f.Content = bytes.ReplaceAll(f.Content, []byte(`"depmemo/`+pkg+`/internal/runtime"`), []byte(`"depmemo/internal/runtime"`))
			p.files = append(p.files, f)
		}
		if rt == nil {
			rt = thisRT
		} else if len(rt) != len(thisRT) {
			t.Fatalf("%s: the embedded runtime has %d files, that of %s has %d", c.Name, len(thisRT), cases[0].Name, len(rt))
		} else {
			for k := range rt {
				if rt[k].Path != thisRT[k].Path || !bytes.Equal(rt[k].Content, thisRT[k].Content) {
					t.Fatalf("%s: the embedded runtime file %s differs from that of %s", c.Name, rt[k].Path, cases[0].Name)
				}
			}
		}
		var calls strings.Builder
		for _, in := range c.Inputs {
			fmt.Fprintf(&calls, "\tshow(%s.Parse(%q))\n\tshow(%[1]s.ParseAST(%[2]q))\n", pkg, in)
		}
		p.calls = calls.String()
		pkgs = append(pkgs, p)
	}
	return pkgs, rt
}

// depMemoOutcomes writes the packages of depMemoPackages, with the runtime changed by the mutation
// (none if ""), parses their inputs, untyped and typed, with a program built with the GOFLAGS flags
// and returns the outcomes. The values record the rules that produced them: two step results that
// differ only in that are different seeds.
func depMemoOutcomes(goBin string, pkgs []depMemoPackage, rt []gen.GeneratedFile, mutation, flags string) (string, error) {
	dir, err := os.MkdirTemp("", "depmemo-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	write := func(name string, content []byte) error {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, content, 0o644)
	}
	if err := write("go.mod", []byte("module depmemo\n\ngo 1.27.1\n")); err != nil {
		return "", err
	}
	reps := depMemoMutations[mutation]
	applied := make([]bool, len(reps))
	for _, f := range rt {
		content := string(f.Content)
		if mutation != "" {
			for k, r := range reps {
				if n := strings.Count(content, r[0]); n > 1 {
					return "", fmt.Errorf("mutation %s: %q is not unique", mutation, r[0])
				} else if n == 1 {
					content = strings.Replace(content, r[0], r[1], 1)
					applied[k] = true
				}
			}
		}
		if err := write(f.Path, []byte(content)); err != nil {
			return "", err
		}
	}
	for k, ok := range applied {
		if !ok {
			return "", fmt.Errorf("mutation %s: %q not in the runtime", mutation, reps[k][0])
		}
	}
	var imports, calls strings.Builder
	for _, p := range pkgs {
		for _, f := range p.files {
			if err := write(p.name+"/"+f.Path, f.Content); err != nil {
				return "", err
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
		return "", err
	}
	cmd := exec.Command(goBin, "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS="+flags, "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v\n%s", err, out)
	}
	return string(out), nil
}
