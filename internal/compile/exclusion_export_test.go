package compile

// ExclusionHooks is a setting of the test hooks of the exclusion analysis, for the tests of
// package compile_test.
type ExclusionHooks struct {
	Decided, Trivia, Extended, Vars, Memo bool
}

// Set sets the hooks, with decided units at k = 3, and returns a function that restores the
// defaults. The hooks are package variables, so the tests that set them do not run in parallel.
func (h ExclusionHooks) Set() func() {
	decidedUnitsOn, decidedUnitsK = h.Decided, 3
	triviaOn, triviaExtended, triviaVars, triviaMemo = h.Trivia, h.Extended, h.Vars, h.Memo
	return func() {
		decidedUnitsOn, decidedUnitsK = false, 3
		triviaOn, triviaExtended, triviaVars, triviaMemo = false, false, false, false
	}
}
