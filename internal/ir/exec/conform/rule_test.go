package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

func (m modelImpl) Body(index int64) any { return ref.BodyRef{Index: index} }

// ruleMutant breaks one operation of the rules of the model, with the registers at hand.
type ruleMutant struct {
	modelImpl
	regs map[string]any
	op   string
	edit func(m *ruleMutant, args []any) []any
}

func (m *ruleMutant) UseRegisters(regs map[string]any) {
	m.regs = regs
	m.Model.UseRegisters(regs)
}

func (m *ruleMutant) Call(o *exec.Op, args []any) []any {
	if o.Name == m.op {
		return m.edit(m, args)
	}
	return m.Model.Call(o, args)
}

func (m *ruleMutant) call(name string, args ...any) []any {
	return m.Model.Call(ruleModule.Op(name), args)
}

var ruleModule = RuleModule()

// ruleSuite is a suite of the rules over the model, broken by mu if it is not nil, and of a target
// whose Document parse takes the call protocol if callMemo.
func ruleSuite(name string, callMemo bool, mu *ruleMutant) Suite {
	return Suite{Name: name, Ops: append(append(append([]string{}, RuleOps...), RuleCaseOps...), DocOps...), DocCallMemo: callMemo,
		New: func(c ref.Config) Impl {
			if mu == nil {
				return modelImpl{ref.New(c)}
			}
			b := *mu
			b.modelImpl = modelImpl{ref.New(c)}
			return &b
		}}
}

// randomRule returns the randomized case of the rules with the seed: every third parse reads a
// Document.
func randomRule(seed uint64) Case {
	return RandomRule("random", rand.New(rand.NewPCG(seed, 9)), append(append([]string{}, RuleOps...), RuleCaseOps...), 60, seed%3 == 2)
}

func TestRuleCasesVerify(t *testing.T) {
	if err := exec.Verify(RuleCases()[0].Module); err != nil {
		t.Fatal(err)
	}
	for seed := range uint64(3) {
		if err := exec.Verify(randomRule(seed).Module); err != nil {
			t.Fatal(err)
		}
	}
}

// ruleFails reports whether some handwritten or randomized case of the rules fails on s.
func ruleFails(s Suite) (handwritten, random bool) {
	for _, c := range RuleCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < 300 && !random; seed++ {
		_, err := Run(s, randomRule(seed))
		random = err != nil
	}
	return handwritten, random
}

// TestModelPassesRuleCases checks the cases of the rules against the contract as the model states it.
func TestModelPassesRuleCases(t *testing.T) {
	for _, callMemo := range []bool{false, true} {
		s := ruleSuite("model", callMemo, nil)
		for _, c := range RuleCases() {
			if skipped, err := Run(s, c); err != nil || skipped {
				t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
			}
		}
		for seed := range uint64(300) {
			if _, err := Run(s, randomRule(seed)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestRandomRuleCasesUseTheOperations checks that randomized cases exercise every operation of the
// rules, in parses with and without a Document.
func TestRandomRuleCasesUseTheOperations(t *testing.T) {
	counts := map[string]int{}
	for seed := range uint64(300) {
		walkCalls(randomRule(seed).Func.Body, func(x *exec.Call) { counts[x.Callee]++ })
	}
	for _, name := range RuleOps {
		if counts[name] < 100 {
			t.Errorf("randomized cases call %s %d times", name, counts[name])
		}
	}
}

// TestDocCallMemoDiffers checks that the cases tell the two meanings of rule.memo in a Document parse
// apart: the model of either kind of target fails the suite of the other.
func TestDocCallMemoDiffers(t *testing.T) {
	for _, callMemo := range []bool{false, true} {
		s := Suite{Name: "other", Ops: append(append(append([]string{}, RuleOps...), RuleCaseOps...), DocOps...), DocCallMemo: callMemo, New: func(c ref.Config) Impl {
			c.DocCallMemo = !callMemo
			return modelImpl{ref.New(c)}
		}}
		if handwritten, random := ruleFails(s); !handwritten || !random {
			t.Errorf("the other meaning passes (call memo %v): handwritten %v, random %v", callMemo, handwritten, random)
		}
	}
}

// sharedValue is the one value of a broken model that makes the same value for every match.
var sharedValue = &ref.RuleValue{Rule: -1}

// ruleMutants are broken models of the rules, the negative control of their cases.
func ruleMutants() map[string]*ruleMutant {
	flip := func(op string) *ruleMutant {
		return &ruleMutant{op: op, edit: func(m *ruleMutant, a []any) []any { return []any{!m.call(op, a...)[0].(bool)} }}
	}
	pos := func(m *ruleMutant) int64 { return m.regs[ops.Pos].(int64) }
	return map[string]*ruleMutant{
		"memo is negated":        flip("rule.memo"),
		"leader is negated":      flip("rule.leader"),
		"plain is negated":       flip("rule.plain"),
		"docMemo is negated":     flip("rule.docMemo"),
		"docPlain is negated":    flip("rule.docPlain"),
		"positional is negated":  flip("rule.positional"),
		"docOnlyMemo is negated": flip("rule.docOnlyMemo"),
		"docOnlyMemo counts leaders": {op: "rule.docOnlyMemo", edit: func(m *ruleMutant, a []any) []any {
			return []any{m.call("rule.docMemo", a...)[0].(bool) && !m.call("rule.memo", a...)[0].(bool)}
		}},
		"id is one more": {op: "rule.id", edit: func(m *ruleMutant, a []any) []any {
			return []any{m.call("rule.id", a...)[0].(int64) + 1}
		}},
		"invoke evaluates at level 0": {op: "rule.invoke", edit: func(m *ruleMutant, a []any) []any {
			return m.call("rule.invoke", a[0], int64(0))
		}},
		"invoke stays at the start of a match": {op: "rule.invoke", edit: func(m *ruleMutant, a []any) []any {
			start := pos(m)
			r := m.call("rule.invoke", a...)
			m.regs[ops.Pos] = start
			return r
		}},
		"invoke drops the errors of a failure": {op: "rule.invoke", edit: func(m *ruleMutant, a []any) []any {
			mark := m.call("rec.mark")[0]
			r := m.call("rule.invoke", a...)
			if !r[1].(bool) {
				m.call("rec.truncate", mark)
			}
			return r
		}},
		"invoke takes a nesting level for every rule": {op: "rule.invoke", edit: func(m *ruleMutant, a []any) []any {
			m.call("depth.enter")
			r := m.call("rule.invoke", a...)
			m.call("depth.exit")
			return r
		}},
		"invoke counts two evaluations": {op: "rule.invoke", edit: func(m *ruleMutant, a []any) []any {
			r := m.call("rule.invoke", a...)
			if m.call("doc.active")[0].(bool) {
				m.call("doc.evaluated")
			}
			return r
		}},
		"invoke makes one value for every match": {op: "rule.invoke", edit: func(m *ruleMutant, a []any) []any {
			r := m.call("rule.invoke", a...)
			if r[1].(bool) {
				r[0] = sharedValue
			}
			return r
		}},
		"invokePlain keeps the position of a failure": {op: "rule.invokePlain", edit: func(m *ruleMutant, a []any) []any {
			return m.call("rule.invoke", a...)
		}},
		"invokePlain keeps the errors of a failure": {op: "rule.invokePlain", edit: func(m *ruleMutant, a []any) []any {
			start := pos(m)
			r := m.call("rule.invoke", a...)
			if !r[1].(bool) {
				m.regs[ops.Pos] = start
			}
			return r
		}},
	}
}

// TestRuleMutantsFail is the negative control of the cases of the rules: each broken model fails a
// handwritten or a randomized case.
func TestRuleMutantsFail(t *testing.T) {
	for name, mu := range ruleMutants() {
		t.Run(name, func(t *testing.T) {
			handwritten, random := ruleFails(ruleSuite(name, false, mu))
			if !handwritten {
				t.Error("no handwritten case detected the mutant")
			}
			if !random {
				t.Error("no randomized case detected the mutant")
			}
		})
	}
}
