//go:build !zp_long

package complexitytest

// reduced reports whether a test measures the reduced sizes and counts of its series: always in the
// default tier, which keeps every control, corpus entry and family of random grammars at the sizes
// that go test -short runs.
func reduced() bool { return true }
