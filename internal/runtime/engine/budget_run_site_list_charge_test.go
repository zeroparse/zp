//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"slices"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/runtime/state"
)

// computeSweep returns, for a parse of input at each compute limit from 1 up, the distinct sizes of the
// reservations of more than one unit that fail, in the order of the limits, and the least limit at which
// the parse succeeds: the units it charges.
func computeSweep(t *testing.T, prog *Program, input string, b Backend) (sizes []uint64, total uint64) {
	t.Helper()
	for l := uint64(1); l < 20000; l++ {
		_, err := prog.ParseWith("main", input, ParseOptions{Backend: b, Limits: core.Limits{Compute: l}})
		var le *core.LimitError
		if err == nil || !errors.As(err, &le) {
			if err != nil {
				t.Fatalf("limit %d: %v", l, err)
			}
			return sizes, l
		}
		if le.Requested > 1 && (len(sizes) == 0 || sizes[len(sizes)-1] != le.Requested) {
			sizes = append(sizes, le.Requested)
		}
	}
	t.Fatal("the parse does not succeed within 20000 units")
	return nil, 0
}

// TestRunSiteListTakeOffCharge pins what a hot run site charges for the list of its walk: the take-off of
// the n values that the walk gathered is one reservation of 2n units, on the closure and both bytecode
// backends alike, and the walk's copy of them into its record is not charged again. At a hot span of one
// the restarted lists of the grammar turn hot, and the input has walks of 7 and 6 values, so that the
// reservations of 14 and 12 units must appear, and the total of units that each backend charges is fixed.
func TestRunSiteListTakeOffCharge(t *testing.T) {
	defer func(n, m int) { state.RunHotSpan, state.ScanHotSpan = n, m }(state.RunHotSpan, state.ScanHotSpan)
	state.RunHotSpan, state.ScanHotSpan = 1, 1
	prog := mustCompile(t, `rule main = (a "y" / "q" a / [x])* $$
rule a = x+
rule x = "x"`)
	const input = "xxxxxxxxx"
	for _, c := range []struct {
		backend Backend
		total   uint64
	}{{Closure, 415}, {Bytecode, 433}, {BytecodeIterative, 423}} {
		t.Run(c.backend.String(), func(t *testing.T) {
			sizes, total := computeSweep(t, prog, input, c.backend)
			for _, want := range []uint64{14, 12} {
				if !slices.Contains(sizes, want) {
					t.Errorf("no reservation of %d units for a list take-off; the reservations are %v", want, sizes)
				}
			}
			if total != c.total {
				t.Errorf("the parse charges %d units, want %d", total, c.total)
			}
		})
	}
}

// TestFailedRepetitionDropsItsElements pins that a repetition that fails below its minimum makes no list: its
// three gathered elements are dropped, which the closure backend charges as three units, and are not taken off
// with a reservation of six, as a list is. The only take-off of the parse is the list of the second
// alternative, which every engine backend reserves after the same units.
func TestFailedRepetitionDropsItsElements(t *testing.T) {
	prog := mustCompile(t, "@start\nrule main = [a-z]#repeat(5,) / [a-z]*")
	reservations := func(b Backend) (used []uint64) {
		for l := uint64(1); l < 200; l++ {
			_, err := prog.ParseWith("main", "abc", ParseOptions{Backend: b, Limits: core.Limits{Compute: l}})
			var le *core.LimitError
			if errors.As(err, &le) && le.Requested == 6 && !slices.Contains(used, le.Used) {
				used = append(used, le.Used)
			}
		}
		return used
	}
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(b.String(), func(t *testing.T) {
			if got := reservations(b); !slices.Equal(got, []uint64{27}) {
				t.Errorf("reservations of six units after %v units, want one after 27: the take-off of the second alternative's list only", got)
			}
		})
	}
	// The closure backend drops the three elements of the failed repetition as units of a run; the
	// bytecode backends, generated Go and TypeScript charge nothing for the drop. That difference is the
	// open one of the drop charge of a failed alternative (#7).
	var dropped bool
	for l := uint64(1); l < 200 && !dropped; l++ {
		_, err := prog.ParseWith("main", "abc", ParseOptions{Backend: Closure, Limits: core.Limits{Compute: l}})
		var le *core.LimitError
		dropped = errors.As(err, &le) && le.Used == 16 && le.Requested == 3
	}
	if !dropped {
		t.Error("the closure backend does not drop the elements of the failed repetition as three units after 16")
	}
}
