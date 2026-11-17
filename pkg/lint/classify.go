package lint

import (
	"context"

	"zeroparse.dev/zp/internal/analysis/complexity"
	impl "zeroparse.dev/zp/internal/analysis/lint"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Class is the class of the cost of a parse from a start rule (see "Classification" in the parsing
// specification). The classes are ordered from the strongest claim to the weakest: Linear and
// LinearEnv are guarantees, Polynomial estimates a bound, Unknown is no bound, and Exponential is
// an estimate of growth that no polynomial bounds.
type Class int

const (
	// Linear promises a cost of at most c · (n + 1).
	Linear Class = iota
	// LinearEnv promises a cost of at most c · K · (L + 1) · (n + 1).
	LinearEnv
	// Polynomial estimates a cost that grows as n^Degree: every condition for linear cost that
	// fails has a witnessed cause, and Degree bounds them all.
	Polynomial
	// Unknown makes no claim: a condition for linear cost fails that no cause accounts for.
	Unknown
	// Exponential estimates a cost that can double when n grows by a constant.
	Exponential
)

// String returns the name of the class, such as "linear" or "polynomial".
func (c Class) String() string { return complexity.ComplexityClass(c).String() }

// CauseKind is the pattern of grammar that a cause comes from. Each kind but CauseOther is a
// pattern that the recipe of linear rewrites in the cookbook shows a rewrite for.
type CauseKind uint8

const (
	// CauseOther is a reason for the class Unknown, or a cause of a pattern that no rewrite covers.
	CauseOther CauseKind = iota
	// CauseRestartedRepetition is a repetition that a failed alternative restarts inside the input
	// it examined.
	CauseRestartedRepetition
	// CauseLookaheadScan is a lookahead whose scan restarts at every iteration of a repetition.
	CauseLookaheadScan
	// CauseRecoverySkip is a #recover skip expression walked again after the recovery fails.
	CauseRecoverySkip
	// CauseRegrowth is a left-recursive rule or an Operators Rule that grows again over the same
	// input, or a construct evaluated in each step of such a growth.
	CauseRegrowth
	// CauseDivergingBinding is a binding of a variable that reads its earlier value, or that makes
	// the number of values of the variables unbounded.
	CauseDivergingBinding
	// CauseSelfGrowingValue is a value built from its own earlier value, or copied more often than
	// the input bounds.
	CauseSelfGrowingValue
)

// String returns the name of the kind, such as "restarted repetition".
func (k CauseKind) String() string { return complexity.ComplexityCauseKind(k).String() }

// Cause is one polynomial or exponential cause, at the scanning source, binding or operation that
// causes it, or one reason for the class Unknown.
type Cause struct {
	// Kind is the pattern the cause comes from; CauseOther for a reason.
	Kind  CauseKind
	Class Class
	// Degree is k of a polynomial cause.
	Degree int
	// Rule is the rule that holds Pos.
	Rule string
	Pos  ast.Pos
	// Message names the class and the witness (or, for the class Unknown, the reason).
	Message string
}

// Classification is the class of the cost of a grammar from one start rule, with what decides it.
type Classification struct {
	Class Class
	// Degree is the largest degree of the causes of the class Polynomial, and zero for any other
	// class.
	Degree int
	// WitnessedDegree is the largest degree of the polynomial causes of the class Unknown, and zero
	// for any other class. It is a lower estimate from the witnesses, not a bound.
	WitnessedDegree int
	// Causes are the polynomial and exponential causes, sorted by position. A grammar of the class
	// Unknown can have them too.
	Causes []Cause
	// Reasons are the reasons for the class Unknown, sorted by position. Only an exponential
	// grammar has them besides.
	Reasons []Cause
	// Env says why the class is at least linear given an environment bound, or is empty.
	Env []Cause
	// Combinations is the product of the numbers of values of the variables with finite values,
	// 1 when there are none. For the class Linear it bounds the number of combinations of the
	// values of the variables that memoized rules read at one position.
	Combinations int
}

// Classify classifies the cost of parsing with the grammar g from its start rule start, the
// classification that Config.Classify asks a lint run for. An empty start selects the default
// start rule. Classify returns an error if g does not compile, if start is not a start rule of g,
// or if ctx ends.
func Classify(ctx context.Context, g *ast.Grammar, start string) (*Classification, error) {
	c, err := impl.Classify(ctx, g, start)
	if err != nil {
		return nil, err
	}
	return &Classification{
		Class:           Class(c.Class),
		Degree:          c.Degree,
		WitnessedDegree: c.WitnessedDegree,
		Causes:          causesFromImpl(c.Causes),
		Reasons:         causesFromImpl(c.Reasons),
		Env:             causesFromImpl(c.Env),
		Combinations:    c.Combinations,
	}, nil
}

func causesFromImpl(in []complexity.ComplexityCause) []Cause {
	if in == nil {
		return nil
	}
	out := make([]Cause, len(in))
	for i, c := range in {
		out[i] = Cause{Kind: CauseKind(c.Kind), Class: Class(c.Class), Degree: c.Degree, Rule: c.Rule, Pos: c.Pos, Message: c.Message}
	}
	return out
}
