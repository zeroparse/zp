package tsgen

import (
	"strings"
	"testing"
)

// TestSubstituteMethods checks that the printed compositions replace the hand-written methods of
// the library, with the comment lines before them, and nothing else.
func TestSubstituteMethods(t *testing.T) {
	ms, err := tsMethods()
	if err != nil {
		t.Fatal(err)
	}
	got := substituteMethods(readLib(t, tsLibFileName), ms)
	want := readLib(t, tsLibFileName)
	for _, m := range ms {
		if strings.Count(got, m.Text) != 1 {
			t.Fatalf("the printed method %s is not in the library once", m.Name)
		}
		got = strings.Replace(got, m.Text, "<"+m.Name+">", 1)
		// The hand-written method: its comment lines, its first line and its closing brace.
		i := strings.Index(want, "\n  "+m.Name+"(") + 1
		start := i
		for {
			j := strings.LastIndex(want[:start-1], "\n") + 1
			if !strings.HasPrefix(want[j:], "  // ") {
				break
			}
			start = j
		}
		end := i + strings.Index(want[i:], "\n  }\n") + len("\n  }\n")
		want = want[:start] + "<" + m.Name + ">" + want[end:]
	}
	if got != want {
		t.Error("the substitution changed the library outside the replaced methods")
	}
	if strings.Contains(got, "  callBody(") || strings.Contains(got, "  grow(") {
		t.Error("a hand-written method is left")
	}
}

// TestSubstituteMethodsRefusesMalformedLib checks that a library without the method, with it
// twice, or without its closing brace at the class body's indentation stops the substitution.
func TestSubstituteMethodsRefusesMalformedLib(t *testing.T) {
	m := []tsMethod{{Name: "grow", Text: "  grow() {}\n"}}
	for name, src := range map[string]string{
		"missing":          "class P {\n  other() {\n  }\n}\n",
		"twice":            "class P {\n  grow() {\n  }\n  grow() {\n  }\n}\n",
		"no closing brace": "class P {\n  grow() {\n    return 1;\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("no panic")
				}
			}()
			substituteMethods(src, m)
		})
	}
}

// TestPrintedMethodsHaveNoConstantTest checks that a condition the TypeScript library prints as a
// constant leaves no test in the printed methods.
func TestPrintedMethodsHaveNoConstantTest(t *testing.T) {
	ms, err := tsMethods()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		for _, c := range []string{"if (true)", "if (false)"} {
			if strings.Contains(m.Text, c) {
				t.Errorf("method %s has %q:\n%s", m.Name, c, m.Text)
			}
		}
	}
}

// TestPrintedMethodsVerify checks that the functions verify for every specialization the
// TypeScript library prints, and that each printed method defines its name.
func TestPrintedMethodsVerify(t *testing.T) {
	ms, err := tsMethods()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range ms {
		if !strings.Contains(x.Text, x.Name+"(") {
			t.Errorf("method %s prints as\n%s", x.Name, x.Text)
		}
	}
}
