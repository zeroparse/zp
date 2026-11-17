package bytecode_test

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/enginetest"
)

// TestModuleRecursionValidation checks that a module in which a rule reaches a call of itself through
// rules not marked recursive is rejected when it is loaded: only the entries of recursive rules count
// against the nesting limit, so such a recursion would end only when the host stack overflows. The
// compiled modules pass, and so does a cycle with one recursive rule on it; a cleared mark, a rule
// entry moved to the body of a caller and a CALL changed to call a caller are rejected.
func TestModuleRecursionValidation(t *testing.T) {
	t.Parallel()
	prog := enginetest.CompileBare(t, `rule main = (stmt ";")* $$
rule stmt = name "=" item
rule item = "(" list ")" / name
rule list = item ("," item)*
rule name = [a-z]+ #atomic`)
	base := prog.Module()
	if err := compile.ValidateModule(base); err != nil {
		t.Fatal(err)
	}
	rule := func(m *bytecode.Module, name string) int {
		i := slices.IndexFunc(m.Rules, func(r bytecode.RuleInfo) bool { return m.Strings[r.Name] == name })
		if i < 0 {
			t.Fatalf("no rule %s", name)
		}
		return i
	}
	clone := func() *bytecode.Module {
		m := *base
		m.Rules = slices.Clone(base.Rules)
		m.Code = slices.Clone(base.Code)
		return &m
	}
	for _, name := range []string{"item", "list"} {
		if !base.Rules[rule(base, name)].Rec {
			t.Fatalf("rule %s is not marked recursive", name)
		}
	}
	for _, name := range []string{"main", "stmt", "name"} {
		if base.Rules[rule(base, name)].Rec {
			t.Fatalf("rule %s is marked recursive", name)
		}
	}
	reject := func(what string, m *bytecode.Module) {
		t.Helper()
		if err := compile.ValidateModule(m); err == nil || !strings.Contains(err.Error(), "through rules not marked recursive") {
			t.Errorf("%s: got %v, want an error about the recursion", what, err)
		}
	}

	// One recursive rule on the cycle bounds its nesting.
	m := clone()
	m.Rules[rule(m, "list")].Rec = false
	if err := compile.ValidateModule(m); err != nil {
		t.Errorf("one mark cleared: %v", err)
	}
	m.Rules[rule(m, "item")].Rec = false
	reject("both marks cleared", m)

	// The entry of name moved to that of main: name runs main's body, which calls name again
	// through stmt.
	m = clone()
	m.Rules[rule(m, "name")].Entry = m.Rules[rule(m, "main")].Entry
	reject("entry moved", m)

	// The CALL of name in stmt changed to call main, which calls stmt.
	m = clone()
	ip := m.Rules[rule(m, "stmt")].Entry
	for m.Code[ip].Op != bytecode.OpCall || int(m.Code[ip].A) != rule(m, "name") {
		if m.Code[ip].Op == bytecode.OpReturn {
			t.Fatal("stmt does not call name")
		}
		ip++
	}
	m.Code[ip].A = int32(rule(m, "main"))
	reject("call changed", m)
}
