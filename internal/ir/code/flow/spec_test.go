package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The tests of this file hold the Flow IR specification (spec/flow-ir) current with this package:
// every operation of the Flow dialect, with its operands and immediate, every type and every register
// has an entry in the specification, and every entry names one that the dialect has. They live beside
// the code, so that a change of the dialect runs them; the site's tests run only when documentation
// changes.

// specDir is the directory of the Flow IR specification, relative to this package.
const specDir = "../../../../spec/flow-ir"

// specColumn returns the first cells of the rows of the tables of the chapter file whose first header
// cell is head, with a cell that is one code span replaced by its content.
func specColumn(t *testing.T, file, head string) []string {
	t.Helper()
	var out []string
	for _, r := range specRows(t, file, head) {
		out = append(out, r[0])
	}
	return out
}

// specRows returns the rows of the tables of the chapter file whose first header cell is head, with
// each cell trimmed and a cell that is one code span replaced by its content.
func specRows(t *testing.T, file, head string) [][]string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(specDir, file))
	if err != nil {
		t.Fatal(err)
	}
	var out [][]string
	table, in, fenced := false, false, false
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if fenced || !strings.HasPrefix(line, "|") {
			table, in = false, false
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		for i := range cells {
			cells[i] = strings.Trim(strings.TrimSpace(cells[i]), "`")
		}
		first := cells[0]
		switch {
		case strings.HasPrefix(line, "|:") || strings.HasPrefix(line, "|-"):
		case !table:
			// The header row: the rows that follow count only under the header head.
			table, in = true, first == head
		case in:
			out = append(out, cells)
		}
	}
	return out
}

// kindErrors returns the differences between the names that the code declares, want, and those that
// the specification lists, got.
func kindErrors(kind string, want, got []string) []string {
	var errs []string
	for i, w := range want {
		if slices.Contains(want[:i], w) {
			errs = append(errs, fmt.Sprintf("%s %s: named twice by the code, so one has no name of its own", kind, w))
		}
	}
	seen := map[string]bool{}
	for _, g := range got {
		if seen[g] {
			errs = append(errs, fmt.Sprintf("%s %s: listed twice in the specification", kind, g))
		}
		seen[g] = true
		if !slices.Contains(want, g) {
			errs = append(errs, fmt.Sprintf("%s %s: in the specification but not in Flow IR", kind, g))
		}
	}
	for _, w := range want {
		if !seen[w] {
			errs = append(errs, fmt.Sprintf("%s %s: in Flow IR but not in the specification", kind, w))
		}
	}
	return errs
}

// specKinds is what the specification is checked against.
type specKinds struct{ ops, types, regs []string }

func currentKinds() specKinds {
	k := specKinds{regs: []string{Pos, Cut}}
	for _, o := range Ops() {
		k.ops = append(k.ops, o.Name)
	}
	for i := range typeNames {
		k.types = append(k.types, Type(i).String())
	}
	return k
}

// operands returns, for each operation, its operands as the Operands column lists them ("name type",
// with " (variadic)" after the last one of a variadic operation) and the name of its immediate (immName).
func operands() map[string][2]string {
	out := map[string][2]string{}
	for _, o := range Ops() {
		var ps []string
		for _, p := range o.Params {
			ps = append(ps, p.Name+" "+p.Type.String())
		}
		list := strings.Join(ps, ", ")
		if o.Variadic {
			list += " (variadic)"
		}
		imm := ""
		if o.ImmType != nil {
			imm = immName(o.ImmType)
		}
		out[o.Name] = [2]string{list, imm}
	}
	return out
}

// immName is the name by which the specification lists an immediate of the Go type t: the name of
// the type, or of the type it points to, in lower case, which keeps code names out of the specification.
func immName(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return strings.ToLower(t.Name())
}

// operandErrors returns the differences between the operands and immediates that the operations
// declare and those that the specification lists for them.
func operandErrors(t *testing.T, declared map[string][2]string) []string {
	var errs []string
	for _, r := range specRows(t, "operations.md", "Operation") {
		if len(r) < 4 {
			errs = append(errs, fmt.Sprintf("%s: row without the Operands and Immediate columns", r[0]))
			continue
		}
		want, ok := declared[r[0]]
		if !ok {
			continue // reported by kindErrors
		}
		if r[1] != want[0] {
			errs = append(errs, fmt.Sprintf("%s: the specification lists the operands %q, the code declares %q", r[0], r[1], want[0]))
		}
		if r[3] != want[1] {
			errs = append(errs, fmt.Sprintf("%s: the specification lists the immediate %q, the code declares %q", r[0], r[3], want[1]))
		}
	}
	return errs
}

func specErrors(t *testing.T, k specKinds) []string {
	var errs []string
	errs = append(errs, kindErrors("operation", k.ops, specColumn(t, "operations.md", "Operation"))...)
	errs = append(errs, kindErrors("type", k.types, specColumn(t, "functions.md", "Type"))...)
	errs = append(errs, kindErrors("register", k.regs, specColumn(t, "functions.md", "Register"))...)
	errs = append(errs, operandErrors(t, operands())...)
	return errs
}

// TestSpecKinds checks the operations, types and registers of the specification.
func TestSpecKinds(t *testing.T) {
	for _, e := range specErrors(t, currentKinds()) {
		t.Error(e)
	}
}

// TestSpecOperandsNegative is the permanent negative control of the operands check: an operand
// renamed in the code, an operand that the specification lacks, one that the code lacks and a changed
// immediate are each reported.
func TestSpecOperandsNegative(t *testing.T) {
	d := operands()
	d[OpAdvance] = [2]string{"count int", ""}
	d[OpReset] = [2]string{d[OpReset][0] + ", extra int", ""}
	d[OpPushKid] = [2]string{"", ""}
	d[OpSkipBegin] = [2]string{"", "slot"}
	errs := strings.Join(operandErrors(t, d), "\n")
	for _, want := range []string{
		`flow.advance: the specification lists the operands "n int", the code declares "count int"`,
		"flow.reset: the specification lists the operands",
		"flow.pushKid: the specification lists the operands",
		`flow.skipBegin: the specification lists the immediate "site", the code declares "slot"`,
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("no error %q in:\n%s", want, errs)
		}
	}
}

// TestSpecNamesNegative is the permanent negative control for a name that the code gives twice: two
// registers of one name are reported.
func TestSpecNamesNegative(t *testing.T) {
	errs := strings.Join(kindErrors("register", []string{"pos", "env", "pos"}, []string{"pos", "env"}), "\n")
	if !strings.Contains(errs, "register pos: named twice by the code") {
		t.Errorf("no error for the duplicate name in:\n%s", errs)
	}
}

// TestSpecKindsNegative is the permanent negative control of TestSpecKinds: an operation that the
// specification does not list and one that the dialect no longer has are each reported by name.
func TestSpecKindsNegative(t *testing.T) {
	k := currentKinds()
	k.ops = append(k.ops, "flow.planted")
	k.ops = slices.DeleteFunc(k.ops, func(s string) bool { return s == OpMatchLit })
	k.regs = append(k.regs, "planted")
	errs := strings.Join(specErrors(t, k), "\n")
	for _, want := range []string{
		"operation flow.planted: in Flow IR but not in the specification",
		"operation flow.matchLit: in the specification but not in Flow IR",
		"register planted: in Flow IR but not in the specification",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("no error %q in:\n%s", want, errs)
		}
	}
}
