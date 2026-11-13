package core

// Family selects one of the two rule sets that a backend builds from a program. A parse runs one
// family from start to end, chosen by what it observes of the rule calls (Parser.NoPlain).
type Family uint8

const (
	// Inlined is the family of the parses that observe no rule call: a call site of a plain rule
	// (Rule.Plain) runs the rule with Parser.InvokePlain, which leaves out the call protocol of
	// Parser.Call that such a rule does not need.
	Inlined Family = iota
	// CallPreserving is the family of the parses that observe or memoize every rule call (traced
	// parses, whose tracer Call reports to, and the parses of a Document, which memoize plain rules
	// too): every call site of every rule goes through Parser.Call.
	CallPreserving
)

// FamilyOf returns the family that the parse p runs: CallPreserving when p sets NoPlain, and
// otherwise Inlined.
func FamilyOf(p *Parser) Family {
	if p.NoPlain {
		return CallPreserving
	}
	return Inlined
}
