package runtime

import (
	"fmt"
	"reflect"
)

// CacheStats describes the incremental history a Document retains. EstimatedBytes includes
// backing buffers and run records, but excludes map buckets, allocator metadata, the document
// text, result trees, shared expectation, error and environment payloads, and strings shared with
// results the caller holds.
type CacheStats struct {
	LiveMemoEntries    int
	RetiredMemoEntries int
	RepetitionElements int
	EstimatedBytes     uint64
}

// ParseStats counts the rule evaluations and reuses of the last parse. Reused is MemoHits plus
// ResumedElements.
type ParseStats struct {
	Evaluated       int // rule body evaluations
	Reused          int // memo hits and resumed repetition elements
	MemoHits        int // memo results reused
	ResumedElements int // repetition elements reused
}

// SnapshotCaptureError reports a field value of a type that a snapshot cannot capture. It
// retains the type and the parse error, never the value.
type SnapshotCaptureError struct {
	Path       string
	ValueType  reflect.Type
	ParseError error
}

func (e *SnapshotCaptureError) Error() string {
	return fmt.Sprintf("cannot capture snapshot field %s with type %v", e.Path, e.ValueType)
}

// Unwrap returns the parse error.
func (e *SnapshotCaptureError) Unwrap() error { return e.ParseError }
