package core

import (
	"go/format"
	"slices"
	"strconv"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/execgo"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// The conformance bindings of the operations that a target prints inline: for each, a function that
// calls only the operation, printed by the target's own printer, so that the conformance suite
// (internal/ir/exec/conform) runs the code that the target's compositions run, not a copy of it.

// EngineExpBindingsFile is the file of the engine's conformance bindings of the expectation record,
// relative to the directory of this package. It is a test file of package core of the engine.
const EngineExpBindingsFile = "../../../engine/core/exp_bind_exec_test.go"

// expBindingMemoOps are the operations of the memo that keep the expectations of an entry.
var expBindingMemoOps = []string{"memo.far", "memo.expected", "memo.setExpected"}

// ExpBindingName returns the name of the printed binding of the operation op: "conf" and the parts of
// its name, each capitalized, as confExpIsolate for exp.isolate.
func ExpBindingName(op string) string {
	var b strings.Builder
	b.WriteString("conf")
	for part := range strings.SplitSeq(op, ".") {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// docBindingEngineOps are the doc operations that the engine has a Go form for: it has no observation of
// calls, so it has neither doc.examinedBegin nor doc.examinedEnd.
var docBindingEngineOps = []string{"doc.enter", "doc.leave", "doc.active", "doc.touch", "doc.touchFrom", "doc.low", "doc.high",
	"doc.reused", "doc.evaluated"}

// docBindingOps are the doc operations of the node runtime.
var docBindingOps = []string{"doc.examinedBegin", "doc.examinedEnd", "doc.enter", "doc.leave", "doc.active", "doc.touch",
	"doc.touchFrom", "doc.low", "doc.high", "doc.reused", "doc.evaluated"}

// oprBindingDescriptorOps are the operations of the Operators Rule descriptors and of the matches of
// operators that conformance cases cover. oprBindingEngineOps adds those of the records of the split
// forms of the loop, which the engine has and the node runtime does not.
var oprBindingDescriptorOps = []string{
	"opr.led", "opr.prefix", "opr.count", "opr.at", "opr.operands", "opr.operand", "opr.line", "opr.level",
	"opr.infix", "opr.postfix", "opr.right", "opr.nonAssoc", "opr.scope", "opr.hasSkip",
	"opr.attempt", "opr.found", "opr.op", "opr.end", "opr.cut", "opr.env", "opr.errs",
	"opr.setCutFailed", "opr.cutFailed", "opr.setNudBound", "opr.nudBound", "opr.setAfterEmpty", "opr.afterEmpty",
}

// oprBindingEngineOps returns the operations of the Operators Rule family that the engine binds.
func oprBindingEngineOps() []string {
	names := slices.Clone(oprBindingDescriptorOps)
	for _, o := range Build().Ops {
		if o.Component == ops.Suspended && strings.HasPrefix(o.Name, "opr") {
			names = append(names, o.Name)
		}
	}
	return names
}

// expBindingOps returns the operations of the expectation record and of expBindingMemoOps.
func expBindingOps() []string {
	var names []string
	for _, o := range Build().Ops {
		if o.Component == ops.Exp {
			names = append(names, o.Name)
		}
	}
	return append(names, expBindingMemoOps...)
}

// bindings returns the module of the compositions with a binding function for each operation of
// names, and the bindings in that order. A context state (ops.Ctx) is a value that the operations
// change in place, so a binding returns each one it takes after its results, for the harness to keep.
func bindings(names []string) (*exec.Module, []*exec.Func) {
	m := Build()
	var fs []*exec.Func
	for _, name := range names {
		o := m.Op(name)
		var states []exec.Expr
		results := slices.Clone(o.Results)
		for _, p := range o.Params {
			if p.Type == exec.THandle(ops.Ctx) {
				states = append(states, exec.L(p.Name))
				results = append(results, p.Type)
			}
		}
		f := m.AddFunc(ExpBindingName(name), o.Params, results, o.Features, func(b *exec.Builder) {
			args := make([]exec.Expr, len(o.Params))
			for i, p := range o.Params {
				args[i] = exec.L(p.Name)
			}
			switch len(o.Results) {
			case 0:
				b.Do(name, args...)
				if len(states) > 0 {
					b.Return(states...)
				}
			case 1:
				// A declared result, since a nil result has no type of its own.
				// A result that a parameter does not name.
				res := "r"
				if slices.ContainsFunc(o.Params, func(p exec.Param) bool { return p.Name == res }) {
					res = "res"
				}
				b.Var(res, o.Results[0])
				b.Set(res, exec.C(name, args...))
				b.Return(append([]exec.Expr{exec.L(res)}, states...)...)
			default:
				rs := make([]string, len(o.Results))
				for i := range rs {
					rs[i] = "r" + string(rune('0'+i))
				}
				b.Return(append(b.LetN(rs, exec.C(name, args...)), states...)...)
			}
		})
		f.Doc = ExpBindingName(name) + " runs " + name + " for the conformance suite."
		fs = append(fs, f)
	}
	return m, fs
}

// bindingFile prints the bindings with the target t into a Go file of package pkg.
func bindingFile(pkg string, names []string, t *execgo.Target, table string, imports ...string) (string, error) {
	m, fs := bindings(names)
	name := t.FuncName
	t.FuncName = func(n string) string {
		if strings.HasPrefix(n, "conf") {
			return n
		}
		return name(n)
	}
	var b strings.Builder
	b.WriteString("// Code generated by internal/ir/exec/core; DO NOT EDIT.\n\npackage " + pkg + "\n")
	for _, path := range imports {
		b.WriteString("\nimport " + strconv.Quote(path) + "\n")
	}
	for _, f := range fs {
		src, err := execgo.Func(m, f, t)
		if err != nil {
			return "", err
		}
		b.WriteString("\n" + src)
	}
	if table != "" {
		// The table of the bindings by operation, for a harness that cannot name unexported methods.
		b.WriteString("\n// " + table + " returns the bindings of p by the name of the operation.\nfunc " + table + "(" + t.Recv + ") map[string]any {\n\treturn map[string]any{\n")
		for i, f := range fs {
			b.WriteString("\t\t" + strconv.Quote(names[i]) + ": p." + f.Name + ",\n")
		}
		b.WriteString("\t}\n}\n")
		out, err := format.Source([]byte(b.String()))
		return string(out), err
	}
	return b.String(), nil
}

// EngineExpBindings returns the Go test file of the conformance bindings of the expectation record
// for package core of the engine.
func EngineExpBindings() (string, error) {
	return bindingFile("core", expBindingOps(), engineTarget(), "")
}

// GoExpBindings returns the Go test file of the conformance bindings of the expectation record for
// package runtime, for the node runtime's Parser.
func GoExpBindings() (string, error) {
	return bindingFile("runtime", expBindingOps(), goTarget(goSpec{fn: "expect", name: "Expect"}), "")
}

// EngineDocBindingsFile is the file of the engine's conformance bindings of the doc operations,
// relative to the directory of this package. It is a test file of package core of the engine.
const EngineDocBindingsFile = "../../../engine/core/doc_bind_exec_test.go"

// EngineDocBindings returns the Go test file of the conformance bindings of the doc operations for
// package core of the engine.
func EngineDocBindings() (string, error) {
	return bindingFile("core", docBindingEngineOps, engineTarget(), "")
}

// GoDocBindings returns the Go test file of the conformance bindings of the doc operations for
// package runtime, for the node runtime's Parser.
func GoDocBindings() (string, error) {
	return bindingFile("runtime", docBindingOps, goTarget(goSpec{fn: "expect", name: "Expect"}), "")
}

// EngineOprBindingsFile is the file of the engine's conformance bindings of the Operators Rule
// operations, relative to the directory of this package. It is a test file of package core of the
// engine.
const EngineOprBindingsFile = "../../../engine/core/opr_bind_exec_test.go"

// EngineOprBindings returns the Go test file of the conformance bindings of the Operators Rule
// operations for package core of the engine.
func EngineOprBindings() (string, error) {
	return bindingFile("core", oprBindingEngineOps(), engineTarget(), "confOprBindings", "zeroparse.dev/zp/grammar", "zeroparse.dev/zp/internal/value")
}

// GoOprBindings returns the Go test file of the conformance bindings of the Operators Rule operations
// for package runtime, for the node runtime's Parser.
func GoOprBindings() (string, error) {
	return bindingFile("runtime", oprBindingDescriptorOps, goTarget(goSpec{fn: "expect", name: "Expect"}), "confOprBindings")
}

// ctxBindingOps are the operations of the dependency memo of left recursion that conformance cases
// cover.
var ctxBindingOps = []string{
	"ctx.enter", "ctx.found", "ctx.ok", "ctx.value", "ctx.replay", "ctx.allows", "ctx.active",
	"ctx.lookup", "ctx.push", "ctx.pop", "ctx.storeMemo", "ctx.storeCtx", "memo.setPresent",
	"ctx.keep", "ctx.done",
}

// EngineCtxBindingsFile is the file of the engine's conformance bindings of the dependency memo,
// relative to the directory of this package. It is a test file of package core of the engine.
const EngineCtxBindingsFile = "../../../engine/core/ctx_bind_exec_test.go"

// EngineCtxBindings returns the Go test file of the conformance bindings of the dependency memo for
// package core of the engine.
func EngineCtxBindings() (string, error) {
	return bindingFile("core", ctxBindingOps, engineTarget(), "confCtxBindings", "zeroparse.dev/zp/internal/value")
}

// GoCtxBindings returns the Go test file of the conformance bindings of the dependency memo for
// package runtime, for the node runtime's Parser.
func GoCtxBindings() (string, error) {
	return bindingFile("runtime", ctxBindingOps, goTarget(goSpec{fn: "expect", name: "Expect"}), "confCtxBindings")
}

// frameBindingOps are the frame operations that conformance cases cover.
var frameBindingOps = []string{
	"frame.mark", "frame.markPos", "frame.markRec", "frame.markEnv", "frame.markMissing", "frame.markTrail",
	"frame.undo", "frame.current", "frame.use", "frame.open", "frame.set", "frame.close", "frame.slots",
	"frame.save", "frame.restore",
}

// frameBindingGoOps are the frame operations of the Go runtime, which has no missing-token record.
var frameBindingGoOps = slices.DeleteFunc(slices.Clone(frameBindingOps), func(n string) bool { return n == "frame.markMissing" })

// EngineFrameBindingsFile is the file of the engine's conformance bindings of the capture frames,
// relative to the directory of this package. It is a test file of package core of the engine.
const EngineFrameBindingsFile = "../../../engine/core/frame_bind_exec_test.go"

// EngineFrameBindings returns the Go test file of the conformance bindings of the capture frames for
// package core of the engine.
func EngineFrameBindings() (string, error) {
	return bindingFile("core", frameBindingOps, engineTarget(), "confFrameBindings", "zeroparse.dev/zp/internal/value")
}

// GoFrameBindings returns the Go test file of the conformance bindings of the capture frames for
// package runtime, for the node runtime's Parser.
func GoFrameBindings() (string, error) {
	return bindingFile("runtime", frameBindingGoOps, goTarget(goSpec{fn: "expect", name: "Expect"}), "confFrameBindings")
}

// GoTypedFrameBindings returns the Go test file of the conformance bindings of the capture frames for
// package runtime, for the typed runtime's TParser.
func GoTypedFrameBindings() (string, error) {
	return bindingFile("runtime", frameBindingGoOps, goTarget(goSpec{fn: "reset", typed: true, name: "Reset"}), "confTFrameBindings")
}

// EngineRuleBindingsFile is the file of the engine's conformance bindings of the operations of the
// rules, relative to the directory of this package. It is a test file of package core of the engine.
const EngineRuleBindingsFile = "../../../engine/core/rule_bind_exec_test.go"

// ruleBindingEngineOps are the operations of the rules that the engine provides.
var ruleBindingEngineOps = []string{"rule.memo", "rule.leader", "rule.plain", "rule.docOnlyMemo", "rule.docPlain",
	"rule.positional", "rule.id", "rule.invoke", "rule.invokePlain"}

// ruleBindingOps are the operations of the rules that the node runtime provides.
var ruleBindingOps = []string{"rule.memo", "rule.leader", "rule.plain", "rule.docMemo", "rule.docOnlyMemo",
	"rule.positional", "rule.id", "rule.invoke", "rule.invokePlain"}

// EngineRuleBindings returns the Go test file of the conformance bindings of the operations of the rules
// for package core of the engine.
func EngineRuleBindings() (string, error) {
	return bindingFile("core", ruleBindingEngineOps, engineTarget(), "", "zeroparse.dev/zp/internal/value")
}

// GoRuleBindings returns the Go test file of the conformance bindings of the operations of the rules for
// package runtime, for the node runtime's Parser.
func GoRuleBindings() (string, error) {
	return bindingFile("runtime", ruleBindingOps, goTarget(goSpec{fn: "expect", name: "Expect"}), "")
}

// obsBindingOps are the operations of the observation of a parse.
var obsBindingOps = []string{"obs.active", "obs.memoHit", "obs.enter", "obs.exit"}

// GoObsBindingsFile is the file of the conformance bindings of the observation that the test of the
// runtime prints, relative to the copy of the runtime.
const GoObsBindingsFile = "obs_bind_exec_test.go"

// GoObsBindings returns the Go test file of the conformance bindings of the observation for package
// runtime, for the node runtime's Parser. The engine has no observation of this kind: it traces a call
// on a path of its own.
func GoObsBindings() (string, error) {
	return bindingFile("runtime", obsBindingOps, goTarget(goSpec{fn: "expect", name: "Expect"}), "")
}

// EngineMissingBindingsFile is the file of the engine's conformance bindings of the missing-token
// mode, relative to the directory of this package. It is a test file of package core of the engine.
const EngineMissingBindingsFile = "../../../engine/core/missing_bind_exec_test.go"

// missingBindingOps are the operations of the missing-token mode.
var missingBindingOps = []string{"missing.active", "missing.recompute", "missing.mark", "missing.reset", "missing.replay"}

// EngineMissingBindings returns the Go test file of the conformance bindings of the missing-token mode
// for package core of the engine, the only target that has the mode.
func EngineMissingBindings() (string, error) {
	return bindingFile("core", missingBindingOps, engineTarget(), "")
}
