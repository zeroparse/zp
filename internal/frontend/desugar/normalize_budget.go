package desugar

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Copies use private destination storage, so cancellation never changes the
// source AST. Allocations themselves remain nonpreemptible.
func normalizationCopy[T any](dst, src []T, control *cancel.Control) {
	if control == nil {
		copy(dst, src)
		return
	}
	for off, end := 0, min(len(dst), len(src)); off < end; {
		control.Check()
		next := min(off+256, end)
		copy(dst[off:next], src[off:next])
		off = next
	}
}

// normalizationClone returns a copy of src, polling control while copying.
func normalizationClone[T any](src []T, control *cancel.Control) []T {
	if control == nil {
		return append([]T(nil), src...)
	}
	control.Check()
	if len(src) == 0 {
		return nil
	}
	out := make([]T, len(src))
	normalizationCopy(out, src, control)
	return out
}

// appendNormalizedAttribute returns a copy of src with a appended, polling control while copying.
func appendNormalizedAttribute(src []*ast.Attribute, a *ast.Attribute, control *cancel.Control) []*ast.Attribute {
	if control == nil {
		return append(append([]*ast.Attribute(nil), src...), a)
	}
	control.Check()
	out := make([]*ast.Attribute, len(src)+1)
	normalizationCopy(out, src, control)
	out[len(src)] = a
	return out
}

func appendNormalizedPredicates(dst, src []ast.Expr, control *cancel.Control) []ast.Expr {
	if control == nil {
		return append(dst, src...)
	}
	for off := 0; off < len(src); {
		control.Check()
		next := min(off+256, len(src))
		dst = append(dst, src[off:next]...)
		off = next
	}
	return dst
}
