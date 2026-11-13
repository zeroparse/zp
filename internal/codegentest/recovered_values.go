package codegentest

// RecoveredValueCases are grammars whose parses of some inputs recover from a syntax error and still
// return a value: an error inside a tuple element, a struct inside a list, and field reads chained
// through a recovered struct. Each input list has inputs that parse without an error, recover once,
// and recover in several places.
var RecoveredValueCases = []Case{
	{"recovered value/error in a tuple element", `rule main = "(" a:item "," b:item ")" $$
rule item = [0-9] #recover(skip=[^,)]*)`, []string{"(1,2)", "(x,2)", "(1,y)", "(x,y)"}},
	{"recovered value/struct in a list", `type S = { A int; B *(node[string] | error) }
type L = { Items []S }
rule main L = xs:s+ $$ => L{Items: $xs.Value}
rule s S = a:[0-9] b:item ";" => S{A: len($a), B: $b}
rule item = [a-z] #recover(skip=[^;]*)`, []string{"1a;2b;", "1a;2!;", "1!;2!;3c;"}},
	{"recovered value/chained field reads through an error", `type P = { N int; W *(node[string] | error) }
type R = { First int; Second *(node[string] | error) }
rule main R = x:p => R{First: $x.N, Second: $x.W}
rule p P = n:[0-9] w:item ";" => P{N: len($n), W: $w}
rule item = [a-z] #recover(skip=[^;]*)`, []string{"1a;", "1!;", "12!;"}},
}
