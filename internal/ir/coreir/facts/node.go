package facts

import "zeroparse.dev/zp/internal/ir/coreir"

// The queries below answer for any node of the module the facts were computed on, with the rule
// facts of Facts. A node's nullability, first terminal and effects are its attributes
// (coreir.Node.Attrs); these are the facts that depend on the rules called.

// Succeeds reports a proof that n cannot fail, at any position. Errors that end the parse are
// not failures.
func (f *Facts) Succeeds(n coreir.Node) bool { return f.c.alwaysSucceeds(n) }

// Fails reports a proof that n cannot succeed.
func (f *Facts) Fails(n coreir.Node) bool { return f.c.alwaysFails(n) }

// LeftCalls returns the rules n may call before it consumes input, in the order of the body, a
// rule called at several such places listed at each; with token insertion enabled if ins.
func (f *Facts) LeftCalls(n coreir.Node, ins bool) []int { return f.c.leftCalls(n, nil, ins) }
