package desugar

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/diag"
)

// LinearAttribute is the name of the declaration attribute that declares linear cost: on the
// package clause for every start rule of the grammar, on a rule for that rule.
const LinearAttribute = "linear"

// IsLinear reports whether a is the declaration attribute @linear. The attribute only adds a check
// of the grammar, so the analyses that treat a rule with declaration attributes apart do not count
// it.
func IsLinear(a *grammar.Attribute) bool { return !a.Pragma && a.Name == LinearAttribute }

// LinearDeclarations reads the @linear declaration attributes of g. It returns whether the
// package clause declares the whole grammar linear and the rules that declare themselves linear,
// in declaration order. @linear takes no arguments and appears at most once on a declaration;
// the package clause takes no other declaration attribute. Each violation is an error. The other
// declaration attributes of rules are checked by ResolveStarts.
func LinearDeclarations(g *grammar.Grammar, control *cancel.Control) (whole bool, rules []string, errs diag.ErrorList) {
	errorf := func(pos grammar.Pos, format string, args ...any) {
		errs = append(errs, &diag.Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
	}
	declared := func(attrs []*grammar.Attribute, what string, others bool) bool {
		found := false
		for _, a := range attrs {
			control.Poll()
			if !IsLinear(a) {
				if !others {
					errorf(a.Diagnostic(), "unknown declaration attribute %s%s of the package clause", AnnotationMarker(a), a.Name)
				}
				continue
			}
			if found {
				errorf(a.Diagnostic(), "duplicate @linear on %s", what)
				continue
			}
			found = true
			if len(a.Args) != 0 || len(a.KwArgs) != 0 {
				errorf(a.Diagnostic(), "@linear takes no arguments")
			}
		}
		return found
	}
	whole = declared(g.File.Pkg.Attrs, "the package clause", false)
	for _, rd := range control.Rules(g) {
		if declared(rd.Attrs, "rule "+rd.Name, true) && !slices.Contains(rules, rd.Name) {
			rules = append(rules, rd.Name)
		}
	}
	return whole, rules, errs
}
