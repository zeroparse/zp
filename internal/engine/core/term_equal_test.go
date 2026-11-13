package core

import (
	"context"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// TestEngineValueEqualCharges holds ValueEqual, whose fast path for two strings is written by hand,
// to the composition termEqual: the same answer for every pair of strings and, under a budget
// control, the same units, which a fast path that ignores the control would not charge.
func TestEngineValueEqualCharges(t *testing.T) {
	var strs []string
	for _, v := range conform.ValuePool() {
		if v.Kind == ref.KStr {
			strs = append(strs, v.S)
		}
	}
	long := strings.Repeat("ab", 300)
	strs = append(strs, long, long[:len(long)-1]+"c", long[:len(long)-1])
	for _, controlled := range []bool{false, true} {
		for _, l := range strs {
			for _, r := range strs {
				// Copies, so that two equal strings are not the same bytes of the same memory.
				l, r := strings.Clone(l), strings.Clone(r)
				var viaFast, viaComposition uint64
				var got, want bool
				for i, eval := range []func(p *Parser){
					func(p *Parser) { got = p.ValueEqual(l, r) },
					func(p *Parser) { want, _ = p.termEqual(l, r) },
				} {
					p := &Parser{}
					if controlled {
						p.Input.Control = NewBudgetControl(context.Background(), core.Limits{Compute: 1 << 62})
					}
					eval(p)
					used := uint64(0)
					if controlled {
						used = p.Control.Used
					}
					if i == 0 {
						viaFast = used
					} else {
						viaComposition = used
					}
				}
				if got != want {
					t.Errorf("controlled=%v: ValueEqual(%.8q, %.8q) = %v, termEqual says %v", controlled, l, r, got, want)
				}
				if viaFast != viaComposition {
					t.Errorf("controlled=%v: ValueEqual(%.8q, %.8q) charges %d, termEqual charges %d", controlled, l, r, viaFast, viaComposition)
				}
			}
		}
	}
}
