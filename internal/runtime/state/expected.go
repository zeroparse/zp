package state

import (
	"fmt"
	"slices"
	"strings"
)

// expectedKind ranks an expected entry by its kind: a named entry (such as "end of input" or a
// label) first, then a literal, which begins with a double quote, then a character class, which
// begins with [.
func expectedKind(s string) int {
	if s != "" {
		switch s[0] {
		case '"':
			return 1
		case '[':
			return 2
		}
	}
	return 0
}

// compareExpected orders expected entries: by kind (expectedKind), then by their bytes.
func compareExpected(a, b string) int {
	if ka, kb := expectedKind(a), expectedKind(b); ka != kb {
		return ka - kb
	}
	return strings.Compare(a, b)
}

// sortExpectedEntries sorts expected entries in the order of compareExpected.
func sortExpectedEntries(values []string) { slices.SortFunc(values, compareExpected) }

// expectedTail is the end of a syntax error message whose list of expected entries leaves out rest
// entries (core.SyntaxError.ExpectedInMessage). It matches the text of core.SyntaxError.Message,
// which TestBudgetDiagnosticRecoveryTextMatchesNative checks.
func expectedTail(rest int) string { return fmt.Sprintf(", ... and %d more", rest) }
