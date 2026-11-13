//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
)

func diagnosticParser(limits core.Limits) *state.Parser {
	return &state.Parser{Expectations: true, Input: state.Input{Control: state.NewBudgetControl(nil, limits)}}
}

func diagnosticHash(id state.ExpID) uint32 {
	return uint32(id) * 0x9e3779b1 >> 26
}

func collidingDiagnosticIDs(n int) []state.ExpID {
	buckets := make(map[uint32][]state.ExpID)
	for id := state.ExpID(0); id < 1<<20; id++ {
		b := diagnosticHash(id)
		buckets[b] = append(buckets[b], id)
		if len(buckets[b]) == n {
			return buckets[b]
		}
	}
	panic("could not find deterministic expectation hash collision")
}

func uniqueDiagnosticIDs(n int) []state.ExpID {
	seen := make(map[uint32]bool)
	ids := make([]state.ExpID, 0, n)
	for id := state.ExpID(0); id < 1<<20 && len(ids) < n; id++ {
		b := diagnosticHash(id)
		if !seen[b] {
			seen[b] = true
			ids = append(ids, id)
		}
	}
	if len(ids) != n {
		panic("could not find unique expectation hash buckets")
	}
	return ids
}

func TestBudgetExpectationCollisionScanBudgetsAndShortCircuits(t *testing.T) {
	t.Parallel()
	ids := collidingDiagnosticIDs(16)
	bit := uint64(1) << diagnosticHash(ids[0])
	prefix := append([]state.ExpID(nil), ids[:len(ids)-1]...)
	p := diagnosticParser(core.Limits{Compute: uint64(len(prefix))})
	p.Farthest, p.ExpBits, p.Exp = 7, bit, append([]state.ExpID(nil), prefix...)
	p.Expect(7, ids[len(ids)-1])
	if len(p.Exp) != len(ids) || p.Exp[len(p.Exp)-1] != ids[len(ids)-1] || p.Control.Used != uint64(len(prefix)) {
		t.Fatalf("late colliding expectation result=%v steps=%d", p.Exp, p.Control.Used)
	}

	p = diagnosticParser(core.Limits{Compute: uint64(len(ids))})
	p.Farthest, p.ExpBits, p.Exp = 7, bit, append([]state.ExpID(nil), ids...)
	p.Expect(7, ids[len(ids)-1])
	if len(p.Exp) != len(ids) || p.Control.Used != uint64(len(ids)) {
		t.Fatalf("late duplicate scan changed record or steps: len=%d steps=%d", len(p.Exp), p.Control.Used)
	}

	p = diagnosticParser(core.Limits{Compute: uint64(len(prefix) - 1)})
	p.Farthest, p.ExpBits, p.Exp = 7, bit, append([]state.ExpID(nil), prefix...)
	before := append([]state.ExpID(nil), p.Exp...)
	if err := catchFatal(t, func() { p.Expect(7, ids[len(ids)-1]) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("one-short collision scan error = %v, want ErrLimit", err)
	}
	if !reflect.DeepEqual(p.Exp, before) || p.Control.Used != uint64(len(prefix)-1) {
		t.Fatalf("failed expectation scan mutated record or used %d steps", p.Control.Used)
	}

	p = diagnosticParser(core.Limits{Compute: 4})
	p.Farthest, p.Exp, p.Silent = 12, []state.ExpID{1, 2, 3}, 1
	p.Expect(12, 4)
	p.Silent = 0
	p.Expect(11, 5)
	if p.Control.Used != 0 || len(p.Exp) != 3 {
		t.Fatalf("silent/older expectation was not a no-op: steps=%d exp=%v", p.Control.Used, p.Exp)
	}
}

func TestBudgetExpectationKeepSnapshotsAndAtomicCopy(t *testing.T) {
	t.Parallel()
	ids := make([]state.ExpID, 300)
	for i := range ids {
		ids[i] = state.ExpID(i)
	}
	p := diagnosticParser(core.Limits{Compute: 100_000})
	first := p.Keep(ids)
	if len(first) != len(ids) || cap(first) != len(first) {
		t.Fatalf("kept snapshot len/cap=%d/%d", len(first), cap(first))
	}
	if again := p.Keep(ids); len(again) == 0 || &again[0] != &first[0] {
		t.Fatal("equal expectation sequence did not preserve snapshot identity")
	}
	if got := p.Keep(nil); got != nil {
		t.Fatalf("empty keep = %#v, want nil", got)
	}

	equalParser := diagnosticParser(core.Limits{Compute: 299})
	equalParser.Arena = append([]state.ExpID(nil), first...)
	equalParser.LastKept = equalParser.Arena[:len(ids):len(ids)]
	oldLast := &equalParser.LastKept[0]
	if err := catchFatal(t, func() { equalParser.Keep(ids) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("one-short keep equality error = %v, want ErrLimit", err)
	}
	if &equalParser.LastKept[0] != oldLast || len(equalParser.LastKept) != len(ids) {
		t.Fatal("interrupted equality changed the published lastKept view")
	}

	changed := append([]state.ExpID(nil), ids...)
	for i := range changed {
		changed[i] += 1000
	}
	copyParser := diagnosticParser(core.Limits{Compute: 129})
	copyParser.Arena = make([]state.ExpID, len(first), 1024)
	copy(copyParser.Arena, first)
	copyParser.LastKept = copyParser.Arena[:len(first):len(first)]
	priorAlias := copyParser.LastKept
	oldSnapshot := append([]state.ExpID(nil), copyParser.LastKept...)
	if err := catchFatal(t, func() { copyParser.Keep(changed) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("partial expectation copy error = %v, want ErrLimit", err)
	}
	if len(copyParser.Arena) != len(first) || !reflect.DeepEqual(copyParser.LastKept, oldSnapshot) {
		t.Fatal("interrupted copy published partial arena or changed old snapshot")
	}
	copyParser.Control = state.NewBudgetControl(nil, core.Limits{Compute: 301})
	complete := copyParser.Keep(changed)
	if len(complete) != len(changed) || cap(complete) != len(complete) || !reflect.DeepEqual(copyParser.LastKept, changed) || copyParser.Control.Used != 301 {
		t.Fatal("exact-budget retry did not publish exact-capacity changed snapshot")
	}
	if len(priorAlias) != len(oldSnapshot) || cap(priorAlias) != len(oldSnapshot) || !reflect.DeepEqual(priorAlias, oldSnapshot) {
		t.Fatal("successful retry changed the retained prior snapshot alias")
	}

	rollover := make([]state.ExpID, 500)
	for i := range rollover {
		rollover[i] = state.ExpID(5000 + i)
	}
	copyParser.Control = state.NewBudgetControl(nil, core.Limits{Compute: 100_000})
	rolled := copyParser.Keep(rollover)
	if len(rolled) != len(rollover) || cap(rolled) != len(rolled) || !reflect.DeepEqual(priorAlias, oldSnapshot) {
		t.Fatal("arena rollover changed the retained exact-capacity snapshot")
	}
}

// The filter of the expectations holds only the current record: a record that starts anew forgets
// the filter of the last one, and the scan a collision starts ends at the base of the record.
func TestBudgetExpectationFilterFollowsTheRecord(t *testing.T) {
	t.Parallel()
	ids := collidingDiagnosticIDs(2)
	a, c := ids[0], ids[1]
	b := state.ExpID(0)
	for diagnosticHash(b) == diagnosticHash(a) {
		b++
	}

	p := diagnosticParser(core.Limits{Compute: 100})
	p.Expect(1, a)
	p.Expect(1, a)
	if !reflect.DeepEqual(p.Exp, []state.ExpID{a}) {
		t.Fatalf("a repeated expectation joined the record: %v", p.Exp)
	}
	used := p.Control.Used
	p.Expect(2, b)
	p.Expect(2, c)
	if !reflect.DeepEqual(p.Exp, []state.ExpID{b, c}) || p.Control.Used != used {
		t.Fatalf("a new record scanned for the last one's filter: exp=%v steps=%d, want %d", p.Exp, p.Control.Used, used)
	}

	inner := diagnosticParser(core.Limits{Compute: 100})
	inner.Farthest, inner.Exp, inner.ExpBase, inner.ExpBits = 3, []state.ExpID{a}, 1, 1<<diagnosticHash(a)
	inner.Expect(3, a)
	if !reflect.DeepEqual(inner.Exp, []state.ExpID{a, a}) || inner.Control.Used != 0 {
		t.Fatalf("the scan read the enclosing record: exp=%v steps=%d", inner.Exp, inner.Control.Used)
	}
}

// Without expectations a merge carries only the position: one placeholder, under the same rules as
// the expectations it stands for.
func TestBudgetMergeExpectedWithoutExpectations(t *testing.T) {
	t.Parallel()
	s := []state.ExpID{1, 2}
	for _, c := range []struct {
		name         string
		far, silent  int
		wantFar      int
		wantExpected []state.ExpID
	}{
		{name: "before the farthest position", far: 4, wantFar: 5},
		{name: "in a lookahead", far: 6, silent: 1, wantFar: 5},
		{name: "at the farthest position", far: 5, wantFar: 5, wantExpected: []state.ExpID{state.ExpFar}},
		{name: "past the farthest position", far: 6, wantFar: 6, wantExpected: []state.ExpID{state.ExpFar}},
	} {
		p := diagnosticParser(core.Limits{Compute: 100})
		p.Expectations, p.Farthest, p.Silent = false, 5, c.silent
		p.MergeExpected(c.far, s)
		if p.Farthest != c.wantFar || !slices.Equal(p.Exp, c.wantExpected) || p.Control.Used != uint64(len(s)) {
			t.Errorf("%s: far=%d exp=%v steps=%d", c.name, p.Farthest, p.Exp, p.Control.Used)
		}
	}
}

// Kept sets fill chunks of the arena: a set that does not fit the room left starts a chunk, of the
// size of the set when it is larger than a chunk, and every chunk is charged once.
func TestBudgetExpectationKeepFillsChunks(t *testing.T) {
	t.Parallel()
	const size = uint64(unsafe.Sizeof(state.ExpID(0)))
	ids := make([]state.ExpID, 3000)
	for i := range ids {
		ids[i] = state.ExpID(i)
	}

	p := diagnosticParser(core.Limits{Compute: 100_000})
	first := p.Keep(ids[:1000])
	if p.MemUsed != 1024*size {
		t.Fatalf("the first chunk charged %d bytes, want %d", p.MemUsed, 1024*size)
	}
	second := p.Keep(ids[1000:1100])
	if p.MemUsed != 2*1024*size || len(p.Arena) != 100 {
		t.Fatalf("a set past the room left: charged %d bytes, %d used in the chunk", p.MemUsed, len(p.Arena))
	}
	if !slices.Equal(first, ids[:1000]) || !slices.Equal(second, ids[1000:1100]) {
		t.Fatal("a new chunk changed the kept sets")
	}

	large := diagnosticParser(core.Limits{Compute: 100_000})
	large.Keep(ids[:2000])
	if large.MemUsed != 2000*size || cap(large.Arena) != 2000 {
		t.Fatalf("a set larger than a chunk: charged %d bytes, chunk of %d", large.MemUsed, cap(large.Arena))
	}

	prefix := diagnosticParser(core.Limits{Compute: 100})
	prefix.Keep(ids[:3])
	if got := prefix.Keep(ids[:2]); !slices.Equal(got, ids[:2]) {
		t.Fatalf("a prefix of the set kept last kept %v", got)
	}
}

func TestBudgetMergeExpectedForwardAliasAndBudgets(t *testing.T) {
	t.Parallel()
	ids := uniqueDiagnosticIDs(12)
	stack := make([]state.ExpID, 2+2+len(ids), 2+2+2*len(ids))
	stack[0], stack[1], stack[2], stack[3] = 90, 91, 92, 93
	copy(stack[4:], ids)
	p := diagnosticParser(core.Limits{Compute: uint64(len(ids))})
	p.Exp, p.ExpBase, p.Farthest = stack, 2, 1
	aliased := p.Exp[4:]
	p.MergeExpected(5, aliased)
	if p.Farthest != 5 || len(p.Exp) != 2+len(ids) || !reflect.DeepEqual(p.Exp[:2], []state.ExpID{90, 91}) || !reflect.DeepEqual(p.Exp[2:], ids) {
		t.Fatalf("forward aliased merge state: far=%d exp=%v", p.Farthest, p.Exp)
	}
	if p.Control.Used != uint64(len(ids)) {
		t.Fatalf("merge charged %d steps, want one per visited ID", p.Control.Used)
	}

	ignored := diagnosticParser(core.Limits{Compute: uint64(len(ids))})
	ignored.Exp, ignored.ExpBase, ignored.Farthest, ignored.Silent = []state.ExpID{7, 8}, 1, 5, 1
	ignored.MergeExpected(5, ids)
	if ignored.Control.Used != uint64(len(ids)) || !reflect.DeepEqual(ignored.Exp, []state.ExpID{7, 8}) {
		t.Fatalf("silent merge steps/state = %d/%v", ignored.Control.Used, ignored.Exp)
	}

	canceled := diagnosticParser(core.Limits{})
	canceled.Control = state.NewBudgetControl(newPollingContext(1), core.Limits{})
	canceled.Exp, canceled.ExpBase, canceled.Farthest, canceled.Silent = []state.ExpID{7, 8}, 1, 5, 1
	longIDs := make([]state.ExpID, 300)
	for i := range longIDs {
		longIDs[i] = state.ExpID(i)
	}
	if err := catchFatal(t, func() { canceled.MergeExpected(5, longIDs) }); !errors.Is(err, context.Canceled) {
		t.Fatalf("polling cancellation in ignored merge = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(canceled.Exp, []state.ExpID{7, 8}) {
		t.Fatal("silent interrupted merge changed the active record")
	}
}

func diagnosticDescriptions() []string {
	return []string{"zeta", "\xff", "alpha", "\xc3\xff", "é", "alpha", "message-0", "message-1"}
}

func TestBudgetMakeErrorByteSortAndMessageOrder(t *testing.T) {
	t.Parallel()
	p := diagnosticParser(core.Limits{Compute: 100_000})
	p.Descs = diagnosticDescriptions()
	ids := []state.ExpID{0, state.MsgBit | 7, 1, 2, state.MsgBit | 6, 3, 4, 5}
	first := p.MakeError(0, ids)
	wantExpected := []string{"alpha", "alpha", "zeta", "é", "\xc3\xff", "\xff"}
	wantMessages := []string{"message-1", "message-0"}
	if !reflect.DeepEqual(first.Expected, wantExpected) || !reflect.DeepEqual(first.Messages, wantMessages) {
		t.Fatalf("diagnostic order Expected=%q Messages=%q", first.Expected, first.Messages)
	}
	firstExpected := append([]string(nil), first.Expected...)
	firstMessages := append([]string(nil), first.Messages...)

	long := strings.Repeat("prefix-", 512)
	p.Descs = []string{long + "z", long + "a", long + "m", "later-message"}
	second := p.MakeError(0, []state.ExpID{0, 1, state.MsgBit | 3, 2})
	if !reflect.DeepEqual(second.Expected, []string{long + "a", long + "m", long + "z"}) || !reflect.DeepEqual(second.Messages, []string{"later-message"}) {
		t.Fatal("long-prefix diagnostic sort or message order changed")
	}
	if !reflect.DeepEqual(first.Expected, firstExpected) || !reflect.DeepEqual(first.Messages, firstMessages) {
		t.Fatal("sorting a later diagnostic mutated the retained earlier error")
	}
}

func TestBudgetMakeErrorInterruptionDuringConstructionAndSort(t *testing.T) {
	t.Parallel()
	const n = 300
	descs := make([]string, n)
	ids := make([]state.ExpID, n)
	for i := range descs {
		descs[i] = strings.Repeat("common-prefix-", 64) + string(rune('a'+i%26)) + strings.Repeat("x", i%13)
		ids[i] = state.ExpID(i)
	}
	p := diagnosticParser(core.Limits{Compute: n - 1})
	p.Descs = descs
	if err := catchFatal(t, func() { _ = p.MakeError(0, ids) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("description construction error = %v, want ErrLimit", err)
	}
	if p.Control.Used != n-1 {
		t.Fatalf("description interruption used %d steps, want %d", p.Control.Used, n-1)
	}

	// Permit every description visit, then stop within the controlled sort of long-prefix strings.
	p = diagnosticParser(core.Limits{Compute: n + 512})
	p.Descs = descs
	if err := catchFatal(t, func() { _ = p.MakeError(0, ids) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("sort interruption error = %v, want ErrLimit", err)
	}
	if p.Control.Used <= n || p.Control.Used > n+512 {
		t.Fatalf("sort interruption steps = %d", p.Control.Used)
	}
}
