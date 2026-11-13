package tsgen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

const tsBudgetFormsGrammar = `type Item = { Name string }
rule main = xs:item* !.
rule item Item = n:name ","? => Item{Name: text($n)}
rule name = [a-z]+
rule fast = "z"`

// The generated TypeScript has two forms: without -limits the rules do not spend budget and the
// entry points reject the control options; with it they charge and accept them.
func TestGeneratedTSBudgetControlForms(t *testing.T) {
	t.Parallel()
	g, err := parseTest(tsBudgetFormsGrammar)
	if err != nil {
		t.Fatal(err)
	}
	generate := func(controls bool) string {
		files, err := gentest.GenerateTS(g, gen.GenOptions{Start: "main", Types: true, Recognize: true, LimitControls: controls})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if f.Path == "parser.ts" {
				return string(f.Content)
			}
		}
		t.Fatal("no parser.ts")
		return ""
	}
	plain, controlled := generate(false), generate(true)
	for _, c := range []struct{ name, text string }{{"plain", plain}, {"controlled", controlled}} {
		for _, entry := range []string{"parseWithOptions(", "parseRuleWithOptions(", "recognizeWithOptions(", "parseValueWithOptions("} {
			if !strings.Contains(c.text, "function "+entry) {
				t.Errorf("%s form has no %s", c.name, entry)
			}
		}
	}
	for _, charge := range []string{"p.spend(", "c.spend(", "charge("} {
		if strings.Contains(plain, charge) {
			t.Errorf("a parser generated without limit controls mentions %q:\n%s", charge, plain)
		}
	}
	for _, charge := range []string{"p.spend(", "c.spend("} {
		if !strings.Contains(controlled, charge) {
			t.Errorf("a parser generated with limit controls has no %q", charge)
		}
	}
	if strings.Contains(plain, "CancellationSignal") {
		t.Error("a parser generated without limit controls exports the cancellation signal type")
	}
	for _, name := range []string{"ParseInterruptionError", "ParseInterruptionKind", "Unlimited"} {
		if !strings.Contains(plain, name) || !strings.Contains(controlled, name) {
			t.Errorf("a form of the parser does not export %s, which the depth limit needs", name)
		}
	}
	if !strings.Contains(plain, "PlainLimits as Limits") || strings.Contains(controlled, "PlainLimits") {
		t.Error("the limits type of a form does not match its controls")
	}
	if !strings.Contains(plain, "PlainParseOptions as ParseOptions") || !strings.Contains(plain, "options?: PlainParseOptions") {
		t.Error("a parser generated without limit controls has the full options type")
	}
	if strings.Contains(controlled, "PlainParseOptions") || !strings.Contains(controlled, "CancellationSignal") || !strings.Contains(controlled, "options?: ParseOptions") {
		t.Error("a parser generated with limit controls does not have the full options type")
	}
	if !strings.Contains(plain, "mkGrammar(") || !strings.Contains(plain, ", false);") || !strings.Contains(controlled, ", true);") {
		t.Error("the grammar does not record whether the parser is controlled")
	}
	if len(plain) >= len(controlled) {
		t.Errorf("the plain parser (%d bytes) is not smaller than the controlled one (%d bytes)", len(plain), len(controlled))
	}
}

func TestGeneratedTSWithoutBudgetControlsRejectsControlOptions(t *testing.T) {
	t.Parallel()
	consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
const input = "ab,cd";
check(p.parse(input).error === null, "parse");
for (const unit of [p.CodePoints, p.Bytes]) {
  const ordinary = p.parse(input, unit), typed = p.parseValue(input, unit);
  const o = { unit, limits: { depth: 0 } };
  check(stable(p.parseWithOptions(input, o).node) === stable(ordinary.node), "options without controls parse as parse");
  check(stable(p.parseValueWithOptions(input, o).value) === stable(typed.value), "typed options without controls");
  check(p.parseRuleWithOptions("fast", "z", o).error === null && p.recognizeWithOptions(input, o) === null, "rule and recognizer options");
  // The options type has no control option; a caller that bypasses it still gets an error.
  // @ts-expect-error The options of a parser without limit controls have no signal.
  const withSignal = p.parseWithOptions(input, { unit, signal: { aborted: false } });
  // @ts-expect-error The options of a parser without limit controls have no budget limit.
  const withBudget = p.parseValueWithOptions(input, { unit, limits: { compute: 1 } });
  // @ts-expect-error The options of a parser without limit controls have no budget limit.
  const withRecognize = p.recognizeWithOptions(input, { unit, limits: { compute: 1 } });
  for (const error of [withSignal.error, withBudget.error, withRecognize]) {
    check(error !== null && error.message.includes("generated without limit controls"), "a control option is rejected");
  }
  const rejected = [{ signal: { aborted: false } }, { limits: { compute: 1 } }, { limits: { memory: 1 } }] as unknown as p.ParseOptions[];
  for (const options of rejected) {
    for (const error of [p.parseWithOptions(input, options).error, p.parseRuleWithOptions("fast", "z", options).error, p.recognizeWithOptions(input, options), p.parseValueWithOptions(input, options).error]) {
      check(error !== null && error.message.includes("generated without limit controls"), "a control option is rejected by every entry point: " + JSON.stringify(options));
    }
  }
  // Zero limits and an absent signal set nothing.
  check(p.parseWithOptions(input, { unit, signal: null, limits: { compute: 0 } } as unknown as p.ParseOptions).error === null, "unset controls are accepted");
  // The input limit needs no control: it is checked against the length of the input.
  const short = { unit, limits: { input: 4 } };
  for (const error of [p.parseWithOptions(input, short).error, p.parseRuleWithOptions("main", input, short).error, p.recognizeWithOptions(input, short), p.parseValueWithOptions(input, short).error]) {
    check(error instanceof p.ParseInterruptionError && error.kind === "input-limit" && error.limit === 4 && error.requested === 5, "the input limit without controls");
  }
  check(p.parseWithOptions(input, { unit, limits: { input: 5 } }).error === null, "an input as long as the limit parses");
  // A removed or misspelled option is an error, not ignored.
  const unknown: [object, RegExp][] = [[{ budget: 1 }, /^unknown parse option: budget$/], [{ maxInput: 1 }, /^unknown parse option: maxInput$/],
    [{ maxDepth: 1 }, /^unknown parse option: maxDepth$/], [{ limits: { bytes: 1 } }, /^unknown limit: bytes$/]];
  for (const [options, want] of unknown) {
    const o = { unit, ...options } as unknown as p.ParseOptions;
    for (const error of [p.parseWithOptions(input, o).error, p.parseRuleWithOptions("fast", "z", o).error, p.recognizeWithOptions(input, o), p.parseValueWithOptions(input, o).error]) {
      check(error !== null && want.test(error.message), "unknown option " + JSON.stringify(options) + ": " + error?.message);
    }
  }
}
`
	runTypedTSFixture(t, tsBudgetFormsGrammar, consumer, gen.GenOptions{Start: "main", Recognize: true}, "")
}

func TestGeneratedTSWithBudgetControlsAcceptsControlOptions(t *testing.T) {
	t.Parallel()
	consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
const input = "ab,cd";
for (const unit of [p.CodePoints, p.Bytes]) {
  const ordinary = p.parse(input, unit), typed = p.parseValue(input, unit);
  const o = { unit, signal: { aborted: false }, limits: { compute: 1_000_000 } };
  check(stable(p.parseWithOptions(input, o).node) === stable(ordinary.node), "controlled parse matches");
  check(stable(p.parseValueWithOptions(input, o).value) === stable(typed.value), "controlled typed parse matches");
  check(p.recognizeWithOptions(input, o) === null && p.parseRuleWithOptions("fast", "z", o).error === null, "controlled rule and recognizer");
  const limited = p.parseWithOptions(input, { unit, limits: { compute: 3 } });
  check(limited.error instanceof p.ParseInterruptionError && limited.error.kind === "compute-limit", "compute-limit");
  const canceled = p.parseValueWithOptions(input, { unit, signal: { aborted: true } });
  check(canceled.error instanceof p.ParseInterruptionError && canceled.error.kind === "canceled", "cancellation");
  check(p.parse(input, unit).error === null, "a parse after an interruption");
  // A removed or misspelled option is an error, not ignored.
  const unknown: [object, RegExp][] = [[{ budget: 1 }, /^unknown parse option: budget$/], [{ maxInput: 1 }, /^unknown parse option: maxInput$/],
    [{ maxDepth: 1 }, /^unknown parse option: maxDepth$/], [{ limits: { bytes: 1 } }, /^unknown limit: bytes$/]];
  for (const [options, want] of unknown) {
    const o = { unit, ...options } as unknown as p.ParseOptions;
    for (const error of [p.parseWithOptions(input, o).error, p.parseRuleWithOptions("fast", "z", o).error, p.recognizeWithOptions(input, o), p.parseValueWithOptions(input, o).error]) {
      check(error !== null && want.test(error.message), "unknown option " + JSON.stringify(options) + ": " + error?.message);
    }
  }
}
`
	runTypedTSFixture(t, tsBudgetFormsGrammar, consumer, gen.GenOptions{Start: "main", Recognize: true, LimitControls: true}, "")
}
