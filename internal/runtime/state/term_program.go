package state

import (
	"zeroparse.dev/zp/internal/runtime/program"

	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// Terms of the engine lowered from Exec IR.
//
// The engine evaluates an action, a predicate or the value of a binding as the function of Exec IR
// that lower.Term makes of its Core IR term, as generated parsers do: the same compositions of the
// operators of terms (term_exec.go), in the same order, with the same failures. The compiler
// translates the function once, when the program is built, into the instructions of a TermProgram
// (internal/backend/actionlower), which TermProgram.Eval runs; a compiled grammar file holds them
// (Export, LoadTerm). Instructions read and write registers of four files, by the Exec IR type
// of the local they hold: values (Datum, Obj), integers (int and bool), element slices (Seq) and
// strings (Str), so that no integer, slice or string is boxed to be held.
//
// Every composition that can fail is followed in the lowered function by the return of its
// failure; the translation folds the two into the one instruction, whose failure ends Eval.

// TermProgram is a term of the action language compiled for the engine. It is immutable and may be
// shared by parses that run concurrently.
type TermProgram struct {
	Code []TermInstr
	// The sizes of the register files.
	NA, NN, NS, NT int32
	Consts         []any
	Names          []string // term.name and the names of Operators Rule locals and struct fields
	Members        []TermMember
	NodeTypes      []*nodes.NodeTypeKinds
	Casts          []TermCast
	Structs        []TermStruct
}

type TermMember struct {
	Kind        int
	Name, Field string
}

type TermCast struct {
	Type      *program.FieldType
	Line, Col int
}

type TermStruct struct {
	Kind   *valueimpl.Kind
	Fields int
}

type TermOp uint8

const (
	TNop TermOp = iota
	// Moves and constants.
	TConst // a[dst] = consts[a]
	TIntK  // n[dst] = a
	TMoveA // a[dst] = a[a]
	TMoveN // n[dst] = n[a]
	TMoveS // s[dst] = s[a]
	TMoveT // t[dst] = t[a]
	TCap   // a[dst] = the capture in slot a of the frame
	TLocal // a[dst] = the Operators Rule local names[a]
	TVar   // a[dst] = the variable in slot a
	TUnbox // a[dst] = unboxed a[a]
	TFromB // a[dst] = bool n[a]
	TFromT // a[dst] = string t[a]
	TNot   // n[dst] = !n[a]
	TLt    // n[dst] = n[a] < n[b]
	TAdd   // n[dst] = n[a] + n[b]
	TEqN   // n[dst] = n[a] == n[b]
	TIsNil // n[dst] = a[a] == nil
	// Compositions; those marked fallible end Eval with their failure.
	TMember   // fallible: a[dst] = member(kind, a[a]) of members[b]
	TUnary    // fallible: a[dst] = unary(b, a[a])
	TBinary   // fallible: a[dst] = binary(c, a[a], a[b])
	TLogic    // fallible: n[dst] = logic(b, a[a])
	TText     // fallible: t[dst] = text(a[a])
	TTextEq   // n[dst] = t[a] == t[b]
	TLength   // fallible: a[dst] = length(a[a])
	TConvert  // fallible: a[dst] = convert(nodeTypes[b], a[a])
	TConforms // n[dst] = conforms(casts[b], a[a])
	TCastErr  // a[dst] = the failure of the cast casts[b] of a[a]
	TKids     // n[dst] = the number of gathered elements
	TPush     // fallible: push(names[b], a[a])
	TPushAll  // fallible: pushItems(names[b], a[a])
	TAbove    // s[dst] = the elements gathered above n[a], taken off
	THull     // a[dst] = hull(b != 0, s[a])
	TItems    // fallible: s[dst] = items(names[b], a[a])
	TItemAt   // a[dst] = itemAt(s[a], n[b], c != 0)
	TSeqLen   // n[dst] = len(s[a])
	TStep     // charge the step of an element
	TKeep     // keep element n[b] of s[a]
	TFilter   // fallible: n[dst] = filterKeep(a[a])
	TCopy     // a[dst] = a new list of the elements s[a]
	TStruct   // a[dst] = a new struct structs[a]
	TSetField // set field names[c] of the struct a[a] to a[b]
	TCover    // n[dst] = cover(a[a], a[b], n[c])
	TMade     // made(a[a], n[b])
	// Control.
	TJump   // continue at dst
	TJumpF  // continue at dst if n[a] is false
	TRet    // return a[a]
	TRetErr // return the failure a[a]
	// Fused.
	TField // step; set field names[c] of the struct a[a] to a[b]; n[dst] = cover(a[a], a[b], n[dst])
)

type TermInstr struct {
	Op      TermOp
	Dst     int32
	A, B, C int32
}
