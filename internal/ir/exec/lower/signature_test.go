package lower

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// TestLoweredTermHasTheSignatureOfTerm holds the definition of a lowered term to the callable type Term, which
// is what an application of a term runs: the lowered function takes the context and returns the value and the
// failure. The negative controls change the function by one parameter or result and must be rejected.
func TestLoweredTermHasTheSignatureOfTerm(t *testing.T) {
	l, err := Term(&core.Const{Value: 1}, TermOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := checkTermSignature(l.Module, l.Func); err != nil {
		t.Fatalf("the lowered term does not have the signature of Term: %v", err)
	}
	if len(l.Func.Params) != 1 || l.Func.Params[0].Name != CtxParam {
		t.Fatalf("params = %v, want the context %s", l.Func.Params, CtxParam)
	}
	act, datum, fail := exec.THandle(ops.Act), exec.THandle(ops.Datum), exec.THandle(ops.Fail)
	for _, c := range []struct {
		name    string
		params  []exec.Param
		results []exec.Type
	}{
		{"no context", nil, []exec.Type{datum, fail}},
		{"a second parameter", []exec.Param{exec.P("c", act), exec.P("x", datum)}, []exec.Type{datum, fail}},
		{"a parameter of another type", []exec.Param{exec.P("c", datum)}, []exec.Type{datum, fail}},
		{"no failure", []exec.Param{exec.P("c", act)}, []exec.Type{datum}},
		{"a third result", []exec.Param{exec.P("c", act)}, []exec.Type{datum, fail, fail}},
		{"results in the other order", []exec.Param{exec.P("c", act)}, []exec.Type{fail, datum}},
	} {
		f := &exec.Func{Name: "term", Params: c.params, Results: c.results}
		err := checkTermSignature(l.Module, f)
		if err == nil || !strings.Contains(err.Error(), "signature of Term") {
			t.Errorf("%s: err = %v, want the signature of Term to be reported", c.name, err)
		}
	}
}

// TestTermChecksItsSignature goes through Term itself: when the callable type Term declares another signature than
// the function that Term lowers, lowering fails, so a lowering that dropped the check would pass for any change to
// the signature. The test changes the handle type of the shared base module of terms and restores it.
func TestTermChecksItsSignature(t *testing.T) {
	m, err := termModule()
	if err != nil {
		t.Fatal(err)
	}
	h := m.HandleType(ops.Term)
	saved := h.Callable
	defer func() { h.Callable = saved }()
	for _, c := range []struct {
		name string
		sig  exec.Signature
	}{
		{"a second parameter", exec.Signature{Params: append(slices.Clone(saved.Params), saved.Params[0]), Results: saved.Results}},
		{"one result", exec.Signature{Params: saved.Params, Results: saved.Results[:1]}},
		{"no parameter", exec.Signature{Results: saved.Results}},
	} {
		h.Callable = &c.sig
		_, err := Term(&core.Const{Value: 1}, TermOptions{})
		if err == nil || !strings.Contains(err.Error(), "signature of Term") {
			t.Errorf("%s: err = %v, want the signature of Term to be reported", c.name, err)
		}
	}
	h.Callable = saved
	if _, err := Term(&core.Const{Value: 1}, TermOptions{}); err != nil {
		t.Errorf("with the declared signature: %v", err)
	}
}
