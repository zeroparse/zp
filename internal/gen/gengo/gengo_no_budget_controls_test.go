package gengo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// A parser generated without limit controls holds no code that spends budget or reads a control, does not
// export the control options, and fails a parse that passes them instead of ignoring them.
func TestGeneratedWithoutBudgetControls(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	const src = `rule main = item+ !.
rule item = word / num / " "
rule word = [a-zA-Z_ä-ö]+
rule num = [0-9]+ ("." [0-9]+)?
`
	g, err := parseTest(src)
	if err != nil {
		t.Fatal(err)
	}
	gen := func(budgetControls bool, document ...bool) string {
		files, err := gentest.Generate(t, g, gen.GenOptions{Package: "wc", Module: "wc", Start: "main", Runtime: gen.RuntimeImport, Types: true, Recognize: true, LimitControls: budgetControls, Document: len(document) > 0})
		if err != nil {
			t.Fatal(err)
		}
		return string(gentest.MergeGenerated(files))
	}
	with, without := gen(true), gen(false)
	for _, want := range []string{"p.Spend()", "p.Control.Take(", "func WithContext"} {
		if !strings.Contains(with, want) {
			t.Errorf("with limit controls: missing %q", want)
		}
		if strings.Contains(without, want) {
			t.Errorf("without limit controls: found %q", want)
		}
	}
	for _, bad := range []string{"p.Control", "BudgetClass"} {
		if strings.Contains(without, bad) {
			t.Errorf("without limit controls: found %q", bad)
		}
	}
	// Both forms take the depth limit through WithLimits.
	if !strings.Contains(with, "func WithLimits") || !strings.Contains(without, "func WithLimits") {
		t.Error("a form of the parser has no WithLimits")
	}
	// The helpers with an Ordinary form belong to the form without limit controls and Documents only.
	for name, text := range map[string]string{"with limit controls": with, "Document with limit controls": gen(true, true)} {
		if strings.Contains(text, "Ordinary(") {
			t.Errorf("%s: a call of an Ordinary helper", name)
		}
	}
	if len(without) >= len(with) {
		t.Errorf("parser without limit controls is %d bytes, with %d: want smaller", len(without), len(with))
	}
	// With a Document the parser has the form that serves a Document, which has neither budget charges
	// nor control reads when the parser has no controls.
	documentWith, documentWithout := gen(true, true), gen(false, true)
	for _, bad := range []string{"p.Control", ".Spend()", ".SpendRead(", "BudgetClass"} {
		if strings.Contains(documentWithout, bad) {
			t.Errorf("a Document parser without limit controls has %q", bad)
		}
		if !strings.Contains(documentWith, bad) && bad != "BudgetClass" {
			t.Errorf("a Document parser with limit controls lacks %q: the check is void", bad)
		}
	}
	if !strings.Contains(documentWithout, "p.Document == nil") {
		t.Error("a Document parser without limit controls has no in-place read guard")
	}
	t.Logf("sizes: with controls %d, without %d; with Document: with controls %d, without %d", len(with), len(without), len(documentWith), len(documentWithout))
	if len(documentWithout) >= len(documentWith) {
		t.Errorf("Document parser without limit controls is %d bytes, with %d", len(documentWithout), len(documentWith))
	}
	tests := `package wc
import("errors";"strings";"testing";"context";zprt "zeroparse.dev/zp/runtime";"zeroparse.dev/zp/core")
func TestOptions(t *testing.T) {
 if _,err:=Parse("abc 12.5 x"); err!=nil {t.Fatal(err)}
 if err:=Recognize("abc 12.5 x"); err!=nil {t.Fatal(err)}
 if _,err:=Parse("abc ("); err==nil {t.Fatal("no error for bad input")}
 if _,err:=ParseValue("abc 12.5 x"); err!=nil {t.Fatal(err)}
 // A zero Limits and a nil context set no control, so they are accepted.
 if _,err:=Parse("abc",zprt.WithLimits(core.Limits{}),zprt.WithContext(nil)); err!=nil {t.Fatal(err)}
 for name,opt:=range map[string]zprt.ParseOption{"limits":zprt.WithLimits(core.Limits{Compute:1}),"memory":zprt.WithLimits(core.Limits{Memory:1}),"context":zprt.WithContext(context.Background())} {
  for entry,call:=range map[string]func() error{
   "Parse":func() error {_,err:=Parse("abc",opt);return err},
   "Recognize":func() error {return Recognize("abc",opt)},
   "ParseValue":func() error {_,err:=ParseValue("abc",opt);return err},
   "ParseRule":func() error {_,err:=ParseRule("word","abc",opt);return err},
  } {
   err:=call()
   if err==nil || !strings.Contains(err.Error(),"without limit controls") {t.Errorf("%s %s: got %v",entry,name,err)}
  }
 }
 // A finite Input needs no control: it is checked against the length of the input. A later
 // WithLimits keeps the fields it leaves zero.
 for entry,call:=range map[string]func(...zprt.ParseOption) error{
  "Parse":func(o ...zprt.ParseOption) error {_,err:=Parse("abc 12.5 x",o...);return err},
  "Recognize":func(o ...zprt.ParseOption) error {return Recognize("abc 12.5 x",o...)},
  "ParseValue":func(o ...zprt.ParseOption) error {_,err:=ParseValue("abc 12.5 x",o...);return err},
  "ParseRule":func(o ...zprt.ParseOption) error {_,err:=ParseRule("word","abcdefghij",o...);return err},
 } {
  if err:=call(WithLimits(Limits{Input:10}));err!=nil {t.Errorf("%s: an input as long as the limit: %v",entry,err)}
  for _,opts:=range [][]zprt.ParseOption{{WithLimits(Limits{Input:9})},{WithLimits(Limits{Input:9}),WithLimits(Limits{Depth:50})}} {
   var le *LimitError
   if err:=call(opts...);!errors.As(err,&le) || le.Kind!=LimitInput || le.Limit!=9 || le.Requested!=10 {t.Errorf("%s: a longer input: %v",entry,err)}
  }
 }
}
`
	dir := t.TempDir()
	for name, data := range map[string][]byte{"parser.go": []byte(without), "wc_test.go": []byte(tests), "go.mod": []byte(gentest.GoMod("wc"))} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
