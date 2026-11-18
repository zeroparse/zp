package flowlower

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/action"
	"zeroparse.dev/zp/internal/ir/code/exec"
)

// TestTermLeavesWriteOwnComponent pins that the leaves of terms (addTermOps) read and write only their own
// component: what terms gather and make is the state of the components gathered and made, whose operations
// have reference models, and a leaf that wrote it would leave that state outside the models. Another
// writer needs a reason recorded here, as TestCrossComponentWriters of package ops records those of the
// parser VM.
func TestTermLeavesWriteOwnComponent(t *testing.T) {
	m, err := termModule()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range m.Ops {
		if o.Component != action.Runtime {
			continue
		}
		for _, s := range slices.Concat(o.Reads, o.Writes) {
			if s != action.Runtime {
				got = append(got, o.Name+" uses "+s)
			}
		}
	}
	if len(got) != 0 {
		t.Errorf("leaves of terms that use another component: %q", got)
	}
}

// TestTermLeavesArePrimitives checks that the Action dialect declares no transitional operation, and that
// the leaves of terms are primitive operations of the Action dialect whose immediate operands are
// typed.
func TestTermLeavesArePrimitives(t *testing.T) {
	m, err := termModule()
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range m.Ops {
		if o.Dialect == action.Dialect && o.Kind == code.OpTransitional {
			t.Errorf("%s is a transitional operation", o.Name)
		}
		if o.Component != action.Runtime {
			continue
		}
		if o.Kind != code.OpPrimitive || o.Dialect != action.Dialect {
			t.Errorf("%s: %s of dialect %q, want a primitive op of the Action dialect", o.Name, o.Kind, o.Dialect)
		}
		if (o.Imm == "") != (o.ImmType == nil) {
			t.Errorf("%s: immediate %q with type %v", o.Name, o.Imm, o.ImmType)
		}
	}
}

// TestTermLeafImmediateChecked is the negative control of the typed immediates: a leaf whose immediate
// operand has another type fails verification.
func TestTermLeafImmediateChecked(t *testing.T) {
	for _, tc := range []struct {
		name string
		imm  any
		ok   bool
	}{
		{"read of a capture", action.Read{Kind: action.ReadCapture, Name: "x"}, true},
		{"read with a constant", action.Const{Value: 1}, false},
		{"read with no immediate", nil, false},
	} {
		m, err := termModule()
		if err != nil {
			t.Fatal(err)
		}
		nOps, nFuncs := len(m.Ops), len(m.Funcs)
		m.AddFunc("leaf", nil, []code.Type{code.THandle(exec.Datum)}, nil, func(b *code.Builder) {
			b.Return(&code.Call{Callee: action.OpTermRead, Imm: tc.imm})
		})
		if err := code.VerifyExtension(m, nOps, nFuncs); (err == nil) != tc.ok {
			t.Errorf("%s: verify = %v, want ok %v", tc.name, err, tc.ok)
		}
	}
}
