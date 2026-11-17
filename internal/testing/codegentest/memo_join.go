package codegentest

import (
	"fmt"
	"strings"
)

// MemoJoinCase is a family of grammars and inputs whose memoized rule k is called at one position
// outside every context of growths and in contexts of growths of its cycle, inside and outside
// lookaheads, and with different leaders growing. A store never narrows the calls that the entry of
// a key answers ("Evaluations of Rules" in spec/runtime/parsing.md): an entry computed inside a
// lookahead does not replace one computed outside, and an entry stored in a context keeps the
// leaders of the entry it replaces. So the evaluations of k at the position are the same in every
// run of the family, however many calls of k it makes. A store that replaced the entry evaluated k
// again after the lookahead, and in each step of the sibling growths.
type MemoJoinCase struct {
	Name string
	// Runs are the grammars and inputs of the family, each making more calls of k than the one
	// before.
	Runs []MemoJoinRun
	// Evals and Recognized are the body evaluations of k (with its value-free twin) in a parse and
	// in a recognition of each run.
	Evals, Recognized int
}

// MemoJoinRun is a grammar, with start rule s, and an input that it accepts.
type MemoJoinRun struct{ Src, Input string }

// MemoJoinCases returns the MemoJoinCase families.
func MemoJoinCases() []MemoJoinCase {
	// The leader l grows at the position and calls k inside a lookahead there; the calls of k
	// outside l alternate with growths of l, inside and outside lookaheads. The result of k stored
	// inside the lookahead in l's context replaced the entry computed outside every lookahead, so
	// the next call of k outside l evaluated it again.
	lookahead := func(phases int) MemoJoinRun {
		alts := []string{`k "1"`, `&(l "z") "w"`}
		for i := range phases {
			alts = append(alts, fmt.Sprintf(`k "%d"`, 2*i+2), fmt.Sprintf(`l "%d"`, 2*i+3))
		}
		alts = append(alts, `k "."`)
		return MemoJoinRun{"@start\nrule s = " + strings.Join(alts, " / ") + "\nrule l = l \"x\" / &(k) \"a\" / \"c\"\nrule k = \"a\" / l \"k\"\n", "a."}
	}
	// Each step of the growth of a grows b or c at the position, whose sets of growing leaders are
	// not comparable, and both call k there. A store in one replaced the entry stored in the other,
	// so k was evaluated in each step.
	const siblings = "@start\nrule s = a \"#\"\nrule a = b \"x\" / c \"y\" / \"a\"\n" +
		"rule b = k \"kb\" / k \"ke\" / b \"q\" / a \"b\"\nrule c = k \"kc\" / k \"kd\" / c \"r\" / a \"c\"\n" +
		"rule k = \"a\" [^#]* / a \"z\"\n"
	sib := func(m int) MemoJoinRun { return MemoJoinRun{siblings, "a" + strings.Repeat("cy", m) + "#"} }
	return []MemoJoinCase{
		{Name: "lookahead in a growth", Runs: []MemoJoinRun{lookahead(1), lookahead(3), lookahead(6)}, Evals: 3, Recognized: 3},
		{Name: "sibling growths", Runs: []MemoJoinRun{sib(5), sib(20), sib(40)}, Evals: 3, Recognized: 3},
	}
}
