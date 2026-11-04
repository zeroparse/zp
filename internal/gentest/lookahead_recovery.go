package gentest

// LookaheadRecoveryCases are grammars that recover errors inside lookaheads and inside the skip of
// an Operators Rule, whose ends discard those errors: a positive lookahead and a double negative
// one give the same results, and a call after a lookahead at the same position reports its own
// errors.
func LookaheadRecoveryCases() []Case {
	const r = `
rule r = ("a" "b") #recover(skip=[^qz]+)
rule t = [a-z]+
`
	inputs := []string{"abq", "axq", "aq", "axz", "abz", "ab", "q", ""}
	return []Case{
		{Name: "recovery in a positive lookahead", Src: `rule main = &(r "q") t $$` + r, Inputs: inputs},
		{Name: "recovery in a double negative lookahead", Src: `rule main = !!(r "q") t $$` + r, Inputs: inputs},
		{Name: "recovery in a negative lookahead", Src: `rule main = !(r "z") t $$` + r, Inputs: inputs},
		{Name: "recovery in a lookahead and after it", Src: `rule main = &(r "q") r "q" $$` + r, Inputs: inputs},
		{Name: "recovery in an Operators Rule skip", Src: `rule main = e $$
rule e = #operators {
    skip (" " "!") #recover(skip=[^0-9+]+)
    operand [0-9]+ #atomic
    level { infix left "+" }
}
`, Inputs: []string{"1 !+2", "1 x+2", "1+2", "1 +2", "1 x", "1 ", ""}},
	}
}
