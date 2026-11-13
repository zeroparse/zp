package engine

import (
	"zeroparse.dev/zp/internal/engine/state"
)

// The parse options, results, errors and trace events are declared in the state package, which
// every backend shares. The engine re-exports them under the same names for the package engine.

type (
	Backend      = state.Backend
	ParseOptions = state.ParseOptions
	TraceKind    = state.TraceKind
	TraceEvent   = state.TraceEvent
	GNode        = state.GNode
	Tuple        = state.Tuple
	List         = state.List
	Struct       = state.Struct
	ErrorValue   = state.ErrorValue
)

const (
	Default           = state.Default
	Closure           = state.Closure
	Bytecode          = state.Bytecode
	BytecodeIterative = state.BytecodeIterative

	DefaultMaxDepth          = state.DefaultMaxDepth
	DefaultMaxDepthIterative = state.DefaultMaxDepthIterative

	TraceEnter = state.TraceEnter
	TraceExit  = state.TraceExit
)

// ComposeTrace returns a trace function that calls prev and then next; see state.ComposeTrace.
func ComposeTrace(prev, next func(TraceEvent)) func(TraceEvent) {
	return state.ComposeTrace(prev, next)
}
