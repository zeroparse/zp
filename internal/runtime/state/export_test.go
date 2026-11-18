package state

// The exports of the internal test helpers to the tests of package state_test, which compile terms
// with the compiler side (actionlower) that imports this package.
var (
	Element           = element
	EngineMemberCanon = engineMemberCanon
)
