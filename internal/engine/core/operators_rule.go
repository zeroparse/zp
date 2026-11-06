package core

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/value"
)

// OperatorsRule is a compiled Operators Rule expression. Binding levels are numbered from 1 in declaration
// order (higher binds tighter).
type OperatorsRule struct {
	Skip      Matcher
	SkipEntry int // start of the skip bytecode (bytecode backend only; -1 if none)
	Operands  []*OperatorsRuleLine
	Prefix    []*OperatorsRuleOp
	Led       []*OperatorsRuleOp // infix and postfix
}

// OperatorsRuleLine is an operand line or an operator line. Each has its own capture scope.
type OperatorsRuleLine struct {
	Scope *compile.Scope
	M     Matcher
	Act   Evaluator // action (nil if none)
	// Value makes the value of an operand line without an action, and single that of an operator
	// line's operator, from the value of the line's expression.
	Value  func(p *Parser, v *value.Node, start int) *value.Node
	Single compile.SingleMode
	Shape  compile.Shaper
	Entry  int // start of the bytecode (bytecode backend only)
	// Key is the line's *grammar.OperatorsRuleOperand or *grammar.OperatorsRuleOperator.
	Key any
}

type OperatorsRuleOp struct {
	Id    int
	Kind  string
	Assoc string
	Level int
	Line  *OperatorsRuleLine
}

// OperatorsRuleAttempt is the result of trying an operator part.
type OperatorsRuleAttempt struct {
	Op         *OperatorsRuleOp
	Frame      *Frame
	V          *value.Node
	Start, End int
	Env        *Env
	Cut        bool
	Errs       []*SyntaxError // syntax errors recovered in this candidate
	missing    []MissingTokenEvent
}

// LongestState is the record of a suspended longest match (Parser.LongestBegin, LongestNext and
// LongestStep), which the iterative VM holds. Best is the match when the last step returned.
type LongestState struct {
	L         []*OperatorsRuleOp
	I         int
	M0        Mark
	Best      OperatorsRuleAttempt
	CutFailed bool
	PrevFrame *Frame
	PrevCut   bool
	F         *Frame
}

// NudState is the record of the suspended start of an expression (Parser.NudBegin and its steps). The
// caller puts the longest match of the prefix operators in A.
type NudState struct {
	M0        Mark
	A         OperatorsRuleAttempt
	I         int
	PrevFrame *Frame
	PrevCut   bool
	F         *Frame
}

// ExprState is the record of the suspended loop of an expression (Parser.ExprStart and its steps). The
// caller sets Min and puts the longest match of each iteration in A; Lhs is the value so far.
type ExprState struct {
	Min      int
	Lhs      *value.Node
	LastNone int
	Bound    int
	M0       Mark
	A        OperatorsRuleAttempt
}

// ApplyRecovered replays the recovery errors, and under missing-token support the missing-token
// events, of an applied operator. It is separate so that the step that applies an operator stays
// small: the common strict parse has neither.
func (p *Parser) ApplyRecovered(a *OperatorsRuleAttempt) {
	if p.Missing == nil {
		p.Recovered = AppendSpend(p, p.Recovered, a.Errs)
	} else {
		p.restoreMissing(a.missing, a.Errs, false)
	}
}

// LineResult builds the value of an operand line, and checks it against the rule's declared type
// if needed.
func (p *Parser) LineResult(r *Rule, l *OperatorsRuleLine, f *Frame, v *value.Node, start int, locals *Local) *value.Node {
	if r.Novalue {
		return nil
	}
	if l.Act != nil {
		ctx := p.UseCtx(EvalCtx{P: p, Scope: l.Scope, Frame: f, Start: start, End: p.Pos, Locals: locals, Cbase: len(p.Created)})
		v = p.ActionResult(ctx, l.Act, r.WhereName())
	} else {
		v = l.Value(p, v, start)
		if len(l.Scope.Names) > 0 {
			v = p.AttachCaptures(v, l.Scope, f, start, p.Pos)
		}
		if v != nil && value.D(v).Fresh {
			if p.Prog.RecordsSourceInfo && value.D(v).Kind.Rule == "" {
				value.D(v).Kind = r.Kinds.Named(value.D(v).Kind, r.Name)
			}
			value.D(v).Fresh = false
		}
	}
	return v
}

// convertOperator converts v, the value of an operator of rule r, to the named node type of the
// rule's operators.
func (p *Parser) convertOperator(r *Rule, v *value.Node) *value.Node {
	n, ok := r.OpKinds.Convert(value.UnboxNode(v))
	if !ok {
		p.Fail("cannot convert %s to %s in result of rule %s", TypeName(value.UnboxNode(v)), r.OpNodeType, r.WhereName())
	}
	return n
}

// OperatorsRuleBuild builds the value of an operator application.
func (p *Parser) OperatorsRuleBuild(r *Rule, a *OperatorsRuleAttempt, lhs, rhs *value.Node) *value.Node {
	if r.Novalue {
		return nil
	}
	start, end := a.Start, p.Pos
	if lhs != nil {
		start = int(value.D(lhs).Start)
	}
	if a.Op.Kind == grammar.Postfix {
		end = a.End
	}
	l := a.Op.Line
	// The operator's value is its one element, or the node of its tuple, which has the line's
	// captures.
	opv, ok := ShapeSingle(p, l.Shape, l.Single, a.V, a.Start)
	if !ok {
		opv = p.NewNode(value.Data{Kind: value.SeqKind, Start: int32(a.Start), End: int32(a.End)})
	}
	if l.Single == compile.SingleTuple && len(l.Scope.Names) > 0 {
		opv = p.AttachCaptures(opv, l.Scope, a.Frame, a.Start, a.End)
	}
	if opv != nil {
		value.D(opv).Fresh = false
	}
	if l.Act != nil {
		// The locals live in the parser, like the evaluation context (see UseCtx).
		ls := &p.oplocals
		ls[0] = Local{Name: "op", Val: NodeOrNil(opv)}
		locals := &ls[0]
		if a.Op.Kind != grammar.Prefix {
			ls[1] = Local{Name: "lhs", Val: NodeOrNil(lhs), Next: locals}
			locals = &ls[1]
		}
		if a.Op.Kind != grammar.Postfix {
			ls[2] = Local{Name: "rhs", Val: NodeOrNil(rhs), Next: locals}
			locals = &ls[2]
		}
		ctx := p.UseCtx(EvalCtx{P: p, Scope: l.Scope, Frame: a.Frame, Start: start, End: end, Locals: locals, Cbase: len(p.Created)})
		v := p.ActionResult(ctx, l.Act, r.WhereName())
		if r.OpKinds != nil {
			v = p.convertOperator(r, v)
		}
		return v
	}
	// Without an action, the application is the node of the tuple (lhs, op, rhs) of the line's
	// kind, with those captures.
	var kids []*value.Node
	var fields value.Fields
	switch a.Op.Kind {
	case grammar.Prefix:
		kids = p.Nodes(2)
		kids[0], kids[1] = opv, rhs
		fields = value.Fields{{Name: "op", Value: NodeOrNil(opv)}, {Name: "rhs", Value: NodeOrNil(rhs)}}
	case grammar.Postfix:
		kids = p.Nodes(2)
		kids[0], kids[1] = lhs, opv
		fields = value.Fields{{Name: "lhs", Value: NodeOrNil(lhs)}, {Name: "op", Value: NodeOrNil(opv)}}
	default:
		kids = p.Nodes(3)
		kids[0], kids[1], kids[2] = lhs, opv, rhs
		fields = value.Fields{{Name: "lhs", Value: NodeOrNil(lhs)}, {Name: "op", Value: NodeOrNil(opv)}, {Name: "rhs", Value: NodeOrNil(rhs)}}
	}
	kind := value.SeqKind
	if r.OpKinds != nil {
		kind = r.OpKinds[value.BaseTuple] // made with the named node type of the operators
	} else if p.Prog.RecordsSourceInfo {
		kind = r.Kinds.Named(value.SeqKind, r.Name)
	}
	n := p.NewNode(value.Data{Kind: kind, Start: int32(start), End: int32(end), Children: kids, Fields: fields, Labeled: p.Prog.RecordsSourceInfo})
	return n
}
