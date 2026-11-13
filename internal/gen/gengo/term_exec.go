package gengo

import (
	"fmt"
	"strconv"
	"strings"

	"zeroparse.dev/zp/internal/compile"
	irc "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/core"
	"zeroparse.dev/zp/internal/ir/exec/execgo"
	"zeroparse.dev/zp/internal/ir/exec/lower"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Terms printed from Exec IR.
//
// Every term is lowered by lower.Term and printed from Exec IR: the calls of the compositions of the
// runtime (TermBinary, ...) with the failure of each checked after it, and the leaves, which read
// the captures and variables of the action.

// The runtime methods that print the operations term.nodeType (of the Node runtime) and
// term.castError.
const (
	rtTermNodeType   = "TermNodeType"
	rtTermCastFailed = "TermCastFailed"
	rtTermStruct     = "TermStruct"
	rtTermSetField   = "TermSetField"
)

// loweredBody returns the body of a function that returns the value of the term t of Core IR and its
// failure, printed from Exec IR. The scope s and the locals resolve the leaves (leaf).
func (g *Generator) loweredBody(t irc.Term, s *compile.Scope, locals map[string]string) string {
	return g.lowered(t, s, locals, "", false)
}

// loweredStmts returns the statements that compute the value of the term t into a local that they
// declare, and the name of the local, as loweredBody does. The locals of the computation are scoped
// to a block.
func (g *Generator) loweredStmts(t irc.Term, s *compile.Scope, locals map[string]string) (string, string) {
	res := fmt.Sprintf("tr%d", g.LoweredTerms)
	text := g.lowered(t, s, locals, res, false)
	return fmt.Sprintf("var %s any\n{\n%s}\n", res, text), res
}

// lowered prints the term t from Exec IR, into the local result if it is not empty; dispatch makes a
// predicate that compares the text of a capture with literals test them at once (text-dispatch).
func (g *Generator) lowered(t irc.Term, s *compile.Scope, locals map[string]string, result string, dispatch bool) string {
	if t == nil {
		panic(fmt.Errorf("rule %s: a term without Core IR", g.Cur.Name))
	}
	l, err := lower.Term(t, lower.TermOptions{Result: result, StructCall: g.Table == "trules", Unbounded: g.NoBudget, Proj: g.Proj,
		FieldCompare: g.Table == "trules" && g.Opt.On("field-compare"), TextDispatch: dispatch})
	if err != nil {
		panic(fmt.Errorf("rule %s: %w", g.Cur.Name, err))
	}
	res := execgo.Body(l.Module, l.Func, g.termTarget(s, locals), nil)
	g.LoweredTerms++
	return res.Text
}

// actionBody returns the body of a function that returns the value of the term ct of Core IR and its
// failure.
func (g *Generator) actionBody(ct irc.Term, s *compile.Scope, locals map[string]string) string {
	return g.loweredBody(ct, s, locals)
}

// predicate returns the body of the function of a predicate with the term ct of Core IR: a
// membership of a captured text in literals tests them at once without limit controls
// (text-dispatch).
func (g *Generator) predicate(ct irc.Term, s *compile.Scope) string {
	return g.lowered(ct, s, nil, "", g.Opt.On("text-dispatch"))
}

// leaf returns the expression that reads the capture (or, if variable is set, the variable) name of
// an action: a local that holds it (a parameter or $lhs, $rhs and $op, which may hold boxes that
// stand for their values), the local that a frame printed in place keeps it in, or the slot of the
// scope s.
func (g *Generator) leaf(name string, variable bool, s *compile.Scope, locals map[string]string) string {
	if variable {
		return fmt.Sprintf("c.Var(%d)", g.Prog.Vars.Index(name))
	}
	if l, ok := locals[name]; ok {
		return g.pick("UnboxValue(", "(") + l + ")"
	}
	if g.caps != nil {
		return g.caps[name]
	}
	return fmt.Sprintf("c.Cap(%d)", s.Slots[name])
}

// termTarget returns the Go implementation of the operations of a lowered term whose leaves the scope
// s and the locals resolve.
func (g *Generator) termTarget(s *compile.Scope, locals map[string]string) *execgo.Target {
	handles := map[string]string{ops.Datum: "any", ops.Fail: "any", ops.Str: "string", ops.Name: "string",
		ops.Act: "*" + g.ctxType(), ops.FType: "*FieldType", ops.NodeType: g.pick("*nodeTypeKinds", "uint16"),
		ops.Seq: g.pick("[]*Node", "[]any"), ops.Obj: g.pick("*Node", "TVal"), lower.HandleOperand: "TOperand"}
	return &execgo.Target{
		Handle: func(name string) string { return handles[name] },
		FuncName: func(name string) string {
			n, ok := core.GoTermExports[name]
			if !ok {
				panic("gengo: the runtime does not export the composition " + name)
			}
			return "c.P." + n
		},
		Op: func(c *exec.Call, a []string) string {
			switch c.Callee {
			case lower.OpTermConst:
				switch v := c.Imm.(lower.TermConst).Value.(type) {
				case nil:
					return "any(nil)"
				case string:
					return "any(" + strconv.Quote(v) + ")"
				default:
					return fmt.Sprintf("any(%v)", v)
				}
			case lower.OpTermRead:
				switch x := c.Imm.(lower.TermRead).Term.(type) {
				case *irc.SlotRef:
					return g.leaf(x.Name, false, s, locals)
				case *irc.Local:
					return g.leaf(x.Name, false, s, locals)
				case *irc.VarRef:
					return g.leaf(x.Name, true, s, locals)
				}
			case lower.OpTermCtx:
				return "c"
			case lower.OpTermName:
				return strconv.Quote(c.Imm.(string))
			case lower.OpTermField:
				// The name is cut from the name of the member, as ActCtx.Member cuts it: compareTextControlled
				// charges a constant for the same string data, and a literal of the bare name would share its
				// data with the names of the captures, so a member would charge less here than in the runtime.
				f := c.Imm.(lower.TermField)
				if cut := len(f.Name) - len(f.Field); cut > 0 {
					return fmt.Sprintf("%s[%d:]", strconv.Quote(f.Name), cut)
				}
				return strconv.Quote(f.Field)
			case lower.OpTermNodeType:
				if g.Table == "trules" {
					return g.nodeTypeID(c.Imm.(string))
				}
				return "c.P." + rtTermNodeType + "(" + strconv.Quote(c.Imm.(string)) + ")"
			case lower.OpTermCastType:
				return g.castTypeVar(c.Imm.(lower.TermCast).Cast)
			case lower.OpTermRaise:
				return "c.TermRaise(" + a[0] + ")"
			case lower.OpTermCastError:
				x := c.Imm.(lower.TermCast).Cast
				return fmt.Sprintf("c.%s(%s, %s, %d, %d)", rtTermCastFailed, a[0], g.castTypeVar(x), x.Line, x.Col)
			case lower.OpTermTextEqual:
				return "c.P.TextEqual(" + a[0] + ", " + a[1] + ")"
			case lower.OpTermFromStr, lower.OpTermFromObj, "val.fromBool":
				return "any(" + a[0] + ")"
			case "term.kids":
				return "len(c.P.KidStack)"
			case lower.OpTermElemsAbove:
				return "c.P.Kids(" + a[0] + ")"
			case "seq.step":
				return "c.P.Spend()"
			case "seq.len":
				return "len(" + a[0] + ")"
			case "val.unbox":
				// A parameter may hold a box, which stands for its value.
				return g.pick("UnboxValue(", "(") + a[0] + ")"
			case lower.OpTermKeep:
				return fmt.Sprintf("c.P.KidStack = append(c.P.KidStack, %s[%s])", a[0], a[1])
			case lower.OpTermFieldOperand:
				return fmt.Sprintf("c.TermFieldOperand(%s, %q)", a[0], c.Imm.(string))
			case lower.OpTermOperand:
				return "TOperand{V: " + a[0] + "}"
			case lower.OpTermCompare:
				return fmt.Sprintf("c.TermCompare(rt.%s, %s, %s)", ops.OperatorIdent(c.Imm.(string), false, false), a[0], a[1])
			case lower.OpTermUncontrolled:
				return "c.P.Control == nil"
			case lower.OpTermTextIn:
				var eqs []string
				for _, lit := range c.Imm.([]string) {
					eqs = append(eqs, a[0]+" == "+strconv.Quote(lit))
				}
				return "(" + strings.Join(eqs, " || ") + ")"
			case lower.OpTermCopyList:
				return g.pick("any(c.NewList(", "(c.NewList(") + a[0] + "))"
			case lower.OpTermStruct:
				t := c.Imm.(lower.TermNew).New
				return fmt.Sprintf("c.%s(%q, %d)", rtTermStruct, t.Type, len(t.Fields))
			case lower.OpTermSetField:
				return fmt.Sprintf("c.%s(%s, %q, %s)", rtTermSetField, a[0], c.Imm.(string), a[1])
			}
			if t, ok := c.Imm.(lower.TermNew); ok && strings.HasPrefix(c.Callee, lower.OpTermNew) {
				// The typed runtime calls the constructor of the type (tmk_T, see typedRuntime).
				return "tmk_" + t.New.Type + "(" + strings.Join(append([]string{"c", strconv.FormatBool(t.New.Final)}, a...), ", ") + ")"
			}
			panic("gengo: term operation " + c.Callee)
		},
		Prec: goPrec,
		Name: termConstName,
		Pure: func(c *exec.Call) bool {
			switch c.Callee {
			case lower.OpTermConst, lower.OpTermName, lower.OpTermField, lower.OpTermCtx, lower.OpTermFromStr, "val.fromBool",
				"val.unbox", "seq.len", lower.OpTermOperand, lower.OpTermTextIn:
				return true
			}
			return false
		},
	}
}

// loweredTermNames returns the names of the runtime functions and methods that only lowered terms
// call: the compositions of core.GoTermExports and the operations of the leaves that the runtime
// exports for them.
func loweredTermNames() []string {
	names := []string{rtTermNodeType, rtTermCastFailed, rtTermStruct, rtTermSetField, "TermRaise", "TermFieldOperand", "TermCompare"}
	for _, n := range core.GoTermExports {
		names = append(names, n)
	}
	return names
}

// castTypeVar returns the variable of the checked type of the cast x.
func (g *Generator) castTypeVar(x *irc.Cast) string {
	return g.FieldTypeVar(x.Key, x.Check.(compile.Ty), func(v string, t *compile.FieldType) string {
		return fmt.Sprintf("var %s = %s\n", v, goFieldType(t))
	})
}

// termConstName returns the constant of the runtime that names the code v that the composition callee
// takes as its argument number arg: the operator of binary, unary and logic, and the kind of a member
// read.
func termConstName(callee string, arg int, v int64) (string, bool) {
	var names []string
	switch {
	case callee == "binary" && arg == 0:
		names = ops.BinaryNames
	case callee == "unary" && arg == 0:
		names = ops.UnaryNames
	case callee == "logic" && arg == 0:
		names = ops.LogicalNames
	case callee == "member" && arg == 1:
		if v < 0 || int(v) >= len(ops.MemberKinds) {
			return "", false
		}
		return "rt.Member" + ops.MemberKinds[v].Name, true
	default:
		return "", false
	}
	if v < 0 || int(v) >= len(names) {
		return "", false
	}
	return "rt." + ops.Ident(names[v]), true
}
