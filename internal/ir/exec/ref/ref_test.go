package ref

import (
	"errors"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// The model is the executable contract; these tests pin the behaviors its contract text states.

var pilot = ops.Pilot()

func call(m *Model, name string, args ...any) []any {
	return m.Call(pilot.Op(name), args)
}

func TestMemoModel(t *testing.T) {
	m := New(Config{})
	k := []any{int64(1), int64(5), int64(0), int64(7)}
	if r := call(m, "memo.lookup", k...); r[1].(bool) {
		t.Fatal("empty memo found an entry")
	}
	e := call(m, "memo.store", append(k, true, int64(9), false, nil, nil)...)[0]
	if r := call(m, "memo.lookup", k...); r[0] != e || !r[1].(bool) {
		t.Fatal("lookup after store")
	}
	other := []any{int64(1), int64(5), int64(0), int64(8)}
	if r := call(m, "memo.lookup", other...); r[1].(bool) {
		t.Fatal("other values found the entry")
	}
	e2 := call(m, "memo.store", append(other, false, int64(5), true, nil, nil)...)[0]
	if r := call(m, "memo.lookup", other...); r[0] != e2 || call(m, "memo.ok", e2)[0].(bool) || !call(m, "memo.silent", e2)[0].(bool) {
		t.Fatal("lookup under other values")
	}
	if r := call(m, "memo.lookup", k...); r[0] != e {
		t.Fatal("a store under other values replaced the entry")
	}
	if got := m.Quantity(ops.QKeyScan, nil, k); got != 4 {
		t.Fatalf("key scan at 5 = %d, want 4 (two entries of one value)", got)
	}
	// A store under the key of e replaces it: e is dead, and an implementation may reuse it.
	if m.Dead(e) {
		t.Fatal("a live entry is dead")
	}
	e3 := call(m, "memo.store", append(k, false, int64(5), false, nil, nil)...)[0]
	if !m.Dead(e) || m.Dead(e3) || m.Allows(pilot.Op("memo.ok"), []any{e}) {
		t.Fatal("the replaced entry is not dead")
	}
	if got := m.Quantity(ops.QKeyScan, nil, k); got != 4 {
		t.Fatalf("key scan at 5 after a replacement = %d, want 4", got)
	}
}

func TestSlotGrowth(t *testing.T) {
	m := New(Config{})
	store := func(pos int64) int64 {
		args := []any{int64(0), pos, int64(0), int64(0), true, pos, false, nil, nil}
		q := m.Quantity(ops.QSlotGrowth, nil, args)
		call(m, "memo.store", args...)
		return q
	}
	for _, c := range []struct{ pos, want int64 }{{0, 0}, {1, 1}, {2, 0}, {3, 3}, {7, 4}, {7, 0}, {8, 0}, {15, 9}} {
		if got := store(c.pos); got != c.want {
			t.Errorf("growth storing at %d = %d, want %d", c.pos, got, c.want)
		}
	}
}

func TestSeenModel(t *testing.T) {
	m := New(Config{Rules: 2})
	if !call(m, "seen.first", int64(0), int64(3))[0].(bool) || call(m, "seen.first", int64(0), int64(3))[0].(bool) {
		t.Fatal("first and repeated call")
	}
	// One repeat in two calls exceeds 2/16: the rule is eager and records nothing more.
	if call(m, "seen.first", int64(0), int64(4))[0].(bool) {
		t.Fatal("eager rule deferred a call")
	}
	if !call(m, "seen.first", int64(1), int64(4))[0].(bool) {
		t.Fatal("rules share counts")
	}
}

func TestBudgetAndDepthModels(t *testing.T) {
	m := New(Config{Limit: 5, MaxDepth: 1})
	call(m, "budget.charge", uint64(5))
	func() {
		defer func() {
			if x := recover(); !errors.As(asErr(x), new(exec.Interrupt)) {
				t.Fatalf("charge beyond the limit: %v", x)
			}
		}()
		call(m, "budget.charge", uint64(1))
	}()
	if call(m, "budget.used")[0] != uint64(5) {
		t.Fatal("an interrupted charge changed the units used")
	}
	call(m, "depth.enter")
	func() {
		defer func() {
			if _, ok := recover().(exec.Interrupt); !ok {
				t.Fatal("depth beyond the limit did not interrupt")
			}
		}()
		call(m, "depth.enter")
	}()
}

func asErr(x any) error {
	err, _ := x.(error)
	return err
}

// TestEmptyExaminedRecord checks that a record of an empty examined input at the entry's position,
// at position 0 too, is a record: an edit after it keeps the entry where it is.
func TestEmptyExaminedRecord(t *testing.T) {
	m := New(Config{})
	k := []any{int64(1), int64(0), int64(0), int64(7)}
	e := call(m, "memo.store", append(k, true, int64(0), false, nil, nil)...)[0]
	args := []any{e, false, int64(0)}
	if !m.Allows(pilot.Op("memo.examine"), args) {
		t.Fatal("an empty examined input at the entry's position is not allowed")
	}
	call(m, "memo.examine", args...)
	call(m, "memo.splice", int64(2), int64(3), int64(1))
	if r := call(m, "memo.lookup", k...); !r[1].(bool) {
		t.Error("an edit after the empty examined input removed the entry")
	}
}

// TestCaptureFields pins ops.QCaptureFields: the search of the captures counts once more when one
// matched, and each capture walks the fields of those before it, comparing names of the same length
// two units a byte.
func TestCaptureFields(t *testing.T) {
	m := New(Config{RuleSpecs: []RuleSpec{{Captures: 3, Body: []Eval{
		{OK: true, Captures: []int64{0, 2}}, {OK: true, Captures: []int64{2, 1, 0}}, {OK: true}, {Captures: []int64{1}},
	}}}, RuleVars: make([][]int64, 1)})
	m.UseRegisters(map[string]any{ops.Pos: int64(0), ops.Silent: false})
	full := ops.Pilot()
	ops.Transitional(full)
	invoke := full.Op("rule.invoke")
	args := []any{RuleRef{Index: 0}, int64(0)}
	// c2 after c0: 3 + (1 + 4); then c0, c1, c2: 3 + (1 + 4) + 2*(1 + 4); then none, and a failure.
	for k, want := range []int64{8, 18, 0, 0} {
		if got := m.Quantity(ops.QCaptureFields, invoke, args); got != want {
			t.Errorf("evaluation %d: capture_fields = %d, want %d", k, got, want)
		}
		m.Call(invoke, args)
	}
	if got := textCompare("c10", "c11"); got != 6 {
		t.Errorf("comparing two names of 3 bytes charges %d, want 6", got)
	}
	if got := textCompare("c1", "c10"); got != 0 {
		t.Errorf("comparing names of different lengths charges %d, want 0", got)
	}
}
