package conform

// CtxBrokenOps returns the broken versions of the operations of the dependency memo that its cases
// must tell from the correct ones, as OprBrokenOps does for the Operators Rule family.
func CtxBrokenOps() []OprBroken {
	var bs []OprBroken
	add := func(name, op string, edit func(call func(string, ...any) []any, a []any) []any) {
		bs = append(bs, OprBroken{Name: name, Op: op, Edit: edit})
	}
	ignore := func(op string) {
		add(op+" is ignored", op, func(call func(string, ...any) []any, a []any) []any { return nil })
	}
	add("ctx.allows ignores the present set", "ctx.allows", func(call func(string, ...any) []any, a []any) []any {
		return []any{true}
	})
	add("ctx.allows is negated", "ctx.allows", func(call func(string, ...any) []any, a []any) []any {
		return []any{!call("ctx.allows", a...)[0].(bool)}
	})
	add("ctx.active is negated", "ctx.active", func(call func(string, ...any) []any, a []any) []any {
		return []any{!call("ctx.active", a...)[0].(bool)}
	})
	add("ctx.ok is negated", "ctx.ok", func(call func(string, ...any) []any, a []any) []any {
		return []any{!call("ctx.ok", a...)[0].(bool)}
	})
	add("ctx.found is always false", "ctx.found", func(call func(string, ...any) []any, a []any) []any {
		return []any{false}
	})
	add("ctx.pop always stores in the memo", "ctx.pop", func(call func(string, ...any) []any, a []any) []any {
		call("ctx.pop", a...)
		return []any{call("ctx.storeMemo")[0]}
	})
	add("ctx.storeCtx is ctx.storeMemo", "ctx.storeCtx", func(call func(string, ...any) []any, a []any) []any {
		return call("ctx.storeMemo")
	})
	add("ctx.replay replays a seed", "ctx.replay", func(call func(string, ...any) []any, a []any) []any {
		return call("ctx.replay", a[0], true)
	})
	ignore("ctx.keep")
	ignore("ctx.done")
	ignore("ctx.replay")
	ignore("memo.setPresent")
	return bs
}
