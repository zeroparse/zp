package flow

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/ir/code"
)

// VerifyModule checks a module of grammar code at the Flow stage: the invariants of Code IR
// (code.Verify, which also checks that each label an exit, break or continue names encloses it,
// that each local is declared and that each assignment has as many places as values), and for each
// function the invariants of lowered grammar code that hold from lowering on:
//
//   - its attributes (Attrs) are set; a local function has the entry variables Start, Result and
//     Ctx, a framed one none and no other local function has a cut flag; the locals are the entry
//     variables and the variables (Attrs.Vars), each once;
//   - every block statement has a label; a loop is left (break) or continued only from its own body,
//     outside nested loops, as a printed break or continue without a label does; a loop without a
//     condition breaks to itself;
//   - a peek assigns the variable for whether there is a character;
//   - a scan end, a skip-run begin and the operations of a run site have a positive site;
//   - a stream start and a stream element are in framed Node functions only, and a frame trail and
//     a frame release are not in a Node function;
//   - a Node function calls no typed value operation (opsTyped), and a typed one no Node one (opsNode);
//   - a traced function (Attrs.Traced) reads no input in place (opsInPlace);
//   - in a Document function (Attrs.Document), an in-place literal read is the whole condition of a
//     branch whose alternative records the read (matchLit);
//   - a function lowered with budget controls (Attrs.Budget) charges a step before it consumes input
//     and in each iteration of a loop without a counter (verifyCharges).
func VerifyModule(m *code.Module) error {
	if err := code.Verify(m); err != nil {
		return err
	}
	for _, fn := range m.Funcs {
		if err := verifyFunc(fn); err != nil {
			return fmt.Errorf("%s: %w", fn.Name, err)
		}
	}
	return nil
}

// Names of the operations that the invariants of VerifyModule name.
var (
	// opsTyped are the operations of the typed runtime: a Node function has none.
	opsTyped = map[string]string{OpNewMatch: "match", OpNewNode: "node", OpErrorValue: "error value",
		OpFreshNode: "fresh node", OpConvertResult: "result conversion", OpUseCtx: "action context",
		OpCtxFinish: "action context", OpEval: "action context",
		// The Node runtime attaches the captures of a frame (attachFrame); the values of local
		// variables are the typed runtime's form.
		OpAttachCaptures: "capture values of local variables"}
	// opsNode are the operations of the Node runtime: a typed function has none.
	opsNode = map[string]string{OpNewTerminalNode: "terminal", OpNewBranchNode: "branch", OpNewErrorNode: "error node"}
	// opsInPlace read the input in place: a traced function has none.
	opsInPlace = map[string]string{OpHasLit: "an in-place literal read", OpMatchClass: "a class table read",
		OpSkipTerminal: "a terminal program", OpSkipClasses: "a class run"}
	// opsConsume consume input: a function with budget controls charges before them.
	opsConsume = []string{OpMatchLit, OpMatchClass, OpSkipTerminal, OpSkipClasses, OpSkipHit, OpScanReach}
)

func verifyFunc(fn *code.Func) error {
	a := AttrsOf(fn)
	if a == nil {
		return fmt.Errorf("no Flow attributes")
	}
	entry := []string{a.Start, a.Result, a.Ctx}
	for _, x := range entry {
		if a.Frame != (x == "") {
			return fmt.Errorf("entry variable missing or in a framed function")
		}
	}
	if !a.Frame && a.Cut != "" {
		return fmt.Errorf("cut flag of a local function")
	}
	listed := map[string]bool{}
	for _, x := range a.Vars {
		if listed[x] || slices.Contains(entry, x) {
			return fmt.Errorf("variable %s listed twice or as an entry variable", x)
		}
		listed[x] = true
	}
	if a.Cut != "" && !listed[a.Cut] {
		return fmt.Errorf("cut flag %s is not a variable", a.Cut)
	}
	for _, l := range fn.Locals {
		if !listed[l.Name] && (l.Name == "" || !slices.Contains(entry, l.Name)) {
			return fmt.Errorf("local %s is neither a variable nor an entry variable", l.Name)
		}
	}
	v := &flowVerifier{a: a}
	v.block(fn.Body)
	if v.err == nil {
		code.Walk(fn.Body, nil, func(x code.Expr) {
			if c, ok := x.(*code.Call); ok {
				v.call(c)
			}
		})
	}
	if v.err == nil && a.Budget {
		return verifyCharges(fn)
	}
	return v.err
}

type flowVerifier struct {
	a     *Attrs
	loops []*code.Loop // the enclosing loops, innermost last
	err   error
}

func (v *flowVerifier) fail(format string, args ...any) {
	if v.err == nil {
		v.err = fmt.Errorf(format, args...)
	}
}

func (v *flowVerifier) block(b *code.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		v.stmt(s)
	}
}

// innermost reports whether the label names the innermost enclosing loop, and outer whether it
// names another enclosing loop.
func (v *flowVerifier) innermost(label string) (innermost, outer bool) {
	for k, l := range v.loops {
		if l.Label == label {
			return k == len(v.loops)-1, k < len(v.loops)-1
		}
	}
	return false, false
}

func (v *flowVerifier) stmt(s code.Stmt) {
	switch s := s.(type) {
	case *code.Block:
		if s.Label == "" {
			v.fail("block without a label of its own")
		}
		v.block(s)
	case *code.Loop:
		if s.Cond == nil && !breaks(s.Body, s.Label) {
			v.fail("unbounded loop that never exits")
		}
		v.loops = append(v.loops, s)
		v.block(s.Body)
		v.loops = v.loops[:len(v.loops)-1]
	case *code.If:
		if v.a.Document {
			v.documentRead(s)
		}
		v.block(s.Then)
		v.block(s.Else)
	case *code.Break:
		if _, outer := v.innermost(s.Label); outer {
			v.fail("exit to a loop that is not the innermost one")
		}
	case *code.Continue:
		if _, outer := v.innermost(s.Label); outer {
			v.fail("continue of a loop that is not the innermost one")
		}
	case *code.Set:
		if c := callOf(s.Values); c != nil && c.Callee == OpPeek && (len(s.Dst) != 3 || s.Dst[2].Name == "") {
			v.fail("peek without a variable for whether there is a character")
		}
	}
}

// call checks the call c of an operation against the attributes of the function.
func (v *flowVerifier) call(c *code.Call) {
	a := v.a
	switch c.Callee {
	case OpScanEnd, OpSkipBegin, OpRunHot, OpRunWalked:
		if site, _ := c.Imm.(Site); site <= 0 {
			v.fail("%s site %d", c.Callee, site)
		}
	case OpRunBegin:
		if w, _ := c.Imm.(RunWalk); w.Site <= 0 {
			v.fail("%s site %d", c.Callee, w.Site)
		}
	case OpStreamStart, OpEmitStream:
		if !a.Node || !a.Frame {
			v.fail("%s outside a framed Node function", c.Callee)
		}
	case OpDropTrail, OpFreeFrame:
		if a.Node {
			v.fail("%s in a Node function", c.Callee)
		}
	}
	if what, ok := opsTyped[c.Callee]; ok && a.Node {
		v.fail("typed %s in a Node function", what)
	}
	if what, ok := opsNode[c.Callee]; ok && !a.Node {
		v.fail("Node %s in a typed function", what)
	}
	if what, ok := opsInPlace[c.Callee]; ok && a.Traced {
		v.fail("%s in a traced function", what)
	}
}

// documentRead checks the condition of the branch s in a function lowered for Documents: a read of the
// input in place (hasLit) is the whole condition, and the alternative is the recording primitive
// (matchLit), which the target takes when the parse is a Document's; otherwise a Document parse would
// read without recording how far it examined.
func (v *flowVerifier) documentRead(s *code.If) {
	if !hasCall(s.Cond, func(c *code.Call) bool { return c.Callee == OpHasLit }) {
		return
	}
	if c, ok := s.Cond.(*code.Call); !ok || c.Callee != OpHasLit {
		v.fail("an in-place read is part of a larger condition in a Document function")
		return
	}
	found := false
	if s.Else != nil {
		code.Walk(s.Else, func(st code.Stmt) {
			if set, ok := st.(*code.Set); ok {
				for _, x := range set.Values {
					if c, ok := x.(*code.Call); ok && c.Callee == OpMatchLit {
						found = true
					}
				}
			}
		}, nil)
	}
	if !found {
		v.fail("an in-place read without the recording primitive as its alternative in a Document function")
	}
}

// callOf returns the value of a one-value assignment if it is a call.
func callOf(xs []code.Expr) *code.Call {
	if len(xs) != 1 {
		return nil
	}
	c, _ := xs[0].(*code.Call)
	return c
}

// hasCall reports whether the expression x contains a call for which pred holds.
func hasCall(x code.Expr, pred func(*code.Call) bool) bool {
	if x == nil {
		return false
	}
	found := false
	code.Walk(&code.Block{Stmts: []code.Stmt{&code.Assert{Cond: x}}}, nil, func(y code.Expr) {
		if c, ok := y.(*code.Call); ok && pred(c) {
			found = true
		}
	})
	return found
}

// breaks reports whether the block b contains a break of the loop label.
func breaks(b *code.Block, label string) bool {
	found := false
	code.Walk(b, func(s code.Stmt) {
		if br, ok := s.(*code.Break); ok && br.Label == label {
			found = true
		}
	}, nil)
	return found
}
