package runtime

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

// A Document counts the memo it retains, keeps it under a limit the count fits, and drops it before
// parsing when the retained bytes alone exceed the limit. A retained memo that the limit admits
// stays, even when the parse then passes the limit that a dropped memo would have met; the
// interruption drops it, so the next parse meets the limit.
func TestDocumentMemoryLimitRetainedMemo(t *testing.T) {
	main, _ := documentTestRules()
	text := strings.Repeat("a", 2000)
	opts := func(mem uint64) parseOptions {
		return parseOptions{maxDepth: defaultMaxDepth, limits: parse.Limits{Memory: mem}}
	}
	fresh := func(mem uint64) error {
		_, err := testDocumentState(main, text, parse.CodePoints).parse(opts(mem))
		return err
	}
	lo, hi := uint64(0), uint64(1<<40)
	for hi-lo > 1 {
		if mid := lo + (hi-lo)/2; fresh(mid) == nil {
			hi = mid
		} else {
			lo = mid
		}
	}
	m := hi
	if err := fresh(m); err != nil {
		t.Fatalf("limit %d: %v", m, err)
	}
	var le *parse.LimitError
	if err := fresh(m - 1); !errors.As(err, &le) || le.Kind != parse.LimitMemory || le.Used != m {
		t.Fatalf("limit %d: %v", m-1, err)
	}

	d := testDocumentState(main, text, parse.CodePoints)
	if _, err := d.parse(opts(0)); err != nil {
		t.Fatal(err)
	}
	retained := d.memo.chargedBytes()
	if retained == 0 || retained > m {
		t.Fatalf("retained %d bytes, a fresh parse counts %d", retained, m)
	}
	if _, err := d.parse(opts(parse.Unlimited - 1)); err != nil || d.stats.Reused == 0 {
		t.Fatalf("finite limit: %v, stats %+v", err, d.stats)
	}

	// Retained bytes beyond the limit are dropped, and the parse counts as a fresh one.
	d.memo.memBytes += m
	if _, err := d.parse(opts(m)); err != nil || d.stats.Reused != 0 {
		t.Fatalf("retained beyond the limit: %v, stats %+v", err, d.stats)
	}

	// Retained bytes equal to the limit stay, and the parse passes it.
	if _, err := d.parse(opts(0)); err != nil {
		t.Fatal(err)
	}
	d.memo.memBytes += m - d.memo.chargedBytes()
	if _, err := d.parse(opts(m)); !errors.As(err, &le) || le.Kind != parse.LimitMemory {
		t.Fatalf("retained at the limit: %v", err)
	}
	// The interruption drops the retained memo, so the next parse counts as a fresh one.
	if _, err := d.parse(opts(m)); err != nil || d.stats.Reused != 0 {
		t.Fatalf("parse after the interruption: %v, stats %+v", err, d.stats)
	}
}
