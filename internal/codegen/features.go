package codegen

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/grammaranalysis"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/opt"
)

// A Feature is a mechanism of the library of generated parsers that a parser runs only if its grammar
// or its generation options can need it. The feature analysis (Analyze) decides once per generation
// which features a parser can use, from the options and from the rules that its entry points reach;
// every target reads the same FeatureSet, so that no printer decides on its own which library code a
// parser carries.
//
// The analysis over-approximates: a feature that a parse could use is always in the set. The bits are
// part of the contract between the generator and the Go library (codegen/go/lib/features.go declares
// the same values), and a test checks that the two agree.
type Feature uint32

const (
	// FeatureDocument is the incremental Document (-document).
	FeatureDocument Feature = 1 << iota
	// FeatureStream is the streaming parse (-stream).
	FeatureStream
	// FeatureObserve is tracing and profiling (-observability).
	FeatureObserve
	// FeatureBudget is the limit controls: cancellation and the compute and memory limits (-limits).
	FeatureBudget
	// FeatureMemo is the memo table: a reached rule is memoized or leads a left-recursion cycle.
	FeatureMemo
	// FeatureLeftRecursion is the growth of left-recursion seeds and the dependency memo of their
	// contexts: a reached rule is on a left-recursion cycle.
	FeatureLeftRecursion
	// FeatureRecovery is error recovery: a reached rule contains a recovery (#recover, an Attributed pragma once desugared).
	FeatureRecovery
	// FeatureOperators is the Operators Rule: a reached rule is one.
	FeatureOperators
	// FeatureVars is the variable environment: the grammar declares variables.
	FeatureVars
	// FeatureSkipRuns is the skip-run records: the program has skip-run sites.
	FeatureSkipRuns
	// FeatureScanSites is the scan records: the program has scan sites.
	FeatureScanSites

	// AllFeatures is every feature: the set of a parser that the analysis does not narrow.
	AllFeatures = 1<<iota - 1
)

// featureNames are the names of the features, by bit, as FeatureSet.String writes them.
var featureNames = []string{"document", "stream", "observe", "budget", "memo", "leftRecursion", "recovery", "operators", "vars", "skipRuns", "scanSites"}

// FeatureNames returns the names of the features, by bit.
func FeatureNames() []string { return append([]string(nil), featureNames...) }

// FeatureSet is the features a generated parser can use.
type FeatureSet Feature

// Has reports whether s has every feature of f.
func (s FeatureSet) Has(f Feature) bool { return Feature(s)&f == f }

// String lists the names of the features of s, separated by commas.
func (s FeatureSet) String() string {
	var names []string
	for i, n := range featureNames {
		if Feature(s)&(1<<i) != 0 {
			names = append(names, n)
		}
	}
	return strings.Join(names, ",")
}

// UsedFeatures is the optimization that narrows a parser's features to those the analysis finds; with
// it off, a parser has AllFeatures, the differential baseline of the pruning.
const UsedFeatures = "used-features"

// Analyze returns the features of the parser generated with opts from prog and, if the parser has
// Recognize, from the recognizer rec (nil otherwise). The rules considered are those that the
// entry points can enter: every start rule (ParseRule and the typed entry points name any) and the
// start rule of opts, followed through the calls of the Core IR of each program, value-free twins
// included.
func Analyze(prog, rec *compile.Program, opts GenOptions) (FeatureSet, error) {
	var f Feature
	if opts.Document {
		f |= FeatureDocument
	}
	if opts.Stream {
		f |= FeatureStream
	}
	if opts.Observability {
		f |= FeatureObserve
	}
	if opts.LimitControls {
		f |= FeatureBudget
	}
	for _, p := range []*compile.Program{prog, rec} {
		if p == nil {
			continue
		}
		reached, err := reachedRules(p, opts.Start)
		if err != nil {
			return 0, err
		}
		for _, r := range reached {
			f |= ruleFeatures(r)
		}
		if p.Vars != nil {
			f |= FeatureVars
		}
		if p.NumSkipSites > 0 {
			f |= FeatureSkipRuns
		}
		if p.NumScanSites > 0 {
			f |= FeatureScanSites
		}
	}
	return FeatureSet(f), nil
}

// ruleFeatures returns the features that a call of r can use.
func ruleFeatures(r *compile.Rule) Feature {
	var f Feature
	if r.Memo || r.Leader {
		f |= FeatureMemo
	}
	if r.Leader || r.Member || r.Cycle != 0 {
		f |= FeatureLeftRecursion
	}
	if r.Def == nil {
		return f
	}
	if _, ok := r.Def.Expr.(*grammar.OperatorsRule); ok {
		f |= FeatureOperators
	}
	grammaranalysis.WalkExpr(r.Def.Expr, func(e grammar.Expr) {
		switch e := e.(type) {
		case *grammar.Recovery:
			f |= FeatureRecovery
		case *grammar.Attributed:
			for _, a := range e.Attrs {
				if a.Name == "recover" {
					f |= FeatureRecovery
				}
			}
		}
	})
	return f
}

// reachedRules returns the rules of p, value-free twins included, that a parse can enter from the
// start rules of p and the start rule named start.
func reachedRules(p *compile.Program, start string) ([]*compile.Rule, error) {
	m, err := irbuild.CoreFrom(p, nil)
	if err != nil {
		return nil, fmt.Errorf("feature analysis: %w", err)
	}
	var roots []core.Root
	for _, name := range p.StartRules {
		roots = append(roots, core.Root{Rule: p.ByName[name].ID, Entry: core.EnterSlot})
	}
	if s, err := p.StartRule(start); err == nil {
		roots = append(roots, core.Root{Rule: s.ID, Entry: core.EnterSlot})
	}
	rules := append(append([]*compile.Rule(nil), p.RuleList...), p.Twins...)
	var out []*compile.Rule
	for id, e := range m.Reach(roots, func(int) core.Entry { return core.EnterSlot }) {
		if e != 0 {
			out = append(out, rules[id])
		}
	}
	return out, nil
}

// ParserFeatures returns the features of the parser generated with opts and the optimization settings
// s: those Analyze finds, or AllFeatures with the optimization UsedFeatures off.
func ParserFeatures(prog, rec *compile.Program, opts GenOptions, s opt.Settings) (FeatureSet, error) {
	if !s.On(UsedFeatures) {
		return FeatureSet(AllFeatures), nil
	}
	return Analyze(prog, rec, opts)
}
