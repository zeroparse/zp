// Package parse defines the types of a parse that the ZeroParse engine (package
// zeroparse.dev/zp) and generated Go parsers share: the limits a parse runs under, the position
// unit of its input, the errors it reports, the edits of a Document and the statistics it
// counts. A result of either producer has the same type, so errors.Is, errors.As and code
// written against these types work with both.
//
// Package parse is the stable API of the runtime module. The package that holds it,
// zeroparse.dev/zp/runtime, is support for generated code and has no stable API.
package parse
