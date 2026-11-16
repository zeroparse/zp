package zpls

import (
	"encoding/json"

	"zeroparse.dev/zp/internal/lsp/protocol"
)

func (s *Server) semanticTokens(params json.RawMessage) (any, *protocol.RPCError) {
	return renderRequest(s.requestContext, func(control *renderControl) (any, *protocol.RPCError) {
		return s.semanticTokensControlled(params, control)
	})
}

func (a *analysis) semanticTokens() []int {
	return a.semanticTokensControlled(nil)
}
