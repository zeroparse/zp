package tsgen

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/compile"
	irc "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/core"
	"zeroparse.dev/zp/internal/ir/exec/exects"
	"zeroparse.dev/zp/internal/ir/exec/lower"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Terms printed from Exec IR.
//
// Every term is lowered by lower.Term and printed from Exec IR as the statements of a function: the
// calls of the compositions of the runtime's Parser (termBinary, ...) with the failure of each checked
// after it, a step charged for each subterm under budget controls, and the leaves, which read the
// captures and variables of the action.

// termStmts returns the statements of a function that returns the value of the term ct of Core IR,
// printed from Exec IR. The scope s and the locals resolve the leaves (leaf). A predicate that
// compares the text of a capture with literals tests them at once without limit controls if
// dispatch is set (text-dispatch).
func (g *Generator) termStmts(ct irc.Term, s *compile.Scope, locals map[string]string, dispatch bool) string {
	if ct == nil {
		panic(fmt.Errorf("rule %s: a term without Core IR", g.Cur.Name))
	}
	l, err := lower.Term(ct, lower.TermOptions{Steps: !g.NoBudget, Unbounded: g.NoBudget, Proj: g.Proj, TextDispatch: dispatch})
	if err != nil {
		panic(fmt.Errorf("rule %s: %w", g.Cur.Name, err))
	}
	text, err := exects.Statements(l.Module, l.Func, g.termTarget(s, locals), 1)
	if err != nil {
		panic(fmt.Errorf("rule %s: %w", g.Cur.Name, err))
	}
	g.LoweredTerms++
	return text
}

// actionFunc returns the function that evaluates the term ct of Core IR, declared with the name name.
func (g *Generator) actionFunc(name string, ct irc.Term, s *compile.Scope, locals map[string]string) string {
	body := g.termStmts(ct, s, locals, false)
	return fmt.Sprintf("function %s(%s: Actx): [Value, Value] {\n%s}\n\n", name, tsActxParam(body), body)
}

// predicateFunc returns the arrow function of a predicate with the term ct of Core IR: a membership
// of a captured text in literals tests them at once without limit controls (text-dispatch).
func (g *Generator) predicateFunc(ct irc.Term, s *compile.Scope) string {
	body := g.termStmts(ct, s, nil, g.Opt.On("text-dispatch"))
	return fmt.Sprintf("(%s): [Value, Value] => {\n%s}", tsActxParam(body), body)
}

// leaf returns the expression that reads the capture (or, if variable is set, the variable) name of
// an action: a local that holds it (a parameter or $lhs, $rhs and $op, which may hold boxes that
// stand for their values), or the slot of the scope s.
func (g *Generator) leaf(name string, variable bool, s *compile.Scope, locals map[string]string) string {
	if variable {
		return fmt.Sprintf("c.variable(%d)", g.Prog.Vars.Index(name))
	}
	if l, ok := locals[name]; ok {
		return "unboxValue(" + l + ")"
	}
	return fmt.Sprintf("c.cap(%d)", s.Slots[name])
}

// termTarget returns the TypeScript implementation of the operations of a lowered term whose leaves
// the scope s and the locals resolve.
func (g *Generator) termTarget(s *compile.Scope, locals map[string]string) *exects.Target {
	return &exects.Target{
		Self:     "c.p",
		Handle:   core.TSHandle,
		FuncName: core.TSTermMethod,
		Reg:      func(name string) string { panic("tsgen: a lowered term reads the register " + name) },
		Op: func(c *exec.Call, a []string) string {
			switch c.Callee {
			case lower.OpTermConst:
				switch v := c.Imm.(lower.TermConst).Value.(type) {
				case nil:
					return "null"
				case string:
					return TsString(v)
				case int:
					return tsInt(v)
				default:
					return fmt.Sprint(v)
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
			case lower.OpTermName, lower.OpTermNodeType:
				return TsString(c.Imm.(string))
			case lower.OpTermField:
				return TsString(c.Imm.(lower.TermField).Field)
			case lower.OpTermCastType:
				return g.castTypeVar(c.Imm.(lower.TermCast).Cast)
			case lower.OpTermUncontrolled:
				return "c.p.control === null"
			case lower.OpTermTextIn:
				var eqs []string
				for _, lit := range c.Imm.([]string) {
					eqs = append(eqs, a[0]+" === "+TsString(lit))
				}
				return "(" + strings.Join(eqs, " || ") + ")"
			case lower.OpTermCastError:
				x := c.Imm.(lower.TermCast).Cast
				return fmt.Sprintf("castFailure(c, %s, %s, %d, %d)", a[0], g.castTypeVar(x), x.Line, x.Col)
			case lower.OpTermTextEqual:
				// == of two strings, which charges as binary does.
				return fmt.Sprintf("c.p.%s(%d, %s, %s)[0] === true", core.TSTermMethod("binary"), ops.BinaryCode("=="), a[0], a[1])
			case lower.OpTermFromStr, lower.OpTermFromObj, "val.fromBool":
				return a[0]
			case lower.OpTermSpend, "seq.step":
				return "c.spend()"
			case "term.kids":
				return "c.p.kidStack.length"
			case "seq.len":
				return a[0] + ".length"
			case "val.unbox":
				// A parameter may hold a box, which stands for its value.
				return "unboxValue(" + a[0] + ")"
			case lower.OpTermKeep:
				return fmt.Sprintf("c.p.kidStack.push(%s[%s]!)", a[0], a[1])
			case lower.OpTermCopyList:
				return "c.newList(" + a[0] + ")"
			case lower.OpTermElemsAbove:
				return "c.p.kids(" + a[0] + ")"
			case lower.OpTermStruct:
				return "c.termStruct(" + TsString(c.Imm.(lower.TermNew).New.Type) + ")"
			case lower.OpTermSetField:
				return fmt.Sprintf("c.termSetField(%s, %s, %s)", a[0], TsString(c.Imm.(string)), a[1])
			}
			panic("tsgen: term operation " + c.Callee)
		},
	}
}

// castTypeVar returns the constant of the checked type of the cast x.
func (g *Generator) castTypeVar(x *irc.Cast) string {
	return g.FieldTypeVar(x.Key, x.Check.(compile.Ty), tsFieldTypeDecl)
}

// tsCastFailure is the check function castFailure of fieldCheckSource.
var tsCastFailure = strings.TrimSpace(`
// castFailure returns the failure of a cast of v to t, a type that v does not fit (castValue).
function castFailure(c: Actx, v: Value, t: FieldType, gline: number, gcol: number): string {
  const e = c.p.makeError(c.start, []);
  return (` + "`cannot cast ${typeName(v)} to ${t.text} at input position ${c.start} (line ${e.line}, column ${e.col})${gline > 0 ? `; cast at grammar position ${gline}:${gcol}` : \"\"}`" + `);
}`)
