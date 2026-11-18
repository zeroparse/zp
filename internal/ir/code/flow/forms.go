package flow

import (
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
)

// Forms record where the lowering from Core IR wrote the forms that a Flow pass may replace, with
// the Core nodes they come from (Attrs.Forms). The lowering writes one form of each construct: a
// class as a peek, a test and an advance, a choice with guards that only peek, and a terminal region
// as its literals and classes. The passes of internal/middleend/flowpass choose another form where
// the target and the function allow it, and use the record to find the statements and the facts of
// Core IR that the choice depends on. The record is provenance: the statements of the function stay
// what the verifier checks and the printers print.
//
// A form holds its statements in a block of its own, labeled form<N> as every Flow block has a label
// and left by no exit, which flowpass.Finish splices into the enclosing statement list after the
// passes, so that a form leaves no trace in what is printed.
type Forms struct {
	// Classes are the forms of the classes, in the order the lowering wrote them.
	Classes []*Form
	// Regions are the forms of the terminal regions (corepass.TerminalRegions) of a local function.
	Regions []*Form
	// Seqs are the value-free sequences with at least two classes among their items.
	Seqs []*SeqForm
	// Choices are the choices with an alternative that begins with a class.
	Choices []*ChoiceForm
	// Events record, in the order of the lowering, each use of a variable shared by the whole function
	// (ch, size, ok) and each choice: Finish places the shared variables, and a pass places the
	// variables it makes, where the lowering would have made them had it written the forms the passes
	// chose.
	Events []*Event
	// N is the counter of the numbered names (x1, L2, ...) when the lowering finished; a pass names
	// the variables it makes above it.
	N int
	// Named holds the variables that the statements of the function named when the lowering
	// finished; Order is Attrs.Vars then.
	Named map[string]bool
	Order []string
	// Placed holds the event at which each variable a pass made belongs.
	Placed map[string]*Event
}

// FormKind says what a form holds.
type FormKind uint8

const (
	// FormClass is a class: peek(ch, size, ok); if !ok || !inClass(ch) { expect; fail }; then, when it
	// builds its value, start = pos; advance(size); and value = match(start, text(start)), or else
	// advance(size).
	FormClass FormKind = iota
	// FormRegion is a terminal region: a value-free expression of literals and classes with a choice,
	// which skipTerminal can match as one program.
	FormRegion
)

// Form is one form the lowering wrote.
type Form struct {
	Kind FormKind
	// Block is the block of its own that holds the statements of the form.
	Block *code.Block
	// Node is the Core node: the *core.ClassNode of a class, the root of a region.
	Node core.Node
	// Build reports that the form builds the node's value.
	Build bool
	// Fail is the label the form leaves on failure, and Loop reports that it is a loop's (left by
	// a break).
	Fail string
	Loop bool
	// Guard is, for a class that begins a guarded alternative (core.Alt.GuardedClass), the choice
	// whose guard peeked the class's character.
	Guard *ChoiceForm
	// Outer are the forms around this one, outermost first.
	Outer []*Form
	// By is the name of the pass that replaced the statements the lowering wrote, or "" if none did.
	By string
}

// Live reports whether the statements the lowering wrote for s are still in the function: neither
// s nor a form around it was replaced.
func (s *Form) Live() bool {
	if s.By != "" {
		return false
	}
	for _, o := range s.Outer {
		if o.By != "" {
			return false
		}
	}
	return true
}

// FailJump returns the statement that leaves the form on failure.
func (s *Form) FailJump() code.Stmt {
	if s.Loop {
		return &code.Break{Label: s.Fail}
	}
	return &code.Exit{Label: s.Fail}
}

// SeqForm is a value-free sequence (core.Seq) with at least two classes among its items.
type SeqForm struct {
	Node *core.Seq
	// Items holds the form of each item that is a class, in the order of the items, and nil for
	// the others.
	Items []*Form
	Outer []*Form
}

// Live reports whether the statements of the sequence are still in the function.
func (s *SeqForm) Live() bool {
	for _, o := range s.Outer {
		if o.By != "" {
			return false
		}
	}
	return true
}

// ChoiceForm is a choice (core.Choice) with an alternative that begins with a class.
type ChoiceForm struct {
	Node *core.Choice
	// Peek is the peek of the choice's guards, or nil if no alternative has a guard.
	Peek *code.Set
	// Event is the event of the choice, at the point where the lowering would make a variable of the
	// choice before its alternatives.
	Event *Event
	Outer []*Form
}

// Live reports whether the statements of the choice are still in the function.
func (s *ChoiceForm) Live() bool {
	for _, o := range s.Outer {
		if o.By != "" {
			return false
		}
	}
	return true
}

// Event is a point of the lowering: the use of a shared variable, or a choice.
type Event struct {
	// Name is the shared variable used, or empty at a choice.
	Name string
	// Vars is the number of variables (Attrs.Vars) the lowering had made at the event.
	Vars  int
	Outer []*Form
}

// Live reports whether the statements written at the event are still in the function.
func (e *Event) Live() bool {
	for _, o := range e.Outer {
		if o.By != "" {
			return false
		}
	}
	return true
}

// NamedVars returns the locals that the statements of b read or assign.
func NamedVars(b *code.Block) map[string]bool {
	out := map[string]bool{}
	code.Walk(b, func(s code.Stmt) {
		switch s := s.(type) {
		case *code.Set:
			for _, d := range s.Dst {
				if d.Name != "" && !d.Reg {
					out[d.Name] = true
				}
			}
		case *code.Let:
			for _, n := range s.Names {
				out[n] = true
			}
		}
	}, func(e code.Expr) {
		if l, ok := e.(*code.Local); ok {
			out[l.Name] = true
		}
	})
	return out
}
