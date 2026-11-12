package compile

import "testing"

// classifierSoundnessCases pins the class of the grammars that the soundness review of the cost
// classifier found: reads of variables that hold values of calls, growing units led by predicates
// and calls that match no input, growths that a unit throws away, and units that a lookahead
// decides. Degree -1 leaves the degree open; growing -1 leaves the growing-variable bindings open.
var classifierSoundnessCases = []struct {
	group, name, src string
	class            ComplexityClass
	degree, growing  int
}{
	{"variables", "copy of a read of a call value, doubling", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = ("(" c:r ")" {x := $c} => x + x) / ("a" => "a")
`, ClassExp, -1, -1}, // p1
	{"variables", "x + \"b\" over a call value", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = ("(" c:r ")" {x := $c} => x + "b") / ("a" => "a")
`, ClassPoly, 2, -1}, // p2
	{"variables", "$c + \"b\" control", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = ("(" c:r ")" {x := $c} => $c + "b") / ("a" => "a")
`, ClassPoly, 2, -1}, // p3
	{"variables", "copy in a rule called after the binding", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = ("(" c:r ")" {x := $c} v:t => $v) / ("a" => "a")
rule t string = "" => x + x
`, ClassExp, -1, -1}, // p4
	{"variables", "x + \"b\" in a rule called after the binding", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = ("(" c:r ")" {x := $c} v:t => $v) / ("a" => "a")
rule t string = "" => x + "b"
`, ClassPoly, 2, -1}, // p5
	{"variables", "read without a copy", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = ("(" c:r ")" {x := $c} => x) / ("a" => "a")
`, ClassLinearEnv, -1, -1}, // p6
	{"variables", "doubling in a positive lookahead", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = (&({x := x + x}) "y" v:r => $v) / ("z" => x)
`, ClassExp, -1, 1}, // p7
	{"variables", "extension in a positive lookahead", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = (&({x := x + "a"}) "y" v:r => $v) / ("z" => x)
`, ClassExp, -1, -1}, // p8
	{"variables", "extension before the call control", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = ({x := x + "a"} "y" v:r => $v) / ("z" => x)
`, ClassExp, -1, -1}, // p9
	{"variables", "doubling in a discard control", `var x string = "q"
@start
rule s string = v:r $$ => $v
rule r string = (-({x := x + x} "y") v:r => $v) / ("z" => x)
`, ClassExp, -1, -1}, // p10
	{"variables", "bounded chain without recursion", `var x string = "q"
@start
rule s = a:A {x := $a} A $$
rule A string = {x := x + x} => x
`, ClassLinearEnv, -1, -1}, // p11
	{"growing", "predicate before the left call (off-cycle empty call)", `@start
rule s = E $$
rule E = Z E R "!" / E "+" "x" / "x"
rule Z = ""
rule R = [ ]*
`, ClassLinear, 0, -1}, // p12
	{"growing", "predicate before the left call", `@start
rule s = E $$
rule E = {true} E R "!" / E "+" "x" / "x"
rule Z = ""
rule R = [ ]*
`, ClassLinear, 0, -1}, // p13
	{"growing", "growing unit without predicate control", `@start
rule s = E $$
rule E = E R "!" / E "+" "x" / "x"
rule Z = ""
rule R = [ ]*
`, ClassLinear, 0, -1}, // p14
	{"growing", "alternative dropped before a growing one, dropped result regrown", `@start
rule main = S $$
rule S = R "z" / S "q" / "y"
rule R = R "q" / S
`, ClassUnknown, 2, -1}, // p15
	{"growing", "dropped result behind a predicate", `@start
rule main = S $$
rule S = R "z" / {true} S "q" / "y"
rule R = R "q" / S
`, ClassUnknown, 2, -1}, // p16
	{"growing", "left call that fails after a path through a call", `@start
rule s = E $$
rule E = F "+" "x" / E [a-z] / "a"
rule F = E "z" / "a" [a-z]* "?"
`, ClassUnknown, -1, -1}, // p17
	{"growing", "left call that fails after a path through a group", `@start
rule s = E $$
rule E = (E "z" / "a" [a-z]* "?") "+" "x" / E [a-z] / "a"
`, ClassUnknown, -1, -1}, // p18
	{"growing", "dropped result, last alternative", `@start
rule main = S $$
rule S = R "z" / S "q"
rule R = R "q" / S / "y"
`, ClassUnknown, 2, -1}, // c5last
	{"growing", "dropped result behind an option", `@start
rule main = S $$
rule S = R "z" / "w"? S "q" / "y"
rule R = R "q" / S
`, ClassUnknown, 2, -1}, // c5w
	{"growing", "lookahead of the cycle itself", `@start
rule s = E $$
rule E = &E "A" [a-z]* "?" / E [a-z] / "A"
`, ClassUnknown, -1, -1}, // pla
	{"growing", "lookahead of the cycle itself in a group", `@start
rule s = E $$
rule E = &(E) "A" [a-z]* "?" / E [a-z] / "A"
`, ClassUnknown, -1, -1}, // pla2
	{"growing", "lookahead scan over the left call", `@start
rule s = E $$
rule E = &("A" [a-z]*) E [a-z] / "A"
`, ClassUnknown, 2, -1}, // la1
	{"growing", "empty call before a scan", `@start
rule s = E $$
rule E = Z "a" [a-z]* "?" / Z E [a-z] / "a"
rule Z = ""
`, ClassUnknown, 2, -1}, // zs1
	{"growing", "shared scan after the growing alternative", `@start
rule s = E $$
rule E = "A" W "!" [a-z]* "?" / E "y" / "A" W "x"
rule W = [ ]*
`, ClassPoly, 2, -1}, // dw1
	{"thrown", "growth thrown away in the step of another leader (main repeats S)", `@start
rule main = S* $$
rule S = R "z" / S "q" / "a"
rule R = R [ac] / S
`, ClassUnknown, 2, -1}, // n1
	{"thrown", "growth thrown away in the step of another leader", `@start
rule main = (S "b")* $$
rule S = R "z" / S "q" / "a"
rule R = R [ab] / S "b"
`, ClassUnknown, -1, -1}, // n2
	{"thrown", "growth thrown away by a rule that is not a leader", `@start
rule main = (S "b")* $$
rule S = R "z" / "a"
rule R = R [ab] / S "b"
`, ClassUnknown, -1, -1}, // n3
	{"thrown", "growth of a leader that the other leader reads", `@start
rule main = S $$
rule S = R "z" / S "q" / R
rule R = R "b" / S "y" / "c"
`, ClassUnknown, -1, -1}, // s4b
	{"thrown", "cut after a left call makes a later step take an alternative the first did not reach", `@start
rule main = S $$
rule S = S "w" / R "z" / "c" Bs
rule Bs = "b" Bs / ""
rule R = (R #cut "b" / S "y" / "c") / S "x"
`, ClassUnknown, -1, -1}, // s4d
	{"thrown", "nested reads", `@start
rule main = S $$
rule S = S "q" / R "s" / "y"
rule R = R "r" / S "t"
`, ClassUnknown, -1, -1}, // t1
	{"thrown", "lookahead regrowth", `@start
rule main = S $$
rule S = &R S "q" / "y"
rule R = R ("q" / "r") / S "t" / "y"
`, ClassUnknown, -1, -1}, // t2
	{"thrown", "three leaders", `@start
rule main = S $$
rule S = &R S "q" / "y"
rule R = &T R "q" / "y"
rule T = T "q" / S "z" / "y"
`, ClassUnknown, -1, -1}, // t3
	{"thrown", "tree carried after the growing alternative", `@start
rule main = S $$
rule S = S "q" / R "!" / "y"
rule R = R "q" / S
`, ClassUnknown, -1, -1}, // t4
	{"decided", "deciding negative lookahead with a scan", `@start
rule main = ( !(a ([a-z] #cut)* "!" / a) a ";" / a )* $$
rule a = "a"
`, ClassPoly, 2, -1}, // k8
	{"decided", "deciding negative lookahead of a rule with a scan", `@start
rule main = ( !k a ";" / a )* $$
rule k = a ([a-z] #cut)* "!" / a
rule a = "a"
`, ClassPoly, 2, -1}, // m1
	{"decided", "deciding negative lookahead of a rule that calls a memoized scan", `@start
rule main = ( !k a ";" / a )* $$
rule k = a s "!" / a
rule s = ([a-z] #cut)*
rule a = "a"
`, ClassPoly, 2, -1}, // m2
	{"decided", "decisive call that expands to a deciding negative lookahead", `@start
rule main = ( c "x" / "q" )* $$
rule c = !("q" s) "b"
rule s = ([q] #cut)*
`, ClassPoly, 2, -1}, // c5
	{"decided", "passed positive lookahead starting with a scanning negative lookahead", `@start
rule main = ( &k "x" / "q" )* $$
rule k = !("q" s) "b"
rule s = ([q] #cut)*
`, ClassPoly, 2, -1}, // c6
	{"decided", "passed positive lookahead of a rule with a scan", `@start
rule main = ( &k "x" / "q" )* $$
rule k = "q" s "!" / "q"
rule s = ([q] #cut)*
`, ClassPoly, 2, -1}, // c10
	{"decided", "passed positive lookahead of a memoized scanning call", `@start
rule main = ( &(id "=") "Q" id "=" / [a-z] )* $$
rule id = [a-z] ([a-z] #cut)*
`, ClassPoly, 2, -1}, // c13
	{"decided", "deciding negative lookahead without a scan", `@start
rule main = ( !(a "!" / a) a ";" / a )* $$
rule a = "a"
`, ClassLinear, 0, -1}, // m3
	{"decided", "passed positive lookahead, continuation repeats the call", `@start
rule main = ( &(id "=") id "=" / id ":" )* $$
rule id = [a-z] ([a-z] #cut)*
`, ClassLinear, 0, -1}, // c12
	{"decided", "restart through another alternative", `@start
rule main = ( !(a ([a-z] #cut)* "!" / a) a ";" / a / "q" a )* $$
rule a = "a"
`, ClassPoly, 2, -1}, // k9
	{"decided", "restart through a choice", `@start
rule main = ( (k / a) ";" / a )* $$
rule k = a s "!"
rule s = ([a-z] #cut)*
rule a = "a"
`, ClassPoly, 2, -1}, // c8
	{"decided", "undecided restart", `@start
rule main = ( &(id "=") id "=" / [a-z] )* $$
rule id = [a-z] ([a-z] #cut)*
`, ClassPoly, 2, -1}, // c11
	{"decided", "pair of constant lookaheads", `@start
rule main = ( "x" &"a" [a-z] ([a-z] #cut)* "!" / "x" &"b" [a-z] )* $$
rule a = "a"
`, ClassLinear, 0, -1}, // p1
	{"decided", "pair of lookaheads with a class", `@start
rule main = ( "x" &"a" [a-z] ([a-z] #cut)* "!" / "x" &[ab] [a-z] )* $$
rule a = "a"
`, ClassPoly, 2, -1}, // p2
	{"decided", "continuation lookahead that scans, deciding against a unit lookahead", `@start
rule main = ( &"a" [a-z] ";" / &k [a-z] )* $$
rule k = "b" s "!" / "b"
rule s = ([b] #cut)*
`, ClassPoly, 2, -1}, // f2a
	{"decided", "unit lookahead against a continuation negative lookahead", `@start
rule main = ( &"a" [a-z] ";" / !k "b" )* $$
rule k = "b" s "!"
rule s = ([b] #cut)*
`, ClassPoly, 2, -1}, // f2b
	{"decided", "case-insensitive literal against a case-sensitive one", `@start
rule main = ( "ab"i s "!" / "AC" )* $$
rule s = ([a-zA-Z] #cut)*
`, ClassPoly, 2, -1}, // f3a
	{"decided", "case-insensitive literals that differ", `@start
rule main = ( "ab"i s "!" / "AX"i )* $$
rule s = ([a-zA-Z] #cut)*
`, ClassLinear, 0, -1}, // f3b
	{"decided", "alike bounded repetitions", `@start
rule main = ( "x" [a-z] #repeat(,3) "1" (!";" .)* "!" / "x" [a-z] #repeat(,3) "2" (!";" .)* ";" )* $$
`, ClassLinear, 0, -1}, // r1
	{"decided", "alike bounded repetitions with a predicate", `@start
rule main = ( "x" ([a-z] {1 == 1}) #repeat(,3) "1" (!";" .)* "!" / "x" ([a-z] {1 == 1}) #repeat(,3) "2" (!";" .)* ";" )* $$
`, ClassPoly, 2, -1}, // r2
}

// TestClassifierSoundness checks the class of each grammar in classifierSoundnessCases.
func TestClassifierSoundness(t *testing.T) {
	t.Parallel()
	for _, tc := range classifierSoundnessCases {
		t.Run(tc.group+"/"+tc.name, func(t *testing.T) {
			t.Parallel()
			c := classifySource(t, tc.src)
			if c.Class != tc.class {
				t.Fatalf("class %v, want %v\ncauses %v\nreasons %v", c.Class, tc.class, c.Causes, c.Reasons)
			}
			if tc.degree >= 0 && classDegree(c) != tc.degree {
				t.Errorf("degree %d, want %d\ncauses %v\nreasons %v", classDegree(c), tc.degree, c.Causes, c.Reasons)
			}
			if tc.growing >= 0 && len(c.Growing) != tc.growing {
				t.Errorf("growing %v, want %d", c.Growing, tc.growing)
			}
		})
	}
}
