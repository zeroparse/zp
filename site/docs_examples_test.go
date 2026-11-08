package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The examples of the guides, the tutorial, the cookbook and the README are run by
// TestDocumentationExamples, so that a grammar or an output cannot fall out of step with the
// code. The conventions of the pages that the test relies on are in
// docs/documentation-style.md ("Examples"):
//
//   - A zp block whose first line is `// name.zp` is a file to save under that name, and one whose
//     first line is `// name.zp (changed rules)` replaces the rules of the same names in that
//     file. The files of a page live in one directory, in the order of the page.
//   - A zp block that contains `@start` is a complete grammar and must compile.
//   - A bash block of lines that start with `zp`, followed by a text block, is a command and
//     its output (standard output and standard error together, or the standard output alone). The command is run in the
//     directory of the page; a path that the page does not create but the repository has is
//     made available there.
//
// A marker on the line before a block changes what the test does with it:
//
//	<!-- example: error -->           the grammar is rejected on purpose; it must not compile
//	<!-- example: file <name> -->     the block is a file to save under that name (any language)
//	<!-- example: skip: <reason> -->  the block is not run, for the reason given
//	<!-- example: use <page.md> -->   anywhere on a page: the files that page saves are available here
//
// In a shown output, a line of only "..." stands for any number of lines.
//
// A file is also saved by a one-line `cat <name>` block that is followed by a text block (the
// text is the content of the file), and by a bash block that is a `cat > <name> <<'EOF'` here
// document.
var (
	exampleMarkerRe  = regexp.MustCompile(`^<!--\s*example:\s*(error|file\s+[A-Za-z0-9_][A-Za-z0-9_./-]*|skip:\s*\S.*?)\s*-->$`)
	exampleFileRe    = regexp.MustCompile(`^//\s*([A-Za-z0-9_][A-Za-z0-9_./-]*\.zp)(\s+\(changed rules\))?\s*$`)
	exampleUseRe     = regexp.MustCompile(`^<!--\s*example:\s*use\s+(\S+\.md)\s*-->$`)
	exampleCatRe     = regexp.MustCompile(`^cat\s+([A-Za-z0-9_][A-Za-z0-9_./-]*)$`)
	exampleHereRe    = regexp.MustCompile(`^cat\s*>\s*([A-Za-z0-9_][A-Za-z0-9_./-]*)\s*<<\s*'?(\w+)'?$`)
	exampleCommandRe = regexp.MustCompile(`^(printf '[^']*' \| )?zp [a-z]`)
	exampleDeclRe    = regexp.MustCompile(`^(rule|type|var|const)\s+([A-Za-z_][A-Za-z0-9_]*)`)
)

// exampleBlock is one fenced code block of a page. Marker is "error", "file name" or
// "skip: reason", or empty.
type exampleBlock struct {
	Lang   string
	Body   []string
	Line   int // the line of the opening fence, from 1
	Marker string
}

// exampleBlocks returns the fenced code blocks of a page in order, with the marker that stands
// on the line before each (blank lines between a marker and its block are allowed).
func exampleBlocks(lines []string) []exampleBlock {
	var blocks []exampleBlock
	marker := ""
	for i := 0; i < len(lines); i++ {
		if m := fenceLineRe.FindStringSubmatch(lines[i]); m != nil {
			indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
			j := i + 1
			for j < len(lines) && !fenceCloseRe.MatchString(lines[j]) {
				j++
			}
			var body []string
			for _, l := range lines[i+1 : min(j, len(lines))] {
				body = append(body, strings.TrimPrefix(l, indent))
			}
			blocks = append(blocks, exampleBlock{Lang: strings.TrimSpace(m[1]), Body: body, Line: i + 1, Marker: marker})
			marker = ""
			i = j
			continue
		}
		switch s := strings.TrimSpace(lines[i]); {
		case s == "":
		case exampleMarkerRe.MatchString(s):
			marker = exampleMarkerRe.FindStringSubmatch(s)[1]
		default:
			marker = ""
		}
	}
	return blocks
}

// exampleDecl is a top-level declaration of a grammar file, with the comments and attributes
// before it.
type exampleDecl struct {
	Key  string // "rule main", or "" for text that belongs to no declaration
	Text string
}

// splitDecls splits grammar source into declarations. A declaration starts at the first line of
// its leading comments and attributes; the keys of `rule`, `type`, `var` and `const` are the
// keyword and the name.
func splitDecls(src string) []exampleDecl {
	var decls []exampleDecl
	var cur []string
	key := ""
	flush := func() {
		if len(cur) > 0 {
			decls = append(decls, exampleDecl{Key: key, Text: strings.Join(cur, "\n")})
		}
		cur, key = nil, ""
	}
	lead := true // the lines collected so far are comments and attributes, not yet a declaration
	for _, l := range strings.Split(strings.TrimRight(src, "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "//") || strings.HasPrefix(l, "@"):
			if !lead {
				flush()
				lead = true
			}
			cur = append(cur, l)
		case exampleDeclRe.MatchString(l):
			if !lead {
				flush()
			}
			m := exampleDeclRe.FindStringSubmatch(l)
			key = m[1] + " " + m[2]
			lead = false
			cur = append(cur, l)
		default:
			if strings.TrimSpace(l) == "" && lead && len(cur) == 0 {
				continue
			}
			cur = append(cur, l)
		}
	}
	flush()
	return decls
}

// replaceRules returns base with the declarations of changed put in place of the declarations
// of the same keys; declarations of changed that base lacks are appended.
func replaceRules(base, changed string) string {
	byKey := map[string]string{}
	var order []string
	for _, d := range splitDecls(changed) {
		if d.Key != "" {
			byKey[d.Key] = d.Text
			order = append(order, d.Key)
		}
	}
	var out []string
	used := map[string]bool{}
	for _, d := range splitDecls(base) {
		if t, ok := byKey[d.Key]; ok && d.Key != "" {
			out = append(out, t)
			used[d.Key] = true
			continue
		}
		out = append(out, d.Text)
	}
	for _, k := range order {
		if !used[k] {
			out = append(out, byKey[k])
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// zpBinary builds cmd/zp once, for all pages.
var zpBinary = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "zp-doc-examples-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "zp")
	cmd := exec.Command("go", "build", "-o", bin, "zeroparse.dev/zp/cmd/zp")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build ./cmd/zp: %v\n%s", err, out)
	}
	return bin, nil
})

// exampleCommandPaths makes the files that a command names available in dir: a path that the
// page did not create, but that exists in the repository, is linked there.
func exampleCommandPaths(dir, command string) error {
	for _, f := range strings.Fields(command) {
		f = strings.Trim(f, `'"`)
		if f == "" || strings.HasPrefix(f, "-") || !strings.Contains(f, "/") && filepath.Ext(f) == "" {
			continue
		}
		if filepath.IsAbs(f) || strings.Contains(f, "..") {
			continue
		}
		if _, err := os.Lstat(filepath.Join(dir, f)); err == nil {
			continue
		}
		src, err := filepath.Abs(filepath.Join("..", f))
		if err != nil {
			return err
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}
	return nil
}

// runExampleCommand runs a command line in dir with the built zp first on the path and returns
// its standard output and standard error together, in the order written, or with withStderr
// false its standard output alone.
func runExampleCommand(bin, dir, command string, withStderr bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+dir)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if withStderr {
		cmd.Stderr = &buf
	}
	err := cmd.Run()
	if ctx.Err() != nil {
		return buf.String(), ctx.Err()
	}
	// A nonzero exit status is part of what a page shows (a syntax error exits 1), so it is not an error.
	if _, ok := err.(*exec.ExitError); ok {
		err = nil
	}
	return buf.String(), err
}

// outputMatches reports whether got is the output that want shows. A line of want that holds
// only "..." stands for any number of lines (including none), as pages use it to cut a long output.
func outputMatches(got, want string) bool {
	if got == want {
		return true
	}
	wl := strings.Split(want, "\n")
	var segs [][]string
	cur := []string{}
	elided := false
	for _, l := range wl {
		if strings.TrimSpace(l) == "..." {
			segs = append(segs, cur)
			cur = []string{}
			elided = true
			continue
		}
		cur = append(cur, l)
	}
	if !elided {
		return false
	}
	segs = append(segs, cur)
	gl := strings.Split(got, "\n")
	first, last := segs[0], segs[len(segs)-1]
	if len(gl) < len(first)+len(last) || !slices.Equal(gl[:len(first)], first) || !slices.Equal(gl[len(gl)-len(last):], last) {
		return false
	}
	rest := gl[len(first) : len(gl)-len(last)]
	for _, seg := range segs[1 : len(segs)-1] {
		at := -1
		for i := 0; i+len(seg) <= len(rest); i++ {
			if slices.Equal(rest[i:i+len(seg)], seg) {
				at = i
				break
			}
		}
		if at < 0 {
			return false
		}
		rest = rest[at+len(seg):]
	}
	return true
}

// trimOutput removes the trailing spaces of every line and the blank lines around the text.
func trimOutput(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// deferredPages are pages whose examples are not run yet, with the reason. A page leaves this
// list when its examples carry the markers that they need.
var deferredPages = map[string]string{}

// exampleStats counts what TestDocumentationExamples did, for the log.
type exampleStats struct{ Compiled, Rejected, Commands, Skipped int }

// isCommandBlock reports whether every line of a bash block is a zp command.
func isCommandBlock(body []string) bool {
	for _, l := range body {
		if !exampleCommandRe.MatchString(l) {
			return false
		}
	}
	return len(body) > 0
}

// fileOfBlock returns the file that block i of a page saves, if it saves one. files holds the
// files saved so far (the base of a "changed rules" block).
func fileOfBlock(blocks []exampleBlock, i int, files map[string]string) (name, text string, ok bool) {
	b := blocks[i]
	if strings.HasPrefix(b.Marker, "skip:") {
		return "", "", false
	}
	text = strings.Join(b.Body, "\n") + "\n"
	if n, ok := strings.CutPrefix(b.Marker, "file "); ok {
		return n, text, true
	}
	switch {
	case b.Lang == "zp" && len(b.Body) > 0:
		m := exampleFileRe.FindStringSubmatch(b.Body[0])
		if m == nil {
			return "", "", false
		}
		if m[2] != "" {
			text = replaceRules(files[m[1]], strings.Join(b.Body[1:], "\n"))
		}
		return m[1], text, true
	case b.Lang == "bash" && len(b.Body) == 1:
		if m := exampleCatRe.FindStringSubmatch(b.Body[0]); m != nil && i+1 < len(blocks) && blocks[i+1].Lang == "text" {
			if _, have := files[m[1]]; !have {
				return m[1], strings.Join(blocks[i+1].Body, "\n") + "\n", true
			}
		}
	case b.Lang == "bash" && len(b.Body) >= 2:
		if m := exampleHereRe.FindStringSubmatch(b.Body[0]); m != nil && b.Body[len(b.Body)-1] == m[2] {
			return m[1], strings.Join(b.Body[1:len(b.Body)-1], "\n") + "\n", true
		}
	}
	return "", "", false
}

// pageFiles returns the files that the page saves, and the pages that it uses.
func pageFiles(rel string) (files map[string]string, uses []string, blocks []exampleBlock, err error) {
	data, err := os.ReadFile(filepath.Join("..", rel))
	if err != nil {
		return nil, nil, nil, err
	}
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if m := exampleUseRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			uses = append(uses, m[1])
		}
	}
	blocks = exampleBlocks(lines)
	files = map[string]string{}
	for i := range blocks {
		if name, text, ok := fileOfBlock(blocks, i, files); ok {
			files[name] = text
		}
	}
	return files, uses, blocks, nil
}

// saveExampleFile writes a file of a page into dir.
func saveExampleFile(t *testing.T, dir, name, text string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// checkPageExamples runs the examples of one page.
func checkPageExamples(t *testing.T, bin, rel string, st *exampleStats) {
	_, uses, blocks, err := pageFiles(rel)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, u := range uses {
		used, _, _, err := pageFiles(u)
		if err != nil {
			t.Fatal(err)
		}
		for name, text := range used {
			saveExampleFile(t, dir, name, text)
		}
	}
	files := map[string]string{}
	for i, b := range blocks {
		at := fmt.Sprintf("%s:%d", rel, b.Line)
		skip := strings.HasPrefix(b.Marker, "skip:")
		text := strings.Join(b.Body, "\n") + "\n"
		name := ""
		if n, t2, ok := fileOfBlock(blocks, i, files); ok {
			name, text = n, t2
			files[name] = text
			saveExampleFile(t, dir, name, text)
		}
		switch {
		case b.Lang == "zp":
			if skip || !strings.Contains(text, "@start") && b.Marker != "error" {
				if skip {
					st.Skipped++
				}
				continue
			}
			if name == "" {
				name = fmt.Sprintf("block-%d.zp", b.Line)
				saveExampleFile(t, dir, name, text)
			}
			out, err := runExampleCommand(bin, dir, "zp compile -g '"+name+"' -o /dev/null", true)
			if err != nil {
				t.Fatal(err)
			}
			failed := trimOutput(out) != ""
			switch {
			case b.Marker == "error" && !failed:
				t.Errorf("%s: the grammar is marked as rejected on purpose, but it compiles", at)
			case b.Marker == "error":
				st.Rejected++
			case failed:
				t.Errorf("%s: the complete grammar does not compile:\n%s", at, trimOutput(out))
			default:
				st.Compiled++
			}
		case b.Lang == "bash" && isCommandBlock(b.Body):
			if skip {
				st.Skipped++
				continue
			}
			command := strings.Join(b.Body, "\n")
			if err := exampleCommandPaths(dir, command); err != nil {
				t.Fatal(err)
			}
			got, err := runExampleCommand(bin, dir, command, true)
			if err != nil {
				t.Errorf("%s: %s: %v", at, command, err)
				continue
			}
			// A command without a shown output still runs, for the files that it writes.
			if i+1 >= len(blocks) || blocks[i+1].Lang != "text" {
				continue
			}
			st.Commands++
			// The output is one text block, or two when a page shows the standard output and then
			// the standard error separately.
			want := trimOutput(strings.Join(blocks[i+1].Body, "\n"))
			got = trimOutput(got)
			if !outputMatches(got, want) && i+2 < len(blocks) && blocks[i+2].Lang == "text" &&
				outputMatches(got, want+"\n"+trimOutput(strings.Join(blocks[i+2].Body, "\n"))) {
				want = got
			}
			if !outputMatches(got, want) {
				// A page may show the standard output alone.
				if out, err := runExampleCommand(bin, dir, command, false); err == nil && outputMatches(trimOutput(out), want) {
					continue
				}
				t.Errorf("%s: %s\ngot:\n%s\nwant:\n%s", at, command, got, want)
			}
		}
	}
}

// exampleDocs returns the pages whose examples are run, relative to the repository root.
func exampleDocs(t *testing.T) []string {
	t.Helper()
	pages := []string{"README.md"}
	for _, dir := range []string{"docs/tutorial", "docs/guide", "docs/cookbook"} {
		ms, err := filepath.Glob(filepath.Join("..", dir, "*.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range ms {
			if filepath.Base(m) != "TEMPLATE.md" {
				rel, _ := filepath.Rel("..", m)
				pages = append(pages, filepath.ToSlash(rel))
			}
		}
	}
	slices.Sort(pages)
	return pages
}

// TestDocumentationExamples compiles the complete grammars of the pages and runs their
// single-command examples against the output that the pages show.
func TestDocumentationExamples(t *testing.T) {
	bin, err := zpBinary()
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var total exampleStats
	t.Cleanup(func() {
		t.Logf("compiled %d grammars, %d rejected on purpose, ran %d commands, skipped %d marked blocks",
			total.Compiled, total.Rejected, total.Commands, total.Skipped)
	})
	for _, rel := range exampleDocs(t) {
		t.Run(rel, func(t *testing.T) {
			if reason, ok := deferredPages[rel]; ok {
				t.Skip(reason)
			}
			t.Parallel()
			var st exampleStats
			checkPageExamples(t, bin, rel, &st)
			mu.Lock()
			total.Compiled += st.Compiled
			total.Rejected += st.Rejected
			total.Commands += st.Commands
			total.Skipped += st.Skipped
			mu.Unlock()
		})
	}
}

func TestExampleBlocks(t *testing.T) {
	page := strings.Split(strings.Join([]string{
		"text",
		"<!-- example: error -->",
		"",
		"```zp",
		"// a.zp",
		"```",
		"- item",
		"",
		"  ```bash",
		"  zp fmt a.zp",
		"  ```",
		"<!-- example: skip: prints times -->",
		"```bash",
		"zp profile",
		"```",
		"```text",
		"x",
		"```",
	}, "\n"), "\n")
	got := exampleBlocks(page)
	want := []exampleBlock{
		{"zp", []string{"// a.zp"}, 4, "error"},
		{"bash", []string{"zp fmt a.zp"}, 9, ""},
		{"bash", []string{"zp profile"}, 13, "skip: prints times"},
		{"text", []string{"x"}, 16, ""},
	}
	if len(got) != len(want) {
		t.Fatalf("exampleBlocks found %d blocks, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Lang != want[i].Lang || got[i].Line != want[i].Line || got[i].Marker != want[i].Marker || !slices.Equal(got[i].Body, want[i].Body) {
			t.Errorf("block %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestReplaceRules(t *testing.T) {
	base := "type T = node[string]\n\n@start\nrule main T = a b\n\n// helper\nrule a = \"a\"\nrule b = (\n    \"b\"\n)\n"
	changed := "// note\nrule a = \"x\"\n\nrule c = \"c\"\n"
	got := replaceRules(base, changed)
	want := "type T = node[string]\n\n@start\nrule main T = a b\n\n// note\nrule a = \"x\"\n\nrule b = (\n    \"b\"\n)\nrule c = \"c\"\n"
	if got != want {
		t.Errorf("replaceRules:\n%q\nwant\n%q", got, want)
	}
	if n := len(splitDecls(base)); n != 4 {
		t.Errorf("splitDecls found %d declarations, want 4: %+v", n, splitDecls(base))
	}
}

func TestOutputMatches(t *testing.T) {
	for _, c := range []struct {
		got, want string
		ok        bool
	}{
		{"a\nb", "a\nb", true},
		{"a\nb", "a\nc", false},
		{"a\nb\nc\nd", "a\n...\nd", true},
		{"a\nd", "a\n...\nd", true},
		{"a\nb\nc\nd", "a\n  ...\nc\n...\nd", true},
		{"a\nb\nc\nd", "a\n...\ne\n...\nd", false},
		{"a\nb\nc\nd", "...\nd", true},
		{"a\nb\nc\nd", "a\n...", true},
		{"x\nb\nc\nd", "a\n...", false},
		{"a\nb\nc\nd", "a\n...\nc", false},
	} {
		if got := outputMatches(c.got, c.want); got != c.ok {
			t.Errorf("outputMatches(%q, %q) = %v, want %v", c.got, c.want, got, c.ok)
		}
	}
}
