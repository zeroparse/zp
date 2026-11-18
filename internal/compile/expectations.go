package compile

import (
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/runtime/program"
)

// ExpID is an expectation (an index into descs); see program.ExpID.
type ExpID = program.ExpID

// MsgBit, ExpFar and the fixed expectations are those of the run-time program.
const (
	MsgBit        = program.MsgBit
	ExpFar        = program.ExpFar
	IDAny         = program.IDAny
	IDBeginInput  = program.IDBeginInput
	IDEndInput    = program.IDEndInput
	IDBeginLine   = program.IDBeginLine
	IDEndLine     = program.IDEndLine
	NumFixedDescs = program.NumFixedDescs
)

// FixedDescs are the descriptions of the fixed expectations, in ExpID order: the Core IR's table,
// which the run-time program's (program.FixedDescs) equals.
var FixedDescs = core.FixedDescs
