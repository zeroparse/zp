package lint

import "zeroparse.dev/zp/internal/frontend"

// ruleTables are the rule facts the analysis reads, by rule name: the rules the engine finds
// nullable, the rules on a left-recursion cycle, and the rules proven never to fail. They are
// facts of the front end's Core IR module; a value-free twin has the facts of its rule.
type ruleTables struct {
	nullable, cyclic, succeeds map[string]bool
}

func tablesOf(res *frontend.Result) ruleTables {
	t := ruleTables{nullable: map[string]bool{}, cyclic: map[string]bool{}, succeeds: map[string]bool{}}
	for _, r := range res.Module.Rules {
		if r == nil {
			continue
		}
		f := &res.Facts.Rules[r.ID]
		t.nullable[r.Name] = t.nullable[r.Name] || f.Nullable
		t.cyclic[r.Name] = t.cyclic[r.Name] || f.Cyclic()
		t.succeeds[r.Name] = t.succeeds[r.Name] || f.Succeeds
	}
	return t
}
