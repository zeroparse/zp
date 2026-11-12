package ref

import (
	"fmt"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// The reference model of the observation of a parse (component observe): the calls that obs.enter
// began and obs.exit has not ended, innermost last, and the events that the observation delivered. An
// event is what a trace of the parse sees: the begin of a call, and its end with its outcome and
// whether the call reused a memo entry.

// RuleName returns the name that an implementation gives the rule of Config.RuleVars with the index.
func RuleName(index int64) string { return fmt.Sprint("r", index) }

// obsCall is a call that obs.enter began.
type obsCall struct {
	rule  string
	depth int
	// memo reports obs.memoHit since the begin of the call.
	memo bool
}

// obsState is the state of the component observe.
type obsState struct {
	open   []obsCall
	events []string
}

// allowsObs reports whether a call of an operation of the observation meets its precondition.
func (m *Model) allowsObs(o *exec.Op) (allowed, ok bool) {
	switch o.Name {
	case "obs.enter":
		return m.cfg.Observed, true
	case "obs.exit":
		return m.cfg.Observed && len(m.obs.open) > 0, true
	}
	return false, false
}

// callObs runs an operation of the observation, and reports false for any other.
func (m *Model) callObs(o *exec.Op, args []any) ([]any, bool) {
	s := &m.obs
	switch o.Name {
	case "obs.active":
		return []any{m.cfg.Observed}, true
	case "obs.memoHit":
		if m.cfg.Observed && len(s.open) > 0 {
			s.open[len(s.open)-1].memo = true
		}
		return nil, true
	case "obs.enter":
		c := obsCall{rule: RuleName(args[0].(RuleRef).Index), depth: len(s.open) + 1}
		s.open = append(s.open, c)
		s.events = append(s.events, fmt.Sprintf("enter %s pos=%d level=%d depth=%d", c.rule, args[1], args[2], c.depth))
		return nil, true
	case "obs.exit":
		c := s.open[len(s.open)-1]
		s.open = s.open[:len(s.open)-1]
		examined, known := args[2].(int64), args[3].(bool)
		if !known {
			examined = 0
		}
		s.events = append(s.events, fmt.Sprintf("exit %s matched=%v end=%d examined=%d known=%v memo=%v depth=%d",
			c.rule, args[0], args[1], examined, known, c.memo, c.depth))
		return nil, true
	}
	return nil, false
}

// Observe returns what the model holds of a component for the conformance suite to compare with an
// implementation's: the events that the observation delivered, for component ops.Obs, and the tokens
// inserted in the missing-token mode, for ops.Rec. It returns nil for any other component or parse.
func (m *Model) Observe(component string) []string {
	switch component {
	case ops.Obs:
		return append([]string{}, m.obs.events...)
	case ops.Rec:
		return m.observeMissing()
	}
	return nil
}
