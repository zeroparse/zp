package astfacts

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func proofJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPolledSuccessFacts(t *testing.T) {
	committed := &ast.Seq{Items: []ast.Expr{&ast.Cut{}, &ast.Literal{Value: "a"}}}
	for _, test := range []struct {
		name         string
		expr         ast.Expr
		success, cut bool
	}{
		{"empty", &ast.Literal{}, true, false},
		{"known call", &ast.Ref{Name: "known"}, true, false},
		{"level call", &ast.Ref{Name: "known", Level: "named"}, false, false},
		{"unknown call", &ast.Ref{Name: "unknown"}, false, false},
		{"committed failure", committed, false, true},
		{"committed choice", &ast.Choice{Alts: []ast.Expr{committed, &ast.Top{}}}, false, false},
		{"committed optional", &ast.Optional{Expr: committed}, false, false},
		{"empty repeat", &ast.Repeat{Expr: committed, Max: 0}, true, false},
		{"larger minimum", &ast.Repeat{Expr: &ast.Top{}, Min: 2, Max: 3}, false, false},
		{"negative failure", &ast.Not{Expr: &ast.Bottom{}}, true, false},
		{"nested negative", &ast.Not{Expr: &ast.Not{Expr: &ast.Top{}}}, true, false},
		{"positive", &ast.And{Expr: &ast.Top{}}, true, false},
		{"group", &ast.Group{Expr: &ast.Cut{}}, true, true},
		{"capture", &ast.Capture{Expr: &ast.Cut{}}, true, true},
		{"atomic", &ast.Atomic{Expr: &ast.Cut{}}, true, true},
		{"discard", &ast.Discard{Expr: &ast.Cut{}}, true, true},
		{"stream", &ast.Stream{Expr: &ast.Cut{}}, true, true},
		{"recovery", &ast.Recovery{Expr: &ast.Bottom{}, Skip: &ast.Cut{}}, false, true},
		{"recovery attribute", &ast.Attributed{Expr: &ast.Top{}, Attrs: []*ast.Attribute{{Name: "recover", KwArgs: map[string]any{"skip": &ast.Cut{}}}}}, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sure := map[string]bool{"known": true}
			before := proofJSON(t, test.expr)
			polls := 0
			poll := func() { polls++ }
			for _, callback := range []func(){nil, poll} {
				if got := AlwaysSucceedsWithPoll(test.expr, sure, callback); got != test.success || got != AlwaysSucceeds(test.expr, sure) {
					t.Fatalf("success = %v, want %v", got, test.success)
				}
				if got := HasLooseCutWithPoll(test.expr, callback); got != test.cut || got != HasLooseCut(test.expr) {
					t.Fatalf("loose cut = %v, want %v", got, test.cut)
				}
			}
			after := proofJSON(t, test.expr)
			if polls == 0 || before != after || !reflect.DeepEqual(sure, map[string]bool{"known": true}) {
				t.Fatal("proof did not poll or modified its input")
			}
		})
	}
}

func TestPolledSuccessFixedPoint(t *testing.T) {
	rules := []*ast.RuleDef{
		{Name: "main", Expr: &ast.Ref{Name: "later"}},
		{Name: "later", Expr: &ast.Top{}},
		{Name: "cycle", Expr: &ast.Top{}},
	}
	cyclic := map[string]bool{"cycle": true}
	polls := 0
	got := SucceedingRulesWithPoll(rules, cyclic, func() { polls++ })
	want := map[string]bool{"main": true, "later": true}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(got, SucceedingRules(rules, cyclic)) || polls == 0 || !reflect.DeepEqual(cyclic, map[string]bool{"cycle": true}) {
		t.Fatalf("facts = %v, polls = %d", got, polls)
	}
}

func TestSharedProofCancellation(t *testing.T) {
	tops, bottoms := make([]ast.Expr, 1024), make([]ast.Expr, 1024)
	attrs := make([]*ast.Attribute, 1024)
	rules := make([]*ast.RuleDef, 1024)
	cyclic := map[string]bool{}
	for i := range tops {
		tops[i], bottoms[i] = &ast.Top{}, &ast.Bottom{}
		attrs[i] = &ast.Attribute{Name: "error"}
		name := fmt.Sprint(i)
		rules[i] = &ast.RuleDef{Name: name, Expr: &ast.Top{}}
		cyclic[name] = true // Excluded rules still require bounded polling.
	}
	for name, run := range map[string]func(func()){
		"success": func(p func()) { AlwaysSucceedsWithPoll(&ast.Seq{Items: tops}, nil, p) },
		"failure": func(p func()) { AlwaysSucceedsWithPoll(&ast.Not{Expr: &ast.Choice{Alts: bottoms}}, nil, p) },
		"cut":     func(p func()) { HasLooseCutWithPoll(&ast.Seq{Items: tops}, p) },
		"failure attributes": func(p func()) {
			AlwaysSucceedsWithPoll(&ast.Not{Expr: &ast.Attributed{Expr: &ast.Bottom{}, Attrs: attrs}}, nil, p)
		},
		"cut attributes": func(p func()) { HasLooseCutWithPoll(&ast.Attributed{Expr: &ast.Top{}, Attrs: attrs}, p) },
		"excluded rules": func(p func()) { SucceedingRulesWithPoll(rules, cyclic, p) },
	} {
		t.Run(name, func(t *testing.T) {
			before := proofJSON(t, []any{tops, bottoms, attrs, rules, cyclic})
			token, polls := new(int), 0
			func() {
				defer func() {
					if recover() != token || polls != 17 {
						t.Fatal("cancellation was swallowed or changed")
					}
				}()
				run(func() {
					polls++
					if polls == 17 {
						panic(token)
					}
				})
				t.Fatal("proof finished without observing cancellation")
			}()
			after := proofJSON(t, []any{tops, bottoms, attrs, rules, cyclic})
			if before != after {
				t.Fatal("interrupted proof modified its input")
			}
		})
	}
}
