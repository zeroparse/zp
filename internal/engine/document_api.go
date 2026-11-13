package engine

import (
	"zeroparse.dev/zp/internal/engine/document"
)

// The Document, its snapshots and change notifications are declared in the document package. The
// engine re-exports them under the same names for the package engine.

type (
	Document        = document.Document
	ChangeDocument  = document.ChangeDocument
	DocumentVersion = document.DocumentVersion
	DocumentChange  = document.DocumentChange
	Snapshot        = document.Snapshot
	SnapshotNode    = document.SnapshotNode
	SnapshotField   = document.SnapshotField
)
