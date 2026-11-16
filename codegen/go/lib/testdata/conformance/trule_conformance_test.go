package libgo

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The operations of the rules of the typed library under conformance: every operation runs through its
// binding in trule_bind_exec_test.go, which the test that copies the library prints with the library's
// typed printer, so that the suite runs the code that the typed compositions run. The rules of a case
// are typed rules over the node rules of the environment, whose typed bodies run the scripts of
// ref.RuleSpec with the library's own operations, as the node rule machine's do (ruleMachine). The
// doc operations, the recovered errors and the budget are the node Parser's, which the TParser embeds.

// typedRuleMachine binds the operations of the rules of the typed library, and the doc operations
// through docMachine.
type typedRuleMachine struct {
	*docMachine
	tp     *TParser
	trules []*TRule
	// invs are the invocations of the rules by index, one handle for each rule, as the model's.
	invs  map[int64]*typedInvocation
	specs []ref.RuleSpec
	evals map[int64]int64
	log   []string
}

func newTypedRuleMachine(cfg ref.Config, controlled bool) conform.Impl {
	var base *rtMachine
	switch r := newRTMachine(cfg, controlled, false, false).(type) {
	case *rtMachine:
		base = r
	case meteredRT:
		base = r.rtMachine
	}
	tp := &TParser{Parser: base.p}
	if cfg.Document {
		// A typed Document parse sets the node Document too, which keeps the statistics.
		base.p.Document = &documentState{}
		tp.TypedDocument = &typedDocumentState{meta: map[*tmemoEntry]typedDocumentMeta{}}
	}
	if cfg.Stream {
		// The cases of a stream parse read only seen.defers and the rules' descriptors.
		base.p.Stream = &streamInput{}
	}
	base.p.track = cfg.Document || cfg.Observed
	base.p.setDeferMemo()
	m := &typedRuleMachine{docMachine: &docMachine{rtMachine: base}, tp: tp, invs: map[int64]*typedInvocation{}, specs: cfg.RuleSpecs,
		evals: map[int64]int64{}}
	scriptRules(base.env.rules, cfg, func(int64, *Parser, int) (*core.Node, bool) {
		panic("the typed library runs the typed body of a rule")
	})
	for i, r := range base.env.rules {
		index := int64(i)
		m.trules = append(m.trules, &TRule{Rule: r, Body: func(p *TParser, min int) (any, bool) { return m.evaluate(index, p, min) }})
	}
	if controlled {
		return meteredTypedRule{m}
	}
	return m
}

type meteredTypedRule struct{ *typedRuleMachine }

func (m meteredTypedRule) Used() uint64 { return m.p.Control.budgetUsed() }

func (m *typedRuleMachine) Rule(index int64) any { return m.trules[index] }

// typedInvocation is the handle of an invocation that a generated typed call passes: a pointer, since
// the suite tells handles apart by identity and a function has none.
type typedInvocation struct{ run func(*TParser) (any, bool) }

// Body returns the invocation of the rule with the index that a generated typed call passes: the
// rule's body at binding level 0 in a frame of its own, with the nesting level of a rule on a cycle of
// calls, as the generated invocation runs it.
func (m *typedRuleMachine) Body(index int64) any {
	if inv := m.invs[index]; inv != nil {
		return inv
	}
	r := m.trules[index]
	m.invs[index] = &typedInvocation{func(p *TParser) (any, bool) {
		f := p.NewFrame(len(r.Scope))
		prev := p.Frame
		p.Frame = f
		if r.Rec {
			p.Enter()
		}
		v, ok := r.Body(p, 0)
		if r.Rec {
			p.Depth--
		}
		p.Frame = prev
		p.FreeFrame(f)
		return v, ok
	}}
	return m.invs[index]
}

// evaluate runs the next evaluation of the script of the rule with the index on the typed library.
func (m *typedRuleMachine) evaluate(index int64, p *TParser, level int) (any, bool) {
	var e ref.Eval
	if int(index) < len(m.specs) {
		if s := m.specs[index]; len(s.Body) > 0 {
			e = s.Body[min(m.evals[index], int64(len(s.Body)-1))]
		}
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
		c := p.NewNodeOrdinary("capture", start, start, nil)
		c.TSetFresh(false)
		p.Frame.Vals[k] = c
	}
	p.Pos = start + int(e.Len)
	if !e.OK {
		return nil, false
	}
	return p.NewNodeOrdinary("body", start, p.Pos, nil), true
}

func (m *typedRuleMachine) Observe(component string) []string {
	if component != ops.Rules {
		return nil
	}
	return append([]string{}, m.log...)
}

func (m *typedRuleMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupted()
	p := m.tp
	rule := func() *TRule { return args[0].(*TRule) }
	// at runs f at the position of the registers and moves them to where it leaves the parser.
	at := func(f func() (any, bool)) []any {
		p.Pos = int(m.regs[ops.Pos].(int64))
		defer func() { m.regs[ops.Pos] = int64(p.Pos) }()
		v, ok := f()
		return []any{v, ok}
	}
	invoke := func(f func(r *TRule, min int) (any, bool)) []any {
		return at(func() (any, bool) { return f(rule(), int(args[1].(int64))) })
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
	case "rule.seen":
		return []any{int64(p.confRuleSeen(rule()))}
	case "seen.defers":
		return []any{p.confSeenDefers()}
	case "seen.first":
		return []any{p.confSeenFirst(int(args[0].(int64)), int(args[1].(int64)))}
	case "rule.invoke":
		return invoke(p.confRuleInvoke)
	case "rule.invokePlain":
		return invoke(p.confRuleInvokePlain)
	case "rule.run":
		return at(func() (any, bool) { return p.confRuleRun(args[0].(*typedInvocation).run) })
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

// typedRuleLibOps are the operations of the rules that the typed library provides, with rule.run and
// the doc operations.
var typedRuleLibOps = append([]string{conform.RuleRunOp}, ruleLibOps...)

// typedRuleSuites are the suites of the typed library's operations of the rules, which charge the unit
// of a call before the call invokes the rule, as the node library's do (ruleSuites).
func typedRuleSuites() []conform.Suite {
	s := conform.Suite{Name: "typed library rules", Ops: typedRuleLibOps, CallUnitBefore: true,
		New: func(c ref.Config) conform.Impl { return newTypedRuleMachine(c, false) }}
	controlled := s
	controlled.Name = "typed library rules, controlled"
	controlled.Ops = append(slices.Clone(typedRuleLibOps), rtBudgetOps...)
	controlled.New = func(c ref.Config) conform.Impl { return newTypedRuleMachine(c, true) }
	return []conform.Suite{s, controlled}
}

func TestExecConformanceTypedRule(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, s := range typedRuleSuites() {
		fails, ran := ruleConformanceFailures(s, seeds, 60)
		if ran < seeds {
			t.Fatalf("%s: only %d cases ran", s.Name, ran)
		}
		for _, err := range fails {
			t.Error(err)
		}
	}
}

// TestExecConformanceTypedRuleDetectsBrokenLib is the negative control: the typed library with one
// operation of the rules broken, as a wrong line of its printed code would break it, fails the suite.
func TestExecConformanceTypedRuleDetectsBrokenLib(t *testing.T) {
	inv := func(args []any) func(*TParser) (any, bool) { return args[0].(*typedInvocation).run }
	run := func(m *typedRuleMachine, f func() (any, bool)) []any {
		p := m.tp
		p.Pos = int(m.regs[ops.Pos].(int64))
		v, ok := f()
		m.regs[ops.Pos] = int64(p.Pos)
		return []any{v, ok}
	}
	variants := map[string]func(m *typedRuleMachine, args []any) []any{
		"rule.memo reports the Document's": func(m *typedRuleMachine, args []any) []any { return []any{args[0].(*TRule).DocMemo} },
		"rule.plain is true":               func(m *typedRuleMachine, args []any) []any { return []any{true} },
		"rule.docOnlyMemo counts leaders": func(m *typedRuleMachine, args []any) []any {
			r := args[0].(*TRule)
			return []any{r.DocMemo && !r.Memo}
		},
		"rule.id is the index of the rule after it": func(m *typedRuleMachine, args []any) []any {
			return []any{int64(args[0].(*TRule).Id + 1)}
		},
		"rule.seen is the rule's index": func(m *typedRuleMachine, args []any) []any {
			return []any{int64(args[0].(*TRule).Id)}
		},
		"seen.defers ignores the Document": func(m *typedRuleMachine, args []any) []any { return []any{m.tp.Stream == nil} },
		"seen.defers ignores streams":      func(m *typedRuleMachine, args []any) []any { return []any{m.tp.Document == nil} },
		"rule.invoke runs the body without its own frame": func(m *typedRuleMachine, args []any) []any {
			return run(m, func() (any, bool) { return m.tp.invokePlainBody(args[0].(*TRule), int(args[1].(int64))) })
		},
		"rule.invokePlain keeps the errors of a failure": func(m *typedRuleMachine, args []any) []any {
			p := m.tp
			start := int(m.regs[ops.Pos].(int64))
			return run(m, func() (any, bool) {
				v, ok := p.invoke(args[0].(*TRule), int(args[1].(int64)))
				if !ok {
					p.Pos = start
				}
				return v, ok
			})
		},
		"rule.run counts the evaluation": func(m *typedRuleMachine, args []any) []any {
			return run(m, func() (any, bool) {
				if m.tp.TypedDocument != nil {
					m.tp.Document.stats.Evaluated++
				}
				return m.tp.confRuleRun(inv(args))
			})
		},
		"rule.run drops the errors of a failure": func(m *typedRuleMachine, args []any) []any {
			p := m.tp
			mark := len(p.Recovered)
			return run(m, func() (any, bool) {
				v, ok := p.confRuleRun(inv(args))
				if !ok {
					p.Recovered = p.Recovered[:mark]
				}
				return v, ok
			})
		},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := typedRuleSuites()[0]
			s.Name = name
			s.New = func(c ref.Config) conform.Impl {
				return brokenTypedRule{newTypedRuleMachine(c, false).(*typedRuleMachine), op, v}
			}
			if fails, _ := ruleConformanceFailures(s, 100, 60); len(fails) == 0 {
				t.Fatal("no case detected the broken library")
			}
		})
	}
}

// TestExecConformanceTypedRuleDetectsCharges is the negative control of the charges: the typed library
// whose rule.seen or rule.run charges a unit more fails the controlled suite on the bound.
func TestExecConformanceTypedRuleDetectsCharges(t *testing.T) {
	variants := map[string]func(m *typedRuleMachine, args []any) []any{
		"rule.seen charges a unit": func(m *typedRuleMachine, args []any) []any {
			m.tp.Spend()
			return []any{int64(m.tp.confRuleSeen(args[0].(*TRule)))}
		},
		"rule.run charges a unit": func(m *typedRuleMachine, args []any) []any {
			p := m.tp
			p.Pos = int(m.regs[ops.Pos].(int64))
			p.Spend()
			v, ok := p.confRuleRun(args[0].(*typedInvocation).run)
			m.regs[ops.Pos] = int64(p.Pos)
			return []any{v, ok}
		},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := typedRuleSuites()[1]
			s.Name = name
			s.New = func(c ref.Config) conform.Impl {
				return meteredBrokenTypedRule{brokenTypedRule{newTypedRuleMachine(c, true).(meteredTypedRule).typedRuleMachine, op, v}}
			}
			fails, _ := ruleConformanceFailures(s, 30, 60)
			if len(fails) == 0 {
				t.Fatal("no case detected the charge")
			}
			if msg := fails[0].Error(); !strings.Contains(msg, "more than the bound") {
				t.Fatalf("the first failure is not the charge: %s", msg)
			}
		})
	}
}

type meteredBrokenTypedRule struct{ brokenTypedRule }

func (m meteredBrokenTypedRule) Used() uint64 { return m.p.Control.budgetUsed() }

// brokenTypedRule is the typed library with one operation of the rules replaced.
type brokenTypedRule struct {
	*typedRuleMachine
	op   string
	call func(m *typedRuleMachine, args []any) []any
}

func (b brokenTypedRule) Call(o *exec.Op, args []any) (res []any) {
	if o.Name == b.op {
		defer interrupted()
		return b.call(b.typedRuleMachine, args)
	}
	return b.typedRuleMachine.Call(o, args)
}
