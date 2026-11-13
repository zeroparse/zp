package gents_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGeneratedTSBudgetControlCaptureRollback(t *testing.T) {
	t.Parallel()
	const width = 512
	captures := make([]string, width)
	for i := range captures {
		captures[i] = fmt.Sprintf(`c%d:"z"`, i)
	}
	sequence := strings.Join(captures, " ")
	// Two child frames in fast account for the root allocation and empty-frame
	// inspection in main. Only main also restores both failed capture trails.
	src := `rule main = ({true} ` + sequence + ` "!") / ({1 == 1} ` + sequence + ` "?") / "z" #repeat(512)
 rule fast = first / second / "z" #repeat(512)
 rule first = {true} ` + sequence + ` "!"
 rule second = {1 == 1} ` + sequence + ` "?"`
	consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
function minimum(fits:(n:number)=>boolean):number {
 let lo=0,hi=1;while(!fits(hi)){hi*=2;check(hi<10000000,"finite rollback budget")}
 while(lo+1<hi){const mid=Math.floor((lo+hi)/2);if(fits(mid))hi=mid;else lo=mid}
 return hi;
}
for(const unit of [p.CodePoints,p.Bytes]) {
 const input="z".repeat(512),ordinary=p.parse(input,unit),active=p.parseWithOptions(input,{unit,signal:{aborted:false}});
 check(ordinary.error===null&&active.error===null&&stable(active.node)===stable(ordinary.node)&&active.node?.fields.length===0,"failed captures are absent from successful fallback");
 const typed=p.parseAST(input,unit),converted=p.parseASTWithOptions(input,{unit,signal:{aborted:false}});
 check(typed.error===null&&converted.error===null&&stable(converted.value)===stable(typed.value),"typed rollback output parity");
 const baseline=minimum(n=>p.parseRuleWithOptions("fast",input,{unit,limits:{compute:n}}).error===null),limit=baseline+128;
 const stopped=p.parseWithOptions(input,{unit,limits:{compute:limit}});
 check(stopped.node===null&&stopped.error instanceof p.ParseInterruptionError&&stopped.error.kind==="compute-limit","capture restoration consumes spend beyond equivalent fresh frames");
 check(p.parseASTWithOptions(input,{unit,limits:{compute:limit}}).value===null,"typed rollback interruption exposes no partial value");
 check(p.parseWithOptions(input,{unit,limits:{compute:0}}).error===null,"fresh retry after interrupted capture restoration");
}
`
	runTypedTSFixture(t, src, consumer, gen.GenOptions{Start: "main", LimitControls: true, Types: true, Recognize: true}, "")
}

func TestGeneratedTSBudgetControlOperatorsRuleCapturePreservation(t *testing.T) {
	t.Parallel()
	const width = 512
	captures := make([]string, width)
	for i := range captures {
		captures[i] = fmt.Sprintf(`c%d:"-"`, i)
	}
	sequence := strings.Join(captures, " ")
	src := `type Word = { Text string }
 type Op = { Token node[string] }
 type Box = { Token node[string]; Rhs Expr }
 type Expr ~ Word | Box
 rule main Expr = #operators {
  operand word
  level { prefix ` + sequence + ` => Box{Token:$c0,Rhs:$rhs} }
 }
 rule fast Box = o:operator operator? x:word => Box{Token:$o.Token,Rhs:$x}
 rule operator Op = ` + sequence + ` => Op{Token:$c0}
 rule word Word = t:("z") #atomic => Word{Text: text($t)}`
	consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
function minimum(fits:(n:number)=>boolean):number {
 let lo=0,hi=1;while(!fits(hi)){hi*=2;check(hi<10000000,"finite Operators Rule budget")}
 while(lo+1<hi){const mid=Math.floor((lo+hi)/2);if(fits(mid))hi=mid;else lo=mid}
 return hi;
}
for(const unit of [p.CodePoints,p.Bytes]) {
 const input="-".repeat(512)+"z",ordinary=p.parse(input,unit),typed=p.parseAST(input,unit);
 const active=p.parseWithOptions(input,{unit,signal:{aborted:false}}),converted=p.parseASTWithOptions(input,{unit,signal:{aborted:false}});
 check(ordinary.error===null&&typed.error===null&&active.error===null&&converted.error===null&&stable(active.node)===stable(ordinary.node)&&stable(converted.value)===stable(typed.value),"Operators Rule capture preservation parity");
 check(converted.value?.type==="Box"&&converted.value.Token?.value==="-"&&converted.value.Rhs?.type==="Word","successful operator retains its captured token across candidate reset");
 const baseline=minimum(n=>p.parseRuleWithOptions("fast",input,{unit,limits:{compute:n}}).error===null);
 // Allow the successful trail reset, operator span materialization and
 // either one preservation pass over both candidate frames. Completing
 // both passes must exceed this limit.
 const limit=baseline+4*512+128;
 const stopped=p.parseWithOptions(input,{unit,limits:{compute:limit}});
 check(stopped.node===null&&stopped.error instanceof p.ParseInterruptionError&&stopped.error.kind==="compute-limit","Operators Rule preservation visits consume cumulative spend");
 check(p.parseASTWithOptions(input,{unit,limits:{compute:limit}}).value===null,"typed Operators Rule interruption exposes no partial value");
 check(p.parseASTWithOptions(input,{unit,limits:{compute:0}}).error===null,"fresh Operators Rule retry");
}
`
	runTypedTSFixture(t, src, consumer, gen.GenOptions{Start: "main", LimitControls: true, Types: true, Recognize: true}, "")
}

func TestGeneratedTSBudgetControlRecoveredMemoReplay(t *testing.T) {
	t.Parallel()
	// Prewarm an empty-error memo so slot allocation is common to both routes. The lookaheads
	// store and replay recovered errors that their ends discard; the call after them reports its own.
	// Storing the errors costs a unit for each, and replaying them a constant: main replays them
	// once more than fast, within 128 units of it.
	src := `rule main = &seed &seed &recovered &recovered &recovered recovered $$

rule fast = &seed &seed &recovered &recovered recovered $$

rule fresh = &seed &seed &recovered &other recovered $$

rule other = bad #repeat(512)

rule recovered = bad #repeat(512)

rule bad = ("a" "!") #recover(skip="a" "?")

rule seed = bit

rule bit = ""`
	consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
function minimum(fits:(n:number)=>boolean):number {
 let lo=0,hi=1;while(!fits(hi)){hi*=2;check(hi<10000000,"finite recovered replay budget")}
 while(lo+1<hi){const mid=Math.floor((lo+hi)/2);if(fits(mid))hi=mid;else lo=mid}
 return hi;
}
for(const unit of [p.CodePoints,p.Bytes]) {
 const input="a?".repeat(512),ordinary=p.parse(input,unit),active=p.parseWithOptions(input,{unit,signal:{aborted:false}});
 check(ordinary.node!==null&&active.node!==null&&ordinary.error instanceof p.SyntaxErrors&&active.error instanceof p.SyntaxErrors&&active.error.errors.length===512&&stable(active.node)===stable(ordinary.node)&&active.error.message===ordinary.error.message,"memo hit across lookahead states preserves recovered errors and output");
 for(let i=0;i<512;i++)check(active.error.errors[i]?.pos===i*2+1,"memo replay preserves error positions and order");
 const freshBudget=minimum(n=>{const r=p.parseRuleWithOptions("fresh",input,{unit,limits:{compute:n}});return r.node!==null&&r.error instanceof p.SyntaxErrors&&r.error.errors.length===512});
 const snapshot=p.parseRuleWithOptions("fast",input,{unit,limits:{compute:freshBudget+128}});
 check(snapshot.node===null&&snapshot.error instanceof p.ParseInterruptionError&&snapshot.error.kind==="compute-limit","recovered memo snapshots consume spend beyond unretained errors");
 const baseline=minimum(n=>{const r=p.parseRuleWithOptions("fast",input,{unit,limits:{compute:n}});return r.node!==null&&r.error instanceof p.SyntaxErrors&&r.error.errors.length===512});
 const replayed=p.parseWithOptions(input,{unit,limits:{compute:baseline+128}});
 check(replayed.node!==null&&replayed.error instanceof p.SyntaxErrors&&replayed.error.errors.length===512,"a memo replay of recovered errors costs a constant, not a unit for each error");
 const retry=p.parseWithOptions(input,{unit,limits:{compute:0}});
 check(retry.node!==null&&retry.error instanceof p.SyntaxErrors&&retry.error.errors.length===512,"fresh recovered memo retry");
}
`
	runTypedTSFixture(t, src, consumer, gen.GenOptions{Start: "main", LimitControls: true, Types: true, Recognize: true}, "")
}

// TestGeneratedTSRecoveredErrorsLinear measures two grammars whose results hold the recovered
// errors of every item after them: a right-recursive rule whose memo entry at each item holds those
// of the rest, and a left-recursive leader whose seed at each step holds those of the items before.
// Shared lists keep the Compute of each item constant, where copies made it grow with the input:
// four times the items must cost less than five times the Compute.
func TestGeneratedTSRecoveredErrorsLinear(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, src, input string
		n                int
	}{
		{"right recursion", `rule main = s $$

rule s = item s "!" / item s / item

rule item = ("a" ";") #recover(skip=[^;]* ";")`, `(n:number)=>"b;".repeat(n)`, 250},
		{"left recursion", `rule main = e $$

rule e = e "," item / item

rule item = ("a" ";") #recover(skip=[^;]* ";")`, `(n:number)=>"b;,".repeat(n).slice(0,-1)`, 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
void stable;
const input=` + tt.input + `;
function minimum(s:string):number {
 const fits=(n:number)=>!(p.parseWithOptions(s,{limits:{compute:n}}).error instanceof p.ParseInterruptionError);
 let lo=0,hi=1;while(!fits(hi)){lo=hi;hi*=2;check(hi<100000000,"finite budget")}
 while(lo+1<hi){const mid=Math.floor((lo+hi)/2);if(fits(mid))hi=mid;else lo=mid}
 return hi;
}
const n=` + fmt.Sprint(tt.n) + `,small=minimum(input(n)),large=minimum(input(4*n));
const r=p.parse(input(4*n));
check(r.error instanceof p.SyntaxErrors&&r.error.errors.length===4*n,"every item recovers: "+String(r.error));
check(large<5*small,"Compute grows linearly: "+small+" for "+n+" items, "+large+" for "+4*n);
`
			runTypedTSFixture(t, tt.src, consumer, gen.GenOptions{Start: "main", LimitControls: true}, "")
		})
	}
}

func TestGeneratedTSBudgetControlRecoveryParity(t *testing.T) {
	t.Parallel()
	cases := append(append([]gentest.Case(nil), gentest.TypedOperatorsRuleBodyCases...), gentest.TypedLRBodyCases...)
	for _, tt := range cases {
		if tt.Name != "typed/OperatorsRule_recovery" && tt.Name != "typed/LR_recovery" {
			continue
		}
		t.Run(tt.Name, func(t *testing.T) {
			inputs, err := json.Marshal(tt.Inputs)
			if err != nil {
				t.Fatal(err)
			}
			consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
for(const unit of [p.CodePoints,p.Bytes])for(const input of ` + string(inputs) + `) {
 const ordinary=p.parse(input,unit),active=p.parseWithOptions(input,{unit,signal:{aborted:false}});
 check(stable(active.node)===stable(ordinary.node)&&active.error?.message===ordinary.error?.message,"Operators Rule/LR recovered snapshot and replay parity");
 const typed=p.parseAST(input,unit),converted=p.parseASTWithOptions(input,{unit,signal:{aborted:false}});
 check(stable(converted.value)===stable(typed.value)&&converted.error?.message===typed.error?.message,"typed Operators Rule/LR recovered parity");
 check(p.recognizeWithOptions(input,{unit,signal:{aborted:false}})?.message===p.recognize(input,unit)?.message,"recognizer recovered parity");
}
`
			runTypedTSFixture(t, tt.Src, consumer, gen.GenOptions{Start: "main", LimitControls: true, Types: true, Recognize: true}, "")
		})
	}
}

func TestGeneratedTSBudgetControlOperatorsRuleRecoveredTransfer(t *testing.T) {
	t.Parallel()
	// An attempt of the prefix operator stores its recovered errors for a unit each, and the
	// attempt that wins applies them for a constant.
	src := `type Word = { Text string }

type Box = {
    Token node[string]
    Rhs   Expr
}

type Expr ~ Word | Box

rule main Expr = #operators {
    operand word
    level {
        prefix (bad #repeat(512) #atomic) => Box{Token: $op, Rhs: $rhs}
    }
}

rule fast Box = t:(bad #repeat(512) #atomic) (-bad #repeat(512))? x:word => Box{Token: $t, Rhs: $x}

rule bad = ("a" "!") #recover(skip="a" "?")

rule word Word = t:("z") #atomic => Word{Text: text($t)}`
	consumer := `import * as p from "./parser.ts";
` + tsTypedAssert + `
function minimum(fits:(n:number)=>boolean):number {
 let lo=0,hi=1;while(!fits(hi)){hi*=2;check(hi<10000000,"finite Operators Rule recovered budget")}
 while(lo+1<hi){const mid=Math.floor((lo+hi)/2);if(fits(mid))hi=mid;else lo=mid}
 return hi;
}
for(const unit of [p.CodePoints,p.Bytes]) {
 const input="a?".repeat(512)+"z",ordinary=p.parse(input,unit),active=p.parseWithOptions(input,{unit,signal:{aborted:false}});
 check(ordinary.node!==null&&active.node!==null&&ordinary.error instanceof p.SyntaxErrors&&active.error instanceof p.SyntaxErrors&&active.error.errors.length===512&&stable(active.node)===stable(ordinary.node)&&active.error.message===ordinary.error.message,"Operators Rule recovered snapshot and apply parity");
 const typed=p.parseAST(input,unit),converted=p.parseASTWithOptions(input,{unit,signal:{aborted:false}});
 check(converted.error instanceof p.SyntaxErrors&&stable(converted.value)===stable(typed.value)&&converted.value?.type==="Box"&&converted.value.Token?.value==="a?".repeat(512),"typed Operators Rule recovered output");
 const baseline=minimum(n=>{const r=p.parseRuleWithOptions("fast",input,{unit,limits:{compute:n}});return r.node!==null&&r.error instanceof p.SyntaxErrors&&r.error.errors.length===512});
 const snapshot=p.parseWithOptions(input,{unit,limits:{compute:baseline+128}});
 check(snapshot.node===null&&snapshot.error instanceof p.ParseInterruptionError&&snapshot.error.kind==="compute-limit","Operators Rule recovered snapshots consume spend");
 const applied=p.parseWithOptions(input,{unit,limits:{compute:baseline+512+128}});
 check(applied.node!==null&&applied.error instanceof p.SyntaxErrors&&applied.error.errors.length===512,"an Operators Rule applies recovered errors for a constant, not a unit for each error");
 const retry=p.parseWithOptions(input,{unit,limits:{compute:0}});
 check(retry.node!==null&&retry.error instanceof p.SyntaxErrors&&retry.error.errors.length===512,"fresh Operators Rule recovered retry");
}
`
	runTypedTSFixture(t, src, consumer, gen.GenOptions{Start: "main", LimitControls: true, Types: true, Recognize: true}, "")
}
