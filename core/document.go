package core

import (
	"fmt"
	"reflect"
)

// TextEdit replaces [Start, End) in the position unit of its base Document with Text.
type TextEdit struct {
	Start, End int
	Text       string
}

// DocumentChangeKind is the kind of a Document change notification.
type DocumentChangeKind string

const (
	// ChangeEdit is an accepted edit of the text.
	ChangeEdit DocumentChangeKind = "edit"
	// ChangeOutcome is the outcome of a parse compared with the previous one.
	ChangeOutcome DocumentChangeKind = "outcome"
	// ChangeFullRefresh tells consumers to rebuild from the latest snapshot instead of replaying
	// edits.
	ChangeFullRefresh DocumentChangeKind = "full-refresh"
)

// OutputChange describes whether the whole result of a parse equals the previous one; it is not
// a minimal tree difference.
type OutputChange string

const (
	OutputUnchanged      OutputChange = "unchanged"
	OutputChanged        OutputChange = "changed"
	OutputMayHaveChanged OutputChange = "may-have-changed"
	OutputUnavailable    OutputChange = "unavailable"
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
