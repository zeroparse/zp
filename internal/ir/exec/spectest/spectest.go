// Package spectest reads the tables of the Exec IR specification (spec/exec-ir) for the tests that
// hold it current with the code: the compositions and operations of internal/ir/exec/core, and the
// specializations that each target prints.
package spectest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Table is a Markdown table of a chapter: its header cells and its rows of cells. A cell that is
// one code span is its content.
type Table struct {
	File    string
	Section string // the heading of the section that holds the table
	Head    []string
	Rows    [][]string
}

// Read returns the tables of the chapters of the specification in dir, by chapter file name.
func Read(t testing.TB, dir string) map[string][]Table {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no chapters in %s: %v", dir, err)
	}
	out := map[string][]Table{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = parseTables(filepath.Base(f), string(src))
	}
	return out
}

// parseTables returns the tables of the Markdown text src, outside fenced blocks.
func parseTables(file, src string) []Table {
	var out []Table
	var cur *Table
	fenced := false
	section := ""
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "## ") {
			section = strings.TrimPrefix(line, "## ")
		}
		if fenced || !strings.HasPrefix(line, "|") {
			cur = nil
			continue
		}
		cells := splitRow(line)
		switch {
		case cur == nil:
			out = append(out, Table{File: file, Section: section, Head: cells})
			cur = &out[len(out)-1]
		case strings.HasPrefix(line, "|:") || strings.HasPrefix(line, "|-"):
		default:
			cur.Rows = append(cur.Rows, cells)
		}
	}
	return out
}

// splitRow returns the cells of a table row, with escaped pipes unescaped and a cell that is one code
// span replaced by its content.
func splitRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	var cells []string
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			b.WriteByte('|')
			i++
		case line[i] == '|':
			cells = append(cells, b.String())
			b.Reset()
		default:
			b.WriteByte(line[i])
		}
	}
	cells = append(cells, b.String())
	for i, c := range cells {
		c = strings.TrimSpace(c)
		if len(c) > 1 && c[0] == '`' && c[len(c)-1] == '`' && !strings.Contains(c[1:len(c)-1], "`") {
			c = c[1 : len(c)-1]
		}
		cells[i] = c
	}
	return cells
}

// Column returns the rows of the tables whose first header cell is head, in every chapter.
func Column(spec map[string][]Table, head string) [][]string {
	var out [][]string
	for _, file := range slices.Sorted(func(yield func(string) bool) {
		for f := range spec {
			if !yield(f) {
				return
			}
		}
	}) {
		for _, t := range spec[file] {
			if len(t.Head) > 0 && t.Head[0] == head {
				out = append(out, t.Rows...)
			}
		}
	}
	return out
}

// Compare reports the differences between what the code declares, want, and what the specification
// lists, got, both keyed by name: a declaration without an entry, an entry without a declaration, an
// entry listed twice and an entry that differs.
func Compare(kind string, want map[string]string, got [][2]string) []string {
	var errs []string
	seen := map[string]bool{}
	for _, g := range got {
		name, entry := g[0], g[1]
		if seen[name] {
			errs = append(errs, fmt.Sprintf("%s %s: listed twice in the specification", kind, name))
		}
		seen[name] = true
		w, ok := want[name]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("%s %s: in the specification but not in the module", kind, name))
		case w != entry:
			errs = append(errs, fmt.Sprintf("%s %s: the specification has\n\t%s\nthe module has\n\t%s", kind, name, entry, w))
		}
	}
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for n := range want {
			if !yield(n) {
				return
			}
		}
	}) {
		if !seen[name] {
			errs = append(errs, fmt.Sprintf("%s %s: in the module but not in the specification:\n\t%s", kind, name, want[name]))
		}
	}
	return errs
}

// specializationsHead is the first header cell of the table of specializations, whose other cells
// name the runtimes.
const specializationsHead = "Composition"

// Runtimes returns the runtime columns of the table of specializations in compositions.md.
func Runtimes(spec map[string][]Table) []string {
	for _, t := range spec["compositions.md"] {
		if len(t.Head) > 0 && t.Head[0] == specializationsHead {
			return t.Head[1:]
		}
	}
	return nil
}

// SpecializationErrors returns the differences between the specializations want and the table of
// specializations in compositions.md, for the runtimes of want: one row per composition, one column
// per runtime, each cell the feature sets in the catalog form ("[document, budget]"), separated by
// spaces. Each target checks the columns of the runtimes it prints.
func SpecializationErrors(want map[string]map[string][]string, spec map[string][]Table) []string {
	var errs []string
	for _, t := range spec["compositions.md"] {
		if len(t.Head) == 0 || t.Head[0] != specializationsHead {
			continue
		}
		got := map[string]map[string][]string{}
		for _, runtime := range t.Head[1:] {
			got[runtime] = map[string][]string{}
		}
		for runtime := range want {
			if got[runtime] == nil {
				errs = append(errs, "specializations: no runtime "+runtime)
			}
		}
		for _, r := range t.Rows {
			for i, cell := range r[1:] {
				if cell == "" || i+1 >= len(t.Head) {
					continue
				}
				sets := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(cell, "`", ""), ", ", ","))
				for j, s := range sets {
					sets[j] = strings.ReplaceAll(s, ",", ", ")
				}
				got[t.Head[i+1]][r[0]] = sets
			}
		}
		for _, runtime := range slices.Sorted(func(yield func(string) bool) {
			for r := range want {
				if !yield(r) {
					return
				}
			}
		}) {
			var g [][2]string
			for fn, sets := range got[runtime] {
				g = append(g, [2]string{fn, strings.Join(sets, " ")})
			}
			slices.SortFunc(g, func(a, b [2]string) int { return strings.Compare(a[0], b[0]) })
			w := map[string]string{}
			for fn, sets := range want[runtime] {
				w[fn] = strings.Join(sets, " ")
			}
			errs = append(errs, Compare(runtime+": specialization", w, g)...)
		}
		return errs
	}
	return []string{"compositions.md: no table of specializations"}
}

// AddSpecialization records in out that runtime prints the composition fn specialized on the
// features on, in the catalog form, with the features in the order features declares them.
func AddSpecialization(out map[string]map[string][]string, features []string, runtime, fn string, on []string) {
	on = slices.Clone(on)
	slices.SortFunc(on, func(a, b string) int { return slices.Index(features, a) - slices.Index(features, b) })
	set := "[" + strings.Join(on, ", ") + "]"
	if out[runtime] == nil {
		out[runtime] = map[string][]string{}
	}
	if !slices.Contains(out[runtime][fn], set) {
		out[runtime][fn] = append(out[runtime][fn], set)
	}
}
