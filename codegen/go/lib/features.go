package libgo

import "fmt"

// Feature is a mechanism of the library that a generated parser uses only if its grammar or its
// generation options can need it. The values are those of the generator's feature analysis, which
// decides the features of each parser.
type Feature uint32

// The features, in the order of the generator's analysis.
const (
	FeatureDocument Feature = 1 << iota
	FeatureStream
	FeatureObserve
	FeatureBudget
	FeatureMemo
	FeatureLeftRecursion
	FeatureRecovery
	FeatureOperators
	FeatureVars
	FeatureSkipRuns
	FeatureScanSites
)

// WitnessFeatures records the features that the generator found for the parser of g, so that a build
// with the tag zp_feature_witness checks every use of a feature against them. Only parsers generated
// for the tests of the analysis call it.
//
// WitnessFeatures is for generated parsers; not a stable API.
func (g *grammar) WitnessFeatures(f Feature) { g.features = f | featureWitnessed }

// featureWitnessed marks the features of a grammar as recorded, so that a parser for which nothing was
// recorded is not checked.
const featureWitnessed Feature = 1 << 31

// use reports that the parse uses the feature f. With the tag zp_feature_witness it panics when the
// generator ruled f out for the parser; otherwise it does nothing and compiles away.
func (p *Parser) use(f Feature) {
	if witnessFeatures {
		useFeature(p.g, f)
	}
}

// useFeature is use for the grammar g.
func useFeature(g *grammar, f Feature) {
	if g != nil && g.features&featureWitnessed != 0 && g.features&f != f {
		panic(fmt.Sprintf("zp: the parser uses feature %#x outside its analysed features %#x", uint32(f), uint32(g.features&^featureWitnessed)))
	}
}
