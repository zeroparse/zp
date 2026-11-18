package state

import (
	"context"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/runtime/program"
)

// A Source is the compiler side of a program compiled from a grammar: what the parses of the
// program build from the grammar on first use, and what the compiler knows about its rules. The
// driver supplies it; a program loaded from a compiled grammar has none.
type Source interface {
	// Runtime returns the run-time program that the parses run.
	Runtime() *program.Program
	// Module compiles the program to a bytecode module, once.
	Module() (*bytecode.Module, error)
	// Core builds the representation that the closure families are built from, polling ctx (nil
	// for none): a context that ends returns a BuildCancelled with its error, and a panic of a
	// method of ctx unwinds as ContextPanic.
	Core(ctx context.Context) (any, error)
	// Family builds the rules of the closure family f from core, by ID; plainDispatch leaves out
	// the budget charge of the loop iterations.
	Family(core any, f Family, plainDispatch bool) ([]*Rule, error)
	// Recognizer compiles the grammar again as a program that builds no tree, and Insertion with
	// the rule flags of missing-token insertion (program.Program.InsertionCycles).
	Recognizer() (Source, error)
	Insertion() (Source, error)
	// RuleType, RuleTypeWithPoll, ShapeSignature and ValueType describe the types of the rules
	// (see the methods of engine.Program of the same names).
	RuleType(name string) (string, bool)
	RuleTypeWithPoll(name string, poll func()) (string, bool)
	ShapeSignature(name string, poll func()) (string, bool)
	ValueType(name string) (string, bool)
}
