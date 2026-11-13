package core

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

// The engine's operations of the rules under conformance: every operation runs through its binding in
// rule_bind_exec_test.go, which the engine's printer prints from the operation, so that the suite runs
// the code that the engine's compositions run. The rules of a case are rules of the engine whose bodies
// run the scripts of ref.RuleSpec with the engine's own operations (scriptRules), so that an evaluation
// does to the parser what the model's does to the model.

// ruleMachine binds the operations of the rules of the engine, and the doc operations through
// docMachine.
type ruleMachine struct {
	*docMachine
	specs []ref.RuleSpec
	regs  map[string]any
	evals map[int64]int64
	log   []string
}

// ruleOps is the module of the operations that the scripts of bodies call.
var ruleOps = conform.RuleModule()

func newRuleMachine(cfg ref.Config, controlled bool) conform.Impl {
	em := newEngineMachine(cfg, controlled, false)
	var base *engineMachine
	switch e := em.(type) {
	case *engineMachine:
		base = e
	case meteredEngine:
		base = e.engineMachine
	}
	base.p.MemoAll = cfg.Document
	if cfg.Missing {
		base.p.Missing = &MissingTokenState{Limit: 1 << 30}
	}
	// Making a rule's value reads the program, which these rules have no other use for.
	base.p.Prog = &SharedProgram{staticProgram: &staticProgram{}}
	m := &ruleMachine{docMachine: &docMachine{base}, specs: cfg.RuleSpecs, evals: map[int64]int64{}}
	scriptRules(base.envRules, cfg, m.evaluate)
	for i, r := range base.envRules {
		r.Missing = i < len(cfg.MissingRules) && cfg.MissingRules[i]
	}
	if controlled {
		return meteredRule{m}
	}
	return m
}

type meteredRule struct{ *ruleMachine }

func (m meteredRule) Used() uint64 { return m.p.Control.Used }

// scriptRules gives the rules of cfg.RuleVars the properties of cfg.RuleSpecs, and bodies that run eval.
func scriptRules(rules []*Rule, cfg ref.Config, eval func(index int64, p *Parser, min int) (*value.Node, bool)) {
	for i, r := range rules {
		var s ref.RuleSpec
		if i < len(cfg.RuleSpecs) {
			s = cfg.RuleSpecs[i]
		}
		r.ID, r.Name = i, ref.RuleName(int64(i))
		// The engine marks a rule that some parse memoizes; one that a parse from scratch does not is
		// transient.
		r.Memo, r.Transient = s.Memo || s.DocMemo, !s.Memo
		r.Leader, r.Plain, r.Positional, r.Rec = s.Leader, s.DocPlain, s.Positional, s.Rec
		r.Scope = &compile.Scope{}
		for k := range s.Captures {
			r.Scope.Names = append(r.Scope.Names, ref.CaptureName(k))
		}
		// Every other rule's value is the tuple of the body's one element, as a sequence's is, so that
		// the suite also holds the shaping of a value to the cost of the call.
		r.Value = func(p *Parser, v *value.Node, start int) *value.Node { return v }
		if i%2 == 1 {
			r.Value = func(p *Parser, v *value.Node, start int) *value.Node {
				return ShapeTuple(p, compile.Shaper{K: 1}, v, start)
			}
		}
		index := int64(i)
		r.Body = func(p *Parser, min int) (*value.Node, bool) { return eval(index, p, min) }
	}
}

// evaluate runs the next evaluation of the script of the rule with the index on the engine.
func (m *ruleMachine) evaluate(index int64, p *Parser, level int) (*value.Node, bool) {
	var e ref.Eval
	if s := m.spec(index); len(s.Body) > 0 {
		e = s.Body[min(m.evals[index], int64(len(s.Body)-1))]
	}
	m.evals[index]++
	start := p.Pos
	m.log = append(m.log, fmt.Sprintf("body %s level=%d pos=%d", ref.RuleName(index), level, start))
	if e.Charge > 0 {
		m.engineMachine.Call(ruleOps.Op("budget.charge"), []any{e.Charge})
	}
	for _, id := range e.Errs {
		m.engineMachine.Call(ruleOps.Op("rec.push"), []any{m.Err(id)})
	}
	if p.Missing != nil && len(e.Missing) > 0 {
		// The grammar's code of a missing-token literal: what it charges is not the body's script's.
		c := p.Control
		p.Control = nil
		for _, t := range e.Missing {
			err := m.Err(t.Err).(*parse.SyntaxError)
			if p.InsertMissing(MissingTokenEvent{MissingTokenSite{int(t.Site), int(t.Pos)}, err}) {
				p.PushRecovered(err)
			}
		}
		p.Control = c
	}
	if e.Touch > 0 {
		p.confDocTouch(start + int(e.Touch))
	}
	for _, k := range e.Captures {
		p.Frame.Vals[k] = value.Ptr(value.Data{})
	}
	p.Pos = start + int(e.Len)
	if !e.OK {
		return nil, false
	}
	return value.Ptr(value.Data{Fresh: true}), true
}

func (m *ruleMachine) spec(index int64) ref.RuleSpec {
	if int(index) < len(m.specs) {
		return m.specs[index]
	}
	return ref.RuleSpec{}
}

// UseRegisters hands the machine the registers of the run: the operations of the rules read and move
// the position.
func (m *ruleMachine) UseRegisters(regs map[string]any) { m.regs = regs }

func (m *ruleMachine) Observe(component string) []string {
	switch component {
	case ops.Rules:
		return append([]string{}, m.log...)
	case ops.Rec:
		return (&missingMachine{m.engineMachine}).Observe("recovered")
	}
	return nil
}

func (m *ruleMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupt()
	p := m.p
	rule := func() *Rule { return args[0].(*Rule) }
	invoke := func(f func(r *Rule, min int) (*value.Node, bool)) []any {
		p.Pos = int(m.regs[ops.Pos].(int64))
		defer func() { m.regs[ops.Pos] = int64(p.Pos) }()
		v, ok := f(rule(), int(args[1].(int64)))
		return []any{v, ok}
	}
	switch o.Name {
	case "rule.memo":
		return []any{p.confRuleMemo(rule())}
	case "rule.leader":
		return []any{p.confRuleLeader(rule())}
	case "rule.plain":
		return []any{p.confRulePlain(rule())}
	case "rule.docOnlyMemo":
		return []any{p.confRuleDocOnlyMemo(rule())}
	case "rule.docPlain":
		return []any{p.confRuleDocPlain(rule())}
	case "rule.positional":
		return []any{p.confRulePositional(rule())}
	case "rule.id":
		return []any{int64(p.confRuleId(rule()))}
	case "rule.invoke":
		return invoke(p.confRuleInvoke)
	case "rule.invokePlain":
		return invoke(p.confRuleInvokePlain)
	case "depth.enter":
		p.Enter()
		return nil
	case "depth.exit":
		p.Depth--
		return nil
	case "depth.current":
		return []any{int64(p.Depth)}
	case "missing.active":
		return []any{p.confMissingActive()}
	case "missing.mark":
		return []any{int64(p.confMissingMark())}
	}
	return m.docMachine.Call(o, args)
}

// ruleEngineOps are the operations of the rules that the engine provides, with the doc operations.
var ruleEngineOps = append([]string{"rule.memo", "rule.leader", "rule.plain", "rule.docOnlyMemo", "rule.docPlain",
	"rule.positional", "rule.id", "rule.invoke", "rule.invokePlain", "depth.enter", "depth.exit", "depth.current",
	"rec.mark", "rec.truncate", "missing.active", "missing.mark"}, conform.DocCoreOps...)

// ruleSuites are the suites of the engine's operations of the rules: its Document parse takes the call
// protocol, and its doc.enter starts the range of a call as the empty range at its position. The
// controlled suite checks the charges against the cost bounds.
func ruleSuites() []conform.Suite {
	s := conform.Suite{Name: "engine rules", Ops: ruleEngineOps, DocCallMemo: true, DocEmptyEnter: true,
		New: func(c ref.Config) conform.Impl { return newRuleMachine(c, false) }}
	controlled := s
	controlled.Name = "engine rules, controlled"
	controlled.Exact = ruleExact
	controlled.Ops = append(slices.Clone(ruleEngineOps), engineBudgetOps...)
	controlled.New = func(c ref.Config) conform.Impl { return newRuleMachine(c, true) }
	return []conform.Suite{s, controlled}
}

// ruleConformanceFailures runs the handwritten cases of the rules and seeds randomized cases of n calls
// on s, and returns the failures and the number of cases that ran.
func ruleConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.RuleCases() {
		run(c)
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 67))
		run(conform.RandomRule(fmt.Sprint("random_", seed), rng, s.Ops, n, seed%3 == 2))
	}
	return fails, ran
}

// TestEngineRuleCasesRun checks that every handwritten case of the rules runs on the controlled engine:
// those of the missing-token mode and of the budget too, but those that read rule.docMemo, which the
// engine does not have.
func TestEngineRuleCasesRun(t *testing.T) {
	s := ruleSuites()[1]
	for _, c := range conform.RuleCases() {
		if strings.HasPrefix(c.Func.Name, "rule_flags_document_") {
			continue
		}
		if skipped, err := conform.Run(s, c); skipped || err != nil {
			t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
		}
	}
}

func TestEngineRuleConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, s := range ruleSuites() {
		fails, ran := ruleConformanceFailures(s, seeds, 60)
		if ran < seeds {
			t.Fatalf("%s: only %d cases ran", s.Name, ran)
		}
		for _, err := range fails {
			t.Error(err)
		}
	}
}

// TestEngineRuleConformanceDetectsBrokenEngine is the negative control: the engine with one operation of
// the rules broken, as a wrong line of its printed code would break it, fails the suite.
func TestEngineRuleConformanceDetectsBrokenEngine(t *testing.T) {
	variants := map[string]func(m *ruleMachine, args []any) []any{
		"rule.memo ignores the Document": func(m *ruleMachine, args []any) []any {
			r := args[0].(*Rule)
			return []any{r.Memo && !r.Transient}
		},
		"rule.plain is true": func(m *ruleMachine, args []any) []any { return []any{true} },
		"rule.docOnlyMemo counts leaders": func(m *ruleMachine, args []any) []any {
			r := args[0].(*Rule)
			return []any{m.p.MemoAll && r.Memo && r.Transient}
		},
		"rule.id is the position of the rule after it": func(m *ruleMachine, args []any) []any {
			return []any{int64(args[0].(*Rule).ID + 1)}
		},
		"rule.invoke runs the body without its own frame": func(m *ruleMachine, args []any) []any {
			p := m.p
			p.Pos = int(m.regs[ops.Pos].(int64))
			v, ok := p.InvokePlain(args[0].(*Rule), int(args[1].(int64)))
			m.regs[ops.Pos] = int64(p.Pos)
			return []any{v, ok}
		},
		"rule.memo memoizes in the missing-token mode": func(m *ruleMachine, args []any) []any {
			r := args[0].(*Rule)
			return []any{r.Memo && (!r.Transient || m.p.MemoAll)}
		},
		"rule.docOnlyMemo counts the rules that insert tokens": func(m *ruleMachine, args []any) []any {
			r := args[0].(*Rule)
			return []any{m.p.MemoAll && r.Memo && r.Transient && !r.Leader}
		},
		"rule.invokePlain leaves the tokens of a failure": func(m *ruleMachine, args []any) []any {
			p := m.p
			start, rec := int(m.regs[ops.Pos].(int64)), len(p.Recovered)
			p.Pos = start
			v, ok := p.invoke(args[0].(*Rule), int(args[1].(int64)))
			if !ok {
				p.Pos, p.Recovered = start, p.Recovered[:rec]
			}
			m.regs[ops.Pos] = int64(p.Pos)
			return []any{v, ok}
		},
		"rule.invokePlain keeps the errors of a failure": func(m *ruleMachine, args []any) []any {
			p := m.p
			start := int(m.regs[ops.Pos].(int64))
			p.Pos = start
			v, ok := p.invoke(args[0].(*Rule), int(args[1].(int64)))
			if !ok {
				p.Pos = start
			}
			m.regs[ops.Pos] = int64(p.Pos)
			return []any{v, ok}
		},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := ruleSuites()[0]
			s.Name = name
			s.New = func(c ref.Config) conform.Impl { return brokenRule{newRuleMachine(c, false).(*ruleMachine), op, v} }
			if fails, _ := ruleConformanceFailures(s, 100, 60); len(fails) == 0 {
				t.Fatal("no case detected the broken engine")
			}
		})
	}
}

// ruleExact are the operations of the rules whose charges the controlled suite holds to their exact
// cost, with the captures of the cases matched and attached as fields.
var ruleExact = []string{"rule.invoke", "rule.invokePlain"}

// TestEngineRuleChargesArePinned is the negative control of the charges: the engine whose rule.invoke
// charges one more unit for each field it attaches, or whose rule.invokePlain charges one more unit,
// fails the controlled suite.
func TestEngineRuleChargesArePinned(t *testing.T) {
	invoke := func(m *ruleMachine, args []any, f func(r *Rule, min int) (*value.Node, bool)) (*value.Node, bool) {
		p := m.p
		p.Pos = int(m.regs[ops.Pos].(int64))
		defer func() { m.regs[ops.Pos] = int64(p.Pos) }()
		return f(args[0].(*Rule), int(args[1].(int64)))
	}
	variants := map[string]func(m *ruleMachine, args []any) []any{
		"rule.invoke charges one more for each field it attaches": func(m *ruleMachine, args []any) []any {
			v, ok := invoke(m, args, m.p.confRuleInvoke)
			if ok && v != nil {
				for range value.D(v).Fields {
					m.p.Spend()
				}
			}
			return []any{v, ok}
		},
		// The tokens of a failure go without a charge: invoke, which leaves them, and an uncharged reset.
		"rule.invokePlain charges nothing for the tokens it removes": func(m *ruleMachine, args []any) []any {
			p := m.p
			mark, rec := p.missingMark(), len(p.Recovered)
			start := int(m.regs[ops.Pos].(int64))
			v, ok := invoke(m, args, p.invoke)
			if !ok {
				p.Pos, p.Recovered = start, p.Recovered[:rec]
				m.regs[ops.Pos] = int64(start)
				c := p.Control
				p.Control = nil
				p.ResetMissing(mark)
				p.Control = c
			}
			return []any{v, ok}
		},
		"rule.invokePlain charges one more": func(m *ruleMachine, args []any) []any {
			v, ok := invoke(m, args, m.p.confRuleInvokePlain)
			m.p.Spend()
			return []any{v, ok}
		},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := ruleSuites()[1]
			s.Name = name
			s.New = func(c ref.Config) conform.Impl {
				return meteredBrokenRule{brokenRule{newRuleMachine(c, true).(meteredRule).ruleMachine, op, v}}
			}
			if fails, _ := ruleConformanceFailures(s, 100, 60); len(fails) == 0 {
				t.Fatal("no case detected the changed charge")
			}
		})
	}
}

type meteredBrokenRule struct{ brokenRule }

func (m meteredBrokenRule) Used() uint64 { return m.p.Control.Used }

// brokenRule is the engine with one operation of the rules replaced.
type brokenRule struct {
	*ruleMachine
	op   string
	call func(m *ruleMachine, args []any) []any
}

func (b brokenRule) Call(o *exec.Op, args []any) (res []any) {
	if o.Name == b.op {
		defer interrupt()
		return b.call(b.ruleMachine, args)
	}
	return b.ruleMachine.Call(o, args)
}
