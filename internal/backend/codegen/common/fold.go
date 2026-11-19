package common

import "zeroparse.dev/zp/internal/runtime/state"

// FoldedRunes returns rs with the ASCII folding of #ignorecase (state.FoldASCIIRune) applied to each
// rune: the text a generated parser compares a folded literal with.
func FoldedRunes(rs []rune) []rune {
	out := make([]rune, len(rs))
	for i, r := range rs {
		out[i] = state.FoldASCIIRune(r)
	}
	return out
}
