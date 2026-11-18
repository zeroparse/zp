// Package runtime defines what a parse runs under and reports that every ZeroParse parser shares:
// the limits of a parse and its limit errors, the stream iterator, and the statistics of a
// Document. The nodes a parse produces are in package value, the position unit and the edits of
// its input in package input, and traces and profiles in package instrument. Parsers loaded from
// compiled grammars (package vm), compiled from grammars (package compiler) and generated Go
// parsers return the same types, so errors.Is, errors.As and code written against these types work
// with all of them.
//
// The runtime packages are stable in intent at v0.x: a change needs a reason and is announced.
// The library of generated Go parsers and the package value/impl are support for generated code;
// they are judged by the library protocol version and have no stable API.
package runtime
