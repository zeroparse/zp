package codegentest

import "strings"

// MutateFunc replaces the first old by new in the body of the Go function or method fn of src, the
// text from "func ... fn(" to the next function, and reports whether src has old there. It makes the
// source mutants of the printed code that the tests of operators hold the implementations to.
func MutateFunc(src, fn, old, new string) (string, bool) {
	start := -1
	for _, pre := range []string{") " + fn + "(", "func " + fn + "("} {
		if i := strings.Index(src, pre); i >= 0 {
			start = i
			break
		}
	}
	if start < 0 {
		return src, false
	}
	end := len(src)
	if i := strings.Index(src[start:], "\nfunc "); i >= 0 {
		end = start + i
	}
	body := src[start:end]
	if !strings.Contains(body, old) {
		return src, false
	}
	return src[:start] + strings.Replace(body, old, new, 1) + src[end:], true
}
