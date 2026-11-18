package state

import "zeroparse.dev/zp/internal/runtime/program"

// staticProgram and staticRule are the run-time program and rule. SharedProgram and Rule embed
// them under these names, so the embedded fields stay unexported.
type (
	staticProgram = program.Program
	staticRule    = program.Rule
)

// ExpID and MsgBit are the run-time program's expectation IDs and the bit that marks an @error
// message; the runtime records them under these names, as the runtime package of generated
// parsers does.
type ExpID = program.ExpID

const MsgBit = program.MsgBit

// ExpFar is the placeholder of a record that does not keep expectations; see program.ExpFar.
const ExpFar = program.ExpFar
