package libgo

import "fmt"

// feature is a mechanism of the library that a generated parser uses only if its grammar or its
// generation options can need it. The values are those of the generator's feature analysis, which
// decides the features of each parser; a test of this package checks that the two agree.
type feature uint32

// The features, in the order of the generator's analysis.
const (
	featureDocument feature = 1 << iota
	featureStream
	featureObserve
	featureBudget
	featureMemo
	featureLeftRecursion
	featureRecovery
	featureOperators
	featureVars
	featureSkipRuns
	featureScanSites
)

// featureWitnessed marks the features of a grammar as recorded (WitnessFeatures, declared only with
// the tag zp_feature_witness), so that a parser for which nothing was recorded is not checked.
const featureWitnessed feature = 1 << 31

// use reports that the parse uses the feature f. With the tag zp_feature_witness it panics when the
// generator ruled f out for the parser; otherwise it does nothing and compiles away.
func (p *Parser) use(f feature) {
	if witnessFeatures {
		useFeature(p.g, f)
	}
}

// useFeature is use for the grammar g.
func useFeature(g *grammar, f feature) {
	if g != nil && g.features&featureWitnessed != 0 && g.features&f != f {
		panic(fmt.Sprintf("zp: the parser uses feature %#x outside its analysed features %#x", uint32(f), uint32(g.features&^featureWitnessed)))
	}
}
