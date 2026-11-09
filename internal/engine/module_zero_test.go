package engine

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
)

// TestModuleZeroDescriptorValidation checks that a module whose zero descriptors are not well formed
// is rejected when it is loaded, since a runtime builds the zero value from them: a descriptor that is
// not one descriptor, one that names a type that is not a struct type of the module, and one that
// leads a struct type back to itself. Well-formed descriptors, including the one a field without a zero
// value has, pass.
func TestModuleZeroDescriptorValidation(t *testing.T) {
	t.Parallel()
	prog := compileBare(t, "type In = { N int }\ntype Z = { I int; T (int, string); S In }\nrule z Z = \"z\" => Z{}\nrule main = x:z $$ => $x.I\n")
	base := prog.Module()
	if err := compile.ValidateModule(base); err != nil {
		t.Fatal(err)
	}
	with := func(desc string) *bytecode.Module {
		m := *base
		m.Strings = append(append([]string(nil), base.Strings...), desc)
		m.Types = append([]bytecode.TypeInfo(nil), base.Types...)
		for i := range m.Types {
			if m.Strings[m.Types[i].Name] == "Z" {
				m.Types[i].Zeros = append([]int(nil), m.Types[i].Zeros...)
				m.Types[i].Zeros[0] = len(m.Strings) - 1 // the field I
			}
		}
		return &m
	}
	for _, d := range []string{"i", "s", "b", "n", "l", "-", "()", "(i(sb))", "{In}", "({In}l)"} {
		if err := compile.ValidateModule(with(d)); err != nil {
			t.Errorf("descriptor %q: %v", d, err)
		}
	}
	for _, tc := range []struct{ desc, want string }{
		{"{Z", "malformed"},       // unterminated: the reading of it once recursed without end
		{"{Z}", "reaches itself"}, // a struct type that holds its own zero value
		{"{}", "malformed"},
		{"x", "malformed"},
		{"{Nope}", "not a struct type"},
		{"(", "malformed"},
		{"(i", "malformed"},
		{"ii", "malformed"},
		{"", "malformed"},
		{"(-)", "malformed"}, // a type without a zero value has no zero value in a tuple either
	} {
		if err := compile.ValidateModule(with(tc.desc)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("descriptor %q: got %v, want an error with %q", tc.desc, err, tc.want)
		}
	}
}
