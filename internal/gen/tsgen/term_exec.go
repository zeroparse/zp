package tsgen

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gen"
	irc "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/execcore"
	"zeroparse.dev/zp/internal/ir/exec/exects"
	"zeroparse.dev/zp/internal/ir/exec/execlower"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Terms printed from Exec IR.
//
// A term of a kind that lower.Term lowers is printed from Exec IR as the statements of a function: the
// calls of the compositions of the runtime's Parser (termBinary, ...) with the failure of each checked
// after it, a step charged for each subterm under budget controls, the leaves as the term printer
// prints them. Any other term is printed by the term printer (term), whole.

// termStmts returns the statements of a function that returns the value of the term ct of Core IR,
// built from the grammar term t, printed from Exec IR, or reports false if the term printer prints it.
// The scope s and the locals resolve the leaves as for term.
func (g *Generator) termStmts(ct irc.Term, t grammar.Term, s *compile.Scope, locals map[string]string) (string, bool) {
	if ct == nil || !g.castsChecked(t) {
		return "", false
	}
	l, err := execlower.Term(ct, execlower.TermOptions{Steps: !g.NoBudget})
	if err == execlower.ErrUnsupported {
		return "", false
	}
	if err != nil {
		panic(fmt.Errorf("rule %s: %w", g.Cur.Name, err))
	}
	text, err := exects.Statements(l.Module, l.Func, g.termTarget(s, locals), 1)
	if err != nil {
		panic(fmt.Errorf("rule %s: %w", g.Cur.Name, err))
	}
	g.LoweredTerms++
	return text, true
}

// castsChecked reports whether every cast of t checks its value: a cast to a type that every value
// fits has no term of Core IR, while the term printer charges a step for it under budget controls.
func (g *Generator) castsChecked(t grammar.Term) bool {
	ok := true
	compile.WalkTerm(t, func(x grammar.Term) {
		if c, is := x.(*grammar.Cast); is && g.Prog.ResolveCheck(c.Type) == nil {
			ok = false
		}
	})
	return ok
}

// actionFunc returns the function that evaluates the term ct of Core IR, built from the grammar term t,
// printed from Exec IR if it can be and by the term printer otherwise, declared with the name name.
func (g *Generator) actionFunc(name string, ct irc.Term, t grammar.Term, s *compile.Scope, locals map[string]string) string {
	if body, ok := g.termStmts(ct, t, s, locals); ok {
		return fmt.Sprintf("function %s(%s: Actx): Value {\n%s}\n\n", name, tsActxParam(body), body)
	}
	val := g.term(t, s, locals)
	return fmt.Sprintf("function %s(%s: Actx): Value {\n  return %s;\n}\n\n", name, tsActxParam(val), val)
}

// predicateFunc returns the arrow function of a predicate with the term ct of Core IR, built from the
// grammar term t: a membership of a captured text in literals is a switch (predicateTerm).
func (g *Generator) predicateFunc(ct irc.Term, t grammar.Term, s *compile.Scope) string {
	dispatch := false
	if g.Opt.On("text-dispatch") {
		_, _, dispatch = gen.LiteralTextMembership(t)
	}
	if !dispatch {
		if body, ok := g.termStmts(ct, t, s, nil); ok {
			return fmt.Sprintf("(%s) => {\n%s}", tsActxParam(body), body)
		}
	}
	body := g.predicateTerm(t, s)
	return fmt.Sprintf("(%s) => %s", tsActxParam(body), body)
}

// termTarget returns the TypeScript implementation of the operations of a lowered term whose leaves
// the scope s and the locals resolve.
func (g *Generator) termTarget(s *compile.Scope, locals map[string]string) *exects.Target {
	return &exects.Target{
		Self:     "c.p",
		Handle:   execcore.TSHandle,
		FuncName: execcore.TSTermMethod,
		Reg:      func(name string) string { panic("tsgen: a lowered term reads the register " + name) },
		Op: func(c *exec.Call, a []string) string {
			switch c.Callee {
			case execlower.OpTermConst:
				switch v := c.Imm.(execlower.TermConst).Value.(type) {
				case nil:
					return "null"
				case string:
					return TsString(v)
				case int:
					return tsInt(v)
				default:
					return fmt.Sprint(v)
				}
			case execlower.OpTermRead:
				switch x := c.Imm.(execlower.TermRead).Term.(type) {
				case *irc.SlotRef:
					return g.termBody(&grammar.CaptureRef{Name: x.Name}, s, locals)
				case *irc.Local:
					return g.termBody(&grammar.CaptureRef{Name: x.Name}, s, locals)
				case *irc.VarRef:
					return g.termBody(&grammar.VarRef{Name: x.Name}, s, locals)
				}
			case execlower.OpTermCtx:
				return "c"
			case execlower.OpTermName, execlower.OpTermNodeType:
				return TsString(c.Imm.(string))
			case execlower.OpTermCastType:
				return g.castTypeVar(c.Imm.(execlower.TermCast).Cast)
			case execlower.OpTermRaise:
				return "evalError(" + a[0] + " as string)"
			case execlower.OpTermCastError:
				x := c.Imm.(execlower.TermCast).Cast
				return fmt.Sprintf("castFailed(c, %s, %s, %d, %d)", a[0], g.castTypeVar(x), x.Line, x.Col)
			case execlower.OpTermTextEqual:
				// As the term printer does, == of two strings, which charges as it does.
				return fmt.Sprintf("c.p.%s(%d, %s, %s)[0] === true", execcore.TSTermMethod("binary"), ops.BinaryCode("=="), a[0], a[1])
			case execlower.OpTermFromStr, "val.fromBool":
				return a[0]
			case execlower.OpTermSpend:
				return "c.spend()"
			}
			panic("tsgen: term operation " + c.Callee)
		},
	}
}

// castTypeVar returns the constant of the checked type of the cast x.
func (g *Generator) castTypeVar(x *irc.Cast) string {
	return g.FieldTypeVar(x.Key, x.Check.(compile.Ty), tsFieldTypeDecl)
}

// tsCastFailed is the check function castFailed of fieldCheckSource.
var tsCastFailed = strings.TrimSpace(`
// castFailed raises the evaluation error of a cast of v to t, a type that v does not fit (castValue).
function castFailed(c: Actx, v: Value, t: FieldType, gline: number, gcol: number): never {
  const e = c.p.makeError(c.start, []);
  return evalError(` + "`cannot cast ${typeName(v)} to ${t.text} at input position ${c.start} (line ${e.line}, column ${e.col})${gline > 0 ? `; cast at grammar position ${gline}:${gcol}` : \"\"}`" + `);
}`)
