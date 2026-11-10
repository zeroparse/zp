package runtime

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// The struct table (conform.StructCases) on the compositions cover and made of the node and the typed
// runtime (termCover and termMade, printed from Exec IR), through the methods that a generated parser
// calls to make a struct.

func TestValueStructTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		c, vals := nodeBuiltinFixture(controlled)
		vals[conform.StructLate] = &Node{RawKind: RawKindMatch, RawTerminal: true, RawText: "q", RawStart: 7, RawEnd: 9}
		c.start, c.end = conform.StructRuleStart, conform.StructRuleEnd
		if err := conform.RunStructs("node", func(fields []string) string {
			var kv []any
			for i, f := range fields {
				kv = append(kv, fmt.Sprintf("F%d", i), vals[f])
			}
			before := len(c.P.created)
			n := c.NewStruct("S", kv...).(*Node)
			return structResult(n.RawStart, n.RawEnd, len(c.P.created) == before+1 && c.P.created[before] == n)
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
		tc, tvals := typedBuiltinFixture(controlled)
		tvals[conform.StructLate] = &TMatch{TNode: TNode[string]{Span: Span{7, 9}, Value: "q"}}
		tc.Start, tc.End = conform.StructRuleStart, conform.StructRuleEnd
		if err := conform.RunStructs("typed", func(fields []string) string {
			var fs []any
			for _, f := range fields {
				fs = append(fs, tvals[f])
			}
			before := len(tc.P.Created)
			n := &TSeq{Typ: "S"}
			tc.MadeOf(n, fs...)
			return structResult(int32(n.Start), int32(n.End), len(tc.P.Created) == before+1 && tc.P.Created[before] == TVal(n))
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}

// structResult is the range of a struct as the struct table writes it, with conform.StructRecorded if the
// context recorded it.
func structResult(start, end int32, recorded bool) string {
	s := fmt.Sprintf("%d-%d", start, end)
	if recorded {
		s += conform.StructRecorded
	}
	return s
}
