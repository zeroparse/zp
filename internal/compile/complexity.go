package compile

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/desugar"
)

// ComplexityClass is the class of the cost of a parse from a start rule (see "Classification" in
// spec/runtime/parsing.md). The classes are ordered from the strongest claim to the weakest:
// Linear and LinearEnv are guarantees, Poly estimates a bound, Unknown is no bound, and Exp is an
// estimate of growth that no polynomial bounds.
type ComplexityClass int

const (
	// ClassLinear promises a cost of at most c · (n + 1).
	ClassLinear ComplexityClass = iota
	// ClassLinearEnv promises a cost of at most c · K · (L + 1) · (n + 1).
	ClassLinearEnv
	// ClassPoly estimates a cost that grows as n^Degree: every condition for linear cost that
	// fails has a witnessed cause, and Degree bounds them all.
	ClassPoly
	// ClassUnknown makes no claim: a condition for linear cost fails that no cause accounts for.
	ClassUnknown
	// ClassExp estimates a cost that can double when n grows by a constant.
	ClassExp
)

func (c ComplexityClass) String() string {
	switch c {
	case ClassLinear:
		return "linear"
	case ClassLinearEnv:
		return "linear given an environment bound"
	case ClassUnknown:
		return "unknown"
	case ClassPoly:
		return "polynomial"
	case ClassExp:
		return "exponential"
	}
	return fmt.Sprintf("ComplexityClass(%d)", int(c))
}

// ComplexityCause is one polynomial or exponential cause, at the scanning source,
// binding or operation that causes it, or one reason for the class unknown.
type ComplexityCause struct {
	Class ComplexityClass
	// Degree is k of a polynomial cause.
	Degree int
	// Rule is the rule that holds Pos.
	Rule string
	Pos  grammar.Pos
	// Message names the class and the witness (or, for the class unknown, the reason).
	Message string
}

// GrowingBinding is a binding of a string variable that joins the variable's own value at least
// twice and whose value reaches a later evaluation of the same binding (the growing-variable lint
// check).
type GrowingBinding struct {
	Rule string
	Pos  grammar.Pos
	// Var is the variable, and Text the binding as written.
	Var, Text string
}

// Complexity is the classification of the cost of a grammar from one start rule.
type Complexity struct {
	Class ComplexityClass
	// Degree is the largest degree of the causes of a polynomial class, and zero for any other class.
	Degree int
	// WitnessedDegree is the largest degree of the polynomial causes of the class unknown, and zero
	// for any other class. It is a lower estimate from the witnesses, not a bound: the reasons
	// leave the cost open.
	WitnessedDegree int
	// Causes are the polynomial and exponential causes, sorted by position. A grammar of the class
	// unknown can have them too.
	Causes []ComplexityCause
	// Reasons are the reasons for the class unknown, sorted by position. Only an exponential
	// grammar has them besides; they are shown only when the classification is requested.
	Reasons []ComplexityCause
	// Env says why the class is at least linear given an environment bound, or is empty.
	Env []ComplexityCause
	// Combinations is the product of the numbers of values of the variables with finite values
	// (see "Conditions for Linear Cost" in spec/runtime/parsing.md), 1 when there are none. For
	// the class Linear it bounds K, the number of combinations of the values of the variables
	// that memoized rules read at one position.
	Combinations int
	// Growing are the bindings that the growing-variable check reports.
	Growing []GrowingBinding
	// Memoized are the rules that the classifier assumes the memo answers, which are the rules
	// that every parse memoizes (see TestComplexityMemoPolicy).
	Memoized map[string]bool
}

// ClassifyComplexity classifies the cost of g from the start rule start ("" selects the default
// start rule, and its absence is an error). g must compile. A start that parsing rejects, as a
// name that is not defined or not a start rule, is the error that parsing gives. It analyzes the grammar that the compiler runs: inline actions are
// lowered to hidden rules and attributes are normalized. A non-nil control polls for
// cancellation.
func ClassifyComplexity(g *grammar.Grammar, start string, control *cancel.Control) (*Complexity, error) {
	k, names, err := prepareClassifier(g, start, control)
	if err != nil {
		return nil, err
	}
	c := k.classify()
	if k.stringLen {
		// Only the types tell whether the operand of len is a string value; the classification
		// without them stands when no len counted its operand, or when g does not compile.
		if typed, typedNames := prepareTypedClassifier(g, start, control); typed != nil {
			c, names = typed.classify(), typedNames
		}
	}
	return c.written(names), nil
}

// GrowingBindings returns the Growing member of the classification of g from start, without the
// rest of the classification, which costs more. It has the preconditions of ClassifyComplexity.
func GrowingBindings(g *grammar.Grammar, start string, control *cancel.Control) ([]GrowingBinding, error) {
	k, names, err := prepareClassifier(g, start, control)
	if err != nil {
		return nil, err
	}
	k.collect()
	c := &Complexity{Growing: k.growingBindings()}
	return c.written(names).Growing, nil
}

// prepareTypedClassifier is prepareClassifier with the value types of the terms of g, which tell
// a node from a string value. It compiles g, without building matchers, for them, and returns nil
// if g does not compile.
func prepareTypedClassifier(g *grammar.Grammar, start string, control *cancel.Control) (*classifier, desugar.InlineNames) {
	control.Check()
	names := desugar.InlineNames{}
	prog, err := CompileProgram(g, Options{}, nil, names, control, func(*Compiler) ExprBuilder[struct{}] { return NoExprs{} })
	if err != nil {
		return nil, nil
	}
	if start == "" {
		start = prog.DefaultStartRule
	}
	k := newViewClassifier(prog.Grammar, control.Rules(prog.Grammar), start, control)
	k.types = prog.ValueRecord()
	return k, names
}

// prepareClassifier lowers and normalizes g as the compiler does and returns a classifier of it
// from start, with the names of the hidden rules of its inline actions.
func prepareClassifier(g *grammar.Grammar, start string, control *cancel.Control) (*classifier, desugar.InlineNames, error) {
	return prepareClassifierWith(g, start, control, newViewClassifier)
}

// prepareBaseClassifier is prepareClassifier over the rules as they are written, not their normal
// view. The exclusion analysis itself reads the normal view, as ClassifyComplexity does; the tests
// that check how it compares the alternatives of a choice read the rules as written, since the
// normal view left-factors alternatives that begin alike (N12) and leaves no shared prefix to
// compare.
func prepareBaseClassifier(g *grammar.Grammar, start string, control *cancel.Control) (*classifier, desugar.InlineNames, error) {
	return prepareClassifierWith(g, start, control, newClassifier)
}

func prepareClassifierWith(g *grammar.Grammar, start string, control *cancel.Control, build func(*grammar.Grammar, []*grammar.RuleDef, string, *cancel.Control) *classifier) (*classifier, desugar.InlineNames, error) {
	control.Check()
	names := desugar.InlineNames{}
	if desugar.HasInlineAction(g, control) {
		lowered, errs := desugar.LowerInlineActionsControlled(g, names, control)
		if errs != nil {
			return nil, nil, errs
		}
		g = lowered
	}
	g, err := desugar.NormalizeAttributesControlled(g, false, control)
	if err != nil {
		return nil, nil, err
	}
	rules, errs := desugar.UniqueRules(g, control)
	if len(errs) > 0 {
		return nil, nil, errs
	}
	starts, def, errs := desugar.ResolveStarts(rules, control)
	if len(errs) > 0 {
		return nil, nil, errs
	}
	defined := func(name string) bool {
		return slices.ContainsFunc(rules, func(rd *grammar.RuleDef) bool { return rd.Name == name })
	}
	if err := checkStartName(starts, def, defined, start); err != nil {
		return nil, nil, err
	}
	if start == "" {
		start = def
	}
	return build(g, rules, start, control), names, nil
}

// newViewClassifier returns a classifier of the normal view of rules (normal_view.go) from start.
// What the runtime decides by the grammar as it is written stays decided by rules: the
// left-recursion cycles and their leaders, the memoized rules, the variables that key the memo, and
// the run sites.
func newViewClassifier(g *grammar.Grammar, rules []*grammar.RuleDef, start string, control *cancel.Control) *classifier {
	an := AnalyzeControlled(rules, control)
	memo, _ := memoizedRules(rules, an, control)
	v := normalizeRules(rules, an, memo, costView, control)
	k := newClassifierOf(g, v.rules, rules, an, start, control)
	k.viewText = v.text
	return k
}

// written describes the causes as the grammar is written: a cause in the hidden rule of an inline
// action is in the rule that holds it, and the messages name the inline action.
func (c *Complexity) written(names desugar.InlineNames) *Complexity {
	if len(names) == 0 {
		return c
	}
	rule := func(r string) string {
		if holder, ok := names.Holder(r); ok {
			return holder
		}
		return r
	}
	for _, cs := range [][]ComplexityCause{c.Causes, c.Reasons, c.Env} {
		for i := range cs {
			cs[i].Rule, cs[i].Message = rule(cs[i].Rule), names.Text(cs[i].Message)
		}
	}
	for i := range c.Growing {
		c.Growing[i].Rule = rule(c.Growing[i].Rule)
	}
	return c
}

// rhsLevel marks the synthetic call that stands for the right operand of an operator part of an
// Operators Rule. A right operand is parsed at a new position like a call, but it is not one: the
// memo does not answer it and it shares nothing.
const rhsLevel = "\x00right operand"

type classifier struct {
	control *cancel.Control
	g       *grammar.Grammar
	rules   []*grammar.RuleDef
	byName  map[string]*grammar.RuleDef
	start   string
	an      *Analysis
	types   *ValueRecord // the value types of the terms, or nil if they are unknown
	// combinations is the product of the sizes of the value sets of the variables with finite
	// values, which checkValues computes.
	combinations int
	// stringLen reports that a len counted its operand, without its type, as a string value of
	// unbounded size.
	stringLen bool

	reach     map[string]bool // rules that the start rule reaches
	calls     map[string][]string
	scc       map[string]int // the strongly connected component of the call graph
	recursive map[string]bool
	memo      map[string]bool
	// members are the rules of left-recursion cycles, other than leaders, that are memoized at a
	// call outside the growths of their cycle at its position; outside are those calls.
	members map[string]bool
	outside map[*grammar.Ref]bool
	// starters are the rules of cycles, other than leaders, whose left calls of leaders can start
	// growths (startsGrowth); computed after the walk.
	starters map[string]bool
	// fixed records, by cycle, whether the bodies of the cycle evaluate every scanning source at a
	// position that is the same in each step of a growth inside a memoized call (fixedMemoized).
	fixed   map[int]bool
	vars    map[string][]string
	varType map[string]string

	nullRule    map[string]bool
	firstRule   map[string]charset
	boundedRule map[string]bool
	reachFrom   map[string]map[string]bool

	sites           []*site
	nestSites       map[*site]bool // the call sites whose evaluations can nest and end at one position (nestsAt)
	nestCycles      map[int]bool   // the left-recursion cycles with a call of a rule that nests at one end (nestsEnd)
	nestDry         bool           // nestsAt is being computed: no call site nests yet
	restarts        []restart      // the shared calls that nestsAt reads (recordShared)
	usedRestarts    []restart      // the shared calls that the checks of the units relied on
	retried         []retriedPart  // the operator parts after operands (addRetried)
	units           []*unit
	walked          map[string][]*source // the scanning sources that the walk found in each rule
	local           map[string]*source   // the first scanning source in the body or action of a rule
	ruleSrc         map[string]*source   // the first scanning source that a rule contains, through calls
	follow          map[string]charset
	growing         map[int]map[string]charset // FOLLOW without the seed reads of a cycle, by cycle
	binds           []*binding
	sourceIndex     map[sourceKey]*source
	predicateSource map[*grammar.Predicate]*source
	repeatedRules   map[string]string
	runMemo         map[*grammar.Repeat]bool
	runRestart      map[*grammar.Repeat]bool
	runBad          map[string]bool
	includesMemo    map[sizeKey]bool
	feedsMemo       map[feedKey]bool
	heldMemo        map[sizeKey][]sizeKey
	sizes           sizeMemo              // the size degrees of the values of rules (complexity_sizes.go)
	srcExprs        map[grammar.Expr]bool // the expressions that are scanning sources
	failRule        map[string]bool       // rules that can fail after a scanning source examined input
	srcMemo         map[grammar.Expr]bool
	again           map[*grammar.Repeat]*grammar.Repeat  // the rest of each repetition (againOf)
	captures        map[string]map[string][]grammar.Expr // captured expressions by rule and name

	scanRun     map[*grammar.Repeat]int  // the longest run before each repetition (computeBoundedScans)
	scanCounted map[*grammar.Repeat]bool // the repetitions with a constant number of evaluations at a position
	regionScan  map[*grammar.Repeat]bool // the bodies of trivia rules with the region bound (regionScans)
	stepRules   map[string]bool          // the rules called in a repetition that each step of a growth evaluates
	sureMemo    map[grammar.Expr]charset // what sureOf has found
	sureRule    map[string]charset       // what sure has found for the body of each rule
	rigidRule   map[string]bool          // what rigid has found for the body of each rule

	trivia map[string]bool // the trivia rules among the rules asked about (isTrivia)
	// exclOverBudget counts the comparisons of the exclusion analysis that exceeded exclBudget
	// and were left undecided.
	exclOverBudget int

	causes, reasons, env []ComplexityCause

	// base are the rules of the grammar when rules is a view of them, or nil. baseRuns is the
	// classifier of base that decides the run sites (runSite), and baseRepeats are the repetitions
	// of base.
	base        []*grammar.RuleDef
	baseRuns    *classifier
	baseRepeats map[*grammar.Repeat]bool
	// viewText gives the text of the grammar that an expression of the view stands for.
	viewText map[grammar.Expr]string
}

func newClassifier(g *grammar.Grammar, rules []*grammar.RuleDef, start string, control *cancel.Control) *classifier {
	return newClassifierOf(g, rules, rules, AnalyzeControlled(rules, control), start, control)
}

// newClassifierOf returns a classifier of the bodies of rules, which are those of base or a view
// of them. The analysis an of base, its variables and its memoization stand for the grammar.
func newClassifierOf(g *grammar.Grammar, rules, base []*grammar.RuleDef, an *Analysis, start string, control *cancel.Control) *classifier {
	k := &classifier{control: control, g: g, rules: rules, start: start, byName: map[string]*grammar.RuleDef{},
		calls: map[string][]string{}, scc: map[string]int{}, recursive: map[string]bool{}, memo: map[string]bool{},
		varType: map[string]string{}, nullRule: map[string]bool{}, firstRule: map[string]charset{},
		boundedRule: map[string]bool{}, reachFrom: map[string]map[string]bool{}, local: map[string]*source{}, walked: map[string][]*source{},
		ruleSrc: map[string]*source{}, follow: map[string]charset{}, growing: map[int]map[string]charset{},
		captures: map[string]map[string][]grammar.Expr{}, sourceIndex: map[sourceKey]*source{},
		predicateSource: map[*grammar.Predicate]*source{}, includesMemo: map[sizeKey]bool{}, feedsMemo: map[feedKey]bool{}, heldMemo: map[sizeKey][]sizeKey{},
		srcExprs: map[grammar.Expr]bool{}, srcMemo: map[grammar.Expr]bool{}, again: map[*grammar.Repeat]*grammar.Repeat{}}
	for _, r := range rules {
		k.byName[r.Name] = r
	}
	k.an = an
	k.vars = ruleVariablesControlled(base, control)
	k.memo, k.members = memoizedRules(base, k.an, control)
	if len(base) != len(rules) || !slices.Equal(base, rules) {
		k.base = base
	}
	k.outside = map[*grammar.Ref]bool{}
	for _, r := range rules {
		k.calls[r.Name] = allCallsControlled(r.Expr, nil, control)
	}
	for i, comp := range Tarjan(rules, k.calls, control) {
		for _, n := range comp {
			k.scc[n] = i
			if len(comp) > 1 || slices.Contains(k.calls[n], n) {
				k.recursive[n] = true
			}
		}
	}
	k.reach = k.reachable(start)
	for _, v := range g.Vars() {
		k.varType[v.Name] = varTypeName(v)
	}
	if triviaMemo {
		for _, r := range rules {
			if k.isTrivia(&grammar.Ref{Name: r.Name}) {
				k.memo[r.Name] = true
			}
		}
	}
	return k
}

// memoizedRules returns the rules whose results every parse memoizes: the leaders of
// left-recursion cycles, and the rules that are neither on a cycle nor transient. Another rule of a
// cycle is memoized only outside the growths whose seeds it reads, so it counts as not memoized. A
// rule that
// contains #missing, directly or through its callees, is not memoized when missing tokens are
// enabled, so it counts as not memoized unless it is a leader.
func memoizedRules(rules []*grammar.RuleDef, an *Analysis, control *cancel.Control) (out, members map[string]bool) {
	transient := transientRules(rules, control)
	missing := missingRules(rules, control)
	out, members = map[string]bool{}, map[string]bool{}
	for _, r := range rules {
		out[r.Name] = an.leaders[r.Name] || !an.cyclic[r.Name] && !transient[r.Name] && !missing[r.Name]
		members[r.Name] = an.cyclic[r.Name] && !an.leaders[r.Name] && !transient[r.Name] && !missing[r.Name]
	}
	return out, members
}

// missingRules returns the rules that contain #missing, directly or through their callees.
func missingRules(rules []*grammar.RuleDef, control *cancel.Control) map[string]bool {
	out := map[string]bool{}
	calls := map[string][]string{}
	for _, r := range rules {
		calls[r.Name] = allCallsControlled(r.Expr, nil, control)
		walkExprControlled(r.Expr, func(e grammar.Expr) {
			if a, ok := e.(*grammar.Attributed); ok && hasPragma(a, "missing") {
				out[r.Name] = true
			}
		}, control)
	}
	propagate(rules, calls, out, control)
	return out
}

func varTypeName(v *grammar.VarDef) string {
	if t, ok := v.Type.(*grammar.TypeRef); ok {
		return t.Name
	}
	switch v.Value.(type) {
	case *grammar.StringLit:
		return "string"
	case *grammar.IntLit:
		return "int"
	case *grammar.BoolLit:
		return "bool"
	}
	return ""
}

func (k *classifier) reachable(from string) map[string]bool {
	if m := k.reachFrom[from]; m != nil {
		return m
	}
	seen := map[string]bool{}
	var visit func(n string)
	visit = func(n string) {
		if seen[n] || k.byName[n] == nil {
			return
		}
		seen[n] = true
		for _, c := range k.calls[n] {
			k.control.Poll()
			visit(c)
		}
	}
	visit(from)
	k.reachFrom[from] = seen
	return seen
}

// shareable reports whether a call of the rule is answered alike in a unit and its continuation:
// the rule reads no variable and is memoized or calls no rule.
func (k *classifier) shareable(r *grammar.Ref) bool {
	return r.Level != rhsLevel && len(k.vars[r.Name]) == 0 && (k.memoAt(r) || len(k.calls[r.Name]) == 0)
}

// memoAt reports whether the memo answers the call r: the rule is memoized, or it is a member of a
// cycle that is memoized and the call is outside the growths of its cycle at its position.
func (k *classifier) memoAt(r *grammar.Ref) bool {
	return k.memo[r.Name] || k.members[r.Name] && k.outside[r]
}

// sameCycle reports whether a and b are rules of one left-recursion cycle.
func (k *classifier) sameCycle(a, b string) bool {
	return k.an.cyclic[a] && k.an.cyclic[b] && k.an.cycle[a] == k.an.cycle[b]
}

func (k *classifier) classify() *Complexity {
	k.collect()
	k.computeBoundedScans()
	k.settleSources()
	k.computeRuleSources()
	k.computeFailRules()
	k.computeFollow()
	k.addRetried()
	k.checkUnits()
	k.checkRestarts()
	k.checkGrowth()
	k.checkCarriedGrowth()
	k.checkNesting()
	k.checkValues()
	return k.result()
}

// collect collects the units, sources and bindings of the rules that the start rule reaches.
func (k *classifier) collect() {
	k.computeBounded()
	k.computeFirst()
	// The size of a value can depend on the captures of a rule that the walk reaches later, so
	// every rule's captures are known before the walk.
	for _, r := range k.rules {
		if k.reach[r.Name] {
			k.collectCaptures(r)
		}
	}
	for _, r := range k.rules {
		if k.reach[r.Name] {
			k.walkRule(r)
		}
	}
}

func (k *classifier) result() *Complexity {
	c := &Complexity{Memoized: k.memo, Combinations: max(k.combinations, 1)}
	sortCauses := func(cs []ComplexityCause) []ComplexityCause {
		slices.SortStableFunc(cs, func(a, b ComplexityCause) int {
			if a.Pos.Line != b.Pos.Line {
				return a.Pos.Line - b.Pos.Line
			}
			if a.Pos.Col != b.Pos.Col {
				return a.Pos.Col - b.Pos.Col
			}
			return strings.Compare(a.Message, b.Message)
		})
		return slices.CompactFunc(cs, func(a, b ComplexityCause) bool { return a == b })
	}
	c.Causes, c.Reasons, c.Env = sortCauses(k.causes), sortCauses(k.reasons), sortCauses(k.env)
	c.Growing = k.growingBindings()
	// A reason leaves the cost open, so a degree of the causes bounds it only without reasons; an
	// exponential cause is evidence of growth that no polynomial bounds, so it outranks them.
	switch {
	case slices.ContainsFunc(c.Causes, func(x ComplexityCause) bool { return x.Class == ClassExp }):
		c.Class = ClassExp
	case len(c.Reasons) > 0:
		c.Class = ClassUnknown
	case len(c.Causes) > 0:
		c.Class = ClassPoly
	case len(c.Env) > 0:
		c.Class = ClassLinearEnv
	}
	for _, x := range c.Causes {
		switch c.Class {
		case ClassPoly:
			c.Degree = max(c.Degree, x.Degree)
		case ClassUnknown:
			c.WitnessedDegree = max(c.WitnessedDegree, x.Degree)
		}
	}
	return c
}

// --- Positions and descriptions ---

func exprPos(e grammar.Expr) grammar.Pos {
	if s, ok := e.(interface{ Source() *grammar.SourceInfo }); ok && s.Source() != nil {
		return s.Source().Diagnostic()
	}
	return grammar.Pos{}
}

func termPosOf(t grammar.Term) grammar.Pos {
	if s, ok := t.(interface{ Source() *grammar.SourceInfo }); ok && s.Source() != nil {
		return s.Source().Diagnostic()
	}
	return grammar.Pos{}
}

// at describes a position in a rule, as "3:12 in rule A", or "rule A" without a position.
func at(rule string, p grammar.Pos) string {
	if p.IsValid() {
		return fmt.Sprintf("%d:%d in rule %s", p.Line, p.Col, rule)
	}
	return "rule " + rule
}

func (k *classifier) show(e grammar.Expr) string {
	if t, ok := k.viewText[e]; ok {
		return abbreviate(t)
	}
	return abbreviate(grammar.FormatExpr(e))
}

func (k *classifier) showTerm(t grammar.Term) string {
	return abbreviate(grammar.FormatTerm(t))
}

// abbreviate collapses the white space of the source text s and shortens it to at most 40 bytes,
// cutting at a character boundary.
func abbreviate(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= 40 {
		return s
	}
	n := 37
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "..."
}

func (k *classifier) cause(class ComplexityClass, degree int, rule string, pos grammar.Pos, format string, args ...any) {
	k.causes = append(k.causes, ComplexityCause{Class: class, Degree: degree, Rule: rule, Pos: pos, Message: fmt.Sprintf(format, args...)})
}

func (k *classifier) reason(rule string, pos grammar.Pos, format string, args ...any) {
	k.reasons = append(k.reasons, ComplexityCause{Class: ClassUnknown, Rule: rule, Pos: pos, Message: fmt.Sprintf(format, args...)})
}

func (k *classifier) envCause(rule string, pos grammar.Pos, format string, args ...any) {
	k.env = append(k.env, ComplexityCause{Class: ClassLinearEnv, Rule: rule, Pos: pos, Message: fmt.Sprintf(format, args...)})
}

func degreeText(class ComplexityClass, degree int) string {
	if class == ClassExp {
		return "the cost can grow exponentially"
	}
	return fmt.Sprintf("the cost can grow as n^%d", degree)
}
