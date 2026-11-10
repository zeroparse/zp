package gentest

// LeaderStoreCase is a grammar whose leader e is called several times at position 0, inside
// and outside lookaheads. A growth of a leader stores its result even where the memo leaves the
// first evaluation of other rules unstored, so a later call at the position in the same lookahead
// state or a greater one is answered and does not grow e again (Evaluations of Rules in
// spec/runtime/parsing.md).
type LeaderStoreCase struct {
	Name, Src, Input string
	// Evals is the number of body evaluations of e, the steps of its growths: Steps for each
	// growth that the calls make.
	Evals int
	// Calls lists, for each call of e at position 0 from main in order, "grow" when the call grows
	// e and "memo" when the memo answers it.
	Calls string
}

// leaderStoreSteps is the number of steps of a growth of e at 0 on "x+x?": the failed seed,
// "x", "x+x" and the step that fails to grow it further.
const leaderStoreSteps = 3

// LeaderStoreCases returns the LeaderStoreCase grammars.
func LeaderStoreCases() []LeaderStoreCase {
	const e = "\nrule e = e \"+\" \"x\" / \"x\"\n"
	cases := []struct {
		name, main, calls string
		growths           int
	}{
		// Three calls outside lookaheads: only the first grows.
		{"outside", `e ";" / e "!" / e "?"`, "grow memo memo", 1},
		// Inside a lookahead, then outside twice: the entry stored inside answers only calls
		// inside, so the first call outside grows again.
		{"inside then outside", `&(e ";") e ";" / e "!" / e "?"`, "grow grow memo", 2},
		// Outside, then inside, then outside: the entry stored outside answers every later call.
		{"outside then inside", `e ";" / &(e "?") e "!" / e "?"`, "grow memo memo memo", 1},
		// Twice inside a lookahead, then outside: the second call inside is answered.
		{"inside twice", `&(e "?") &(e "?") e "?"`, "grow memo grow", 2},
	}
	out := make([]LeaderStoreCase, len(cases))
	for i, c := range cases {
		out[i] = LeaderStoreCase{Name: c.name, Src: "@start\nrule main = " + c.main + e, Input: "x+x?", Evals: c.growths * leaderStoreSteps, Calls: c.calls}
	}
	return out
}
