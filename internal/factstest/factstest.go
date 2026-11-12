// Package factstest pins the results of the static analyses of grammars, so that moving an
// analysis keeps them. Its main pin holds the grammar facts of the repository's grammars (Corpus):
// per rule, the facts that the static analyses compute (nullability, left recursion, memo policy,
// positions, recovery, variables) and the first terminal of the rule and of every alternative of
// its choices. Every producer of these facts renders them through Render, so that their outputs
// compare byte for byte against one golden file per grammar in testdata. The other golden files
// (GoldenFile) hold the cost class of every corpus grammar, the lint findings on the example
// grammars and the error lists of grammars with errors of several kinds; the tests that write
// them live with the analyses.
package factstest

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/ir/coreir"
	"zeroparse.dev/zp/internal/ir/coreir/facts"
)

var update = flag.Bool("update-facts", false, "rewrite the grammar facts golden files")

// Corpus lists the grammars whose facts are pinned, relative to the repository root.
var Corpus = []string{
	"bench/grammars/terms/heavy.zp",
	"bench/grammars/terms/pred.zp",
	"examples/calculator/calc_lr.zp",
	"examples/calculator/calc.zp",
	"examples/minilang/minilang.zp",
	"examples/outline/outline.zp",
	"internal/gen/gogen/testdata/routes/lr_calls_ops.zp",
	"internal/gen/gogen/testdata/routes/lr_inline.zp",
	"internal/gen/gogen/testdata/routes/lr_looks.zp",
	"internal/gen/gogen/testdata/routes/lr_recover_rep.zp",
	"internal/gen/gogen/testdata/routes/lr_recover.zp",
	"internal/gen/gogen/testdata/routes/ops_lines.zp",
	"internal/gen/gogen/testdata/routes/ops_recover.zp",
	"internal/gen/gogen/testdata/routes/recover_nested.zp",
	"internal/gen/gogen/testdata/routes/scopes.zp",
	"internal/gentest/testdata/typed/calclr.zp",
	"internal/gentest/testdata/typed/direct_backtrack.zp",
	"internal/gentest/testdata/typed/direct_errors.zp",
	"internal/gentest/testdata/typed/direct_lists.zp",
	"internal/gentest/testdata/typed/direct_literals.zp",
	"internal/gentest/testdata/typed/direct_names.zp",
	"internal/gentest/testdata/typed/element_scope_optional.zp",
	"internal/gentest/testdata/typed/element_scope_plain.zp",
	"internal/gentest/testdata/typed/element_scope_recover.zp",
	"internal/gentest/testdata/typed/errlist.zp",
	"internal/gentest/testdata/typed/errlist2.zp",
	"internal/gentest/testdata/typed/fields.zp",
	"internal/gentest/testdata/typed/mix.zp",
	"internal/gentest/testdata/typed/mixf.zp",
	"internal/gentest/testdata/typed/operators_rule1.zp",
	"internal/gentest/testdata/typed/operators_rule1f.zp",
	"internal/gentest/testdata/typed/order.zp",
	"internal/gentest/testdata/typed/proj1.zp",
	"internal/gentest/testdata/typed/proj2.zp",
	"internal/gentest/testdata/typed/proj3.zp",
	"internal/gentest/testdata/typed/proj4.zp",
	"internal/gentest/testdata/typed/proj5.zp",
	"internal/gentest/testdata/typed/proj6.zp",
	"internal/gentest/testdata/typed/proj7.zp",
	"internal/gentest/testdata/typed/spanname.zp",
	"internal/gentest/testdata/typed/splice_recursive_capture_action.zp",
	"internal/gentest/testdata/typed/splice_recursive_capture.zp",
	"internal/gentest/testdata/typed/splice_recursive_choice_in_seq.zp",
	"internal/gentest/testdata/typed/splice_recursive_choice.zp",
	"internal/gentest/testdata/typed/splice_recursive_optional.zp",
	"internal/gentest/testdata/typed/splice_recursive_repeat.zp",
	"internal/gentest/testdata/typed/unif.zp",
	"spec/language/testdata/comment-boundaries.zp",
	"testdata/grammars/cel/cel.zp",
	"testdata/grammars/csv/csv.zp",
	"testdata/grammars/cue/cue.zp",
	"testdata/grammars/duckdb/duckdb.zp",
	"testdata/grammars/golang/golang.zp",
	"testdata/grammars/json/json.zp",
	"testdata/grammars/python/python.zp",
	"testdata/grammars/typescript/typescript.zp",
	"testdata/grammars/xml/xml.zp",
	"testdata/grammars/yaml/yaml.zp",
	"zp.zp",
}

// Root returns the repository root: the nearest directory above the working directory with the
// repository's go.mod.
func Root(tb testing.TB) string {
	tb.Helper()
	dir, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && strings.HasPrefix(string(b), "module zeroparse.dev/zp\n") {
			return dir
		}
		up := filepath.Dir(dir)
		if up == dir {
			tb.Fatal("factstest: repository root not found")
		}
		dir = up
	}
}

// Read returns the source of the corpus grammar at path (relative to the repository root).
func Read(tb testing.TB, path string) string {
	tb.Helper()
	b, err := os.ReadFile(filepath.Join(Root(tb), path))
	if err != nil {
		tb.Fatal(err)
	}
	return string(b)
}

// Terminal is the terminal that an expression must begin with, named by the expectation it
// records (a literal or a character class as written), and the number of rule calls on the way.
type Terminal struct {
	// Exp is the terminal's expectation, "" when there is no terminal.
	Exp   string
	Depth int
}

func (t Terminal) String() string {
	if t.Exp == "" {
		return "-"
	}
	return t.Exp + "@" + strconv.Itoa(t.Depth)
}

// Alt is the first terminal of one alternative of a choice, which is named by its position in the
// grammar and the index of the alternative.
type Alt struct {
	Line, Col, Index int
	First            Terminal
}

// Rule holds the facts of one rule.
type Rule struct {
	Name string
	// Nullable is the runtime nullability, the one that decides left calls.
	Nullable bool
	// Cyclic reports a rule on a left-recursion cycle; Cycle numbers its cycle from 1 (0 when it
	// is on none); Leader and Member split the cyclic rules.
	Cyclic         bool
	Cycle          int
	Leader, Member bool
	// InsCycle, InsLeader and InsMember are Cycle, Leader and Member with token insertion
	// enabled. They are rendered only when they differ from those without.
	InsCycle             int
	InsLeader, InsMember bool
	// Rec reports a rule on a cycle of the call graph.
	Rec bool
	// Positional reports a result that may depend on positions.
	Positional bool
	// Recovering names the rule whose #recover can put an error value in this rule's value, or is
	// "" for none.
	Recovering string
	// Vars are the variables the rule and its callees read, sorted.
	Vars []string
	// Memo, Transient and Missing are the memo policy: memoized, memoized only by incremental
	// parsing, and reaching a #missing insertion.
	Memo, Transient, Missing bool
	First                    Terminal
	Alts                     []Alt
}

// Render returns the facts of the rules as text, one line per rule followed by one line per
// alternative.
func Render(rules []Rule) string {
	var b strings.Builder
	// Cycles are renumbered from 1 in the order of their first rule: only which rules share a
	// cycle is a fact.
	cycles := map[int]int{}
	for _, r := range rules {
		if r.Cycle != 0 && cycles[r.Cycle] == 0 {
			cycles[r.Cycle] = len(cycles) + 1
		}
	}
	insCycles := map[int]int{}
	for _, r := range rules {
		if r.InsCycle != 0 && insCycles[r.InsCycle] == 0 {
			insCycles[r.InsCycle] = len(insCycles) + 1
		}
	}
	insDiffers := false
	for _, r := range rules {
		insDiffers = insDiffers || insCycles[r.InsCycle] != cycles[r.Cycle] || r.InsLeader != r.Leader || r.InsMember != r.Member
	}
	for _, r := range rules {
		b.WriteString(r.Name)
		flag := func(name string, v bool) {
			b.WriteString(" " + name + "=" + strconv.FormatBool(v))
		}
		flag("null", r.Nullable)
		flag("cyclic", r.Cyclic)
		b.WriteString(" cycle=" + strconv.Itoa(cycles[r.Cycle]))
		flag("leader", r.Leader)
		flag("member", r.Member)
		flag("rec", r.Rec)
		flag("positional", r.Positional)
		rec := r.Recovering
		if rec == "" {
			rec = "-"
		}
		b.WriteString(" recovering=" + rec)
		vars := strings.Join(r.Vars, ",")
		if vars == "" {
			vars = "-"
		}
		b.WriteString(" vars=" + vars)
		flag("memo", r.Memo)
		flag("transient", r.Transient)
		flag("missing", r.Missing)
		if insDiffers {
			b.WriteString(" ins-cycle=" + strconv.Itoa(insCycles[r.InsCycle]))
			flag("ins-leader", r.InsLeader)
			flag("ins-member", r.InsMember)
		}
		b.WriteString(" first=" + r.First.String() + "\n")
		for _, a := range r.Alts {
			b.WriteString("  alt " + strconv.Itoa(a.Line) + ":" + strconv.Itoa(a.Col) + "/" + strconv.Itoa(a.Index) + " first=" + a.First.String() + "\n")
		}
	}
	return b.String()
}

// Golden compares got with the golden facts of the corpus grammar at path, or rewrites the golden
// file when the test runs with -update-facts.
func Golden(tb testing.TB, path, got string) {
	tb.Helper()
	GoldenFile(tb, goldenName(path), got)
}

func goldenName(path string) string {
	return strings.ReplaceAll(strings.TrimSuffix(path, ".zp"), "/", "__") + ".txt"
}

// GoldenFile compares got with the file name in this package's testdata, or rewrites it when the
// test runs with -update-facts.
func GoldenFile(tb testing.TB, name, got string) {
	tb.Helper()
	file := filepath.Join(Root(tb), "internal", "factstest", "testdata", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
			tb.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		tb.Fatalf("%v (run with -update-facts to create it)", err)
	}
	if string(want) != got {
		tb.Errorf("%s differs:\n%s", file, lineDiff(string(want), got))
	}
}

// Load returns the pinned facts of the corpus grammar at path: for each rule, its facts by name
// ("null", "first", ...) as Render writes them.
func Load(tb testing.TB, path string) map[string]map[string]string {
	tb.Helper()
	b, err := os.ReadFile(filepath.Join(Root(tb), "internal", "factstest", "testdata", goldenName(path)))
	if err != nil {
		tb.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}
		// first is the last fact, and a terminal may hold spaces.
		i := strings.LastIndex(line, " first=")
		name, rest, _ := strings.Cut(line[:i], " ")
		m := map[string]string{"first": line[i+len(" first="):]}
		for _, kv := range strings.Split(rest, " ") {
			k, v, _ := strings.Cut(kv, "=")
			m[k] = v
		}
		out[name] = m
	}
	return out
}

// lineDiff lists the lines that differ between want and got, at most 40 of them.
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	n := 0
	for i := 0; i < max(len(w), len(g)) && n < 40; i++ {
		var x, y string
		if i < len(w) {
			x = w[i]
		}
		if i < len(g) {
			y = g[i]
		}
		if x != y {
			b.WriteString("- " + x + "\n+ " + y + "\n")
			n++
		}
	}
	return b.String()
}

// CoreRules returns the facts that internal/ir/coreir/facts computed for the rules of a module that
// are not value-free twins, in the form Render writes. origin maps the module's nodes to the
// expressions they were built from, which name the choices of the alternatives.
func CoreRules(m *coreir.Module, f *facts.Facts, origin map[coreir.Node]grammar.Expr) []Rule {
	twin := map[int]bool{}
	for _, r := range m.Rules {
		if r.Twin != nil {
			twin[r.Twin.ID] = true
		}
	}
	terminal := func(n coreir.Node) Terminal {
		if f := n.Attrs().First; f != nil {
			return Terminal{Exp: m.Exps[f.Exp], Depth: f.Depth}
		}
		return Terminal{}
	}
	var out []Rule
	for _, r := range m.Rules {
		if twin[r.ID] {
			continue
		}
		x := f.Rules[r.ID]
		rule := Rule{Name: r.Name, Nullable: x.Nullable, Cyclic: x.Cyclic(), Cycle: x.Cycle, Leader: x.Leader,
			Member: x.Member, InsCycle: x.CycleIns, InsLeader: x.LeaderIns, InsMember: x.MemberIns, Rec: x.Rec, Positional: x.Positional, Vars: x.Vars,
			Memo: x.Memo, Transient: x.Transient, Missing: x.Missing, First: terminal(r.Body)}
		if x.Recovering >= 0 {
			rule.Recovering = m.Rules[x.Recovering].Name
		}
		coreir.Walk(r.Body, func(n coreir.Node) {
			c, ok := n.(*coreir.Choice)
			if !ok {
				return
			}
			pos := origin[c].(*grammar.Choice).Diagnostic()
			for i, alt := range c.Alts {
				rule.Alts = append(rule.Alts, Alt{Line: pos.Line, Col: pos.Col, Index: i, First: terminal(alt.Body)})
			}
		})
		out = append(out, rule)
	}
	return out
}
