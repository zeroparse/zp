//go:build !zp_workcount

package parser

// charge counts n units of the work k in a build with the tag zp_workcount, and does nothing here.
func charge(k workKind, n int) {}
