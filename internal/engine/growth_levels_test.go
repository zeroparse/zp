package engine

import "testing"

// TestGrowthsAreKeptPerBindingLevel pins the rule of "Growths of Left Recursion" in
// spec/runtime/parsing.md that a call reads the current result of a growth only of its own leader and
// binding level. The operand f calls level(e, mul) at the position where e started, a left call of the
// leader e at the level mul. It grows e at that level apart from the growth of e without a level that
// main starts. That growth stops at "1" and absorbs the "!" of every step, so f fails, e matches
// "1+2" and "!" is left over. A call that read the growth without a level would take "1+2" as the
// operand of f and match "1+2!" as a whole.
func TestGrowthsAreKeptPerBindingLevel(t *testing.T) {
	t.Parallel()
	check(t, `rule main = x:e $$ => $x
rule e = #operators {
    operand f / [0-9]+ #atomic
    level { infix left "+" }
    level mul { infix left "*" }
}
rule f = level(e, mul) "!"`,
		fails("1+2!", "1:5: syntax error"),
		ok("1+2", `(lhs:"1"@e op:"+" rhs:"2"@e)@e`),
	)
}
