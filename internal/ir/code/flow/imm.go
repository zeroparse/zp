package flow

import (
	"fmt"
	"strconv"
	"strings"

	"zeroparse.dev/zp/internal/ir/core"
)

// ImmString returns the immediate imm of a Flow operation as listings print it (code.Op.FormatImm):
// its static data, without the addresses of the Core nodes it points to, so that a listing of the
// same function is the same in every run.
func ImmString(imm any) string {
	switch x := imm.(type) {
	case Silence:
		s := fmt.Sprintf("%+d", x.Delta)
		if x.Site != 0 {
			s += fmt.Sprintf(" site %d", x.Site)
		}
		if x.Ok {
			s += " ok"
		}
		return s
	case Site:
		return fmt.Sprintf("site %d", int(x))
	case RunWalk:
		return fmt.Sprintf("site %d build=%t key-env=%t", x.Site, x.Build, x.KeyEnv)
	case Slot:
		return fmt.Sprintf("slot %d", int(x))
	case Expectation:
		if x.Message {
			return fmt.Sprintf("exp %d message", x.Exp)
		}
		return fmt.Sprintf("exp %d", x.Exp)
	case Spread:
		if x {
			return "spread"
		}
		return "one"
	case RuleCall:
		return fmt.Sprintf("rule %d level %d", x.Rule, x.Level)
	case Outline:
		s := fmt.Sprintf("%s slots %v", nodeKind(x.Node), core.SlotNames(x.Slots))
		if x.Build {
			s += " build"
		}
		return s
	case Chars:
		return strconv.Quote(string(x))
	case List:
		if x {
			return "list"
		}
		return "tuple"
	case Room:
		return fmt.Sprintf("room %d", int(x))
	case Attach:
		s := names(x.Names)
		if x.Frame {
			s += " frame"
		}
		return s
	case StateMark:
		var parts []string
		for _, p := range []struct {
			on   bool
			name string
		}{{x.Env, "env"}, {x.Trail, "trail"}, {x.Rec, "rec"}} {
			if p.on {
				parts = append(parts, p.name)
			}
		}
		return "state " + strings.Join(append([]string{"pos"}, parts...), " ")
	case Conversion:
		if x.Owned {
			return fmt.Sprintf("rule %d owned", x.Rule)
		}
		return fmt.Sprintf("rule %d", x.Rule)
	case Predicate:
		return core.ActionExprString(x.Node.Term) + capture(x.Capture)
	case Binding:
		return x.Node.Var + " = " + core.ActionExprString(x.Node.Term) + capture(x.Capture)
	case Action:
		return core.ActionExprString(x.Term) + capture(x.Capture)
	case *Names:
		s := names(x)
		if x.Declared {
			s += " declared"
		}
		return s
	case core.ExpID:
		return fmt.Sprintf("exp %d", x)
	case *core.Lit:
		s := strconv.Quote(x.Text)
		if x.Fold {
			s += "i"
		}
		return s
	case *core.Class:
		return fmt.Sprintf("class exp %d", x.Exp)
	case *core.First:
		return fmt.Sprintf("guard exp %d depth %d", x.Exp, x.Depth)
	case core.AnchorKind:
		return [...]string{"begin input", "end input", "begin line", "end line"}[x]
	case *core.Terminal:
		return "terminal"
	case *core.ClassRun:
		return fmt.Sprintf("%d classes", len(x.Classes))
	case core.Shape:
		return fmt.Sprintf("%+v", x)
	case core.Node:
		return nodeKind(x)
	}
	return fmt.Sprintf("%T", imm)
}

// nodeKind names the kind of the Core node n.
func nodeKind(n core.Node) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", n), "*core.")
}

func names(n *Names) string {
	if n == nil {
		return "[]"
	}
	return fmt.Sprint(n.List)
}

// capture prints the slots of a capture, and where it reads them from the current frame.
func capture(c Capture) string {
	s := fmt.Sprintf(" slots %v", c.Names)
	if c.Frame {
		s += " frame"
	}
	return s
}
