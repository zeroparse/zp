package vmlower

import (
	"fmt"
	"maps"
	"slices"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/code"
	flowpkg "zeroparse.dev/zp/internal/ir/code/flow"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/runtime/program"
)

// CoreOf returns the Core IR of prog after the bytecode pipeline, the module that Compile emits
// the match code from.
func CoreOf(prog *compile.Program) (*ir.Module, error) {
	m, err := elaborate.CoreControlled(prog, nil)
	if err != nil {
		return nil, fmt.Errorf("internal error: Core IR: %w", err)
	}
	if _, err := corepass.Run(m, &corepass.Env{}, corepass.RunOptions{}); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	return m, nil
}

// FlowModule lowers the rule with index rule of the Core IR m, which CoreOf made, to a Flow module
// of one function named name, with the options of the VM's bodies (budget charges on, Node values),
// and runs the Flow pipeline on it.
func FlowModule(m *ir.Module, rule int, name string) (*code.Module, error) {
	opts := flowlower.Options{Budget: true, Recovers: flowlower.Recovers(m), Node: true}
	fm, err := flowlower.Lower(m, m.Rules[rule], name, opts)
	if err != nil {
		return nil, fmt.Errorf("internal error: flow of %s: %w", name, err)
	}
	if _, err := flowpkg.Run(fm, &flowpkg.Env{}, flowpkg.RunOptions{VerifyEach: true}); err != nil {
		return nil, fmt.Errorf("internal error: flow of %s: %w", name, err)
	}
	return fm, nil
}

// SelectFlows sets the Flow bodies of the rules of flows (rule index to the Flow module that
// FlowModule made) in bc, a module that Compile emitted from prog and the Core IR m. A rule whose
// Flow form uses an operation that the bytecode Flow bodies do not yet have is an error, and the
// module keeps no body of that rule.
func SelectFlows(bc *bytecode.Module, prog *compile.Program, m *ir.Module, flows map[int]*code.Module) error {
	c := &compiler{prog: prog, core: m, m: bc, strs: map[string]int{}}
	for i, s := range bc.Strings {
		if _, ok := c.strs[s]; !ok {
			c.strs[s] = i
		}
	}
	for _, rule := range slices.Sorted(maps.Keys(flows)) {
		fn := flowpkg.FuncOf(flows[rule])
		f, err := c.flow(fn, rule)
		if err != nil {
			return fmt.Errorf("rule %s: %w", fn.Name, err)
		}
		if bc.Flows == nil {
			bc.Flows = map[int]*bytecode.Flow{}
		}
		bc.Flows[rule] = f
	}
	return nil
}

// flow returns the Flow body of the function fn, the flow form of rule index rule.
func (c *compiler) flow(fn *code.Func, rule int) (*bytecode.Flow, error) {
	a := flowpkg.AttrsOf(fn)
	if a.Frame || a.Cut != "" || a.Binds {
		return nil, fmt.Errorf("a function of this shape has no bytecode Flow body yet")
	}
	if a.Rule == nil || a.Rule.ID != rule {
		return nil, fmt.Errorf("the function is not the body of rule %d", rule)
	}
	s := &flowSel{c: c, f: &bytecode.Flow{}, regs: map[string]int{}, labels: map[string][]int{}}
	for _, p := range fn.Locals {
		r := s.local(p.Name)
		zero, err := zeroOf(p.Type)
		if err != nil {
			return nil, fmt.Errorf("local %s: %w", p.Name, err)
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowConst, Dst: []int{r}, Imm: zero})
	}
	if a.Start != "" {
		r, ok := s.regs[a.Start]
		if !ok {
			return nil, fmt.Errorf("the entry position %s is not a local", a.Start)
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowPos, Dst: []int{r}})
	}
	// The body is the block of the fail label: running to its end succeeds with the result, and an
	// exit to the label fails. Both returns are emitted after the body.
	for _, st := range fn.Body.Stmts {
		if err := s.stmt(st); err != nil {
			return nil, err
		}
	}
	res, ok := s.regs[a.Result]
	if !ok {
		return nil, fmt.Errorf("the result %q is not a local", a.Result)
	}
	yes := s.reg()
	s.emit(bytecode.FlowInstr{Op: bytecode.FlowConst, Dst: []int{yes}, Imm: true})
	s.emit(bytecode.FlowInstr{Op: bytecode.FlowReturn, Args: []int{res, yes}})
	failAt := len(s.f.Code)
	nothing, no := s.reg(), s.reg()
	s.emit(bytecode.FlowInstr{Op: bytecode.FlowConst, Dst: []int{nothing}})
	s.emit(bytecode.FlowInstr{Op: bytecode.FlowConst, Dst: []int{no}, Imm: false})
	s.emit(bytecode.FlowInstr{Op: bytecode.FlowReturn, Args: []int{nothing, no}})
	for _, j := range s.labels[a.Fail] {
		s.f.Code[j].Target = failAt
	}
	delete(s.labels, a.Fail)
	if len(s.labels) > 0 {
		return nil, fmt.Errorf("an exit to a block that does not enclose it")
	}
	return s.f, nil
}

// zeroOf returns the zero value of a local of type t in the Flow body's registers.
func zeroOf(t code.Type) (any, error) {
	switch t.Kind {
	case code.KBool:
		return false, nil
	case code.KInt:
		return 0, nil
	case code.KHandle:
		return nil, nil
	}
	return nil, fmt.Errorf("a local of type %s has no bytecode Flow register", t)
}

// flowSel selects the instructions of one Flow function: regs maps its locals to registers, and
// labels holds the jumps that wait for the end of each block they exit.
type flowSel struct {
	c      *compiler
	f      *bytecode.Flow
	regs   map[string]int
	labels map[string][]int
}

func (s *flowSel) reg() int {
	s.f.Regs++
	return s.f.Regs - 1
}

func (s *flowSel) emit(in bytecode.FlowInstr) int {
	s.f.Code = append(s.f.Code, in)
	return len(s.f.Code) - 1
}

func (s *flowSel) local(name string) int {
	if r, ok := s.regs[name]; ok {
		return r
	}
	r := s.reg()
	s.regs[name] = r
	return r
}

func (s *flowSel) block(b *code.Block) error {
	if b == nil {
		return nil
	}
	for _, st := range b.Stmts {
		if err := s.stmt(st); err != nil {
			return err
		}
	}
	if b.Label != "" {
		end := len(s.f.Code)
		for _, j := range s.labels[b.Label] {
			s.f.Code[j].Target = end
		}
		delete(s.labels, b.Label)
	}
	return nil
}

func (s *flowSel) stmt(st code.Stmt) error {
	switch x := st.(type) {
	case *code.Block:
		return s.block(x)
	case *code.Comment:
		return nil
	case *code.Exit:
		j := s.emit(bytecode.FlowInstr{Op: bytecode.FlowJump})
		s.labels[x.Label] = append(s.labels[x.Label], j)
		return nil
	case *code.Return:
		if len(x.Values) != 2 {
			return fmt.Errorf("a return of %d values has no bytecode Flow form", len(x.Values))
		}
		v, err := s.expr(x.Values[0])
		if err != nil {
			return err
		}
		ok, err := s.expr(x.Values[1])
		if err != nil {
			return err
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowReturn, Args: []int{v, ok}})
		return nil
	case *code.Do:
		return s.call(x.Call, nil)
	case *code.If:
		if x.Init != nil {
			if err := s.set(x.Init); err != nil {
				return err
			}
		}
		cond, err := s.expr(x.Cond)
		if err != nil {
			return err
		}
		jf := s.emit(bytecode.FlowInstr{Op: bytecode.FlowJumpFalse, Args: []int{cond}})
		if err := s.block(x.Then); err != nil {
			return err
		}
		if x.Else == nil {
			s.f.Code[jf].Target = len(s.f.Code)
			return nil
		}
		jend := s.emit(bytecode.FlowInstr{Op: bytecode.FlowJump})
		s.f.Code[jf].Target = len(s.f.Code)
		if err := s.block(x.Else); err != nil {
			return err
		}
		s.f.Code[jend].Target = len(s.f.Code)
		return nil
	case *code.Let:
		dst := make([]int, len(x.Names))
		for i, n := range x.Names {
			dst[i] = s.local(n)
		}
		return s.assign(dst, x.Value)
	case *code.Set:
		return s.set(x)
	}
	return fmt.Errorf("statement %T has no bytecode Flow form", st)
}

// set assigns the places of x: with a call of as many results as places, its results, otherwise
// one value per place, all evaluated before the first assignment.
func (s *flowSel) set(x *code.Set) error {
	dst := make([]int, len(x.Dst))
	for i, p := range x.Dst {
		switch {
		case p.Reg && p.Name == flowpkg.Pos:
			dst[i] = posDst
		case p.Reg:
			return fmt.Errorf("an assignment to register %s has no bytecode Flow form", p.Name)
		case p.Name == "":
			dst[i] = -1
		default:
			r, ok := s.regs[p.Name]
			if !ok {
				return fmt.Errorf("assignment to undeclared local %s", p.Name)
			}
			dst[i] = r
		}
	}
	if len(x.Values) == 1 && len(dst) > 1 {
		return s.assign(dst, x.Values[0])
	}
	if len(x.Values) != len(dst) {
		return fmt.Errorf("assignment of %d values to %d places", len(x.Values), len(dst))
	}
	tmp := make([]int, len(x.Values))
	for i, v := range x.Values {
		r, err := s.expr(v)
		if err != nil {
			return err
		}
		tmp[i] = r
	}
	for i, d := range dst {
		s.store(d, tmp[i])
	}
	return nil
}

// posDst is the destination of an assignment to the position register.
const posDst = -2

// store assigns the register r to the destination d (-1 discards it, posDst sets the position).
func (s *flowSel) store(d, r int) {
	switch {
	case d == posDst:
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowSetPos, Args: []int{r}})
	case d >= 0:
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowMove, Dst: []int{d}, Args: []int{r}})
	}
}

// assign stores the value of e in the registers dst (-1 discards one): a call with as many
// results as registers stores its results, and any other expression one value.
func (s *flowSel) assign(dst []int, e code.Expr) error {
	if c, ok := e.(*code.Call); ok {
		return s.call(c, dst)
	}
	if len(dst) != 1 {
		return fmt.Errorf("an expression of one value assigned to %d places", len(dst))
	}
	r, err := s.expr(e)
	if err != nil {
		return err
	}
	s.store(dst[0], r)
	return nil
}

// expr evaluates e and returns the register that holds its value.
func (s *flowSel) expr(e code.Expr) (int, error) {
	switch x := e.(type) {
	case *code.Local:
		r, ok := s.regs[x.Name]
		if !ok {
			return 0, fmt.Errorf("read of undeclared local %s", x.Name)
		}
		return r, nil
	case *code.Const:
		var v any
		switch val := x.Value.(type) {
		case nil:
		case bool:
			v = val
		case int64:
			v = int(val)
		default:
			return 0, fmt.Errorf("constant %v of type %s has no bytecode Flow form", val, x.Type)
		}
		r := s.reg()
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowConst, Dst: []int{r}, Imm: v})
		return r, nil
	case *code.Reg:
		if x.Name != flowpkg.Pos {
			return 0, fmt.Errorf("register %s has no bytecode Flow form", x.Name)
		}
		r := s.reg()
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowPos, Dst: []int{r}})
		return r, nil
	case *code.Unary:
		if x.Op != "!" {
			return 0, fmt.Errorf("unary %s has no bytecode Flow form", x.Op)
		}
		a, err := s.expr(x.X)
		if err != nil {
			return 0, err
		}
		r := s.reg()
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowNot, Dst: []int{r}, Args: []int{a}})
		return r, nil
	case *code.Call:
		r := s.reg()
		if err := s.call(x, []int{r}); err != nil {
			return 0, err
		}
		return r, nil
	}
	return 0, fmt.Errorf("expression %T has no bytecode Flow form", e)
}

// call emits the operation c with its results in dst (nil for none). Each operation selected
// here is one Flow operation whose charges and effects its VM body reproduces.
func (s *flowSel) call(c *code.Call, dst []int) error {
	for _, d := range dst {
		if d == posDst {
			return fmt.Errorf("an operation that assigns the position has no bytecode Flow form")
		}
	}
	args := func(es []code.Expr) ([]int, error) {
		out := make([]int, len(es))
		for i, e := range es {
			r, err := s.expr(e)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	}
	switch c.Callee {
	case flowpkg.OpMarkState:
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowMark, Dst: dst})
	case flowpkg.OpReset:
		a, err := args(c.Args)
		if err != nil {
			return err
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowReset, Args: a[:1]})
	case flowpkg.OpPeek:
		if len(dst) != 3 {
			return fmt.Errorf("flow.peek with %d results", len(dst))
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowPeek, Dst: dst})
	case flowpkg.OpGuardFails:
		g := c.Imm.(*ir.First)
		a, err := args(c.Args)
		if err != nil {
			return err
		}
		of := bytecode.FlowGuardOf{Lit: g.Lit, Rune: g.Rune, Fold: g.Fold, Class: -1, Depth: g.Depth}
		if !g.Lit {
			of.Class = int(s.c.class(g.Class))
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowGuard, Dst: dst, Args: a, Imm: of})
	case flowpkg.OpExpect:
		x := c.Imm.(flowpkg.Expectation)
		a, err := args(c.Args)
		if err != nil {
			return err
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowExpect, Args: a[:1], Imm: bytecode.FlowExpectOf{Exp: s.expID(x.Exp), Message: x.Message}})
	case flowpkg.OpScopeValue:
		// The value of a scope that is neither a tuple nor a list of elements is its body's value
		// (program.Shaper.One), so the operation is a move.
		sh := c.Imm.(ir.Shape)
		if sh.Elems || sh.Tuple {
			return fmt.Errorf("a scope value of a tuple or of elements has no bytecode Flow form yet")
		}
		a, err := args(c.Args[:1])
		if err != nil {
			return err
		}
		if len(dst) != 1 {
			return fmt.Errorf("flow.scopeValue with %d results", len(dst))
		}
		s.store(dst[0], a[0])
	case flowpkg.OpUnfresh:
		a, err := args(c.Args)
		if err != nil {
			return err
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowUnfresh, Args: a})
	case flowpkg.OpCallRule:
		x := c.Imm.(flowpkg.RuleCall)
		if want, got := s.c.core.Rules[x.Rule].Name, s.c.ruleName(x.Rule); want != got {
			return fmt.Errorf("rule %d is %s in the Core IR and %s in the module", x.Rule, want, got)
		}
		s.emit(bytecode.FlowInstr{Op: bytecode.FlowCall, Dst: dst, Imm: bytecode.FlowCallOf{Rule: x.Rule, Level: x.Level}})
	default:
		return fmt.Errorf("operation %s has no bytecode Flow form yet", c.Callee)
	}
	return nil
}

// expID returns the parser's ID of the expectation id of the Core IR: the fixed expectations keep
// their IDs, and the others are the strings of the module after the fixed table.
func (s *flowSel) expID(id ir.ExpID) int {
	if id <= ir.ExpID(program.IDEndLine) {
		return int(id)
	}
	return int(program.NumFixedDescs) + int(s.c.exp(id))
}

// ruleName returns the name of the rule of index i in the module.
func (c *compiler) ruleName(i int) string {
	if i < 0 || i >= len(c.m.Rules) {
		return ""
	}
	return c.m.Strings[c.m.Rules[i].Name]
}
