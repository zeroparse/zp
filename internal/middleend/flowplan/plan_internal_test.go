package flowplan

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/driver"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/testing/compiletest"
)

// TestPlanTakesTheFallbackOfARefusedFunction checks the decision of the plan: a flat function that
// the refusal rejects is replaced by its framed fallback under the same key, and an accepted one
// stays flat. (No grammar of the repository has a flat function that Refused rejects.)
func TestPlanTakesTheFallbackOfARefusedFunction(t *testing.T) {
	prog := compiletest.Program(t, "rule main = a b !.\nrule a = \"a\"+\nrule b = [b]*\n", compile.Options{Recognition: compile.Recognize})
	m, err := elaborate.CoreProjecting(prog, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if m, _, err = corepass.Run(m, &corepass.Env{}, corepass.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	var rules []*Rule
	byName := map[string]int{}
	for _, r := range append(append([]*compile.Rule(nil), prog.RuleList...), prog.Twins...) {
		rules = append(rules, &Rule{ID: r.ID, Name: r.Name, Body: r.Def.Expr, Action: r.Action, Plain: r.Plain, Lean: r.Lean, Novalue: r.Novalue, Leader: r.Leader})
		byName[r.Name] = r.ID
	}
	ir := &TableIR{M: m, Opts: flowlower.Options{Budget: true, Recovers: flowlower.Recovers(m), Node: true}}
	all := route(m, rules, nil)
	for _, name := range []string{"main", "a"} {
		if s, ok := all.Func(Key{Rule: byName[name]}); !ok || s.Form != flowlower.Flat || s.Fallback == nil {
			t.Fatalf("rule %s is not routed flat with a fallback: %+v", name, s)
		}
	}
	refuse := func(l *execlower.Lowered) bool { return l.Func.Name == "a" }
	d := &driver.Driver{}
	p, err := plan(ir, rules, nil, func(u flowlower.Unit) (*execlower.Lowered, error) { return d.PlanExec("rules", u, false) }, refuse)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := p.Func(Key{Rule: byName["main"]}); !ok || s.Form != flowlower.Flat {
		t.Errorf("main: %+v, want the accepted flat function", s)
	}
	if s, ok := p.Func(Key{Rule: byName["a"]}); !ok || s.Form != flowlower.Frame || s != all.byKey[Key{Rule: byName["a"]}].Fallback && s.Body != all.byKey[Key{Rule: byName["a"]}].Fallback.Body {
		t.Errorf("a: %+v, want its framed fallback", s)
	}
	for _, s := range p.Funcs {
		if s.Key.Rule == byName["a"] && s.Form == flowlower.Flat {
			t.Error("a: the refused flat function stays in the functions of the plan")
		}
	}
}
