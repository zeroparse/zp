package compile

import (
	"strings"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// normalizeAttributes builds the executable grammar without changing the
// source AST. Attribute syntax remains generic; built-ins define their own
// binding and lowerings before type checking and backend generation.
func normalizeAttributes(g *grammar.Grammar) (*grammar.Grammar, error) {
	return NormalizeAttributesWithCopies(g, false)
}

// NormalizeAttributesWithCopies is normalizeAttributes where eager preserves the
// allocation-heavy normalization path for counterfactual measurements. It affects compilation
// only, never generated parser execution.
func NormalizeAttributesWithCopies(g *grammar.Grammar, eager bool) (*grammar.Grammar, error) {
	return NormalizeAttributesControlled(g, eager, nil)
}

// NormalizeAttributesControlled is NormalizeAttributesWithCopies polling control for
// cancellation; a nil control never cancels.
func NormalizeAttributesControlled(g *grammar.Grammar, eager bool, control *Control) (*grammar.Grammar, error) {
	control.check()
	n := &attributeNormalizer{eager: eager, control: control}
	if control != nil {
		n.poll = control.poll
	}
	statements, changed := normalizeItems(g.File.Decls, eager, n.decl, control)
	control.check()
	if len(n.errors) != 0 {
		return nil, n.errors
	}
	if !changed {
		return g, nil
	}
	copy := *g
	copy.File.Decls = statements
	return &copy, nil
}

// attributeNormalizer binds and lowers the attributes of a grammar, collecting the errors of
// invalid ones. An eager normalizer copies every attribute it visits.
type attributeNormalizer struct {
	errors  ErrorList
	eager   bool
	control *Control
	poll    func()
}

func (n *attributeNormalizer) decl(decl grammar.Decl) grammar.Decl {
	switch rd := decl.(type) {
	case *grammar.RuleDef:
		expr := n.expr(rd.Expr)
		typ := grammaranalysis.UngroupTypeWithPoll(rd.Type, n.poll)
		action := grammaranalysis.UngroupTermWithPoll(rd.Action, n.poll)
		if !n.eager && expr == rd.Expr && typ == rd.Type && action == rd.Action {
			return rd
		}
		rule := *rd
		rule.Expr, rule.Type, rule.Action = expr, typ, action
		return &rule
	case grammar.TypeDecl:
		return n.typeDecl(rd)
	default:
		return decl
	}
}

// Changed containers are copied before their first write. Nil input containers
// still become nonnil empty slices, matching the executable AST representation.
func normalizeItems[T comparable](items []T, eager bool, normalize func(T) T, control *Control) ([]T, bool) {
	var normalized []T
	changed := eager || items == nil
	if changed {
		control.check()
		normalized = make([]T, len(items))
		normalizationCopy(normalized, items, control)
	}
	for i, item := range items {
		control.poll()
		child := normalize(item)
		if child != item && !changed {
			control.check()
			normalized = make([]T, len(items))
			normalizationCopy(normalized, items, control)
			changed = true
		}
		if changed {
			normalized[i] = child
		}
	}
	if changed {
		return normalized, true
	}
	return items, false
}

func (n *attributeNormalizer) error(a *grammar.Attribute, message string) {
	n.errors = append(n.errors, &Error{Pos: a.Diagnostic(), Msg: annotationMarker(a) + a.Name + ": " + message})
}

func (n *attributeNormalizer) value(value any) (any, bool) {
	if e, ok := value.(grammar.Expr); ok {
		child := n.expr(e)
		return child, child != e
	}
	// Extension values can be maps or slices; comparing arbitrary interface
	// values would panic. Only expression arguments undergo normalization.
	return value, false
}

// attribute returns a with its arguments normalized; it returns a itself when nothing changes and
// the normalizer is not eager.
func (n *attributeNormalizer) attribute(a *grammar.Attribute) *grammar.Attribute {
	n.control.poll()
	copy := *a
	changed := n.eager || a.Args == nil || (len(a.KeywordOrder) == 0 && a.KeywordOrder != nil)
	argsCopied := n.eager || a.Args == nil
	if argsCopied {
		n.control.check()
		copy.Args = make([]any, len(a.Args))
	}
	for i, value := range a.Args {
		n.control.poll()
		child, lowered := n.value(value)
		if lowered && !argsCopied {
			copy.Args = normalizationClone(a.Args, n.control)
			argsCopied, changed = true, true
		}
		if argsCopied {
			copy.Args[i] = child
		}
	}
	keywordsCopied := n.eager && a.KwArgs != nil
	if keywordsCopied {
		n.control.check()
		copy.KwArgs = make(map[string]any, len(a.KwArgs))
	}
	for name, value := range a.KwArgs {
		n.control.poll()
		child, lowered := n.value(value)
		if lowered && !keywordsCopied {
			copy.KwArgs = cloneAttributeKeywords(a.KwArgs, n.control)
			keywordsCopied, changed = true, true
		}
		if keywordsCopied {
			copy.KwArgs[name] = child
		}
	}
	if len(a.KeywordOrder) == 0 {
		copy.KeywordOrder = nil
	} else if n.eager {
		copy.KeywordOrder = normalizationClone(a.KeywordOrder, n.control)
	}
	if !changed {
		return a
	}
	return &copy
}

// cloneAttributeKeywords returns a copy of the keyword arguments values, polling control per
// entry.
func cloneAttributeKeywords(values map[string]any, control *Control) map[string]any {
	control.check()
	copy := make(map[string]any, len(values))
	for name, value := range values {
		control.poll()
		copy[name] = value
	}
	return copy
}

// bind moves the one positional argument of a to the keyword argument name. It reports an error
// when a has more than one positional argument, an empty one, or a keyword argument name
// already, and returns an attribute without positional arguments as it is. A normalizer that is
// not eager binds into a copy of a.
func (n *attributeNormalizer) bind(a *grammar.Attribute, name string) *grammar.Attribute {
	if len(a.Args) == 0 {
		return a
	}
	if len(a.Args) != 1 || a.Args[0] == nil {
		n.error(a, "requires one nonempty positional argument")
		return a
	}
	if _, exists := a.KwArgs[name]; exists {
		n.error(a, "argument "+name+" is specified twice")
		return a
	}
	if !n.eager {
		copy := *a
		copy.KwArgs = cloneAttributeKeywords(a.KwArgs, n.control)
		a = &copy
	} else if a.KwArgs == nil {
		a.KwArgs = map[string]any{}
	}
	a.KwArgs[name] = a.Args[0]
	a.Args = nil
	return a
}

func (n *attributeNormalizer) repeat(e grammar.Expr, a *grammar.Attribute) grammar.Expr {
	if len(a.KwArgs) != 0 {
		n.error(a, "accepts only positional arguments")
		return e
	}
	if len(a.Args) != 1 && len(a.Args) != 2 {
		n.error(a, "requires one or two positional slots")
		return e
	}
	if a.Args[0] == nil && (len(a.Args) == 1 || a.Args[1] == nil) {
		n.error(a, "requires at least one bound")
		return e
	}
	min := 0
	if a.Args[0] != nil {
		value, ok := a.Args[0].(int)
		if !ok || value < 0 {
			n.error(a, "minimum must be a nonnegative integer")
			return e
		}
		min = value
	}
	max := min
	if len(a.Args) == 2 {
		max = -1
		if a.Args[1] != nil {
			value, ok := a.Args[1].(int)
			if !ok || value < 0 {
				n.error(a, "maximum must be a nonnegative integer")
				return e
			}
			max = value
		}
	}
	if max >= 0 && max < min {
		n.error(a, "maximum is less than minimum")
		return e
	}
	return &grammar.Repeat{SourceInfo: grammar.SourceInfo{Pos: a.Diagnostic()}, Expr: e, Min: min, Max: max}
}

func (n *attributeNormalizer) expr(expr grammar.Expr) grammar.Expr {
	n.control.poll()
	switch e := expr.(type) {
	case *grammar.Group:
		return n.expr(e.Expr)
	case *grammar.InlineAction:
		child := n.expr(e.Expr)
		term := grammaranalysis.UngroupTermWithPoll(e.Action, n.poll)
		if !n.eager && child == e.Expr && term == e.Action {
			return e
		}
		copy := *e
		copy.Expr, copy.Action = child, term
		return &copy
	case *grammar.Predicate:
		term := e.Term
		if block, ok := term.(*grammar.Block); ok && len(block.Statements) > 0 {
			return predicateBlock(e, block, n.control)
		}
		term = grammaranalysis.UngroupTermWithPoll(term, n.poll)
		if !n.eager && term == e.Term {
			return e
		}
		copy := *e
		copy.Term = term
		return &copy
	case *grammar.Seq:
		items, changed := normalizeItems(e.Items, n.eager, n.expr, n.control)
		items, expanded := expandPredicateBlocks(e.Items, items, n.control)
		changed = changed || expanded
		if !changed {
			return e
		}
		copy := *e
		copy.Items = items
		return &copy
	case *grammar.Choice:
		items, changed := normalizeItems(e.Alts, n.eager, n.expr, n.control)
		if !changed {
			return e
		}
		copy := *e
		copy.Alts = items
		return &copy
	case *grammar.Repeat:
		child := n.expr(e.Expr)
		if !n.eager && child == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = child
		return &copy
	case *grammar.Optional:
		child := n.expr(e.Expr)
		if !n.eager && child == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = child
		return &copy
	case *grammar.And:
		child := n.expr(e.Expr)
		if !n.eager && child == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = child
		return &copy
	case *grammar.Not:
		child := n.expr(e.Expr)
		if !n.eager && child == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = child
		return &copy
	case *grammar.Stream:
		return n.wrap(n.expr(e.Expr), &grammar.Attribute{SourceInfo: grammar.SourceInfo{Pos: e.Diagnostic()}, Name: "stream", Pragma: true})
	case *grammar.Recovery:
		return n.wrap(n.expr(e.Expr), &grammar.Attribute{SourceInfo: grammar.SourceInfo{Pos: e.Diagnostic()}, Name: "recover", Pragma: true, KwArgs: map[string]any{"skip": n.expr(e.Skip)}})
	case *grammar.Atomic:
		child := n.expr(e.Expr)
		if !n.eager && child == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = child
		return &copy
	case *grammar.Discard:
		child := n.expr(e.Expr)
		if !n.eager && child == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = child
		return &copy
	case *grammar.Capture:
		child := n.expr(e.Expr)
		if !n.eager && child == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = child
		return &copy
	case *grammar.Attributed:
		inner := n.expr(e.Expr)
		seenDiagnostic := false
		for _, source := range e.Attrs {
			n.control.poll()
			a := n.attribute(source)
			pragma := a.Name == "repeat" || a.Name == "atomic" || a.Name == "recover" || a.Name == "stream" || a.Name == "missing" || a.Name == "ignorecase" || a.Name == "sep"
			if pragma && !a.Pragma {
				n.error(a, "is a pragma; use #"+a.Name)
				continue
			}
			if (a.Name == "error" || a.Name == "diagnostic") && a.Pragma {
				n.error(a, "is an attribute; use @"+a.Name)
				continue
			}
			switch a.Name {
			case "repeat":
				inner = n.repeat(inner, a)
				continue
			case "atomic":
				if len(a.Args)+len(a.KwArgs) != 0 {
					n.error(a, "takes no arguments")
				}
				inner = &grammar.Atomic{SourceInfo: grammar.SourceInfo{Pos: a.Diagnostic()}, Expr: inner}
				continue
			case "ignorecase":
				if len(a.Args)+len(a.KwArgs) != 0 {
					n.error(a, "takes no arguments")
				}
				inner = foldLiterals(inner)
				continue
			case "error":
				a = n.bind(a, "message")
			case "diagnostic":
				if seenDiagnostic {
					n.error(a, "may occur only once on an expression")
				}
				seenDiagnostic = true
				if len(a.Args) != 0 || len(a.KwArgs) != 1 {
					n.error(a, "requires exactly one named argument, name")
				} else if _, ok := a.KwArgs["name"]; !ok {
					n.error(a, "requires a name argument")
				} else if lit, ok := a.Arg("name"); !ok {
					n.error(a, "name must be a string")
				} else if stringLit, ok := lit.(*grammar.Literal); !ok {
					n.error(a, "name must be a string")
				} else if stringLit.Value == "" || strings.ContainsAny(stringLit.Value, "\r\n\u0085\u2028\u2029") {
					n.error(a, "name must be nonempty and contain no line separator")
				}
			case "recover":
				a = n.bind(a, "skip")
			case "sep":
				var ok bool
				if a, ok = n.separator(inner, a); !ok {
					continue
				}
			}
			if a.Name == "sep" {
				sep, trailing, _ := Separator(&grammar.Attributed{Attrs: []*grammar.Attribute{a}})
				inner = withSeparator(inner, sep, trailing)
			}
			if old, ok := inner.(*grammar.Attributed); ok {
				copy := *old
				copy.Attrs = appendNormalizedAttribute(old.Attrs, a, n.control)
				inner = &copy
			} else {
				inner = &grammar.Attributed{Expr: inner, Attrs: []*grammar.Attribute{a}}
			}
		}
		return inner
	case *grammar.OperatorsRule:
		skip := n.expr(e.Skip)
		operands, operandsChanged := normalizeItems(e.Operands, n.eager, n.operand, n.control)
		levels, levelsChanged := normalizeItems(e.Levels, n.eager, n.level, n.control)
		if !n.eager && skip == e.Skip && !operandsChanged && !levelsChanged {
			return e
		}
		copy := *e
		copy.Skip, copy.Operands, copy.Levels = skip, operands, levels
		return &copy
	}
	return expr
}

func (n *attributeNormalizer) operand(o *grammar.OperatorsRuleOperand) *grammar.OperatorsRuleOperand {
	expr, action := n.expr(o.Expr), grammaranalysis.UngroupTermWithPoll(o.Action, n.poll)
	if !n.eager && expr == o.Expr && action == o.Action {
		return o
	}
	copy := *o
	copy.Expr, copy.Action = expr, action
	return &copy
}

func (n *attributeNormalizer) level(l *grammar.OperatorsRuleLevel) *grammar.OperatorsRuleLevel {
	operators, changed := normalizeItems(l.Operators, n.eager, n.operator, n.control)
	if !changed {
		return l
	}
	copy := *l
	copy.Operators = operators
	return &copy
}

func (n *attributeNormalizer) operator(o *grammar.OperatorsRuleOperator) *grammar.OperatorsRuleOperator {
	expr, action := n.expr(o.Expr), grammaranalysis.UngroupTermWithPoll(o.Action, n.poll)
	if !n.eager && expr == o.Expr && action == o.Action {
		return o
	}
	copy := *o
	copy.Expr, copy.Action = expr, action
	return &copy
}

// AttributeMessage returns the message argument of a normalized @error attribute.
func AttributeMessage(a *grammar.Attribute) string {
	value, _ := a.Arg("message")
	return value.(*grammar.Literal).Value
}

// AttributeDiagnosticName returns the name argument of a normalized @diagnostic attribute.
func AttributeDiagnosticName(a *grammar.Attribute) string {
	value, _ := a.Arg("name")
	return value.(*grammar.Literal).Value
}

func annotationMarker(a *grammar.Attribute) string {
	if a.Pragma {
		return "#"
	}
	return "@"
}

func (n *attributeNormalizer) wrap(e grammar.Expr, a *grammar.Attribute) grammar.Expr {
	if old, ok := e.(*grammar.Attributed); ok {
		copy := *old
		copy.Attrs = appendNormalizedAttribute(old.Attrs, a, n.control)
		return &copy
	}
	return &grammar.Attributed{Expr: e, Attrs: []*grammar.Attribute{a}}
}

// Source grouping lowers before checking/generation; unchanged children stay
// shared so ordinary action and type trees need no extra copies.
func (n *attributeNormalizer) typeDecl(td grammar.TypeDecl) grammar.TypeDecl {
	switch s := td.(type) {
	case *grammar.TypeAlias:
		child := grammaranalysis.UngroupTypeWithPoll(s.Type, n.poll)
		if child != s.Type {
			v := *s
			v.Type = child
			return &v
		}
	case *grammar.TypeDef:
		st := s.Struct()
		if st == nil {
			if s.Type == nil {
				return td
			}
			if child := grammaranalysis.UngroupTypeWithPoll(s.Type, n.poll); child != s.Type {
				v := *s
				v.Type = child
				return &v
			}
			return td
		}
		var fields []*grammar.Field
		for i, f := range st.Fields {
			n.control.poll()
			child := grammaranalysis.UngroupTypeWithPoll(f.Type, n.poll)
			if child == f.Type {
				continue
			}
			if fields == nil {
				fields = normalizationClone(st.Fields, n.control)
			}
			v := *f
			v.Type = child
			fields[i] = &v
		}
		if fields != nil {
			cp := *st
			cp.Fields = fields
			v := *s
			v.Type = &cp
			return &v
		}
	}
	return td
}

// foldLiterals returns e with each string literal in it set to match ASCII letters in either case,
// for #ignorecase. The literals are the ones written in e, its pragma arguments and an Operators
// Rule's operands, operators and skip; a called rule and a character class are not changed. Nodes
// that hold no such literal are shared with the result, and the source tree is not modified.
func foldLiterals(e grammar.Expr) grammar.Expr {
	switch e := e.(type) {
	case *grammar.Literal:
		if e.IgnoreCase {
			return e
		}
		copy := *e
		copy.IgnoreCase = true
		return &copy
	case *grammar.Group:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.InlineAction:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Repeat:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Optional:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.And:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Not:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Stream:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Atomic:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Discard:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Capture:
		if child := foldLiterals(e.Expr); child != e.Expr {
			copy := *e
			copy.Expr = child
			return &copy
		}
	case *grammar.Recovery:
		child, skip := foldLiterals(e.Expr), foldLiterals(e.Skip)
		if child != e.Expr || skip != e.Skip {
			copy := *e
			copy.Expr, copy.Skip = child, skip
			return &copy
		}
	case *grammar.Seq:
		if items, changed := foldExprs(e.Items); changed {
			copy := *e
			copy.Items = items
			return &copy
		}
	case *grammar.Choice:
		if alts, changed := foldExprs(e.Alts); changed {
			copy := *e
			copy.Alts = alts
			return &copy
		}
	case *grammar.Attributed:
		inner := foldLiterals(e.Expr)
		attrs, changed := e.Attrs, false
		for i, a := range e.Attrs {
			if b := foldAttribute(a); b != a {
				if !changed {
					attrs = append([]*grammar.Attribute(nil), e.Attrs...)
					changed = true
				}
				attrs[i] = b
			}
		}
		if inner != e.Expr || changed {
			if sep, trailing, ok := Separator(&grammar.Attributed{Attrs: attrs}); ok && changed {
				inner = withSeparator(inner, sep, trailing) // the separator the repetition runs is the folded one
			}
			copy := *e
			copy.Expr, copy.Attrs = inner, attrs
			return &copy
		}
	case *grammar.OperatorsRule:
		skip := foldLiterals(e.Skip)
		operands := make([]*grammar.OperatorsRuleOperand, len(e.Operands))
		operandsChanged := false
		for i, o := range e.Operands {
			operands[i] = o
			if x := foldLiterals(o.Expr); x != o.Expr {
				copy := *o
				copy.Expr = x
				operands[i] = &copy
				operandsChanged = true
			}
		}
		levels := make([]*grammar.OperatorsRuleLevel, len(e.Levels))
		levelsChanged := false
		for i, l := range e.Levels {
			levels[i] = l
			ops := make([]*grammar.OperatorsRuleOperator, len(l.Operators))
			opsChanged := false
			for j, o := range l.Operators {
				ops[j] = o
				if x := foldLiterals(o.Expr); x != o.Expr {
					copy := *o
					copy.Expr = x
					ops[j] = &copy
					opsChanged = true
				}
			}
			if opsChanged {
				copy := *l
				copy.Operators = ops
				levels[i] = &copy
				levelsChanged = true
			}
		}
		if skip != e.Skip || operandsChanged || levelsChanged {
			copy := *e
			copy.Skip = skip
			if operandsChanged {
				copy.Operands = operands
			}
			if levelsChanged {
				copy.Levels = levels
			}
			return &copy
		}
	}
	return e
}

// foldExprs applies foldLiterals to each expression, and reports whether any changed.
func foldExprs(items []grammar.Expr) ([]grammar.Expr, bool) {
	var out []grammar.Expr
	for i, it := range items {
		if f := foldLiterals(it); f != it {
			if out == nil {
				out = append([]grammar.Expr(nil), items...)
			}
			out[i] = f
		}
	}
	if out == nil {
		return items, false
	}
	return out, true
}

// foldAttribute applies foldLiterals to the expression arguments of a, and returns a if none change.
func foldAttribute(a *grammar.Attribute) *grammar.Attribute {
	var args []any
	argsChanged := false
	for i, v := range a.Args {
		if e, ok := v.(grammar.Expr); ok {
			if f := foldLiterals(e); f != e {
				if args == nil {
					args = append([]any(nil), a.Args...)
				}
				args[i] = f
				argsChanged = true
			}
		}
	}
	var kw map[string]any
	for k, v := range a.KwArgs {
		if e, ok := v.(grammar.Expr); ok {
			if f := foldLiterals(e); f != e {
				if kw == nil {
					kw = map[string]any{}
					for k2, v2 := range a.KwArgs {
						kw[k2] = v2
					}
				}
				kw[k] = f
			}
		}
	}
	if !argsChanged && kw == nil {
		return a
	}
	copy := *a
	if argsChanged {
		copy.Args = args
	}
	if kw != nil {
		copy.KwArgs = kw
	}
	return &copy
}

// The trailing-separator policies of #sep.
const (
	TrailingForbidden = "forbidden"
	TrailingOptional  = "optional"
	TrailingRequired  = "required"
)

// separator checks and binds the arguments of the pragma #sep a on inner: the separator is the
// keyword argument sep as an expression and the policy trailing one of the policies (forbidden if
// omitted), both as strings or expressions the normalizer lowers. It reports false after an
// error.
func (n *attributeNormalizer) separator(inner grammar.Expr, a *grammar.Attribute) (*grammar.Attribute, bool) {
	rep, ok := separatedRepeat(inner)
	if !ok {
		n.error(a, "applies to a repetition (*, + or #repeat(...))")
		return a, false
	}
	if rep.Max == 0 {
		n.error(a, "needs a repetition that can match an element")
		return a, false
	}
	if at, ok := inner.(*grammar.Attributed); ok {
		for _, old := range at.Attrs {
			if old.Name == "sep" {
				n.error(a, "may occur only once on a repetition")
				return a, false
			}
		}
	}
	for name := range a.KwArgs {
		if name != "sep" && name != "trailing" {
			n.error(a, "has no argument "+name)
			return a, false
		}
	}
	a = n.bind(a, "sep")
	value, present := a.KwArgs["sep"]
	if !present {
		n.error(a, "requires a separator")
		return a, false
	}
	var sep grammar.Expr
	switch v := value.(type) {
	case string:
		sep = &grammar.Literal{SourceInfo: a.SourceInfo, Value: v}
	case grammar.Expr:
		sep = v
	}
	if sep == nil {
		n.error(a, "separator must be a string or a parser expression")
		return a, false
	}
	if lit, ok := sep.(*grammar.Literal); ok && lit.Value == "" {
		n.error(a, "separator must not be empty")
		return a, false
	}
	captured := false
	WalkExpr(sep, func(e grammar.Expr) {
		if _, ok := e.(*grammar.Capture); ok {
			captured = true
		}
	})
	if captured {
		n.error(a, "separator must not contain captures")
		return a, false
	}
	policy := TrailingForbidden
	if t, present := a.KwArgs["trailing"]; present {
		s, ok := t.(string)
		if !ok || s != TrailingForbidden && s != TrailingOptional && s != TrailingRequired {
			n.error(a, `trailing must be "forbidden", "optional" or "required"`)
			return a, false
		}
		policy = s
	}
	copy := *a
	copy.Args = nil
	copy.KwArgs = map[string]any{"sep": sep, "trailing": policy}
	copy.KeywordOrder = nil
	return &copy, true
}

// withSeparator returns e, a repetition with pragmas attached, with the separator and policy set on
// the repetition.
func withSeparator(e grammar.Expr, sep grammar.Expr, trailing string) grammar.Expr {
	switch x := e.(type) {
	case *grammar.Repeat:
		copy := *x
		copy.Sep, copy.Trailing = sep, trailing
		return &copy
	case *grammar.Attributed:
		copy := *x
		copy.Expr = withSeparator(x.Expr, sep, trailing)
		return &copy
	}
	return e
}

// separatedRepeat returns the repetition that e is, possibly with pragmas attached.
func separatedRepeat(e grammar.Expr) (*grammar.Repeat, bool) {
	for {
		switch x := e.(type) {
		case *grammar.Repeat:
			return x, true
		case *grammar.Attributed:
			e = x.Expr
		default:
			return nil, false
		}
	}
}

// Separator returns the separator and the trailing-separator policy of the repetition that the
// normalized expression e is, or false if e has no #sep.
func Separator(e grammar.Expr) (sep grammar.Expr, trailing string, ok bool) {
	at, isAttr := e.(*grammar.Attributed)
	if !isAttr {
		return nil, "", false
	}
	for _, a := range at.Attrs {
		if a.Name != "sep" {
			continue
		}
		sep, _ = a.KwArgs["sep"].(grammar.Expr)
		trailing, _ = a.KwArgs["trailing"].(string)
		return sep, trailing, sep != nil
	}
	return nil, "", false
}
