package gentest

// RecoverBottomCase is a grammar with #recover on an expression, with the inputs on which its parse
// succeeds and those on which it fails. A predicate checks the length of the capture x around the
// recovered expression, so a parse succeeds only if the expression gives the elements that the
// value model says it gives.
type RecoverBottomCase struct {
	Case
	Accept []bool // Accept[i] reports that the parse of Inputs[i] succeeds
}

// RecoverBottomCases are grammars with #recover on an expression of type _|_, which never
// matches: its recovery is an error value, one element, whether the expression is _|_ itself or
// has that type by the rules of the type system (a sequence with an element of type _|_, a choice
// of such alternatives, a repetition of at least one such element, a group, a label, a call of a
// splice rule with such a body), and whatever elements it would have if it matched. The cases
// whose names start with "no value" are their controls: an expression without a value stays
// without one, also when its operand is _|_. Those whose names start with "container" check the
// types []_|_ and *_|_: an optional expression whose operand has type _|_ is nil, one element as
// for any optional expression, and a repetition of _|_ matches the empty input and is the empty
// list; a recovery of either keeps that value.
var RecoverBottomCases = []RecoverBottomCase{
	{Case{"bottom/bare", `rule main = x:("q" _|_ #recover(skip="z")) {len($x) == 2} $$`, []string{"qz", "q", "qa"}}, []bool{true, false, false}},
	{Case{"bottom/sequence", `rule main = x:("q" (_|_ -"a") #recover(skip="z")) {len($x) == 2} $$`, []string{"qz", "q", "qa"}}, []bool{true, false, false}},
	{Case{"bottom/choice", `rule main = x:("q" (_|_ / _|_) #recover(skip="z")) {len($x) == 2} $$`, []string{"qz", "q", "qa"}}, []bool{true, false, false}},
	{Case{"bottom/choice of sequences", `rule main = x:("q" (-"a" _|_ / !"b" (_|_ / _|_)) #recover(skip="z")) {len($x) == 2} $$`, []string{"qz", "qaz", "q"}}, []bool{true, false, false}},
	{Case{"bottom/nested attributes", `rule main = x:("q" (((_|_ -"a") @error("want a")) @error("want b")) #recover(skip="z")) {len($x) == 2} $$`, []string{"qz", "q"}}, []bool{true, false}},
	{Case{"bottom/splice rule body", `rule main = x:("q" _r #recover(skip="z")) {len($x) == 2} $$
rule _r = -"a" _s
rule _s = _|_ / (_|_ !"b")`, []string{"qz", "qa", "q"}}, []bool{true, false, false}},
	{Case{"bottom/rule body", `rule main = x:("q" r #recover(skip="z")) {len($x) == 2} $$
rule r = -"a" _|_`, []string{"qz", "qa", "q"}}, []bool{true, false, false}},
	{Case{"bottom/alternative dropped", `rule main = x:("q" (-"a" / "b" _|_) #recover(skip="z")) {len($x) == 1} $$`, []string{"qa", "qz", "qb", "q"}}, []bool{true, true, false, false}},
	{Case{"bottom/alternative with a capture", `rule main = x:("q" (-"a" / _|_ y:"b") #recover(skip="z")) {len($x) == 1 && $y == nil} $$`, []string{"qa", "qz", "qb"}}, []bool{true, true, false}},
	{Case{"no value/discard of bottom", `rule main = x:("q" (-(_|_)) #recover(skip="z")) {len($x) == 1} $$`, []string{"qz", "q"}}, []bool{true, false}},
	{Case{"no value/lookahead of bottom", `rule main = x:("q" (!(_|_ -"a")) #recover(skip="z")) {len($x) == 1} $$`, []string{"q", "qz"}}, []bool{true, false}},
	{Case{"bottom/sequence with values", `rule main = x:("q" ("a" y:"b" _|_) #recover(skip="z")) {len($x) == 2 && $y == nil} $$`, []string{"qz", "qab", "q"}}, []bool{true, false, false}},
	{Case{"bottom/repetition of at least one", `rule main = x:("q" (_|_ "a")+ #recover(skip="z")) {len($x) == 2} $$`, []string{"qz", "q", "qa"}}, []bool{true, false, false}},
	{Case{"container/optional bottom", `rule main = x:("q" (_|_ -"a")? #recover(skip="z")) {len($x) == 2 && $x.Value[1] == nil} $$`, []string{"q", "qz"}}, []bool{true, false}},
	{Case{"container/optional of bare bottom", `rule main = x:("q" (_|_)? "r") {len($x) == 3 && $x.Value[1] == nil} $$`, []string{"qr", "q"}}, []bool{true, false}},
	{Case{"container/optional bottom in a choice", `rule main = x:("q" ((_|_)? / -"a")) {len($x) == 2} $$`, []string{"q", "qa"}}, []bool{true, false}},
	{Case{"container/repetition of bare bottom", `rule main = x:("q" (_|_)* "r") {len($x) == 3 && len($x.Value[1].Value) == 0} $$`, []string{"qr", "q"}}, []bool{true, false}},
	{Case{"no value/sequence", `rule main = x:("q" (-"a" -"b") #recover(skip="z")) {len($x) == 1} $$`, []string{"qab", "qz", "q"}}, []bool{true, true, false}},
	{Case{"container/optional of a bottom with a value", `rule main = x:("q" (_|_ y:"a")? #recover(skip="z")) {len($x) == 2 && $y == nil} $$`, []string{"q", "qa", "qz"}}, []bool{true, false, false}},
	{Case{"container/repetition of bottom", `rule main = x:("q" (_|_ -"a")* #recover(skip="z")) {len($x) == 2} $$`, []string{"q", "qz"}}, []bool{true, false}},
}

// RecoverBottomGenCases are the grammars of RecoverBottomCases as Cases.
func RecoverBottomGenCases() []Case {
	out := make([]Case, len(RecoverBottomCases))
	for i, c := range RecoverBottomCases {
		out[i] = c.Case
	}
	return out
}
