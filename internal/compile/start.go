package compile

import (
	"fmt"
	"slices"
	"strings"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/diag"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
)

// resolveStarts reads the declaration attributes of rules and returns the
// start rules, the rules marked @start where parsing may begin, in declaration
// order, and the default one, which Parse uses when no start rule is named.
// Every declaration attribute other than @start and @start(default) is an
// error, and so is @start on a hidden rule. A grammar needs a start rule;
// with several, one of them must be the default.
func resolveStarts(rules []*grammar.RuleDef, control *cancel.Control) (starts []string, def string, errs diag.ErrorList) {
	errorf := func(pos grammar.Pos, format string, args ...any) {
		errs = append(errs, &diag.Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
	}
	var second *grammar.Attribute
	for _, rd := range rules {
		control.Poll()
		var start *grammar.Attribute
		isDefault := false
		for _, a := range rd.Attrs {
			control.Poll()
			if a.Pragma || a.Name != "start" {
				errorf(a.Diagnostic(), "unknown declaration attribute %s%s", annotationMarker(a), a.Name)
				continue
			}
			if start != nil {
				errorf(a.Diagnostic(), "duplicate @start on rule %s", rd.Name)
				continue
			}
			start = a
			switch {
			case len(a.Args) == 0 && len(a.KwArgs) == 0:
			case len(a.Args) == 1 && len(a.KwArgs) == 0 && isDefaultArg(a.Args[0]):
				isDefault = true
			default:
				errorf(a.Diagnostic(), "@start: the only argument is default")
			}
		}
		if start == nil || slices.Contains(starts, rd.Name) {
			continue // a rule defined twice is reported by the compiler
		}
		if HiddenRule(rd.Name) {
			errorf(start.Diagnostic(), "rule %s is hidden: a hidden rule cannot be a start rule", rd.Name)
			continue
		}
		starts = append(starts, rd.Name)
		if len(starts) == 2 {
			second = start
		}
		if isDefault {
			if def != "" {
				errorf(start.Diagnostic(), "more than one default start rule: %s and %s", def, rd.Name)
				continue
			}
			def = rd.Name
		}
	}
	switch {
	case len(errs) > 0:
	case len(starts) == 0:
		var pos grammar.Pos
		if len(rules) > 0 {
			pos = rules[0].Diagnostic()
		}
		errorf(pos, "the grammar has no start rule: mark each rule where parsing may begin with @start")
	case def == "" && len(starts) == 1:
		def = starts[0]
	case def == "":
		errorf(second.Diagnostic(), "several start rules (%s) and no default: mark one with @start(default)", strings.Join(starts, ", "))
	}
	return starts, def, errs
}

// startMarked reports whether rd is marked @start, so that a parse may begin at it.
func startMarked(rd *grammar.RuleDef) bool {
	return slices.ContainsFunc(rd.Attrs, func(a *grammar.Attribute) bool { return !a.Pragma && a.Name == "start" })
}

// isDefaultArg reports whether the attribute argument is the bare identifier
// default.
func isDefaultArg(arg any) bool {
	ref, ok := arg.(*grammar.Ref)
	return ok && ref.Name == "default" && ref.Level == ""
}

// CheckStart reports why name cannot begin a parse of prog: it is empty and there is no default
// start rule, it is hidden, it is not defined, or it is not a start rule.
func (prog *Program) CheckStart(name string) error {
	return checkStartName(prog.StartRules, prog.DefaultStartRule, prog.definesRule, name)
}

// checkStartName reports why name cannot begin a parse of a grammar with the start rules starts,
// the default start rule def and the rules that defined reports. It is the check that every entry
// to a parse makes, and that the classification of a grammar makes of its start rule.
func checkStartName(starts []string, def string, defined func(string) bool, name string) error {
	resolved := name
	if name == "" {
		resolved = def
	}
	if slices.Contains(starts, resolved) {
		return nil
	}
	if name == "" {
		return fmt.Errorf("no start rule is given")
	}
	if err := EntryError(name); err != nil {
		return err
	}
	if defined(name) {
		return fmt.Errorf("rule %s is not a start rule; parsing can begin only at a rule marked @start", name)
	}
	return fmt.Errorf("rule %s is not defined", name)
}

// ResolveStart returns the rule name selects: name, or the default start rule
// when name is empty.
func (prog *Program) ResolveStart(name string) string {
	if name == "" {
		return prog.DefaultStartRule
	}
	return name
}

// StartRule returns the start rule name selects.
func (prog *Program) StartRule(name string) (*Rule, error) {
	if err := prog.CheckStart(name); err != nil {
		return nil, err
	}
	return prog.ByName[prog.ResolveStart(name)], nil
}

// definesRule reports whether the grammar defines the rule name, without preparing a backend.
func (prog *Program) definesRule(name string) bool {
	if prog.Grammar != nil {
		return prog.ByName[name] != nil
	}
	m := prog.BytecodeModule()
	return slices.ContainsFunc(m.Rules, func(r bytecode.RuleInfo) bool { return m.Strings[r.Name] == name })
}
