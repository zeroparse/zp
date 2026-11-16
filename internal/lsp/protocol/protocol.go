// Package protocol implements the base protocol of the Language Server Protocol, JSON-RPC 2.0 framing
// with Content-Length headers, and the subset of its message types that the server uses.
package protocol

// The subset of the Language Server Protocol (version 3.17) types that the server uses.

// Position is a position in a text document: a zero-based line and a zero-based offset in UTF-16
// code units in that line.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a range in a text document; End is exclusive.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type DidOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// ContentChange is a change of a document: Text replaces Range, or the whole document if Range is
// nil.
type ContentChange struct {
	Range *Range `json:"range"`
	Text  string `json:"text"`
}

type DidChangeParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []ContentChange                 `json:"contentChanges"`
}

type DidSaveParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text"`
}

type DidCloseParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SeverityError is the severity of diagnostics (all of them are errors).
const SeverityError = 1

// TextEdit replaces Range with NewText.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

type PrepareRenameResult struct {
	Range       Range  `json:"range"`
	Placeholder string `json:"placeholder"`
}

// Symbol kinds.
const (
	SymbolKindClass     = 5
	SymbolKindField     = 8
	SymbolKindInterface = 11
	SymbolKindFunction  = 12
	SymbolKindString    = 15
	SymbolKindStruct    = 23
)

// SymbolInformation is a symbol of a document outline for clients without hierarchical symbols.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// DocumentSymbol is a symbol of a document outline.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// Completion item kinds.
const (
	CompletionKindFunction = 3
	CompletionKindField    = 5
	CompletionKindVariable = 6
	CompletionKindClass    = 7
	CompletionKindProperty = 10
	CompletionKindKeyword  = 14
	CompletionKindSnippet  = 15
	CompletionKindStruct   = 22
)

// InsertSnippet is the format of insert text in the snippet syntax.
const InsertSnippet = 2

// CompletionItem is a completion proposal.
type CompletionItem struct {
	Label            string         `json:"label"`
	Kind             int            `json:"kind,omitempty"`
	Detail           string         `json:"detail,omitempty"`
	Documentation    *MarkupContent `json:"documentation,omitempty"`
	SortText         string         `json:"sortText,omitempty"`
	InsertText       string         `json:"insertText,omitempty"`
	InsertTextFormat int            `json:"insertTextFormat,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// Document highlight kinds.
const (
	HighlightRead  = 2
	HighlightWrite = 3
)

type DocumentHighlight struct {
	Range Range `json:"range"`
	Kind  int   `json:"kind"`
}
