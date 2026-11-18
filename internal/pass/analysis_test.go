package pass

import (
	"errors"
	"strings"
	"testing"
)

// doc is a unit whose analyses count their runs in it.
type doc struct {
	words string
	runs  map[string]int
}

func (d *doc) ran(name string) { d.runs[name]++ }

var (
	errEmpty = errors.New("no words")

	keyWords = NewAnalysisKey("test-words", func(d *doc, _ *AnalysisManager[*doc]) ([]string, error) {
		d.ran("test-words")
		if d.words == "" {
			return nil, errEmpty
		}
		return strings.Fields(d.words), nil
	})
	keyCount = NewAnalysisKey("test-count", func(d *doc, am *AnalysisManager[*doc]) (int, error) {
		d.ran("test-count")
		ws, err := Get(am, d, keyWords)
		return len(ws), err
	})
	keyLen = NewAnalysisKey("test-len", func(d *doc, _ *AnalysisManager[*doc]) (int, error) {
		d.ran("test-len")
		return len(d.words), nil
	})
	// keyLoop asks for keyLoop2, which asks for keyLoop.
	keyLoop  AnalysisKey[*doc, int]
	keyLoop2 AnalysisKey[*doc, int]
)

func init() {
	keyLoop = NewAnalysisKey("test-loop", func(d *doc, am *AnalysisManager[*doc]) (int, error) {
		return Get(am, d, keyLoop2)
	})
	keyLoop2 = NewAnalysisKey("test-loop2", func(d *doc, am *AnalysisManager[*doc]) (int, error) {
		return Get(am, d, keyLoop)
	})
}

func newDoc(words string) *doc { return &doc{words: words, runs: map[string]int{}} }

// An analysis is computed once while it stays valid, and a cached result is returned after.
func TestAnalysisIsCached(t *testing.T) {
	am, d := NewAnalysisManager[*doc](nil), newDoc("a b c")
	for range 3 {
		n, err := Get(am, d, keyCount)
		if err != nil || n != 3 {
			t.Fatalf("count = %d, %v", n, err)
		}
	}
	if d.runs["test-count"] != 1 || d.runs["test-words"] != 1 {
		t.Fatalf("runs %v, want one of each", d.runs)
	}
	if am.Computations("test-count") != 1 {
		t.Fatalf("computations %d", am.Computations("test-count"))
	}
}

// A failing analysis returns its error, through the analyses that use it, and caches nothing.
func TestFailingAnalysisReturnsError(t *testing.T) {
	am, d := NewAnalysisManager[*doc](nil), newDoc("")
	if _, err := Get(am, d, keyCount); !errors.Is(err, errEmpty) || !strings.Contains(err.Error(), "analysis test-count: analysis test-words") {
		t.Fatalf("error %v", err)
	}
	if am.Cached("test-words") || am.Cached("test-count") {
		t.Fatal("a failed analysis is cached")
	}
	d.words = "x"
	if n, err := Get(am, d, keyCount); err != nil || n != 1 {
		t.Fatalf("after the fix: %d, %v", n, err)
	}
}

// An analysis that asks for itself, directly or not, fails instead of recursing.
func TestSelfDependencyIsAnError(t *testing.T) {
	am := NewAnalysisManager[*doc](nil)
	_, err := Get(am, newDoc("a"), keyLoop)
	if err == nil || !strings.Contains(err.Error(), "test-loop depends on itself") {
		t.Fatalf("error %v", err)
	}
	if len(am.running) != 0 {
		t.Fatalf("running %v after the error", am.running)
	}
}

// Nothing is preserved by default: an analysis that a change does not preserve is computed again.
func TestMissingPreservationRecomputes(t *testing.T) {
	am, d := NewAnalysisManager[*doc](nil), newDoc("a b")
	Get(am, d, keyLen)
	am.Invalidate(PreservedSet{})
	Get(am, d, keyLen)
	if d.runs["test-len"] != 2 {
		t.Fatalf("runs %v: the zero PreservedSet preserved an analysis", d.runs)
	}
	am.Invalidate(Preserve("test-len"))
	am.Invalidate(PreserveAll())
	Get(am, d, keyLen)
	if d.runs["test-len"] != 2 {
		t.Fatalf("runs %v: a preserved analysis was computed again", d.runs)
	}
}

// Discarding an analysis discards the analyses that used it, even if they are preserved.
func TestInvalidationIsTransitive(t *testing.T) {
	am, d := NewAnalysisManager[*doc](nil), newDoc("a b")
	Get(am, d, keyCount)
	Get(am, d, keyLen)
	am.Invalidate(Preserve("test-count", "test-len"))
	if !am.Cached("test-len") || am.Cached("test-count") || am.Cached("test-words") {
		t.Fatalf("after preserving count and len: words %v count %v len %v",
			am.Cached("test-words"), am.Cached("test-count"), am.Cached("test-len"))
	}
	d.words = "a b c"
	if n, _ := Get(am, d, keyCount); n != 3 {
		t.Fatalf("count %d read stale words", n)
	}
}

// A request for another unit discards the analyses of the first: a result is always of its unit.
func TestAnotherUnitDiscardsTheCache(t *testing.T) {
	am := NewAnalysisManager[*doc](nil)
	a, b := newDoc("a"), newDoc("a b")
	Get(am, a, keyCount)
	if n, _ := Get(am, b, keyCount); n != 2 {
		t.Fatalf("count of the second unit = %d", n)
	}
}

// A name is declared once, so that two analyses cannot share a cache entry.
func TestAnalysisNameIsUnique(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a second key of one name was declared")
		}
	}()
	NewAnalysisKey("test-words", func(*doc, *AnalysisManager[*doc]) (int, error) { return 0, nil })
}

// The program-wide count counts every computation of every manager.
func TestComputationsCountsEveryManager(t *testing.T) {
	before := Computations("test-len")
	for range 2 {
		Get(NewAnalysisManager[*doc](nil), newDoc("a"), keyLen)
	}
	if got := Computations("test-len") - before; got != 2 {
		t.Fatalf("counted %d computations, want 2", got)
	}
}
