package codegentest

import (
	"slices"

	"zeroparse.dev/zp/core"
)

// SelectedEvents returns the events of a whole trace that a trace function receives with the trace
// filter rules and the depth limit maxDepth, as the specification of the trace options defines them:
// the calls of the named rules and the calls nested in them (every call if rules is empty), at most
// maxDepth deep (any depth if maxDepth is not positive). A skipped call is selected like a call made
// at its place. It is the reference that the producers' own selection is checked against.
func SelectedEvents(events []core.TraceEvent, rules []string, maxDepth int) []core.TraceEvent {
	var out []core.TraceEvent
	var open []bool // whether each call in progress is selected
	inner := func(rule string) bool {
		return len(rules) == 0 || slices.Contains(rules, rule) || len(open) > 0 && open[len(open)-1]
	}
	for _, e := range events {
		var selected bool
		switch e.Kind {
		case core.TraceEnter:
			selected = inner(e.Rule)
			open = append(open, selected)
		case core.TraceExit:
			selected, open = open[len(open)-1], open[:len(open)-1]
		default:
			selected = inner(e.Rule)
		}
		if selected && (maxDepth <= 0 || e.Depth <= maxDepth) {
			out = append(out, e)
		}
	}
	return out
}
