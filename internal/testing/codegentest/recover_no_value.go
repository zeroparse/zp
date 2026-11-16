package codegentest

// RecoverNoValueCases are grammars with #recover on an expression without a value, which stays
// without one whether it matches or recovers: in a sequence it adds no element, which the
// predicates check through the length of a capture; as an alternative it gives nil; as a rule
// body, a capture or a repetition element it gives the unit. Each input list has inputs on which
// the expression matches, recovers, and fails.
var RecoverNoValueCases = []Case{
	{"recover no value/discard in a sequence", `rule main = x:("q" (-"a") #recover(skip="z")) {len($x) == 1} $$`, []string{"qa", "qz", "q", "qy"}},
	{"recover no value/lookahead in a sequence", `rule main = x:("q" (!"a") #recover(skip="a")) {len($x) == 1} $$`, []string{"q", "qa", "qb"}},
	{"recover no value/nested recoveries", `rule main = x:("q" (-"a") #recover(skip="z") #recover(skip="y")) {len($x) == 1} $$`, []string{"qa", "qz", "qy", "qx"}},
	{"recover no value/label inside", `rule main = x:("q" ((-"a") @error("want a")) #recover(skip="z")) {len($x) == 1} $$`, []string{"qa", "qz", "qx"}},
	{"recover no value/alternative", `rule main = x:((-"a") #recover(skip="z") / "b") "c" $$`, []string{"ac", "zc", "bc", "c"}},
	{"recover no value/rule body", `rule main = r "c" $$
rule r = (-"a") #recover(skip="z")`, []string{"ac", "zc", "c"}},
	{"recover no value/capture", `rule main = x:((-"a") #recover(skip="z")) "c" $$`, []string{"ac", "zc", "c"}},
	{"recover no value/repetition element", `rule main = ((-"a") #recover(skip="z"))* "c" $$`, []string{"aac", "zac", "c", "azzc"}},
}
