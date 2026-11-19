// Package flowplan decides, for one rule table, which functions are lowered to Flow IR and in which
// form, before any backend prints or selects them. Every backend that runs the Node or recognizer
// table from Flow IR plans it here, so the generated Go parser and the bytecode VM run the same
// functions: a value-free flat rule where its body allows one, and otherwise the framed body that
// runs inside the rule frame its runtime owns.
//
// A plan lists FuncSpecs in the order a backend lowers them, keyed by their use in the table, and
// fixes the capture-slot layout of each rule's frame, from which both the Go table rows and the
// VM's rule scopes derive. The route facts are analyses of Core IR and of the rules of the table
// (Rule, RecEligible, NodeFrameOK); the refusal of a flat function (Refused) is an analysis of its Exec IR.
package flowplan

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Use is what a planned function is in its table.
type Use uint8

const (
	// RuleBody is the body of a rule's table row: a flat rule or a framed body.
	RuleBody Use = iota
	// OperatorsSkip is the skip line of an Operators Rule.
	OperatorsSkip
	// OperatorsLine is a line of an Operators Rule: its operands in order, then its operators in the
	// order of their levels.
	OperatorsLine
)

// Key names a planned function by its use, its rule's ID and, for an Operators Rule line, the line's
// index.
type Key struct {
	Use  Use
	Rule int
	Line int
}

// Form is the form a function is lowered in.
type Form uint8

const (
	// Flat is a whole rule as a value-free function with no frame (flowlower.Lower).
	Flat Form = iota
	// Frame is a body that runs inside the frame its runtime owns (flowlower.LowerFrame).
	Frame
)

// FuncSpec is one function of a plan: exactly the arguments of its lowering.
type FuncSpec struct {
	Key  Key
	Form Form
	// Rule is the Core rule the function belongs to, and Name the name of the function.
	Rule *core.Rule
	Name string
	// Body, Slots and Build are the framed body, its capture slots and whether it builds its value;
	// they are unset for a flat function.
	Body  core.Node
	Slots []core.Slot
	Build bool
	// Fallback is the framed body that replaces a flat function that Refused rejects, or nil if the
	// rule has none (the table cannot run the rule then).
	Fallback *FuncSpec
}

// Lower lowers the function of s in the Core IR m with the options of its table.
func (s *FuncSpec) Lower(m *core.Module, opts flowlower.Options) (*code.Module, error) {
	if s.Form == Flat {
		return flowlower.Lower(m, s.Rule, s.Name, opts)
	}
	return flowlower.LowerFrame(m, s.Rule, s.Body, s.Slots, s.Build, s.Name, opts)
}

// Table is the plan of one rule table.
type Table struct {
	// Funcs holds the functions in the order a backend lowers them: the bodies of the rules by ID,
	// then the skip and the lines of each Operators Rule.
	Funcs []*FuncSpec
	// Layouts holds the capture-slot layout of each rule's frame by rule ID.
	Layouts [][]string
	byKey   map[Key]*FuncSpec
}

// Func returns the function planned for the use k, or false if the table has none: the rule is left
// out, or Flow IR cannot lower it, which a backend reports.
func (t *Table) Func(k Key) (*FuncSpec, bool) {
	if t == nil {
		return nil, false
	}
	s, ok := t.byKey[k]
	return s, ok
}

// CheckLayout reports an error if the frame of rule has slots other than its planned layout. A
// backend calls it with the slots of each table row it emits.
func (t *Table) CheckLayout(rule int, name string, slots []string) error {
	if !slices.Equal(t.Layouts[rule], slots) && (len(t.Layouts[rule]) != 0 || len(slots) != 0) {
		return fmt.Errorf("internal error: the frame of rule %s has slots %v, not the planned %v", name, slots, t.Layouts[rule])
	}
	return nil
}

// Plan plans the Node or recognizer table whose rules, value-free twins included, are rules by ID
// and whose Core IR, after the Core passes of the table, is m. live reports whether a rule is
// emitted; nil emits every rule.
func Plan(m *core.Module, rules []*Rule, live func(rule int) bool) *Table {
	t := &Table{Layouts: make([][]string, len(rules)), byKey: map[Key]*FuncSpec{}}
	add := func(s *FuncSpec) {
		t.Funcs = append(t.Funcs, s)
		t.byKey[s.Key] = s
	}
	for _, r := range rules {
		cr := m.Rules[r.ID]
		t.Layouts[r.ID] = core.SlotNames(cr.Scope)
		if live != nil && !live(r.ID) {
			continue
		}
		if _, ok := r.Body.(*ast.OperatorsRule); ok {
			continue
		}
		var frame *FuncSpec
		if NodeFrameOK(r, cr) {
			frame = &FuncSpec{Key: Key{RuleBody, r.ID, 0}, Form: Frame, Rule: cr, Name: r.Name, Body: cr.Body, Slots: cr.Scope, Build: !r.Lean}
		}
		if RecEligible(r) && flowlower.LocalRule(cr) {
			add(&FuncSpec{Key: Key{RuleBody, r.ID, 0}, Form: Flat, Rule: cr, Name: r.Name, Fallback: frame})
		} else if frame != nil {
			add(frame)
		}
	}
	for _, r := range rules {
		e, ok := r.Body.(*ast.OperatorsRule)
		if !ok || live != nil && !live(r.ID) {
			continue
		}
		t.operators(m.Rules[r.ID], r, e, add)
	}
	return t
}

// operators plans the skip and the lines of the Operators Rule e of the rule r, whose Core rule is
// cr. A line that Flow IR cannot lower inside the rule's frame is left out.
func (t *Table) operators(cr *core.Rule, r *Rule, e *ast.OperatorsRule, add func(*FuncSpec)) {
	co, _ := cr.Body.(*core.OperatorsRule)
	if co == nil {
		return
	}
	line := func(k Key, l *core.OpLine, x ast.Expr, build bool) {
		if l != nil && flowlower.LineOK(x) && flowlower.FramedBody(l.Body) {
			add(&FuncSpec{Key: k, Form: Frame, Rule: cr, Name: r.Name, Body: l.Body, Slots: l.Scope, Build: build})
		}
	}
	if e.Skip != nil {
		line(Key{OperatorsSkip, r.ID, 0}, co.Skip, e.Skip, false)
	}
	// Lines whose value is unused are matched without values; so are all lines of a rule without a
	// value.
	i := 0
	for _, o := range e.Operands {
		line(Key{OperatorsLine, r.ID, i}, co.Operands[i], o.Expr, !r.Novalue && !check.LeanLine(o.Action, nil))
		i++
	}
	id := 0
	for _, lv := range e.Levels {
		for _, op := range lv.Operators {
			line(Key{OperatorsLine, r.ID, i}, co.Ops[id].Line, op.Expr, !r.Novalue && !check.LeanLine(op.Action, []string{"lhs", "rhs", "op"}))
			i++
			id++
		}
	}
}

// Refused reports whether the table runs the framed fallback instead of the flat function l, which
// the lowering of a Flat FuncSpec made and the Exec passes ran on: a reachable statement of l reads
// its entry position or its context, which a flat rule has no parameters for, or a variable of l
// holds a value, which a value-free flat rule does not keep. Reachability is that of the printers
// (codepass.DeadStores).
func Refused(l *execlower.Lowered) bool {
	a := flow.AttrsOf(l.Func)
	reads := codepass.DeadStores(l.Func, nil, func(code.Expr) bool { return false }).Named
	if a.Start != "" && reads[a.Start] || a.Ctx != "" && reads[a.Ctx] {
		return true
	}
	for _, v := range flow.VarsOf(l.Func) {
		switch flow.TypeOf(v.Type) {
		case flow.TypeInt, flow.TypeBool, flow.TypeRune, flow.TypeMark, flow.TypeExpMark, flow.TypeExpIDs:
		default:
			return true
		}
	}
	return false
}
