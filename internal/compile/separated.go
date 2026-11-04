package compile

import "zeroparse.dev/zp/grammar"

// SeparatedForms returns the parts of the separated list e (a repetition with a #sep) that a
// backend runs, as expressions of the grammar:
//
//   - body is the element; under the required policy it is the element, a cut and the separator.
//   - pre, if not nil, runs before every element after the first: the separator and a cut, and
//     under the optional policy a lookahead of the first characters of the element between them.
//     If it fails, the list ends before the separator; the cut makes a failing element fail the
//     list.
//   - trail, if not nil, runs once after the last element when the list has one: the separator of
//     the optional policy, which ends the list if it matches.
//
// The message is not empty if the list is not sound: the commit after the separator needs an
// element and a separator that cannot match the empty input, and an element that cannot start
// with a character that starts the separator.
func SeparatedForms(prog *Program, e *grammar.Repeat, control *Control) (body, pre, trail grammar.Expr, msg string) {
	k := prog.sepChars
	if k == nil {
		k = newClassifier(prog.Grammar, control.rules(prog.Grammar), "", control)
		k.computeFirst()
		prog.sepChars = k
	}
	switch {
	case k.nullable(e.Expr):
		return nil, nil, nil, "the element can match the empty input"
	case k.nullable(e.Sep):
		return nil, nil, nil, "the separator can match the empty input"
	case k.first(e.Expr).intersects(k.first(e.Sep)):
		return nil, nil, nil, "the element can start with a character that starts the separator"
	}
	sep := &grammar.Discard{SourceInfo: grammar.SourceInfo{Pos: e.Diagnostic()}, Expr: e.Sep}
	cut := &grammar.Cut{}
	switch e.Trailing {
	case TrailingRequired:
		return &grammar.Seq{Items: []grammar.Expr{e.Expr, cut, sep}}, nil, nil, ""
	case TrailingOptional:
		look := &grammar.And{Expr: &grammar.CharClass{Ranges: k.first(e.Expr)}}
		return e.Expr, &grammar.Seq{Items: []grammar.Expr{sep, look, cut}}, sep, ""
	}
	return e.Expr, &grammar.Seq{Items: []grammar.Expr{sep, cut}}, nil, ""
}
