package complexitytest

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
)

// DefaultDifferentialGrammars is the default number of random grammars that TestLinearClaims
// generates: the long tier generates this many, and the default tier a quarter of them.
const DefaultDifferentialGrammars = 600

// SharedContinuationCount is the number of SharedContinuationGrammars that TestLinearClaims
// measures among count random grammars.
func SharedContinuationCount(count int) int { return count / 2 }

// SharedContinuationGrammars returns count random grammars with the start rule main, from a fixed
// seed: the first n of count grammars are the n grammars of a smaller count. They are the grammars
// on which the claims of linear cost of the classifier rest on the exclusion analysis, which
// decides a unit from the symbols that it and its continuations evaluate alike: a recursive rule
// whose alternatives share a prefix, a separator or a call of a trivia rule, a later alternative or
// a continuation that begins with a lookahead that scans, a caller that goes on with a separator
// where the recursion ends, a list with a trailing separator, and an Operators Rule with a skip.
//
// The alphabet is a, which opens, b, which closes, c, an operand, a space, whitespace, and a comma;
// the scans that fail after they scan end at an x that no input of SharedContinuationInputs has.
func SharedContinuationGrammars(count int) []string {
	rng := rand.New(rand.NewPCG(29, 31))
	gs := make([]string, count)
	for i := range gs {
		gs[i] = sharedContinuationGrammar(rng, i%sharedShapes)
	}
	return gs
}

// SharedContinuationInputs are the inputs of SharedContinuationGrammars of the differential test:
// openings followed by whitespace, a word or separators, which the continuations of the recursion
// evaluate again at each level, and plain repetitions. The input of size n has about n bytes.
var SharedContinuationInputs = []func(n int) string{
	func(n int) string { return rep("a", n/2) + rep(" ", n/2) },
	func(n int) string { return rep("a", n) + "c" },
	func(n int) string { return rep("a", n/3) + "c" + rep(" ", n/3) + rep("b", n/3) },
	func(n int) string { return rep("a", n/2) + "c" + rep(", ", n/4) },
	func(n int) string { return rep("a", n/2) + rep("b", n/2) },
	func(n int) string { return rep("a ", n/2) },
	func(n int) string { return rep("ac ", n/3) },
	func(n int) string { return rep("a", n/2) + "c" + rep("b ", n/4) },
}

// sharedShapes is the number of shapes of SharedContinuationGrammars, which take turns.
const sharedShapes = 5

// sharedContinuationGrammar returns grammar number shape (mod sharedShapes) of the shapes of
// SharedContinuationGrammars, with the rules that it calls chosen at random.
func sharedContinuationGrammar(rng *rand.Rand, shape int) string {
	// pick chooses one of xs; a repeated entry is more likely, which favors the forms on which the
	// classifier's claims depend on the exclusion analysis.
	pick := func(xs ...string) string { return xs[rng.IntN(len(xs))] }
	// The rules of the grammar, written for the names that the shapes use. ws is a trivia rule of
	// one of its forms; sc and sw scan without being trivia and fail after they scan; m is a call
	// that the memo answers; pre is a prefix that scans.
	rules := map[string]string{
		"ws":  pick(`[ ]*`, `[ ]*`, `[ ]*`, `[ ]*`, `" "*`, `" "*`, "sp*", `(sp / "\t")*`, `(sp / "#" [^\n]*)*`),
		"sp":  `" "`,
		"sc":  pick(`"a" [ab]* "x"`, `[ab ]* "x"`, `" "+ "x"`, `[a-c]* "x"`),
		"sw":  pick(`" "+`, `[ ]+ "b"`, `" "* "x"`),
		"m":   "mx",
		"mx":  `"b"`,
		"pre": pick(`"a"*`, `"a"+`, "[ab]+"),
	}
	call := func() string { return pick("ws", "ws", "ws", "sc", "sw", "m", "ws ws", "ws sc", "m ws", "pre") }
	sep := func() string { return pick("ws", "ws", "ws ws", "m ws", "sw", "sc", "") }
	var b strings.Builder
	var def func(name, body string)
	def = func(name, body string) { fmt.Fprintf(&b, "rule %s = %s\n", name, body) }
	switch shape {
	case 0:
		// A later alternative begins with a lookahead that scans, in a recursion that the caller
		// repeats.
		look := func() string {
			return pick("sc", "sc", "sc", `(sc / "a")`, `(sc / "a")`, `"a" sc`, "pre", "ws sc", `ws "b"`, "m")
		}
		alts := []string{`"a" e ` + pick(`"x"`, `"x"`, `"b"`, `"b"`, `"b"`, `("b" / "x")`, `"b" ws`, `"b" m`)}
		if rng.IntN(10) == 0 {
			alts = append([]string{pick(`pre "b"`, `"c" sc "x"`, `m ws "c"`)}, alts...)
		}
		alts = append(alts, pick("!", "&")+look()+pick(` "b"`, ` "c"`, ` "a"`))
		if rng.IntN(10) == 0 {
			alts = append(alts, `"b" !`+look())
		}
		alts = append(alts, pick(`"c"`, `"c"`, `"b"`))
		def("main", pick("e", "e", "e", "e $$", "e $$", "e $$", `(e / "a")* $$`, "e ws $$"))
		def("e", strings.Join(alts, " / "))
	case 1:
		// A recursion whose optional tail scans where it fails, and a caller that scans the same
		// call after it.
		tail := fmt.Sprintf(`(%s %s "b")?`, pick("", `"c"`, `"c"`, "m"), call())
		body := `"a" ` + pick("e?", "e?", "(e)?")
		switch rng.IntN(4) {
		case 0:
			// The tail through rules that only pass it on.
			def("t", "o")
			def("o", tail)
			tail = "t"
		case 1:
			// A memoized call before the recursion and in the tail.
			body = `"a" m ` + pick("e?", "(e)?")
			tail = fmt.Sprintf(`(m %s "b")?`, pick("ws", "ws sc", "sw"))
		}
		def("main", pick("e ws $$", "e ws $$", "e m ws $$", "e ws sc? $$", "e sw $$", "e $$", "(e ws)* $$"))
		def("e", body+" "+tail)
	case 2:
		// A list with a separator that may trail, and whitespace after each token.
		w := pick("ws", "ws", "ws ws", "sw", "")
		s := pick(`","`, `","`, `";"`)
		trail := pick(fmt.Sprintf("(%s %s)?", s, w), s+"?", fmt.Sprintf("(%s %s)?", s, pick("ws", w)))
		def("main", pick("value $$", "value ws $$", "(value ws)* $$"))
		alts := []string{fmt.Sprintf(`"a" %s elems? %s "b" %s`, w, trail, w), `"c" ` + w}
		if rng.IntN(3) == 0 {
			alts = append(alts, `"a" `+w+` "b" `+w)
		}
		def("value", strings.Join(alts, " / "))
		def("elems", fmt.Sprintf(`value (%s %s value)*`, s, w))
	case 3:
		// Alternatives of a repeated choice that begin with the same call or literals, and part
		// after a separator.
		pre := pick("pre", `"a"`, `"a" "a"`, "m", "sc", "sw")
		alts := []string{
			fmt.Sprintf(`%s %s "b"`, pre, sep()),
			fmt.Sprintf(`%s %s "c"`, pre, sep()),
		}
		if rng.IntN(2) == 0 {
			alts = append(alts, fmt.Sprintf(`%s %s %s "a"`, pre, sep(), call()))
		}
		alts = append(alts, pick(`"a"`, `"c"`, `" "`))
		def("main", pick("(p)* $$", "(p)* ws $$", "p+ $$"))
		def("p", strings.Join(alts, " / "))
	case 4:
		// An Operators Rule whose operands and parts share a skip or a call with what follows. A
		// level of a prefix operator part alone is left out: with a skip, the classifier calls it
		// linear, but a parse of "a"^n "c" " "^n costs n^2 steps.
		pair := "prefix \"a\"\n        postfix \"b\""
		part := pick(pair, pair, pair, pair, pair, pair, pair, pair,
			"postfix \"b\"", "infix right ws \",\"", "infix left \",\"", "postfix ws \"b\"")
		skip := pick("    skip ws\n", "    skip ws\n", "    skip ws\n", "    skip ws\n", "    skip ws\n", "    skip ws\n", "")
		operand := pick(`"c"`, `"c"`, `"c"`, `"c"`, `"c"`, `"c"`, `"c"`, `"c"`, `"c" / "a" x? "b"`)
		def("main", pick("x ws $$", "x ws $$", "x ws $$", "x ws $$", "x $$", "(x ws)* $$"))
		fmt.Fprintf(&b, "rule x = #operators {\n%s    operand %s\n    level { %s }\n}\n", skip, operand, part)
	}
	names := make([]string, 0, len(rules))
	for name := range rules {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		def(name, rules[name])
	}
	return b.String()
}
