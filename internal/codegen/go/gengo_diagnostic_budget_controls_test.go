package gogen_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGeneratedGoDiagnosticBudget(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated diagnostic-control consumers")
	}
	g, err := parseTest(`type Word = { Text string }

rule main Word = t:(letter #recover(skip="!")) #atomic => Word{Text: text($t)}

rule letter Word = t:("z" / "a" / "é") #atomic => Word{Text: text($t)}`)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("diagnosticfixture")), 0644); err != nil {
		t.Fatal(err)
	}
	for _, convert := range []bool{false, true} {
		pkg := fmt.Sprintf("diagnostic_%v", convert)
		code, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: pkg, Start: "main", Types: true, ConvertTypes: convert})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, pkg)
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		fixture := strings.NewReplacer("PACKAGE", pkg).Replace(generatedGoDiagnosticConsumer)
		for name, data := range map[string][]byte{"parser.go": code, "diagnostic_test.go": []byte(fixture)} {
			if err := os.WriteFile(filepath.Join(path, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated diagnostic accounting: %v\n%s", err, out)
	}
}

const generatedGoDiagnosticConsumer = `package PACKAGE

import zprt "zeroparse.dev/zp/codegen/go/lib"
import("context";"errors";"fmt";"reflect";"slices";"strings";"testing")
func diagnosticParser(limits Limits)*zprt.Parser{return &zprt.Parser{Control:zprt.X_newBudgetControl(nil,limits)}}
func catchFatal(t *testing.T,f func())(err error){t.Helper();defer func(){if v:=recover();v!=nil{fatal,ok:=v.(zprt.X_fatal);if !ok{panic(v)};err=fatal.X_err}}();f();return nil}
type pollingContext struct{context.Context}
func(*pollingContext)Done()<-chan struct{}{return make(chan struct{})}
func(*pollingContext)Err()error{return context.Canceled}
func TestDiagnosticTextParityAndBudget(t *testing.T){
 maxInt:=int(^uint(0)>>1)
 rows:=[]*SyntaxError{
  {Line:1,Col:1},
  {Line:0,Col:0,Messages:[]string{""},Expected:[]string{"ignored"}},
  {Line:-1,Col:-2,Messages:[]string{"","two","","é\xff"}},
  {Line:maxInt,Col:-maxInt-1,Expected:[]string{"","x","","é","\xff"}},
  {Line:99,Col:101,Messages:[]string{strings.Repeat("λ\xff",180)}},
  {Line:2,Col:3,Expected:make([]string,320)},
  {Line:4,Col:5,Expected:[]string{"end of input","\"a\"","\"b\"","\"c\"","\"d\"","\"e\"","\"f\"","\"g\"","[0-9]","[x-z]"}},
  {Line:4,Col:5,Expected:[]string{"end of input","\"a\"","\"b\"","\"c\"","\"d\"","\"e\"","\"f\"","\"g\"","\"h\"","[0-9]","[x-z]"}},
 }
 for i,e:=range rows{t.Run(fmt.Sprint(i),func(t *testing.T){
  want:=e.Error();parts:=e.Messages;if len(parts)==0{parts=e.Expected[:min(len(e.Expected),10)]}
  steps:=uint64(len(want)+2*len(parts))
  p:=diagnosticParser(Limits{Compute:steps})
  retained:=p.RecoveryErrorText(e)
  if retained!=want{t.Fatal("message parity",retained,want)}
  if p.Control.X_budgetUsed()!=steps{t.Fatalf("text spend %+v",p.Control)}
  p.Control=zprt.X_newBudgetControl(nil,Limits{Compute:steps-1})
  if err:=catchFatal(t,func(){p.RecoveryErrorText(e)});!errors.Is(err,ErrLimit){t.Fatal("one-short text",err)}
  p.Control=zprt.X_newBudgetControl(nil,Limits{Compute:100000})
  p.RecoveryErrorText(&SyntaxError{Line:1,Col:2,Messages:[]string{"other"}})
  if retained!=want || e.Error()!=want{t.Fatal("retained message overwritten")}
 })}
 p:=&zprt.Parser{Control:zprt.X_newBudgetControl(&pollingContext{Context:context.Background()},Limits{Compute:100000})}
 var partial string
 if err:=catchFatal(t,func(){partial=p.RecoveryErrorText(&SyntaxError{Line:2,Col:3,Messages:make([]string,320)})});!errors.Is(err,context.Canceled)||partial!=""{t.Fatal("format cancellation published partial",partial,err)}
}
func TestDiagnosticSortingAndSnapshots(t *testing.T){
 for _,n:=range []int{0,1,2,3,7,8,9,127,128,129,511}{
  values:=make([]string,n)
  for i:=range values{values[i]=[]string{"","é","\xff","z","\xc3\xff",strings.Repeat("p",300),"\"a\"","[a]","\"\xff","[é"}[(n-i)%10]+strings.Repeat("p",(n-i)%3)}
  kind:=func(s string)int{if s!=""{switch s[0]{case '"':return 1;case '[':return 2}};return 0}
  want:=slices.Clone(values);slices.SortFunc(want,func(a,b string)int{if d:=kind(a)-kind(b);d!=0{return d};return strings.Compare(a,b)})
  p:=diagnosticParser(Limits{Compute:1<<50});p.X_sortExpected(values)
  if !slices.Equal(values,want){t.Fatalf("sort length %d changed",n)}
 }
 p:=diagnosticParser(Limits{Compute:128})
 if err:=catchFatal(t,func(){p.X_sortExpected(make([]string,320))});!errors.Is(err,ErrLimit){t.Fatal("sort moves unbounded",err)}
 ids:=make([]zprt.ExpID,320);for i:=range ids{ids[i]=zprt.ExpID(i)}
 p=diagnosticParser(Limits{Compute:100000})
 kept:=p.Keep(ids);before:=slices.Clone(kept)
 p.Control=zprt.X_newBudgetControl(nil,Limits{Compute:319})
 if err:=catchFatal(t,func(){p.Keep(ids)});!errors.Is(err,ErrLimit){t.Fatal("snapshot equality unbounded",err)}
 if !slices.Equal(kept,before){t.Fatal("snapshot overwritten")}
 p=diagnosticParser(Limits{Compute:319})
 if err:=catchFatal(t,func(){p.Keep(ids)});!errors.Is(err,ErrLimit){t.Fatal("snapshot copy unbounded",err)}
 p=diagnosticParser(Limits{Compute:100000});p.MergeExpected(0,ids)
 p.Control=zprt.X_newBudgetControl(nil,Limits{Compute:128})
 if err:=catchFatal(t,func(){p.MergeExpected(0,ids)});!errors.Is(err,ErrLimit){t.Fatal("merged expectations unbounded",err)}
 p=diagnosticParser(Limits{Compute:319})
 for range 320{p.PushRecovered(&SyntaxError{})}
 if err:=catchFatal(t,func(){p.X_recSince(0)});!errors.Is(err,ErrLimit){t.Fatal("recovered error lists unbounded",err)}
}
func TestRecoveredPublicDiagnostics(t *testing.T){
 for _,unit:=range []PositionUnit{Bytes,CodePoints}{
  retained,err:=Parse("!",WithUnit(unit));var recovered SyntaxErrors
  if !errors.As(err,&recovered)||len(recovered)!=1{t.Fatal("baseline recovery",retained,err)}
  expected:=[]string{"\"a\"","\"z\"","\"é\""}
  if !slices.Equal(recovered[0].Expected,expected){t.Fatalf("recovery sorting changed: %v",recovered[0].Expected)}
  before:=retained.String();message:=err.Error();diagnostic:=*recovered[0]
  for _,ast:=range []bool{false,true}{
   call:=func(steps uint64)(any,error){o:=[]ParseOption{WithUnit(unit),WithLimits(Limits{Compute:steps})};if ast{v,e:=ParseValue("!",o...);return v,e};v,e:=Parse("!",o...);return v,e}
   lo,hi:=uint64(1),uint64(100000)
   for lo<hi{mid:=lo+(hi-lo)/2;_,e:=call(mid);if errors.Is(e,ErrLimit){lo=mid+1}else if errors.As(e,&recovered){hi=mid}else{t.Fatal("unexpected diagnostic outcome",e)}}
   _,err:=call(lo);if !errors.As(err,&recovered){t.Fatal("exact recovery limit",err)}
   v,err:=call(lo-1)
   if (v!=nil && !reflect.ValueOf(v).IsZero()) || !errors.Is(err,ErrLimit){t.Fatal("partial recovery published",v,err)}
  }
  if retained.String()!=before || message!=diagnostic.Error() || !slices.Equal(diagnostic.Expected,expected){t.Fatal("retained recovery changed")}
  if n,err:=Parse("é",WithUnit(unit));err!=nil||n==nil{t.Fatal("valid retry",n,err)}
 }
}
`

func TestGeneratedGoRecoveryMessageBudget(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated long recovery-message consumers")
	}
	message := strings.Repeat("details λ; ", 512)
	g, err := parseTest(fmt.Sprintf(`type Text ~ node[string] | error
type Doc = { Text Text }
 rule main Doc = x:(("z" @error(message=%q)) #recover(skip="!")) => Doc{Text: $x}`, message))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("recoverytext")), 0644); err != nil {
		t.Fatal(err)
	}
	for _, convert := range []bool{false, true} {
		pkg := fmt.Sprintf("text_%v", convert)
		code, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: pkg, Start: "main", Types: true, Recognize: true, ConvertTypes: convert})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, pkg)
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		fixture := strings.NewReplacer("PACKAGE", pkg, "LONG_MESSAGE", fmt.Sprintf("%q", message)).Replace(generatedGoRecoveryTextConsumer)
		for name, data := range map[string][]byte{"parser.go": code, "text_test.go": []byte(fixture)} {
			if err := os.WriteFile(filepath.Join(path, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated long recovery message: %v\n%s", err, out)
	}
}

const generatedGoRecoveryTextConsumer = `package PACKAGE
import("errors";"reflect";"testing")
func TestLongRecoveredMessage(t *testing.T){
 message:=LONG_MESSAGE
 for _,unit:=range []PositionUnit{Bytes,CodePoints}{
  retained,err:=Parse("!",WithUnit(unit));var recovered SyntaxErrors
  if !errors.As(err,&recovered)||len(recovered)!=1 || len(recovered[0].Messages)!=1 || recovered[0].Messages[0]!=message{t.Fatal("recovery label changed",err)}
  want:=recovered[0].Error();captured,ok:=retained.Field("Text").(*Node);if !ok || captured.Kind()!=KindError || captured.Text()!=want{t.Fatalf("recovered message changed: %v",retained)}
  for _,ast:=range []bool{false,true}{
   var v any
   opts:=[]ParseOption{WithUnit(unit),WithLimits(Limits{Compute:256})}
   if ast{v,err=ParseValue("!",opts...)}else{v,err=Parse("!",opts...)}
   if (v!=nil && !reflect.ValueOf(v).IsZero()) || !errors.Is(err,ErrLimit){t.Fatalf("AST=%v long diagnostic copy unbounded: value=%T error=%T",ast,v,err)}
  }
  // Recognition records the error but does not construct a node message.
  err=Recognize("!",WithUnit(unit),WithLimits(Limits{Compute:256}))
  if !errors.As(err,&recovered){t.Fatal("recognizer charged unconstructed node message",err)}
  if captured.Text()!=want{t.Fatal("retained message overwritten")}
  if _,err:=ParseValue("!",WithUnit(unit),WithLimits(Limits{Compute:100000}));!errors.As(err,&recovered){t.Fatal("generous recovery retry",err)}
 }
}
`

// TestGeneratedGoRecoveredErrorsLinear measures two grammars whose results hold the recovered
// errors of every item after them: a right-recursive rule whose memo entry at each item holds those
// of the rest, and a left-recursive leader whose seed at each step holds those of the items before.
// Shared lists keep the Compute of each item constant, where copies made it grow with the input:
// four times the items must cost less than five times the Compute.
func TestGeneratedGoRecoveredErrorsLinear(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("recoveredlinear")), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		pkg, src, input string
		n               int
	}{
		{"right", `rule main = s $$

rule s = item s "!" / item s / item

rule item = ("a" ";") #recover(skip=[^;]* ";")`, `strings.Repeat("b;", n)`, 250},
		{"left", `rule main = e $$

rule e = e "," item / item

rule item = ("a" ";") #recover(skip=[^;]* ";")`, `strings.TrimSuffix(strings.Repeat("b;,", n), ",")`, 500},
	} {
		g, err := parseTest(tt.src)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: tt.pkg, Start: "main"})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, tt.pkg)
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		fixture := strings.NewReplacer("PACKAGE", tt.pkg, "INPUT", tt.input, "ITEMS", fmt.Sprint(tt.n)).Replace(generatedGoRecoveredLinearConsumer)
		for name, data := range map[string][]byte{"parser.go": code, "linear_test.go": []byte(fixture)} {
			if err := os.WriteFile(filepath.Join(path, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated recovered-error Compute: %v\n%s", err, out)
	}
}

const generatedGoRecoveredLinearConsumer = `package PACKAGE

import ("errors";"strings";"testing")
func input(n int) string { return INPUT }
func minimum(t *testing.T, s string) uint64 {
 fits := func(n uint64) bool { _, err := Parse(s, WithLimits(Limits{Compute: n})); var le *LimitError; return !errors.As(err, &le) }
 lo, hi := uint64(0), uint64(1)
 for !fits(hi) { lo, hi = hi, hi*2; if hi > 1<<30 { t.Fatal("no finite budget") } }
 for lo+1 < hi { mid := (lo + hi) / 2; if fits(mid) { hi = mid } else { lo = mid } }
 return hi
}
func TestLinear(t *testing.T) {
 n := ITEMS
 _, err := Parse(input(4 * n))
 var se SyntaxErrors
 if !errors.As(err, &se) || len(se) != 4*n { t.Fatalf("every item recovers: %v", err) }
 small, large := minimum(t, input(n)), minimum(t, input(4*n))
 if large >= 5*small { t.Fatalf("Compute grows faster than the input: %d for %d items, %d for %d", small, n, large, 4*n) }
}
`
