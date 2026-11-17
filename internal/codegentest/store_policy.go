package codegentest

import (
	"fmt"
	"slices"
	"strings"
)

// StorePolicyCase is a grammar whose memoized rule R is called, phase by phase, several times at one
// position per phase, from different places, in different lookahead states and with different keys
// (the values of a variable that R reads, or a binding level), and a model of the body evaluations
// of R that the store policy of "Evaluations of Rules" in spec/runtime/parsing.md allows: of the
// evaluations of a memoized rule at a position outside every context of growths, only the first
// may leave no result in the memo, whatever its binding level, the values of its variables and its
// lookahead state, and the same holds of the evaluations of its twin; every later one stores its
// result.
//
// Each phase is a choice at its own position whose alternatives call R and fail, one alternative
// for each letter of the phase, and whose last alternative calls R once more and consumes. The
// letters are O (outside a lookahead, key 0), I (inside a lookahead, key 0), o and i (the same,
// key 1). The calls of the letters discard their value, so they call the value-free twin of R in a
// parse; the last call uses its value, so it calls R itself.
type StorePolicyCase struct {
	Name string
	// Src, Input and Start are the grammar, an input that it accepts, and its start rule.
	Src, Input, Start string
	// Rule is the rule R, as a profile names it.
	Rule string
	// Phases are the letters of each phase, which is at the position after the previous one.
	Phases []string
	// Twins reports that a parse gives R a value-free twin (a recognition never does).
	Twins bool
	// Stream is the case for a stream: its phases are the elements of a #stream repetition.
	Stream bool
}

// StorePolicyKeys is how the second key of a case differs from the first.
type StorePolicyKeys int

const (
	// KeysEnv: key 1 binds the variable x that R reads to 1, key 0 leaves its initial value.
	KeysEnv StorePolicyKeys = iota
	// KeysLevel: key 1 calls the Operators Rule R at the level mul, key 0 at the level add.
	KeysLevel
)

// StorePolicyCases returns the cases of the store policy.
func StorePolicyCases() []StorePolicyCase {
	type form struct {
		name   string
		phases []string
	}
	// The forms hold the rule's first-call deferral and the model's estimator to the backends:
	// eager mode after a repeated call, and no eager mode where no position has a repeated call.
	forms := []form{
		{"alternating", []string{"OIOIOIOI"}},
		{"inside first", []string{"IOIOIOIO"}},
		{"outside only", []string{"OOOO"}},
		{"inside only", []string{"IIII"}},
		{"one twin call", []string{"O"}},
		{"one inside call", []string{"I"}},
		{"keys alternating", []string{"OoOo"}},
		{"keys inside", []string{"iIiI"}},
		{"keys mixed", []string{"oIiOoi"}},
		{"keys outside last", []string{"iiOo"}},
		{"phases without repeats", []string{"O", "I", "O", "I", "o", "i"}},
		{"eager by an earlier position", []string{"OO", "IOIO"}},
		{"eager by an earlier position, keys", []string{"oo", "iOoI"}},
		{"eager by an earlier position, inside", []string{"II", "IIOI"}},
		{"deferred at every position", []string{"IOI", "IOI", "OIO", "oio"}},
		{"not eager after many single calls", append(slices.Repeat([]string{"O"}, 40), "OIO", "OIO")},
	}
	var cases []StorePolicyCase
	for _, f := range forms {
		cases = append(cases, storePolicyCase(f.name, f.phases, KeysEnv, false, false))
	}
	// An Operators Rule has an action, so it has no twin in a parse: every call is of the one rule.
	for _, f := range forms {
		if strings.ContainsAny(strings.Join(f.phases, ""), "oi") {
			cases = append(cases, storePolicyCase(f.name+", levels", f.phases, KeysLevel, false, false))
		}
	}
	// A grammar of more than 64 memoized rules and an input of more than 1024 units selects the
	// sparse seen table: the policy holds there as it does in the dense one.
	for _, f := range forms[:6] {
		cases = append(cases, storePolicyCase(f.name+", sparse", f.phases, KeysEnv, true, false))
	}
	cases = append(cases, storePolicyCase("keys mixed, sparse", []string{"OO", "oIiOoi"}, KeysEnv, true, false))
	// A stream has the same phase in each element.
	for _, f := range forms[:4] {
		cases = append(cases, storePolicyCase(f.name+", stream", slices.Repeat(f.phases, 3), KeysEnv, false, true))
	}
	cases = append(cases, storePolicyCase("keys mixed, stream", slices.Repeat([]string{"oIiOoi"}, 3), KeysEnv, false, true))
	return cases
}

func storePolicyCase(name string, phases []string, keys StorePolicyKeys, sparse, stream bool) StorePolicyCase {
	hasKey1 := strings.ContainsAny(strings.Join(phases, ""), "oi")
	var b strings.Builder
	var input string
	rule := "a"
	twins := true
	switch keys {
	case KeysEnv:
		if hasKey1 {
			b.WriteString("var x int\n")
		}
	case KeysLevel:
		rule, twins = "e", false
		b.WriteString("type Atom = { Text string }\ntype Bin = { L Expr; Op node[string]; R Expr }\ntype Expr ~ Atom | Bin\n")
	}
	call := func(letter rune, lit string) string {
		inside := letter == 'I' || letter == 'i'
		key1 := letter == 'o' || letter == 'i'
		var c string
		switch keys {
		case KeysEnv:
			c = "a"
			if key1 {
				c = "{x := 1} " + c
			}
		case KeysLevel:
			c = "level(e, add)"
			if key1 {
				c = "level(e, mul)"
			}
		}
		if inside {
			return fmt.Sprintf("&(%s %q)", c, lit)
		}
		if keys == KeysEnv && key1 {
			return fmt.Sprintf("{x := 1} -a %q", lit)
		}
		return fmt.Sprintf("-%s %q", c, lit)
	}
	last, unit := "a", "xb"
	if keys == KeysLevel {
		last, unit = "level(e, add) \",\"", "1,"
	}
	var seq []string
	for i, ph := range phases {
		if stream && i > 0 {
			input += unit
			continue
		}
		var alts []string
		for j, l := range ph {
			alts = append(alts, call(l, fmt.Sprintf("k%d_%d", i, j)))
		}
		alts = append(alts, last)
		fmt.Fprintf(&b, "rule ph%d = %s\n", i, strings.Join(alts, " / "))
		seq = append(seq, fmt.Sprintf("ph%d", i))
		input += unit
	}
	switch {
	case stream:
		// Each element is the phase, at its own position. The guard keeps the repetition's last
		// attempt, at the end of the input, from calling the rule.
		fmt.Fprintf(&b, "@start\nrule s = items:el* #stream $$\nrule el = &%q ph0\n", unit[:1])
	default:
		tail := ""
		if sparse {
			tail = " tail"
		}
		fmt.Fprintf(&b, "@start\nrule s = %s%s\n", strings.Join(seq, " "), tail)
	}
	switch keys {
	case KeysEnv:
		pred := ""
		if hasKey1 {
			pred = "{x >= 0} "
		}
		fmt.Fprintf(&b, "rule a = %s\"x\" b\nrule b = \"b\"\n", pred)
	case KeysLevel:
		b.WriteString("rule num Atom = t:([0-9]+) #atomic => Atom{Text: text($t)}\n" +
			"rule e Expr = #operators {\n  operand num\n" +
			"  level add { infix left \"+\" => Bin{L: $lhs, Op: $op, R: $rhs} }\n" +
			"  level mul { infix left \"*\" => Bin{L: $lhs, Op: $op, R: $rhs} }\n}\n")
	}
	if sparse {
		// Memoized rules beyond the 64 that make the table sparse: each is called from two places.
		b.WriteString("rule tail = \"z\"*\n")
		var pads []string
		for i := range 70 {
			pads = append(pads, fmt.Sprintf("pad%d", i))
		}
		fmt.Fprintf(&b, "rule dead = %s\n", strings.Join(append(pads, pads...), " "))
		for i := range 70 {
			fmt.Fprintf(&b, "rule pad%d = b%d \"x\"\nrule b%d = \"b\"\n", i, i, i)
		}
		input += strings.Repeat("z", 1100)
	}
	if sparse && !strings.Contains(b.String(), "rule s = ") {
		panic("no start rule")
	}
	if sparse {
		// dead is reached from s only when the phases fail.
		src := b.String()
		src = strings.Replace(src, "tail\n", "tail / dead\n", 1)
		b.Reset()
		b.WriteString(src)
	}
	return StorePolicyCase{Name: name, Src: b.String(), Input: input, Start: "s", Rule: rule, Phases: phases, Twins: twins, Stream: stream}
}

type storeCall struct {
	twin, inside bool
	key          int
}

// StorePolicyEvals returns the body evaluations of the rule that the policy allows when the first
// evaluation at a position leaves no result in the memo (deferred) and when it stores (a rule
// that memoizes every call, as the eager mode and a stream do); parse says whether the parse
// builds values, so that the rule has a twin where the case has one. Any evaluation count between
// them is a mix of the two for different rules or positions. In a parse that defers, the count is
// exact where the model of the deferral below is.
func (c StorePolicyCase) StorePolicyEvals(parse bool) (deferred, stored int) {
	return c.simulate(parse, true), c.simulate(parse, false)
}

// simulate runs the model. A rule defers its first call at each position until the repeated calls
// that it has seen (a call at a position that an earlier call there saw) exceed a sixteenth of all
// its calls, and then stores every call (the adaptive switch of the reference implementation,
// whose effect on the first call a case with repeated calls shows).
func (c StorePolicyCase) simulate(parse, deferFirst bool) int {
	type variant struct {
		calls, repeats int
		eager          bool
	}
	var vs [2]variant
	evals := 0
	for _, ph := range c.Phases {
		var calls []storeCall
		for _, l := range ph {
			key := 0
			if l == 'o' || l == 'i' {
				key = 1
			}
			calls = append(calls, storeCall{twin: parse && c.Twins, inside: l == 'I' || l == 'i', key: key})
		}
		calls = append(calls, storeCall{})
		seen := [2]bool{}
		var stored [2][]storeCall
		for _, call := range calls {
			v := 0
			if call.twin {
				v = 1
			}
			st := &vs[v]
			if deferFirst && !st.eager {
				st.calls++
				if !seen[v] {
					seen[v] = true
					evals++
					continue
				}
				st.repeats++
				if st.repeats > st.calls>>4 {
					st.eager = true
				}
			}
			covered := false
			for _, e := range stored[v] {
				// A result computed outside a lookahead answers every call, one computed inside only
				// the calls inside.
				if e.key == call.key && (!e.inside || call.inside) {
					covered = true
				}
			}
			if !covered {
				evals++
				stored[v] = append(stored[v], call)
			}
		}
	}
	return evals
}
