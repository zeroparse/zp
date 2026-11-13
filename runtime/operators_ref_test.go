package runtime

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

// The hand-written Operators Rule loop and reset that the compositions of internal/ir/exec/core
// replaced, kept as the reference model of TestOperatorsRuleMatchesReference.

func (p *Parser) refReset(m Mark) {
	p.Pos = m.Pos
	p.Env = m.env
	p.Recovered = p.Recovered[:m.recovered]
	p.undoCaptures(m.Trail)
	p.Trail = p.Trail[:m.Trail]
}
func (p *Parser) refSkipTrivia(pr *OperatorsRule) {
	if pr.Skip == nil {
		return
	}
	m := p.Mark()
	p.Silent++
	_, ok := pr.Skip(p)
	p.Silent--
	if !ok {
		p.refReset(m)
		return
	}
	p.Recovered = p.Recovered[:m.recovered]
}
func (p *Parser) refLongest(ops []*OperatorsRuleOp) (best operatorsRuleAttempt, cutFailed bool) {
	m0 := p.Mark()
	for _, op := range ops {
		p.Spend()
		prevFrame, prevCut := p.Frame, p.Cut
		f := p.NewFrame(len(op.Line.Scope))
		p.Frame, p.Cut = f, false
		v, ok := op.Line.M(p)
		cut := p.Cut
		p.Frame, p.Cut = prevFrame, prevCut
		if ok && p.Pos == m0.Pos && op.Kind != "infix" {
			ok = false // an empty prefix or postfix match could be applied forever
		}
		if ok && (best.op == nil || p.Pos > best.end) {
			best = operatorsRuleAttempt{op: op, frame: f, v: v, start: m0.Pos, end: p.Pos, env: p.Env, cut: cut,
				errs: p.recSince(m0.recovered)}
		}
		if !ok && cut {
			cutFailed = true
		}
		if len(f.Vals) == 0 {
			p.refReset(m0)
			continue
		}
		// reset undoes this candidate's captures; restore them. Nothing parses between saving
		// and restoring, so one buffer per parser suffices.
		p.saved = appendSpend(p, p.saved[:0], f.Vals)
		p.refReset(m0)
		copySpend(p, f.Vals, p.saved)
	}
	return best, cutFailed
}
func (p *Parser) refApply(a *operatorsRuleAttempt) {
	p.Pos, p.Env = a.end, a.env
	if !a.errs.none() {
		p.refApplyErrors(a)
	}
}
func (p *Parser) refApplyErrors(a *operatorsRuleAttempt) {
	p.recAdd(a.errs)
}
func (p *Parser) refOperatorsRuleParse(r *Rule, min int) (*parse.Node, bool) {
	p.Depth++
	if p.Depth > p.MaxDepth {
		p.TooDeep()
	}
	v, ok := p.refOperatorsRuleExpr(r, min)
	p.Depth--
	return v, ok
}
func (p *Parser) refOperatorsRuleExpr(r *Rule, min int) (*parse.Node, bool) {
	pr := r.OperatorsRule
	// The right operand of an infix part that consumed no input does not apply an empty infix part
	// at its start, which would begin the same expression again without end.
	guard := -1
	if p.afterEmpty {
		guard = p.Pos
	}
	p.afterEmpty = false
	lhs, bound, ok := p.refOperatorsRuleNud(r)
	if !ok {
		return nil, false
	}
	if len(pr.Led) == 0 {
		return lhs, true
	}
	lastNone := -1
	for {
		m0 := p.Mark()
		p.refSkipTrivia(pr)
		a, cutFailed := p.refLongest(pr.Led)
		if a.op == nil {
			p.refReset(m0)
			if cutFailed {
				return nil, false
			}
			break
		}
		if a.op.Level <= min || a.op.Level > bound || a.op.Assoc == "none" && a.op.Level == lastNone {
			p.refReset(m0)
			break
		}
		if m0.Pos == guard && a.end == m0.Pos {
			p.refReset(m0)
			break
		}
		p.refApply(&a)
		if a.op.Kind == "postfix" {
			lhs = p.operatorsRuleBuild(r, &a, lhs, nil)
			lastNone = -1
			bound = math.MaxInt
			if p.Pos == m0.Pos {
				break
			}
			continue
		}
		rmin := a.op.Level
		if a.op.Assoc == "right" {
			rmin--
		}
		p.afterEmpty = p.Pos == m0.Pos
		rhs, ok := p.refOperatorsRuleParse(r, rmin)
		if !ok {
			if a.cut {
				return nil, false
			}
			p.refReset(m0)
			break
		}
		lhs = p.operatorsRuleBuild(r, &a, lhs, rhs)
		lastNone = -1
		if a.op.Assoc == "none" {
			lastNone = a.op.Level
		}
		// The right operand took the operators that bind tighter than rmin: the next one that does was
		// refused by it, and cannot apply to this application as a whole.
		bound = rmin
		// An application that consumed no input ends the loop, as an empty iteration ends a repetition:
		// otherwise an operator that can match empty after an operand that can loops forever.
		if p.Pos == m0.Pos {
			break
		}
	}
	return lhs, true
}
func (p *Parser) refOperatorsRuleNud(r *Rule) (*parse.Node, int, bool) {
	pr := r.OperatorsRule
	p.refSkipTrivia(pr)
	m0 := p.Mark()
	if a, cutFailed := p.refLongest(pr.Prefix); a.op != nil {
		p.refApply(&a)
		rhs, ok := p.refOperatorsRuleParse(r, a.op.Level)
		if ok {
			return p.operatorsRuleBuild(r, &a, nil, rhs), a.op.Level, true
		}
		if a.cut {
			return nil, 0, false
		}
		p.refReset(m0)
	} else if cutFailed {
		return nil, 0, false
	}
	for _, o := range pr.Operands {
		p.Spend()
		prevFrame, prevCut := p.Frame, p.Cut
		f := p.NewFrame(len(o.Scope))
		p.Frame, p.Cut = f, false
		v, ok := o.M(p)
		cut := p.Cut
		p.Frame, p.Cut = prevFrame, prevCut
		if ok {
			return p.lineResult(r, o, f, v, m0.Pos), math.MaxInt, true
		}
		p.refReset(m0)
		if cut {
			return nil, 0, false
		}
	}
	return nil, 0, false
}

// oprTrace is what the lines of a random Operators Rule share between the two parses of a round:
// the input, the environments, errors and captured nodes that lines install (compared by identity),
// and the log of the line calls of the parse that runs.
type oprTrace struct {
	in    string
	envs  []*Env
	errs  []*parse.SyntaxError
	nodes []*parse.Node
	log   []string
}

// oprLine returns the matcher of a random line named name: it matches the text lit (none for an
// empty match), and as drawn it captures into its slot, cuts after its first unit, recovers an
// error or binds a new environment.
func oprLine(rng *rand.Rand, tr *oprTrace, name, lit string, slots int) matcher {
	capture := slots > 0 && rng.Intn(2) == 0
	cut := lit != "" && rng.Intn(4) == 0
	more := ""
	if cut && rng.Intn(2) == 0 {
		more = string("ab+"[rng.Intn(3)]) // after the cut, the line needs one more unit
	}
	recover := rng.Intn(5) == 0
	env := rng.Intn(5) == 0
	k := rng.Intn(len(tr.envs))
	return func(p *Parser) (*parse.Node, bool) {
		tr.log = append(tr.log, fmt.Sprintf("%s@%d", name, p.Pos))
		if !strings.HasPrefix(tr.in[p.Pos:], lit) {
			return nil, false
		}
		p.Pos += len(lit)
		if cut {
			p.Cut = true
			if !strings.HasPrefix(tr.in[p.Pos:], more) {
				return nil, false
			}
			p.Pos += len(more)
		}
		if capture {
			p.SetCapture(0, tr.nodes[k])
		}
		if recover {
			p.PushRecovered(tr.errs[k])
		}
		if env {
			p.Env = tr.envs[k]
		}
		return nil, true
	}
}

// oprCaps describes the captures of the frame of an action: the index of the node in the slot of a
// line with one, as tr.nodes holds it.
func oprCaps(tr *oprTrace, c *ActCtx) string {
	if len(c.f.Vals) == 0 {
		return ""
	}
	for i, n := range tr.nodes {
		if c.f.Vals[0] == n {
			return fmt.Sprintf("{%d}", i)
		}
	}
	return "{}"
}

// oprShow describes the value of an application or an operand that the actions of oprRule make.
func oprShow(v any) string {
	if v == nil {
		return "_"
	}
	return fmt.Sprint(v)
}

// oprRule returns a random Operators Rule over the units "ab-!+*? #".
func oprRule(rng *rand.Rand, tr *oprTrace) *Rule {
	pr := &OperatorsRule{}
	lit := func(set string) string {
		if rng.Intn(6) == 0 {
			return ""
		}
		return string(set[rng.Intn(len(set))])
	}
	line := func(name, l string) *OperatorsRuleLine {
		slots := rng.Intn(2)
		scope := make([]string, slots)
		for i := range scope {
			scope[i] = "x"
		}
		return &OperatorsRuleLine{Scope: scope, M: oprLine(rng, tr, name, l, slots)}
	}
	for i := range 1 + rng.Intn(3) {
		name := fmt.Sprintf("o%d", i)
		l := line(name, lit("ab"))
		l.Action = func(c *ActCtx) (any, any) {
			return fmt.Sprintf("%s%s[%d,%d]", name, oprCaps(tr, c), c.start, c.end), nil
		}
		pr.Operands = append(pr.Operands, l)
	}
	op := func(id int, kind, set string) *OperatorsRuleOp {
		name := fmt.Sprintf("%s%d", kind[:2], id)
		l := line(name, lit(set))
		l.Action = func(c *ActCtx) (any, any) {
			return fmt.Sprintf("(%s%s %s %s)[%d,%d]", name, oprCaps(tr, c), oprShow(c.Lhs), oprShow(c.Rhs), c.start, c.end), nil
		}
		return &OperatorsRuleOp{Id: id, Kind: kind, Assoc: []string{"left", "right", "none"}[rng.Intn(3)], Level: 1 + rng.Intn(3), Line: l}
	}
	for i := range rng.Intn(3) {
		pr.Prefix = append(pr.Prefix, op(i, "prefix", "-!"))
	}
	for i := range rng.Intn(4) {
		kind := []string{"infix", "infix", "postfix"}[rng.Intn(3)]
		pr.Led = append(pr.Led, op(10+i, kind, "+*!?"))
	}
	if rng.Intn(2) == 0 {
		// A skip that needs a "#" after its spaces recovers an error and then fails after
		// consuming them, so that skipTrivia must reset what it did.
		hash := rng.Intn(2) == 0
		pr.Skip = func(p *Parser) (*parse.Node, bool) {
			tr.log = append(tr.log, fmt.Sprintf("skip@%d", p.Pos))
			if p.Pos == len(tr.in) || tr.in[p.Pos] != ' ' {
				return nil, false
			}
			for p.Pos < len(tr.in) && tr.in[p.Pos] == ' ' {
				p.Pos++
			}
			if hash {
				p.PushRecovered(tr.errs[0])
				if p.Pos == len(tr.in) || tr.in[p.Pos] != '#' {
					return nil, false
				}
				p.Pos++
			}
			return nil, true
		}
	}
	return &Rule{Name: "e", OperatorsRule: pr}
}

// TestOperatorsRuleMatchesReference parses random inputs with random Operators Rules through the
// printed OperatorsRuleParse and the hand-written form kept above, with and without budget controls,
// and compares the result, the calls of the lines, the position, the recovered errors, the
// environment, the trail, the current frame and cut flag, the depth and the units charged.
func TestOperatorsRuleMatchesReference(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		rng := rand.New(rand.NewSource(1))
		for round := 0; round < 3000; round++ {
			tr := &oprTrace{}
			for range 3 {
				tr.envs = append(tr.envs, &Env{})
				tr.errs = append(tr.errs, &parse.SyntaxError{Pos: len(tr.errs)})
				tr.nodes = append(tr.nodes, nodeOfData(&Data{}))
			}
			r := oprRule(rng, tr)
			var in strings.Builder
			for range rng.Intn(9) {
				in.WriteByte("ab-!+*? #"[rng.Intn(9)])
			}
			tr.in = in.String()
			min := rng.Intn(3)
			run := func(parseFn func(p *Parser) (*parse.Node, bool)) string {
				tr.log = nil
				p := &Parser{MaxDepth: 12}
				if controlled {
					p.Control = newBudgetControl(nil, parse.Limits{Compute: 1 << 40})
				}
				var v *parse.Node
				var ok bool
				fatalMsg := ""
				func() {
					defer func() {
						if x := recover(); x != nil {
							f, ok := x.(fatal)
							if !ok {
								panic(x)
							}
							fatalMsg = f.err.Error()
						}
					}()
					v, ok = parseFn(p)
				}()
				spent := uint64(0)
				if controlled {
					spent = p.Control.budgetUsed()
				}
				env := -1
				for i, e := range tr.envs {
					if p.Env == e {
						env = i
					}
				}
				var errs []int
				for _, e := range p.recoveredErrors() {
					errs = append(errs, e.Pos)
				}
				return fmt.Sprintf("v=%s ok=%v fatal=%q pos=%d errs=%v env=%d trail=%d frame=%v cut=%v depth=%d spent=%d log=%v",
					oprShow(UnboxNode(v)), ok, fatalMsg, p.Pos, errs, env, len(p.Trail), p.Frame != nil, p.Cut, p.Depth, spent, tr.log)
			}
			got := run(func(p *Parser) (*parse.Node, bool) { return p.OperatorsRuleParse(r, min) })
			want := run(func(p *Parser) (*parse.Node, bool) { return p.refOperatorsRuleParse(r, min) })
			if got != want {
				t.Fatalf("controlled %v round %d input %q min %d:\nprinted   %s\nreference %s", controlled, round, tr.in, min, got, want)
			}
		}
	}
}
