package language_test

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/grammar"
)

// The comparison of the frontends is only worth something if it sees a change
// in either of them. These controls change each side and require the
// difference to be reported.

// mutateLeaves changes, one at a time, every leaf of the AST g that the JSON
// form holds, and calls check after each change with the number of the leaf.
// It returns the number of leaves it changed. A leaf is a string, a number or
// a boolean field, an element of a list of strings, and a value of a
// positional or keyword attribute argument.
func mutateLeaves(g *grammar.Grammar, check func(path string)) int {
	count := 0
	var walk func(path string, v reflect.Value)
	flip := func(path string, v reflect.Value, set func(reflect.Value)) bool {
		var changed reflect.Value
		switch v.Kind() {
		case reflect.String:
			changed = reflect.ValueOf(v.String() + "x").Convert(v.Type())
		case reflect.Int, reflect.Int32, reflect.Int64:
			changed = reflect.ValueOf(v.Int() + 1).Convert(v.Type())
		case reflect.Bool:
			changed = reflect.ValueOf(!v.Bool())
		default:
			return false
		}
		old := reflect.New(v.Type()).Elem()
		old.Set(v)
		set(changed)
		count++
		check(path)
		set(old)
		return true
	}
	walk = func(path string, v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				walk(path, v.Elem())
			}
		case reflect.Interface:
			if v.IsNil() {
				return
			}
			elem := v.Elem()
			if elem.Kind() == reflect.Pointer {
				walk(path, elem)
				return
			}
			// A scalar held in an interface: replace the interface value.
			flip(path, elem, func(x reflect.Value) { v.Set(x) })
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(path+"["+strconv.Itoa(i)+"]", v.Index(i))
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				elem := v.MapIndex(k)
				if elem.Kind() == reflect.Interface && !elem.IsNil() && elem.Elem().Kind() != reflect.Pointer {
					key := k
					flip(path+"."+k.String(), elem.Elem(), func(x reflect.Value) { v.SetMapIndex(key, x) })
				} else {
					walk(path+"."+k.String(), elem)
				}
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				f := v.Type().Field(i)
				if !f.IsExported() || f.Tag.Get("json") == "-" {
					continue
				}
				fv := v.Field(i)
				if !flip(path+"."+f.Name, fv, func(x reflect.Value) { fv.Set(x) }) {
					walk(path+"."+f.Name, fv)
				}
			}
		}
	}
	walk("", reflect.ValueOf(g).Elem())
	return count
}

// nodeSwaps are the groups of parser expressions that hold the same fields, so
// that one can replace another in the AST and the JSON form stays the same.
var nodeSwaps = [][]reflect.Type{
	{typeOf[grammar.Optional](), typeOf[grammar.And](), typeOf[grammar.Not](), typeOf[grammar.Atomic](), typeOf[grammar.Stream](), typeOf[grammar.Discard](), typeOf[grammar.Group]()},
	{typeOf[grammar.Any](), typeOf[grammar.Cut](), typeOf[grammar.Top](), typeOf[grammar.Bottom](), typeOf[grammar.BeginInput](), typeOf[grammar.EndInput](), typeOf[grammar.BeginLine](), typeOf[grammar.EndLine]()},
}

func typeOf[T any]() reflect.Type { return reflect.TypeFor[T]() }

// swapNodeTypes replaces, one at a time, every parser expression of the AST g
// that has an alternative in nodeSwaps by each alternative, which keeps the
// fields the two have in common, and calls check after each replacement. It
// returns the number of replacements.
func swapNodeTypes(g *grammar.Grammar, check func(path string)) int {
	count := 0
	var walk func(path string, v reflect.Value)
	walk = func(path string, v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				walk(path, v.Elem())
			}
		case reflect.Interface:
			if v.IsNil() {
				return
			}
			old := v.Elem()
			if old.Kind() == reflect.Pointer {
				for _, group := range nodeSwaps {
					if !slices.Contains(group, old.Type().Elem()) {
						continue
					}
					for _, to := range group {
						if to == old.Type().Elem() {
							continue
						}
						swapped := reflect.New(to)
						for i := 0; i < to.NumField(); i++ {
							if f, ok := old.Type().Elem().FieldByName(to.Field(i).Name); ok && f.Type == to.Field(i).Type {
								swapped.Elem().Field(i).Set(old.Elem().FieldByName(f.Name))
							}
						}
						v.Set(swapped)
						count++
						check(path + " " + old.Type().Elem().Name() + " as " + to.Name())
						v.Set(old)
					}
				}
			}
			walk(path, old)
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(path+"["+strconv.Itoa(i)+"]", v.Index(i))
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				if elem := v.MapIndex(k); elem.Kind() == reflect.Interface && !elem.IsNil() && elem.Elem().Kind() == reflect.Pointer {
					walk(path+"."+k.String(), elem.Elem())
				}
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if f := v.Type().Field(i); f.IsExported() && f.Tag.Get("json") != "-" {
					walk(path+"."+f.Name, v.Field(i))
				}
			}
		}
	}
	walk("", reflect.ValueOf(g).Elem())
	return count
}

// TestGrammarComparisonSeesNodeTypes swaps the Go type of each node, such as
// #atomic for #stream, which the JSON form does not name, and requires the
// comparison to report it.
func TestGrammarComparisonSeesNodeTypes(t *testing.T) {
	p, _ := sourceGrammar(t)
	total := 0
	for _, src := range append(append([]string(nil), ownershipCorpus...), nodeBases...) {
		manual, merr := zp.ParseGrammar(src)
		g, err := adaptCanonical(p, src)
		if merr != nil || err != nil {
			continue
		}
		want := grammarJSON(t, manual)
		total += swapNodeTypes(g, func(path string) {
			if firstDifference(want, grammarJSON(t, g)) == "" {
				t.Errorf("%q: the swap of %s is not reported", src, path)
			}
		})
	}
	if total < 100 {
		t.Fatalf("only %d nodes were swapped", total)
	}
	t.Logf("swapped the type of %d nodes", total)
}

// TestGrammarComparisonSeesEveryField changes every JSON-visible leaf of the
// grammar the adapter builds, for each grammar of the parity corpora, and
// requires the comparison with the unchanged grammar to report it.
func TestGrammarComparisonSeesEveryField(t *testing.T) {
	p, _ := sourceGrammar(t)
	total, kinds := 0, map[string]bool{}
	for _, src := range append(append([]string(nil), ownershipCorpus...), nodeBases...) {
		manual, merr := zp.ParseGrammar(src)
		g, err := adaptCanonical(p, src)
		if merr != nil || err != nil {
			continue
		}
		want := grammarJSON(t, manual)
		if d := firstDifference(want, grammarJSON(t, g)); d != "" {
			t.Fatalf("control %q differs without a change: %s", src, d)
		}
		total += mutateLeaves(g, func(path string) {
			kinds[path[strings.LastIndex(path, ".")+1:]] = true
			if firstDifference(want, grammarJSON(t, g)) == "" {
				t.Errorf("%q: a change of %s is not reported", src, path)
			}
		})
	}
	if total < 5000 {
		t.Fatalf("only %d leaves were changed", total)
	}
	t.Logf("changed %d leaves of %d fields", total, len(kinds))
}

// grammarMutants change the source grammar zp.zp in one place. Each must make
// the canonical frontend disagree with the hand-written parser on some source
// of the parity corpora.
var grammarMutants = []struct{ name, old, new string }{
	{"optional becomes a repetition", `postfix "?" => Optional{X: $lhs}`, `postfix "?" => ZeroOrMore{X: $lhs}`},
	{"choice becomes a sequence", `infix left "/" => Choice{L: $lhs, R: $rhs}`, `infix left "/" => Sequence{L: $lhs, R: $rhs}`},
	{"atomic becomes stream", `Atomic{X: $lhs}`, `Stream{X: $lhs}`},
	{"repeat loses its range flag", `Range: $bounds.Range}`, `Range: false}`},
	{"top takes a name", `rule top Top = ("_" !ident_char) #atomic`, `rule top Top = ("_") #atomic`},
	{"plus is not an operator", `infix left "+" / "-" !"-"`, `infix left "-" !"-"`},
	{"list type becomes optional", `prefix "[]" => ListType{Type: $rhs}`, `prefix "[]" => OptionalType{Type: $rhs}`},
	{"zero is not an escape", `[ntrfvab0]`, `[ntrfvab]`},
	{"none is not an associativity", `("left" / "right" / "none")`, `("left" / "right")`},
	{"ignore-case suffix is another letter", `("i" !ident_char)?`, `("j" !ident_char)?`},
	{"block comments do not start a comment", `rule source_comment_start = "//" / "/*"`, `rule source_comment_start = "//"`},
	{"line comments do not flush before a block", `rule source_line_flush = source_horizontal (source_nl / "/*" / source_scope_end)`, `rule source_line_flush = source_horizontal (source_nl / source_scope_end)`},
	{"lookahead is always a negation", `Lookahead{Op: text($op), X: $rhs}`, `Lookahead{Op: "!", X: $rhs}`},
	{"unary operator is always a minus", `UnaryOp{Op: text($op), X: $rhs}`, `UnaryOp{Op: "-", X: $rhs}`},
}

// mutantProbes add to the corpora the sources that only a few mutants show.
var mutantProbes = []string{
	"rule a = _x\n",
	"rule a = \"\\0\"\n",
	"rule e = #operators { operand \"1\"\n level l { infix none \"==\" } }\n",
	"rule a = x\n// a\n/* b */\nrule b = y\n",
	"rule a = x // t\n/* b */ // c\nrule b = y\n",
}

func TestGrammarComparisonSeesMutantGrammars(t *testing.T) {
	_, source := sourceGrammar(t)
	corpus := append(append(append([]string(nil), ownershipCorpus...), nodeBases...), mutantProbes...)
	for _, m := range grammarMutants {
		t.Run(m.name, func(t *testing.T) {
			if strings.Count(source, m.old) != 1 {
				t.Fatalf("the mutant must change exactly one place, found %d", strings.Count(source, m.old))
			}
			mutant, err := zp.CompileSource(strings.Replace(source, m.old, m.new, 1), "main")
			if err != nil {
				t.Fatalf("the mutant grammar does not compile: %v", err)
			}
			for _, src := range corpus {
				if grammarDiff(t, mutant, src) != "" {
					return
				}
			}
			t.Fatal("no source of the corpus shows the mutant")
		})
	}
}
