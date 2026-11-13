//go:build !zp_budget_controls_reference

package vm

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

func TestBudgetStreamDiscardAndStackClear(t *testing.T) {
	for _, u := range []parse.PositionUnit{parse.Bytes, parse.CodePoints} {
		t.Run(u.String(), func(t *testing.T) {
			in := core.Input{Unit: u, Eof: true, BaseLine: 1, BaseCol: 1, Lines: []int{0}, Control: core.NewBudgetControl(nil, parse.Limits{Compute: 128})}
			if u == parse.Bytes {
				in.Bs = []byte(strings.Repeat("a", 258))
			} else {
				in.In = []rune(strings.Repeat("a", 258))
			}
			got := catchStreamPanic(func() { in.Discard(129) })
			if _, ok := got.(core.Fatal); !ok || in.Base != 0 || in.Control.Used != 128 {
				t.Fatalf("discard interruption=%v base=%d steps=%d", got, in.Base, in.Control.Used)
			}
		})
	}
	for _, phase := range []string{"trail", "values"} {
		t.Run(phase, func(t *testing.T) {
			node := value.Ptr(value.Data{Text: "owned"})
			p := &core.Parser{Input: core.Input{Unit: parse.Bytes, Eof: true, Bs: []byte(strings.Repeat("a", 3000)), BaseLine: 1, BaseCol: 1, Lines: []int{0}, Control: core.NewBudgetControl(nil, parse.Limits{Compute: 953 + 128})}, Memo: core.NewMemoTable()}
			if phase == "trail" {
				p.Trail = make([]core.Undo, 0, 129)
			} else {
				machineOf(p).vals = make([]any, 1, 130)
				machineOf(p).vals[0] = node
				for i := 1; i < cap(machineOf(p).vals); i++ {
					machineOf(p).vals[:cap(machineOf(p).vals)][i] = node
				}
			}
			got := catchStreamPanic(func() { p.Commit(2048) })
			if _, ok := got.(core.Fatal); !ok || p.Base != 2047 || p.Memo.Base != 0 || value.D(node).Text != "owned" {
				t.Fatalf("commit %s interruption=%v base=%d pruned=%d", phase, got, p.Base, p.Memo.Base)
			}
			if phase == "values" && (machineOf(p).vals[0] != node || machineOf(p).vals[:cap(machineOf(p).vals)][129] != node) {
				t.Fatal("clear crossed reservation or live values")
			}
		})
	}
}
