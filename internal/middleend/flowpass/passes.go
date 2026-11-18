package flowpass

import (
	"strconv"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/pass"
)

// Names of the passes. A name is also the name of the flag that turns the pass off; the passes that
// take a number (class-tables, inline-literals) are off at 0.
const (
	PassTerminals      = "terminals"
	PassGuardedClasses = "guarded-classes"
	PassClassRuns      = "class-runs"
	PassClassTables    = "class-tables"
	PassInlineLiterals = "inline-literals"
)

// ordinary reports a function printed in the ordinary form: without budget controls, Documents or
// the records of a traced parse, each of which charges or records the input a read examines.
func ordinary(a *flow.Attrs) bool { return !a.Budget && !a.Document && !a.Traced }

// leanTyped reports a function of typed rules printed in the ordinary form.
func leanTyped(a *flow.Attrs) bool { return !a.Node && ordinary(a) }

// passes are the passes of Pipeline, in order. Each replaces the statements of forms the lowering
// wrote (flow.Forms) and marks them replaced, so that a later pass leaves them alone.
var passes = []pass.Pass[*code.Module, *Env]{
	{
		Name:       PassTerminals,
		Obligation: "skipTerminal consumes the same input and records the same expectations as the region it replaces, and charges nothing, as the region's literals and classes charge nothing in the ordinary form.",
		// A terminal program reports no rule and no skipped alternative, and charges nothing.
		Applies: func(e *Env) bool {
			return e.Target.Terminals && !e.Observe && e.For(func(a *flow.Attrs) bool { return !a.Frame && leanTyped(a) })
		},
		Run: run(terminals),
	},
	{
		Name:       PassGuardedClasses,
		After:      []string{PassTerminals},
		Obligation: "The class that begins a guarded alternative consumes the character that the guard's peek read and tested with the class's own test.",
		Applies:    func(e *Env) bool { return e.Target.GuardedClasses && e.On(corepass.PassGuards) && e.For(ordinary) },
		Run:        run(guardedClasses),
	},
	{
		Name:       PassClassRuns,
		After:      []string{PassGuardedClasses},
		Obligation: "skipClasses consumes the same input and records the same expectations as the consecutive classes it replaces; a class that consumes a guard's character is not one of them.",
		Applies:    func(e *Env) bool { return e.Target.ClassRuns && e.For(leanTyped) },
		Run:        run(classRuns),
	},
	{
		Name:       PassClassTables,
		After:      []string{PassClassRuns},
		Obligation: "matchClass consumes the same character and records the same expectation as the class's peek and test.",
		Applies:    func(e *Env) bool { return e.Target.ClassTables && e.ValueOf(PassClassTables) > 0 && e.For(leanTyped) },
		Run:        run(classTables),
	},
	{
		Name:       PassInlineLiterals,
		Obligation: "A literal compared in place (hasLit) consumes what matchLit consumes; matchLit stays the alternative that records the expectation when they differ.",
		Applies: func(e *Env) bool {
			return e.Target.InlineLiterals && e.ValueOf(PassInlineLiterals) > 0 && e.For(func(a *flow.Attrs) bool { return !a.Traced })
		},
		Run: run(inlineLiterals),
	},
}

// run adapts a pass over the function of a module.
func run(f func(fn *code.Func, a *flow.Attrs, e *Env)) func(*code.Module, *Env) error {
	return func(m *code.Module, e *Env) error {
		fn := flow.FuncOf(m)
		if a := flow.AttrsOf(fn); a != nil && a.Forms != nil {
			f(fn, a, e)
		}
		return nil
	}
}

// failIf returns the statement that leaves the form on failure when cond holds.
func failIf(form *flow.Form, cond code.Expr) code.Stmt {
	return &code.If{Cond: cond, Then: &code.Block{Stmts: []code.Stmt{form.FailJump()}}}
}

// terminals matches each terminal region of a local function of typed rules in the ordinary form as
// one program.
func terminals(fn *code.Func, a *flow.Attrs, _ *Env) {
	for _, r := range a.Forms.Regions {
		if !r.Live() || r.Build {
			continue
		}
		t := &core.Terminal{Body: r.Node}
		r.Block.Stmts = []code.Stmt{failIf(r, code.Not(&code.Call{Callee: flow.OpSkipTerminal, Imm: t}))}
		r.By = PassTerminals
	}
}

// guardedClasses keeps the size of the character that the guards of a choice peek, and lets the
// class that begins a guarded alternative consume it.
func guardedClasses(fn *code.Func, a *flow.Attrs, _ *Env) {
	sizes := map[*flow.ChoiceForm]string{}
	for _, c := range a.Forms.Choices {
		if !c.Live() {
			continue
		}
		size := newVar(fn, a, flow.TypeInt, c.Event)
		sizes[c] = size
		if c.Peek != nil {
			c.Peek.Dst[1] = code.Place{Name: size}
		}
	}
	for _, cl := range a.Forms.Classes {
		size, ok := sizes[cl.Guard]
		if !ok || !cl.Live() {
			continue
		}
		// The peek and the test go; the advance consumes the guard's character.
		rest := cl.Block.Stmts[2:]
		for _, st := range rest {
			if d, ok := st.(*code.Do); ok && d.Call.Callee == flow.OpAdvance {
				d.Call.Args = []code.Expr{code.L(size)}
			}
		}
		cl.Block.Stmts = rest
		cl.By = PassGuardedClasses
	}
}

// classRuns matches the consecutive classes of a value-free sequence as one.
func classRuns(fn *code.Func, a *flow.Attrs, _ *Env) {
	for _, s := range a.Forms.Seqs {
		if !s.Live() {
			continue
		}
		for i := 0; i < len(s.Items); i++ {
			n := 0
			for i+n < len(s.Items) && s.Items[i+n] != nil && s.Items[i+n].Live() {
				n++
			}
			if n < 2 {
				continue
			}
			run := &core.ClassRun{}
			for _, cl := range s.Items[i : i+n] {
				run.Classes = append(run.Classes, cl.Node.(*core.ClassNode).Class)
				cl.Block.Stmts = nil
				cl.By = PassClassRuns
			}
			first := s.Items[i]
			first.Block.Stmts = []code.Stmt{failIf(first, code.Not(&code.Call{Callee: flow.OpSkipClasses, Imm: run}))}
			i += n - 1
		}
	}
}

// classTables matches each class of at least the pass's number of ranges through its membership
// table, in a function of typed rules in the ordinary form.
func classTables(fn *code.Func, a *flow.Attrs, e *Env) {
	min := e.ValueOf(PassClassTables)
	for _, cl := range a.Forms.Classes {
		class := cl.Node.(*core.ClassNode).Class
		if !cl.Live() || len(class.Ranges) < min {
			continue
		}
		match := failIf(cl, code.Not(&code.Call{Callee: flow.OpMatchClass, Imm: class}))
		st := cl.Block.Stmts
		if cl.Build {
			// peek, test, start = pos, advance, value: the start is taken before the match.
			cl.Block.Stmts = []code.Stmt{st[2], match, st[4]}
		} else {
			cl.Block.Stmts = []code.Stmt{match}
		}
		cl.By = PassClassTables
	}
}

// inlineLiterals compares each exact literal of at most the pass's number of code points in place
// first, outside the traced copies, which read every literal through the operation that records it.
func inlineLiterals(fn *code.Func, a *flow.Attrs, e *Env) {
	max := e.ValueOf(PassInlineLiterals)
	var stmts func(list []code.Stmt)
	block := func(b *code.Block) {
		if b != nil {
			stmts(b.Stmts)
		}
	}
	stmts = func(list []code.Stmt) {
		for i, st := range list {
			switch st := st.(type) {
			case *code.Block:
				block(st)
			case *code.Loop:
				block(st.Body)
			case *code.Protect:
				block(st.Body)
				block(st.Finally)
			case *code.If:
				if lit, k := inlineLiteral(st, max); lit != nil {
					list[i] = &code.If{Cond: &code.Call{Callee: flow.OpHasLit, Imm: lit},
						Then: &code.Block{Stmts: []code.Stmt{&code.Do{Call: &code.Call{Callee: flow.OpAdvance, Args: []code.Expr{code.Int(int64(k))}}}}},
						Else: &code.Block{Stmts: []code.Stmt{st}}}
					continue
				}
				block(st.Then)
				block(st.Else)
			}
		}
	}
	block(fn.Body)
}

// inlineLiteral returns the literal that the If st matches, as the lowering writes a literal
// (if _, ok = matchLit(lit); !ok { fail }), and its number of code points, if it is exact and has at
// most max of them.
func inlineLiteral(st *code.If, max int) (*core.Lit, int) {
	if st.Init == nil || len(st.Init.Values) != 1 || st.Else != nil {
		return nil, 0
	}
	c, ok := st.Init.Values[0].(*code.Call)
	if !ok || c.Callee != flow.OpMatchLit {
		return nil, 0
	}
	lit, ok := c.Imm.(*core.Lit)
	if !ok || lit.Fold {
		return nil, 0
	}
	if k := len([]rune(lit.Text)); k > 0 && k <= max {
		return lit, k
	}
	return nil, 0
}

// newVar makes a variable of the type t that the lowering would have made at the event at, had it
// written the form a pass chose; Finish places it there.
func newVar(fn *code.Func, a *flow.Attrs, t flow.Type, at *flow.Event) string {
	a.Forms.N++
	name := "x" + strconv.Itoa(a.Forms.N)
	fn.Locals = append(fn.Locals, code.P(name, flow.CodeType(t)))
	a.Vars = append(a.Vars, name)
	if a.Forms.Placed == nil {
		a.Forms.Placed = map[string]*flow.Event{}
	}
	a.Forms.Placed[name] = at
	return name
}
