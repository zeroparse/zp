// Package parse defines the types of a parse that the ZeroParse engine (package
// zeroparse.dev/zp) and generated Go parsers share: the nodes a parse produces with their JSON
// form, traversal and cursors, the stream iterator, the limits a parse runs under, the position
// unit of its input, the errors it reports, the edits of a Document and the statistics it
// counts. A result of either producer has the same type, so errors.Is, errors.As and code
// written against these types work with both.
//
// Package parse is part of the API of generated Go parsers and of zeroparse.dev/zp, and it is
// stable in intent at v0.x: a change needs a reason and is announced. The other packages of the
// runtime module, zeroparse.dev/zp/runtime and zeroparse.dev/zp/runtime/noderep, are support for
// generated code; they are judged by the runtime protocol version and have no stable API.
package parse
