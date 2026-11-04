package compile

// DepthBoundResearch is a research hook: when set, the classifier assumes a bound on the nesting
// of recursive rule calls, so that the evaluations that nest and end at one position are bounded.
// A call reached after leaving a body through a nesting call is then shared like any other
// (rescans), and the operator parts that the enclosing levels of a nesting Operators Rule try
// again are no continuation (addRetried). It is not safe for concurrent classification.
var DepthBoundResearch bool
