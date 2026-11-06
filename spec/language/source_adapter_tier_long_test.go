//go:build zp_long

package language_test

const (
	// mutationCases is the number of mutated grammars the long tier compares for each seed.
	mutationCases = 100000
)

// mutationSeeds are the seeds of the random mutations of the tier.
var mutationSeeds = []int64{7, 1, 2, 3, 4, 5, 6, 8}
