package complexity

import "testing"

// TestDegOf pins the degree of a form outside a fold: the largest power of n, and unknownDeg as
// soon as a term reads a quantity of an accumulator.
func TestDegOf(t *testing.T) {
	for _, c := range []struct {
		name string
		x    lin
		want int
	}{
		{"constant", constLin(3), 0},
		{"linear", lin{{o: 1}: 2}, 1},
		{"largest power", lin{{o: 1}: 1, {o: 3}: 1, unitMono: 1}, 3},
		{"size of an accumulator", lin{one(varS): 1}, unknownDeg},
		{"count of an accumulator times n", lin{{o: 1}: 1, one(varN).mul(mono{o: 2}): 1}, unknownDeg},
		{"element of a step", lin{{y: 1}: 1, {o: 1}: 1}, 1},
	} {
		if got := degOf(c.x); got != c.want {
			t.Errorf("%s: degOf(%v) = %d, want %d", c.name, c.x, got, c.want)
		}
	}
}
