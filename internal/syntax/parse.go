package syntax

import (
	"context"
	"errors"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/genparser"
)

// sourceLimits are the limits of the source parser. Depth bounds the nesting of its recursive
// rule calls: about 500 levels of parentheses in rule expressions, actions and types, so that a
// malformed or hostile source does not build a tree that the consumers of the grammar AST cannot
// walk. Compute is unlimited: a caller stops a long parse with its context.
var sourceLimits = genparser.WithLimits(genparser.Limits{Depth: 2500, Compute: genparser.Unlimited})

// Parse parses the ZeroParse source code src.
//
// The source is parsed by the parser generated from the grammar of ZeroParse
// grammar files, and FromSourceTree builds the grammar AST from its typed
// values. A rejected source returns an ErrorList: the lexical errors, then the
// syntax errors and the errors of the checks that the grammar does not make,
// in the order of the source.
func Parse(src string) (*grammar.Grammar, error) {
	text, invalid := WithoutInvalidCharacters(src)
	f, err := genparser.ParseAST(text, genparser.WithExpectations(), sourceLimits)
	if err != nil {
		errs, ok := sourceErrorsOf(src, err)
		if !ok {
			return nil, err
		}
		return nil, SourceErrors(src, errs)
	}
	if invalid {
		return nil, SourceErrors(src, nil)
	}
	return FromSourceTree(src, f)
}

// ParsePartial parses src like Parse, but returns the grammar even when there
// are errors: the declarations that are complete. The parser recovers from a
// syntax error at the next declaration, so a declaration with an error in the
// middle is missing (see FromPartialSourceTree). Errors that the grammar does
// not catch, such as an invalid escape sequence, leave the declaration in the
// grammar. The errors are the syntax errors, then the errors of the checks on
// the declarations kept.
func ParsePartial(src string) (*grammar.Grammar, ErrorList) {
	g, errs, _ := ParsePartialContext(context.Background(), src)
	return g, errs
}

// ParsePartialContext is ParsePartial with cancellation during parsing.
// Cancelled calls return no grammar or diagnostics.
func ParsePartialContext(ctx context.Context, src string) (*grammar.Grammar, ErrorList, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	text, _ := WithoutInvalidCharacters(src)
	f, err := genparser.ParseAST(text, genparser.WithExpectations(), sourceLimits, genparser.WithContext(ctx))
	var errs []SourceError
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, ctxErr
		}
		var ok bool
		if errs, ok = sourceErrorsOf(src, err); !ok {
			return nil, nil, err
		}
	}
	list := SourceErrors(src, errs)
	g, checks := FromPartialSourceTree(src, f)
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	// The checks decode the literals that the lexer decoded for SourceErrors, so an invalid escape
	// or number is in both lists.
	reported := make(map[Error]bool, len(list))
	for _, e := range list {
		reported[*e] = true
	}
	for _, e := range checks {
		if !reported[*e] {
			list = append(list, e)
		}
	}
	return g, list, nil
}

// sourceErrorsOf returns the syntax errors that the generated parser reports
// in err for src, and whether err holds them. A source nested deeper than the
// depth limit of the parser is reported as "nesting too deep" at the opening
// delimiter where the nesting is deepest.
func sourceErrorsOf(src string, err error) ([]SourceError, bool) {
	var list genparser.SyntaxErrors
	var one *genparser.SyntaxError
	var limit *genparser.LimitError
	switch {
	case errors.As(err, &limit) && limit.Kind == genparser.LimitDepth:
		return []SourceError{{Offset: deepestOpening(src), Msg: "nesting too deep"}}, true
	case errors.As(err, &list):
		out := make([]SourceError, 0, len(list))
		for _, e := range list {
			out = append(out, SourceError{Offset: e.Pos, Msg: e.Message()})
		}
		return out, true
	case errors.As(err, &one):
		return []SourceError{{Offset: one.Pos, Msg: one.Message()}}, true
	}
	return nil, false
}

// deepestOpening returns the code point offset in src of the first opening
// parenthesis, bracket or brace at the greatest depth of nesting, or 0 if there
// is none.
func deepestOpening(src string) int {
	var lines []int // the offset of the start of each line
	lines = append(lines, 0)
	n := 0
	for _, r := range src {
		n++
		if r == '\n' {
			lines = append(lines, n)
		}
	}
	depth, deepest, at := 0, 0, 0
	for _, t := range lex(src, &errorList{}) {
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
			if depth > deepest {
				deepest, at = depth, lines[t.pos.Line-1]+t.pos.Col-1
			}
		case ")", "]", "}":
			depth--
		}
	}
	return at
}
