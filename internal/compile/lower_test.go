package compile

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/factstest"
)

// recorder is an expression builder whose form of a construct is the text of the call that built
// it, with every argument the walk decided, so that two walks that call a builder alike build the
// same text. It records the completed rules in order.
type recorder struct{ rules []string }

var _ ExprBuilder[string] = (*recorder)(nil)

// names returns the capture names of s, nil for no scope.
func names(s *Scope) []string {
	if s == nil {
		return nil
	}
	return s.Names
}

func (b *recorder) Literal(e *grammar.Literal, desc ExpID, build bool) string {
	return fmt.Sprintf("lit(%p,%d,%t)", e, desc, build)
}
func (b *recorder) Class(e *grammar.CharClass, desc ExpID, build bool) string {
	return fmt.Sprintf("class(%p,%d,%t)", e, desc, build)
}
func (b *recorder) Any(build bool) string { return fmt.Sprintf("any(%t)", build) }
func (b *recorder) Call(r *Rule, min int) string {
	return fmt.Sprintf("call(%d,%s,%t,%d)", r.ID, r.Name, r.Novalue, min)
}
func (b *recorder) Seq(items []string, flat, valued []bool, n int, build bool) string {
	return fmt.Sprintf("seq(%s;%v;%v;%d;%t)", strings.Join(items, ","), flat, valued, n, build)
}
func (b *recorder) Choice(e *grammar.Choice, alts []string, firsts []First, build bool) string {
	return fmt.Sprintf("choice(%p;%s;%v;%t)", e, strings.Join(alts, ","), firsts, build)
}
func (b *recorder) Optional(e *grammar.Optional, m string, build bool) string {
	return fmt.Sprintf("opt(%p,%s,%t)", e, m, build)
}
func (b *recorder) And(m string, keepsState bool) string {
	return fmt.Sprintf("and(%s,%t)", m, keepsState)
}
func (b *recorder) Not(m string) string                { return "not(" + m + ")" }
func (b *recorder) Atomic(m string, build bool) string { return fmt.Sprintf("atomic(%s,%t)", m, build) }
func (b *recorder) Discard(m string) string            { return "discard(" + m + ")" }
func (b *recorder) Capture(e *grammar.Capture, slot int, m string, projected, build bool) string {
	return fmt.Sprintf("cap(%p,%d,%s,%t,%t)", e, slot, m, projected, build)
}
func (b *recorder) Cut() string   { return "cut" }
func (b *recorder) Empty() string { return "empty" }
func (b *recorder) Fail() string  { return "fail" }
func (b *recorder) Anchor(e grammar.Expr, desc ExpID) string {
	return fmt.Sprintf("anchor(%p,%d)", e, desc)
}
func (b *recorder) Predicate(e *grammar.Predicate, s *Scope) string {
	return fmt.Sprintf("pred(%p)", e)
}
func (b *recorder) ScanRepeat(e *grammar.Repeat, desc ExpID, scan int) string {
	return fmt.Sprintf("scan(%p,%d,%d)", e, desc, scan)
}
func (b *recorder) SkipRepeat(e *grammar.Repeat, elem string, skip int) string {
	return fmt.Sprintf("skip(%p,%s,%d)", e, elem, skip)
}
func (b *recorder) Repeat(e *grammar.Repeat, elem string, elemScope *Scope, ownScope, build, stream bool, site int, shiftable bool, in *Rule, run, scan int) string {
	// Without a scope of its own, elemScope is the enclosing scope, which the walk is still
	// filling (the builder reads it when the parser runs), so only an own scope is recorded.
	if !ownScope {
		elemScope = nil
	}
	return fmt.Sprintf("rep(%p,%s,%v,%t,%t,%t,%d,%t,%d,%d,%d)", e, elem, names(elemScope), ownScope, build, stream, site, shiftable, ruleID(in), run, scan)
}
func (b *recorder) ProjectRepeat(e *grammar.Repeat, elem string, slot, n int) string {
	return fmt.Sprintf("proj(%p,%s,%d,%d)", e, elem, slot, n)
}
func (b *recorder) Missing(m string, site int, expected string, build bool) string {
	return fmt.Sprintf("missing(%s,%d,%q,%t)", m, site, expected, build)
}
func (b *recorder) ErrorMessage(m string, msg ExpID, missingScope bool) string {
	return fmt.Sprintf("error(%s,%d,%t)", m, msg, missingScope)
}
func (b *recorder) Diagnostic(m string, name ExpID) string {
	return fmt.Sprintf("diag(%s,%d)", m, name)
}
func (b *recorder) Recover(e grammar.Expr, m, skip string, build bool) string {
	return fmt.Sprintf("recover(%p,%s,%s,%t)", e, m, skip, build)
}
func (b *recorder) RuleBody(r *Rule, body string) {
	b.rules = append(b.rules, fmt.Sprintf("rule %d %s %t %v: %s", r.ID, r.Name, r.Novalue, names(r.Scope), body))
}
func (b *recorder) OperatorsRule(r *Rule, skip string, hasSkip bool, operands, operators []OpLine[string]) {
	line := func(ls []OpLine[string]) string {
		var parts []string
		for _, l := range ls {
			parts = append(parts, fmt.Sprintf("%p %p %v %s %d %d %s %s", l.Expr, l.Action, names(l.Scope), l.Form, l.ID, l.Level, l.Kind, l.Assoc))
		}
		return strings.Join(parts, "; ")
	}
	b.rules = append(b.rules, fmt.Sprintf("operators %d %s %t %v: %s %t [%s] [%s]", r.ID, r.Name, r.Novalue, names(r.Scope), skip, hasSkip, line(operands), line(operators)))
}

// loweringSources are grammars with the constructs that the corpus lacks: #missing (also after
// @error and through a left-recursion cycle that exists only with insertion, so that the program
// has insertion flags), #recover, #stream and an Operators Rule.
var loweringSources = []string{
	`@start
rule main = stmt+ $$
rule stmt = "x" ";" @error("semicolon") #missing / "y" ")" #missing #recover(skip=[^;]* ";")`,
	`@start
rule main = e $$
rule e = ";" #missing e "+" "n" / "n"`,
	`@start
rule main = x:item* #stream $$
rule item = "a" ("b" / "c") #recover(skip=[^a]*)`,
	`@start
rule main = expr $$
rule expr = #operators {
  operand "n"
  level { infix left "+" @error("plus") #missing }
  level { infix left "*" }
}`,
}

// TestLoweringWalksAsChecking requires CompileProgram to call its builder exactly as the walk of
// Check calls it, for every corpus grammar, every grammar of loweringSources and every option
// variant that changes the walk, also with the rule flags that the engine checks its recognizer,
// budget and insertion programs with: same constructs, expectation IDs, slots, sites, twins and
// rules, in the same order. It also requires lowering to leave the program's expectation table and
// twins as Check left them.
func TestLoweringWalksAsChecking(t *testing.T) {
	t.Parallel()
	variants := []Options{{}, {Recognize: true}, {Recognize: true, Recognizer: true}, {NoProjections: true}, {DefaultValue: true}}
	srcs := map[string]string{}
	for _, path := range factstest.Corpus {
		srcs[path] = factstest.Read(t, path)
	}
	for i, src := range loweringSources {
		srcs[fmt.Sprintf("source %d", i)] = src
	}
	checked, flagged, missing, insertion := 0, 0, 0, 0
	lower := func(name string, g *grammar.Grammar, opts Options, flags []RuleFlags) {
		var first *recorder
		prog, err := checkWith(g, opts, flags, desugar.InlineNames{}, nil, func(*Compiler) ExprBuilder[string] {
			first = &recorder{}
			return first
		})
		if err != nil {
			return // the grammar does not check with this variant
		}
		descs, twins := len(prog.DescTable), len(prog.Twins)
		var again *recorder
		if err := CompileProgram(prog, nil, func(*Compiler) ExprBuilder[string] {
			again = &recorder{}
			return again
		}); err != nil {
			t.Fatalf("%s %+v: %v", name, opts, err)
		}
		if len(prog.DescTable) != descs || len(prog.Twins) != twins {
			t.Errorf("%s %+v: lowering changed the program", name, opts)
		}
		if got, want := strings.Join(again.rules, "\n"), strings.Join(first.rules, "\n"); got != want {
			t.Errorf("%s %+v flags %t: lowering walks differently:\n%s\nwant\n%s", name, opts, flags != nil, firstDiff(got, want), firstDiff(want, got))
		}
		checked++
		if flags != nil {
			flagged++
		}
		if prog.HasMissing {
			missing++
		}
	}
	for name, src := range srcs {
		g, err := parseTest(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, opts := range variants {
			lower(name, g, opts, nil)
		}
		prog, err := checkWith(g, Options{}, nil, desugar.InlineNames{}, nil, func(*Compiler) ExprBuilder[struct{}] { return noExprs{} })
		if err != nil {
			if strings.HasPrefix(name, "source ") {
				t.Fatalf("%s: %v", name, err)
			}
			continue
		}
		// The engine's recognizer and insertion programs.
		lower(name, g, Options{Recognize: true, Recognizer: true}, prog.Flags())
		if prog.InsertionFlags != nil {
			insertion++
			lower(name, g, Options{}, prog.InsertionFlags)
		}
	}
	if checked == 0 || flagged == 0 || missing == 0 || insertion == 0 {
		t.Fatalf("lowered %d programs, %d with flags, %d with #missing, %d grammars with insertion flags", checked, flagged, missing, insertion)
	}
}

// ruleID returns the ID of r, -1 for no rule.
func ruleID(r *Rule) int {
	if r == nil {
		return -1
	}
	return r.ID
}

// firstDiff returns the line of a where it first differs from b.
func firstDiff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i, l := range al {
		if i >= len(bl) || l != bl[i] {
			return l
		}
	}
	return "(prefix)"
}

// TestCompileProgramNeedsCheck requires CompileProgram to refuse a program that Check did not make.
func TestCompileProgramNeedsCheck(t *testing.T) {
	t.Parallel()
	err := CompileProgram(&Program{}, nil, func(*Compiler) ExprBuilder[string] { return &recorder{} })
	if err == nil {
		t.Fatal("CompileProgram lowered a program that Check did not make")
	}
}

// TestCompileProgramConcurrently requires lowering to only read the checked program: several
// lowerings of one program run at once, under the race detector, and each walks as Check did.
// Lowering a checked program while another one reads it (a Result shared by tools) must be safe.
func TestCompileProgramConcurrently(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"examples/minilang/minilang.zp", "examples/calculator/calc_lr.zp", "internal/gentest/testdata/typed/proj1.zp"} {
		g, err := parseTest(factstest.Read(t, path))
		if err != nil {
			t.Fatal(err)
		}
		var first *recorder
		prog, err := checkWith(g, Options{}, nil, desugar.InlineNames{}, nil, func(*Compiler) ExprBuilder[string] {
			first = &recorder{}
			return first
		})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		want := strings.Join(first.rules, "\n")
		const n = 4
		got := make([]string, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				var r *recorder
				// Half of them poll a control, as the engine's lowering does.
				var control *cancel.Control
				if i%2 == 1 {
					control = &cancel.Control{Ctx: context.Background()}
				}
				errs[i] = CompileProgram(prog, control, func(*Compiler) ExprBuilder[string] {
					r = &recorder{}
					return r
				})
				got[i] = strings.Join(r.rules, "\n")
			})
		}
		wg.Wait()
		for i := range n {
			if errs[i] != nil {
				t.Fatalf("%s: lowering %d: %v", path, i, errs[i])
			}
			if got[i] != want {
				t.Errorf("%s: lowering %d walks differently:\n%s\nwant\n%s", path, i, firstDiff(got[i], want), firstDiff(want, got[i]))
			}
		}
	}
}
