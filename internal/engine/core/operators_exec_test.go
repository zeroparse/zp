package core

import (
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/runtime/parse"
)

// An oprForm runs the Operators Rule loop in one of its two printed forms. The recursive form calls
// the lines itself; the split form stops at each line, skip and expression, and the functions below
// drive it as the iterative VM does.
type oprForm struct {
	name    string
	longest func(p *Parser, l []*OperatorsRuleOp) OperatorsRuleAttempt
	trivia  func(p *Parser, r *Rule)
	expr    func(p *Parser, r *Rule, min int) bool
}

var oprForms = []oprForm{
	{"recursive", (*Parser).Longest, (*Parser).skipTrivia, func(p *Parser, r *Rule, min int) bool {
		_, ok := p.OperatorsRuleParse(r, min)
		return ok
	}},
	{"split", splitLongest, splitTrivia, func(p *Parser, r *Rule, min int) bool {
		_, ok := splitExpr(p, r, min)
		return ok
	}},
}

func splitLongest(p *Parser, l []*OperatorsRuleOp) OperatorsRuleAttempt {
	var st LongestState
	p.LongestBegin(&st, l)
	line, more := p.LongestNext(&st)
	for more {
		v, ok := line.M(p)
		line, more = p.LongestStep(&st, v, ok)
	}
	return st.Best
}

func splitTrivia(p *Parser, r *Rule) {
	if r.OperatorsRule.Skip == nil {
		return
	}
	m := p.TriviaBegin()
	_, ok := r.OperatorsRule.Skip(p)
	p.TriviaEnd(m, ok)
}

func splitNud(p *Parser, r *Rule) (*parse.Node, bool) {
	var st NudState
	splitTrivia(p, r)
	p.NudBegin(&st)
	st.A = splitLongest(p, r.OperatorsRule.Prefix)
	level, ok := p.NudPrefix(r, &st)
	if !ok {
		return nil, false
	}
	if st.A.Op != nil {
		rhs, rok := splitExpr(p, r, level)
		if v, ok, done := p.NudRhs(r, &st, rhs, rok); done {
			return v, ok
		}
	}
	for {
		line, more := p.NudNext(r, &st)
		if !more {
			return nil, false
		}
		v, lok := line.M(p)
		if v, ok, done := p.NudOperand(r, &st, v, lok); done {
			return v, ok
		}
	}
}

func splitExpr(p *Parser, r *Rule, min int) (*parse.Node, bool) {
	p.Enter()
	defer func() { p.Depth-- }()
	st := ExprState{Min: min}
	p.ExprBegin(&st)
	lhs, ok := splitNud(p, r)
	if done, ok := p.ExprStart(r, &st, lhs, ok); done {
		return st.Lhs, ok
	}
	for {
		p.ExprLoop(&st)
		splitTrivia(p, r)
		st.A = splitLongest(p, r.OperatorsRule.Led)
		done, ok, rmin := p.ExprLed(r, &st)
		if done {
			return st.Lhs, ok
		}
		if rmin < 0 {
			continue
		}
		rhs, rok := splitExpr(p, r, rmin)
		if done, ok := p.ExprRhs(r, &st, rhs, rok); done {
			return st.Lhs, ok
		}
	}
}

// oprParser returns a parser for the lines of these tests, which read text themselves. The limits
// turn a loop that never ends into a failure.
func oprParser() *Parser {
	p := &Parser{MaxDepth: 64}
	p.Control = NewBudgetControl(nil, parse.Limits{Compute: 10000})
	return p
}

func oprLine(m Matcher) *OperatorsRuleLine {
	return &OperatorsRuleLine{Scope: &compile.Scope{}, M: m}
}

func oprOp(kind, assoc string, level int, m Matcher) *OperatorsRuleOp {
	return &OperatorsRuleOp{Kind: kind, Assoc: assoc, Level: level, Line: oprLine(m)}
}

// oprRule returns a rule without values, so that the end of a parse shows which operators applied.
func oprRule(pr *OperatorsRule) *Rule {
	r := &Rule{OperatorsRule: pr}
	r.Novalue = true
	return r
}

// oprLit matches the byte c of text.
func oprLit(text string, c byte) Matcher {
	return func(p *Parser) (*parse.Node, bool) {
		if p.Pos < len(text) && text[p.Pos] == c {
			p.Pos++
			return nil, true
		}
		return nil, false
	}
}

func oprFail(*Parser) (*parse.Node, bool) { return nil, false }

// oprFailCut fails after a cut, away from where it started.
func oprFailCut(p *Parser) (*parse.Node, bool) {
	p.Cut = true
	p.Pos += 3
	return nil, false
}

// TestOperatorsLoop pins which operators apply, in both printed forms.
func TestOperatorsLoop(t *testing.T) {
	type rule func(text string) *OperatorsRule
	operand := func(text string) []*OperatorsRuleLine {
		return []*OperatorsRuleLine{oprLine(oprLit(text, 'a'))}
	}
	led := func(ops ...func(string) *OperatorsRuleOp) rule {
		return func(text string) *OperatorsRule {
			pr := &OperatorsRule{Operands: operand(text)}
			for _, op := range ops {
				pr.Led = append(pr.Led, op(text))
			}
			return pr
		}
	}
	infix := func(c byte, assoc string, level int) func(string) *OperatorsRuleOp {
		return func(text string) *OperatorsRuleOp { return oprOp(grammar.Infix, assoc, level, oprLit(text, c)) }
	}
	postfix := func(c byte, level int) func(string) *OperatorsRuleOp {
		return func(text string) *OperatorsRuleOp { return oprOp(grammar.Postfix, "", level, oprLit(text, c)) }
	}
	prefixed := func(c byte, level int, r rule) rule {
		return func(text string) *OperatorsRule {
			pr := r(text)
			pr.Prefix = []*OperatorsRuleOp{oprOp(grammar.Prefix, "", level, oprLit(text, c))}
			return pr
		}
	}
	nothing := func(*Parser) (*parse.Node, bool) { return nil, true }
	empty := func(string) *OperatorsRuleOp { return oprOp(grammar.Infix, grammar.AssocLeft, 1, nothing) }
	for _, c := range []struct {
		name, text string
		rule       rule
		min, end   int
		ok         bool
	}{
		{"a left operator repeats", "a+a+a", led(infix('+', grammar.AssocLeft, 1)), 0, 5, true},
		{"an operator at the minimum stays out", "a+a", led(infix('+', grammar.AssocLeft, 1)), 1, 1, true},
		{"a right operator takes its own level on the right", "a^a^a", led(infix('^', grammar.AssocRight, 1)), 0, 5, true},
		{"a non-associative operator does not repeat", "a=a=a", led(infix('=', grammar.AssocNone, 1)), 0, 3, true},
		{"a postfix application clears the non-associative level", "a=a!=a", led(infix('=', grammar.AssocNone, 2), postfix('!', 1)), 0, 6, true},
		{"an infix application clears the non-associative level", "a=a&a=a", led(infix('=', grammar.AssocNone, 1), infix('&', grammar.AssocLeft, 1)), 0, 7, true},
		{"a looser infix operator starts a new non-associative chain", "a=a|a=a", led(infix('=', grammar.AssocNone, 2), infix('|', grammar.AssocLeft, 1)), 0, 7, true},
		{"a non-associative operator does not chain in the operand of a looser left infix", "a|a=a=a", led(infix('=', grammar.AssocNone, 2), infix('|', grammar.AssocLeft, 1)), 0, 5, true},
		{"a non-associative operator does not chain in the operand of a looser right infix", "a^a=a=a", led(infix('=', grammar.AssocNone, 2), infix('^', grammar.AssocRight, 1)), 0, 5, true},
		{"a non-associative operator does not chain in the operand of a prefix operator", "~a=a=a", prefixed('~', 1, led(infix('=', grammar.AssocNone, 2))), 0, 4, true},
		{"a non-associative operator does not chain below a postfix operator in an operand", "a|a=a!=a", led(infix('=', grammar.AssocNone, 2), infix('|', grammar.AssocLeft, 1), postfix('!', 3)), 0, 6, true},
		{"a postfix application lifts the bound of the infix before it", "a+a!*a", led(infix('+', grammar.AssocLeft, 1), postfix('!', 1), infix('*', grammar.AssocLeft, 2)), 0, 6, true},
		{"a right operator bounds the operators after it below its own level", "a^a=a=a", led(infix('^', grammar.AssocRight, 1), infix('=', grammar.AssocNone, 1)), 0, 5, true},
		{"a tighter operator whose operand fails is not retried", "a|a=", led(infix('=', grammar.AssocNone, 2), infix('|', grammar.AssocLeft, 1)), 0, 3, true},
		{"a missing right operand ends the loop before the operator", "a+", led(infix('+', grammar.AssocLeft, 1)), 0, 1, true},
		{"an application that consumes nothing ends the loop", "", func(string) *OperatorsRule {
			return &OperatorsRule{Operands: []*OperatorsRuleLine{oprLine(nothing)}, Led: []*OperatorsRuleOp{empty("")}}
		}, 0, 0, true},
		{"a postfix operator that consumes nothing does not apply", "a", led(func(string) *OperatorsRuleOp {
			return oprOp(grammar.Postfix, "", 1, nothing)
		}), 0, 1, true},
		{"a prefix operator that consumes nothing does not apply", "a", func(text string) *OperatorsRule {
			return &OperatorsRule{Operands: operand(text), Prefix: []*OperatorsRuleOp{oprOp(grammar.Prefix, "", 1, nothing)}}
		}, 0, 1, true},
		{"a failed start fails", "+", led(infix('+', grammar.AssocLeft, 1)), 0, 0, false},
	} {
		for _, form := range oprForms {
			t.Run(c.name+"/"+form.name, func(t *testing.T) {
				p := oprParser()
				if ok := form.expr(p, oprRule(c.rule(c.text)), c.min); ok != c.ok || p.Pos != c.end {
					t.Fatalf("ok=%t pos=%d, want ok=%t pos=%d", ok, p.Pos, c.ok, c.end)
				}
				if p.Depth != 0 {
					t.Fatalf("depth %d after the parse", p.Depth)
				}
			})
		}
	}
}

// TestOperatorsWithoutLedEndAtTheStart pins that nothing runs after the start of an expression when
// no infix or postfix operator can follow it, not even the skip.
func TestOperatorsWithoutLedEndAtTheStart(t *testing.T) {
	for _, form := range oprForms {
		t.Run(form.name, func(t *testing.T) {
			skips := 0
			r := oprRule(&OperatorsRule{
				Operands: []*OperatorsRuleLine{oprLine(oprLit("a", 'a'))},
				Skip: func(*Parser) (*parse.Node, bool) {
					skips++
					return nil, true
				},
			})
			p := oprParser()
			if !form.expr(p, r, 0) || skips != 1 {
				t.Fatalf("pos=%d skips=%d", p.Pos, skips)
			}
		})
	}
}

// TestOperatorsLongestLineState pins what a longest match gives each line and what it leaves.
func TestOperatorsLongestLineState(t *testing.T) {
	for _, form := range oprForms {
		t.Run(form.name, func(t *testing.T) {
			p := oprParser()
			p.Cut = true
			frame := &Frame{}
			p.Frame = frame
			lines := 0
			line := func(ok bool) Matcher {
				return func(p *Parser) (*parse.Node, bool) {
					lines++
					if p.Cut {
						t.Fatalf("line %d starts inside the cut of its caller", lines)
					}
					if p.Frame == frame {
						t.Fatalf("line %d runs in the frame of its caller", lines)
					}
					p.Cut = true
					p.Pos++
					return nil, ok
				}
			}
			a := form.longest(p, []*OperatorsRuleOp{oprOp(grammar.Infix, "", 1, line(false)), oprOp(grammar.Infix, "", 1, line(true)), oprOp(grammar.Infix, "", 1, line(false))})
			if lines != 3 || a.Op == nil || !a.Cut {
				t.Fatalf("lines=%d attempt=%+v", lines, a)
			}
			if !p.Cut || p.Frame != frame || p.Pos != 0 {
				t.Fatalf("the match left cut=%t pos=%d, frame kept=%t", p.Cut, p.Pos, p.Frame == frame)
			}
			if !p.cutFailed {
				t.Fatal("a failure after a cut is not reported")
			}
		})
	}
	for _, form := range oprForms {
		t.Run(form.name+"/a match after a cut is no failure", func(t *testing.T) {
			p := oprParser()
			p.cutFailed = true
			a := form.longest(p, []*OperatorsRuleOp{oprOp(grammar.Infix, "", 1, func(p *Parser) (*parse.Node, bool) {
				p.Cut = true
				return nil, true
			})})
			if a.Op == nil || p.cutFailed || p.Cut {
				t.Fatalf("attempt=%+v cutFailed=%t, cut=%t after the match", a, p.cutFailed, p.Cut)
			}
		})
		t.Run(form.name+"/no operators report no failure", func(t *testing.T) {
			p := oprParser()
			p.cutFailed = true
			if form.longest(p, nil); p.cutFailed {
				t.Fatal("the failure of an earlier match survives")
			}
		})
	}
}

// oprMissing inserts a token at the position, with no error of its own: the event alone must travel.
func oprMissing(p *Parser) {
	p.InsertMissing(MissingTokenEvent{Site: MissingTokenSite{ID: 1, Pos: p.Pos}})
}

// TestOperatorsAttemptKeepsItsState pins that a match returns to the position and that its attempt
// holds what the line recorded, and that applying the attempt brings that back.
func TestOperatorsAttemptKeepsItsState(t *testing.T) {
	env := &Env{}
	errs := []*parse.SyntaxError{{}}
	for _, form := range oprForms {
		t.Run(form.name, func(t *testing.T) {
			p := oprParser()
			p.Missing = &MissingTokenState{Limit: 8}
			a := form.longest(p, []*OperatorsRuleOp{oprOp(grammar.Infix, "", 1, func(p *Parser) (*parse.Node, bool) {
				oprMissing(p)
				p.Env = env
				p.Pos = 2
				p.PushRecovered(errs[0])
				return nil, true
			})})
			if a.Env != env || a.Errs.Len() != 1 || len(a.missing) != 1 {
				t.Fatalf("attempt %+v", a)
			}
			if p.Env != nil || p.Pos != 0 || len(p.Recovered) != 0 || p.MissingLen != 0 {
				t.Fatalf("the match left env=%v pos=%d errors=%d missing=%d", p.Env, p.Pos, len(p.Recovered), p.MissingLen)
			}
		})
		t.Run(form.name+"/applied", func(t *testing.T) {
			text := "a+a"
			for _, level := range []string{"infix", "prefix"} {
				p := oprParser()
				p.Missing = &MissingTokenState{Limit: 8}
				line := func(c byte) Matcher {
					lit := oprLit(text, c)
					return func(p *Parser) (*parse.Node, bool) {
						v, ok := lit(p)
						if ok {
							oprMissing(p)
							p.Env = env
						}
						return v, ok
					}
				}
				pr := &OperatorsRule{Operands: []*OperatorsRuleLine{oprLine(oprLit(text, 'a'))}}
				start := 0
				if level == "infix" {
					pr.Led = []*OperatorsRuleOp{oprOp(grammar.Infix, grammar.AssocLeft, 1, line('+'))}
				} else {
					pr.Prefix = []*OperatorsRuleOp{oprOp(grammar.Prefix, "", 1, line('+'))}
					start = 1
				}
				p.Pos = start
				if !form.expr(p, oprRule(pr), 0) || p.Pos != 3 {
					t.Fatalf("%s: pos=%d", level, p.Pos)
				}
				if p.Env != env || p.MissingLen != 1 {
					t.Fatalf("%s: applying left env=%v missing=%d", level, p.Env, p.MissingLen)
				}
			}
		})
	}
}

// TestOperatorsTrivia pins the skip between operands: a lookahead that leaves no errors or inserted
// tokens, and leaves the parse as it was when it fails.
func TestOperatorsTrivia(t *testing.T) {
	for _, form := range oprForms {
		for _, ok := range []bool{true, false} {
			name := "matches"
			if !ok {
				name = "fails"
			}
			t.Run(form.name+"/"+name, func(t *testing.T) {
				p := oprParser()
				p.Missing = &MissingTokenState{Limit: 8}
				silent := 0
				r := oprRule(&OperatorsRule{Skip: func(p *Parser) (*parse.Node, bool) {
					silent = p.Silent
					oprMissing(p)
					p.PushRecovered(&parse.SyntaxError{})
					p.Pos = 2
					return nil, ok
				}})
				form.trivia(p, r)
				want := 0
				if ok {
					want = 2
				}
				if silent != 1 || p.Silent != 0 {
					t.Fatalf("silent %d in the skip, %d after it", silent, p.Silent)
				}
				if p.Pos != want || len(p.Recovered) != 0 || p.MissingLen != 0 {
					t.Fatalf("pos=%d errors=%d missing=%d", p.Pos, len(p.Recovered), p.MissingLen)
				}
			})
		}
	}
}

// TestOperatorsOperandLines pins the operand lines: each runs at the start with a cut flag of its
// own, and a failure after a cut fails the expression instead of trying the next line.
func TestOperatorsOperandLines(t *testing.T) {
	for _, form := range oprForms {
		t.Run(form.name+"/a failed line returns to the start", func(t *testing.T) {
			p := oprParser()
			p.Cut = true
			second := false
			r := oprRule(&OperatorsRule{Operands: []*OperatorsRuleLine{
				oprLine(func(p *Parser) (*parse.Node, bool) {
					if p.Cut {
						t.Fatal("the first line starts inside the cut of its caller")
					}
					p.Pos = 2
					return nil, false
				}),
				oprLine(func(p *Parser) (*parse.Node, bool) {
					if p.Pos != 0 {
						t.Fatalf("the second line starts at %d", p.Pos)
					}
					if p.Cut {
						t.Fatal("the second line starts inside the cut of its caller")
					}
					second = true
					p.Cut = true
					return nil, true
				}),
			}})
			if !form.expr(p, r, 0) || !second {
				t.Fatalf("second=%t", second)
			}
			if !p.Cut {
				t.Fatal("the cut flag of the caller is lost")
			}
		})
		t.Run(form.name+"/a cut in a line stays in it", func(t *testing.T) {
			p := oprParser()
			r := oprRule(&OperatorsRule{Operands: []*OperatorsRuleLine{oprLine(func(p *Parser) (*parse.Node, bool) {
				p.Cut = true
				return nil, true
			})}})
			if !form.expr(p, r, 0) || p.Cut {
				t.Fatalf("cut=%t after the expression", p.Cut)
			}
		})
		t.Run(form.name+"/a failure after a cut fails", func(t *testing.T) {
			p := oprParser()
			r := oprRule(&OperatorsRule{Operands: []*OperatorsRuleLine{oprLine(oprFailCut), oprLine(func(*Parser) (*parse.Node, bool) {
				t.Fatal("the next line ran after a cut")
				return nil, true
			})}})
			if form.expr(p, r, 0) || p.Pos != 0 {
				t.Fatalf("pos=%d", p.Pos)
			}
		})
		t.Run(form.name+"/no line matches", func(t *testing.T) {
			p := oprParser()
			if form.expr(p, oprRule(&OperatorsRule{Operands: []*OperatorsRuleLine{oprLine(oprFail)}}), 0) {
				t.Fatal("the expression matched")
			}
		})
	}
}
