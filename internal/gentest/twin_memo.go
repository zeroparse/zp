package gentest

// TwinOnePlaceSrc is a grammar in which the memoized rule r is called from two places and its
// value-free twin from one place only, `-r` in w, whose value is never discarded, so that w has no
// twin and the place is the only one of the twin of r in the compiled program too. The twin is
// memoized all the same (Evaluations of Rules in spec/runtime/parsing.md). On "abcy", w calls it at
// 1 three times: twice from w at 0, after w consumes the "a", and once from w at 1; the first two
// evaluate it, the second storing its result, which answers the third. w fails each time, and main
// ends with a call of r at 0 whose value is used.
const TwinOnePlaceSrc = "@start\nrule main = w \"1\" / w \"2\" / \"a\" w \"3\" / r \"y\"\nrule w = \"a\"? -r \"x\"\nrule r = [a-c]* t?\nrule t = \"d\"\n"

// TwinOwnEntriesSrc is a grammar that calls the twin of the memoized rule r at 0 three times and
// then r itself four times. A rule and its twin have memo entries of their own (Evaluations of
// Rules in spec/runtime/parsing.md), so on "abcy" the first two calls of each evaluate the body
// and the second stores its result, which answers the later calls of the same variant: the body
// of r is evaluated four times, twice for each variant.
const TwinOwnEntriesSrc = "@start\nrule main = -r \"1\" / -r \"2\" / -r \"3\" / r \"4\" / r \"5\" / r \"6\" / r \"y\"\nrule r = [a-c]* t?\nrule t = \"d\"\n"
