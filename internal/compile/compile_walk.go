package compile

import (
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// walker walks the rule bodies of a program for the compiler: it checks them, reports the errors,
// and decides the expectation table, the value-free twins and the capture slots of the program. The
// backends build their forms from Core IR, not from the walk.
type walker struct {
	*Compiler
}

func (c *walker) compileRule(r *Rule) {
	control := c.Control
	c.Control.Poll()
	c.rule = r
	if p, ok := r.Def.Expr.(*ast.OperatorsRule); ok {
		c.compileOperatorsRule(r, p)
		return
	}
	body := r.BodyExpr()
	r.Stream = c.checkStream(body)
	r.Lean = r.Novalue || leanBody(r)
	c.proj = ruleProjectionsControlled(c.Prog, r, control)
	c.Expr(body, r.Scope, !r.Lean)
	c.proj = nil
	if r.Action != nil {
		c.CheckTerm(r.Action, r.Scope, nil)
	}
}

// Expr walks the expression. If build is false, the expression is matched without building values.
func (c *walker) Expr(e ast.Expr, s *check.Scope, build bool) {
	c.Control.Poll()
	control := c.Control
	switch e := e.(type) {
	case *ast.Literal:
		c.desc(LiteralDesc(e))

	case *ast.CharClass:
		c.desc(CharClassStringControlled(e, c.Control))

	case *ast.Any:

	case *ast.Ref:
		r := c.Prog.ByName[e.Name]
		if r == nil {
			c.errorf(e.Diagnostic(), "undefined rule %s", e.Name)
			return
		}
		if e.Level != "" {
			c.levelMin(e, r)
		}
		if !build && r.TwinOK {
			c.twinOf(r)
		}

	case *ast.Seq:
		for _, it := range e.Items {
			c.Expr(it, s, build)
		}

	case *ast.Choice:
		for _, alt := range e.Alts {
			c.Expr(alt, s, build)
		}

	case *ast.Repeat:
		c.repeat(e, s, build)

	case *ast.Optional:
		c.Expr(e.Expr, s, build)

	case *ast.And:
		// Captures and variable definitions inside a positive lookahead remain valid after it.
		c.noRecoveringCaptures(e.Expr)
		c.Expr(e.Expr, s, build && hasCapturesControlled(e.Expr, control))

	case *ast.Not:
		c.noCaptures(e.Expr)
		c.Expr(e.Expr, s, false)

	case *ast.Atomic:
		c.noCaptures(e.Expr)
		c.Expr(e.Expr, s, false)

	case *ast.Discard:
		c.noCaptures(e.Expr)
		c.Expr(e.Expr, s, false)

	case *ast.Capture:
		if !build && c.rule != nil && c.rule.PredCaps != nil && !c.rule.PredCaps.Records(e) {
			pc := c.rule.PredCaps
			// In a program that builds no tree, captures that no predicate can read are not recorded.
			if pc.Slotted(e) {
				s.Slot(e.Name)
			}
			c.Expr(e.Expr, s, false)
			return
		}
		// Even inside a value-free body (leanBody), captures build and record their values.
		s.Slot(e.Name)
		if field := c.proj[e.Name]; field != "" {
			// A projected repetition (project.go): its elements share one scope of their own and
			// build no values.
			c.Expr(e.Expr.(*ast.Repeat).Expr, check.NewScope(), false)
			return
		}
		c.Expr(e.Expr, s, true)

	case *ast.Cut, *ast.Top, *ast.Bottom,
		*ast.BeginInput, *ast.EndInput, *ast.BeginLine, *ast.EndLine:

	case *ast.Predicate:
		c.predicate(e, s)

	case *ast.Attributed:
		c.attributed(e, s, build)

	case *ast.OperatorsRule:
		c.errorf(e.Diagnostic(), "operatorsRule is only allowed as the whole body of a rule")

	default:
		c.errorf(ast.Pos{}, "unsupported expression %T", e)
	}
}

// repeat walks a repetition.
func (c *walker) repeat(e *ast.Repeat, s *check.Scope, build bool) {
	if c.Prog.SkipSite(e) != 0 {
		// A skip-run site has no captures, values or stream.
		c.Expr(e.Expr, s, false)
		return
	}
	elemScope := s
	if elementScopedControlled(e.Expr, c.Control) {
		elemScope = check.NewElementScope(s)
	}
	c.Expr(e.Expr, elemScope, build)
}

// predicate checks a Predicate Action { ... } and walks it.
func (c *walker) predicate(e *ast.Predicate, s *check.Scope) {
	if a, ok := e.Term.(*ast.Bind); ok {
		if c.Prog.Vars.Lookup(a.Name) == nil {
			c.errorf(a.Diagnostic(), "undefined variable %s; declare it with var %s at the top level", a.Name, a.Name)
		}
		c.CheckTerm(a.Value, s, nil)
	} else {
		c.CheckTerm(e.Term, s, nil)
	}
}

// attributed walks an attributed expression and checks its attributes.
func (c *walker) attributed(e *ast.Attributed, s *check.Scope, build bool) {
	c.Expr(e.Expr, s, build)
	for _, a := range e.Attrs {
		switch a.Name {
		case "stream":
			if _, ok := e.Expr.(*ast.Repeat); !ok {
				c.errorf(a.Diagnostic(), "#stream must be attached to a repetition (*, +, or #repeat(...))")
			}
			if len(a.Args)+len(a.KwArgs) > 0 {
				c.errorf(a.Diagnostic(), "#stream takes no arguments")
			}
		case "missing":
			lit, ok := e.Expr.(*ast.Literal)
			if !ok || lit.Value == "" {
				c.errorf(a.Diagnostic(), "#missing requires a nonempty literal")
				continue
			}
			if len(a.Args)+len(a.KwArgs) != 0 {
				c.errorf(a.Diagnostic(), "#missing takes no arguments")
			}
		case "error":
			if msg, ok := c.stringArg(a, "message"); ok {
				c.desc(msg)
			}
		case "diagnostic":
			if name, ok := c.stringArg(a, "name"); ok {
				c.desc(name)
			}
		case "recover":
			skipExpr, ok := a.Arg("skip")
			if !ok {
				c.errorf(a.Diagnostic(), "#recover requires a skip argument")
				continue
			}
			c.noCaptures(skipExpr)
			c.Expr(skipExpr, s, false)
		default:
			c.errorf(a.Diagnostic(), "unknown attribute @%s", a.Name)
		}
		for name := range a.KwArgs {
			if !validArg(a.Name, name) {
				c.errorf(a.Diagnostic(), "%s%s has no argument %s", desugar.AnnotationMarker(a), a.Name, name)
			}
		}
	}
}

// compileOperatorsRule walks the Operators Rule e, the body of the rule r.
func (c *walker) compileOperatorsRule(r *Rule, e *ast.OperatorsRule) {
	if e.Skip != nil {
		c.noCaptures(e.Skip)
		c.Expr(e.Skip, r.Scope, false)
	}
	if len(e.Operands) == 0 {
		c.errorf(e.Diagnostic(), "operatorsRule needs at least one operand")
	}
	for _, o := range e.Operands {
		// A loaded compiled grammar (c.an is nil) has already been checked. A grammar must be valid
		// whether or not insertion is enabled, so the left calls are also those with insertion.
		if c.an != nil && (contains(c.an.LeftCalls(o.Expr, nil), r.Name) ||
			c.ins != nil && contains(c.ins.LeftCalls(o.Expr, nil), r.Name)) {
			c.errorf(e.Diagnostic(), "operand of %s calls %s at its start (left recursion); use infix or postfix instead", r.Name, r.Name)
		}
		c.checkValueCaptures(o.Diagnostic(), c.operatorsRuleLine(o.Expr, o.Action, nil))
	}
	for _, l := range e.Levels {
		for _, op := range l.Operators {
			var locals []string
			switch op.Kind {
			case ast.Prefix:
				locals = []string{"rhs", "op"}
			case ast.Postfix:
				locals = []string{"lhs", "op"}
			case ast.Infix:
				locals = []string{"lhs", "rhs", "op"}
				if op.Assoc != ast.AssocLeft && op.Assoc != ast.AssocRight && op.Assoc != ast.AssocNone {
					c.errorf(op.Diagnostic(), "invalid associativity %q", op.Assoc)
				}
			default:
				c.errorf(op.Diagnostic(), "invalid operator kind %q", op.Kind)
				continue
			}
			scope := c.operatorsRuleLine(op.Expr, op.Action, locals)
			for _, name := range scope.Names {
				if name == "lhs" || name == "rhs" || name == "op" {
					c.errorf(op.Diagnostic(), "capture name %s is reserved in operatorsRule operators", name)
				}
			}
			c.checkValueCaptures(op.Diagnostic(), scope)
		}
	}
	seen := map[string]bool{}
	for _, l := range e.Levels {
		if l.Name != "" {
			if seen[l.Name] {
				c.errorf(e.Diagnostic(), "duplicate level name %s", l.Name)
			}
			seen[l.Name] = true
		}
	}
	if r.Action != nil {
		// The whole-rule action sees only the final folded value, as $value.
		c.CheckTerm(r.Action, r.Scope, OperatorsRuleActionLocals)
	}
}

// checkValueCaptures reports a capture named value in the scope of the operand or operator line
// at d: the name is reserved for the folded value, which the whole-rule action reads as $value.
func (c *walker) checkValueCaptures(d ast.Pos, scope *check.Scope) {
	for _, name := range scope.Names {
		if name == OperatorsRuleValue {
			c.errorf(d, "capture name %s is reserved for the whole-rule action of an Operators Rule, which reads the folded value as $%s", name, name)
		}
	}
}

// OperatorsRuleValue is the local name of the final folded value of an Operators Rule in its
// whole-rule action, and OperatorsRuleActionLocals the locals of that action.
const OperatorsRuleValue = check.OperatorsRuleValue

var OperatorsRuleActionLocals = []string{OperatorsRuleValue}

// operatorsRuleLine walks the line e of an Operators Rule and checks its action, whose locals are
// locals, and returns the line's scope.
func (c *walker) operatorsRuleLine(e ast.Expr, action ast.Term, locals []string) *check.Scope {
	s := check.NewScope()
	c.Expr(e, s, !c.rule.Novalue && !LeanLine(action, locals))
	if action != nil && !c.rule.Novalue {
		c.CheckTerm(action, s, locals)
	}
	return s
}

// LeanLine is check.LeanLine.
func LeanLine(action ast.Term, locals []string) bool { return check.LeanLine(action, locals) }
