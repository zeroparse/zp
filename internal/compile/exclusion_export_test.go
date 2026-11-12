package compile

// ExclusionHooks is a setting of the test hooks of the exclusion analysis, for the tests of
// package compile_test. The zero value turns the analysis off: the classifier without decided
// units, trivia rules or the memo of trivia rules.
type ExclusionHooks struct {
	Decided, Trivia, Memo bool
}

// Set sets the hooks, with decided units at k = 3, and returns a function that restores the
// defaults. The hooks are package variables, so the tests that set them do not run in parallel.
func (h ExclusionHooks) Set() func() {
	restoreExclusionDefaults()
	decidedUnitsOn = h.Decided
	triviaOn, triviaMemo = h.Trivia, h.Memo
	return restoreExclusionDefaults
}

// restoreExclusionDefaults sets the hooks of the exclusion analysis to what the classifier uses:
// decided units at k = 3 and trivia rules on, the memo of trivia rules and cut prefixes off.
func restoreExclusionDefaults() {
	cutPrefixesOn, decidedUnitsOn, decidedUnitsK, deadPathsOn = false, true, 3, true
	triviaOn, triviaOnly, triviaMemo = true, nil, false
}
