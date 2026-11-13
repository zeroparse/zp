package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// A generated Document parse under the default limits succeeds whenever a parse of the same text from
// scratch does, whatever edits are pending, and returns what that parse returns.
func TestGeneratedDocumentDefaultLimitsSucceedWhereFreshDoes(t *testing.T) {
	t.Parallel()
	runDocumentBudgetFixture(t, "rule main = line* $$\nrule line = ^ word \" \" word \"\\n\"\nrule word = [a-z]+ #atomic", fixtureLines)
}

// At the boundary, a generated Document parse succeeds with pending edits where a parse from scratch
// succeeds, up to the largest text the default admits; and an explicit limit counts the work that
// only a Document does.
func TestGeneratedDocumentDefaultLimitsAtBoundary(t *testing.T) {
	t.Parallel()
	runDocumentBudgetFixture(t, "rule main = (. &rest)* $$\nrule rest = . #repeat(0, 1000000)", fixtureBoundary)
}

// A runaway grammar stays stopped by the default with edits pending.
func TestGeneratedDocumentRunawayStoppedByDefault(t *testing.T) {
	t.Parallel()
	runDocumentBudgetFixture(t, "rule main = (. &mid)* $$\nrule mid = (. &.*) #repeat(0, 1000000)", fixtureRunaway)
}

// A text of recovered errors that a parse from scratch completes under the default limit is completed by
// a generated Document parse as well: the lists of errors are shared between the results.
func TestGeneratedDocumentRecoveredErrorsWithinFresh(t *testing.T) {
	t.Parallel()
	runDocumentBudgetFixture(t, "rule main = s $$\nrule s = item s \"!\" / item s / item\nrule item = (\"a\" \";\") #recover(skip=[^;]* \";\")", fixtureRecovered)
}

// A memo entry that lags behind 128 pending edits is reused; one that lags behind 129 is a miss.
func TestGeneratedDocumentMemoLagBoundary(t *testing.T) {
	t.Parallel()
	item := strings.TrimSpace(strings.Repeat("item ", 40))
	runDocumentBudgetFixture(t, "rule main = "+item+" $$\nrule item = [a-z]+ \" \" #atomic", fixtureLag)
}

// The comparison of memo keys costs what it does: a key whose string is the same bytes compares in a
// constant. A generated Document parse completes on a text whose key is long, and does a constant factor
// of the work of a parse from scratch, however long the text, whether the rule that the key belongs to
// is memoized by a parse from scratch too or only by a Document.
func TestGeneratedDocumentCountsMemoKeyWork(t *testing.T) {
	t.Parallel()
	for name, grammar := range map[string]string{
		"leaf":    "var s string\nrule main = h:head { s := text($h) } \";\" (u \"!\" / u)* $$\nrule head = [a-z]* #atomic\nrule u = { s != \"\" } \"b\"",
		"nonleaf": "var s string\nrule main = h:head { s := text($h) } \";\" (u \"!\" / u)* $$\nrule head = [a-z]* #atomic\nrule u = { s != \"\" } bb\nrule bb = \"b\"",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runDocumentBudgetFixture(t, grammar, fixtureEnvKey)
		})
	}
}

// The comparisons of a lookup with the entries of its walk count in a generated Document as they do in
// a parse from scratch: a text whose keys differ only at the end of a long string exceeds the default
// limits in a parse from scratch, and so does the first parse of a generated Document.
func TestGeneratedDocumentCountsMemoWalkComparisons(t *testing.T) {
	t.Parallel()
	grammar := "var s string\nvar a string\nvar b string\nvar c string\n" +
		"rule main = h:head { a := text($h) + \"1\"; b := text($h) + \"2\"; c := \"z\" } \";\" " +
		"({ s := a } u \"!\" / { s := c } { s := b } u \"!\" / { s := c } { s := a } u)* $$\n" +
		"rule head = [a-z]* #atomic\nrule u = { s != \"\" } bb\nrule bb = \"b\""
	runDocumentBudgetFixture(t, grammar, fixtureWalk)
}

// The entries that earlier texts left in the chain of a position are not entries of a parse from scratch,
// so the comparisons with them are work of a generated Document that the default limit does not count:
// after edits that rewrite the end of the head, the parse still succeeds where a parse from scratch does.
func TestGeneratedDocumentLeftoverEntriesAreNotCounted(t *testing.T) {
	t.Parallel()
	grammar := "var s string\nvar a string\nvar b string\nvar c string\n" +
		"rule main = h:head { a := text($h) + \"1\"; b := text($h) + \"2\"; c := \"z\" } \";\" " +
		"({ s := a } u \"!\" / { s := c } { s := b } u \"!\" / { s := c } { s := a } u)* $$\n" +
		"rule head = [a-z]* #atomic\nrule u = { s != \"\" } bb\nrule bb = \"b\""
	runDocumentBudgetFixture(t, grammar, fixtureLeftover)
}

// A typed generated Document keeps and moves its memo too: the comparisons with the entries that earlier
// texts left are work of the Document there as well, so the typed parse succeeds after the same edits.
func TestGeneratedTypedDocumentLeftoverEntriesAreNotCounted(t *testing.T) {
	t.Parallel()
	grammar := "var s string\nvar a string\nvar b string\nvar c string\n" +
		"rule main = h:head { a := text($h) + \"1\"; b := text($h) + \"2\"; c := \"z\" } \";\" " +
		"({ s := a } u \"!\" / { s := c } { s := b } u \"!\" / { s := c } { s := a } u)* $$\n" +
		"rule head = [a-z]* #atomic\nrule u = { s != \"\" } bb\nrule bb = \"b\""
	typed := strings.NewReplacer("Parse(head", "ParseValue(head", "d.Parse()", "d.ParseValue()").Replace(fixtureLeftover)
	runDocumentBudgetFixtureWith(t, codegen.GenOptions{Package: "fixture", Start: "main", Types: true, Document: true}, grammar, typed)
}

// A value that equals the key of an entry but is not the same bytes is a miss for a rule that only a
// generated Document memoizes, and a miss gives the result of a parse from scratch for the work that
// the rule takes.
func TestGeneratedDocumentIdentityMissIsSound(t *testing.T) {
	t.Parallel()
	runDocumentBudgetFixture(t, "var s string\nrule main = line* $$\n"+
		"rule line = h:head { s := text($h) + \"x\" } u \"!\" / h:head { s := text($h) + \"x\" } u \"\\n\"\n"+
		"rule head = \"a\"* #atomic\nrule u = { s == \"aax\" } \"b\"", fixtureIdentityMiss)
}

// A typed generated Document parse stores an entry at the first call of a rule, where a typed parse from
// scratch defers the memoization to the second call. The units it counts against the default limit stay
// within those of the parse from scratch: it completes the texts of recovered errors that the parse from
// scratch completes.
func TestGeneratedTypedDocumentCountedWorkWithinFresh(t *testing.T) {
	t.Parallel()
	runDocumentBudgetFixtureWith(t, codegen.GenOptions{Package: "fixture", Start: "main", Types: true, Document: true},
		"rule main = s $$\nrule s = item s \"!\" / item s / item\nrule item = (\"a\" \";\") #recover(skip=[^;]* \";\")", fixtureTypedRecovered)
}

// runDocumentBudgetFixture generates a Document parser for the grammar and runs the test source
// against it.
func runDocumentBudgetFixture(t *testing.T, grammar, testSource string) {
	t.Helper()
	runDocumentBudgetFixtureWith(t, codegen.GenOptions{Package: "fixture", Start: "main", Document: true}, grammar, testSource)
}

func runDocumentBudgetFixtureWith(t *testing.T, opts codegen.GenOptions, grammar, testSource string) {
	t.Helper()
	codegentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds and runs a generated parser")
	}
	g, err := parseTest(grammar)
	if err != nil {
		t.Fatal(err)
	}
	code, err := codegentest.GenerateCode(t, g, opts)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, source := range map[string]string{
		"go.mod":         codegentest.GoMod("fixture"),
		"parser.go":      string(code),
		"parser_test.go": testSource,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod -trimpath")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Document build/test: %v\n%s", err, out)
	} else {
		t.Logf("%s", out)
	}
}

const fixtureLines = `package fixture
import (
  "encoding/json"
  "fmt"
  "errors"
  "strings"
  "testing"
  "time"
)
func dump(n *Node, err error) string {
  b, _ := json.Marshal(n)
  return string(b) + "/" + fmt.Sprint(err)
}
func TestDefaultLimits(t *testing.T) {
  insertEvery := func(d *Document, count, stride int) {
    for i := 0; i < count; i++ {
      if err := d.Edit(i*stride+1, i*stride+1, "x"); err != nil { t.Fatal(err) }
    }
  }
  scenarios := []struct{ name string; lines int; edit func(d *Document) }{
    {"20000 lines and 6000 edits", 20000, func(d *Document) { insertEvery(d, 6000, 18) }},
    {"one edit", 20000, func(d *Document) { insertEvery(d, 1, 18) }},
    {"many small edits", 3000, func(d *Document) { insertEvery(d, 2999, 6) }},
    {"one huge edit", 20000, func(d *Document) {
      if err := d.Edit(6, len(d.Text())/2, strings.Repeat("zz yy\n", 7000)); err != nil { t.Fatal(err) }
    }},
  }
  for _, unit := range []PositionUnit{Bytes, CodePoints} {
    for _, sc := range scenarios {
      t.Run(fmt.Sprint(unit, "/", sc.name), func(t *testing.T) {
        d, err := NewDocument(strings.Repeat("ab cd\n", sc.lines), WithUnit(unit))
        if err != nil { t.Fatal(err) }
        if _, err := d.Parse(); err != nil { t.Fatalf("first parse: %v", err) }
        sc.edit(d)
        got, err := d.Parse()
        if err != nil { t.Fatalf("document parse of %d bytes: %v", len(d.Text()), err) }
        want, werr := Parse(d.Text(), WithUnit(unit))
        if dump(got, err) != dump(want, werr) { t.Fatal("document result differs from the parse from scratch") }
        t.Logf("%+v", d.Stats())
      })
    }
  }
}

// letterEdits replaces count letters of the text, spread over it, by another letter.
func letterEdits(t *testing.T, d *Document, count int) time.Duration {
  text := []byte(d.Text())
  var at []int
  for i, c := range text {
    if c >= 'a' && c <= 'z' { at = append(at, i) }
  }
  start := time.Now()
  for i := 0; i < count; i++ {
    p := at[(i*7919)%len(at)]
    if err := d.Edit(p, p+1, string(rune('a'+(i%25)+1))); err != nil { t.Fatal(err) }
  }
  return time.Since(start)
}

// minimalFresh returns the smallest explicit Compute limit under which a parse of the text succeeds.
func minimalFresh(t *testing.T, text string) uint64 {
  ok := func(l uint64) bool {
    _, err := Parse(text, WithLimits(Limits{Compute: l}))
    if err != nil && !errors.Is(err, ErrLimit) { t.Fatal(err) }
    return err == nil
  }
  lo, hi := uint64(1), uint64(1)<<30
  if ok(lo) || !ok(hi) { t.Fatal("range") }
  for hi-lo > 1 {
    if mid := (lo + hi) / 2; ok(mid) { hi = mid } else { lo = mid }
  }
  return hi
}

// The units of a Document parse after any number of pending edits are within a factor of those of a
// parse from scratch, all counted (an explicit limit), and the edits themselves, which are not
// charged, cost no more than the edits up to the limit on pending edits: a generated Document drops
// its memo past that limit instead of applying each edit to it.
func TestWorkWithinFactorOfFresh(t *testing.T) {
  const factor = 6
  // The time per edit depends on the load of the machine, so the memo-drop check compares the per-edit
  // times of one attempt and passes if any of a few attempts does: a memo that is not dropped is slower
  // in every attempt.
  const attempts = 3
  perEdit := func(edits int) (*Document, time.Duration) {
    d, err := NewDocument(strings.Repeat("ab cd\n", 20000))
    if err != nil { t.Fatal(err) }
    if _, err := d.Parse(); err != nil { t.Fatalf("first parse: %v", err) }
    took := letterEdits(t, d, edits)
    t.Logf("%d edits took %v", edits, took)
    return d, took / time.Duration(edits)
  }
  for _, edits := range []int{128, 6000, 65536} {
    var d *Document
    if edits == 128 {
      d, _ = perEdit(edits)
    } else {
      // Up to the limit each edit splices the memo, at a cost in proportion to it; past the limit an
      // edit costs what the edit of the text costs.
      var each, base time.Duration
      for attempt := 0; attempt < attempts; attempt++ {
        _, base = perEdit(128)
        d, each = perEdit(edits)
        if each <= base/4 { break }
        if attempt == attempts-1 {
          t.Fatalf("%d edits took %v each, more than a quarter of the %v of the first 128: the memo is not dropped past the limit",
            edits, each, base)
        }
      }
    }
    fresh := minimalFresh(t, d.Text())
    if _, err := d.Parse(WithLimits(Limits{Compute: factor * fresh})); err != nil {
      t.Fatalf("%d pending edits: a document parse needs more than %d times the %d units of a parse from scratch: %v", edits, factor, fresh, err)
    }
  }
}
`

const fixtureBoundary = `package fixture
import (
  "encoding/json"
  "errors"
  "fmt"
  "strings"
  "testing"
)
func dump(n *Node, err error) string {
  b, _ := json.Marshal(n)
  return string(b) + "/" + fmt.Sprint(err)
}
func sameLength(t *testing.T, d *Document, count int) {
  for i := 0; i < count; i++ {
    if err := d.Edit(i, i+1, "b"); err != nil { t.Fatal(err) }
  }
}
// largestFresh returns the largest length of a text of "a" that a parse from scratch admits by default.
func largestFresh(t *testing.T) int {
  fits := func(n int) bool {
    _, err := Parse(strings.Repeat("a", n))
    if err != nil && !errors.Is(err, ErrLimit) { t.Fatal(err) }
    return err == nil
  }
  lo, hi := 1, 1<<14
  if !fits(lo) || fits(hi) { t.Fatal("range") }
  for hi-lo > 1 {
    if mid := (lo + hi) / 2; fits(mid) { lo = mid } else { hi = mid }
  }
  return lo
}
func TestBoundary(t *testing.T) {
  n := largestFresh(t)
  t.Logf("largest text a parse from scratch admits: %d", n)
  for _, edits := range []int{0, 1, 50, 129, 1000} {
    t.Run(fmt.Sprint(edits), func(t *testing.T) {
      d, err := NewDocument(strings.Repeat("a", n))
      if err != nil { t.Fatalf("new: %v", err) }
      if _, err := d.Parse(); err != nil { t.Fatalf("first parse: %v", err) }
      sameLength(t, d, edits)
      got, err := d.Parse()
      if err != nil { t.Fatalf("document parse with %d edits: %v", edits, err) }
      want, werr := Parse(d.Text())
      if dump(got, err) != dump(want, werr) { t.Fatal("document result differs from the parse from scratch") }
    })
  }
  if _, err := Parse(strings.Repeat("a", n+1)); !errors.Is(err, ErrLimit) { t.Fatalf("fresh beyond boundary: %v", err) }
}
// minimal returns the smallest explicit compute limit under which run succeeds.
func minimal(t *testing.T, run func(limit uint64) error) uint64 {
  lo, hi := uint64(1), uint64(1)<<30
  if run(lo) == nil || run(hi) != nil { t.Fatal("range") }
  for hi-lo > 1 {
    if mid := (lo + hi) / 2; run(mid) == nil { hi = mid } else { lo = mid }
  }
  return hi
}
// An explicit limit counts all the work of a Document parse: the one after more edits than the memo
// survives needs more than a parse from scratch, which defers memoization, and stops one unit below what it needs.
func TestExplicitLimitCountsDocumentWork(t *testing.T) {
  text := strings.Repeat("a", 200)
  fresh := minimal(t, func(l uint64) error { _, err := Parse(text, WithLimits(Limits{Compute: l})); return err })
  doc := func(l uint64) error {
    d, err := NewDocument(text)
    if err != nil { return err }
    if _, err := d.Parse(); err != nil { return err }
    sameLength(t, d, 150)
    _, err = d.Parse(WithLimits(Limits{Compute: l}))
    return err
  }
  m := minimal(t, doc)
  t.Logf("fresh %d, document %d", fresh, m)
  if m <= fresh { t.Fatalf("document minimal limit %d does not exceed fresh %d", m, fresh) }
  if err := doc(m - 1); !errors.Is(err, ErrLimit) { t.Fatalf("limit %d: %v", m-1, err) }
}
`

const fixtureRunaway = `package fixture
import (
  "errors"
  "strings"
  "testing"
  "time"
)
func TestRunawayStopped(t *testing.T) {
  for _, pending := range []int{0, 1, 70000} {
    d, err := NewDocument(strings.Repeat("a", 30))
    if err != nil { t.Fatal(err) }
    if _, err := d.Parse(); err != nil { t.Fatalf("first parse: %v", err) }
    for i := 0; i < pending; i++ {
      n := len(d.Text())
      if err := d.Edit(n, n, "a"); err != nil { t.Fatal(err) }
    }
    // A parse that the default does not stop would run for hours: bound the time it may take.
    done := make(chan error, 1)
    go func() { _, err := d.Parse(); done <- err }()
    select {
    case err = <-done:
    case <-time.After(20 * time.Second):
      t.Fatalf("%d pending: the parse of a runaway grammar was not stopped within 20s", pending)
    }
    if pending <= 1 {
      if err != nil { t.Fatalf("%d pending: %v", pending, err) }
      continue
    }
    var le *LimitError
    if !errors.As(err, &le) { t.Fatalf("%d pending: %v", pending, err) }
    if le.Used > le.Limit { t.Fatalf("stopped at %d units used under a limit of %d", le.Used, le.Limit) }
    if want := uint64(max(1<<20, 1024*len(d.Text()))); le.Limit != want {
      t.Fatalf("limit %d, want the default %d", le.Limit, want)
    }
  }
}
`

const fixtureRecovered = `package fixture
import (
  "errors"
  "strings"
  "testing"
)
func TestRecoveredErrorsWithinFresh(t *testing.T) {
  for _, n := range []int{1 << 12, 1 << 14} {
    text := strings.Repeat("b;", n)
    if _, err := Parse(text); errors.Is(err, ErrLimit) { t.Fatalf("n=%d: parse from scratch: %.80v", n, err) }
    d, err := NewDocument(text)
    if err != nil { t.Fatal(err) }
    if _, err := d.Parse(); errors.Is(err, ErrLimit) { t.Errorf("n=%d: Document parse: %.80v", n, err) }
  }
}
`

const fixtureLag = `package fixture
import (
  "strings"
  "testing"
)
func hitsAfter(t *testing.T, count int) int {
  d, err := NewDocument(strings.Repeat("ab ", 40))
  if err != nil { t.Fatal(err) }
  if _, err := d.Parse(); err != nil { t.Fatal(err) }
  for i := 0; i < count; i++ {
    if err := d.Edit(3*39, 3*39+1, string(rune('a'+i%2))); err != nil { t.Fatal(err) }
  }
  if _, err := d.Parse(); err != nil { t.Fatal(err) }
  return d.Stats().Reused
}
func TestMemoLagBoundary(t *testing.T) {
  hits := map[int]int{}
  for _, k := range []int{127, 128, 129, 130} { hits[k] = hitsAfter(t, k) }
  t.Logf("memo hits by pending edits: %v", hits)
  if hits[127] < 30 || hits[128] < 30 { t.Errorf("entries that lag behind 127 and 128 edits are not reused: %v", hits) }
  if hits[129] > 5 || hits[130] > 5 { t.Errorf("entries that lag behind 129 and 130 edits are reused: %v", hits) }
}
`

const fixtureTypedRecovered = `package fixture
import (
  "errors"
  "strings"
  "testing"
)
func TestTypedDocumentCountedWithinFresh(t *testing.T) {
  for _, n := range []int{1 << 12, 1 << 14} {
    text := strings.Repeat("b;", n)
    if _, err := ParseValue(text); errors.Is(err, ErrLimit) { t.Fatalf("n=%d: typed parse from scratch: %.80v", n, err) }
    d, err := NewDocument(text)
    if err != nil { t.Fatal(err) }
    if _, err := d.ParseValue(); errors.Is(err, ErrLimit) {
      t.Errorf("n=%d: the typed Document parse ends at the limit where a parse from scratch does not: %.80v", n, err)
    }
  }
}
`

const fixtureEnvKey = `package fixture
import (
  "errors"
  "strings"
  "testing"
)
func TestMemoKeyWorkWithinFresh(t *testing.T) {
  least := func(text string, doc bool) uint64 {
    ok := func(l uint64) bool {
      var err error
      if doc {
        d, derr := NewDocument(text)
        if derr != nil { t.Fatal(derr) }
        _, err = d.Parse(WithLimits(Limits{Compute: l}))
      } else {
        _, err = Parse(text, WithLimits(Limits{Compute: l}))
      }
      if err != nil && !errors.Is(err, ErrLimit) { t.Fatal(err) }
      return err == nil
    }
    lo, hi := uint64(1), uint64(1)<<30
    if ok(lo) || !ok(hi) { t.Fatal("range") }
    for hi-lo > 1 {
      if mid := (lo + hi) / 2; ok(mid) { hi = mid } else { lo = mid }
    }
    return hi
  }
  for _, n := range []int{2000, 64000} {
    text := strings.Repeat("a", n) + ";" + strings.Repeat("b", n)
    fresh, doc := least(text, false), least(text, true)
    t.Logf("n=%d: fresh %d units, Document %d units", n, fresh, doc)
    if doc > 3*fresh {
      t.Errorf("n=%d: a Document parse needs %d units, more than 3 times the %d of a parse from scratch", n, doc, fresh)
    }
    // Under the default limit the Document parse completes.
    d, err := NewDocument(text)
    if err != nil { t.Fatal(err) }
    if _, err := d.Parse(); err != nil { t.Errorf("n=%d: Document parse under the default limits: %.80v", n, err) }
  }
}
`

const fixtureWalk = `package fixture
import (
  "errors"
  "strings"
  "testing"
)
func TestWalkComparisonsCount(t *testing.T) {
  text := strings.Repeat("a", 16000) + ";" + strings.Repeat("b", 16000)
  if _, err := Parse(text); !errors.Is(err, ErrLimit) { t.Fatalf("parse from scratch: %.80v, want the limit to be exceeded", err) }
  d, err := NewDocument(text)
  if err != nil { t.Fatal(err) }
  if _, err := d.Parse(); !errors.Is(err, ErrLimit) { t.Errorf("Document parse: %.80v, want the limit to be exceeded", err) }
}
`

const fixtureLeftover = `package fixture
import (
  "fmt"
  "strings"
  "testing"
)
func head(n int, tail string) string { return strings.Repeat("a", n-2) + tail + ";" + strings.Repeat("b", n) }
func TestLeftoverEntriesAreNotCounted(t *testing.T) {
  // The longest text whose parse from scratch completes under the default limit: the work of the
  // text is just below the limit, so work counted beyond a parse from scratch's is a limit error.
  lo, hi := 100, 4000
  for hi-lo > 1 {
    mid := (lo + hi) / 2
    if _, err := Parse(head(mid, "aa")); err == nil { lo = mid } else { hi = mid }
  }
  n := lo
  t.Logf("n=%d", n)
  d, err := NewDocument(head(n, "aa"))
  if err != nil { t.Fatal(err) }
  if _, err := d.Parse(); err != nil { t.Fatalf("first Document parse: %.100v", err) }
  for k := 0; k < 12; k++ {
    rep := fmt.Sprintf("%c%c", 'c'+k/6, 'c'+k%6)
    if err := d.Edit(n-2, n, rep); err != nil { t.Fatal(err) }
    if _, err := Parse(head(n, rep)); err != nil { t.Fatalf("edit %d: parse from scratch: %.100v", k, err) }
    if _, err := d.Parse(); err != nil { t.Fatalf("edit %d: Document parse: %.100v", k, err) }
  }
}
`

const fixtureIdentityMiss = `package fixture
import (
  "encoding/json"
  "errors"
  "strings"
  "testing"
)
func TestIdentityMissIsSound(t *testing.T) {
  text := strings.Repeat("aab\n", 3000)
  want, err := Parse(text)
  if err != nil { t.Fatal(err) }
  d, err := NewDocument(text)
  if err != nil { t.Fatal(err) }
  got, err := d.Parse()
  if err != nil { t.Fatal(err) }
  w, _ := json.Marshal(want)
  g, _ := json.Marshal(got)
  if string(w) != string(g) { t.Error("the Document parse returns a different tree than the parse from scratch") }
  least := func(doc bool) uint64 {
    ok := func(l uint64) bool {
      var err error
      if doc {
        d, derr := NewDocument(text)
        if derr != nil { t.Fatal(derr) }
        _, err = d.Parse(WithLimits(Limits{Compute: l}))
      } else {
        _, err = Parse(text, WithLimits(Limits{Compute: l}))
      }
      if err != nil && !errors.Is(err, ErrLimit) { t.Fatal(err) }
      return err == nil
    }
    lo, hi := uint64(1), uint64(1)<<30
    if ok(lo) || !ok(hi) { t.Fatal("range") }
    for hi-lo > 1 {
      if mid := (lo + hi) / 2; ok(mid) { hi = mid } else { lo = mid }
    }
    return hi
  }
  fresh, doc := least(false), least(true)
  t.Logf("fresh %d units, Document %d units", fresh, doc)
  if doc > 3*fresh { t.Errorf("a Document parse needs %d units, more than 3 times the %d of a parse from scratch", doc, fresh) }
}
`
