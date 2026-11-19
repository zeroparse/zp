package compile

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/types"
)

func TestRuleTypeWithPollParity(t *testing.T) {
	record := types.RecordTy{Fields: map[string]types.Ty{"z": types.ListTy{Elem: types.JoinTy{Alts: []types.Ty{types.TyMatch, types.TyNil}}}, "a": types.OptTy{Elem: types.TyString}}}
	cases := []types.Ty{types.TyAny, types.TyNever, types.TyNil, types.TyInt, types.TyBool, types.TyString, types.TyNode, types.TyMatch, types.ListTy{Elem: types.TyMatch}, types.OptTy{Elem: types.TyMatch}, types.JoinTy{}, types.JoinTy{Alts: []types.Ty{types.TyMatch, types.TySeq, types.TyMissing}}, record, types.RecordTy{Fields: map[string]types.Ty{}}, types.ListTy{Elem: record},
		// Value-model types
		types.TyTop, types.TyBottom, types.RuleTy{Name: "expr"}, types.NodeTy{Value: types.TyTop}, types.NodeTy{Value: types.ListTy{Elem: types.TyStringNode}}, types.TupleTy{Elems: []types.Ty{types.TyStringNode}},
		types.NodeTy{Value: types.TupleTy{Elems: []types.Ty{types.OptTy{Elem: types.TyStringNode}, types.RuleTy{Name: "x"}}}, Caps: []types.CapIndex{{Name: "a", Paths: [][]int{{0}, {1, 0}}}, {Name: "b", Side: true}}}}
	for _, typ := range cases {
		p := shownProgram(map[string]types.Ty{"main": typ})
		want, known := p.RuleType("main")
		if !known {
			t.Fatal("missing test rule")
		}
		calls := 0
		for _, poll := range []func(){nil, func() { calls++ }} {
			got, ok := p.RuleTypeWithPoll("main", poll)
			if !ok || got != want {
				t.Fatalf("%T: %q/%v want %q", typ, got, ok, want)
			}
		}
		if calls < 2 {
			t.Fatal("type traversal did not poll")
		}
		if typ.String() != want {
			t.Fatal("type traversal changed its input")
		}
	}
	for _, p := range []*Program{{}, shownProgram(map[string]types.Ty{})} {
		for _, poll := range []func(){nil, func() {}} {
			if got, ok := p.RuleTypeWithPoll("missing", poll); got != "" || ok {
				t.Fatalf("missing=%q/%v", got, ok)
			}
		}
	}
}

func TestRuleTypeWithPollCancelsWithinCollections(t *testing.T) {
	var union types.JoinTy
	record := types.RecordTy{Fields: map[string]types.Ty{}}
	for i := range 1024 {
		union.Alts = append(union.Alts, types.TyMatch)
		record.Fields[fmt.Sprintf("f%d", i)] = types.TyMatch
	}
	for _, typ := range []types.Ty{union, record, types.JoinTy{Alts: []types.Ty{types.NamedTy{Name: strings.Repeat("x", 1<<20)}, types.TyMatch}}} {
		func() {
			marker := &struct{}{}
			calls := 0
			defer func() {
				if got := recover(); got != marker {
					t.Fatalf("%T did not preserve cancellation callback: %v", typ, got)
				}
			}()
			p := shownProgram(map[string]types.Ty{"main": typ})
			p.RuleTypeWithPoll("main", func() {
				calls++
				if calls == 32 {
					panic(marker)
				}
			})
			t.Fatal("large type completed without cancellation callback")
		}()
	}
}

type renderProbeTy struct {
	done    *bool
	failure any
}

func (t renderProbeTy) String() string {
	*t.done = true
	if t.failure != nil {
		panic(t.failure)
	}
	return "T"
}
func TestRuleTypeWithPollRecursiveReturn(t *testing.T) {
	done := false
	var typ types.Ty = renderProbeTy{done: &done}
	for range 1024 {
		typ = types.ListTy{Elem: typ}
	}
	marker := &struct{}{}
	returns := 0
	defer func() {
		if got := recover(); got != marker {
			t.Fatalf("recursive return did not stop: %v", got)
		}
	}()
	renderRuleType(typ, func() {
		if done {
			returns++
			if returns == 256 {
				panic(marker)
			}
		}
	})
	t.Fatal("recursive returns completed without polling")
}
func TestRuleTypeWithPollPreservesForeignPanic(t *testing.T) {
	done := false
	marker := &struct{}{}
	replacement := &struct{ n int }{1}
	defer func() {
		if got := recover(); got != marker {
			t.Fatalf("foreign panic changed: %v", got)
		}
	}()
	renderRuleType(types.ListTy{Elem: renderProbeTy{done: &done, failure: marker}}, func() {
		if done {
			panic(replacement)
		}
	})
}

// shownProgram returns a program whose checker and its program hold types that show the types shown.
func shownProgram(shown map[string]types.Ty) *Program {
	info := check.NewShownInfo(shown)
	p := &Program{Typed: info}
	p.Checker().Typed = info
	return p
}
