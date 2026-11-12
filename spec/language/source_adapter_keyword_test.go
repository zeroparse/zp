package language_test

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp"
)

// keywordNameCorpus are grammars that use rule, type and var as names: fields, rule references and
// variables. They are valid, and a line or a statement that starts with one of the words is not a
// declaration. The mutation tests take them with the other grammars.
var keywordNameCorpus = []string{
	"type T = {\n type string\n var int\n rule Foo\n}\n",
	"rule a = {\n var := 1\n y\n}\n",
	"rule a = x:\"a\" {\n type := 1\n}\n",
	"rule a = (\n rule b\n c d )\n",
	"rule a = {\n var / t\n y\n y\n y\n y\n y\n y\n y\n y\n y\n}\n",
	"rule a = {\n x := 1; var / t\n y\n}\n",
	"rule a = {\n rule := 1; type := 2; var := 3\n z\n}\n",
	"type A = { type string }; type B ~ A\n",
	"type A = { var int; rule Foo }; type B ~ A\n",
	"rule a = { x := 1; var / t }; rule b = a\n",
	"rule a = { type := 1 }; rule b = { var := 2 }\n",
	"type T = {\n type string /* = \"a\" */\n}\n",
	"type T = {\n var int /* default = 1 */\n name string\n}\n",
	"rule a = {\n var != \"=\"\n}\n",
	"rule a = {\n type + \"=\" == \"x\"\n}\n",
	"rule a = x:\"a\" {\n type + \"=\" != \"\"\n}\n",
	"rule a = b => {\n var + \"=\"\n}\n",
	"rule a = {\n var > 0 /* = */\n}\n",
	"rule a = (\n var \"=\" value\n)\n",
	"rule a = (\n type name \"=\" value\n)\n",
	"rule a = (\n rule [=~]\n)\n",
	"rule a = (\n var \"~\" x\n)\n",
	"rule a = (\n var /* = */ x\n)\n",
	"rule a = b @error(\n type = \"x\"\n)\n",
	"@x(\n type = \"x\"\n)\nrule a = b\n",
	"rule a = (\n b\n @var\n c\n)\n",
	"rule a = (\n b\n @rule\n c\n)\n",
	"rule a = (\n b\n @error(message=\"x\") rule\n c\n)\n",
	"type A = node[\n type\n]\n",
	"rule a = b => x[\n type\n]\n",
	"rule a = b\nrule b = {\n type := 1\n var := 2\n type / var\n}\n",
}

// TestKeywordNamesAreNotDeclarations requires the grammars of keywordNameCorpus to be accepted, and a
// mistake after them to be reported where the reference reports it, with no error in the valid text
// before it: the recovery after a mistake must not read that text as declarations.
func TestKeywordNamesAreNotDeclarations(t *testing.T) {
	junk := []string{"/ /\n", "/ / v\n", ") )\n", " ?? \n"}
	for _, src := range keywordNameCorpus {
		if _, err := zp.ParseGrammar(src); err != nil {
			t.Fatalf("%q is not valid: %v", src, err)
		}
		if _, err := adaptCanonical(src); err != nil {
			t.Errorf("%q: the canonical frontend rejects it: %v", src, err)
		}
		compareGrammars(t, "keyword names", src)
		// The junk follows the last closing delimiter, so that everything before it is valid.
		end := len(src) - 1
		for end > 0 && src[end] != '}' && src[end] != ')' && src[end] != ']' {
			end--
		}
		for _, j := range junk {
			bad := src[:end+1] + j
			_, merr := zp.ParseGrammar(bad)
			_, err := adaptCanonical(bad)
			if merr == nil || err == nil {
				continue // the junk made the text valid
			}
			// The reference may report the valid text before the junk as well (it reads a line that
			// starts with rule as a declaration when a later mistake makes it recover); the canonical
			// parser must not, and must report the junk where the reference does.
			var h []errorAt
			for _, e := range errorsOf(bad, merr) {
				if e.off > end {
					h = append(h, e)
				}
			}
			g := errorsOf(bad, err)
			if len(h) == 0 {
				continue
			}
			if fg, fh := first(g), first(h); fg.off != fh.off {
				t.Errorf("%q: the first error is at %d, want %d\n  hand-written: %v\n  canonical:    %v", bad, fg.off, fh.off, merr, err)
			}
		}
	}
	for _, c := range []string{
		"type T = {\n type string\n var int\n rule Foo\n}/ /\n",
		"rule a = {\n var := 1\n y\n}/ / v",
		"rule a = x:\"a\" {\n type := 1\n}/ / v",
		"rule a = (\n rule b\n c d ) )\n",
		"rule a = {\n var / t\n y\n y\n y\n y\n y\n y\n y\n y\n y\n y\n}/ / v\n",
	} {
		_, merr := zp.ParseGrammar(c)
		_, err := adaptCanonical(c)
		if merr == nil || err == nil {
			t.Fatalf("%q must be rejected by both frontends: %v, %v", c, merr, err)
		}
		if fg, fh := first(errorsOf(c, err)), first(errorsOf(c, merr)); fg.off != fh.off {
			t.Errorf("%q: the first error is at %d, want %d\n  hand-written: %v\n  canonical:    %v", c, fg.off, fh.off, merr, err)
		}
	}
}

// keywordStatements are action statements and predicates that start with a keyword, and keywordGroupLines
// the lines of a group that do; keywordFields are struct fields. All are valid when the keyword is a name,
// and each has a character that a declaration has (an =, a ~ or a bracket) where a scan for the shape of
// a declaration may take it: in a string, a class, a comment or a longer operator.
var (
	keywordStatements = []string{
		"%s := 1", "%s := \"=\"", "%s != \"=\"", "%s + \"=\" == \"x\"", "%s > 0 /* = */", "%s == 1 && y",
		"%s <= 2", "%s >= 2", "x := 1; %s / t", "%s := 1; %s := 2", "%s / t", "%s => y", "%s // = 1",
	}
	keywordGroupLines = []string{
		"%s", "%s \"=\" value", "%s [=~]", "%s /* = */ x", "%s \"~\" x", "%s / %s", "@error(message=\"x\") %s",
		"@%s", "%s?", "%s*", "%s // a = b", "%s [a-z]", "%s \"(\" x",
	}
	keywordFields = []string{
		"%s string", "%s []int", "%s int /* default = 1 */", "%s *T // = x", "%s node[string] /* ~ */", "%s T; x int",
	}
)

// keywordGrammar returns a grammar of one to three declarations whose bodies have lines that start with
// rule, type or var as a name. It is not always valid; the caller checks.
func keywordGrammar(rng *rand.Rand) string {
	kw := func() string { return []string{"rule", "type", "var"}[rng.Intn(3)] }
	line := func(forms []string) string {
		f := forms[rng.Intn(len(forms))]
		return strings.ReplaceAll(f, "%s", kw())
	}
	var b strings.Builder
	for i := 0; i < 1+rng.Intn(3); i++ {
		switch rng.Intn(7) {
		case 0:
			fmt.Fprintf(&b, "rule a%d = {\n %s\n %s\n}\n", i, line(keywordStatements), line(keywordStatements))
		case 1:
			fmt.Fprintf(&b, "rule a%d = x:\"a\" {\n %s\n}\n", i, line(keywordStatements))
		case 2:
			fmt.Fprintf(&b, "rule a%d = b => {\n %s\n}\n", i, line(keywordStatements))
		case 3:
			fmt.Fprintf(&b, "rule a%d = (\n %s\n y\n %s\n)\n", i, line(keywordGroupLines), line(keywordGroupLines))
		case 4:
			fmt.Fprintf(&b, "type T%d = {\n %s\n %s\n}\n", i, line(keywordFields), line(keywordFields))
		case 5:
			fmt.Fprintf(&b, "@x(\n %s = \"x\"\n)\nrule a%d = b\n", kw(), i)
		default:
			fmt.Fprintf(&b, "rule a%d = b => x[\n %s\n]; type A%d = node[\n %s\n]\n", i, kw(), i, kw())
		}
	}
	return b.String()
}

// keywordFormCorpus returns n valid grammars of keywordGrammar, the first of a seed; the mutation corpus
// takes them so that the edits land on lines that start with a keyword.
func keywordFormCorpus(n int) []string {
	rng := rand.New(rand.NewSource(97))
	var out []string
	for len(out) < n {
		if g := keywordGrammar(rng); isValidGrammar(g) {
			out = append(out, g)
		}
	}
	return out
}

func isValidGrammar(src string) bool {
	_, err := zp.ParseGrammar(src)
	return err == nil
}

// TestKeywordFormsAreNotDeclarations requires the canonical frontend to accept the generated grammars
// that the reference accepts, with the same grammar, and, when a broken declaration comes before or after
// one of them, to report errors only in the broken declaration: the recovery must not read the lines
// that start with a keyword as declarations or find a bracket in a string.
func TestKeywordFormsAreNotDeclarations(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	broken := []string{"rule zz = )\n", "rule zz \"y\" )\n", "type ZZ = }\n", "rule zz = ( b\n", "var zz = ]\n", "@x(a=1)\nrule zz = )\n"}
	valid := 0
	for i := 0; i < mutationCases/5; i++ {
		g := keywordGrammar(rng)
		if i%4 == 3 {
			g = strings.ReplaceAll(g, "\n", "\r\n")
		}
		if !isValidGrammar(g) {
			continue
		}
		valid++
		if !compareGrammars(t, "keyword forms", g) {
			continue
		}
		j := broken[rng.Intn(len(broken))]
		type brokenAt struct {
			src    string
			lo, hi int
		}
		// A group that is not closed takes what follows it, and the error is where it fails there.
		cases := []brokenAt{{g + j, len(g), len(g) + len(j)}}
		if !strings.Contains(j, "( b") {
			cases = append(cases, brokenAt{j + g, 0, len(j)})
		}
		for _, bad := range cases {
			_, err := adaptCanonical(bad.src)
			if err == nil {
				continue // the broken declaration made the text valid
			}
			for _, e := range errorsOf(bad.src, err) {
				if e.off < bad.lo || e.off > bad.hi {
					t.Errorf("%q: the canonical frontend reports an error at %d, outside %d..%d: %v", bad.src, e.off, bad.lo, bad.hi, err)
					break
				}
			}
		}
	}
	if valid < mutationCases/5/2 {
		t.Fatalf("only %d of %d generated grammars are valid", valid, mutationCases/5)
	}
}

// TestUnterminatedLiteralIsOneError requires an unterminated string or class to be one error, the lexical
// one, and the declaration after it, which is valid, to have none. The grammar fails at the end of the
// literal, which is the same error; or, if the literal holds an opening bracket that no later line closes,
// or swallows the closing bracket of its group, at the declaration after it, where the hand-written parser
// reports nothing more.
func TestUnterminatedLiteralIsOneError(t *testing.T) {
	for _, src := range []string{
		"rule a = \"x\nrule b = \"y\"\n",
		"rule a = [a-z\nrule b = \"y\"\n",
		"rule a = (\n \"x\n y\n)\nrule b = \"y\"\n",
		"type T = {\n A string\n B int // \"\n}\nrule a = \"x\nrule b = \"y\"\n",
		"rule a = \"x\r\nrule b = \"y\"\r\n",
		"rule a = \"x",
		// An opening bracket in the literal opens a group that only the next declaration ends.
		"rule a0 = {\n var != \"={\n}\nrule a1 = b\n",
		"rule a0 = x:\"a\" {\n var != \"={\n}\nrule a1 = b\n",
		"rule a0 = {\n \"={\n}\nrule a1 = b\n",
		"rule a0 = b {\n x != \"={\n}\nrule a1 = b\n",
		"rule a0 = {\n var != [={\n}\nrule a1 = b\n",
		"rule a0 = {\n [x{\n}\nrule a1 = b\n",
		"type T = {\n A \"{\n}\nrule a1 = b\n",
		"rule a = #operators {\n skip \"{\n}\nrule a1 = b\n",
		// The literal swallows the closing bracket: the group ends at the next declaration, silently.
		"type T = { A \"x }\nrule b = c\n",
		"type T = { A int; B \"x }\nrule b = c\n",
		"type T = { A \"x\nrule b = c\n",
		"type T = { A [x }\nrule b = c\n",
		"rule a = {\n x \"}\nrule b = c\n",
		// An indented line with a keyword in the group is not a declaration.
		"rule a0 = {\n var != \"={\n var != \"=\"\n}\nrule a1 = x:\"a\" {\n type := \"=\"\n}\n",
	} {
		_, merr := zp.ParseGrammar(src)
		_, err := adaptCanonical(src)
		if merr == nil || err == nil {
			t.Fatalf("%q must be rejected by both frontends: %v, %v", src, merr, err)
		}
		h, g := errorsOf(src, merr), errorsOf(src, err)
		if len(g) != 1 || len(h) != 1 || g[0].off != h[0].off {
			t.Errorf("%q: want the one error of the reference\n  hand-written: %v\n  canonical:    %v", src, merr, err)
		}
	}
}

// TestUnterminatedLiteralKeepsTheNextError requires an error that is the report of the declaration after an
// unterminated literal to stay: a failure at an attribute without a name is not the closing bracket that
// the literal swallowed.
func TestUnterminatedLiteralKeepsTheNextError(t *testing.T) {
	for _, src := range []string{
		"rule a = \"x\n@\nrule b = c\n",
		"rule a = [x\n@\nrule b = c\n",
		"type T = { A \"x\n@\nrule b = c\n",
	} {
		_, merr := zp.ParseGrammar(src)
		_, err := adaptCanonical(src)
		if merr == nil || err == nil {
			t.Fatalf("%q must be rejected by both frontends: %v, %v", src, merr, err)
		}
		h, g := sortedOffsets(errorsOf(src, merr)), sortedOffsets(errorsOf(src, err))
		if !slices.Equal(h, g) || len(h) != 2 {
			t.Errorf("%q: want the two errors of the reference\n  hand-written: %v\n  canonical:    %v", src, merr, err)
		}
	}
}

// TestUnterminatedLiteralBeforeIndentedDeclaration requires the errors that follow an unterminated literal
// and an indented declaration to stay. The language allows an indented declaration, and the hand-written
// parser ends a group at it; the drop of the failure after the literal must not take the errors of that
// declaration for the literal's.
func TestUnterminatedLiteralBeforeIndentedDeclaration(t *testing.T) {
	for _, src := range []string{
		"rule a = \"x\n  rule b = {\nrule c = d\n",
		"rule a = {\n \"={\n  rule b = {\nrule e = c\n",
		"rule a = {\n \"={\n  rule b = {\n  rule e = c\n",
		"rule a = {\n \"={\n  rule b = {\nrule c = )\n",
		"rule a = \"x\n  rule b = c\nrule d = )\n",
	} {
		_, merr := zp.ParseGrammar(src)
		_, err := adaptCanonical(src)
		if merr == nil || err == nil {
			t.Fatalf("%q must be rejected by both frontends: %v, %v", src, merr, err)
		}
		h, g := errorsOf(src, merr), errorsOf(src, err)
		if len(h) != len(g) {
			t.Errorf("%q: want as many errors as the reference\n  hand-written: %v\n  canonical:    %v", src, merr, err)
		}
	}
}

// TestCanonicalErrorsAreInTheOrderOfTheSource requires the errors after the lexical ones to be in the order of
// the source, as the hand-written parser reports them. The grammar lists the failure that ends the parse
// before the mistakes it recovered from.
func TestCanonicalErrorsAreInTheOrderOfTheSource(t *testing.T) {
	for _, src := range []string{
		"rule a = {\n x \"{\n var = 1\n}\nrule c = d\n",
		"rule a = {\n x \"{\n}\n  rule b; rule c = d\n",
		"rule a = )\nrule b = )\nrule c = {\n  x ) \n}\n",
	} {
		_, err := adaptCanonical(src)
		errs := errorsOf(src, err)
		if len(errs) < 2 {
			t.Fatalf("%q: want at least two errors: %v", src, err)
		}
		for i := 1; i < len(errs); i++ {
			if errs[i].off < errs[i-1].off {
				t.Errorf("%q: error %d at %d comes after the one at %d\n  %v", src, i, errs[i].off, errs[i-1].off, err)
				break
			}
		}
	}
}
