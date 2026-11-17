package corepass

import (
	"strconv"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
)

// The builders below make Core IR the way the tests of package core do.

func lit(s string) *core.Lit { return &core.Lit{Text: s} }

func class(lo, hi rune) *core.ClassNode {
	return &core.ClassNode{Class: &core.Class{Ranges: []core.Range{{Lo: lo, Hi: hi}}}}
}

func seq(items ...core.Node) *core.Seq { return &core.Seq{Items: items} }

func choice(bodies ...core.Node) *core.Choice {
	c := &core.Choice{}
	for _, b := range bodies {
		c.Alts = append(c.Alts, &core.Alt{Body: b})
	}
	return c
}

func rule(id int, name string, body core.Node, scope ...string) *core.Rule {
	var s []core.Slot
	for _, n := range scope {
		s = append(s, core.Slot{Name: n})
	}
	return &core.Rule{ID: id, Name: name, Body: body, Want: true, Scope: s, Policy: core.Policy{Seen: -1}}
}

// module makes a module of the rules, as the builder does: it interns the expectations of the
// literals and labels and the classes, and sets the call graph. A label's expectation is
// "label".
func module(rules ...*core.Rule) *core.Module {
	m := &core.Module{Rules: rules}
	ids := map[string]core.ExpID{}
	exp := func(s string) core.ExpID {
		id, ok := ids[s]
		if !ok {
			id = core.ExpID(len(m.Exps))
			ids[s] = id
			m.Exps = append(m.Exps, s)
		}
		return id
	}
	seen := map[*core.Class]bool{}
	intern := func(c *core.Class) {
		if !seen[c] {
			seen[c] = true
			c.Exp = exp(core.ClassString(c))
			m.Classes = append(m.Classes, c)
		}
	}
	for _, r := range rules {
		if r == nil {
			continue
		}
		core.Walk(r.Body, func(n core.Node) {
			switch n := n.(type) {
			case *core.Lit:
				if n.Text != "" {
					n.Exp = exp(strconv.Quote(n.Text))
				}
			case *core.ClassNode:
				intern(n.Class)
			case *core.Label:
				n.Exp = exp("label")
			case *core.Scan:
				if n.Class != nil {
					intern(n.Class)
				}
			case *core.ClassRun:
				for _, c := range n.Classes {
					intern(c)
				}
			}
		})
	}
	core.CallGraph(m)
	for i, r := range m.Rules {
		if r != nil { // the rules on a cycle of calls count against the nesting limit
			r.Policy.Rec = m.Cyclic[m.SCC[i]]
		}
	}
	return m
}

// analyzed runs Analyze and Verify on m.
func analyzed(t *testing.T, m *core.Module) *core.Module {
	t.Helper()
	core.Analyze(m)
	if err := core.Verify(m); err != nil {
		t.Fatal(err)
	}
	return m
}

func firstString(m *core.Module, f *core.First) string { return core.FirstString(m, f) }
