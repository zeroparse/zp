package parser

// workKind is a counter of the work that the frontend does outside the generated parser. A build
// with the tag zp_workcount counts it (see Work), so that a test can check that the work grows
// linearly with the source whether or not it allocates; in other builds charge does nothing and
// compiles to nothing.
type workKind int

const (
	// lexerWork counts the code points of the source that the lexer reads (see lexer.at).
	lexerWork workKind = iota
	// builderWork counts what the builder of the grammar, its layout replay and the error messages
	// of a source read: the code points of the source, the comments and comment gaps of the source
	// tree, and the tokens of the lexer.
	builderWork
	workKinds
)
