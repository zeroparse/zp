package gogen

import (
	"fmt"
	"strconv"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gen"
	irc "zeroparse.dev/zp/internal/ir/coreir"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/execcore"
	"zeroparse.dev/zp/internal/ir/exec/execgo"
	"zeroparse.dev/zp/internal/ir/exec/execlower"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Terms printed from Exec IR.
//
// A term of a kind that execlower.Term lowers is printed from Exec IR: the calls of the compositions of the
// runtime (TermBinary, ...) with the failure of each checked after it, the leaves as the term printer
// prints them. Any other term is printed by the term printer (term), whole.

// The runtime methods that print the operations term.nodeType (of the Node runtime) and
// term.castError.
const (
	rtTermNodeType   = "TermNodeType"
	rtTermCastFailed = "TermCastFailed"
	rtTermStruct     = "TermStruct"
	rtTermSetField   = "TermSetField"
)

// loweredBody returns the body of a function that returns the value of the term t of Core IR, printed
// from Exec IR, or reports false if t is printed by the term printer. The scope s and the locals resolve
// the leaves as for term.
func (g *Generator) loweredBody(t irc.Term, s *compile.Scope, locals map[string]string) (string, bool) {
	return g.lowered(t, s, locals, "")
}

// loweredStmts returns the statements that compute the value of the term t into a local that they
// declare, and the name of the local, or reports false as loweredBody does. The locals of the
// computation are scoped to a block.
func (g *Generator) loweredStmts(t irc.Term, s *compile.Scope, locals map[string]string) (string, string, bool) {
	res := fmt.Sprintf("tr%d", g.LoweredTerms)
	text, ok := g.lowered(t, s, locals, res)
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("var %s any\n{\n%s}\n", res, text), res, true
}

func (g *Generator) lowered(t irc.Term, s *compile.Scope, locals map[string]string, result string) (string, bool) {
	if t == nil || g.fieldCompared(t) {
		return "", false
	}
	l, err := execlower.Term(t, execlower.TermOptions{Result: result, StructCall: g.Table == "trules", Unbounded: g.NoBudget})
	if err == execlower.ErrUnsupported {
		return "", false
	}
	if err != nil {
		panic(fmt.Errorf("rule %s: %w", g.Cur.Name, err))
	}
	res := execgo.Body(l.Module, l.Func, g.termTarget(s, locals), nil)
	g.LoweredTerms++
	return res.Text, true
}

// actionBody returns the body of a function that returns the value of the term ct of Core IR, printed
// from Exec IR if execlower.Term lowers it, and otherwise the body that the term printer prints for t, the
// grammar term that ct is built from.
func (g *Generator) actionBody(ct irc.Term, t grammar.Term, s *compile.Scope, locals map[string]string) string {
	if body, ok := g.loweredBody(ct, s, locals); ok {
		return body
	}
	return g.termBody(t, s, locals)
}

// predicate returns the body of the function of a predicate with the term ct of Core IR, built from
// the grammar term t: a membership of a captured text in literals is a switch (predicateBody).
func (g *Generator) predicate(ct irc.Term, t grammar.Term, s *compile.Scope) string {
	if g.Opt.On("text-dispatch") {
		if _, _, ok := gen.LiteralTextMembership(t); ok {
			return g.predicateBody(t, s, nil)
		}
	}
	return g.actionBody(ct, t, s, nil)
}

// fieldCompared reports whether the typed table compares a field read of t without boxing it
// (field-compare), which the term printer prints.
func (g *Generator) fieldCompared(t irc.Term) bool {
	if g.Table != "trules" || !g.Opt.On("field-compare") {
		return false
	}
	found := false
	irc.WalkTerm(t, func(x irc.Term) {
		b, ok := x.(*irc.Binary)
		if ok && comparison(b.Op) && (coreFieldRead(b.L) || coreFieldRead(b.R)) {
			found = true
		}
	})
	return found
}

// coreFieldRead reports whether t reads a field of a value, x.F (fieldRead).
func coreFieldRead(t irc.Term) bool {
	m, ok := t.(*irc.Member)
	return ok && (m.Name == "" || m.Name[0] != '$')
}

// termTarget returns the Go implementation of the operations of a lowered term whose leaves the scope
// s and the locals resolve.
func (g *Generator) termTarget(s *compile.Scope, locals map[string]string) *execgo.Target {
	handles := map[string]string{ops.Datum: "any", ops.Fail: "any", ops.Str: "string", ops.Name: "string",
		ops.Act: "*" + g.ctxType(), ops.FType: "*FieldType", ops.NodeType: g.pick("*nodeTypeKinds", "uint16"),
		ops.Seq: g.pick("[]*Node", "[]any"), ops.Obj: g.pick("*Node", "TVal")}
	return &execgo.Target{
		Handle: func(name string) string { return handles[name] },
		FuncName: func(name string) string {
			n, ok := execcore.GoTermExports[name]
			if !ok {
				panic("gogen: the runtime does not export the composition " + name)
			}
			return "c.P." + n
		},
		Op: func(c *exec.Call, a []string) string {
			switch c.Callee {
			case execlower.OpTermConst:
				switch v := c.Imm.(execlower.TermConst).Value.(type) {
				case nil:
					return "any(nil)"
				case string:
					return "any(" + strconv.Quote(v) + ")"
				default:
					return fmt.Sprintf("any(%v)", v)
				}
			case execlower.OpTermRead:
				switch x := c.Imm.(execlower.TermRead).Term.(type) {
				case *irc.SlotRef:
					return g.term(&grammar.CaptureRef{Name: x.Name}, s, locals)
				case *irc.Local:
					return g.term(&grammar.CaptureRef{Name: x.Name}, s, locals)
				case *irc.VarRef:
					return g.term(&grammar.VarRef{Name: x.Name}, s, locals)
				}
			case execlower.OpTermCtx:
				return "c"
			case execlower.OpTermName:
				return strconv.Quote(c.Imm.(string))
			case execlower.OpTermField:
				// The name is cut from the name of the member, as ActCtx.Member cuts it: compareTextControlled
				// charges a constant for the same string data, and a literal of the bare name would share its
				// data with the names of the captures, so a member would charge less here than in the runtime.
				f := c.Imm.(execlower.TermField)
				if cut := len(f.Name) - len(f.Field); cut > 0 {
					return fmt.Sprintf("%s[%d:]", strconv.Quote(f.Name), cut)
				}
				return strconv.Quote(f.Field)
			case execlower.OpTermNodeType:
				if g.Table == "trules" {
					return g.nodeTypeID(c.Imm.(string))
				}
				return "c.P." + rtTermNodeType + "(" + strconv.Quote(c.Imm.(string)) + ")"
			case execlower.OpTermCastType:
				return g.castTypeVar(c.Imm.(execlower.TermCast).Cast)
			case execlower.OpTermRaise:
				return "panic(" + a[0] + ")"
			case execlower.OpTermCastError:
				x := c.Imm.(execlower.TermCast).Cast
				return fmt.Sprintf("c.%s(%s, %s, %d, %d)", rtTermCastFailed, a[0], g.castTypeVar(x), x.Line, x.Col)
			case execlower.OpTermTextEqual:
				return "c.P.TextEqual(" + a[0] + ", " + a[1] + ")"
			case execlower.OpTermFromStr, execlower.OpTermFromObj, "val.fromBool":
				return "any(" + a[0] + ")"
			case "term.kids":
				return "len(c.P.KidStack)"
			case execlower.OpTermElemsAbove:
				return "c.P.Kids(" + a[0] + ")"
			case "seq.step":
				return "c.P.Spend()"
			case execlower.OpTermStruct:
				t := c.Imm.(execlower.TermNew).New
				return fmt.Sprintf("c.%s(%q, %d)", rtTermStruct, t.Type, len(t.Fields))
			case execlower.OpTermSetField:
				return fmt.Sprintf("c.%s(%s, %q, %s)", rtTermSetField, a[0], c.Imm.(string), a[1])
			}
			if t, ok := c.Imm.(execlower.TermNew); ok && strings.HasPrefix(c.Callee, execlower.OpTermNew) {
				// The typed runtime calls the constructor of the type (tmk_T, see typedRuntime).
				return "tmk_" + t.New.Type + "(" + strings.Join(append([]string{"c", strconv.FormatBool(t.New.Final)}, a...), ", ") + ")"
			}
			panic("gogen: term operation " + c.Callee)
		},
		Prec: goPrec,
		Name: termConstName,
		Pure: func(c *exec.Call) bool {
			switch c.Callee {
			case execlower.OpTermConst, execlower.OpTermName, execlower.OpTermField, execlower.OpTermCtx, execlower.OpTermFromStr, "val.fromBool":
				return true
			}
			return false
		},
	}
}

// loweredTermNames returns the names of the runtime functions and methods that only lowered terms
// call: the compositions of execcore.GoTermExports and the operations of the leaves that the runtime
// exports for them.
func loweredTermNames() []string {
	names := []string{rtTermNodeType, rtTermCastFailed, rtTermStruct, rtTermSetField}
	for _, n := range execcore.GoTermExports {
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
