package conform

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The cases of the capture frames (the frame operations). They read the slots of frames with
// ref.FrameSlotOp, since only the compositions that build values read them in the parser VM, and set
// the mark of the missing-token record with ref.FrameMissingOp. A Value handle comes from OprNodeOp, so
// an implementation of the family is also an Operators that makes that handle.

// FrameOps are the frame operations that cases for the reference model cover.
var FrameOps = []string{
	"frame.mark", "frame.markPos", "frame.markRec", "frame.markEnv", "frame.markMissing", "frame.markTrail",
	"frame.undo", "frame.current", "frame.use", "frame.open", "frame.set", "frame.close", "frame.slots",
	"frame.save", "frame.restore",
}

// FrameTestOps are the conformance operations of the family.
var FrameTestOps = []string{ref.FrameSlotOp, ref.FrameMissingOp}

// FrameSupportOps are the operations that cases of the family call to change the environment and the
// recovered errors that backtracking points hold, and to return to them: the implementation provides
// them besides FrameOps and FrameTestOps.
var FrameSupportOps = []string{"env.bind", "env.scope", "env.restore", "rec.push", "rec.truncate"}

// FrameMissingOps are the operations of the family that only a target with the missing-token record
// provides.
var FrameMissingOps = []string{"frame.markMissing", ref.FrameMissingOp}

// FrameSuiteOps returns the operations that a suite of the family provides; without missing, those of a
// target that has no missing-token record.
func FrameSuiteOps(missing bool) []string {
	all := slices.Concat(FrameOps, FrameTestOps, FrameSupportOps)
	if missing {
		return all
	}
	return slices.DeleteFunc(all, func(n string) bool { return slices.Contains(FrameMissingOps, n) })
}

// FrameModule returns the declarations of OprModule with the conformance operations of the family.
func FrameModule() *exec.Module {
	m := OprModule()
	m.AddOp(&exec.Op{Name: ref.FrameSlotOp, Component: ops.Frames, Reads: []string{ops.Frames},
		Params:   []exec.Param{exec.P("f", exec.THandle(ops.Frame)), exec.P("slot", exec.TInt)},
		Results:  []exec.Type{exec.THandle(ops.Value)},
		Contract: `Returns the value in the slot of f, the null value for an empty slot.`,
		Requires: `slot is a slot of f.`})
	m.AddOp(&exec.Op{Name: ref.FrameMissingOp, Component: ops.Rec, Writes: []string{ops.Rec},
		Params:   []exec.Param{exec.P("n", exec.TInt)},
		Contract: `Makes n the mark of the missing-token record, as if it held n inserted tokens.`,
		Requires: `n >= 0.`})
	return m
}

// FrameConfig returns the configuration of the cases: variables for the environment.
func FrameConfig() ref.Config { return ref.Config{Vars: []int64{1, 2, 3}} }

// FrameCases returns the handwritten conformance cases of the capture frames, in one module.
func FrameCases() []Case {
	m := FrameModule()
	var cs []Case
	cfg := FrameConfig()
	add := func(name string, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, []string{"budget", "stream", "document", "missing"}, body), Config: cfg})
	}
	i := exec.Int
	eq := exec.Eq
	node := func(b *exec.Builder, name string) exec.Expr { return b.Let(name, exec.C(OprNodeOp)) }
	slot := func(b *exec.Builder, name string, f exec.Expr, k int64) {
		b.Let(name, exec.C(ref.FrameSlotOp, f, i(k)))
	}
	// reset returns to the backtracking point mk as the reset composition does.
	reset := func(b *exec.Builder, mk exec.Expr) {
		b.SetReg(ops.Pos, exec.C("frame.markPos", mk))
		b.Do("env.restore", exec.C("frame.markEnv", mk))
		b.Do("rec.truncate", exec.C("frame.markRec", mk))
		b.Do("frame.undo", exec.C("frame.markTrail", mk))
	}

	add("frame_mark", func(b *exec.Builder) {
		b.Let("none", exec.C("frame.current"))
		f := b.Let("f", exec.C("frame.open", i(2)))
		b.Do("frame.use", f)
		b.Let("cur", exec.C("frame.current"))
		b.Assert(eq(exec.C("frame.slots", f), i(2)), "a frame has the slots it was opened with")
		v1, v2 := node(b, "v1"), node(b, "v2")
		b.SetReg(ops.Pos, i(4))
		b.Do("env.bind", i(0), exec.C(DatumOp, i(7)))
		b.Do("rec.push", exec.C(ErrOp, i(1)))
		b.Do("frame.set", i(0), v1)
		m0 := b.Let("m0", exec.C("frame.mark"))
		b.Assert(eq(exec.C("frame.markPos", m0), i(4)), "a backtracking point holds the position")
		b.Assert(eq(exec.C("frame.markRec", m0), i(1)), "the recovered errors")
		b.Assert(eq(exec.C("frame.markTrail", m0), i(1)), "and the length of the trail")
		b.Let("e0", exec.C("frame.markEnv", m0))
		b.Let("s0", exec.C("env.scope"))
		b.SetReg(ops.Pos, i(9))
		b.Do("env.bind", i(1), exec.C(DatumOp, i(8)))
		b.Do("rec.push", exec.C(ErrOp, i(2)))
		b.Do("frame.set", i(1), v2)
		b.Do("frame.set", i(0), v2)
		m1 := b.Let("m1", exec.C("frame.mark"))
		b.Assert(eq(exec.C("frame.markPos", m1), i(9)), "a later point holds the later position")
		b.Assert(eq(exec.C("frame.markTrail", m1), i(3)), "and every write since")
		b.Let("e1", exec.C("frame.markEnv", m1))
		slot(b, "a0", f, 0)
		slot(b, "a1", f, 1)
		reset(b, m0)
		slot(b, "b0", f, 0)
		slot(b, "b1", f, 1)
		b.Assert(eq(exec.R(ops.Pos), i(4)), "a reset returns to the position")
		b.Assert(eq(exec.C("frame.markPos", m0), i(4)), "and the point stays as it was")
		b.Let("s1", exec.C("env.scope"))
		m2 := b.Let("m2", exec.C("frame.mark"))
		b.Assert(eq(exec.C("frame.markTrail", m2), i(1)), "the undone writes are dropped from the trail")
		b.Assert(eq(exec.C("frame.markRec", m2), i(1)), "and the errors recovered since")
		reset(b, b.Let("m3", exec.C("frame.mark")))
		slot(b, "c0", f, 0)
	})
	add("frame_missing", func(b *exec.Builder) {
		b.Do(ref.FrameMissingOp, i(3))
		m0 := b.Let("m0", exec.C("frame.mark"))
		b.Do(ref.FrameMissingOp, i(5))
		m1 := b.Let("m1", exec.C("frame.mark"))
		b.Assert(eq(exec.C("frame.markMissing", m0), i(3)), "a backtracking point holds the missing-token mark")
		b.Assert(eq(exec.C("frame.markMissing", m1), i(5)), "as it was when the point was taken")
	})
	add("frame_nested", func(b *exec.Builder) {
		f1, f2 := b.Let("f1", exec.C("frame.open", i(3))), b.Let("f2", exec.C("frame.open", i(2)))
		v1, v2, v3 := node(b, "v1"), node(b, "v2"), node(b, "v3")
		b.Do("frame.use", f1)
		b.Do("frame.set", i(0), v1)
		m0 := b.Let("m0", exec.C("frame.mark"))
		b.Do("frame.use", f2)
		b.Do("frame.set", i(1), v2)
		b.Do("frame.use", f1)
		b.Do("frame.set", i(0), v3)
		b.Do("frame.set", i(2), v2)
		slot(b, "a0", f1, 0)
		slot(b, "a2", f1, 2)
		slot(b, "b1", f2, 1)
		b.Do("frame.undo", exec.C("frame.markTrail", m0))
		slot(b, "c0", f1, 0)
		slot(b, "c1", f1, 1)
		slot(b, "c2", f1, 2)
		slot(b, "d1", f2, 1)
		b.Do("frame.undo", i(0))
		slot(b, "e0", f1, 0)
		b.Do("frame.close", f2)
		b.Let("f3", exec.C("frame.open", i(2)))
		slot(b, "g0", exec.L("f3"), 0)
	})
	add("frame_empty", func(b *exec.Builder) {
		e1, e2 := b.Let("e1", exec.C("frame.open", i(0))), b.Let("e2", exec.C("frame.open", i(0)))
		b.Assert(eq(exec.C("frame.slots", e1), i(0)), "a frame of no slots")
		b.Do("frame.close", e1)
		b.Do("frame.use", e2)
		b.Let("cur", exec.C("frame.current"))
		b.Let("e3", exec.C("frame.open", i(0)))
	})
	add("frame_save_restore", func(b *exec.Builder) {
		f := b.Let("f", exec.C("frame.open", i(3)))
		b.Do("frame.use", f)
		v1, v2 := node(b, "v1"), node(b, "v2")
		m0 := b.Let("m0", exec.C("frame.mark"))
		b.Do("frame.set", i(0), v1)
		b.Do("frame.set", i(2), v2)
		b.Do("frame.set", i(0), v2)
		b.Do("frame.save", f)
		reset(b, m0)
		slot(b, "a0", f, 0)
		slot(b, "a2", f, 2)
		b.Do("frame.restore", f)
		slot(b, "b0", f, 0)
		slot(b, "b1", f, 1)
		slot(b, "b2", f, 2)
		b.Assert(eq(exec.C("frame.markTrail", b.Let("m1", exec.C("frame.mark"))), i(0)), "a restore writes no trail")
		// Another save replaces what the last one set aside.
		g := b.Let("g", exec.C("frame.open", i(1)))
		b.Do("frame.use", g)
		b.Do("frame.set", i(0), v1)
		b.Do("frame.save", g)
		b.Do("frame.undo", i(0))
		b.Do("frame.restore", g)
		slot(b, "c0", g, 0)
	})
	return cs
}

type frameEntry struct {
	expr exec.Expr
	h    any
}

// RandomFrame returns a randomized case of n calls of the frame operations of opNames, drawn with rng,
// that meets every precondition. It returns to backtracking points as the reset composition does,
// and saves and restores frames around a reset as a longest match does.
func RandomFrame(name string, rng *rand.Rand, opNames []string, n int) Case {
	m := FrameModule()
	cfg := FrameConfig()
	mirror := ref.New(cfg)
	regs := map[string]any{ops.Pos: int64(0)}
	mirror.UseRegisters(regs)
	var frames, marks, vals []frameEntry
	f := m.AddFunc(name, nil, nil, []string{"budget", "stream", "document", "missing"}, func(b *exec.Builder) {
		count := 0
		fresh := func() string { count++; return fmt.Sprintf("x%d", count) }
		// call runs the operation on the mirror and in the case; a handle result goes to its pool.
		call := func(opName string, args []any, exprs ...exec.Expr) []any {
			o := m.Op(opName)
			if !mirror.Allows(o, args) {
				panic("conform: a randomized frame case breaks the precondition of " + opName)
			}
			res := mirror.Call(o, args)
			if len(o.Results) == 0 {
				b.Do(opName, exprs...)
				return res
			}
			x := b.Let(fresh(), exec.C(opName, exprs...))
			switch o.Results[0].Handle {
			case ops.Frame:
				if res[0] != nil {
					frames = append(frames, frameEntry{x, res[0]})
				}
			case ops.Backtrack:
				marks = append(marks, frameEntry{x, res[0]})
			}
			return res
		}
		pick := func(es []frameEntry) frameEntry { return es[rng.IntN(len(es))] }
		for range 3 {
			vals = append(vals, frameEntry{b.Let(fresh(), exec.C(OprNodeOp)), mirror.OprHandle(OprNodeOp)})
		}
		vals = append(vals, frameEntry{exec.Null(exec.THandle(ops.Value)), nil})
		markInt := func(op string, mk frameEntry) (int64, exec.Expr) {
			return mirror.Call(m.Op(op), []any{mk.h})[0].(int64), exec.C(op, mk.expr)
		}
		// reset returns to the point mk as the reset composition does, if the errors and the trail
		// have not been cut back before it since.
		reset := func(mk frameEntry) bool {
			rec, recX := markInt("frame.markRec", mk)
			trail, trailX := markInt("frame.markTrail", mk)
			if !mirror.Allows(m.Op("rec.truncate"), []any{rec}) || !mirror.Allows(m.Op("frame.undo"), []any{trail}) {
				return false
			}
			pos, posX := markInt("frame.markPos", mk)
			regs[ops.Pos] = pos
			b.SetReg(ops.Pos, posX)
			env := mirror.Call(m.Op("frame.markEnv"), []any{mk.h})[0]
			call("env.restore", []any{env}, exec.C("frame.markEnv", mk.expr))
			call("rec.truncate", []any{rec}, recX)
			call("frame.undo", []any{trail}, trailX)
			return true
		}
		allows := func(op string, args ...any) bool { return mirror.Allows(m.Op(op), args) }
		for k := 0; count < n && k < 40*n; k++ {
			switch op := opNames[rng.IntN(len(opNames))]; op {
			case "frame.mark":
				pos := rng.Int64N(10)
				regs[ops.Pos] = pos
				b.SetReg(ops.Pos, exec.Int(pos))
				call(op, nil)
			case "frame.markPos", "frame.markRec", "frame.markEnv", "frame.markMissing", "frame.markTrail":
				if len(marks) > 0 {
					mk := pick(marks)
					call(op, []any{mk.h}, mk.expr)
				}
			case "frame.undo":
				if len(marks) == 0 || rng.IntN(4) == 0 {
					call(op, []any{int64(0)}, exec.Int(0))
				} else if mk := pick(marks); rng.IntN(2) == 0 && reset(mk) {
				} else if trail, trailX := markInt("frame.markTrail", mk); allows(op, trail) {
					call(op, []any{trail}, trailX)
				}
			case "frame.current":
				call(op, nil)
			case "frame.use":
				if len(frames) > 0 && rng.IntN(8) > 0 {
					if fr := pick(frames); allows(op, fr.h) {
						call(op, []any{fr.h}, fr.expr)
					}
				} else {
					call(op, []any{nil}, exec.Null(exec.THandle(ops.Frame)))
				}
			case "frame.open":
				k := rng.Int64N(4)
				call(op, []any{k}, exec.Int(k))
			case "frame.set":
				slot, v := rng.Int64N(3), pick(vals)
				if allows(op, slot, v.h) {
					call(op, []any{slot, v.h}, exec.Int(slot), v.expr)
				}
			case "frame.close", "frame.slots", "frame.save":
				if len(frames) > 0 {
					if fr := pick(frames); allows(op, fr.h) {
						call(op, []any{fr.h}, fr.expr)
					}
				}
			case "frame.restore":
				// A longest match saves the frame of a candidate, resets and restores it.
				if len(frames) == 0 || len(marks) == 0 {
					continue
				}
				fr := pick(frames)
				if !allows("frame.save", fr.h) {
					continue
				}
				call("frame.save", []any{fr.h}, fr.expr)
				if rng.IntN(4) > 0 {
					reset(pick(marks))
				}
				call(op, []any{fr.h}, fr.expr)
			case ref.FrameSlotOp:
				if len(frames) > 0 {
					fr, slot := pick(frames), rng.Int64N(3)
					if allows(op, fr.h, slot) {
						call(op, []any{fr.h, slot}, fr.expr, exec.Int(slot))
					}
				}
			case ref.FrameMissingOp:
				k := rng.Int64N(5)
				call(op, []any{k}, exec.Int(k))
			case "env.bind":
				slot, id := rng.Int64N(3), rng.Int64N(6)
				call(op, []any{slot, ref.Datum{ID: id}}, exec.Int(slot), exec.C(DatumOp, exec.Int(id)))
			case "rec.push":
				id := rng.Int64N(20)
				call(op, []any{&ref.Err{ID: id}}, exec.C(ErrOp, exec.Int(id)))
			}
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}

// FrameRandomOps returns the operations that randomized cases of a suite draw: those of FrameOps and the
// conformance and support operations that change what they read, without the missing-token record
// unless missing.
func FrameRandomOps(missing bool) []string {
	names := slices.Concat(FrameOps, FrameTestOps, []string{"env.bind", "rec.push"})
	if missing {
		return names
	}
	return slices.DeleteFunc(names, func(n string) bool { return slices.Contains(FrameMissingOps, n) })
}
