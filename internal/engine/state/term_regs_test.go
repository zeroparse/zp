package state

import (
	"testing"

	"zeroparse.dev/zp/core/noderep"
	"zeroparse.dev/zp/internal/compile"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/value"
)

// TestTermEvalClearsRegisters checks that a term program gives its registers back cleared: the
// registers outlive the evaluation in the parser (and in a pooled parser, the parse), so a value
// left in one would stay reachable after its term returned.
func TestTermEvalClearsRegisters(t *testing.T) {
	prog, err := CompileTerm(&ir.Local{Kind: ir.LocalOp, Name: "op"}, TermEnv{Prog: &compile.Program{}, Scope: &compile.Scope{}})
	if err != nil {
		t.Fatal(err)
	}
	p := &Parser{}
	op := value.Ptr(value.Data{Kind: noderep.SeqKind, Start: 0, End: 1})
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
