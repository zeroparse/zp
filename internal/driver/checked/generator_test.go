package checked

import (
	"maps"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/compiletest"
)

// TestGeneratorViewPointsToTheOwners checks the view that the generators read (Source.Generator)
// against the checked program it is built from: the model, the checker and the run-time program are
// the program's own, every rule view embeds the run-time rule of its ID and points to the rule's
// definition, action and capture scope, and the facts it copies (Lean, the twin and the projections)
// are the checked program's. The grammar has a twin and a projected repetition.
func TestGeneratorViewPointsToTheOwners(t *testing.T) {
	g, err := compiletest.Parse(`
@start
rule main = items !(item ";")
rule items = (first:item rest:(-"," r:item)*) => concat(list($first), map($rest, (e) => e.$r))
rule item = [a-z]+
`)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Compile(g, compile.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Checked()
	v := s.Generator()
	run := p.Runtime()
	if v.Model != p.Model || v.Checker() != p.Checker() || v.Typed != p.Typed || v.Run != run || v.Elab != p {
		t.Fatal("the view does not hold the program's model, checker, types, run-time program and elaboration input")
	}
	if len(v.RuleList) != len(p.RuleList) || len(v.Twins) != len(p.Twins) || len(v.ByName) != len(p.ByName) {
		t.Fatalf("rules %d/%d, twins %d/%d, names %d/%d", len(v.RuleList), len(p.RuleList), len(v.Twins), len(p.Twins), len(v.ByName), len(p.ByName))
	}
	check := func(vr *common.Rule, r *compile.Rule, want *common.Rule) {
		t.Helper()
		if vr.Rule != want.Rule || vr.ID != r.ID || vr.Name != r.Name {
			t.Errorf("rule %s (%d): the view embeds rule %s (%d), not the run-time rule of its ID", r.Name, r.ID, vr.Name, vr.ID)
		}
		if vr.Def != r.Def || vr.Action != r.Action || vr.Captures != r.Scope || vr.Lean != r.Lean {
			t.Errorf("rule %s (%d): definition, action, scope or Lean differ from the checked program's", r.Name, r.ID)
		}
		if vr.BodyExpr() != r.BodyExpr() {
			t.Errorf("rule %s (%d): body differs", r.Name, r.ID)
		}
		if !maps.Equal(vr.Projections, compile.RuleProjections(p, r)) {
			t.Errorf("rule %s (%d): projections %v, want %v", r.Name, r.ID, vr.Projections, compile.RuleProjections(p, r))
		}
	}
	projected, twins := false, 0
	for i, r := range p.RuleList {
		vr := v.RuleList[i]
		check(vr, r, &common.Rule{Rule: run.RuleList[i]})
		if v.ByName[r.Name] != vr {
			t.Errorf("rule %s: ByName holds another view", r.Name)
		}
		switch {
		case r.Twin == nil && vr.Twin != nil:
			t.Errorf("rule %s: the view has a twin, the program none", r.Name)
		case r.Twin != nil && (vr.Twin == nil || vr.Twin.Rule != run.Twins[r.Twin.ID-len(p.RuleList)]):
			t.Errorf("rule %s: the view's twin is not the run-time twin of the program's", r.Name)
		case r.Twin != nil:
			twins++
		}
		projected = projected || len(vr.Projections) > 0
	}
	for i, r := range p.Twins {
		check(v.Twins[i], r, &common.Rule{Rule: run.Twins[i]})
	}
	if twins == 0 || !projected {
		t.Fatalf("the grammar makes %d twins and projections %v: the test needs both", twins, projected)
	}
}
