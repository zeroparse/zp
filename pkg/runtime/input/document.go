package input

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
