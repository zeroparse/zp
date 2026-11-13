package codegentest

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// RandomCycleGrammar returns a random grammar of two to five rules that call one another, often
// at the start of an alternative, with lookaheads, optional and repeated items and nested choices,
// and random inputs over its letters. Most such grammars have left-recursion cycles with members
// besides their leaders, read inside and outside lookaheads. The grammar is a function of seed and
// cut, which adds cuts.
func RandomCycleGrammar(seed int64, cut bool) Case {
	c := randomCycleGrammar(seed, cut, false)
	c.Name = fmt.Sprintf("random cycle grammar %d", seed)
	if cut {
		c.Name += " with cuts"
	}
	return c
}

// RandomRecoveringCycleGrammar returns a grammar of RandomCycleGrammar's kind whose items also
// recover from errors (#recover) and call an Operators Rule, r9, whose operands call r1, at the
// lowest binding level or at the level of its postfix operator. r9 is then often a member of a
// cycle whose leaders have results with recovered errors, which are reused in one kind of context
// only. The grammar is a function of seed.
func RandomRecoveringCycleGrammar(seed int64) Case {
	c := randomCycleGrammar(seed, false, true)
	c.Name = fmt.Sprintf("random recovering cycle grammar %d", seed)
	return c
}

// randomCycleGrammar returns the grammar of RandomCycleGrammar, or of RandomRecoveringCycleGrammar
// with recovering. Without recovering it draws the same random numbers as before recovering was
// added, so the seeds of LeftRecursionMemberSeeds keep their grammars.
func randomCycleGrammar(seed int64, cut, recovering bool) Case {
	rng := rand.New(rand.NewSource(seed))
	names := []string{"r0", "r1", "r2", "r3", "r4"}
	nr := 2 + rng.Intn(4)
	lit := func() string { return strconv.Quote(string("abc"[rng.Intn(3)])) }
	call := func() string {
		if recovering && rng.Intn(5) == 0 {
			if rng.Intn(2) == 0 {
				return "r9"
			}
			return "level(r9, l2)"
		}
		return names[rng.Intn(nr)]
	}
	var item func(first bool, depth int) string
	var alt func(depth int) string
	item = func(first bool, depth int) string {
		if recovering && depth < 2 && rng.Intn(9) == 0 {
			return "(" + alt(depth+1) + ") #recover(skip=[abc])"
		}
		k := rng.Intn(100)
		switch {
		case first && k < 45:
			return call()
		case k < 35:
			return lit()
		case k < 60:
			return call()
		case k < 66 && depth < 2:
			return "&" + item(false, depth+1)
		case k < 74 && depth < 2:
			return "!" + item(false, depth+1)
		case k < 78 && depth < 2:
			return item(false, depth+1) + "?"
		case k < 81 && depth < 2:
			return "(" + lit() + " / " + call() + ")*"
		case k < 84 && cut:
			return "#cut"
		case k < 92 && depth < 2:
			return "(" + alt(depth+1) + " / " + alt(depth+1) + ")"
		default:
			return lit()
		}
	}
	alt = func(depth int) string {
		parts := make([]string, 1+rng.Intn(3))
		for i := range parts {
			parts[i] = item(i == 0, depth)
		}
		return strings.Join(parts, " ")
	}
	var b strings.Builder
	end := ""
	if rng.Intn(2) == 0 {
		end = " $$"
	}
	fmt.Fprintf(&b, "@start\nrule main = r0%s\n", end)
	for i := 0; i < nr; i++ {
		alts := make([]string, 1+rng.Intn(3))
		for j := range alts {
			alts[j] = alt(0)
		}
		// Some alternative matches without calling a rule.
		alts = append(alts, lit())
		fmt.Fprintf(&b, "rule %s = %s\n", names[i], strings.Join(alts, " / "))
	}
	if recovering {
		b.WriteString("rule r9 = #operators {\n    operand r1\n    operand \"b\"\n    level l1 { infix left \"a\" }\n    level l2 { postfix \"c\" }\n}\n")
	}
	inputs := []string{""}
	for i := 0; i < 14; i++ {
		s := make([]byte, rng.Intn(9))
		for j := range s {
			s[j] = "abc"[rng.Intn(3)]
		}
		inputs = append(inputs, string(s))
	}
	return Case{Src: b.String(), Inputs: inputs}
}

// LeftRecursionMemberCases are grammars whose cycles have members besides their leaders, where a
// memoized member result could go stale while a leader grows: a leader that grows again at a
// position where member results read its earlier result, a leader whose result read the seed of
// another leader, an Operators Rule leader that grows at one position at two binding levels, and a
// member that reads such a leader result by finishing the leader's growth.
func LeftRecursionMemberCases() []Case {
	return []Case{
		// r1 grows at 1 inside the lookahead of its own body, where r0 at 1 reads its result; outside
		// the lookahead r1 grows there again, and r0 at 1 must read the new seed.
		{"member read by a leader that grows again", `@start
rule main = r0
rule r0 = r1 "a" r0 / "a"
rule r1 = r1 "c" / &(r0 "a") / "a"`, []string{"aaa", "aaab", "a", "aa", "aca", "acaa", "ab", ""}},
		// r1 grows at 2 inside the growth of r3 there, reading the failing seed of r3, so it fails;
		// r0 at 2 reads that result. Called outside the lookahead, r1 grows again and matches.
		{"member read of a leader that read another seed", `@start
rule main = r0
rule r0 = r1 / "b"
rule r1 = r4 / r2
rule r2 = "b" (r3 (r2 "a" / "b" "a" "c") / &r0) r2 / r1
rule r3 = r3 / !r0
rule r4 = r3`, []string{"bb", "b", "bab", "bbb", "ba", "bbac", ""}},
		// expr grows at one position at binding level 0 and, through level(expr, mul), at the level
		// of mul; x reads both seeds.
		{"member of an Operators Rule cycle with binding levels", `@start
rule main = expr $$
rule expr = #operators {
    operand x
    operand "n"
    level add { infix left "+" }
    level mul { infix left "*" }
}
rule x = level(expr, mul) "!" / expr "?"`, []string{"n", "n?", "n!", "n*n!", "n+n?", "n?!", "n*n?!", "n!?", "n+n*n!?", "n+n!*n?", "", "n*"}},
		// On "bcab", the member r9 at 2 calls the leader r1 there, whose growth reads the unstable
		// result of r0 there and recovers an error, so r9 reads an unstable result by finishing the
		// call. r0 grows at 2 inside a lookahead, and then outside it, and r1 grows again inside
		// each growth: the last result of r1 at 2 differs from the one r9 read, and replaces it.
		{"member read of a leader result with recovered errors", `@start
rule main = r0 $$
rule r0 = r0 r0 / r1 (r0 / "b" #recover(skip=[abc])) / "a"
rule r1 = (("b" / level(r9, l2))* !r1 / level(r9, l2) level(r9, l2) r0) / !r0 "b" / r0 (r0 / "a") / "c"
rule r9 = #operators {
    operand r1
    level l2 { postfix "c" }
}`, []string{"bcaa", "bcab", "bcac", "abcaa", "bbcab", "bcaab", "bcaca", "bca", "b", "a", ""}},
	}
}

// LeftRecursionMemberSeeds are the seeds of RandomCycleGrammar (without cuts) whose grammars gave
// a memoized member result that differs from the result without a memo when member results were
// reused while a leader grew at their position, or when a member result that read the result of
// a leader whose growth read another seed was memoized.
var LeftRecursionMemberSeeds = []int64{906, 1661, 2206, 2331, 2938, 7673, 10627, 13632, 21954}

// LeftRecursionMemberCutSeeds are such seeds of RandomCycleGrammar with cuts.
var LeftRecursionMemberCutSeeds = []int64{276}

// LeftRecursionMemberRecoveringSeeds are the seeds of RandomRecoveringCycleGrammar whose grammars
// gave a memoized member result that differs from the result without a memo when a member result
// that read a leader result with recovered errors was memoized.
var LeftRecursionMemberRecoveringSeeds = []int64{16472, 65734}

// LeftRecursionMemberSeedCases returns the grammars of LeftRecursionMemberSeeds,
// LeftRecursionMemberCutSeeds and LeftRecursionMemberRecoveringSeeds.
func LeftRecursionMemberSeedCases() []Case {
	var cases []Case
	for _, s := range LeftRecursionMemberSeeds {
		cases = append(cases, RandomCycleGrammar(s, false))
	}
	for _, s := range LeftRecursionMemberCutSeeds {
		cases = append(cases, RandomCycleGrammar(s, true))
	}
	for _, s := range LeftRecursionMemberRecoveringSeeds {
		cases = append(cases, RandomRecoveringCycleGrammar(s))
	}
	return cases
}

// LeftRecursionMemberDocumentSeeds are the seeds of RandomCycleGrammar (without cuts) whose
// parses store results that depend on the order in which the parse evaluated the calls: leaders
// grow inside the growth of another leader and read its seed. A later Document parse that reused
// such a result, without the growth it was computed in, gave another outcome than a parse from
// scratch.
var LeftRecursionMemberDocumentSeeds = []int64{117}

// LeftRecursionMemberDocumentRecoveringSeeds are such seeds of RandomRecoveringCycleGrammar, whose
// members also read leader results with recovered errors.
var LeftRecursionMemberDocumentRecoveringSeeds = []int64{5077}

// LeftRecursionMemberDocumentCases returns the grammars of LeftRecursionMemberDocumentSeeds and
// LeftRecursionMemberDocumentRecoveringSeeds.
func LeftRecursionMemberDocumentCases() []Case {
	var cases []Case
	for _, s := range LeftRecursionMemberDocumentSeeds {
		cases = append(cases, RandomCycleGrammar(s, false))
	}
	for _, s := range LeftRecursionMemberDocumentRecoveringSeeds {
		cases = append(cases, RandomRecoveringCycleGrammar(s))
	}
	return cases
}

// DependencyMemoControlSeeds are the seeds, of RandomCycleGrammar with cuts and of
// RandomRecoveringCycleGrammar, whose grammars give a different result when the dependency memo
// ignores the growths present at a call (state.CtxIgnorePresent): the first of each kind above the
// first 120 seeds, which the short tier searches. The short tier compares them in addition, so
// that the control of that check is deterministic; the full tier finds them in its search.
var DependencyMemoControlSeeds = struct{ Cuts, Recovering []int64 }{
	Cuts:       []int64{341},
	Recovering: []int64{267},
}

// WideContextCase returns the random cycle grammar 341 with cuts, padded with 63 leaders P1..P63
// that the cycle enters through r2, so that the padding takes the 63 bits of the engine's word of
// present bits and every other leader of the cycle has none. It parses "bc" to 1:3, but gives
// 1:2 where all wide present sets share one identity, as a result that evaluated a leader afresh
// then answers a call where that leader grows.
func WideContextCase() Case {
	c := RandomCycleGrammar(341, true)
	src := mustReplace(c.Src, "rule r2 = r4 / \"b\"", "rule r2 = r4 / \"b\" / P1 \"q\"")
	src = mustReplace(src, "rule main = r0", "rule main = P1")
	var b strings.Builder
	b.WriteString(src)
	for i := 1; i < 63; i++ {
		fmt.Fprintf(&b, "rule P%d = P%d \"z\" / P%d\n", i, i, i+1)
	}
	b.WriteString("rule P63 = P63 \"z\" / r0\n")
	c.Src = b.String()
	c.Name = "random cycle grammar 341 with cuts and 63 padding leaders"
	c.Inputs = append(c.Inputs, "bc", "bcb", "bcc")
	return c
}

// mustReplace replaces the first old in s with new, and panics if s has no old, so that a change of
// the grammar a case is derived from cannot silently drop what the case is about.
func mustReplace(s, old, new string) string {
	if !strings.Contains(s, old) {
		panic("codegentest: grammar has no " + strconv.Quote(old))
	}
	return strings.Replace(s, old, new, 1)
}

// DependencyMemoControlCases returns the grammars of DependencyMemoControlSeeds.
func DependencyMemoControlCases() []Case {
	cases := []Case{WideContextCase()}
	for _, s := range DependencyMemoControlSeeds.Cuts {
		cases = append(cases, RandomCycleGrammar(s, true))
	}
	for _, s := range DependencyMemoControlSeeds.Recovering {
		cases = append(cases, RandomRecoveringCycleGrammar(s))
	}
	return cases
}

// DependencyMemoCases returns the grammars that the tests of the dependency memo of left recursion
// compare: a grammar where one growth step of a leader at a position has two different results
// with the same end, in two growths of the leader within the growth of another leader there (B at
// 0 grows to ("a" "a")@B and, with the seed "a"@A of A, to ("a"@A "a")@B); a grammar whose leaders
// read each other's seeds inside each other's growths at one position; and, for each seed below
// seeds, random grammars whose cycles have several leaders and members, read inside and outside
// lookaheads, with and without cuts, and random grammars whose items recover from errors.
func DependencyMemoCases(seeds int) []Case {
	cases := []Case{{
		Name: "equal ends of different results",
		Src: `@start
rule main = A $$
rule A = C / "a" / C / "b" C C
rule B = B "b" / A "a" / C / "a" "a"
rule C = B A / &A "a" "c"`,
		Inputs: []string{"aaa", "aa", "aaab", "aac"},
	}, {
		Name: "leaders that read each other's seeds",
		Src: `@start
rule main = S $$
rule S = S "q" / R "s" / "y"
rule R = R "r" / S "t"`,
		Inputs: []string{"y", "yts", "ytsts", "ytrs", "ytsqtrrs", "yq", "yqts", "yt", "ytst", ""},
	}, {
		// The capture that M's lookahead makes holds the seed it read after the lookahead, so
		// the lookahead's outcome does not summarize it.
		Name: "capture in a lookahead",
		Src: `@start
rule main = S $$
rule S = M P S "q" / "y" ("qqqqqqqq")?
rule P = M ""
rule M = &(x:S) => $x
rule U = P P`,
		Inputs: []string{"y", "yq", "yqq", "yqqq", "yqqqq"},
	}, {
		// The variable that M's lookahead binds depends on the seed it read after the lookahead,
		// so the lookahead's outcome does not summarize it.
		Name: "binding in a lookahead",
		Src: `var v int
@start
rule main = S $$
rule S = M P S "q" / "y" ("qqqqqqqq")?
rule P = M ""
rule M = &(S ("qq" {v := 2} / "q" {v := 1} / {v := 0})) (({v == 2} => "two") / ({v == 1} => "one") / ("" => "zero"))
rule U = P P`,
		Inputs: []string{"y", "yq", "yqq", "yqqq", "yqqqq"},
	}, {
		// The error recovered in M's lookahead is at the end of the seed it read.
		Name: "error recovered in a lookahead",
		Src: `@start
rule main = S $$
rule S = M P S "q" / "y" ("qqqqqqqq")?
rule P = M ""
rule M = &(S (("!") #recover(skip=[q]))) ""
rule U = P P`,
		Inputs: []string{"y", "yq", "yqq", "yqqq", "yqqqq"},
	}, {
		// M's lookahead reads O's seed at L's seed "yz" only. The growth of L inside &L memoizes its
		// outcome there; the growth of L after it reuses M from L's seed "y", where the lookahead read
		// L's seed alone, and depends on O's seed only through the seeds that the memoized outcome
		// read.
		Name: "observation that reads another seed where it is reused",
		Src: `@start
rule main = O $$
rule O = &O &L L "o" / &O L "q" "o" "!" / "yz"
rule L = M L ("z" / "q") / "y"
rule M = &(L "z" / &L O "q")
rule U = M M`,
		Inputs: []string{"yzqo!", "yzqo", "yzq", "yz"},
	}, {
		// The lookahead &L of S reads the seed of S in some contexts of T only: a jump of S taken
		// from one seed holds only where the outcome of &L is the same for every seed of S.
		Name: "lookahead that reads the growing seed",
		Src: `@start
rule main = T $$
rule T = &S S "!" "z" / S "q" "!" "z" "#" / T "!" / "yqq"
rule S = &L S "q" / "y"
rule L = T "!" / &T S "q" "q"`,
		Inputs: []string{"yqq!z#", "yqq", "yqq!", "y"},
	}, {
		Name: "lookahead that reads the growing seed, longer alternative",
		Src: `@start
rule main = T $$
rule T = &S S "!" "z" / S "q" "!" "z" "#" / T "!" / "yqq!z#@" / "yqq"
rule S = &L S "q" / "y"
rule L = T "!" / &T S "q" "q"`,
		Inputs: []string{"yqq!z#", "yqq", "yqq!", "y"},
	}}
	// A recovering grammar whose jumps read the growing seed through a lookahead on this input.
	r := RandomRecoveringCycleGrammar(2964)
	r.Inputs = append(r.Inputs, "baabcac")
	cases = append(cases, r)
	// A context with more dependencies than the context store holds inline, and more leaders than
	// a word of present bits holds in generated parsers.
	cases = append(cases, Case{
		Name:   "seven dependencies and 33 leaders",
		Src:    "@start\nrule main = " + ContextChain(32, 7, false),
		Inputs: []string{"axy", "aaxx", "aaaxxx", "axq", "aaxzx", "a", ""},
	})
	// More leaders than a word of present bits holds in the engine too, whose present sets of them are
	// interned: rings of 66 leaders that the evaluation without memos finishes, which the chains
	// above, with their nested leaders, do not.
	for i, alt := range []string{`R%i "x" / &"y" R%n / "a"`, `R%i "x" / "a" R%n? / R%n "b"`} {
		cases = append(cases, Case{
			Name:   fmt.Sprintf("ring of 66 leaders %d", i+1),
			Src:    LeaderRing(66, alt),
			Inputs: []string{"a", "ax", "axb", "axxb", "", "xa", "ab", "yax", "ayx"},
		})
	}
	cases = append(cases, WideContextCase())
	for s := int64(0); s < int64(seeds); s++ {
		cases = append(cases, RandomCycleGrammar(s, false), RandomCycleGrammar(s, true), RandomRecoveringCycleGrammar(s))
	}
	return cases
}

// LeaderRing returns the source of a grammar whose start rule calls R1 and whose rules R1..Rn each
// have the body alt, in which %i stands for the rule and %n for the next one, the first after the
// last: a cycle of n rules.
func LeaderRing(n int, alt string) string {
	var b strings.Builder
	b.WriteString("@start\nrule main = R1\n")
	for i := 1; i <= n; i++ {
		next := i%n + 1
		fmt.Fprintf(&b, "rule R%d = %s\n", i, strings.NewReplacer("%i", strconv.Itoa(i), "%n", strconv.Itoa(next)).Replace(alt))
	}
	return b.String()
}

// ContextChain returns the rules after "rule main = " of a chain of leaders L1..Ln nested around
// L, whose every step calls B at the same position. B and G read the seeds of the first reads
// leaders and G's growth consumes the a's: on the input a^m x^m (ContextChainInput), a parse
// evaluates B at the start a number of times that does not grow with m, once the context store
// indexes the dependencies (reads of them) and the present sets (n+1 leaders) of these contexts.
// With missing, main also inserts a missing ";" at its end.
func ContextChain(n, reads int, missing bool) string {
	var b strings.Builder
	b.WriteString("L1")
	if missing {
		b.WriteString(` ";" #missing`)
	}
	b.WriteString(" $$\n")
	for i := 1; i <= n; i++ {
		next := fmt.Sprintf("L%d", i+1)
		if i == n {
			next = "L"
		}
		fmt.Fprintf(&b, "rule L%d = L%d \"z\" / %s\n", i, i, next)
	}
	var r strings.Builder
	for i := 1; i <= reads; i++ {
		fmt.Fprintf(&r, "(L%d \"q\")? ", i)
	}
	b.WriteString("rule L = B \"y\" / L \"x\" / B\n")
	b.WriteString("rule B = " + r.String() + "G\n")
	b.WriteString("rule G = " + r.String() + "(G \"a\" / \"a\")\n")
	return b.String()
}

// ContextChainInput is the input a^m x^m of ContextChain.
func ContextChainInput(m int) string {
	return strings.Repeat("a", m) + strings.Repeat("x", m)
}

// ContextStoreShapes are shapes of ContextChain whose contexts need more than the fixed-size keys
// of the context store: more dependencies than its inline values, more leaders than one word of
// present bits on each backend. Their parses evaluate a number of calls linear in the input but
// with a large constant, so the tests compare small inputs (ContextStoreSizes) and check no long
// input.
var ContextStoreShapes = []DependencyMemoShape{
	{"seven dependencies", ContextChain(7, 7, false), ContextChainInput},
	{"nine dependencies", ContextChain(9, 9, false), ContextChainInput},
	{"70 leaders", ContextChain(69, 1, false), ContextChainInput},
}

// ContextStoreSizes are the sizes m of the inputs of ContextStoreShapes that the tests compare: a
// parse of the larger evaluates at most ContextStoreGrowth times the calls of the smaller, where a
// context that the store cannot index makes the count quadratic in m.
var ContextStoreSizes = [2]int{10, 40}

// ContextStoreGrowth bounds the ratio of the evaluations at the two ContextStoreSizes.
const ContextStoreGrowth = 5

// ContextStoreBudgetPerUnit is the compute limit, for each unit of the input, under which generated
// parsers with limit controls parse ContextStoreShapes at ContextStoreSizes. Their work is linear in
// the input, but each unit costs tens of thousands of units of compute, beyond the default budget:
// nine dependencies cost about 26,000 for each unit of a large input, and up to 44,000 for each
// unit at the smaller size, where the work at the start of the input is spread over fewer units.
const ContextStoreBudgetPerUnit = 1 << 16

// DependencyMemoShape is a grammar whose leaders grow inside the growth of another leader at the
// same position, with an input family whose parse evaluates a number of calls linear in its length.
// Rules holds the rules after "rule main = ".
type DependencyMemoShape struct {
	Name, Rules string
	Input       func(n int) string
}

// DependencyMemoShapes are the shapes that the linearity tests of the dependency memo check.
var DependencyMemoShapes = []DependencyMemoShape{
	{"lookahead regrowth", `S $$
rule S = &R S "q" / "y"
rule R = R ("q" / "r") / S "t" / "y"`, func(n int) string { return "y" + strings.Repeat("q", n) }},
	{"nested reads", `S $$
rule S = S "q" / R "s" / "y"
rule R = R "r" / S "t"`, func(n int) string { return "y" + strings.Repeat("ts", n) }},
	{"mutual", `A $$
rule A = A "a" / B "b" / "x"
rule B = B "c" / A "d" / "y"`, func(n int) string { return "x" + strings.Repeat("dcb", n) }},
	{"every step reads a success", `S $$
rule S = &R S "q" / "y"
rule R = &S R "q" / "y"`, func(n int) string { return "y" + strings.Repeat("q", n) }},
	{"every step reads an end", `S $$
rule S = &R S "q" / "y"
rule R = &(S "q") R "q" / "y"`, func(n int) string { return "y" + strings.Repeat("q", n) }},
	{"three leaders", `S $$
rule S = &R S "q" / "y"
rule R = &T R "q" / "y"
rule T = T "q" / S "z" / "y"`, func(n int) string { return "y" + strings.Repeat("q", n) }},
}
