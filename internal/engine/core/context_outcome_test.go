package core

import "testing"

// TestObsOutcomeWide checks that outcomes whose examined range does not fit in a word get identities
// that tell equal outcomes from others, and that such an outcome is stored and found in the table of
// its context, as an outcome of a short range is.
func TestObsOutcomeWide(t *testing.T) {
	const pos = 1 << 41
	p := &Parser{}
	type outcome struct {
		ok             bool
		examined, from int
	}
	var outcomes []outcome
	for _, d := range []int{0, 5, 1 << 30, 1 << 31, 1 << 40} {
		for _, ok := range []bool{false, true} {
			outcomes = append(outcomes, outcome{ok, pos + d, pos}, outcome{ok, pos, pos - d}, outcome{ok, pos + d, pos - d})
		}
	}
	ids := map[uint64]outcome{}
	for _, o := range outcomes {
		v, stored := p.obsOutcome(o.ok, pos, o.examined, o.from)
		if !stored {
			t.Fatalf("%+v: no outcome", o)
		}
		if again, _ := p.obsOutcome(o.ok, pos, o.examined, o.from); again != v {
			t.Errorf("%+v: identity %#x, then %#x", o, v, again)
		}
		if prev, dup := ids[v]; dup && prev != o {
			t.Errorf("%+v and %+v: same identity %#x", prev, o, v)
		}
		ids[v] = o
		if v == obsNone || v == obsIgnored {
			t.Errorf("%+v: identity %#x is not an outcome", o, v)
		}
	}
	if !lookaheadStoredAt(p, pos, pos+1<<31, pos-1<<40) {
		t.Error("a lookahead outcome with a range of 2^40 units is not found in its context")
	}
}

// TestObsOutcomeWideControl checks that TestObsOutcomeWide detects outcomes that are not stored when
// their range does not fit in a word.
func TestObsOutcomeWideControl(t *testing.T) {
	ContextMemoMutation = CtxNarrowOutcome
	defer func() { ContextMemoMutation = 0 }()
	if lookaheadStoredAt(&Parser{}, 1<<41, 1<<41+1<<31, 1<<41-1<<40) {
		t.Error("CtxNarrowOutcome not detected")
	}
}

// lookaheadStoredAt ends a lookahead at pos that read the seed of a growth in a context there and
// examined [from, examined), and reports whether its outcome is then found in the context.
func lookaheadStoredAt(p *Parser, pos, examined, from int) bool {
	p.Pos = pos
	p.Gamma = []growRec{
		{key: MemoKey{Rule: 1, Pos: pos}, id: failSeed, bit: 1, cycle: 1},
		{key: MemoKey{Rule: 2, Pos: pos}, id: failSeed + 1, bit: 2, cycle: 1},
	}
	p.pushFrame(pos, 0)
	const site = 7
	p.LookBegin(site)
	p.noteSeedRead(1)
	p.Hw, p.lw = examined, from
	p.LookEnd(true)
	want, _ := p.obsOutcome(true, pos, examined, from)
	return p.obsValue(pos, 0, p.present(0), site) == want
}
