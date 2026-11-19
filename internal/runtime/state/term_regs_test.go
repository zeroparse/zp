package state

import (
	"testing"

	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// TestTermEvalClearsRegisters checks that a term program gives its registers back cleared: the
// registers outlive the evaluation in the parser (and in a pooled parser, the parse), so a value
// left in one would stay reachable after its term returned.
func TestTermEvalClearsRegisters(t *testing.T) {
	// The term $op: the Operators Rule local op, returned.
	prog := &TermProgram{Code: []TermInstr{{Op: TLocal, Dst: 0, A: 0}, {Op: TRet, A: 0}}, NA: 1, Names: []string{"op"}}
	p := &Parser{}
	op := nodes.Ptr(nodes.Data{Kind: valueimpl.SeqKind, Start: 0, End: 1})
	got, err := prog.Eval(&EvalCtx{P: p, Locals: &Local{Name: "op", Val: op}})
	if err != nil {
		t.Fatal(err)
	}
	if got != any(op) {
		t.Fatalf("the term returned %v, want $op", got)
	}
	if len(p.terms.a) != 0 {
		t.Fatalf("%d value registers stay taken after the term returned", len(p.terms.a))
	}
	for i, v := range p.terms.a[:cap(p.terms.a)] {
		if v != nil {
			t.Errorf("value register %d still holds %v after the term returned", i, v)
		}
	}
}
