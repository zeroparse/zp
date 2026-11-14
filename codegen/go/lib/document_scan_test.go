package libgo

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
)

// scanDocumentRules returns a main rule that walks the same scan site from the same start four
// times, and the rule of the fourth walk. A walk matches the a characters, each iteration also
// reading the two characters after the one it matches, so that the walk examines two positions past
// the character where it ends. The site turns hot at the second walk, the third records itself, and
// the fourth takes the record (ScanReach) while its rule's memo entry is recorded; hit tells it did.
func scanDocumentRules(hit *bool) (main, last *Rule) {
	const site = 1
	walk := func(p *Parser) (*core.Node, bool) {
		start, count := p.Pos, 0
		for {
			ch, size, ok := p.Peek()
			if !ok || ch != 'a' {
				p.Expect(p.Pos, 0)
				break
			}
			at := p.Pos
			p.Pos += size
			p.Peek() // the two characters after the matched one
			p.Pos++
			p.Peek()
			p.Pos = at + size
			count++
			if count == scanHotSpan {
				count = p.ScanReach(site, start, count)
			}
		}
		if count >= scanHotSpan {
			p.ScanEnd(site, start, count)
		}
		return nodeOfData(&Data{Kind: coreimpl.ListKind, Start: int32(start), End: int32(p.Pos)}), true
	}
	plain := &Rule{Id: 1, Name: "plain", Seen: -1}
	plain.Body = func(p *Parser, _ int) (*core.Node, bool) { return walk(p) }
	last = &Rule{Id: 2, Name: "last", Memo: true, DocMemo: true, Seen: -1}
	last.Body = func(p *Parser, _ int) (*core.Node, bool) {
		v, ok := walk(p)
		*hit = p.scanWalk.hit
		return v, ok
	}
	main = &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
	main.Body = func(p *Parser, _ int) (*core.Node, bool) {
		for range 3 {
			p.Pos = 0
			if _, ok := p.Call(plain, 0); !ok {
				return nil, false
			}
		}
		p.Pos = 0
		v, ok := p.Call(last, 0)
		if !ok {
			return nil, false
		}
		for {
			_, size, ok := p.Peek()
			if !ok {
				break
			}
			p.Pos += size
		}
		return v, true
	}
	return main, last
}

// TestPrivateDocumentScanRecordHitExaminedEnd checks that a walk of a scan site that takes the
// record of an earlier walk of its start within a Document parse examines what the record's walk
// examined: its rule's entry records the end that a fresh walk records, and an edit of the last
// position before that end drops the entry, so that the parse after it equals a parse of a new
// Document.
func TestPrivateDocumentScanRecordHitExaminedEnd(t *testing.T) {
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		const run = 20
		text := strings.Repeat("a", run) + "bcd"
		var hit bool
		main, last := scanDocumentRules(&hit)
		d := testDocumentState(main, text, unit)
		o := parseOptions{unit: unit, maxDepth: defaultMaxDepth}
		if _, err := d.parse(o); err != nil {
			t.Fatalf("unit %d: %v", unit, err)
		}
		if !hit {
			t.Fatalf("unit %d: the last walk did not take a record", unit)
		}
		// The last matched character, at run-1, reads the two characters after it.
		want := run + 2
		e := d.memo.get(last.Id, 0, 0, nil)
		if e == nil || e.examined != want {
			t.Fatalf("unit %d: entry = %+v, want examined %d", unit, e, want)
		}
		// An edit of the position before the recorded end must drop the entry.
		if err := d.edit(run+1, run+2, "x"); err != nil {
			t.Fatal(err)
		}
		if e := d.memo.get(last.Id, 0, 0, nil); e != nil {
			t.Fatalf("unit %d: entry survived an edit inside its examined range", unit)
		}
		got, gotErr := d.parse(o)
		var h2 bool
		fresh, _ := scanDocumentRules(&h2)
		wantNode, wantErr := testDocumentState(fresh, d.text, unit).parse(o)
		if (gotErr == nil) != (wantErr == nil) || documentNodeString(got) != documentNodeString(wantNode) {
			t.Fatalf("unit %d: edited parse = %s, %v; fresh = %s, %v", unit, documentNodeString(got), gotErr, documentNodeString(wantNode), wantErr)
		}
	}
}
