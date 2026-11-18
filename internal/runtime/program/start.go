package program

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// HiddenRule reports whether the rule name is hidden: a name that begins with an underscore,
// which no parse may enter.
func HiddenRule(name string) bool { return strings.HasPrefix(name, "_") }

// EntryError returns the error for using the rule name as a start rule or another entry point of
// a parse, or nil if it can be one.
func EntryError(name string) error {
	if HiddenRule(name) {
		return fmt.Errorf("rule %s is hidden: a rule whose name begins with an underscore is not an entry point", name)
	}
	return nil
}

// CheckStartName reports why name cannot begin a parse of a grammar with the start rules starts,
// the default start rule def and the rules that defined reports. It is the check that every entry
// to a parse makes, and that the classification of a grammar makes of its start rule.
func CheckStartName(starts []string, def string, defined func(string) bool, name string) error {
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

// MissingUnsupported returns the error of an operation that does not support missing-token
// recovery.
func MissingUnsupported(operation string) error {
	return fmt.Errorf("missing-token recovery is not supported for %s; use Closure whole-input parsing or recognition", operation)
}

// ErrSchemaUnavailable reports that the checked grammar metadata an output schema is exported
// from is unavailable.
var ErrSchemaUnavailable = errors.New("output schema unavailable")

// ErrSchemaNeedsSource reports that an output schema was asked of a program loaded from a compiled
// grammar, which holds no schema. It wraps ErrSchemaUnavailable.
var ErrSchemaNeedsSource = fmt.Errorf("%w: a compiled grammar holds no schema; export the schema from the grammar source", ErrSchemaUnavailable)

// OperatorsRuleValue is the local name of the final folded value of an Operators Rule in its
// whole-rule action.
const OperatorsRuleValue = "value"
