package conform

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// Observer is an Impl that provides a component whose state the model exposes for comparison
// (ref.Model.Observe). After each call of an operation that writes the component, the harness compares
// what the implementation reports with the model's.
type Observer interface {
	// Observe returns the state of the component as the model's Observe lists it, for a component
	// that the implementation provides, and nil for any other.
	Observe(component string) []string
}

// Bodies is an Impl that provides the invocations of rule bodies that generated calls pass (ops.Body).
type Bodies interface {
	// Body returns the implementation's invocation of the body of the rule with the index in
	// ref.Config.RuleVars.
	Body(index int64) any
}

// BodyOp is the operation that conformance cases use to make the invocation of a rule's body
// (Bodies.Body). It is not an operation of the parser VM.
const BodyOp = "test.body"

// RuleOps are the operations of the rules that a parse from scratch and a Document parse read.
var RuleOps = []string{
	"rule.memo", "rule.leader", "rule.plain", "rule.docMemo", "rule.docOnlyMemo", "rule.docPlain",
	"rule.positional", "rule.id", "rule.invoke", "rule.invokePlain",
}

// RuleCaseOps are the operations besides those of the rules that the cases of the rules call: the
// nesting depth and the recovered errors, which an evaluation of a body changes.
var RuleCaseOps = []string{"depth.enter", "depth.exit", "depth.current", "rec.mark", "rec.truncate"}

// RuleMissingOps are the operations of the missing-token mode that the cases of the rules in the mode
// call, so that they run only on a target that has the mode.
var RuleMissingOps = []string{"missing.active", "missing.mark"}

// RuleRules is the number of rules that the cases of the rules name (ref.Config.RuleVars).
const RuleRules = 4

// RuleModule returns the declarations of DocModule, which hold the operations of the rules, with
// BodyOp.
func RuleModule() *exec.Module {
	m := DocModule()
	m.AddOp(&exec.Op{Name: BodyOp, Component: ops.Rules, Params: []exec.Param{exec.P("index", exec.TInt)},
		Results:  []exec.Type{exec.THandle(ops.Body)},
		Contract: "Returns the invocation of the body of a rule of the case's configuration."})
	return m
}

// RuleConfig returns the configuration of a parse of the cases of the rules: the rules of specs, one
// variable that none reads, and a nesting limit of 8.
func RuleConfig(specs ...ref.RuleSpec) ref.Config {
	return ref.Config{Vars: []int64{0}, RuleVars: make([][]int64, len(specs)), RuleSpecs: specs, MaxDepth: 8}
}

var ruleFeatures = []string{"budget", "stream", "document", "observe", "missing"}

// RuleCases returns the handwritten conformance cases of the rules, in one module. As with Cases, the
// model checks each case first.
func RuleCases() []Case {
	m := RuleModule()
	var cs []Case
	add := func(name string, cfg ref.Config, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, ruleFeatures, body), Config: cfg})
	}
	rule := func(i int64) exec.Expr { return exec.C(RuleOp, exec.Int(i)) }
	i, pos := exec.Int, exec.R(ops.Pos)
	ok := func(len int64) ref.Eval { return ref.Eval{OK: true, Len: len} }

	flags := []ref.RuleSpec{
		{Memo: true, DocMemo: true, Positional: true, Captures: 2},
		{DocMemo: true, DocPlain: true},
		{Leader: true, DocMemo: true, Rec: true, Captures: 1},
		{},
	}
	for _, doc := range []bool{false, true} {
		cfg := RuleConfig(flags...)
		cfg.Document = doc
		add(fmt.Sprint("rule_flags_document_", doc), cfg, func(b *exec.Builder) {
			for k := range int64(RuleRules) {
				for _, o := range RuleOps[:8] {
					b.Let(fmt.Sprintf("%s_%d", o[5:], k), exec.C(o, rule(k)))
				}
			}
			b.Assert(exec.Eq(exec.C("rule.id", rule(2)), i(2)), "a rule's index is its identifier")
		})
	}

	// In the missing-token mode, no rule that can insert a token is memoized, in a parse from scratch or
	// in a Document parse.
	for _, doc := range []bool{false, true} {
		cfg := RuleConfig(flags...)
		cfg.Document, cfg.Missing, cfg.MissingRules = doc, true, []bool{true, true, false, false}
		add(fmt.Sprint("rule_flags_missing_document_", doc), cfg, func(b *exec.Builder) {
			b.Assert(exec.C("missing.active"), "the parse is in the missing-token mode")
			for k := range int64(RuleRules) {
				for _, o := range []string{"rule.memo", "rule.docOnlyMemo"} {
					b.Let(fmt.Sprintf("%s_%d", o[5:], k), exec.C(o, rule(k)))
				}
			}
		})
	}

	add("rule_invoke", RuleConfig(ref.RuleSpec{Captures: 1, Body: []ref.Eval{ok(3), ok(0), {Len: 2}}}), func(b *exec.Builder) {
		b.LetN([]string{"v1", "ok1"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.And(exec.L("ok1"), exec.Eq(pos, i(3))), "a match moves the position to its end")
		b.LetN([]string{"v2", "ok2"}, exec.C("rule.invoke", rule(0), i(2)))
		b.Assert(exec.And(exec.L("ok2"), exec.Eq(pos, i(3))), "an empty match leaves the position")
		b.LetN([]string{"v3", "ok3"}, exec.C("rule.invoke", rule(0), i(1)))
		b.Assert(exec.Not(exec.L("ok3")), "the body's third evaluation fails")
		b.SetReg(ops.Pos, i(7))
		b.LetN([]string{"v4", "ok4"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.Not(exec.L("ok4")), "every later evaluation does as the last one of the script")
	})

	// The captures that a match sets become fields of its value: rule.invoke charges the search for them
	// and the walk of the fields before each one, and nothing for a failure or a match that set none.
	add("rule_invoke_captures", RuleConfig(ref.RuleSpec{Captures: 3, Body: []ref.Eval{
		{OK: true, Len: 1, Captures: []int64{0, 2}}, {OK: true, Captures: []int64{2, 1, 0}}, {OK: true, Len: 2},
		{Captures: []int64{1}},
	}}), func(b *exec.Builder) {
		for k := range 3 {
			b.LetN([]string{fmt.Sprint("v", k), fmt.Sprint("ok", k)}, exec.C("rule.invoke", rule(0), i(0)))
		}
		b.Assert(exec.Eq(pos, i(3)), "the captures leave the position to the body")
		b.LetN([]string{"v3", "ok3"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.Not(exec.L("ok3")), "a failure attaches no capture")
	})

	add("rule_invoke_errors", RuleConfig(ref.RuleSpec{Body: []ref.Eval{{OK: true, Len: 2, Errs: []int64{4, 5}}, {Errs: []int64{6}}}}),
		func(b *exec.Builder) {
			b.LetN([]string{"v1", "ok1"}, exec.C("rule.invoke", rule(0), i(0)))
			b.Assert(exec.Eq(exec.C("rec.mark"), i(2)), "the errors that the body recovered stay")
			b.LetN([]string{"v2", "ok2"}, exec.C("rule.invoke", rule(0), i(0)))
			b.Assert(exec.Eq(exec.C("rec.mark"), i(3)), "a failed invocation leaves the errors to its caller")
		})

	add("rule_invoke_plain", RuleConfig(ref.RuleSpec{Body: []ref.Eval{
		{OK: true, Len: 4, Errs: []int64{1}}, {Len: 3, Errs: []int64{2, 3}},
	}}), func(b *exec.Builder) {
		b.SetReg(ops.Pos, i(1))
		b.LetN([]string{"v1", "ok1"}, exec.C("rule.invokePlain", rule(0), i(1)))
		b.Assert(exec.And(exec.L("ok1"), exec.Eq(pos, i(5))), "a plain match moves the position to its end")
		b.LetN([]string{"v2", "ok2"}, exec.C("rule.invokePlain", rule(0), i(0)))
		b.Assert(exec.And(exec.Not(exec.L("ok2")), exec.Eq(pos, i(5))), "a plain failure is at its start")
		b.Assert(exec.Eq(exec.C("rec.mark"), i(1)), "a plain failure drops the errors it recovered")
	})

	// A failed plain body leaves no token it inserted; a failed body of rule.invoke leaves them to its
	// caller.
	missingCfg := RuleConfig(ref.RuleSpec{Body: []ref.Eval{
		{Missing: []ref.MissingInsert{{Site: 1, Pos: 2, Err: 7}, {Site: 2, Pos: 2, Err: 8}, {Site: 1, Pos: 2, Err: 9}}},
		{OK: true, Len: 1, Missing: []ref.MissingInsert{{Site: 1, Pos: 2, Err: 10}}},
		{Missing: []ref.MissingInsert{{Site: 1, Pos: 2, Err: 11}, {Site: 3, Pos: 4, Err: 12}}},
	}}, ref.RuleSpec{Captures: 1, Body: []ref.Eval{{Missing: []ref.MissingInsert{{Site: 5, Pos: 0, Err: 13}}}}})
	missingCfg.Missing, missingCfg.MissingRules = true, []bool{true, true}
	add("rule_invoke_plain_missing", missingCfg, func(b *exec.Builder) {
		b.LetN([]string{"v1", "ok1"}, exec.C("rule.invokePlain", rule(0), i(0)))
		b.Assert(exec.And(exec.Eq(exec.C("missing.mark"), i(0)), exec.Eq(exec.C("rec.mark"), i(0))),
			"a failed plain body leaves neither its tokens nor their errors")
		b.LetN([]string{"v2", "ok2"}, exec.C("rule.invokePlain", rule(0), i(0)))
		b.Assert(exec.And(exec.L("ok2"), exec.Eq(exec.C("missing.mark"), i(1))), "a plain match keeps its token")
		b.LetN([]string{"v3", "ok3"}, exec.C("rule.invokePlain", rule(0), i(0)))
		b.Assert(exec.Eq(exec.C("missing.mark"), i(1)), "a failure removes only the tokens it inserted")
		b.LetN([]string{"v4", "ok4"}, exec.C("rule.invoke", rule(1), i(0)))
		b.Assert(exec.Eq(exec.C("missing.mark"), i(2)), "a failed body of rule.invoke leaves its token")
	})

	nesting := RuleConfig(ref.RuleSpec{Rec: true, Body: []ref.Eval{ok(1)}}, ref.RuleSpec{Body: []ref.Eval{ok(1)}})
	nesting.MaxDepth = 1
	add("rule_invoke_nesting", nesting, func(b *exec.Builder) {
		b.Do("depth.enter")
		b.LetN([]string{"v1", "ok1"}, exec.C("rule.invoke", rule(1), i(0)))
		b.Assert(exec.Eq(exec.C("depth.current"), i(1)), "a rule on no cycle takes no nesting level")
		b.Do("depth.exit")
		b.LetN([]string{"v2", "ok2"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.Eq(exec.C("depth.current"), i(0)), "a rule on a cycle gives its nesting level back")
		b.Do("depth.enter")
		b.LetN([]string{"v3", "ok3"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.BoolC(false), "the nesting limit ends the parse")
	})

	limited := RuleConfig(ref.RuleSpec{Body: []ref.Eval{{OK: true, Len: 1, Charge: 3}}})
	limited.Limit = 10
	add("rule_invoke_budget", limited, func(b *exec.Builder) {
		for k := range 4 {
			b.LetN([]string{fmt.Sprint("v", k), fmt.Sprint("ok", k)}, exec.C("rule.invoke", rule(0), i(0)))
		}
		// In a metered implementation, the body's charges exceed the limit by the fourth call.
	})

	// The budget and the nesting limit both stop the call: it stops on the budget, whose unit the call
	// is charged before it takes the nesting level.
	both := RuleConfig(ref.RuleSpec{Rec: true, Body: []ref.Eval{ok(1)}})
	both.MaxDepth, both.Limit = 1, 5
	add("rule_invoke_budget_and_nesting", both, func(b *exec.Builder) {
		b.Do("budget.charge", exec.U64(5))
		b.Do("depth.enter")
		b.LetN([]string{"v1", "ok1"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.BoolC(false), "the limits end the parse")
	})
	add("rule_invoke_plain_budget_and_nesting", both, func(b *exec.Builder) {
		b.Do("budget.charge", exec.U64(5))
		b.Do("depth.enter")
		b.LetN([]string{"v1", "ok1"}, exec.C("rule.invokePlain", rule(0), i(0)))
		b.Assert(exec.BoolC(false), "the limits end the parse")
	})

	docCfg := RuleConfig(ref.RuleSpec{Body: []ref.Eval{{OK: true, Len: 2, Touch: 5}, {Len: 1, Touch: 1}}},
		ref.RuleSpec{Body: []ref.Eval{{Touch: 3}}})
	docCfg.Document = true
	add("rule_invoke_document", docCfg, func(b *exec.Builder) {
		b.SetReg(ops.Pos, i(2))
		b.LetN([]string{"h", "l"}, exec.C("doc.enter", pos))
		b.LetN([]string{"v1", "ok1"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.Eq(exec.C("doc.examined"), i(7)), "the body extends the examined input")
		b.LetN([]string{"v2", "ok2"}, exec.C("rule.invoke", rule(0), i(0)))
		b.Assert(exec.Eq(exec.C("doc.examined"), i(7)), "a shorter reach leaves the examined input")
		b.SetReg(ops.Pos, i(6))
		b.LetN([]string{"v3", "ok3"}, exec.C("rule.invokePlain", rule(1), i(0)))
		b.Assert(exec.Eq(exec.C("doc.examined"), i(9)), "a failed plain body extends the examined input")
		b.Assert(exec.Eq(exec.C(DocStatOp, i(1)), i(3)), "every evaluation counts in the Document")
		b.Do("doc.leave", exec.L("h"), exec.L("l"))
	})
	return cs
}

// RandomRuleConfig returns the configuration of a randomized case of the rules, drawn with rng:
// RuleRules rules whose properties hold together as a grammar's do (a rule that a parse from scratch
// memoizes, or a leader, a Document memoizes too), with scripts of one to three evaluations.
func RandomRuleConfig(rng *rand.Rand, document bool) ref.Config {
	specs := make([]ref.RuleSpec, RuleRules)
	for k := range specs {
		s := ref.RuleSpec{Memo: rng.IntN(2) == 0, Leader: rng.IntN(4) == 0, Captures: max(rng.Int64N(5)-2, 0),
			DocPlain: rng.IntN(3) == 0, Positional: rng.IntN(3) == 0, Rec: rng.IntN(2) == 0}
		s.DocMemo = s.Memo || s.Leader || rng.IntN(2) == 0
		for range 1 + rng.IntN(3) {
			e := ref.Eval{OK: rng.IntN(3) > 0, Len: rng.Int64N(4), Charge: uint64(rng.IntN(3)), Touch: rng.Int64N(6)}
			for range rng.IntN(3) {
				e.Errs = append(e.Errs, rng.Int64N(50))
			}
			for k := range s.Captures {
				if rng.IntN(2) == 0 {
					e.Captures = append(e.Captures, k)
				}
			}
			s.Body = append(s.Body, e)
		}
		specs[k] = s
	}
	cfg := RuleConfig(specs...)
	cfg.Document = document
	cfg.MaxDepth = 1 + rng.Int64N(3)
	if rng.IntN(4) == 0 {
		cfg.Limit = uint64(10 + rng.IntN(40))
	}
	return cfg
}

// RandomRule returns a randomized case of n calls of the operations of the rules, depth.enter, depth.exit
// and rec.mark that provided lists, drawn with rng, that meets every precondition, in the parse of RandomRuleConfig. In a
// Document parse the calls run inside a rule call that doc.enter begins, and the case ends by reading
// the statistics of the Document. A case ends at the call that ends the parse.
func RandomRule(name string, rng *rand.Rand, provided []string, n int, document bool) Case {
	m := RuleModule()
	cfg := RandomRuleConfig(rng, document)
	// A target with the missing-token mode runs one parse in three in the mode, whose bodies insert
	// tokens at a few sites and positions, so that they repeat.
	missing := slices.Contains(provided, "missing.mark") && rng.IntN(3) == 0
	if missing {
		cfg.Missing = true
		for k := range cfg.RuleSpecs {
			cfg.MissingRules = append(cfg.MissingRules, rng.IntN(2) == 0)
			for j := range cfg.RuleSpecs[k].Body {
				for range rng.IntN(3) {
					cfg.RuleSpecs[k].Body[j].Missing = append(cfg.RuleSpecs[k].Body[j].Missing,
						ref.MissingInsert{Site: rng.Int64N(3), Pos: rng.Int64N(4), Err: 100 + rng.Int64N(50)})
				}
			}
		}
	}
	mirror := ref.New(cfg)
	regs := map[string]any{ops.Pos: int64(0), ops.Silent: false}
	mirror.UseRegisters(regs)
	var choices []string
	for _, o := range append(append([]string{}, RuleOps...), "rule.invoke", "rule.invoke", "rule.invokePlain", "depth.enter",
		"depth.exit", "rec.mark", "rec.mark", "missing.mark") {
		if slices.Contains(provided, o) {
			choices = append(choices, o)
		}
	}
	choices = append(choices, "pos")
	f := m.AddFunc(name, nil, nil, ruleFeatures, func(b *exec.Builder) {
		if document {
			b.LetN([]string{"h", "l"}, exec.C("doc.enter", exec.Int(0)))
			mirror.Call(m.Op("doc.enter"), []any{int64(0)})
		}
		if missing {
			b.Assert(exec.C("missing.active"), "the parse is in the missing-token mode")
		}
		for calls := 0; calls < n; {
			name := choices[rng.IntN(len(choices))]
			if name == "pos" {
				p := rng.Int64N(10)
				regs[ops.Pos] = p
				b.SetReg(ops.Pos, exec.Int(p))
				continue
			}
			o := m.Op(name)
			var args []any
			var exprs []exec.Expr
			if len(o.Params) > 0 {
				k := rng.Int64N(RuleRules)
				args, exprs = append(args, ref.RuleRef{Index: k}), append(exprs, exec.C(RuleOp, exec.Int(k)))
			}
			if len(o.Params) > 1 {
				level := rng.Int64N(3)
				args, exprs = append(args, level), append(exprs, exec.Int(level))
			}
			if !mirror.Allows(o, args) {
				continue
			}
			calls++
			if len(o.Results) > 0 {
				b.LetN(resultNames(calls, len(o.Results)), exec.C(o.Name, exprs...))
			} else {
				b.Do(o.Name, exprs...)
			}
			if interrupts(mirror, o, args) {
				return
			}
		}
		if document {
			b.Let("reused", exec.C(DocStatOp, exec.Int(0)))
			b.Let("evaluated", exec.C(DocStatOp, exec.Int(1)))
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}

// resultNames returns the names of the n results of the call with the number.
func resultNames(call, n int) []string {
	names := make([]string, n)
	for k := range names {
		names[k] = fmt.Sprintf("x%d_%d", call, k)
	}
	return names
}

// interrupts runs the call on the mirror and reports whether it ended the parse. The mirror charges no
// units for the bodies it evaluates (the suite adds an implementation's charges to the model's), so
// that a case may go on past the call where the budget runs out; the run ends there.
func interrupts(mirror *ref.Model, o *exec.Op, args []any) (ended bool) {
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(exec.Interrupt); !ok {
				panic(x)
			}
			ended = true
		}
	}()
	mirror.Call(o, args)
	return false
}
