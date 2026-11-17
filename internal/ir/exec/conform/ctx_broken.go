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
	add("ctx.end is one further", "ctx.end", func(call func(string, ...any) []any, a []any) []any {
		return []any{call("ctx.end", a...)[0].(int64) + 1}
	})
	add("ctx.far is the start", "ctx.far", func(call func(string, ...any) []any, a []any) []any {
		return []any{int64(0)}
	})
	add("ctx.expected is empty", "ctx.expected", func(call func(string, ...any) []any, a []any) []any {
		return call("exp.empty")
	})
	add("ctx.errs is none", "ctx.errs", func(call func(string, ...any) []any, a []any) []any {
		return []any{nil}
	})
	add("grow.ctx is negated", "grow.ctx", func(call func(string, ...any) []any, a []any) []any {
		return []any{!call("grow.ctx", a...)[0].(bool)}
	})
	add("grow.longer ignores the seed", "grow.longer", func(call func(string, ...any) []any, a []any) []any {
		return []any{a[1].(bool)}
	})
	add("grow.adopt drops the errors", "grow.adopt", func(call func(string, ...any) []any, a []any) []any {
		return call("grow.adopt", a[0], a[1], nil)
	})
	ignore("grow.adopt")
	ignore("ctx.keep")
	ignore("ctx.done")
	ignore("memo.setPresent")
	return bs
}
