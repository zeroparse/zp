package core

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/spectest"
)

// The tests of this file hold the Exec IR specification (spec/exec-ir) current with the module that
// Build returns: every declaration of the module has an entry in the specification with the same
// signature, state, features and cost bound, and every entry declares something the module has.
// They live beside the code, so that a change of Exec IR runs them; the site's tests run only when
// documentation changes. They check the structure of each entry, not the prose of contracts, which
// the specification owns.

// specDir is the directory of the Exec IR specification, relative to this package.
const specDir = "../../../../spec/exec-ir"

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

// sectionErrors returns the operations that sit in a section of the catalog with operations of another
// component: a section holds the operations of one component, so that where an operation sits tells
// which component it belongs to.
func sectionErrors(m *exec.Module, spec map[string][]spectest.Table) []string {
	comp := map[string]string{}
	for _, o := range m.Ops {
		comp[o.Name] = o.Component
	}
	var errs []string
	for _, file := range slices.Sorted(func(yield func(string) bool) {
		for f := range spec {
			if !yield(f) {
				return
			}
		}
	}) {
		for _, tb := range spec[file] {
			if len(tb.Head) == 0 || tb.Head[0] != "Declaration" {
				continue
			}
			first := map[string]string{} // component to the first operation of the section that has it
			var order []string
			for _, r := range tb.Rows {
				if !strings.HasPrefix(r[0], "op ") && !strings.HasPrefix(r[0], "host op ") && !strings.HasPrefix(r[0], "transitional op ") {
					continue
				}
				name := declName(r[0])
				c, ok := comp[name]
				if !ok {
					continue // reported by moduleErrors
				}
				if _, seen := first[c]; !seen {
					first[c] = name
					order = append(order, c)
				}
			}
			for i, c := range order {
				if i > 0 {
					errs = append(errs, fmt.Sprintf("%s, section %q: the operation %s of the component %s sits with operations of the component %s", file, tb.Section, first[c], c, order[0]))
				}
			}
		}
	}
	return errs
}

// declName returns the name in a catalog declaration: the word before its parameters.
func declName(decl string) string {
	head, _, _ := strings.Cut(decl, "(")
	return head[strings.LastIndex(head, " ")+1:]
}

// moduleErrors returns the differences between the module m and the specification spec.
func moduleErrors(m *exec.Module, spec map[string][]spectest.Table) []string {
	var errs []string

	opWant := map[string]string{}
	for _, o := range m.Ops {
		opWant[o.Name] = opEntry(o)
	}
	var opGot, fnGot [][2]string
	for _, r := range spectest.Column(spec, "Declaration") {
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
	errs = append(errs, spectest.Compare("operation", opWant, opGot)...)

	fnWant := map[string]string{}
	for _, f := range m.Funcs {
		fnWant[f.Name] = fmt.Sprintf("func %s(%s)%s%s", f.Name, params(f.Params), results(f.Results), features(f.Features))
	}
	errs = append(errs, spectest.Compare("composition", fnWant, fnGot)...)

	simple := func(kind, head string, want map[string]string, cols int) {
		var got [][2]string
		for _, r := range spectest.Column(spec, head) {
			entry := strings.Join(r[:min(cols, len(r))], " ")
			got = append(got, [2]string{r[0], entry})
		}
		errs = append(errs, spectest.Compare(kind, want, got)...)
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

// irKinds returns the names of the types of package exec, in any file, that implement the interface method
// method: the statements (stmt) or the expressions (expr).
func irKinds(t *testing.T, method string) map[string]string {
	t.Helper()
	names, err := filepath.Glob("../*.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("no files in package exec: %v", err)
	}
	out := map[string]string{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			// The marker method of a kind has no parameters and no results; a method of another type with the same name may have some.
			if !ok || fn.Recv == nil || fn.Name.Name != method || len(fn.Recv.List) != 1 || fn.Type.Params.NumFields() != 0 || fn.Type.Results != nil {
				continue
			}
			if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok {
					out[id.Name] = id.Name
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s methods in package exec", method)
	}
	return out
}

// TestSpecModule checks the declarations of the parser VM, the operations and the compositions
// against the specification.
func TestSpecModule(t *testing.T) {
	spec := spectest.Read(t, specDir)
	for _, e := range append(moduleErrors(Build(), spec), sectionErrors(Build(), spec)...) {
		t.Error(e)
	}
}

// TestSpecModuleNegative is the permanent negative control of TestSpecModule: an operation and a
// composition that the specification does not list, a changed signature and a declaration that the
// module no longer has are each reported by name.
func TestSpecModuleNegative(t *testing.T) {
	spec := spectest.Read(t, specDir)
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

// TestSpecSectionsNegative is the permanent negative control of the section check: an operation whose
// component is changed to another sits among the operations of its section and is reported.
func TestSpecSectionsNegative(t *testing.T) {
	m := Build()
	m.Op("rec.push").Component = "depth"
	errs := strings.Join(sectionErrors(m, spectest.Read(t, specDir)), "\n")
	if !strings.Contains(errs, `section "Recovered Errors"`) {
		t.Errorf("no error for the section of the changed operation in:\n%s", errs)
	}
}

// TestSpecSectionsHostNegative is the permanent negative control for a host operation among the
// operations of the value component: an operation of the section of values whose component becomes
// host is reported, as a host operation listed in that section would be.
func TestSpecSectionsHostNegative(t *testing.T) {
	m := Build()
	m.Op("val.kind").Component = ops.Host
	errs := strings.Join(sectionErrors(m, spectest.Read(t, specDir)), "\n")
	if !strings.Contains(errs, `section "Operations on Values"`) {
		t.Errorf("no error for the section of values in:\n%s", errs)
	}
}

// TestSpecStatements checks the statement and expression kinds of Functions against package exec.
func TestSpecStatements(t *testing.T) {
	spec := spectest.Read(t, specDir)
	for _, k := range []struct{ kind, head, method string }{{"statement", "Statement", "stmt"}, {"expression", "Expression", "expr"}} {
		var got [][2]string
		for _, r := range spectest.Column(map[string][]spectest.Table{"functions.md": spec["functions.md"]}, k.head) {
			got = append(got, [2]string{r[0], r[0]})
		}
		for _, e := range spectest.Compare(k.kind, irKinds(t, k.method), got) {
			t.Error(e)
		}
	}
}

// valueKinds are the kinds of a value as val.kind returns them, by code, with the name that the
// specification gives each.
var valueKinds = []struct {
	code int
	name string
}{{ops.KindNil, "nil"}, {ops.KindInt, "int"}, {ops.KindString, "string"}, {ops.KindBool, "bool"}, {ops.KindOther, "other"}}

// kindErrors returns the differences between the rows of the table of value kinds and the kinds of
// the code: the number of rows, and the code and the name of each.
func kindErrors(rows [][]string) []string {
	var errs []string
	if len(rows) != len(valueKinds) {
		errs = append(errs, fmt.Sprintf("value kinds: %d rows, want %d", len(rows), len(valueKinds)))
	}
	for i, r := range rows {
		if i >= len(valueKinds) {
			break
		}
		if k := valueKinds[i]; r[0] != fmt.Sprint(k.code) || r[1] != k.name {
			errs = append(errs, fmt.Sprintf("value kind %d: the specification has %q %q, the code has %d %q", i, r[0], r[1], k.code, k.name))
		}
	}
	return errs
}

// TestSpecValueKindsNegative is the permanent negative control of the value-kinds check: a renamed
// kind and two kinds swapped are each reported.
func TestSpecValueKindsNegative(t *testing.T) {
	rows := [][]string{{"0", "nil"}, {"1", "int"}, {"2", "text"}, {"4", "bool"}, {"3", "other"}}
	errs := strings.Join(kindErrors(rows), "\n")
	for _, want := range []string{`value kind 2: the specification has "2" "text"`, `value kind 3:`, `value kind 4:`} {
		if !strings.Contains(errs, want) {
			t.Errorf("no error %q in:\n%s", want, errs)
		}
	}
}

// TestSpecValues checks the tables of Values against the tables that every target shares.
func TestSpecValues(t *testing.T) {
	spec := spectest.Read(t, specDir)
	tables := map[string]spectest.Table{}
	for _, tb := range spec["values.md"] {
		tables[strings.Join(tb.Head, " ")] = tb
	}
	for _, e := range kindErrors(tables["Code Kind Values"].Rows) {
		t.Error(e)
	}
	groups := map[string]struct{ syms, names []string }{
		"Binary":  {ops.BinaryOps, ops.BinaryNames},
		"Unary":   {ops.UnaryOps, ops.UnaryNames},
		"Logical": {ops.LogicalOps, ops.LogicalNames},
	}
	seen := 0
	for _, r := range tables["Group Operators, by code from 0"].Rows {
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
	if len(members.Rows) != len(ops.MemberKinds) {
		t.Errorf("member kinds: %d rows, want %d", len(members.Rows), len(ops.MemberKinds))
	}
	for i, r := range members.Rows {
		if i >= len(ops.MemberKinds) {
			break
		}
		k := ops.MemberKinds[i]
		if r[0] != fmt.Sprint(i) || r[1] != strings.ToLower(k.Name) || (k.Spelling != "" && r[2] != k.Spelling && !strings.Contains(r[2], "`"+k.Spelling+"`")) {
			t.Errorf("member kind %d: the specification has %q, the code has %+v", i, r, k)
		}
	}
	fails := tables["Code Failure Text Hint Meaning"].Rows
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

// TestSpecSpecializationRuntimes checks that the table of specializations has one column for each
// runtime that prints the compositions; the Go library (internal/codegen/go/lib) and the TypeScript
// generator (internal/codegen/ts) each check their own columns.
func TestSpecSpecializationRuntimes(t *testing.T) {
	want := []string{"Generated Go, nodes", "Generated Go, typed values", "The engine", "TypeScript"}
	if got := spectest.Runtimes(spectest.Read(t, specDir)); !slices.Equal(got, want) {
		t.Errorf("runtimes of the table of specializations: %q, want %q", got, want)
	}
}
