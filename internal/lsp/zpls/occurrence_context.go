package zpls

import (
	"encoding/json"

	"zeroparse.dev/zp/internal/lsp/protocol"
)

func (s *Server) references(params json.RawMessage) (any, *protocol.RPCError) {
	return renderRequest(s.requestContext, func(control *renderControl) (any, *protocol.RPCError) {
		return s.referencesControlled(params, control)
	})
}

func (s *Server) documentHighlight(params json.RawMessage) (any, *protocol.RPCError) {
	return renderRequest(s.requestContext, func(control *renderControl) (any, *protocol.RPCError) {
		return s.documentHighlightControlled(params, control)
	})
}

func (s *Server) prepareRename(params json.RawMessage) (any, *protocol.RPCError) {
	return renderRequest(s.requestContext, func(control *renderControl) (any, *protocol.RPCError) {
		return s.prepareRenameControlled(params, control)
	})
}

func (s *Server) rename(params json.RawMessage) (any, *protocol.RPCError) {
	return renderRequest(s.requestContext, func(control *renderControl) (any, *protocol.RPCError) {
		return s.renameControlled(params, control)
	})
}

func (a *analysis) occurrencesOf(o occurrence) []occurrence {
	return a.occurrencesOfControlled(o, nil)
}

func (a *analysis) checkNewName(kind symKind, old, name string) *protocol.RPCError {
	return a.checkNewNameControlled(kind, old, name, nil)
}

func (a *analysis) checkUnused(kind symKind, name string) *protocol.RPCError {
	return a.checkUnusedControlled(kind, name, nil)
}
