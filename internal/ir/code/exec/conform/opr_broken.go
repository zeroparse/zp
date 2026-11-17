package conform

import (
	"strings"
)

// OprBroken is a wrong version of one operation of the Operators Rule family, the negative control of its
// cases: a model or a target whose operation does what Edit does fails a case.
type OprBroken struct {
	Name, Op string
	// Edit returns the results of the broken operation for args, given a function that calls the
	// correct operation named op of the model or the target.
	Edit func(call func(op string, args ...any) []any, args []any) []any
}

// OprBrokenOps returns the broken versions of the operations that the cases of the Operators Rule family
// must tell from the correct ones.
func OprBrokenOps() []OprBroken {
	var bs []OprBroken
	add := func(name, op string, edit func(call func(string, ...any) []any, a []any) []any) {
		bs = append(bs, OprBroken{Name: name, Op: op, Edit: edit})
	}
	negate := func(op string) {
		add(op+" is negated", op, func(call func(string, ...any) []any, a []any) []any {
			return []any{!call(op, a...)[0].(bool)}
		})
	}
	plus := func(op string) {
		add(op+" is one more", op, func(call func(string, ...any) []any, a []any) []any {
			return []any{call(op, a...)[0].(int64) + 1}
		})
	}
	other := func(op, with string) {
		add(op+" reads "+with, op, func(call func(string, ...any) []any, a []any) []any { return call(with, a...) })
	}
	ignore := func(op string) {
		add(op+" is ignored", op, func(call func(string, ...any) []any, a []any) []any { return nil })
	}
	other("opr.led", "opr.prefix")
	other("opr.prefix", "opr.led")
	other("opr.right", "opr.nonAssoc")
	other("opr.postfix", "opr.infix")
	for _, op := range []string{"opr.count", "opr.operands", "opr.level", "opr.scope", "opr.nudBound"} {
		plus(op)
	}
	for _, op := range []string{"opr.infix", "opr.postfix", "opr.right", "opr.nonAssoc", "opr.hasSkip", "opr.cut", "opr.cutFailed", "opr.afterEmpty"} {
		negate(op)
	}
	add("opr.at stops at the first operator", "opr.at", func(call func(string, ...any) []any, a []any) []any {
		return call("opr.at", a[0], int64(0))
	})
	add("opr.at is the next operator", "opr.at", func(call func(string, ...any) []any, a []any) []any {
		n := call("opr.count", a[0])[0].(int64)
		return call("opr.at", a[0], min(a[1].(int64)+1, n-1))
	})
	add("opr.operand stops at the first line", "opr.operand", func(call func(string, ...any) []any, a []any) []any {
		return call("opr.operand", a[0], int64(0))
	})
	add("opr.end is the start of the parse", "opr.end", func(call func(string, ...any) []any, a []any) []any {
		return []any{int64(0)}
	})
	add("opr.found is always true", "opr.found", func(call func(string, ...any) []any, a []any) []any { return []any{true} })
	add("opr.found is always false", "opr.found", func(call func(string, ...any) []any, a []any) []any { return []any{false} })
	add("opr.setNudBound is one less", "opr.setNudBound", func(call func(string, ...any) []any, a []any) []any {
		return call("opr.setNudBound", a[0].(int64)-1)
	})
	ignore("opr.setCutFailed")
	ignore("opr.setNudBound")
	ignore("opr.setAfterEmpty")
	// A setter that is ignored, and a getter that reads the field of the same type that the record has
	// beside it.
	swaps := map[string]string{
		"oprLongest.prevFrame": "oprLongest.f", "oprLongest.f": "oprLongest.prevFrame", "oprLongest.cutFailed": "oprLongest.prevCut",
		"oprLongest.prevCut": "oprLongest.cutFailed", "oprNud.prevFrame": "oprNud.f", "oprNud.f": "oprNud.prevFrame",
		"oprExpr.min": "oprExpr.bound", "oprExpr.bound": "oprExpr.lastNone", "oprExpr.lastNone": "oprExpr.min",
		"oprExpr.guard": "oprExpr.bound",
	}
	for _, name := range OprRecordOps {
		if strings.Contains(name, ".set") {
			ignore(name)
		} else if with, ok := swaps[name]; ok {
			other(name, with)
		}
	}
	return bs
}
