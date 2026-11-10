package core

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// The tests of this file hold the Exec IR specification (spec/exec-ir) current with the module that
// Build returns: every declaration of the module has an entry in the specification with the same
// signature, state, features and cost bound, and every entry declares something the module has.
// They live beside the code, so that a change of Exec IR runs them; the site's tests run only when
// documentation changes. They check the structure of each entry, not the prose of contracts, which
// the specification owns.

// specDir is the directory of the Exec IR specification, relative to this package.
const specDir = "../../../../spec/exec-ir"

// specTable is a Markdown table of a chapter: its header cells and its rows of cells. A cell that is
// one code span is its content.
type specTable struct {
	file string
	head []string
	rows [][]string
}

// readSpec returns the tables of the chapters of the specification.
func readSpec(t *testing.T) map[string][]specTable {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(specDir, "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no chapters in %s: %v", specDir, err)
	}
	out := map[string][]specTable{}
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
func parseTables(file, src string) []specTable {
	var out []specTable
	var cur *specTable
	fenced := false
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if fenced || !strings.HasPrefix(line, "|") {
			cur = nil
			continue
		}
		cells := splitRow(line)
		switch {
		case cur == nil:
			out = append(out, specTable{file: file, head: cells})
			cur = &out[len(out)-1]
		case strings.HasPrefix(line, "|:") || strings.HasPrefix(line, "|-"):
		default:
			cur.rows = append(cur.rows, cells)
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

// column returns the rows of the tables whose first header cell is head, in every chapter.
func column(spec map[string][]specTable, head string) [][]string {
	var out [][]string
	for _, file := range slices.Sorted(func(yield func(string) bool) {
		for f := range spec {
			if !yield(f) {
				return
			}
		}
	}) {
		for _, t := range spec[file] {
			if len(t.head) > 0 && t.head[0] == head {
				out = append(out, t.rows...)
			}
		}
	}
	return out
}

// compare reports the differences between what the module declares, want, and what the specification
// lists, got, both keyed by name: a declaration without an entry, an entry without a declaration, an
// entry listed twice and an entry that differs.
func compare(kind string, want map[string]string, got [][2]string) []string {
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

func params(ps []exec.Param) string {
	s := make([]string, len(ps))
	for i, p := range ps {
		s[i] = p.Name + " " + p.Type.String()
	}
	return strings.Join(s, ", ")
}

func results(ts []exec.Type) string {
	switch len(ts) {
	case 0:
		return ""
	case 1:
		return " " + ts[0].String()
	}
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = t.String()
	}
	return " (" + strings.Join(s, ", ") + ")"
}

func features(fs []string) string {
	if len(fs) == 0 {
		return ""
	}
	return " [" + strings.Join(fs, ", ") + "]"
}

// opEntry returns the catalog entry of the operation o: its declaration, its state and its cost
// bound, as the columns of its row.
func opEntry(o *exec.Op) string {
	kind := "op"
	switch {
	case o.Transitional:
		kind = "transitional op"
	case o.Host:
		kind = "host op"
	}
	decl := fmt.Sprintf("%s %s(%s)%s%s", kind, o.Name, params(o.Params), results(o.Results), features(o.Features))
	if o.NoReturn {
		decl += " noreturn"
	}
	var state []string
	if len(o.Reads) > 0 {
		state = append(state, "reads "+strings.Join(o.Reads, ", "))
	}
	if len(o.Writes) > 0 {
		state = append(state, "writes "+strings.Join(o.Writes, ", "))
	}
	if len(state) == 0 {
		state = []string{"none"}
	}
	return decl + " | " + strings.Join(state, "; ") + " | " + o.Cost.String()
}

// declName returns the name in a catalog declaration: the word before its parameters.
func declName(decl string) string {
	head, _, _ := strings.Cut(decl, "(")
	return head[strings.LastIndex(head, " ")+1:]
}

// moduleErrors returns the differences between the module m and the specification spec.
func moduleErrors(m *exec.Module, spec map[string][]specTable) []string {
	var errs []string

	opWant := map[string]string{}
	for _, o := range m.Ops {
		opWant[o.Name] = opEntry(o)
	}
	var opGot, fnGot [][2]string
	for _, r := range column(spec, "Declaration") {
		d := r[0]
		switch {
		case strings.HasPrefix(d, "op "), strings.HasPrefix(d, "host op "), strings.HasPrefix(d, "transitional op "):
			if len(r) < 3 {
				errs = append(errs, "operation row without state and cost: "+d)
				continue
			}
			opGot = append(opGot, [2]string{declName(d), d + " | " + r[1] + " | " + r[2]})
		case strings.HasPrefix(d, "func "):
			fnGot = append(fnGot, [2]string{declName(d), d})
		default:
			errs = append(errs, "declaration of no known kind: "+d)
		}
	}
	errs = append(errs, compare("operation", opWant, opGot)...)

	fnWant := map[string]string{}
	for _, f := range m.Funcs {
		fnWant[f.Name] = fmt.Sprintf("func %s(%s)%s%s", f.Name, params(f.Params), results(f.Results), features(f.Features))
	}
	errs = append(errs, compare("composition", fnWant, fnGot)...)

	simple := func(kind, head string, want map[string]string, cols int) {
		var got [][2]string
		for _, r := range column(spec, head) {
			entry := strings.Join(r[:min(cols, len(r))], " ")
			got = append(got, [2]string{r[0], entry})
		}
		errs = append(errs, compare(kind, want, got)...)
	}
	want := map[string]string{}
	for _, f := range m.Features {
		want[f] = f
	}
	simple("feature", "Feature", want, 1)
	want = map[string]string{}
	for _, c := range m.Components {
		want[c.Name] = c.Name
	}
	simple("component", "Component", want, 1)
	want = map[string]string{}
	for _, r := range m.Registers {
		want[r.Name] = r.Name + " " + r.Type.String()
	}
	simple("register", "Register", want, 2)
	want = map[string]string{}
	for _, h := range m.Handles {
		want[h.Name] = h.Name + " " + h.Component
	}
	simple("handle type", "Handle type", want, 2)
	want = map[string]string{}
	for _, o := range m.Ops {
		for _, t := range o.Cost.Terms {
			want[t.Quantity] = t.Quantity
		}
	}
	simple("quantity", "Quantity", want, 1)
	return errs
}

// irKinds returns the names of the types of package exec that implement the interface method
// method: the statements (stmt) or the expressions (expr).
func irKinds(t *testing.T, method string) map[string]string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "../exec.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != method || len(fn.Recv.List) != 1 {
			continue
		}
		if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
			if id, ok := star.X.(*ast.Ident); ok {
				out[id.Name] = id.Name
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s methods in ../exec.go", method)
	}
	return out
}

// TestSpecModule checks the declarations of the parser VM, the operations and the compositions
// against the specification.
func TestSpecModule(t *testing.T) {
	for _, e := range moduleErrors(Build(), readSpec(t)) {
		t.Error(e)
	}
}

// TestSpecModuleNegative is the permanent negative control of TestSpecModule: an operation and a
// composition that the specification does not list, a changed signature and a declaration that the
// module no longer has are each reported by name.
func TestSpecModuleNegative(t *testing.T) {
	spec := readSpec(t)
	m := Build()
	m.Ops = append(m.Ops, &exec.Op{Name: "planted.op", Component: "memo", Contract: "Planted."})
	m.Funcs = append(m.Funcs, &exec.Func{Name: "plantedFunc", Body: &exec.Block{}})
	m.Func("grow").Params = nil
	removed := m.Ops[0].Name
	m.Ops = m.Ops[1:]
	errs := strings.Join(moduleErrors(m, spec), "\n")
	for _, want := range []string{
		"operation planted.op: in the module but not in the specification",
		"composition plantedFunc: in the module but not in the specification",
		"composition grow: the specification has",
		"operation " + removed + ": in the specification but not in the module",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("no error %q in:\n%s", want, errs)
		}
	}
}

// TestSpecStatements checks the statement and expression kinds of Functions against package exec.
func TestSpecStatements(t *testing.T) {
	spec := readSpec(t)
	for _, k := range []struct{ kind, head, method string }{{"statement", "Statement", "stmt"}, {"expression", "Expression", "expr"}} {
		var got [][2]string
		for _, r := range column(map[string][]specTable{"functions.md": spec["functions.md"]}, k.head) {
			got = append(got, [2]string{r[0], r[0]})
		}
		for _, e := range compare(k.kind, irKinds(t, k.method), got) {
			t.Error(e)
		}
	}
}

// specializations returns what each runtime prints: for each runtime of the table of
// Specializations, and each composition, the feature sets of its specializations, in the catalog form
// ("[document, budget]"), with features in the order the module declares them.
func specializations(m *exec.Module) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	add := func(runtime, fn string, on []string) {
		on = slices.Clone(on)
		slices.SortFunc(on, func(a, b string) int { return slices.Index(m.Features, a) - slices.Index(m.Features, b) })
		set := "[" + strings.Join(on, ", ") + "]"
		if out[runtime] == nil {
			out[runtime] = map[string][]string{}
		}
		if !slices.Contains(out[runtime][fn], set) {
			out[runtime][fn] = append(out[runtime][fn], set)
		}
	}
	const nodes, typed, engine, ts = "Generated Go, nodes", "Generated Go, typed values", "The engine", "TypeScript"
	for _, list := range [][]goSpec{goSpecs, goStreamSpecs, goOperatorSpecs, goTermSpecs} {
		for _, s := range list {
			if s.typed {
				add(typed, s.fn, s.on)
			} else {
				add(nodes, s.fn, s.on)
			}
		}
	}
	for _, list := range [][]goSpec{engineSpecs, engineOperatorSpecs, engineStreamSpecs, engineDocumentSpecs} {
		for _, s := range list {
			add(engine, s.fn, s.on)
		}
	}
	for _, f := range termFuncs {
		add(engine, f.fn, nil)
	}
	for _, fn := range engineBackendFuncs {
		add(engine, fn, nil)
	}
	for _, s := range tsSpecs {
		add(ts, s.fn, s.on)
	}
	return out
}

// specializationErrors returns the differences between the specializations want and the table of
// Specializations in spec: one row per composition, one column per runtime, each cell the feature
// sets, separated by spaces.
func specializationErrors(want map[string]map[string][]string, spec map[string][]specTable) []string {
	var errs []string
	for _, t := range spec["compositions.md"] {
		if len(t.head) == 0 || t.head[0] != "Composition" {
			continue
		}
		got := map[string]map[string][]string{}
		for _, runtime := range t.head[1:] {
			if want[runtime] == nil {
				errs = append(errs, "specializations: no runtime "+runtime)
			}
			got[runtime] = map[string][]string{}
		}
		for _, r := range t.rows {
			for i, cell := range r[1:] {
				if cell == "" || i+1 >= len(t.head) {
					continue
				}
				sets := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(cell, "`", ""), ", ", ","))
				for j, s := range sets {
					sets[j] = strings.ReplaceAll(s, ",", ", ")
				}
				got[t.head[i+1]][r[0]] = sets
			}
		}
		for runtime, fns := range want {
			var g [][2]string
			for fn, sets := range got[runtime] {
				g = append(g, [2]string{fn, strings.Join(sets, " ")})
			}
			slices.SortFunc(g, func(a, b [2]string) int { return strings.Compare(a[0], b[0]) })
			w := map[string]string{}
			for fn, sets := range fns {
				w[fn] = strings.Join(sets, " ")
			}
			errs = append(errs, compare(runtime+": specialization", w, g)...)
		}
		return errs
	}
	return []string{"compositions.md: no table of specializations"}
}

// TestSpecSpecializations checks the table of Specializations against what each runtime prints.
func TestSpecSpecializations(t *testing.T) {
	for _, e := range specializationErrors(specializations(Build()), readSpec(t)) {
		t.Error(e)
	}
}

// TestSpecSpecializationsNegative is the permanent negative control of TestSpecSpecializations.
func TestSpecSpecializationsNegative(t *testing.T) {
	want := specializations(Build())
	want["TypeScript"]["plantedFunc"] = []string{"[budget]"}
	want["The engine"]["grow"] = []string{"[typed]"}
	errs := strings.Join(specializationErrors(want, readSpec(t)), "\n")
	for _, w := range []string{"TypeScript: specialization plantedFunc: in the module", "The engine: specialization grow: the specification has"} {
		if !strings.Contains(errs, w) {
			t.Errorf("no error %q in:\n%s", w, errs)
		}
	}
}

// TestSpecValues checks the tables of Values against the tables that every target shares.
func TestSpecValues(t *testing.T) {
	spec := readSpec(t)
	tables := map[string]specTable{}
	for _, tb := range spec["values.md"] {
		tables[strings.Join(tb.head, " ")] = tb
	}
	if n := len(tables["Code Kind Values"].rows); n != ops.KindOther+1 {
		t.Errorf("value kinds: %d rows, want %d", n, ops.KindOther+1)
	}
	groups := map[string]struct{ syms, names []string }{
		"Binary":  {ops.BinaryOps, ops.BinaryNames},
		"Unary":   {ops.UnaryOps, ops.UnaryNames},
		"Logical": {ops.LogicalOps, ops.LogicalNames},
	}
	seen := 0
	for _, r := range tables["Group Operators, by code from 0"].rows {
		g, ok := groups[r[0]]
		if !ok {
			t.Errorf("operators: unknown group %s", r[0])
			continue
		}
		seen++
		var want []string
		for i := range g.syms {
			want = append(want, "`"+g.syms[i]+"` "+strings.ToLower(g.names[i]))
		}
		if got := r[1]; got != strings.Join(want, ", ") {
			t.Errorf("operators %s: the specification has\n\t%s\nthe code has\n\t%s", r[0], got, strings.Join(want, ", "))
		}
	}
	if seen != len(groups) {
		t.Errorf("operators: %d groups, want %d", seen, len(groups))
	}
	members := tables["Code Kind Spelling Reads"]
	if len(members.rows) != len(ops.MemberKinds) {
		t.Errorf("member kinds: %d rows, want %d", len(members.rows), len(ops.MemberKinds))
	}
	for i, r := range members.rows {
		if i >= len(ops.MemberKinds) {
			break
		}
		k := ops.MemberKinds[i]
		if r[0] != fmt.Sprint(i) || r[1] != strings.ToLower(k.Name) || (k.Spelling != "" && r[2] != k.Spelling && !strings.Contains(r[2], "`"+k.Spelling+"`")) {
			t.Errorf("member kind %d: the specification has %q, the code has %+v", i, r, k)
		}
	}
	fails := tables["Code Failure Text Hint Meaning"].rows
	if len(fails) != len(ops.TermErrors) {
		t.Errorf("failures: %d rows, want %d", len(fails), len(ops.TermErrors))
	}
	for i, r := range fails {
		if i >= len(ops.TermErrors) {
			break
		}
		e := ops.TermErrors[i]
		if r[0] != fmt.Sprint(i) || r[1] != e.Name || r[2] != e.Text || r[3] != e.Hint {
			t.Errorf("failure %d: the specification has %q, the code has %+v", i, r[:4], e)
		}
	}
}
