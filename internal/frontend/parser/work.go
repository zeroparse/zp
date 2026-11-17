package parser

// The frontend counts the work it does outside the generated parser, so that a test can check that
// the work grows linearly with the source whether or not it allocates. Every read of the source of
// the lexer (lexer.src) and of the builder of the grammar (canonical.src), and of the tokens of the
// parser (parser.toks) and of the error messages (messages.toks), goes through an accessor: at, runes
// for a run of code points, or tok for the current token of the parser. An accessor counts each code
// point or token it returns as a unit of work with charge; loops over local copies call charge
// themselves. A build with the tag zp_workcount keeps the counts (see Work); in other builds the
// accessors are plain reads and charge does nothing, so that the default build compiles as if there
// were no counter. TestWorkReadsGoThroughAccessors keeps every read of those fields on an accessor.

// workKind is a counter of work.
type workKind int

const (
	// lexerWork counts the code points of the source that the lexer reads.
	lexerWork workKind = iota
	// builderWork counts what the builder of the grammar, its layout replay (which drives the
	// parser's primitives; ParseReference charges them too) and the error messages of a source read:
	// the code points of the source, the comments and comment gaps of the source tree, and the
	// tokens of the lexer.
	builderWork
	workKinds
)
