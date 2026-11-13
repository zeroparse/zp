package engine

import (
	"zeroparse.dev/zp/internal/engine/core"
)

// The parse options, results, errors and trace events are declared in the core package, which
// every backend shares. The engine re-exports them under the same names for the package engine.

type (
	Backend      = core.Backend
	ParseOptions = core.ParseOptions
	TraceKind    = core.TraceKind
	TraceEvent   = core.TraceEvent
	GNode        = core.GNode
	Tuple        = core.Tuple
	List         = core.List
	Struct       = core.Struct
	ErrorValue   = core.ErrorValue
)

const (
	Default           = core.Default
	Closure           = core.Closure
	Bytecode          = core.Bytecode
	BytecodeIterative = core.BytecodeIterative

	DefaultMaxDepth          = core.DefaultMaxDepth
	DefaultMaxDepthIterative = core.DefaultMaxDepthIterative

	TraceEnter = core.TraceEnter
	TraceExit  = core.TraceExit
)

// ComposeTrace returns a trace function that calls prev and then next; see core.ComposeTrace.
func ComposeTrace(prev, next func(TraceEvent)) func(TraceEvent) {
	return core.ComposeTrace(prev, next)
}
