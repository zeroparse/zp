package facts

import (
	"fmt"

	"zeroparse.dev/zp/internal/ir/core"
)

// The queries below answer for any node of the module the facts were computed on, with the rule
// facts of Facts. A node's nullability, first terminal and effects are its attributes
// (core.Node.Attrs); these are the facts that depend on the rules called.

// Succeeds reports a proof that n cannot fail, at any position. Errors that end the parse are
// not failures.
func (f *Facts) Succeeds(n core.Node) bool { return f.c.alwaysSucceeds(n) }

// Fails reports a proof that n cannot succeed.
func (f *Facts) Fails(n core.Node) bool { return f.c.alwaysFails(n) }

// LeftCalls returns the rules n may call before it consumes input, in the order of the body, a
// rule called at several such places listed at each; with token insertion enabled if ins.
func (f *Facts) LeftCalls(n core.Node, ins bool) []int { return f.c.leftCalls(n, nil, ins) }

// LooseCut reports a cut in n that can commit the choice, option or repetition around n: one
// outside nested choices, options, repetitions, lookaheads and calls. A recovery's skip runs in
// the scope of its body, so a cut in it counts.
func LooseCut(n core.Node) bool { return looseCut(n) }

// Nullable reports that n can succeed without consuming input when no token is inserted, by the
// rules of core.Analyze, with the nullability of the rules it calls. It answers for a node that
// Analyze did not see, such as one a rewrite builds, whose attributes are zero.
func (f *Facts) Nullable(n core.Node) bool {
	switch n := n.(type) {
	case nil:
		return true
	case *core.Lit:
		return n.Text == ""
	case *core.Scan:
		return n.Min == 0
	case *core.ClassNode, *core.Any, *core.FailNode, *core.ClassRun:
		return false
	case *core.Anchor, *core.Opt, *core.Look, *core.Cut, *core.Succeed, *core.Pred, *core.BindNode:
		return true
	case *core.Seq:
		for _, it := range n.Items {
			if !f.Nullable(it) {
				return false
			}
		}
		return true
	case *core.Choice:
		for _, alt := range n.Alts {
			if f.Nullable(alt.Body) {
				return true
			}
		}
		return false
	case *core.Repeat:
		return n.Min == 0 || n.Min == 1 && f.Nullable(n.Body)
	case *core.Call:
		return f.Rules[n.Rule].Nullable
	case *core.OperatorsRule:
		for _, l := range n.Operands {
			if f.Nullable(l.Body) {
				return true
			}
		}
		return false
	case *core.Missing:
		return f.Nullable(n.Lit)
	case *core.Stream:
		return f.Nullable(n.Repeat)
	case *core.Nest:
		return f.Nullable(n.Body)
	case *core.Text:
		return f.Nullable(n.Body)
	case *core.Drop:
		return f.Nullable(n.Body)
	case *core.Capture:
		return f.Nullable(n.Body)
	case *core.Label:
		return f.Nullable(n.Body)
	case *core.Recover:
		return f.Nullable(n.Body)
	case *core.Terminal:
		return f.Nullable(n.Body)
	}
	panic(fmt.Sprintf("facts: nullability of %T", n))
}
