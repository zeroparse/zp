//go:build !zp_feature_witness

package libgo

// witnessFeatures turns off the check of use, which then compiles away.
const witnessFeatures = false
