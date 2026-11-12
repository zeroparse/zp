package gentest

import (
	"hash/fnv"
	"strings"
	"testing"
)

// Representative returns the cases that a test of the default tier runs when running all of them
// is too slow to repeat on every change; the long tier (go test -tags zp_long) runs every case.
//
// The selection is fixed and does not depend on the order of the corpus or on a random seed:
//   - the first case of every family is kept, where the family of a case is its name up to the
//     first ':' ("inline action: sequence"), or, for a path, up to the first '_' of its last element
//     ("typed/LR_direct") or else up to its last '/' ("testdata/typed/mix.zp"); a name with neither has no family. So
//     every family that the corpus names (inline actions, hidden rules, typed cuts, left
//     recursion, Operators Rule, recovery, locals, attribute shapes, the typed testdata) is
//     exercised at least once;
//   - every other case, and the cases without a family, are kept if the FNV-1a hash of its name is divisible by eight, an eighth of
//     them, so that adding or renaming a case changes the selection of that case only.
//
// Cases named in alwaysRepresentative are kept as well.
func Representative(cases []Case) []Case {
	if allCases {
		return cases
	}
	seen := map[string]bool{}
	var out []Case
	for _, c := range cases {
		fam := caseFamily(c.Name)
		first := fam != "" && !seen[fam]
		seen[fam] = true
		if first || hashOf(c.Name)%8 == 0 || alwaysRepresentative[c.Name] {
			out = append(out, c)
		}
	}
	return out
}

// alwaysRepresentative names the cases that the default tier keeps whatever their hash.
var alwaysRepresentative = map[string]bool{
	// A generated parser that nests without end on it crashes instead of failing one case.
	"empty operand and right infix": true,
}

func caseFamily(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[:i]
	}
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		if j := strings.IndexByte(name[i:], '_'); j >= 0 {
			return name[:i+j]
		}
		return name[:i]
	}
	return ""
}

func hashOf(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// RepresentativeNames returns the names of the checked-in grammars that the default tier runs of
// names, the same four as RepresentativeGrammars in the order of names; the long tier runs all.
func RepresentativeNames(names []string) []string {
	if allCases {
		return names
	}
	var out []string
	for _, n := range names {
		switch n {
		case "json", "csv", "cel", "cue":
			out = append(out, n)
		}
	}
	return out
}

// RepresentativeGrammars returns the checked-in grammars of GrammarCases that the default tier runs,
// the small ones (json, csv, cel, cue: each with an Operators Rule or a repetition-heavy body of its
// own); the long tier runs all ten.
func RepresentativeGrammars(cases []Case) []Case {
	if allCases {
		return cases
	}
	var out []Case
	for _, c := range cases {
		switch c.Name {
		case "json", "csv", "cel", "cue":
			out = append(out, c)
		}
	}
	return out
}

// ScaleFloor returns the lower bound that a test of the default tier requires of a count that it
// requires to be at least n over the whole corpus: n in the long tier, and an eighth of n (at least
// one) when the test runs Representative cases only, which are about a fifth of the corpus.
func ScaleFloor(n int) int {
	if allCases {
		return n
	}
	return max(1, n/8)
}

// LongOnly skips the test or subtest in the default tier, for a case that repeats what a smaller
// case of the same test already shows and costs much more (a large checked-in grammar built next to
// a small one); the long tier (go test -tags zp_long) runs it.
func LongOnly(t testing.TB, why string) {
	t.Helper()
	if !allCases {
		t.Skip("long tier only: " + why)
	}
}

// Sampled reports whether a test of the default tier runs the fixture of the name, for a test that
// builds a list of fixtures that each show one feature and share no case family: the long tier
// (go test -tags zp_long) runs every fixture, and the default tier those whose name has an FNV-1a hash
// divisible by three, a third of them, so that adding or renaming a fixture changes its own selection only.
func Sampled(name string) bool {
	return allCases || hashOf(name)%3 == 0
}

// Long reports whether the long tier (go test -tags zp_long) is running.
func Long() bool { return allCases }

// SampledShapes returns the dependency-memo shapes that the default tier checks: the ones that
// Sampled keeps, or all of them in the long tier.
func SampledShapes() []DependencyMemoShape {
	var out []DependencyMemoShape
	for _, s := range DependencyMemoShapes {
		if Sampled(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

// SkipUnlessSampled skips the subtest in the default tier unless Sampled keeps its name.
func SkipUnlessSampled(t testing.TB) {
	t.Helper()
	if !Sampled(t.Name()) {
		t.Skip("long tier only: not in the sampled third")
	}
}

// OneForm reports whether a test that loops over the two forms of the generated code (with and
// without budget controls) runs the form in the default tier: the one that the hash of the test
// name picks, as EachMode does; the long tier runs both.
func OneForm(t testing.TB, controls bool) bool {
	if allCases {
		return true
	}
	return (hashOf(t.Name())%2 == 0) == controls
}
