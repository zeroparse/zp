package parser_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/frontend/parser"
	syntaxfmt "zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// shape returns the structure of v: node kinds and semantic fields, without
// source positions, layout (fields tagged json:"-") or parenthesized groups,
// which do not change what an expression means.
func shape(v any) string {
	var b strings.Builder
	writeShape(&b, reflect.ValueOf(v))
	return b.String()
}

func writeShape(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		if g, ok := v.Interface().(*ast.Group); ok {
			writeShape(b, reflect.ValueOf(g.Expr))
			return
		}
		writeShape(b, v.Elem())
	case reflect.Struct:
		b.WriteString(v.Type().Name() + "{")
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.Name == "SourceInfo" || f.Tag.Get("json") == "-" || !f.IsExported() {
				continue
			}
			b.WriteString(f.Name + ":")
			writeShape(b, v.Field(i))
			b.WriteString(" ")
		}
		b.WriteString("}")
	case reflect.Slice:
		b.WriteString("[")
		for i := 0; i < v.Len(); i++ {
			writeShape(b, v.Index(i))
			b.WriteString(",")
		}
		b.WriteString("]")
	default:
		fmt.Fprintf(b, "%v", v.Interface())
	}
}

func lit(s string) ast.Expr { return &ast.Literal{Value: s} }

// operands and postfix operators built by code, as a decoded JSON grammar or an
// API user builds them: without the Group nodes that parsing parentheses makes.
func operandKinds() map[string]ast.Expr {
	a, b := func() ast.Expr { return &ast.Ref{Name: "a"} }, func() ast.Expr { return &ast.Ref{Name: "b"} }
	attr := func(name string, pragma bool) []*ast.Attribute {
		return []*ast.Attribute{{Name: name, Pragma: pragma}}
	}
	return map[string]ast.Expr{
		"ref":        a(),
		"seq":        &ast.Seq{Items: []ast.Expr{a(), b()}},
		"choice":     &ast.Choice{Alts: []ast.Expr{a(), b()}},
		"capture":    &ast.Capture{Name: "x", Expr: a()},
		"and":        &ast.And{Expr: a()},
		"not":        &ast.Not{Expr: a()},
		"discard":    &ast.Discard{Expr: a()},
		"star":       &ast.Repeat{Expr: a(), Min: 0, Max: -1},
		"plus":       &ast.Repeat{Expr: a(), Min: 1, Max: -1},
		"repeat":     &ast.Repeat{Expr: a(), Min: 2, Max: 3},
		"optional":   &ast.Optional{Expr: a()},
		"atomic":     &ast.Atomic{Expr: a()},
		"stream":     &ast.Stream{Expr: a()},
		"recover":    &ast.Recovery{Expr: a(), Skip: b()},
		"attributed": &ast.Attributed{Expr: a(), Attrs: attr("error", false)},
		"ignorecase": &ast.Attributed{Expr: lit("a"), Attrs: attr("ignorecase", true)},
		"missing":    &ast.Attributed{Expr: a(), Attrs: attr("missing", true)},
		"two attrs":  &ast.Attributed{Expr: a(), Attrs: append(attr("error", false), attr("note", false)...)},
	}
}

var postfixKinds = map[string]func(ast.Expr) ast.Expr{
	"*":        func(e ast.Expr) ast.Expr { return &ast.Repeat{Expr: e, Min: 0, Max: -1} },
	"+":        func(e ast.Expr) ast.Expr { return &ast.Repeat{Expr: e, Min: 1, Max: -1} },
	"?":        func(e ast.Expr) ast.Expr { return &ast.Optional{Expr: e} },
	"#repeat":  func(e ast.Expr) ast.Expr { return &ast.Repeat{Expr: e, Min: 2, Max: 3} },
	"#atomic":  func(e ast.Expr) ast.Expr { return &ast.Atomic{Expr: e} },
	"#stream":  func(e ast.Expr) ast.Expr { return &ast.Stream{Expr: e} },
	"#recover": func(e ast.Expr) ast.Expr { return &ast.Recovery{Expr: e, Skip: &ast.Ref{Name: "s"}} },
	"@attribute": func(e ast.Expr) ast.Expr {
		return &ast.Attributed{Expr: e, Attrs: []*ast.Attribute{{Name: "warn"}}}
	},
	"#ignorecase": func(e ast.Expr) ast.Expr {
		return &ast.Attributed{Expr: e, Attrs: []*ast.Attribute{{Name: "ignorecase", Pragma: true}}}
	},
}

// Every expression built from an operand kind under each postfix operator
// prints as text that parses back to the same expression.
func TestFormatExprPostfixOperandRoundTrip(t *testing.T) {
	for opName, wrap := range postfixKinds {
		for kindName := range operandKinds() {
			t.Run(opName+" of "+kindName, func(t *testing.T) {
				e := wrap(operandKinds()[kindName])
				text := syntaxfmt.FormatExpr(e)
				back, err := parser.ParseExpr(text)
				if err != nil {
					t.Fatalf("%q does not parse: %v", text, err)
				}
				if got, want := shape(back), shape(e); got != want {
					t.Errorf("%q parses back differently:\n got %s\nwant %s", text, got, want)
				}
			})
		}
	}
}

// The same holds one level deeper, with the postfix operand itself an operand
// of a prefix operator, a sequence or a choice.
func TestFormatExprNestedRoundTrip(t *testing.T) {
	for opName, wrap := range postfixKinds {
		for kindName := range operandKinds() {
			for ctxName, ctx := range map[string]func(ast.Expr) ast.Expr{
				"not":     func(e ast.Expr) ast.Expr { return &ast.Not{Expr: e} },
				"capture": func(e ast.Expr) ast.Expr { return &ast.Capture{Name: "x", Expr: e} },
				"discard": func(e ast.Expr) ast.Expr { return &ast.Discard{Expr: e} },
				"seq": func(e ast.Expr) ast.Expr {
					return &ast.Seq{Items: []ast.Expr{&ast.Ref{Name: "p"}, e, &ast.Ref{Name: "q"}}}
				},
				"choice": func(e ast.Expr) ast.Expr {
					return &ast.Choice{Alts: []ast.Expr{&ast.Ref{Name: "p"}, e}}
				},
			} {
				t.Run(ctxName+" of "+opName+" of "+kindName, func(t *testing.T) {
					e := ctx(wrap(operandKinds()[kindName]))
					text := syntaxfmt.FormatExpr(e)
					back, err := parser.ParseExpr(text)
					if err != nil {
						t.Fatalf("%q does not parse: %v", text, err)
					}
					if got, want := shape(back), shape(e); got != want {
						t.Errorf("%q parses back differently:\n got %s\nwant %s", text, got, want)
					}
				})
			}
		}
	}
}

// The bundled grammars print and parse back to the same grammar, and again
// print the same text.
func TestFormatBundledGrammarsRoundTrip(t *testing.T) {
	var paths []string
	for _, pattern := range []string{"../../../testdata/grammars/*/*.zp", "../../../examples/*/*.zp", "../../../zp.zp"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, m...)
	}
	if len(paths) < 10 {
		t.Fatalf("found only %d bundled grammars", len(paths))
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := parser.Parse(string(src))
			if err != nil {
				t.Skipf("does not parse on its own: %v", err)
			}
			text := syntaxfmt.Format(g)
			g2, err := parser.Parse(text)
			if err != nil {
				t.Fatalf("printed grammar does not parse: %v", err)
			}
			if got, want := shape(g2), shape(g); got != want {
				t.Errorf("printed grammar parses back differently")
			}
			if again := syntaxfmt.Format(g2); again != text {
				t.Errorf("printing is not stable")
			}
		})
	}
}

// Parentheses are added only where the operand binds weaker than the operator.
// The sources hold no parentheses, so the parsed expressions hold no Group.
func TestFormatExprNoUnneededParentheses(t *testing.T) {
	for _, src := range []string{
		`a #atomic*`, `a @error("m")*`, `a #atomic @warn("m")`, `a* #atomic`, `a? #stream`,
		`a #repeat(2, 3)*`, `a #recover(skip=b)+`, `!a #atomic`, `x:a #atomic`, `-a*`, `a b #atomic / c`,
		`a #ignorecase*`, `a @error("m") #atomic?`,
	} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := syntaxfmt.FormatExpr(e); got != src {
			t.Errorf("FormatExpr(%s) = %q", src, got)
		}
	}
}

// Prefix operators, sequences and choices of every operand kind, and of each
// other, print as text that parses back to the same expression.
func TestFormatExprPrefixOperandRoundTrip(t *testing.T) {
	ctxs := map[string]func(ast.Expr) ast.Expr{
		"not":     func(e ast.Expr) ast.Expr { return &ast.Not{Expr: e} },
		"and":     func(e ast.Expr) ast.Expr { return &ast.And{Expr: e} },
		"capture": func(e ast.Expr) ast.Expr { return &ast.Capture{Name: "x", Expr: e} },
		"discard": func(e ast.Expr) ast.Expr { return &ast.Discard{Expr: e} },
		"seq": func(e ast.Expr) ast.Expr {
			return &ast.Seq{Items: []ast.Expr{e, &ast.Ref{Name: "q"}}}
		},
		"choice": func(e ast.Expr) ast.Expr {
			return &ast.Choice{Alts: []ast.Expr{&ast.Ref{Name: "p"}, e}}
		},
	}
	for ctxName, ctx := range ctxs {
		for kindName := range operandKinds() {
			t.Run(ctxName+" of "+kindName, func(t *testing.T) {
				e := ctx(operandKinds()[kindName])
				text := syntaxfmt.FormatExpr(e)
				back, err := parser.ParseExpr(text)
				if err != nil {
					t.Fatalf("%q does not parse: %v", text, err)
				}
				if got, want := shape(back), shape(e); got != want {
					t.Errorf("%q parses back differently:\n got %s\nwant %s", text, got, want)
				}
			})
		}
	}
}

// An attributed operand of an attribute needs parentheses; the same operand of
// any other operator does not.
func TestFormatExprNestedAttributed(t *testing.T) {
	inner := func() *ast.Attributed {
		return &ast.Attributed{Expr: &ast.Ref{Name: "a"}, Attrs: []*ast.Attribute{{Name: "x"}}}
	}
	for _, tc := range []struct {
		expr ast.Expr
		want string
	}{
		{&ast.Attributed{Expr: inner(), Attrs: []*ast.Attribute{{Name: "y"}}}, "(a @x) @y"},
		{&ast.And{Expr: &ast.And{Expr: &ast.Ref{Name: "a"}}}, "& &a"},
		{&ast.Not{Expr: &ast.Not{Expr: &ast.Ref{Name: "a"}}}, "!!a"},
		{&ast.Repeat{Expr: inner(), Min: 0, Max: -1}, "a @x*"},
		{&ast.Atomic{Expr: inner()}, "a @x #atomic"},
		{&ast.Attributed{Expr: &ast.Atomic{Expr: inner()}, Attrs: []*ast.Attribute{{Name: "y"}}}, "a @x #atomic @y"},
	} {
		if got := syntaxfmt.FormatExpr(tc.expr); got != tc.want {
			t.Errorf("FormatExpr = %q, want %q", got, tc.want)
		}
	}
}
