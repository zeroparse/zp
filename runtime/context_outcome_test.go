package runtime

import "testing"

// TestObsOutcomeWide checks that outcomes whose frontier does not fit in a word get numbers that
// tell equal outcomes from others and from the outcomes that fit.
func TestObsOutcomeWide(t *testing.T) {
	p := &Parser{}
	type outcome struct {
		ok            bool
		pos, examined int
	}
	ids := map[uint64]outcome{}
	for _, d := range []int{0, 5, 1<<62 - 1, 1 << 62, 1<<62 + 1, -1} {
		for _, ok := range []bool{false, true} {
			o := outcome{ok, 0, d}
			v := p.obsOutcome(o.ok, o.pos, o.examined)
			if again := p.obsOutcome(o.ok, o.pos, o.examined); again != v {
				t.Errorf("%+v: number %#x, then %#x", o, v, again)
			}
			if prev, dup := ids[v]; dup {
				t.Errorf("%+v and %+v: same number %#x", prev, o, v)
			}
			ids[v] = o
			if v == obsNone {
				t.Errorf("%+v: number %#x is not an outcome", o, v)
			}
		}
	}
}
