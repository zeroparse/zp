package program

// The kinds and the associativities of the operators of an Operators Rule, as the grammar spells
// them.
const (
	OpPrefix  = "prefix"
	OpPostfix = "postfix"
	OpInfix   = "infix"

	AssocLeft  = "left"
	AssocRight = "right"
	AssocNone  = "none"
)
