package ref

import (
	"fmt"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// The reference model of the rules of a grammar (component rules): what the grammar says of each rule,
// and its body as a script. A rule's body is the code of the grammar, which the parser VM does not
// model; a conformance case states instead what each evaluation of the body does (Eval), and both the
// model and an implementation run that script: the model here, an implementation through a rule whose
// body is a function that runs it with the implementation's own operations. Each evaluation of a body
// is logged, so that the suite also compares which bodies ran, where and at which level.

// RuleSpec is what the grammar says of a rule of Config.RuleVars, and the script of its body.
type RuleSpec struct {
	// Memo: a parse from scratch memoizes the rule. DocMemo: a Document parse memoizes it, as it does
	// every rule a parse from scratch memoizes. Leader: it leads a left-recursive cycle. DocPlain: a
	// parse from scratch calls its plain body at the call site. Positional: its results may depend on
	// positions. Rec: it is on a cycle of calls, so that its evaluation takes one nesting level.
	Memo, DocMemo, Leader, DocPlain, Positional, Rec bool
	// Captures is the number of the rule's capture names; a rule without any is plain.
	Captures int64
	// Body is the script of the body: its k-th evaluation in the parse does Body[k], or the last of
	// Body once k passes it. A rule without a script fails at once.
	Body []Eval
}

// Eval is what one evaluation of a body does, in this order: it charges Charge units, records the
// recovered errors with the identifiers Errs (as rec.push does), extends the examined input of a
// Document parse to Touch units past its start if Touch is positive (as doc.touch does), moves the
// position Len units past its start and succeeds if OK. A body that succeeds makes a value no other
// evaluation made.
type Eval struct {
	OK     bool
	Len    int64
	Charge uint64
	Errs   []int64
	Touch  int64
}

// BodyRef is the model's handle of the invocation of a rule's body that a generated call passes
// (ops.Body): that of the rule of Config.RuleVars with the index.
type BodyRef struct{ Index int64 }

// RuleValue is a value that a rule's body made: the evaluation of the rule that made it.
type RuleValue struct{ Rule, Eval int64 }

// ruleState is the state of the component rules.
type ruleState struct {
	// evals is the number of evaluations of each rule's body so far.
	evals map[int64]int64
	log   []string
}

// spec returns the RuleSpec of the rule with the index.
func (m *Model) spec(index int64) RuleSpec {
	if int(index) < len(m.cfg.RuleSpecs) {
		return m.cfg.RuleSpecs[index]
	}
	return RuleSpec{}
}

// next returns the evaluation that the body of the rule with the index does next, without counting it.
func (m *Model) next(index int64) Eval {
	b := m.spec(index).Body
	if len(b) == 0 {
		return Eval{}
	}
	return b[min(m.rule.evals[index], int64(len(b)-1))]
}

// allowsRule reports whether a call of an operation of the rules meets its precondition.
func (m *Model) allowsRule(o *exec.Op, args []any) (allowed, ok bool) {
	switch o.Name {
	case "rule.invokePlain":
		return m.spec(args[0].(RuleRef).Index).Captures == 0, true
	}
	return false, false
}

// callRule runs an operation of the rules, and reports false for any other.
func (m *Model) callRule(o *exec.Op, args []any) ([]any, bool) {
	flag := func(f func(s RuleSpec) bool) ([]any, bool) {
		return []any{f(m.spec(args[0].(RuleRef).Index))}, true
	}
	// A parse in the missing-token mode memoizes none of the rules that insert missing tokens.
	missing := func() bool {
		i := args[0].(RuleRef).Index
		return m.cfg.Missing && int(i) < len(m.cfg.MissingRules) && m.cfg.MissingRules[i]
	}
	switch o.Name {
	case "rule.memo":
		return flag(func(s RuleSpec) bool {
			return (s.Memo || m.cfg.DocCallMemo && m.cfg.Document && s.DocMemo) && !missing()
		})
	case "rule.leader":
		return flag(func(s RuleSpec) bool { return s.Leader })
	case "rule.plain":
		return flag(func(s RuleSpec) bool { return s.Captures == 0 })
	case "rule.docMemo":
		return flag(func(s RuleSpec) bool { return s.DocMemo })
	case "rule.docOnlyMemo":
		return flag(func(s RuleSpec) bool {
			only := s.DocMemo && !s.Memo && !s.Leader && !missing()
			if m.cfg.DocCallMemo {
				only = only && m.cfg.Document
			}
			return only
		})
	case "rule.docPlain":
		return flag(func(s RuleSpec) bool { return s.DocPlain })
	case "rule.positional":
		return flag(func(s RuleSpec) bool { return s.Positional })
	case "rule.id":
		return []any{args[0].(RuleRef).Index}, true
	case "rule.invoke":
		v, ok := m.evaluate(args[0].(RuleRef).Index, args[1].(int64), true)
		return []any{v, ok}, true
	case "rule.invokePlain":
		mark, start := len(m.rec), m.regs["pos"].(int64)
		v, ok := m.evaluate(args[0].(RuleRef).Index, args[1].(int64), true)
		if !ok {
			m.regs["pos"] = start
			m.rec = m.rec[:mark]
		}
		return []any{v, ok}, true
	case "rule.run":
		v, ok := m.evaluate(args[0].(BodyRef).Index, 0, false)
		return []any{v, ok}, true
	}
	return nil, false
}

// evaluate runs the next evaluation of the body of the rule with the index at the level and returns
// its value and success. It counts the evaluation in the statistics of a Document parse if count.
func (m *Model) evaluate(index, level int64, count bool) (any, bool) {
	if m.rule.evals == nil {
		m.rule.evals = map[int64]int64{}
	}
	e, k := m.next(index), m.rule.evals[index]
	m.rule.evals[index]++
	start := m.regs["pos"].(int64)
	m.rule.log = append(m.rule.log, fmt.Sprintf("body %s level=%d pos=%d", RuleName(index), level, start))
	if count && m.cfg.Document {
		m.doc.evaluated++
	}
	rec := m.spec(index).Rec
	if rec {
		m.depth++
		if m.depth > m.cfg.MaxDepth {
			panic(exec.Interrupt{Reason: "nesting"})
		}
	}
	// The suite adds what the implementation charged to the model's units after the call.
	if m.Exceeds(e.Charge) {
		panic(exec.Interrupt{Reason: "limit"})
	}
	for _, id := range e.Errs {
		m.rec = append(m.rec[:len(m.rec):len(m.rec)], &ErrList{err: id, n: 1})
	}
	if e.Touch > 0 && m.cfg.Document {
		m.doc.high = max(m.doc.high, start+e.Touch)
	}
	m.regs["pos"] = start + e.Len
	if rec {
		m.depth--
	}
	if !e.OK {
		return nil, false
	}
	return &RuleValue{Rule: index, Eval: k}, true
}

// bodyQuantity returns ops.QBody for a call of o with args: the units that the evaluation it runs
// charges.
func (m *Model) bodyQuantity(o *exec.Op, args []any) int64 {
	switch r := args[0].(type) {
	case RuleRef:
		return int64(m.next(r.Index).Charge)
	case BodyRef:
		return int64(m.next(r.Index).Charge)
	}
	panic("ref: no body quantity for " + o.Name)
}

// Unspecified returns the registers that the call of o with args, which returned res, leaves
// unspecified, so that an implementation may leave any value in them: the position after a failed
// rule.invoke or rule.run.
func (m *Model) Unspecified(o *exec.Op, res []any) []string {
	switch o.Name {
	case "rule.invoke", "rule.run":
		if !res[1].(bool) {
			return []string{ops.Pos}
		}
	}
	return nil
}

// RuleName returns the name that an implementation gives the rule of Config.RuleVars with the index.
func RuleName(index int64) string { return fmt.Sprint("r", index) }

// Observe returns what the model holds of a component for the conformance suite to compare with an
// implementation's: the evaluations of rule bodies, for component ops.Rules, and the events that the
// observation delivered, for component ops.Obs, and the tokens inserted in the missing-token mode, for
// ops.Rec. It returns nil for any other component or parse.
func (m *Model) Observe(component string) []string {
	switch component {
	case ops.Rules:
		return append([]string{}, m.rule.log...)
	case ops.Obs:
		return append([]string{}, m.obs.events...)
	case ops.Rec:
		return m.observeMissing()
	}
	return nil
}
