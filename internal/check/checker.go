package check

import (
	"sync"

	"zeroparse.dev/zp/internal/cancel"
)

// Checker checks the types of one program and keeps what the checks compute for it. It reads the
// program through Program.
type Checker struct {
	Program
	// Typed holds the types found by CheckTypes (nil if it did not run).
	Typed *TypeInfo
	// shapeControl is the compilation's cancellation control while the program is being built, so
	// that the shape analysis of rule bodies polls it; nil otherwise.
	shapeControl *cancel.Control
	// bottoms are the rules of type _|_ (bottomRules), computed once.
	bottomsOnce sync.Once
	bottoms     map[string]bool
	// nodeTypeCache holds the results of NodeTypeOf by type name: a NodeTy, or false.
	nodeTypeCache sync.Map
	// structHasZero caches whether the struct types have a zero value (HasZero) by name.
	structHasZero sync.Map
}

// NewChecker returns the checker of the program p.
func NewChecker(p Program) *Checker { return &Checker{Program: p} }

// SetShapeControl sets the cancellation control that the shape analysis of rule bodies polls, nil
// outside a compilation.
func (p *Checker) SetShapeControl(control *cancel.Control) { p.shapeControl = control }

// ShapeControl returns the control set by SetShapeControl.
func (p *Checker) ShapeControl() *cancel.Control { return p.shapeControl }
