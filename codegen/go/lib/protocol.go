package libgo

import "zeroparse.dev/zp/core/noderep"

// The protocol is the interface between generated code and the library: the exported declarations
// of this package, the exported fields and methods of the types they reach in this package and in
// the node representation (package noderep), and what those declarations mean, including the
// encoding of the data generated parsers pass to the library (grammar tables, node kinds and
// expectation IDs). A protocol version names one state of that interface, independent of the
// release version, and increases whenever the interface changes. testdata/protocol.golden records
// the declarations of the current version, and a test fails when the two disagree; a change of
// meaning or data encoding that keeps the declarations needs the same bump, which no test detects.
//
// The library declares a constant ProtocolN for each protocol version N it implements. A parser
// generated with zp gen -lib import refers to the constant of the protocol it was generated for,
// so building it against a library that does not implement that protocol fails with an undefined
// name; regenerate the parser, or require a version of zeroparse.dev/zp that implements its
// protocol.
//
// The library reaches core and core/noderep, which a parser generated with zp gen -lib embed
// imports from zeroparse.dev/zp while it carries its own copy of the library. Package noderep
// declares exactly one constant ProtocolN: N is the protocol version that introduced the node
// representation it serves. The library refers to that constant (below), so a copy of the library
// fails to build, with an undefined name, against a core that serves another representation,
// whether the core is older or newer than the copy; regenerate the parser, or require a version of
// zeroparse.dev/zp that serves it. When a protocol version changes the node representation (the
// layout of its types, the values of its constants or the signatures of its functions, which
// noderep's testdata/representation.golden records), noderep replaces its constant with the one of
// that version, and the library refers to the new constant.

// Protocol15 is declared because the library implements protocol version 15.
const Protocol15 = 15

// The node representation of protocol version 15, which core/noderep must serve.
const _ = noderep.Protocol15
