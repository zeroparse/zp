//go:build zp_feature_witness

package libgo

// witnessFeatures turns on the check of use.
const witnessFeatures = true

// WitnessFeatures records the features that the generator found for the parser of g (the bits of
// its feature analysis), so that every use of a feature is checked against them. Only parsers
// generated for the tests of the analysis call it, and only builds with the test tag
// zp_feature_witness declare it, so it is not part of the protocol.
func (g *grammar) WitnessFeatures(f uint32) { g.features = feature(f) | featureWitnessed }
