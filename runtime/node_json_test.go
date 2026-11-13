package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
)

// referenceJSON builds the JSON shape of a value with encoding/json, without invoking
// Node.MarshalJSON: a string, tuple or list node is {start, end, value} (with rule when it has
// one), and a struct is {type, start, end, fields}. It is intentionally used only for shallow, acyclic test
// values without captures.
func referenceJSON(n *core.Node) any {
	if n == nil {
		return nil
	}
	type node struct {
		Start int32  `json:"start"`
		End   int32  `json:"end"`
		Value any    `json:"value"`
		Rule  string `json:"rule,omitempty"`
	}
	type structValue struct {
		Type   string         `json:"type"`
		Start  int32          `json:"start"`
		End    int32          `json:"end"`
		Fields map[string]any `json:"fields"`
	}
	switch shape(n) {
	case noderep.ShapeString:
		return node{D(n).Start, D(n).End, D(n).Text, n.Rule()}
	case noderep.ShapeTuple, noderep.ShapeList:
		elems := make([]any, len(D(n).Children))
		for i, c := range D(n).Children {
			elems[i] = referenceJSON(c)
		}
		return node{D(n).Start, D(n).End, elems, n.Rule()}
	}
	fields := make(map[string]any, len(D(n).Fields))
	for _, field := range D(n).Fields {
		fields[field.Name] = referenceField(field.Value)
	}
	return structValue{n.Type(), D(n).Start, D(n).End, fields}
}

func referenceField(value any) any {
	switch value := value.(type) {
	case *core.Node:
		return referenceJSON(value)
	case []any:
		if value == nil {
			return []any(nil)
		}
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = referenceField(item)
		}
		return out
	default:
		return value
	}
}

func TestNodeWriteJSONMatchesReferenceShape(t *testing.T) {
	child := nodeOfData(&Data{Kind: noderep.MatchKind, Start: 3, End: 5, Text: "<&>\xff"})
	record := nodeOfData(&Data{Kind: KindOf("Record", "entry"), Start: 1, End: 8, Fields: noderep.Fields{
		{Name: "z", Value: []any{"<tag>", []byte{0, 1, 255}, child}},
		{Name: "a", Value: int64(9007199254740992)},
		{Name: "nilList", Value: []any(nil)},
		{Name: "emptyList", Value: []any{}},
	},
	})
	tuple := nodeOfData(&Data{Kind: KindOf("Seq", "pair"), Start: 1, End: 8, Children: []*core.Node{child, nil, child}})
	for _, n := range []*core.Node{record, tuple} {
		want, err := json.Marshal(referenceJSON(n))
		if err != nil {
			t.Fatal(err)
		}
		got, err := n.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("MarshalJSON mismatch\n got: %s\nwant: %s", got, want)
		}
		var streamed bytes.Buffer
		if err := n.WriteJSON(&streamed); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(streamed.Bytes(), want) {
			t.Fatalf("WriteJSON mismatch\n got: %s\nwant: %s", streamed.Bytes(), want)
		}
	}
}

func TestNodeWriteJSONNilAndFields(t *testing.T) {
	var n *core.Node
	got, err := n.MarshalJSON()
	if err != nil || string(got) != "null" {
		t.Fatalf("nil MarshalJSON = %q, %v; want null, nil", got, err)
	}
	data, err := json.Marshal(n)
	if err != nil || string(data) != "null" {
		t.Fatalf("json.Marshal nil Node = %q, %v; want null, nil", data, err)
	}
	var streamed bytes.Buffer
	if err := n.WriteJSON(&streamed); err != nil || streamed.String() != "null" {
		t.Fatalf("nil WriteJSON = %q, %v; want null, nil", streamed.String(), err)
	}

	fields := noderep.Fields{{Name: "child", Value: nodeOfData(&Data{Kind: noderep.MatchKind, Text: "x"})}, {Name: "bytes", Value: []byte{1, 2}}}
	got, err = nodeOfData(&Data{Kind: KindOf("S", ""), Fields: fields}).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"type":"S","start":0,"end":0,"fields":{"bytes":"AQI=","child":{"start":0,"end":0,"value":"x"}}}`; string(got) != want {
		t.Fatalf("struct MarshalJSON = %s, want %s", got, want)
	}
	listFields := noderep.Fields{{Name: "nil", Value: []any(nil)}, {Name: "empty", Value: []any{}}}
	got, err = nodeOfData(&Data{Kind: KindOf("S", ""), Fields: listFields}).MarshalJSON()
	if want := `{"type":"S","start":0,"end":0,"fields":{"empty":[],"nil":null}}`; err != nil || string(got) != want {
		t.Fatalf("fields nil/empty list JSON = %s, %v; want %s", got, err, want)
	}
}

func TestNodeWriteJSONSharedNodesAndCycles(t *testing.T) {
	leaf := nodeOfData(&Data{Kind: noderep.MatchKind, Text: "shared"})
	root := nodeOfData(&Data{Kind: noderep.SeqKind, Children: []*core.Node{leaf, leaf}})
	data, err := root.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(data, []byte(`"value":"shared"`)); got != 2 {
		t.Fatalf("shared node appeared %d times in %s, want 2", got, data)
	}

	for _, makeCycle := range []struct {
		name string
		make func() *core.Node
	}{
		{"child", func() *core.Node {
			n := nodeOfData(&Data{Kind: noderep.SeqKind})
			D(n).Children = []*core.Node{n}
			return n
		}},
		{"field", func() *core.Node {
			n := nodeOfData(&Data{Kind: KindOf("R", "")})
			D(n).Fields = noderep.Fields{{Name: "self", Value: n}}
			return n
		}},
		{"slice", func() *core.Node {
			var values []any
			values = append(values, nil)
			values[0] = values
			return nodeOfData(&Data{Kind: KindOf("R", ""), Fields: noderep.Fields{{Name: "cycle", Value: values}}})
		}},
	} {
		t.Run(makeCycle.name, func(t *testing.T) {
			_, err := makeCycle.make().MarshalJSON()
			var cycle *json.UnsupportedValueError
			if !errors.As(err, &cycle) {
				t.Fatalf("MarshalJSON error = %T %v, want *json.UnsupportedValueError", err, err)
			}
		})
	}
}

type sentinelJSONError struct{ cause error }

func (e sentinelJSONError) Error() string { return "sentinel marshal failure" }
func (e sentinelJSONError) Unwrap() error { return e.cause }

type failingJSONValue struct{ err error }

func (v failingJSONValue) MarshalJSON() ([]byte, error) { return nil, sentinelJSONError{v.err} }

func TestNodeWriteJSONPreservesLeafErrors(t *testing.T) {
	want := errors.New("leaf cause")
	n := nodeOfData(&Data{Kind: KindOf("R", ""), Fields: noderep.Fields{{Name: "bad", Value: failingJSONValue{want}}}})
	for name, err := range map[string]error{
		"WriteJSON":    n.WriteJSON(io.Discard),
		"MarshalJSON":  func() error { _, err := n.MarshalJSON(); return err }(),
		"json.Marshal": func() error { _, err := json.Marshal(n); return err }(),
	} {
		var marshalerErr *json.MarshalerError
		var sentinel sentinelJSONError
		if !errors.As(err, &marshalerErr) || !errors.As(err, &sentinel) || !errors.Is(err, want) {
			t.Errorf("%s error chain does not preserve leaf error: %T %v", name, err, err)
		}
	}
}

func TestNodeWriteJSONDeepAndStandardLibraryLimit(t *testing.T) {
	const depth = 12_000
	root := nodeOfData(&Data{Kind: noderep.SeqKind})
	current := root
	for i := 1; i < depth; i++ {
		child := nodeOfData(&Data{Kind: noderep.SeqKind})
		D(current).Children = []*core.Node{child}
		current = child
	}
	var sink countingWriter
	if err := root.WriteJSON(&sink); err != nil {
		t.Fatal(err)
	}
	if sink.n < depth*20 {
		t.Fatalf("WriteJSON wrote only %d bytes for %d nodes", sink.n, depth)
	}
	if _, err := root.MarshalJSON(); err != nil {
		t.Fatalf("direct MarshalJSON failed at depth %d: %v", depth, err)
	}
	if _, err := json.Marshal(root); err == nil {
		t.Fatalf("encoding/json unexpectedly accepted %d nested nodes; its output scanner limit remains a boundary", depth)
	} else {
		var marshalerErr *json.MarshalerError
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &marshalerErr) || !errors.As(err, &syntaxErr) {
			t.Fatalf("json.Marshal deep-node error = %T %v, want MarshalerError wrapping the standard scanner error", err, err)
		}
	}
}

type countingWriter struct{ n int }

func (w *countingWriter) Write(p []byte) (int, error) { w.n += len(p); return len(p), nil }

func TestNodeWriteJSONShortWriter(t *testing.T) {
	n := nodeOfData(&Data{Kind: noderep.MatchKind, Text: "x"})
	err := n.WriteJSON(shortWriter{})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("WriteJSON short write error = %v, want io.ErrShortWrite", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

func TestNodeWriteJSONLargeScalarAcrossBufferBoundary(t *testing.T) {
	n := nodeOfData(&Data{Kind: noderep.MatchKind, Text: strings.Repeat("x", 600)})
	want, err := json.Marshal(referenceJSON(n))
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := n.WriteJSON(&got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("large scalar JSON mismatch\n got: %s\nwant: %s", got.Bytes(), want)
	}
}

type partialAfterFlushWriter struct {
	bytes.Buffer
	calls int
	err   error
}

func (w *partialAfterFlushWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		return w.Buffer.Write(p)
	}
	n := len(p) / 2
	w.Buffer.Write(p[:n])
	return n, w.err
}

func TestNodeWriteJSONPartialWriteErrorAfterFlush(t *testing.T) {
	n := nodeOfData(&Data{Kind: noderep.MatchKind, Text: strings.Repeat("x", 600)})
	wantErr := errors.New("writer failed")
	w := &partialAfterFlushWriter{err: wantErr}
	err := n.WriteJSON(w)
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteJSON error = %v, want writer error", err)
	}
	if w.calls != 2 {
		t.Fatalf("writer calls = %d, want prefix flush and partial scalar write", w.calls)
	}
	encoded, err := json.Marshal(D(n).Text)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := `{"start":0,"end":0,"value":` + string(encoded[:len(encoded)/2])
	if w.String() != wantPrefix {
		t.Fatalf("written prefix = %q, want %q", w.String(), wantPrefix)
	}
}

func TestNodeWriteJSONInvalidValueParity(t *testing.T) {
	n := nodeOfData(&Data{Kind: KindOf("R", ""), Fields: noderep.Fields{{Name: "bad", Value: math.NaN()}}})
	_, gotErr := n.MarshalJSON()
	_, wantErr := json.Marshal(struct {
		Type   string         `json:"type"`
		Start  int32          `json:"start"`
		End    int32          `json:"end"`
		Fields map[string]any `json:"fields,omitempty"`
	}{Type: "Seq", Fields: map[string]any{"bad": math.NaN()}})
	var gotUnsupported, wantUnsupported *json.UnsupportedValueError
	if !errors.As(gotErr, &gotUnsupported) || !errors.As(wantErr, &wantUnsupported) || gotUnsupported.Str != wantUnsupported.Str {
		t.Fatalf("unsupported-value errors differ: got %T %v, want %T %v", gotErr, gotErr, wantErr, wantErr)
	}
}

func TestNodeJSONSliceIdentityAllowsRepeatedAcyclicEdges(t *testing.T) {
	shared := []any{"ok"}
	n := nodeOfData(&Data{Kind: KindOf("R", ""), Fields: noderep.Fields{{Name: "a", Value: shared}, {Name: "b", Value: shared}}})
	got, err := n.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"a":["ok"],"b":["ok"]`) {
		t.Fatalf("shared acyclic list did not serialize at both edges: %s", got)
	}
}

func TestNodeWriteJSONSliceIdentityIgnoresCapacity(t *testing.T) {
	x := make([]any, 1, 2)
	x[0] = x[:1:1]
	root := nodeOfData(&Data{Fields: noderep.Fields{{Name: "x", Value: x}}})
	var prefix bytes.Buffer
	err := root.WriteJSON(&prefix)
	var cycle *json.UnsupportedValueError
	if !errors.As(err, &cycle) {
		t.Fatalf("capacity-limited slice cycle = %v, want UnsupportedValueError", err)
	}
	// Reject the first re-entry, before emitting another list delimiter.
	const wantPrefix = `{"type":"","start":0,"end":0,"fields":{"x":[`
	if prefix.String() != wantPrefix {
		t.Fatalf("cycle prefix = %q, want %q", prefix.String(), wantPrefix)
	}
	if data, err := root.MarshalJSON(); data != nil || !errors.As(err, &cycle) {
		t.Fatalf("MarshalJSON cycle = %q, %v, want nil bytes and UnsupportedValueError", data, err)
	}
}
