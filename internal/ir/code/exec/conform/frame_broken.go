package conform

// FrameBroken is a wrong version of one frame operation, the negative control of the cases of the
// capture frames: a model or a target whose operation does what Edit does fails a case.
type FrameBroken struct {
	Name, Op string
	// Edit returns the results of the broken operation for args, given a function that calls the
	// correct operation named op of the model or the target.
	Edit func(call func(op string, args ...any) []any, args []any) []any
}

// FrameBrokenOps returns the broken versions of the frame operations that the cases must tell from the
// correct ones; with missing, also those of the operations of a target with the missing-token record.
func FrameBrokenOps(missing bool) []FrameBroken {
	var bs []FrameBroken
	add := func(name, op string, edit func(call func(string, ...any) []any, a []any) []any) {
		bs = append(bs, FrameBroken{Name: name, Op: op, Edit: edit})
	}
	plus := func(op string) {
		add(op+" is one more", op, func(call func(string, ...any) []any, a []any) []any {
			return []any{call(op, a...)[0].(int64) + 1}
		})
	}
	ignore := func(op string) {
		add(op+" is ignored", op, func(call func(string, ...any) []any, a []any) []any { return nil })
	}
	for _, op := range []string{"frame.markPos", "frame.markRec", "frame.markTrail", "frame.slots"} {
		plus(op)
	}
	if missing {
		plus("frame.markMissing")
	}
	add("frame.markEnv is the current environment", "frame.markEnv", func(call func(string, ...any) []any, a []any) []any {
		return call("env.scope")
	})
	for _, op := range []string{"frame.undo", "frame.use", "frame.set", "frame.save", "frame.restore"} {
		ignore(op)
	}
	add("frame.undo keeps one more write", "frame.undo", func(call func(string, ...any) []any, a []any) []any {
		n := call("frame.markTrail", call("frame.mark")[0])[0].(int64)
		return call("frame.undo", min(a[0].(int64)+1, n))
	})
	add("frame.current is the null frame", "frame.current", func(call func(string, ...any) []any, a []any) []any {
		return []any{nil}
	})
	add("frame.open opens one more slot", "frame.open", func(call func(string, ...any) []any, a []any) []any {
		return call("frame.open", a[0].(int64)+1)
	})
	add("frame.set writes the first slot", "frame.set", func(call func(string, ...any) []any, a []any) []any {
		return call("frame.set", int64(0), a[1])
	})
	return bs
}
