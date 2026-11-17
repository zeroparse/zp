package conform

import (
	"fmt"
	"math/rand/v2"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec"
	"zeroparse.dev/zp/internal/ir/code/exec/ref"
)

// MissingInsertOp inserts a missing token at a site and a position, as the grammar's code of a
// missing-token literal does: it records the token and its error, once for each site and position, and
// returns whether the token was new. It is not an operation of the parser VM.
const MissingInsertOp = ref.MissingInsertOp

// MissingSeedOp records the tokens inserted after a mark as those that the call of an entry inserted,
// as the end of a call does. It is not an operation of the parser VM.
const MissingSeedOp = ref.MissingSeedOp

// MissingOps are the operations of the missing-token mode, with those its cases need to make the tokens
// and the entries of calls and the errors they recovered.
var MissingOps = []string{"missing.active", "missing.recompute", "missing.mark", "missing.reset", "missing.replay",
	MissingInsertOp, MissingSeedOp, "rec.mark", "rec.push", "rec.truncate", "rec.since", "memo.store"}

// MissingRules is the number of rules that the cases of the missing-token mode name.
const MissingRules = 3

// MissingConfig returns the configuration of a parse of the cases of the missing-token mode: a parse in
// the mode if missing, in which the first and the third rule can insert a token.
func MissingConfig(missing bool) ref.Config {
	return ref.Config{Vars: []int64{0}, RuleVars: make([][]int64, MissingRules), Rules: MissingRules,
		Missing: missing, MissingRules: []bool{true, false, true}}
}

// MissingModule returns the declarations of Module with MissingInsertOp and MissingSeedOp.
func MissingModule() *code.Module {
	m := Module()
	m.AddOp(&code.Op{Name: MissingInsertOp, Component: exec.Rec,
		Params:  []code.Param{code.P("site", code.TInt), code.P("pos", code.TInt), code.P("e", code.THandle(exec.Err))},
		Results: []code.Type{code.TBool}, Writes: []string{exec.Rec}, Features: []string{exec.Missing},
		Contract: "Inserts a missing token at the site and the position with the error e, unless one is there."})
	m.AddOp(&code.Op{Name: MissingSeedOp, Component: exec.Rec,
		Params: []code.Param{code.P("e", code.THandle(exec.Entry)), code.P("mark", code.TInt)}, Writes: []string{exec.Rec},
		Features: []string{exec.Missing},
		Contract: "Records the tokens inserted after the mark as those of the call of the entry e."})
	return m
}

var missingFeatures = []string{"budget", "stream", "document", "missing"}

// missingStore stores an entry whose call recovered the errors since the mark of rec.mark, at a position
// that no other entry of the case has.
func missingStore(b *code.Builder, name string, pos int64, errs code.Expr) code.Expr {
	return b.Let(name, code.C("memo.store", code.Int(0), code.Int(pos), code.Int(0), code.C(ValuesOp, code.Int(0)),
		code.BoolC(true), code.Int(pos+1), code.BoolC(false), noValue, errs, noDocOnly))
}

// MissingCases returns the handwritten conformance cases of the missing-token mode, in one module. As
// with Cases, the model checks each case first.
func MissingCases() []Case {
	m := MissingModule()
	var cs []Case
	add := func(name string, missing bool, body func(b *code.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, missingFeatures, body), Config: MissingConfig(missing)})
	}
	rule := func(i int64) code.Expr { return code.C(RuleOp, code.Int(i)) }
	insert := func(b *code.Builder, site, pos, err int64) code.Expr {
		return code.C(MissingInsertOp, code.Int(site), code.Int(pos), code.C(ErrOp, code.Int(err)))
	}
	mark := code.C("missing.mark")

	add("missing_off", false, func(b *code.Builder) {
		b.Assert(code.Not(code.C("missing.active")), "an ordinary parse is not in the mode")
		b.Assert(code.Not(code.C("missing.recompute", rule(0))), "and recomputes no result")
		b.Assert(code.Eq(mark, code.Int(0)), "its mark of the tokens is 0")
		b.Do("missing.reset", code.Int(0))
		b.Assert(code.Eq(mark, code.Int(0)), "and resetting it does nothing")
	})

	add("missing_rules", true, func(b *code.Builder) {
		b.Assert(code.C("missing.active"), "a parse in the mode is in it")
		b.Assert(code.C("missing.recompute", rule(0)), "a rule that can insert a token is recomputed")
		b.Assert(code.Not(code.C("missing.recompute", rule(1))), "a rule that cannot is not")
		b.Assert(code.C("missing.recompute", rule(2)), "also another that can")
	})

	add("missing_tokens", true, func(b *code.Builder) {
		b.Assert(code.Eq(mark, code.Int(0)), "a parse starts with no token")
		b.Assert(insert(b, 1, 5, 11), "a token is new at its site and position")
		b.Assert(code.Not(insert(b, 1, 5, 12)), "a token at the same site and position is not")
		b.Assert(insert(b, 2, 5, 13), "the same position with another site is new")
		b.Assert(insert(b, 1, 6, 14), "and the same site at another position")
		b.Assert(code.Eq(mark, code.Int(3)), "the mark counts the tokens inserted")
		b.Do("missing.reset", code.Int(1))
		b.Assert(code.Eq(mark, code.Int(1)), "resetting removes the tokens after the mark")
		b.Assert(insert(b, 2, 5, 15), "a removed token may be inserted again")
		b.Assert(code.Not(insert(b, 1, 5, 16)), "one that stays may not")
		b.Do("missing.reset", code.Int(2))
		b.Do("missing.reset", code.Int(2))
		b.Assert(code.Eq(mark, code.Int(2)), "resetting to the mark that holds removes nothing")
		b.Do("missing.reset", code.Int(0))
		b.Assert(code.Eq(mark, code.Int(0)), "and to the start removes them all")
	})

	add("missing_replay", true, func(b *code.Builder) {
		// The end of a call that inserted two tokens: its entry keeps them with the errors it recovered.
		b.Let("mm", mark)
		b.Let("rm", code.C("rec.mark"))
		b.Assert(insert(b, 1, 2, 21), "a call inserts a token")
		b.Assert(insert(b, 2, 3, 22), "and another")
		b.Let("errs", code.C("rec.since", code.L("rm")))
		missingStore(b, "e", 2, code.L("errs"))
		b.Do(MissingSeedOp, code.L("e"), code.L("mm"))
		// The call fails and its tokens go; a later call finds the entry and replays it.
		b.Do("missing.reset", code.L("mm"))
		b.Do("rec.truncate", code.L("rm"))
		b.Assert(code.Eq(mark, code.Int(0)), "the failed call leaves no token")
		b.Do("missing.replay", code.L("e"))
		b.Assert(code.Eq(mark, code.Int(2)), "replaying the entry inserts its tokens again")
		b.Assert(code.Not(insert(b, 1, 2, 23)), "so that they are there")
		// A second replay finds the tokens present: none is inserted or recorded twice.
		b.Do("missing.replay", code.L("e"))
		b.Assert(code.Eq(mark, code.Int(2)), "a replay of an entry whose tokens are present inserts none")
		// Only the second token is gone: its error returns, the first token's does not.
		b.Do("missing.reset", code.Int(1))
		b.Do("missing.replay", code.L("e"))
		b.Assert(code.Eq(mark, code.Int(2)), "a replay inserts the tokens that are missing")
	})

	add("missing_replay_no_tokens", true, func(b *code.Builder) {
		// An entry whose call inserted no token, with errors that another recovery made, and one with
		// neither.
		b.Let("mm", mark)
		b.Let("rm", code.C("rec.mark"))
		b.Do("rec.push", code.C(ErrOp, code.Int(31)))
		b.Let("errs", code.C("rec.since", code.L("rm")))
		missingStore(b, "e", 1, code.L("errs"))
		b.Do(MissingSeedOp, code.L("e"), code.L("mm"))
		missingStore(b, "f", 2, noErrs)
		b.Do(MissingSeedOp, code.L("f"), code.L("mm"))
		b.Do("missing.replay", code.L("e"))
		b.Do("missing.replay", code.L("f"))
		b.Assert(code.Eq(mark, code.Int(0)), "no token comes back")
	})
	return cs
}

// RandomMissing returns a randomized case of n calls of the operations of the missing-token mode,
// drawn with rng, that meets every precondition. Sites and positions come from small domains, so that
// tokens repeat and entries replay tokens that are present. A parse outside the mode draws the same
// operations; the model refuses those that need the mode.
func RandomMissing(name string, rng *rand.Rand, n int, missing bool) Case {
	m := MissingModule()
	cfg := MissingConfig(missing)
	mirror := ref.New(cfg)
	type entry struct {
		name string
		ref  *ref.Entry
	}
	var entries []entry
	f := m.AddFunc(name, nil, nil, missingFeatures, func(b *code.Builder) {
		pos := int64(0)
		marks := func() int64 { return mirror.Call(m.Op("missing.mark"), nil)[0].(int64) }
		for calls, tries := 0, 0; calls < n && tries < 50*n; tries++ {
			var o *code.Op
			var args []any
			var exprs []code.Expr
			intArg := func(v int64) { args = append(args, v); exprs = append(exprs, code.Int(v)) }
			v := fmt.Sprintf("x%d", calls)
			switch rng.IntN(10) {
			case 0:
				o = m.Op("missing.active")
			case 1:
				o = m.Op("missing.recompute")
				i := rng.Int64N(MissingRules)
				args, exprs = append(args, ref.RuleRef{Index: i}), append(exprs, code.C(RuleOp, code.Int(i)))
			case 2:
				o = m.Op("missing.mark")
			case 3:
				o = m.Op("missing.reset")
				intArg(rng.Int64N(marks() + 1))
			case 4, 5:
				o = m.Op(MissingInsertOp)
				intArg(rng.Int64N(3))
				intArg(rng.Int64N(3))
				e := rng.Int64N(50)
				args, exprs = append(args, &ref.Err{ID: e}), append(exprs, code.C(ErrOp, code.Int(e)))
			case 6:
				// An entry of a call that recovered the errors since a mark, possibly with tokens.
				rm := rng.Int64N(mirror.Entries() + 1)
				errs := mirror.Call(m.Op("rec.since"), []any{rm})[0]
				b.Let(v+"s", code.C("rec.since", code.Int(rm)))
				pos++
				e := mirror.Call(m.Op("memo.store"), []any{int64(0), pos, int64(0), int64(0), true, pos + 1, false, nil, errs, false})[0].(*ref.Entry)
				b.Let(v, code.C("memo.store", code.Int(0), code.Int(pos), code.Int(0), code.C(ValuesOp, code.Int(0)),
					code.BoolC(true), code.Int(pos+1), code.BoolC(false), noValue, code.L(v+"s"), noDocOnly))
				entries = append(entries, entry{v, e})
				calls++
				continue
			case 7, 8:
				if len(entries) == 0 {
					continue
				}
				e := entries[rng.IntN(len(entries))]
				args, exprs = append(args, e.ref), append(exprs, code.L(e.name))
				o = m.Op("missing.replay")
				if rng.IntN(2) == 0 {
					o = m.Op(MissingSeedOp)
					intArg(rng.Int64N(marks() + 1))
				}
			case 9:
				if rng.IntN(2) == 0 {
					o = m.Op("rec.push")
					e := rng.Int64N(50)
					args, exprs = append(args, &ref.Err{ID: e}), append(exprs, code.C(ErrOp, code.Int(e)))
					break
				}
				o = m.Op("rec.truncate")
				intArg(rng.Int64N(mirror.Entries() + 1))
			}
			if !mirror.Allows(o, args) {
				continue
			}
			mirror.Call(o, args)
			calls++
			if len(o.Results) > 0 {
				b.Let(v, code.C(o.Name, exprs...))
			} else {
				b.Do(o.Name, exprs...)
			}
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}
