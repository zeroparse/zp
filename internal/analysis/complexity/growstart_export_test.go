package complexity

// SetGrowStart sets whether the check of a growing unit of a leader compares the paths on which
// its first step goes on at its start with the later alternatives (growStartOn), for the tests of
// package complexity_test, and returns a function that restores the default. The off setting is
// the mutant that skipped those paths when they contain no scanning source.
func SetGrowStart(on bool) func() {
	growStartOn = on
	return func() { growStartOn = true }
}

// SetGrowNullable sets whether the check of a growing unit of a leader at the start of its
// first-step paths counts a path that can match without consuming input as beginning with any
// character (growNullableOn), for the tests of package complexity_test, and returns a function
// that restores the default. The off setting is the mutant that took the characters that the
// lookaheads of such a path examine for its first characters.
func SetGrowNullable(on bool) func() {
	growNullableOn = on
	return func() { growNullableOn = true }
}
