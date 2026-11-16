package tsgen

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
)

// TestLexTS checks that the lexer reads strings, the text of template literals, regular
// expressions and comments as single tokens, so that the braces and names in them do not count,
// while the code of a substitution does.
func TestLexTS(t *testing.T) {
	src := "const a = `x{${ {b: c} }y${d}}` + \"{\" + '}'; // {\n/* } */ const e = /[/}]x/g.test(f) ? g / h / i : j;\n"
	toks, err := lexTS(src)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, tk := range toks {
		if tk.ident {
			ids = append(ids, tk.text)
		}
		if tk.depth != 0 && tk.text != "}" && tk.text != "b" && tk.text != "c" && tk.text != ":" {
			t.Errorf("token %q at depth %d", tk.text, tk.depth)
		}
	}
	if got, want := strings.Join(ids, " "), "const a b c d const e test f g h i j"; got != want {
		t.Errorf("identifiers %q, want %q", got, want)
	}
	for _, bad := range []string{"a = {", "a = }", "a = `x", "a = \"x\nb\"", "a = /x\n/"} {
		if _, err := lexTS(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// shakeSrc is a library in miniature: a shaken class with methods of each kind, a public class,
// a declaration only a dropped method names and one only a type names.
const shakeSrc = `// A header comment.

export type Value = Node | number;

/** Node is public: its methods stay. */
export class Node {
  name = "";
  unused(): void {}
}

export class Parser {
  // pos is a field: fields stay with the class.
  pos = 0;
  constructor(readonly g: Grammar) {}
  // run is called by the parser's code.
  run(): Value {
    return this.step();
  }
  private step(): Value {
    return helper(this.pos);
  }
  // dropped is called by nothing.
  dropped(): void {
    onlyDropped(
      1,
    );
  }
}

export interface Grammar {
  rules: number;
}

function helper(n: number): Value {
  return n;
}

export function onlyDropped(n: number): void {
  console.log(n);
}

void [helper];
`

func TestShakeLib(t *testing.T) {
	out, err := shakeLib(shakeSrc, []string{"import {Parser} from './lib.ts';\nnew Parser(g).run();\n"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"// A header comment.", "export type Value", "export class Node", "unused(): void {}", "pos = 0;", "constructor(", "// run is called", "run(): Value", "private step()", "export interface Grammar", "function helper", "void [helper];"} {
		if !strings.Contains(out, want) {
			t.Errorf("the shaken library lacks %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"dropped", "onlyDropped"} {
		if strings.Contains(out, gone) {
			t.Errorf("the shaken library keeps %q:\n%s", gone, out)
		}
	}
	if _, err := shakeLib("export const a = 1, b = 2;\n", nil); err == nil {
		t.Error("a declaration of two names: no error")
	}
}

// narrowSrc has guards of each form: alone, with another conjunct, with an else block and with an
// else if, and a guard nested in a removed one.
const narrowSrc = `export const featureDocument = 1 << 0;
export const featureBudget = 1 << 3;
export const usedFeatures: number = allFeatures;
function f(d: D | null, c: C | null): number {
  if ((usedFeatures & featureDocument) !== 0 && d !== null) {
    if ((usedFeatures & featureBudget) !== 0) {
      nested();
    }
    return 1;
  }
  if ((usedFeatures & featureBudget) !== 0) {
    budget();
  } else {
    plain();
  }
  if (c === null) {
    a();
  } else if ((usedFeatures & featureDocument) !== 0) {
    doc();
  } else {
    b();
  }
  if ((usedFeatures & featureDocument) !== 0) {
    x();
  } else if (c !== null) {
    y();
  }
  return 0;
}
`

func TestNarrowLib(t *testing.T) {
	out, err := narrowLib(narrowSrc, codegen.FeatureSet(codegen.FeatureBudget))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "export const usedFeatures: number = 8; // budget\n") {
		t.Errorf("usedFeatures is not the set:\n%s", out)
	}
	for _, want := range []string{"budget();", "if (c === null) {\n    a();\n  } else {\n    b();\n  }", "  if (c !== null) {\n    y();"} {
		if !strings.Contains(out, want) {
			t.Errorf("the narrowed library lacks %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"nested", "return 1", "doc()", "x()"} {
		if strings.Contains(out, gone) {
			t.Errorf("the narrowed library keeps %q:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "plain();") {
		t.Errorf("the else branch of a true guard went:\n%s", out)
	}
	out, err = narrowLib(narrowSrc, codegen.FeatureSet(codegen.FeatureDocument))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "budget();") || !strings.Contains(out, "  {\n    plain();\n  }") || strings.Contains(out, "nested") || !strings.Contains(out, "return 1;") {
		t.Errorf("without budget:\n%s", out)
	}
}

// TestNarrowLibRejectsUnreadableGuards checks that a use of usedFeatures other than a conjunct
// (usedFeatures & featureX) !== 0 of an if condition is an error, rather than code left in place.
func TestNarrowLibRejectsUnreadableGuards(t *testing.T) {
	for _, body := range []string{
		"if ((usedFeatures & featureBudget) === 0) { a(); }",
		"if (a || (usedFeatures & featureBudget) !== 0) { a(); }",
		"if ((usedFeatures & featureNone) !== 0) { a(); }",
		"if ((usedFeatures & featureBudget) !== 0) a();",
		"const on = (usedFeatures & featureBudget) !== 0;",
	} {
		src := "export const usedFeatures: number = allFeatures;\nfunction f() {\n  " + body + "\n}\n"
		if _, err := narrowLib(src, 0); err == nil {
			t.Errorf("%s: no error", body)
		}
	}
}

// TestFeaturesMatchAnalysis checks that the feature bits of the TypeScript library are those of
// the generator's feature analysis.
func TestFeaturesMatchAnalysis(t *testing.T) {
	src, err := os.ReadFile("../../../codegen/ts/lib/lib.ts")
	if err != nil {
		t.Fatal(err)
	}
	decl := regexp.MustCompile(`(?m)^export const feature([A-Za-z]+) = 1 << (\d+);$`)
	got := map[string]int{}
	for _, m := range decl.FindAllStringSubmatch(string(src), -1) {
		n, _ := strconv.Atoi(m[2])
		got[strings.ToLower(m[1][:1])+m[1][1:]] = n
	}
	names := codegen.FeatureNames()
	if len(got) != len(names) {
		t.Errorf("the library declares %d features, the analysis has %d", len(got), len(names))
	}
	for i, name := range names {
		if bit, ok := got[name]; !ok || bit != i {
			t.Errorf("feature %s: the library's bit %d (declared %v), the analysis's %d", name, bit, ok, i)
		}
	}
	if !strings.Contains(string(src), "export const allFeatures = (1 << "+strconv.Itoa(len(names))+") - 1;\n") {
		t.Errorf("allFeatures is not every feature's bit")
	}
}
