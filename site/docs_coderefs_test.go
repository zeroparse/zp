package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The specification is the source and the code depends on it, so pages that readers
// rely on never refer to code, and internal documents refer to it only at the level of
// packages. TestDocumentCodeReferences enforces the rule of the "Code References"
// section of docs/documentation-style.md. Violations that exist today are listed in
// testdata/coderefs_allowlist.tsv; the list only shrinks.

// Categories of a reference to code.
const (
	refCodeLink  = "codelink"  // a link to code
	refGoFile    = "gofile"    // the name or path of a Go file of the repository
	refInternal  = "internal"  // a path under internal/
	refLine      = "lineref"   // a line number of a file or "line N of" a file
	refUnexport  = "unexport"  // an identifier that only the code declares
	refQualified = "qualified" // an exported name qualified by an internal package
)

// codeRef is one reference to code found in a document.
type codeRef struct {
	file, kind, token string
	line              int
}

func (r codeRef) key() string { return r.file + "\t" + r.kind + "\t" + r.token }

// repoCode describes the code of the repository, which a document must not refer to.
type repoCode struct {
	root        string
	goFiles     map[string]bool // repository paths of the Go files
	goBases     map[string]bool // base names of the Go files
	goDirs      map[string]bool // directories that directly hold a Go file
	codeDirs    map[string]bool // goDirs and their ancestors
	declared    map[string]bool // every declared function, type, variable and constant name
	unexported  map[string]bool // declared identifiers that start in lower case and have an upper-case letter
	internalPkg map[string]bool // names of internal packages that no public package shares
}

var (
	declFuncRe  = regexp.MustCompile(`^func (?:\([^)]*\) )?([a-z_]\w*)\s*[\[(]`)
	declTypeRe  = regexp.MustCompile(`^\s*type ([a-z_]\w*)\b`)
	declVarRe   = regexp.MustCompile(`^\s*(?:var|const) ([a-z_]\w*)\b`)
	goFileRe    = regexp.MustCompile(`[\w./-]*\w\.go\b`)
	goLineRe    = regexp.MustCompile(`([\w./-]*\w\.go):\d+`)
	lineOfRe    = regexp.MustCompile("(?i)\\blines? \\d+(?:\\s*(?:-|to|and)\\s*\\d+)? (?:of|in) `?([\\w./-]+\\.go|internal/[\\w./-]+)`?")
	internalRe  = regexp.MustCompile(`\binternal/[\w./-]*[\w-]`)
	nameLineRe  = regexp.MustCompile("`([a-z]\\w*)` (\\d{2,})\\b")
	spanRe      = regexp.MustCompile("`([^`\\n]+)`")
	identPartRe = regexp.MustCompile(`[A-Za-z_]\w*`)
	qualifiedRe = regexp.MustCompile(`\b([a-z]\w*)\.[A-Z]\w*`)
	// publicNames are public names that coincide with identifiers of the code: a field of the
	// Playground messages and a table of generated parsers.
	publicNames = map[string]bool{"maxDepth": true, "recRules": true}
	// readerFileNames are files that readers create in their own projects.
	readerFileNames = map[string]bool{"main.go": true, "main_test.go": true, "go.mod": true, "go.sum": true, "tools.go": true, "generate.go": true,
		// the files that zp gen writes
		"api.go": true, "parser.go": true, "types.go": true}
)

func loadRepoCode(t *testing.T, root string) *repoCode {
	t.Helper()
	c := &repoCode{root: root, goFiles: map[string]bool{}, goBases: map[string]bool{}, goDirs: map[string]bool{},
		codeDirs: map[string]bool{}, declared: map[string]bool{}, unexported: map[string]bool{}, internalPkg: map[string]bool{}}
	public := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".scratch", "dist", "wt", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".go" {
			return nil
		}
		c.goFiles[rel] = true
		c.goBases[path.Base(rel)] = true
		for dir := path.Dir(rel); ; dir = path.Dir(dir) {
			if dir != "." {
				c.codeDirs[dir] = true
			}
			if dir == "." || dir == "/" {
				break
			}
		}
		c.goDirs[path.Dir(rel)] = true
		if under(rel, "internal/bench") {
			// generated parsers and inputs, which stand for the files of a reader's project
		} else if under(rel, "internal") {
			c.internalPkg[path.Base(path.Dir(rel))] = true
		} else {
			public[path.Base(path.Dir(rel))] = true
		}
		if strings.Contains(rel, "testdata/") {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			for _, r := range []*regexp.Regexp{declFuncRe, declTypeRe, declVarRe} {
				if m := r.FindStringSubmatch(sc.Text()); m != nil {
					c.declared[m[1]] = true
				}
				if m := r.FindStringSubmatch(sc.Text()); m != nil && strings.ContainsAny(m[1], "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
					c.unexported[m[1]] = true
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range public {
		delete(c.internalPkg, name)
	}
	delete(c.internalPkg, "_runtime")
	// "value" and "types" are the usual names of variables and generated packages.
	delete(c.internalPkg, "value")
	delete(c.internalPkg, "types")
	return c
}

// repoGoFile reports whether name, a path or a base name written in a document, names a
// Go file of the repository. A base name that readers create themselves does not.
func (c *repoCode) repoGoFile(name string, readerFiles map[string]bool) bool {
	for strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
		name = strings.TrimPrefix(strings.TrimPrefix(name, "./"), "../")
	}
	if strings.Contains(name, "/") {
		if c.goFiles[name] {
			return true
		}
		for f := range c.goFiles {
			if !readerFileNames[path.Base(name)] && strings.HasSuffix(f, "/"+name) {
				return true
			}
		}
		return false
	}
	return c.goBases[name] && !readerFileNames[name] && !readerFiles[name]
}

// exists reports whether the repository holds a Go file or a directory of Go files at rel.
func (c *repoCode) exists(rel string) bool { return c.goFiles[rel] || c.codeDirs[rel] }

// isCodeTarget reports whether the repository path rp is code: a source file or a
// directory that holds Go files. The programs of examples/ are written for readers.
func (c *repoCode) isCodeTarget(rp string) (code, file bool) {
	if under(rp, "examples") {
		return false, false // programs written for readers
	}
	switch path.Ext(rp) {
	case ".go", ".ts", ".js", ".mjs", ".sh":
		return true, true
	}
	return c.codeDirs[rp], false
}

// fenceMask separates the lines of a document into prose and the lines of fenced
// blocks. fenced[i] is the language of the block that holds line i, or "-" for none.
func fenceMask(lines []string) (fenced []string) {
	fenced = make([]string, len(lines))
	for i := range fenced {
		fenced[i] = "-"
	}
	var fence, lang string
	for i, text := range lines {
		if fence != "" {
			trimmed := strings.TrimSpace(text)
			if strings.TrimLeft(trimmed, fence[:1]) == "" && len(trimmed) >= len(fence) {
				fence = ""
				continue
			}
			fenced[i] = lang
			continue
		}
		if m := fenceOpenRe.FindStringSubmatch(text); m != nil {
			fence = m[1]
			lang = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(text), fence[:1]))
			if lang == "" {
				lang = "text"
			}
			fenced[i] = "fence"
		}
	}
	return fenced
}

// readerFileNamesOf returns the Go files that a document has the reader create: a
// block that starts with "// name.go", or "cat > name.go".
func readerFileNamesOf(lines []string, fenced []string) map[string]bool {
	files := map[string]bool{}
	nameRe := regexp.MustCompile(`^(?://\s*|cat\s*>\s*)([\w./-]+\.go)\b`)
	for i, text := range lines {
		if fenced[i] == "-" || fenced[i] == "fence" {
			continue
		}
		if m := nameRe.FindStringSubmatch(strings.TrimSpace(text)); m != nil {
			files[path.Base(m[1])] = true
		}
	}
	return files
}

// codeRefsIn returns the references to code in one document. internal selects the
// rules of internal documents, which may refer to packages.
func (c *repoCode) codeRefsIn(rel string, data []byte, internal bool) []codeRef {
	lines := strings.Split(string(data), "\n")
	fenced := fenceMask(lines)
	readerFiles := readerFileNamesOf(lines, fenced)
	// Identifiers that the examples of the page define belong to the reader.
	fencedIdents := map[string]bool{}
	for i, text := range lines {
		if fenced[i] != "-" && fenced[i] != "fence" {
			for _, id := range identPartRe.FindAllString(text, -1) {
				fencedIdents[id] = true
			}
		}
	}
	masked := maskCode(lines)
	var refs []codeRef
	add := func(i int, kind, token string) {
		refs = append(refs, codeRef{rel, kind, token, i + 1})
	}
	// Links, in prose.
	doc := strings.Join(masked, "\n")
	for _, d := range documentDestinations(doc) {
		rp, ok := linkTarget(rel, d.dest)
		if !ok {
			continue
		}
		code, file := c.isCodeTarget(rp)
		// TestDocumentLinkDirections checks the links of specification, guide, cookbook and
		// tutorial pages.
		if referenceKind(rel) == "reader" && rel != "README.md" {
			continue
		}
		if code && (file || !internal) {
			add(strings.Count(doc[:d.off], "\n"), refCodeLink, rp)
		}
	}
	for i, text := range lines {
		lang := fenced[i]
		if lang == "fence" {
			continue
		}
		if lang != "-" && lang != "bash" && lang != "text" {
			continue // the files and programs that readers write
		}
		for _, m := range goLineRe.FindAllStringSubmatch(text, -1) {
			if c.repoGoFile(m[1], readerFiles) {
				add(i, refLine, m[0])
			}
		}
		for _, m := range lineOfRe.FindAllStringSubmatch(text, -1) {
			if strings.HasSuffix(m[1], ".go") && !c.repoGoFile(m[1], readerFiles) {
				continue
			}
			add(i, refLine, m[0])
		}
		for _, m := range nameLineRe.FindAllStringSubmatch(text, -1) {
			if c.declared[m[1]] {
				add(i, refLine, m[0])
			}
		}
		for _, m := range goFileRe.FindAllString(text, -1) {
			for strings.HasPrefix(m, "./") || strings.HasPrefix(m, "../") {
				m = strings.TrimPrefix(strings.TrimPrefix(m, "./"), "../")
			}
			if lang != "-" && !strings.Contains(m, "/") {
				continue // a file list of a reader's project
			}
			if c.repoGoFile(m, readerFiles) {
				add(i, refGoFile, m)
			}
		}
		for _, m := range internalRe.FindAllString(text, -1) {
			m = strings.TrimRight(m, "./-")
			if strings.HasSuffix(m, ".go") || !c.exists(m) {
				continue // reported as a Go file, or a path of generated code
			}
			if internal && c.goDirs[m] {
				continue // a package
			}
			add(i, refInternal, m)
		}
		if lang != "-" {
			continue
		}
		for _, m := range spanRe.FindAllStringSubmatch(text, -1) {
			if len(strings.Fields(m[1])) >= 3 {
				continue // a quoted message
			}
			seen := map[string]bool{}
			for _, id := range identPartRe.FindAllString(m[1], -1) {
				if c.unexported[id] && !seen[id] && !fencedIdents[id] && !publicNames[id] {
					seen[id] = true
					add(i, refUnexport, id)
				}
			}
			for _, q := range qualifiedRe.FindAllStringSubmatch(m[1], -1) {
				if c.internalPkg[q[1]] && !seen[q[0]] {
					seen[q[0]] = true
					add(i, refQualified, q[0])
				}
			}
		}
	}
	return refs
}

// referenceKind classifies a Markdown file for the code references rule: "reader" for
// the pages that readers and implementers rely on, "internal" for the documents that
// describe the implementation, and "" for the rest.
func referenceKind(rel string) string {
	switch {
	case under(rel, "spec"), under(rel, "docs/tutorial"), under(rel, "docs/guide"), under(rel, "docs/cookbook"), rel == "README.md":
		return "reader"
	case under(rel, "docs/design"), under(rel, "docs/optimizations"), rel == "docs/performance.md", rel == "docs/benchmarks.md", under(rel, "bench"):
		return "internal"
	}
	return ""
}

// allCodeRefs returns the references to code of every checked document.
func allCodeRefs(t *testing.T, repo string) []codeRef {
	t.Helper()
	c := loadRepoCode(t, repo)
	var refs []codeRef
	err := filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".scratch", "dist", "wt", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(repo, p)
		rel = filepath.ToSlash(rel)
		kind := referenceKind(rel)
		if filepath.Ext(p) != ".md" || kind == "" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		refs = append(refs, c.codeRefsIn(rel, data, kind == "internal")...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return refs
}

const coderefsAllowlist = "testdata/coderefs_allowlist.tsv"

// readAllowlist reads the allowlist: file, category, token and the number of references.
func readAllowlist(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(coderefsAllowlist)
	if err != nil {
		t.Fatal(err)
	}
	list := map[string]int{}
	for n, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			t.Fatalf("%s:%d: want file, category, token and count", coderefsAllowlist, n+1)
		}
		count, err := strconv.Atoi(f[3])
		if err != nil || count <= 0 {
			t.Fatalf("%s:%d: bad count %q", coderefsAllowlist, n+1, f[3])
		}
		list[strings.Join(f[:3], "\t")] = count
	}
	return list
}

// coderefProblems compares the references with the allowlist: a reference beyond the
// listed count is new, and a listed count that is not reached is a stale entry.
func coderefProblems(refs []codeRef, list map[string]int) []string {
	got := map[string][]codeRef{}
	for _, r := range refs {
		got[r.key()] = append(got[r.key()], r)
	}
	var problems []string
	for k, rs := range got {
		if len(rs) > list[k] {
			for _, r := range rs[list[k]:] {
				problems = append(problems, fmt.Sprintf("%s:%d: %s reference to code %q; see Code References in docs/documentation-style.md", r.file, r.line, r.kind, r.token))
			}
		}
	}
	for k, n := range list {
		if len(got[k]) < n {
			problems = append(problems, fmt.Sprintf("allowlist entry %q lists %d, but %d remain; lower or remove it in %s", strings.ReplaceAll(k, "\t", " "), n, len(got[k]), coderefsAllowlist))
		}
	}
	sort.Strings(problems)
	return problems
}

// TestDocumentCodeReferences enforces the Code References rule of the documentation
// style.
func TestDocumentCodeReferences(t *testing.T) {
	refs := allCodeRefs(t, "..")
	if dump := os.Getenv("ZP_CODEREFS_DUMP"); dump != "" {
		counts := map[string]int{}
		for _, r := range refs {
			counts[r.key()]++
		}
		var keys []string
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString("# Violations of the Code References rule that are still to be fixed. The list only shrinks.\n# file, category, token, count\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "%s\t%d\n", k, counts[k])
		}
		if err := os.WriteFile(dump, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range coderefProblems(refs, readAllowlist(t)) {
		t.Error(p)
	}
}

func TestCodeReferencesScanner(t *testing.T) {
	c := &repoCode{
		goFiles:     map[string]bool{"internal/engine/core/scan.go": true, "cmd/zp/main.go": true},
		goBases:     map[string]bool{"scan.go": true, "main.go": true},
		goDirs:      map[string]bool{"internal/engine/core": true},
		codeDirs:    map[string]bool{"internal": true, "internal/engine": true, "internal/engine/core": true, "internal/engine/core/sub": true, "cmd": true, "cmd/zp": true},
		declared:    map[string]bool{"scanSpan": true, "seq": true},
		unexported:  map[string]bool{"scanSpan": true},
		internalPkg: map[string]bool{"core": true},
	}
	doc := "See [scan](internal/engine/core/scan.go) and `scanSpan` and `core.ScanHotSpan`.\n" +
		"The file scan.go:42 and line 7 of internal/engine/core/scan.go, in internal/engine/core and internal/engine/core/sub.\n" +
		"A rule (`seq` 303) and a count `seq` 3.\n" +
		"Run it as `go run main.go` and read main.go:3: error.\n" +
		"```text\nmain.go:12:3: syntax error\n```\n" +
		"```go\n// main.go\npackage main\n```\n" +
		"The `zp.Parse` function takes `text`.\n"
	var got []string
	for _, r := range c.codeRefsIn("README.md", []byte(doc), false) {
		got = append(got, r.kind+" "+r.token)
	}
	want := []string{"codelink internal/engine/core/scan.go", "unexport scanSpan", "qualified core.ScanHotSpan",
		"lineref scan.go:42", "lineref line 7 of internal/engine/core/scan.go", "gofile scan.go", "lineref `seq` 303", "gofile internal/engine/core/scan.go", "gofile internal/engine/core/scan.go",
		"internal internal/engine/core", "internal internal/engine/core/sub"}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("reader page:\n got %q\nwant %q", got, want)
	}
	got = nil
	for _, r := range c.codeRefsIn("docs/design/x.md", []byte(doc), true) {
		got = append(got, r.kind+" "+r.token)
	}
	for _, g := range got {
		if g == "internal internal/engine/core" {
			t.Error("a package path is a violation in an internal document")
		}
	}
	if !strings.Contains(strings.Join(got, "|"), "internal internal/engine/core/sub") {
		t.Error("a path below a package is not a violation in an internal document")
	}
}
