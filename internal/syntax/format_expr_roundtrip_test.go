package syntax_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax"
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
		if g, ok := v.Interface().(*grammar.Group); ok {
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

func lit(s string) grammar.Expr { return &grammar.Literal{Value: s} }

// operands and postfix operators built by code, as a decoded JSON grammar or an
// API user builds them: without the Group nodes that parsing parentheses makes.
func operandKinds() map[string]grammar.Expr {
	a, b := func() grammar.Expr { return &grammar.Ref{Name: "a"} }, func() grammar.Expr { return &grammar.Ref{Name: "b"} }
	attr := func(name string, pragma bool) []*grammar.Attribute {
		return []*grammar.Attribute{{Name: name, Pragma: pragma}}
	}
	return map[string]grammar.Expr{
		"ref":        a(),
		"seq":        &grammar.Seq{Items: []grammar.Expr{a(), b()}},
		"choice":     &grammar.Choice{Alts: []grammar.Expr{a(), b()}},
		"capture":    &grammar.Capture{Name: "x", Expr: a()},
		"and":        &grammar.And{Expr: a()},
		"not":        &grammar.Not{Expr: a()},
		"discard":    &grammar.Discard{Expr: a()},
		"star":       &grammar.Repeat{Expr: a(), Min: 0, Max: -1},
		"plus":       &grammar.Repeat{Expr: a(), Min: 1, Max: -1},
		"repeat":     &grammar.Repeat{Expr: a(), Min: 2, Max: 3},
		"optional":   &grammar.Optional{Expr: a()},
		"atomic":     &grammar.Atomic{Expr: a()},
		"stream":     &grammar.Stream{Expr: a()},
		"recover":    &grammar.Recovery{Expr: a(), Skip: b()},
		"attributed": &grammar.Attributed{Expr: a(), Attrs: attr("error", false)},
		"ignorecase": &grammar.Attributed{Expr: lit("a"), Attrs: attr("ignorecase", true)},
		"missing":    &grammar.Attributed{Expr: a(), Attrs: attr("missing", true)},
		"two attrs":  &grammar.Attributed{Expr: a(), Attrs: append(attr("error", false), attr("note", false)...)},
	}
}

var postfixKinds = map[string]func(grammar.Expr) grammar.Expr{
	"*":        func(e grammar.Expr) grammar.Expr { return &grammar.Repeat{Expr: e, Min: 0, Max: -1} },
	"+":        func(e grammar.Expr) grammar.Expr { return &grammar.Repeat{Expr: e, Min: 1, Max: -1} },
	"?":        func(e grammar.Expr) grammar.Expr { return &grammar.Optional{Expr: e} },
	"#repeat":  func(e grammar.Expr) grammar.Expr { return &grammar.Repeat{Expr: e, Min: 2, Max: 3} },
	"#atomic":  func(e grammar.Expr) grammar.Expr { return &grammar.Atomic{Expr: e} },
	"#stream":  func(e grammar.Expr) grammar.Expr { return &grammar.Stream{Expr: e} },
	"#recover": func(e grammar.Expr) grammar.Expr { return &grammar.Recovery{Expr: e, Skip: &grammar.Ref{Name: "s"}} },
	"@attribute": func(e grammar.Expr) grammar.Expr {
		return &grammar.Attributed{Expr: e, Attrs: []*grammar.Attribute{{Name: "warn"}}}
	},
	"#ignorecase": func(e grammar.Expr) grammar.Expr {
		return &grammar.Attributed{Expr: e, Attrs: []*grammar.Attribute{{Name: "ignorecase", Pragma: true}}}
	},
}

// Every expression built from an operand kind under each postfix operator
// prints as text that parses back to the same expression.
func TestFormatExprPostfixOperandRoundTrip(t *testing.T) {
	for opName, wrap := range postfixKinds {
		for kindName := range operandKinds() {
			t.Run(opName+" of "+kindName, func(t *testing.T) {
				e := wrap(operandKinds()[kindName])
				text := grammar.FormatExpr(e)
				back, err := syntax.ParseExpr(text)
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
			for ctxName, ctx := range map[string]func(grammar.Expr) grammar.Expr{
				"not":     func(e grammar.Expr) grammar.Expr { return &grammar.Not{Expr: e} },
				"capture": func(e grammar.Expr) grammar.Expr { return &grammar.Capture{Name: "x", Expr: e} },
				"discard": func(e grammar.Expr) grammar.Expr { return &grammar.Discard{Expr: e} },
				"seq": func(e grammar.Expr) grammar.Expr {
					return &grammar.Seq{Items: []grammar.Expr{&grammar.Ref{Name: "p"}, e, &grammar.Ref{Name: "q"}}}
				},
				"choice": func(e grammar.Expr) grammar.Expr {
					return &grammar.Choice{Alts: []grammar.Expr{&grammar.Ref{Name: "p"}, e}}
				},
			} {
				t.Run(ctxName+" of "+opName+" of "+kindName, func(t *testing.T) {
					e := ctx(wrap(operandKinds()[kindName]))
					text := grammar.FormatExpr(e)
					back, err := syntax.ParseExpr(text)
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
	for _, pattern := range []string{"../../testdata/grammars/*/*.zp", "../../examples/*/*.zp", "../../zp.zp"} {
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
			g, err := syntax.Parse(string(src))
			if err != nil {
				t.Skipf("does not parse on its own: %v", err)
			}
			text := grammar.Format(g)
			g2, err := syntax.Parse(text)
			if err != nil {
				t.Fatalf("printed grammar does not parse: %v", err)
			}
			if got, want := shape(g2), shape(g); got != want {
				t.Errorf("printed grammar parses back differently")
			}
			if again := grammar.Format(g2); again != text {
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
		e, err := syntax.ParseExpr(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := grammar.FormatExpr(e); got != src {
			t.Errorf("FormatExpr(%s) = %q", src, got)
		}
	}
}

// Prefix operators, sequences and choices of every operand kind, and of each
// other, print as text that parses back to the same expression.
func TestFormatExprPrefixOperandRoundTrip(t *testing.T) {
	ctxs := map[string]func(grammar.Expr) grammar.Expr{
		"not":     func(e grammar.Expr) grammar.Expr { return &grammar.Not{Expr: e} },
		"and":     func(e grammar.Expr) grammar.Expr { return &grammar.And{Expr: e} },
		"capture": func(e grammar.Expr) grammar.Expr { return &grammar.Capture{Name: "x", Expr: e} },
		"discard": func(e grammar.Expr) grammar.Expr { return &grammar.Discard{Expr: e} },
		"seq": func(e grammar.Expr) grammar.Expr {
			return &grammar.Seq{Items: []grammar.Expr{e, &grammar.Ref{Name: "q"}}}
		},
		"choice": func(e grammar.Expr) grammar.Expr {
			return &grammar.Choice{Alts: []grammar.Expr{&grammar.Ref{Name: "p"}, e}}
		},
	}
	for ctxName, ctx := range ctxs {
		for kindName := range operandKinds() {
			t.Run(ctxName+" of "+kindName, func(t *testing.T) {
				e := ctx(operandKinds()[kindName])
				text := grammar.FormatExpr(e)
				back, err := syntax.ParseExpr(text)
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
	inner := func() *grammar.Attributed {
		return &grammar.Attributed{Expr: &grammar.Ref{Name: "a"}, Attrs: []*grammar.Attribute{{Name: "x"}}}
	}
	for _, tc := range []struct {
		expr grammar.Expr
		want string
	}{
		{&grammar.Attributed{Expr: inner(), Attrs: []*grammar.Attribute{{Name: "y"}}}, "(a @x) @y"},
		{&grammar.And{Expr: &grammar.And{Expr: &grammar.Ref{Name: "a"}}}, "& &a"},
		{&grammar.Not{Expr: &grammar.Not{Expr: &grammar.Ref{Name: "a"}}}, "!!a"},
		{&grammar.Repeat{Expr: inner(), Min: 0, Max: -1}, "a @x*"},
		{&grammar.Atomic{Expr: inner()}, "a @x #atomic"},
		{&grammar.Attributed{Expr: &grammar.Atomic{Expr: inner()}, Attrs: []*grammar.Attribute{{Name: "y"}}}, "a @x #atomic @y"},
	} {
		if got := grammar.FormatExpr(tc.expr); got != tc.want {
			t.Errorf("FormatExpr = %q, want %q", got, tc.want)
		}
	}
}
