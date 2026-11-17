package sample

// MatchWork returns the steps that the matcher took evaluating checks in the generator's last
// attempt.
func MatchWork(g *Generator) int { return g.g.matchWork }

// MatchBudget returns the most matcher steps an attempt may take, not counting the overshoot of the
// match that runs it out.
func MatchBudget(g *Generator) int { return matchBudget * g.cfg.budget }

// MatchSteps is the most steps one match may take: the overshoot of the match that runs an attempt out
// of its matcher budget.
const MatchSteps = matchSteps
