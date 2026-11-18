package golib

import (
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
)

// TestFeaturesMatchAnalysis checks that the library's features have the bits of the generator's
// feature analysis, which a parser generated for the tests records (WitnessFeatures).
func TestFeaturesMatchAnalysis(t *testing.T) {
	pairs := []struct {
		lib feature
		gen common.Feature
	}{
		{featureDocument, common.FeatureDocument}, {featureStream, common.FeatureStream},
		{featureObserve, common.FeatureObserve}, {featureBudget, common.FeatureBudget},
		{featureMemo, common.FeatureMemo}, {featureLeftRecursion, common.FeatureLeftRecursion},
		{featureRecovery, common.FeatureRecovery}, {featureOperators, common.FeatureOperators},
		{featureVars, common.FeatureVars}, {featureSkipRuns, common.FeatureSkipRuns},
		{featureScanSites, common.FeatureScanSites},
	}
	if len(pairs) != len(common.FeatureNames()) {
		t.Fatalf("the library declares %d features, the analysis %d", len(pairs), len(common.FeatureNames()))
	}
	var all uint32
	for i, p := range pairs {
		if uint32(p.lib) != uint32(p.gen) {
			t.Errorf("feature %s: library %#x, analysis %#x", common.FeatureNames()[i], uint32(p.lib), uint32(p.gen))
		}
		all |= uint32(p.gen)
	}
	if all != common.AllFeatures || all&uint32(featureWitnessed) != 0 {
		t.Errorf("the features %#x are not the analysis's %#x, or use the bit of featureWitnessed", all, common.AllFeatures)
	}
}
