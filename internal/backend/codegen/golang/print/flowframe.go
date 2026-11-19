package print

import (
	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/middleend/flowlower"
)

// typedFrameComment reports whether the body of the typed rule r is printed in place under the
// runtime-owned frame, a left-recursion leader or a rule that recovers, and returns the comment of
// its matcher.
func (g *Generator) typedFrameComment(r *common.Rule) (string, bool) {
	comment := "typed recovery body inlined"
	if r.Leader {
		comment = "typed LR body inlined"
	} else if !exprRecovers(r.BodyExpr()) {
		return "", false
	}
	if !flowlower.BodyOK(r.BodyExpr(), true, true) {
		return "", false
	}
	return comment, true
}
