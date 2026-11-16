package libgo

import (
	"testing"

	"zeroparse.dev/zp/internal/codegen"
)

// TestFeaturesMatchAnalysis checks that the library's features have the bits of the generator's
// feature analysis, which a parser generated for the tests records (WitnessFeatures).
func TestFeaturesMatchAnalysis(t *testing.T) {
	pairs := []struct {
		lib feature
		gen codegen.Feature
	}{
		{featureDocument, codegen.FeatureDocument}, {featureStream, codegen.FeatureStream},
		{featureObserve, codegen.FeatureObserve}, {featureBudget, codegen.FeatureBudget},
		{featureMemo, codegen.FeatureMemo}, {featureLeftRecursion, codegen.FeatureLeftRecursion},
		{featureRecovery, codegen.FeatureRecovery}, {featureOperators, codegen.FeatureOperators},
		{featureVars, codegen.FeatureVars}, {featureSkipRuns, codegen.FeatureSkipRuns},
		{featureScanSites, codegen.FeatureScanSites},
	}
	if len(pairs) != len(codegen.FeatureNames()) {
		t.Fatalf("the library declares %d features, the analysis %d", len(pairs), len(codegen.FeatureNames()))
	}
	var all uint32
	for i, p := range pairs {
		if uint32(p.lib) != uint32(p.gen) {
			t.Errorf("feature %s: library %#x, analysis %#x", codegen.FeatureNames()[i], uint32(p.lib), uint32(p.gen))
		}
		all |= uint32(p.gen)
	}
	if all != codegen.AllFeatures || all&uint32(featureWitnessed) != 0 {
		t.Errorf("the features %#x are not the analysis's %#x, or use the bit of featureWitnessed", all, codegen.AllFeatures)
	}
}
