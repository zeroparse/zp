//go:build !zp_long

package compile_test

// factsSeeds is the number of random grammars TestFactsDifferential compares: few in the default
// tier, many in the long tier (go test -tags zp_long).
const factsSeeds = 300
