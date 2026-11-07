//go:build !unix

package main

import "os"

// peakRSS returns 0: the peak resident set size is read from the rusage of a Unix process.
func peakRSS(*os.ProcessState) int64 { return 0 }
