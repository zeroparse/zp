package flowlower

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowpass"
)

// finish runs on the lowered module f the Flow passes of the target, with inline as the number of
// code points of the inline-literals pass, and ends them, as the driver does after the lowering.
func finish(t *testing.T, f *code.Module, target flowpass.Target, inline int) *code.Module {
	t.Helper()
	env := &flowpass.Env{Target: target, Value: func(name string) int {
		if name == flowpass.PassInlineLiterals {
			return inline
		}
		return 0
	}}
	if _, err := flowpass.Run(f, env, flowpass.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	flowpass.Finish(f)
	return f
}

// lowerFinished lowers the rule r of m and ends the passes, with the in-place literals of at most
// inline code points when inline is positive.
func lowerFinished(t *testing.T, m *core.Module, r *core.Rule, opts Options, inline int) (*code.Module, error) {
	t.Helper()
	f, err := Lower(m, r, "r", opts)
	if err != nil {
		return nil, err
	}
	return finish(t, f, flowpass.Target{InlineLiterals: inline > 0}, inline), nil
}
