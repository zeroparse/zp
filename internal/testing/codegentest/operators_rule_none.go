package codegentest

// OperatorsRuleNoneCases are grammars with a non-associative infix operator below an operator of a
// looser level. The operator does not chain, whatever the operator above it: a second operator of
// its level after its right operand ends the expression, instead of applying to the looser
// expression that holds the first.
var OperatorsRuleNoneCases = []Case{
	{"none-chain: below a left infix", `rule main = e $$
rule e = #operators {
    operand [a-z]
    level { infix left "|" }
    level { infix none "<" }
}`, []string{"a<a", "a<a<a", "a|a<a", "a|a<a<a", "a<a|a<a", "a|a<a|a<a", "a|a<a|a<a<a", "a<a<a|a"}},
	{"none-chain: below a right infix", `rule main = e $$
rule e = #operators {
    operand [a-z]
    level { infix right "=" }
    level { infix none "<" }
}`, []string{"a=a<a", "a=a<a<a", "a=a=a<a<a", "a<a=a<a", "a=a<a=a<a"}},
	{"none-chain: below a prefix and a postfix", `rule main = e $$
rule e = #operators {
    operand [a-z]
    level { prefix "~" }
    level { infix none "<" }
    level { postfix "!" }
}`, []string{"~a<a", "~a<a<a", "a<a!<a", "~a<a!<a", "a<a!", "~a!<a"}},
	{"none-chain: levels of non-associative operators", `rule main = e $$
rule e = #operators {
    operand [a-z]
    level { infix none "==" }
    level { infix none "<" }
    level { infix left "+" }
}`, []string{"a==a<a", "a==a<a<a", "a==a<a==a", "a<a==a", "a<a==a<a", "a==a<a+a<a", "a==a<a+a+a"}},
	{"none-chain: with a left operator of the same level", `rule main = e $$
rule e = #operators {
    operand [a-z]
    level { infix left "|" }
    level {
        infix none "<"
        infix left "&"
    }
}`, []string{"a<a&a", "a<a&a<a", "a|a<a&a<a", "a|a<a<a", "a&a<a&a", "a|a&a<a<a"}},
}
