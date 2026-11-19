// Package pass holds the TypeScript backend's Exec passes: the forms of a lowered function that the
// TypeScript printer prints more compactly than the operations they replace. Each pass makes
// operations of this target (Spend, MergeRecord, Failed) that the printer prints, and the printer
// prints the operations they replace as well, so that each pass can be turned off by its flag.
package pass

import (
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/pass"
)

// Names of the passes, which are also the names of their flags.
const (
	PassSpendRuns     = "ts-spend-runs"
	PassMergedRecords = "ts-merged-records"
	PassTestedInits   = "ts-tested-inits"
)

// The primitive operations of the TypeScript target that the passes make.
const (
	SpendOp  = "ts.spend"
	MergeOp  = "ts.unisolateMerge"
	FailedOp = "ts.failed"
)

// SpendRun is the immediate operand of a call of SpendOp: the number of charges in a row.
type SpendRun struct{ N int }

// Failed is the immediate operand of a call of FailedOp: the variable of the bool that the Init of
// the If sets.
type Failed struct{ Flag string }

// Pipeline is the pipeline of the TypeScript Exec passes. The unit is a lowered function with its
// module; the passes change the function only.
var Pipeline = &pass.Pipeline[*execlower.Lowered, *codepass.Env]{
	Passes: []pass.Pass[*execlower.Lowered, *codepass.Env]{
		{
			Name:       PassSpendRuns,
			Obligation: "One spend of n units charges what the n charges in a row it replaces charge, in the same place.",
			Run: func(l *execlower.Lowered, _ *codepass.Env) (*execlower.Lowered, pass.PreservedSet, error) {
				spendRuns(l)
				return l, pass.PreserveNone(), nil
			},
		},
		{
			Name:       PassMergedRecords,
			Obligation: "unisolateMerge ends the expectation record and merges its expectations into the enclosing record, as the end and the merge it replaces do.",
			Run: func(l *execlower.Lowered, _ *codepass.Env) (*execlower.Lowered, pass.PreservedSet, error) {
				mergedRecords(l)
				return l, pass.PreserveNone(), nil
			},
		},
		{
			Name:       PassTestedInits,
			Obligation: "The condition reports that the call of the If's Init failed, as the negation of the bool the Init sets does.",
			Run: func(l *execlower.Lowered, _ *codepass.Env) (*execlower.Lowered, pass.PreservedSet, error) {
				testedInits(l)
				return l, pass.PreserveNone(), nil
			},
		},
	},
	Analyses: map[pass.Analysis]func(*execlower.Lowered){},
	Verify:   func(l *execlower.Lowered) error { return code.Verify(l.Module) },
	Copy:     (*execlower.Lowered).Clone,
}

func init() {
	pass.Register(
		pass.Flag{Name: PassSpendRuns, Kind: pass.KindExecPass, Default: 1, Doc: "charges in a row are one spend (TypeScript)"},
		pass.Flag{Name: PassMergedRecords, Kind: pass.KindExecPass, Default: 1, Doc: "the end of an expectation record and the merge of its expectations are one call (TypeScript)"},
		pass.Flag{Name: PassTestedInits, Kind: pass.KindExecPass, Default: 1, Doc: "an If tests the call of its Init in its condition, without the bool the call sets (TypeScript)"},
	)
}

// addOp declares the primitive operation o of the target in the module of l.
func addOp(l *execlower.Lowered, o *code.Op) {
	o.Component, o.Dialect, o.Kind = execlower.Component, execlower.Dialect, code.OpPrimitive
	l.Module.AddOp(o)
}

// rewrite replaces each statement list of the function of l, at any depth, with what f makes of it,
// inner lists first.
func rewrite(l *execlower.Lowered, f func([]code.Stmt) []code.Stmt) {
	var block func(ss []code.Stmt) []code.Stmt
	block = func(ss []code.Stmt) []code.Stmt {
		for _, s := range ss {
			switch s := s.(type) {
			case *code.Block:
				s.Stmts = block(s.Stmts)
			case *code.Loop:
				s.Body.Stmts = block(s.Body.Stmts)
			case *code.If:
				s.Then.Stmts = block(s.Then.Stmts)
				if s.Else != nil {
					s.Else.Stmts = block(s.Else.Stmts)
				}
			}
		}
		return f(ss)
	}
	l.Func.Body.Stmts = block(l.Func.Body.Stmts)
}

// spendRuns replaces each run of charges with one call of SpendOp.
func spendRuns(l *execlower.Lowered) {
	addOp(l, &code.Op{Name: SpendOp, Writes: []string{execlower.Component}, NoReturn: true,
		Imm: "the number of charges", Contract: "Charges a step as many times as Imm says."})
	rewrite(l, func(ss []code.Stmt) []code.Stmt {
		var out []code.Stmt
		for i := 0; i < len(ss); i++ {
			if step(ss[i]) {
				n := 1
				for i+n < len(ss) && step(ss[i+n]) {
					n++
				}
				out = append(out, &code.Do{Call: &code.Call{Callee: SpendOp, Imm: SpendRun{n}}})
				i += n - 1
				continue
			}
			out = append(out, ss[i])
		}
		return out
	})
}

// mergedRecords replaces each end of an expectation record that a merge of its expectations follows
// with one call of MergeOp.
func mergedRecords(l *execlower.Lowered) {
	addOp(l, &code.Op{Name: MergeOp, Params: []code.Param{code.P("m", code.THandle(flow.TExpMark))},
		Reads: []string{execlower.Component}, Writes: []string{execlower.Component}, NoReturn: true,
		Contract: "Ends the expectation record m and merges its expectations into the enclosing record."})
	rewrite(l, func(ss []code.Stmt) []code.Stmt {
		var out []code.Stmt
		for i := 0; i < len(ss); i++ {
			if i+1 < len(ss) {
				if end, ok := mergedRecord(ss[i], ss[i+1]); ok {
					out = append(out, &code.Do{Call: &code.Call{Callee: MergeOp, Args: end.Args}})
					i++
					continue
				}
			}
			out = append(out, ss[i])
		}
		return out
	})
}

// testedInits replaces the condition of each If that tests the call of its Init with a call of
// FailedOp, so that nothing else reads the bool the Init sets.
func testedInits(l *execlower.Lowered) {
	addOp(l, &code.Op{Name: FailedOp, Results: []code.Type{code.TBool}, Reads: []string{execlower.Component},
		Imm: "the bool the Init sets", Contract: "Reports that the call of the Init of the If failed: that the bool it sets is false."})
	rewrite(l, func(ss []code.Stmt) []code.Stmt {
		for _, s := range ss {
			if s, ok := s.(*code.If); ok {
				if flag, ok := testedFlag(s); ok {
					s.Cond = &code.Call{Callee: FailedOp, Imm: Failed{flag}}
				}
			}
		}
		return ss
	})
}

// testedFlag reports whether the If s tests that the call of its Init fails, and returns the variable
// of the bool that the Init sets.
func testedFlag(s *code.If) (string, bool) {
	if s.Init == nil || len(s.Init.Dst) != 2 || len(s.Init.Values) != 1 {
		return "", false
	}
	c, ok := s.Init.Values[0].(*code.Call)
	if !ok || !TwoResults(execlower.FlowOp(c.Callee)) {
		return "", false
	}
	flag := s.Init.Dst[1]
	not, ok := s.Cond.(*code.Unary)
	if !ok || not.Op != "!" || flag.Reg || flag.Name == "" {
		return "", false
	}
	if v, ok := not.X.(*code.Local); !ok || v.Name != flag.Name {
		return "", false
	}
	return flag.Name, true
}

// step reports whether s charges a step.
func step(s code.Stmt) bool {
	d, ok := s.(*code.Do)
	return ok && execlower.FlowOp(d.Call.Callee) == flow.OpStep
}

// mergedRecord reports whether a and b end an expectation record and merge its expectations into the
// enclosing record, and returns the call that ends the record.
func mergedRecord(a, b code.Stmt) (*code.Call, bool) {
	set, ok := a.(*code.Set)
	if !ok || len(set.Dst) != 2 || len(set.Values) != 1 {
		return nil, false
	}
	c, ok := set.Values[0].(*code.Call)
	if !ok || execlower.FlowOp(c.Callee) != flow.OpUnisolate {
		return nil, false
	}
	do, ok := b.(*code.Do)
	if !ok || execlower.FlowOp(do.Call.Callee) != flow.OpMergeExpected {
		return nil, false
	}
	far, ids := set.Dst[0], set.Dst[1]
	f, fok := do.Call.Args[0].(*code.Local)
	e, eok := do.Call.Args[1].(*code.Local)
	if !fok || !eok || far.Reg || ids.Reg || far.Name == "" || ids.Name == "" || f.Name != far.Name || e.Name != ids.Name {
		return nil, false
	}
	return c, true
}

// TwoResults reports whether a call of the Flow operation name has two results, the value and whether
// it matched, which an assignment tests.
func TwoResults(name string) bool {
	switch name {
	case flow.OpMatchLit, flow.OpCallRule, flow.OpCallOutline:
		return true
	}
	return false
}
