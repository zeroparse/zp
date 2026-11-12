package ref

import (
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
)

// The reference model of the Operators Rule descriptors, the matches of operators and the records of
// the split forms of the loop (the opr operations and the oprLongest, oprNud and oprExpr records).
// The rules are those of Config.Oprs; the model keeps the handles of their lists, operators and
// lines, one for each, so that two calls that return the same one return the same handle.

// OprOp is an operator of a rule of Config.Oprs.
type OprOp struct {
	// Kind is "prefix", "infix" or "postfix"; Assoc is "left", "right" or "none".
	Kind, Assoc string
	Level       int64
	// Scope is the number of capture slots of the operator's line.
	Scope int64
}

// OprRule is an Operators Rule of Config.Oprs, the rule of the index in RuleRef.
type OprRule struct {
	Prefix, Led []OprOp
	// Operands are the capture slots of each operand line.
	Operands []int64
	// Skip is whether the rule skips trivia between its operands and operators.
	Skip bool
}

// OprList is the model's Operators handle: a list of operators of a rule. Every list of no operators
// is one handle.
type OprList struct{ ops []*OprOperator }

// OprOperator is the model's Operator handle.
type OprOperator struct {
	op   OprOp
	line *OprLine
}

// OprLine is the model's Line handle.
type OprLine struct{ scope int64 }

// OprAttempt is the model's Attempt handle: a match of an operator. The null attempt is nil.
type OprAttempt struct {
	op         *OprOperator
	frame, v   any
	start, end int64
	env        *Scope
	cut        bool
	errs       any
}

// Opaque is the model's handle that a conformance case makes of a frame, a backtracking point, a value
// or a record of the split forms of the loop; it is only compared by identity.
type Opaque struct{ kind string }

// OprRecord is the model's handle of a record of a split form of the loop (oprLongest, oprNud and
// oprExpr). It holds the fields set so far; the others have the zero value of their type.
type OprRecord struct {
	prefix string
	fields map[string]any
}

type oprRule struct{ prefix, led *OprList }

// OprRecords are the records of the split forms of the loop: the prefix of the operations of each, and
// the names of its fields.
var OprRecords = map[string][]string{
	"oprLongest": {"l", "i", "m0", "best", "cutFailed", "prevFrame", "prevCut", "f"},
	"oprNud":     {"m0", "a", "i", "prevFrame", "prevCut", "f"},
	"oprExpr":    {"min", "lhs", "lastNone", "bound", "m0", "a"},
}

// OprHandle returns a new handle of the model for the conformance operation name (conform.OprFrameOp
// and the others that make an opaque handle or a record).
func (m *Model) OprHandle(name string) any {
	if rec, ok := strings.CutPrefix(name, "test.oprRecord."); ok {
		return &OprRecord{prefix: rec, fields: map[string]any{}}
	}
	return &Opaque{kind: name}
}

func (m *Model) oprRule(i int64) *oprRule {
	for int64(len(m.oprs)) <= i {
		m.oprs = append(m.oprs, nil)
	}
	if m.oprs[i] == nil {
		r := m.cfg.Oprs[i]
		list := func(ops []OprOp) *OprList {
			if len(ops) == 0 {
				return &m.noOprs
			}
			l := &OprList{}
			for _, o := range ops {
				l.ops = append(l.ops, &OprOperator{op: o, line: &OprLine{scope: o.Scope}})
			}
			return l
		}
		m.oprs[i] = &oprRule{prefix: list(r.Prefix), led: list(r.Led)}
	}
	return m.oprs[i]
}

// oprOperands returns the lines of the operands of the rule i.
func (m *Model) oprOperands(i int64) []*OprLine {
	for int64(len(m.operands)) <= i {
		m.operands = append(m.operands, nil)
	}
	if m.operands[i] == nil {
		m.operands[i] = []*OprLine{}
		for _, s := range m.cfg.Oprs[i].Operands {
			m.operands[i] = append(m.operands[i], &OprLine{scope: s})
		}
	}
	return m.operands[i]
}

// IsOprOp reports whether the model has the operation o of the Operators Rule descriptors, the
// matches and the records.
func IsOprOp(o *exec.Op) bool { return isOpr(o.Name) }

func isOpr(name string) bool {
	if strings.HasPrefix(name, "opr.") {
		return true
	}
	rec, _, ok := strings.Cut(name, ".")
	return ok && OprRecords[rec] != nil
}

// recordField splits an operation of a record into the record's prefix, the field and whether it
// sets it.
func recordField(name string) (rec, field string, set bool) {
	rec, f, _ := strings.Cut(name, ".")
	if x, ok := strings.CutPrefix(f, "set"); ok && len(x) > 0 && x[0] >= 'A' && x[0] <= 'Z' {
		return rec, strings.ToLower(x[:1]) + x[1:], true
	}
	return rec, f, false
}

// allowsOpr reports whether a call of an operation of the Operators Rule family meets its precondition.
func (m *Model) allowsOpr(o *exec.Op, args []any) bool {
	switch o.Name {
	case "opr.led", "opr.prefix", "opr.operands", "opr.hasSkip":
		return m.oprRuleOK(args[0])
	case "opr.operand":
		return m.oprRuleOK(args[0]) && args[1].(int64) >= 0 && args[1].(int64) < int64(len(m.cfg.Oprs[args[0].(RuleRef).Index].Operands))
	case "opr.at":
		l, i := args[0].(*OprList), args[1].(int64)
		return i >= 0 && i < int64(len(l.ops))
	case "opr.op", "opr.end", "opr.cut", "opr.env", "opr.errs":
		return args[0] != nil
	case "opr.attempt":
		return args[0] != nil
	}
	return true
}

// oprZero returns the value of the field of a record before it is set: the zero value of its type, 0,
// false, the list of no operators, the zero backtracking point, or the null handle.
func (m *Model) oprZero(field string) any {
	switch field {
	case "i", "min", "lastNone", "bound":
		return int64(0)
	case "cutFailed", "prevCut":
		return false
	case "l":
		return &m.noOprs
	case "m0":
		if m.zeroMark == nil {
			m.zeroMark = &Opaque{kind: "zero mark"}
		}
		return m.zeroMark
	}
	return nil
}

func (m *Model) oprRuleOK(r any) bool {
	i := r.(RuleRef).Index
	return i >= 0 && i < int64(len(m.cfg.Oprs))
}

// callOpr runs an operation of the Operators Rule family, and reports false for any other.
func (m *Model) callOpr(o *exec.Op, args []any) ([]any, bool) {
	if rec, field, set := recordField(o.Name); OprRecords[rec] != nil {
		r := args[0].(*OprRecord)
		if set {
			r.fields[field] = args[1]
			return nil, true
		}
		if v, ok := r.fields[field]; ok {
			return []any{v}, true
		}
		return []any{m.oprZero(field)}, true
	}
	switch o.Name {
	case "opr.led":
		return []any{m.oprRule(args[0].(RuleRef).Index).led}, true
	case "opr.prefix":
		return []any{m.oprRule(args[0].(RuleRef).Index).prefix}, true
	case "opr.count":
		return []any{int64(len(args[0].(*OprList).ops))}, true
	case "opr.at":
		return []any{args[0].(*OprList).ops[args[1].(int64)]}, true
	case "opr.operands":
		return []any{int64(len(m.cfg.Oprs[args[0].(RuleRef).Index].Operands))}, true
	case "opr.operand":
		return []any{m.oprOperands(args[0].(RuleRef).Index)[args[1].(int64)]}, true
	case "opr.line":
		return []any{args[0].(*OprOperator).line}, true
	case "opr.level":
		return []any{args[0].(*OprOperator).op.Level}, true
	case "opr.infix":
		return []any{args[0].(*OprOperator).op.Kind == "infix"}, true
	case "opr.postfix":
		return []any{args[0].(*OprOperator).op.Kind == "postfix"}, true
	case "opr.right":
		return []any{args[0].(*OprOperator).op.Assoc == "right"}, true
	case "opr.nonAssoc":
		return []any{args[0].(*OprOperator).op.Assoc == "none"}, true
	case "opr.scope":
		return []any{args[0].(*OprLine).scope}, true
	case "opr.hasSkip":
		return []any{m.cfg.Oprs[args[0].(RuleRef).Index].Skip}, true
	case "opr.attempt":
		return []any{&OprAttempt{op: args[0].(*OprOperator), frame: args[1], v: args[2], start: args[3].(int64),
			end: m.regs["pos"].(int64), env: m.env, cut: args[4].(bool), errs: args[5]}}, true
	case "opr.found":
		return []any{args[0] != nil}, true
	case "opr.op":
		return []any{args[0].(*OprAttempt).op}, true
	case "opr.end":
		return []any{args[0].(*OprAttempt).end}, true
	case "opr.cut":
		return []any{args[0].(*OprAttempt).cut}, true
	case "opr.env":
		return []any{args[0].(*OprAttempt).env}, true
	case "opr.errs":
		return []any{args[0].(*OprAttempt).errs}, true
	case "opr.setCutFailed":
		m.cutFailed = args[0].(bool)
		return nil, true
	case "opr.cutFailed":
		return []any{m.cutFailed}, true
	case "opr.setNudBound":
		m.nudBound = args[0].(int64)
		return nil, true
	case "opr.nudBound":
		return []any{m.nudBound}, true
	}
	return nil, false
}
