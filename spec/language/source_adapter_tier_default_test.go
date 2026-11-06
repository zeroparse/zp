//go:build !zp_long

package language_test

const (
	// mutationCases is the number of mutated grammars the default tier compares for each seed.
	mutationCases = 3000
)

// mutationSeeds are the seeds of the random mutations of the tier.
var mutationSeeds = []int64{7, 1, 2}
