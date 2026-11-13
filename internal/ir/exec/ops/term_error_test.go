package ops

import (
	"reflect"
	"testing"
)

// TestTermErrorSegments checks that Segments cuts a message at its placeholders: fixed text and
// holes alternate in order, adjacent holes and a message that starts or ends with a hole produce no
// empty text segment, and a message without holes is one text segment.
func TestTermErrorSegments(t *testing.T) {
	text := func(s string) MessageSegment { return MessageSegment{Text: s} }
	hole := func(s string) MessageSegment { return MessageSegment{Hole: s} }
	for _, tc := range []struct {
		message string
		want    []MessageSegment
	}{
		{"", nil},
		{"no holes", []MessageSegment{text("no holes")}},
		{"{a}", []MessageSegment{hole("a")}},
		{"{a}{b}", []MessageSegment{hole("a"), hole("b")}},
		{"index {i} out of range for {n} elements", []MessageSegment{text("index "), hole("i"), text(" out of range for "), hole("n"), text(" elements")}},
		{"cannot access .{name}", []MessageSegment{text("cannot access ."), hole("name")}},
		{"{op}: expected a list", []MessageSegment{hole("op"), text(": expected a list")}},
	} {
		if got := (TermError{Text: tc.message}).Segments(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Segments(%q) = %v, want %v", tc.message, got, tc.want)
		}
	}
	// Every message of the table cuts into segments that join back to the message.
	for code, e := range TermErrors {
		joined := ""
		for _, s := range e.Segments() {
			joined += s.Text
			if s.Hole != "" {
				joined += "{" + s.Hole + "}"
			}
		}
		if joined != e.Text {
			t.Errorf("error %v: segments join to %q, want %q", code, joined, e.Text)
		}
	}
}
