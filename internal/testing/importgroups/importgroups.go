// Package importgroups checks and rewrites the grouping of Go import blocks.
//
// The rule: a parenthesised import block has at most one blank line, between the
// standard library group and the group of all other imports; each group is sorted
// by import path. It is shared by the architecture test that enforces the rule
// and by hack/fix-imports.sh, which rewrites a file to it.
package importgroups

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
)

// IsStd reports whether an import path belongs to the standard library: its first
// element has no dot. The pseudo-package "C" counts as standard.
func IsStd(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// entry is one import spec with the source lines that belong to it.
type entry struct {
	path       string
	name       string
	start, end int // 1-based inclusive line range, doc comment and trailing comment included
}

// block is one parenthesised import declaration.
type block struct {
	open, close int // lines of "(" and ")"
	entries     []entry
	groups      [][]entry // entries split at blank lines
}

func parseBlocks(fset *token.FileSet, f *ast.File) []block {
	var blocks []block
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT || !gd.Lparen.IsValid() {
			continue
		}
		b := block{open: fset.Position(gd.Lparen).Line, close: fset.Position(gd.Rparen).Line}
		for _, s := range gd.Specs {
			is := s.(*ast.ImportSpec)
			e := entry{start: fset.Position(is.Pos()).Line, end: fset.Position(is.End()).Line}
			e.path, _ = strconv.Unquote(is.Path.Value)
			if is.Name != nil {
				e.name = is.Name.Name
			}
			if is.Doc != nil {
				e.start = fset.Position(is.Doc.Pos()).Line
			}
			if is.Comment != nil {
				e.end = max(e.end, fset.Position(is.Comment.End()).Line)
			}
			if len(b.groups) == 0 || e.start-b.entries[len(b.entries)-1].end > 1 {
				b.groups = append(b.groups, nil)
			}
			b.entries = append(b.entries, e)
			b.groups[len(b.groups)-1] = append(b.groups[len(b.groups)-1], e)
		}
		blocks = append(blocks, b)
	}
	return blocks
}

func less(a, b entry) int {
	return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.name, b.name))
}

// Check returns one violation message per offending import block of a Go source file;
// each starts with "line N:". A file that does not parse yields one message.
func Check(name string, src []byte) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return []string{"does not parse: " + err.Error()}
	}
	var out []string
	for _, b := range parseBlocks(fset, f) {
		bad := func(format string, args ...any) {
			out = append(out, fmt.Sprintf("line %d: ", b.open)+fmt.Sprintf(format, args...))
		}
		if len(b.groups) > 2 {
			bad("%d import groups; at most two (standard library, then the rest)", len(b.groups))
		}
		for i, g := range b.groups {
			if !slices.IsSortedFunc(g, less) {
				bad("group %d is not sorted by import path", i+1)
			}
			for _, e := range g {
				switch {
				case len(b.groups) > 1 && i == 0 && !IsStd(e.path):
					bad("non-standard import %q in the standard library group", e.path)
				case len(b.groups) > 1 && i > 0 && IsStd(e.path):
					bad("standard library import %q outside the first group", e.path)
				case len(b.groups) == 1 && i == 0 && !IsStd(e.path) && slices.ContainsFunc(g, func(x entry) bool { return IsStd(x.path) }):
					bad("standard library and other imports share one group")
					goto nextGroup
				}
			}
		nextGroup:
		}
	}
	return out
}

// Fix rewrites the import blocks of a Go source file to the rule and returns the new
// source. Blocks whose lines are not all import specs with attached comments (for
// example a detached comment between imports) are left unchanged and listed in
// skipped as "line N".
func Fix(name string, src []byte) (out []byte, skipped []string, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(src), "\n")
	blocks := parseBlocks(fset, f)
	// Rewrite from the last block to the first so earlier line numbers stay valid.
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		if len(b.entries) == 0 {
			continue
		}
		covered := make(map[int]bool)
		for _, e := range b.entries {
			for l := e.start; l <= e.end; l++ {
				covered[l] = true
			}
		}
		ok := b.open < b.entries[0].start && b.entries[len(b.entries)-1].end < b.close
		for l := b.open + 1; l < b.close && ok; l++ {
			if !covered[l] && strings.TrimSpace(lines[l-1]) != "" {
				ok = false
			}
		}
		if !ok {
			skipped = append(skipped, fmt.Sprintf("line %d", b.open))
			continue
		}
		var std, rest []entry
		for _, e := range b.entries {
			if IsStd(e.path) {
				std = append(std, e)
			} else {
				rest = append(rest, e)
			}
		}
		slices.SortStableFunc(std, less)
		slices.SortStableFunc(rest, less)
		var repl []string
		emit := func(es []entry) {
			for _, e := range es {
				repl = append(repl, lines[e.start-1:e.end]...)
			}
		}
		emit(std)
		if len(std) > 0 && len(rest) > 0 {
			repl = append(repl, "")
		}
		emit(rest)
		lines = slices.Concat(lines[:b.open], repl, lines[b.close-1:])
	}
	return []byte(strings.Join(lines, "\n")), skipped, nil
}
