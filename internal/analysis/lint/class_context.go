package lint

import (
	"slices"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func classAcceptsControlled(c *ast.CharClass, r rune, control *lintControl) bool {
	if control == nil {
		return classAccepts(c, r)
	}
	for _, rg := range c.Ranges {
		control.poll()
		if rg.Lo <= r && r <= rg.Hi {
			return !c.Negated
		}
	}
	return c.Negated
}

func normalizeControlled(rs []ast.CharRange, control *lintControl) []ast.CharRange {
	if control == nil {
		return normalize(rs)
	}
	var out []ast.CharRange
	if len(rs) != 0 {
		out = make([]ast.CharRange, len(rs))
	}
	for i, r := range rs {
		control.poll()
		out[i] = r
	}
	slices.SortFunc(out, func(a, b ast.CharRange) int { control.poll(); return int(a.Lo - b.Lo) })
	return mergeRanges(out, control)
}

func mergeRanges(out []ast.CharRange, control *lintControl) []ast.CharRange {
	var merged []ast.CharRange
	for _, r := range out {
		control.poll()
		if n := len(merged); n > 0 && r.Lo <= merged[n-1].Hi+1 {
			merged[n-1].Hi = max(merged[n-1].Hi, r.Hi)
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

func sameRangesControlled(x, y []ast.CharRange, control *lintControl) bool {
	if control == nil {
		return sameRanges(x, y)
	}
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		control.poll()
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
