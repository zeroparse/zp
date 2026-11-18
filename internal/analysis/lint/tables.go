package lint

import (
	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/middleend/corepass"
)

// ruleTables are the rule facts the analysis reads, by rule name: the rules the engine finds
// nullable, the rules on a left-recursion cycle, and the rules proven never to fail. They are
// facts of the front end's Core IR module; a value-free twin has the facts of its rule.
type ruleTables struct {
	nullable, cyclic, succeeds map[string]bool
}

// tablesOf reads the tables from the rule facts of res, through the manager of its analyses.
func tablesOf(res *frontend.Result) ruleTables {
	t := ruleTables{nullable: map[string]bool{}, cyclic: map[string]bool{}, succeeds: map[string]bool{}}
	facts := corepass.FactsOf(res.Analyses, res.Module)
	for _, r := range res.Module.Rules {
		if r == nil {
			continue
		}
		f := &facts.Rules[r.ID]
		t.nullable[r.Name] = t.nullable[r.Name] || f.Nullable
		t.cyclic[r.Name] = t.cyclic[r.Name] || f.Cyclic()
		t.succeeds[r.Name] = t.succeeds[r.Name] || f.Succeeds
	}
	return t
}
