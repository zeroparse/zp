package libgo

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The operations of the rules of the node library under conformance: every operation runs through its
// binding in rule_bind_exec_test.go, which the test that copies the library prints with the library's
// printer, so that the suite runs the code that the library's compositions run. The rules of a case are
// rules of the library whose bodies run the scripts of ref.RuleSpec with the library's own operations
// (scriptRules), so that an evaluation does to the parser what the model's does to the model.

// ruleMachine binds the operations of the rules of the library, and the doc operations through
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
	var base *rtMachine
	switch r := newRTMachine(cfg, controlled, false, false).(type) {
	case *rtMachine:
		base = r
	case meteredRT:
		base = r.rtMachine
	}
	if cfg.Document {
		base.p.Document = &documentState{}
	}
	m := &ruleMachine{docMachine: &docMachine{rtMachine: base}, specs: cfg.RuleSpecs, evals: map[int64]int64{}}
	scriptRules(base.env.rules, cfg, m.evaluate)
	if controlled {
		return meteredRule{m}
	}
	return m
}

type meteredRule struct{ *ruleMachine }

func (m meteredRule) Used() uint64 { return m.p.Control.budgetUsed() }

// scriptRules gives the rules of cfg.RuleVars the properties of cfg.RuleSpecs, and bodies that run eval.
func scriptRules(rules []*Rule, cfg ref.Config, eval func(index int64, p *Parser, min int) (*core.Node, bool)) {
	for i, r := range rules {
		var s ref.RuleSpec
		if i < len(cfg.RuleSpecs) {
			s = cfg.RuleSpecs[i]
		}
		r.Id, r.Name = i, ref.RuleName(int64(i))
		r.Memo, r.DocMemo, r.Leader, r.Positional, r.Rec = s.Memo, s.DocMemo, s.Leader, s.Positional, s.Rec
		r.Scope = nil
		for k := range s.Captures {
			r.Scope = append(r.Scope, ref.CaptureName(k))
		}
		// Every other rule's value is the tuple of the body's one element, as a sequence's is, so that
		// the suite also holds the shaping of a value to the cost of the call.
		r.Value = Shaper{}
		if i%2 == 1 {
			r.Value = Shaper{K: 1, Tuple: true}
		}
		index := int64(i)
		r.Body = func(p *Parser, min int) (*core.Node, bool) { return eval(index, p, min) }
	}
}

// evaluate runs the next evaluation of the script of the rule with the index on the library.
func (m *ruleMachine) evaluate(index int64, p *Parser, level int) (*core.Node, bool) {
	var e ref.Eval
	if s := m.spec(index); len(s.Body) > 0 {
		e = s.Body[min(m.evals[index], int64(len(s.Body)-1))]
	}
	m.evals[index]++
	start := p.Pos
	m.log = append(m.log, fmt.Sprintf("body %s level=%d pos=%d", ref.RuleName(index), level, start))
	if e.Charge > 0 {
		m.rtMachine.Call(ruleOps.Op("budget.charge"), []any{e.Charge})
	}
	for _, id := range e.Errs {
		m.rtMachine.Call(ruleOps.Op("rec.push"), []any{m.Err(id)})
	}
	if e.Touch > 0 {
		p.confDocTouch(start + int(e.Touch))
	}
	for _, k := range e.Captures {
		p.Frame.Vals[k] = nodeOfData(&Data{})
	}
	p.Pos = start + int(e.Len)
	if !e.OK {
		return nil, false
	}
	return nodeOfData(&Data{Fresh: true}), true
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
	if component != ops.Rules {
		return nil
	}
	return append([]string{}, m.log...)
}

func (m *ruleMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupted()
	p := m.p
	rule := func() *Rule { return args[0].(*Rule) }
	invoke := func(f func(r *Rule, min int) (*core.Node, bool)) []any {
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
	case "rule.docMemo":
		return []any{p.confRuleDocMemo(rule())}
	case "rule.docOnlyMemo":
		return []any{p.confRuleDocOnlyMemo(rule())}
	case "rule.positional":
		return []any{p.confRulePositional(rule())}
	case "rule.id":
		return []any{int64(p.confRuleId(rule()))}
	case "rule.invoke":
		return invoke(p.confRuleInvoke)
	case "rule.invokePlain":
		return invoke(p.confRuleInvokePlain)
	case "depth.enter":
		p.Depth++
		if p.Depth > p.MaxDepth {
			p.TooDeep()
		}
		return nil
	case "depth.exit":
		p.Depth--
		return nil
	case "depth.current":
		return []any{int64(p.Depth)}
	}
	return m.docMachine.Call(o, args)
}

// ruleLibOps are the operations of the rules that the library provides, with the doc operations.
var ruleLibOps = append([]string{"rule.memo", "rule.leader", "rule.plain", "rule.docMemo", "rule.docOnlyMemo",
	"rule.positional", "rule.id", "rule.invoke", "rule.invokePlain", "depth.enter", "depth.exit", "depth.current",
	"rec.mark", "rec.truncate"}, conform.DocOps...)

// ruleSuites are the suites of the library's operations of the rules. The controlled suite checks the
// charges against the cost bounds, but not to the unit: the library's call charges the unit of a call
// before it invokes the rule, so that rule.invoke and rule.invokePlain charge one unit less than their
// cost, and a call that reaches them has paid its unit (conform.Suite.CallUnitBefore).
func ruleSuites() []conform.Suite {
	s := conform.Suite{Name: "node library rules", Ops: ruleLibOps, CallUnitBefore: true,
		New: func(c ref.Config) conform.Impl { return newRuleMachine(c, false) }}
	controlled := s
	controlled.Name = "node library rules, controlled"
	controlled.Ops = append(slices.Clone(ruleLibOps), rtBudgetOps...)
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
		rng := rand.New(rand.NewPCG(uint64(seed), 68))
		run(conform.RandomRule(fmt.Sprint("random_", seed), rng, s.Ops, n, seed%3 == 2))
	}
	return fails, ran
}

func TestExecConformanceRule(t *testing.T) {
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

// TestExecConformanceRuleDetectsBrokenLib is the negative control: the library with one operation of
// the rules broken, as a wrong line of its printed code would break it, fails the suite.
func TestExecConformanceRuleDetectsBrokenLib(t *testing.T) {
	variants := map[string]func(m *ruleMachine, args []any) []any{
		"rule.memo reports the Document's": func(m *ruleMachine, args []any) []any { return []any{args[0].(*Rule).DocMemo} },
		"rule.plain is true":               func(m *ruleMachine, args []any) []any { return []any{true} },
		"rule.docOnlyMemo counts leaders": func(m *ruleMachine, args []any) []any {
			r := args[0].(*Rule)
			return []any{r.DocMemo && !r.Memo}
		},
		"rule.id is the index of the rule after it": func(m *ruleMachine, args []any) []any {
			return []any{int64(args[0].(*Rule).Id + 1)}
		},
		"rule.invoke runs the body without its own frame": func(m *ruleMachine, args []any) []any {
			p := m.p
			p.Pos = int(m.regs[ops.Pos].(int64))
			v, ok := p.invokePlainBody(args[0].(*Rule), int(args[1].(int64)))
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
				t.Fatal("no case detected the broken library")
			}
		})
	}
}

// brokenRule is the library with one operation of the rules replaced.
type brokenRule struct {
	*ruleMachine
	op   string
	call func(m *ruleMachine, args []any) []any
}

func (b brokenRule) Call(o *exec.Op, args []any) (res []any) {
	if o.Name == b.op {
		defer interrupted()
		return b.call(b.ruleMachine, args)
	}
	return b.ruleMachine.Call(o, args)
}
