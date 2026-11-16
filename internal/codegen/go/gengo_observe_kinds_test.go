package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestGeneratedObservationEventKinds checks that every entry point of an observability build reports
// every kind of event in the shared core form: enter, exit and skipped events, memo results, calls in
// a lookahead (marked on enter and exit), left-recursive growth, the exact examined end, no reused
// elements and accessors that return zero values, with a profile that adds the same events up. The
// entry points are the Node batch parse and rule parse, the recognizer, the Document, the stream and
// the typed batch, rule and Document parses.
func TestGeneratedObservationEventKinds(t *testing.T) {
	t.Parallel()
	codegentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds standalone generated observation consumers")
	}
	const rules = `rule look = expr ";"
rule expr = expr "+" num / num
rule num = [0-9]+
rule item = first / second
rule first = "a"
rule second = "b"`
	dir := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", codegentest.GoMod("kindsfixture"))
	for _, f := range []struct {
		pkg, grammar, entries string
		opts                  codegen.GenOptions
	}{
		{"node", "rule main = &look expr \";\" item $$\n" + rules,
			`"parse":func(o ...ParseOption)error{_,err:=Parse(input,o...);return err},
"rule":func(o ...ParseOption)error{_,err:=ParseRule("main",input,o...);return err},
"recognize":func(o ...ParseOption)error{return Recognize(input,o...)},
"document":func(o ...ParseOption)error{d,err:=NewDocument(input);if err!=nil{return err};_,err=d.Parse(o...);return err},`,
			codegen.GenOptions{Recognize: true, Document: true}},
		{"stream", "rule main = line* #stream $$\nrule line = &look expr \";\" item \"\\n\"\n" + rules,
			`"stream":func(o ...ParseOption)error{return ParseStream(strings.NewReader(input+"\n"+input+"\n"),func(*Node)error{return nil},o...)},`,
			codegen.GenOptions{Stream: true}},
		{"typed", "type W = { Text string }\nrule main W = &look t:(expr \";\" item) #atomic $$ => W{Text: text($t)}\n" + rules,
			`"value":func(o ...ParseOption)error{_,err:=ParseValue(input,o...);return err},
"rulevalue":func(o ...ParseOption)error{_,err:=ParseRuleValue("main",input,o...);return err},
"recognize":func(o ...ParseOption)error{return Recognize(input,o...)},
"document":func(o ...ParseOption)error{d,err:=NewDocument(input);if err!=nil{return err};_,err=d.ParseValue(o...);return err},`,
			codegen.GenOptions{Types: true, Recognize: true, Document: true}},
	} {
		g, err := parseTest(f.grammar)
		if err != nil {
			t.Fatal(f.pkg, err)
		}
		opts := f.opts
		opts.Package, opts.Start, opts.Observability = f.pkg, "main", true
		codegentest.GeneratePackageIn(t, dir, "kindsfixture", f.pkg, g, opts)
		write(f.pkg+"/kinds_test.go", "package "+f.pkg+"\n"+observationKindsConsumerHead+f.entries+observationKindsConsumerTail)
	}
	cmd := exec.Command("go", "test", "-p", "4", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated observation kinds: %v\n%s", err, out)
	}
}

const observationKindsConsumerHead = `
import ("strings";"testing")

const input = "1+2;b"

var _ = strings.NewReader

func TestKinds(t *testing.T) {
 for name, run := range map[string]func(...ParseOption) error {
`

const observationKindsConsumerTail = `
 } {
  var events []TraceEvent
  var profile Profile
  if err := run(WithTrace(func(e TraceEvent) {
   if l, c := e.LineCol(e.Pos); l != 0 || c != 0 || e.Text() != "" || e.Failure() != nil || e.Recovered() != nil {
    t.Errorf("%s: accessors of a generated event: %d:%d %q", name, l, c, e.Text())
   }
   events = append(events, e)
  }), WithProfile(&profile)); err != nil {
   t.Fatal(name, err)
  }
  var stack []TraceEvent
  var enter, exit, skipped, memo, growth, lookEnter, lookExit int
  for _, e := range events {
   if e.ReusedElements != 0 {
    t.Errorf("%s: reused elements %+v", name, e)
   }
   switch e.Kind {
   case TraceEnter:
    enter++
    if e.Lookahead {
     lookEnter++
    }
    stack = append(stack, e)
   case TraceExit:
    exit++
    top := stack[len(stack)-1]
    stack = stack[:len(stack)-1]
    if top.Rule != e.Rule || top.Pos != e.Pos || top.Lookahead != e.Lookahead || top.Depth != e.Depth {
     t.Errorf("%s: exit %+v of enter %+v", name, e, top)
    }
    if e.Lookahead {
     lookExit++
    }
    if e.Memo {
     memo++
    }
    if e.Evals > 1 {
     growth++
    }
    if e.Examined < e.Pos || e.Matched && e.Examined < e.End {
     t.Errorf("%s: examined end %+v", name, e)
    }
   case TraceSkipped:
    skipped++
    if e.SkipReason != SkipFirstCharacter || e.Depth != len(stack)+1 {
     t.Errorf("%s: skipped %+v", name, e)
    }
   }
  }
  if len(stack) != 0 || enter == 0 || exit == 0 || skipped == 0 || memo == 0 || growth == 0 || lookEnter == 0 || lookEnter != lookExit {
   t.Errorf("%s: enter %d exit %d skipped %d memo %d growth %d lookahead %d/%d open %d", name, enter, exit, skipped, memo, growth, lookEnter, lookExit, len(stack))
  }
  s := profile.Snapshot()
  if s.Entered != enter || s.Calls != exit || s.Skipped != skipped || s.MemoHits != memo || s.ReusedElements != 0 || s.Examined == 0 {
   t.Errorf("%s: profile %+v, events enter %d exit %d skipped %d memo %d", name, s, enter, exit, skipped, memo)
  }
 }
}
`
