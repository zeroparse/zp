package compile

import "zeroparse.dev/zp/grammar"

// DepthBoundResearch is a research hook: when set, the classifier assumes a bound on the nesting
// of recursive rule calls, so that the evaluations that nest and end at one position are bounded.
// A call reached after leaving a body through a nesting call is then shared like any other
// (rescans), and the operator parts that the enclosing levels of a nesting Operators Rule try
// again are no continuation (addRetried). It is not safe for concurrent classification.
var DepthBoundResearch bool

// ResearchVarRange is a research hook: declared inclusive ranges of int variables. A binding to a
// value outside its range binds nothing, as a failed binding does, so finiteInt drops the value.
var ResearchVarRange map[string][2]int

// ResearchFiniteVars returns, for research, the variables that finiteVars finds finite in g from
// start, and the number of values of each int and bool variable before the product cap.
func ResearchFiniteVars(g *grammar.Grammar, start string) (map[string]bool, map[string]int, error) {
	k, _, err := prepareClassifier(g, start, nil)
	if err != nil {
		return nil, nil, err
	}
	k.collect()
	byVar := map[string][]*binding{}
	for _, b := range k.binds {
		byVar[b.b.Name] = append(byVar[b.b.Name], b)
	}
	sizes := map[string]int{}
	for _, v := range k.g.Vars() {
		switch k.varType[v.Name] {
		case "bool":
			sizes[v.Name] = finiteBool(v, byVar[v.Name])
		case "int":
			if n, ok := k.finiteInt(v, byVar[v.Name]); ok {
				sizes[v.Name] = n
			} else {
				sizes[v.Name] = -1
			}
		default:
			sizes[v.Name] = -2
		}
	}
	return k.finiteVars(), sizes, nil
}

// ResearchConstStrings is a research hook: a string variable whose initial value and bindings are
// all constants has finite values.
var ResearchConstStrings bool

// SetResearchMaxFiniteValues sets the cap on the product of the numbers of values of the variables
// with finite values, for research, and returns the previous cap.
func SetResearchMaxFiniteValues(n int) int {
	old := maxFiniteValues
	maxFiniteValues = n
	return old
}

// finiteConstString returns the number of distinct constant values of the string variable v, and
// whether every binding of v binds a constant.
func finiteConstString(v *grammar.VarDef, binds []*binding) (int, bool) {
	set := map[string]bool{}
	if v.Value == nil {
		set[""] = true
	} else if lit, isLit := v.Value.(*grammar.StringLit); isLit {
		set[lit.Value] = true
	} else {
		return 0, false
	}
	for _, b := range binds {
		lit, isLit := ungroupTerm(b.b.Value).(*grammar.StringLit)
		if !isLit {
			return 0, false
		}
		set[lit.Value] = true
	}
	return len(set), true
}

// ResearchVarsShareable is a research hook: a call of a rule that reads variables is shared as if
// the unit and its continuation reached it with equal environments. It is an upper bound on what
// a proof of equal environments can give, not a sound assumption.
var ResearchVarsShareable bool
