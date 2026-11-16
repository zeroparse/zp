package archtest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// modulePath is the path of the module whose graph the test checks.
const modulePath = "zeroparse.dev/zp"

// moduleRoot is the root directory of the module, relative to the directory of this package.
const moduleRoot = "../../.."

// A pkg is a package of a build's import graph.
type pkg struct {
	ImportPath string
	Imports    []string // imports of the non-test files
	Standard   bool
	Module     *struct{ Path string }
}

// A graph is the import graph of one build: every package of the module and its dependencies.
type graph struct {
	tags string // the build tag, "" for the default build
	pkgs map[string]*pkg
}

// inModule reports whether the import path p is a package of the module.
func inModule(p string) bool {
	return p == modulePath || strings.HasPrefix(p, modulePath+"/")
}

// rel returns the module-relative path of the module package p.
func rel(p string) string {
	if p == modulePath {
		return "."
	}
	return strings.TrimPrefix(p, modulePath+"/")
}

// buildTag matches a zp_ build tag in a build constraint.
var buildTag = regexp.MustCompile(`zp_[a-z0-9_]*`)

// referenceTags returns the tags that hack/test-reference-tags.sh checks without arguments: every
// zp_ tag named in a build constraint of a Go file outside the dot directories, except zp_long.
func referenceTags(root string) ([]string, error) {
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(data, []byte("zp_")) {
			return nil
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "//go:build ") && strings.Contains(line, "zp_") {
				for _, t := range buildTag.FindAllString(line, -1) {
					seen[t] = true
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		return nil, err
	}
	delete(seen, "zp_long")
	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	slices.Sort(tags)
	return tags, nil
}

// loadGraph runs go list for the module under the build tag tags.
func loadGraph(root, tags string) (*graph, error) {
	args := []string{"list", "-deps", "-json=ImportPath,Imports,Standard,Module"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "./...")
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	// The workspace adds the site module, whose packages are not part of this module's layout.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %v\n%s", strings.Join(args, " "), err, stderr.Bytes())
	}
	g := &graph{tags: tags, pkgs: map[string]*pkg{}}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		p := new(pkg)
		if err := dec.Decode(p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list -tags %q: %v", tags, err)
		}
		g.pkgs[p.ImportPath] = p
	}
	return g, nil
}

// loadGraphs loads the graph of the default build and of each tag, a few at a time.
func loadGraphs(root string, tags []string) ([]*graph, error) {
	all := append([]string{""}, tags...)
	graphs := make([]*graph, len(all))
	errs := make([]error, len(all))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, t := range all {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			graphs[i], errs[i] = loadGraph(root, t)
		})
	}
	wg.Wait()
	return graphs, errors.Join(errs...)
}

// clone returns a copy of g whose packages can be changed without changing g.
func (g *graph) clone() *graph {
	c := &graph{tags: g.tags, pkgs: make(map[string]*pkg, len(g.pkgs))}
	for k, p := range g.pkgs {
		q := *p
		q.Imports = slices.Clone(p.Imports)
		c.pkgs[k] = &q
	}
	return c
}

// shortestChain returns the shortest import chain from one of the packages from to the package
// to, following only the edges that follow accepts, or nil when there is none. Among chains of
// the same length it returns the one that starts at the first package of from.
func (g *graph) shortestChain(from []string, to string, follow func(a, b string) bool) []string {
	prev := map[string]string{}
	queue := []string{}
	for _, f := range from {
		if _, ok := prev[f]; !ok {
			prev[f] = ""
			queue = append(queue, f)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n == to {
			var chain []string
			for ; n != ""; n = prev[n] {
				chain = append(chain, n)
			}
			slices.Reverse(chain)
			return chain
		}
		p := g.pkgs[n]
		if p == nil {
			continue
		}
		for _, m := range p.Imports {
			if _, ok := prev[m]; ok || !follow(n, m) {
				continue
			}
			prev[m] = n
			queue = append(queue, m)
		}
	}
	return nil
}
