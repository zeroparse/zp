package zpls

import "zeroparse.dev/zp/internal/lsp/protocol"

// Diagnostic is a problem in a document. Its data carries the server's own finding key, which
// is why it is defined here rather than in the protocol package.
type Diagnostic struct {
	Range    protocol.Range `json:"range"`
	Severity int            `json:"severity"`
	Source   string         `json:"source"`
	Message  string         `json:"message"`
	Code     string         `json:"code,omitempty"`
	Data     *findingData   `json:"data,omitempty"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
