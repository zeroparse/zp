package syntax

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/genparser"
	"zeroparse.dev/zp/internal/syntax/sourceast"
)

// sourceLimits are the limits of the source parser. The lexer rejects a source that nests deeper
// than maxLevels; the parser keeps its default bound on nested recursive rule calls, which is far
// above what maxLevels allows, so that its recursion stays bounded on a source that the lexer
// rejects. Compute is unlimited: the recovery of the grammar is linear, and a caller stops a long
// parse with its context.
var sourceLimits = genparser.WithLimits(genparser.Limits{Compute: genparser.Unlimited})

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
// in err for src, and whether err holds them. A source that the parser cannot
// nest has no syntax errors to report: the lexer reports it as nesting too
// deep, and SourceErrors adds the lexical errors. Should the lexer not, the
// source is reported as nesting too deep at its deepest opening delimiter.
func sourceErrorsOf(src string, err error) ([]SourceError, bool) {
	var list genparser.SyntaxErrors
	var one *genparser.SyntaxError
	var limit *genparser.LimitError
	switch {
	case errors.As(err, &limit) && limit.Kind == genparser.LimitDepth:
		lexical := &errorList{}
		lex(src, lexical)
		for _, e := range lexical.list {
			if e.Msg == msgNestingTooDeep {
				return nil, true
			}
		}
		return []SourceError{{Offset: deepestOpening(src), Msg: msgNestingTooDeep}}, true
	case errors.As(err, &list):
		m := newMessages(src)
		out := make([]SourceError, 0, len(list))
		for _, e := range list {
			out = append(out, SourceError{Offset: e.Pos, Msg: m.message(e)})
		}
		return out, true
	case errors.As(err, &one):
		return []SourceError{{Offset: one.Pos, Msg: newMessages(src).message(one)}}, true
	}
	return nil, false
}

// trivia are the names that the grammar of the source gives to white space and to the start of a
// comment. Either may stand between any two tokens, so naming them in what a syntax error expects
// tells nothing.
var trivia = map[string]bool{"whitespace": true, "comment": true}

// starts are the expected items that the name of an operand of the grammar of the source covers:
// the prefix operators, and the "(" of a lambda or the name of a capture, which start an
// expression or a value as well.
var starts = map[string]map[string]bool{
	"an expression": {`"!"`: true, `"&"`: true, `"-"`: true, `[a-zA-Z_]`: true},
	"a value":       {`"!"`: true, `"-"`: true, `"("`: true},
}

// typeSpellings word the errors of a type definition in a spelling that the language no longer
// has, which the grammar of the source gives without the name of the type, with that name.
var typeSpellings = map[string]string{
	"a struct type is defined as type Name = { ... }":                        "a struct type is defined as type %[1]s = {...}; run zp fix to rewrite type %[1]s struct",
	"a terminal type is the struct type Name = { Text string }":              "a terminal type is the struct type %[1]s = { Text string }; run zp fix to rewrite type %[1]s terminal",
	"expected '{' or 'node[' after '='; write type Name ~ Type for an alias": "expected '{' or 'node[' after '='; write type %[1]s ~ <type> for an alias (zp fix rewrites type %[1]s = <type>)",
}

// messages words the syntax errors of the generated parser for a source, with the tokens of the
// lexer around each error.
type messages struct {
	toks []token
	pos  *sourceast.Positions
}

func newMessages(src string) *messages {
	return &messages{toks: lex(src, &errorList{}), pos: sourceast.NewPositions(src)}
}

// message is the message of a syntax error e. Without explicit messages, it lists what e expects,
// without the trivia, and with the items that a named operand covers left to the name. A message
// that says only what is expected also says what was found: the token at the error. An error after the
// name of a pragma that does not exist names it, and an expected value on the line after "=>", or
// after an operator where a line break ends a statement, is the mistake of a value that does not start on the line of
// its operator. The errors of a spelling that
// the language no longer has name what to write and what zp fix rewrites.
func (m *messages) message(e *genparser.SyntaxError) string {
	var msg string
	if len(e.Messages) > 0 {
		msg = e.Message()
	} else {
		covered := map[string]bool{}
		for _, x := range e.Expected {
			for x := range starts[x] {
				covered[x] = true
			}
		}
		kept := &genparser.SyntaxError{}
		for _, x := range e.Expected {
			if !trivia[x] && !covered[x] {
				kept.Expected = append(kept.Expected, x)
			}
		}
		msg = strings.TrimPrefix(kept.Message(), "syntax error: ")
	}
	i := m.tokenAt(e.Pos)
	switch {
	case typeSpellings[msg] != "" && i >= 1 && m.toks[i-1].kind == tIdent:
		return fmt.Sprintf(typeSpellings[msg], m.toks[i-1].text)
	case strings.HasPrefix(msg, "positional captures are removed") && m.toks[i].kind == tIndex:
		return fmt.Sprintf("%s; run zp fix to rewrite $%d", msg, m.toks[i].num)
	case msg == "unknown pragma" && i >= 2 && m.toks[i-1].kind == tIdent && m.toks[i-2].kind == tPunct && m.toks[i-2].text == "#":
		return msg + " #" + m.toks[i-1].text
	case strings.HasPrefix(msg, "expected a value") && i >= 1 && m.toks[i].lineBefore && m.toks[i].kind != tEOF &&
		m.toks[i-1].kind == tPunct && (m.toks[i-1].text == "=>" || m.lineEndsStatement(i)):
		return "multiline expressions require their own open delimiter"
	case expects(msg):
		return msg + ", found " + m.toks[i].String()
	}
	return msg
}

// lineEndsStatement reports whether a line break before the token i ends a statement: no
// parenthesis or bracket is open there, or a brace is the innermost open delimiter.
func (m *messages) lineEndsStatement(i int) bool {
	var open []string
	for _, t := range m.toks[:i] {
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{":
				open = append(open, t.text)
			case ")", "]", "}":
				if len(open) > 0 {
					open = open[:len(open)-1]
				}
			}
		}
	}
	return len(open) == 0 || open[len(open)-1] == "{"
}

// expects reports whether every part of msg says what is expected, so that what was found may
// follow it. A part that gives a hint does not.
func expects(msg string) bool {
	for part := range strings.SplitSeq(msg, "; ") {
		if !strings.HasPrefix(part, "expected ") {
			return false
		}
	}
	return true
}

// tokenAt returns the index of the token at or after the code point offset i.
func (m *messages) tokenAt(i int) int {
	at := m.pos.At(i)
	return sort.Search(len(m.toks)-1, func(k int) bool {
		p := m.toks[k].pos
		return p.Line > at.Line || p.Line == at.Line && p.Col >= at.Col
	})
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
