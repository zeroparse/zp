# The Public API

> [!WARNING]
> This page describes the target architecture. The code may still differ from it.

The public API is the set of packages under `pkg/`. It is the only Go API that users import, and it joins the runtime
side and the compiler side of the module, which [Package Layering](layering.md) keeps apart. The decided shapes, their
names and their roles are listed in [Decided Interfaces](records/072-release-package-layout.md#decided-interfaces) of
[Record 072](records/072-release-package-layout.md). The other shapes follow the current API, and each is settled in the
package documentation as its part is implemented.

## Grammar Source and Grammar AST

`pkg/grammar` owns the grammar source and the operations on it, and its subpackage `pkg/grammar/ast` owns the grammar
AST and its JSON form. The AST imports only the standard library, so the front end, the IR and the diagnostics can
build on it, while `pkg/grammar` builds on the parser and the formatter.

- A **grammar source** is one grammar input: a named value made from bytes, a string, a reader or a path in a file
  system (the constructors `FromBytes`, `FromString`, `FromReader` and `FromFS`), with an explicit or detected encoding
  (grammar syntax or JSON). Its fields are unexported, which guarantees that it has exactly one supplier. A reader is
  read in full when the source is loaded, because a diagnostic needs the text of any line.
- **Loading** a source gives the grammar AST. A program that builds an AST by hand can validate it. Formatting and
  conversion between the two encodings are in `pkg/grammar` too.
- Action expressions have one name in the AST, in the JSON form and in the syntax: the name that the specification
  gives Action Expressions.
- A grammar source shares no type with the parse input of the runtime input package. The parse input is the abstraction
  of what a parser reads: position units, sequential reads of a stream, the chunks and edits of a Document, and charging
  against a budget. A grammar source is read once at compile time and may span several files. The two share only the
  naming convention of their constructors.
- The command line reads grammars through the same type, from a file system.

## The Compiler Entry Points

`pkg/compiler` turns a grammar into executable artifacts: a parser on the VM (the default backend), a parser from an
AST that a program builds, the reference parser for differential tests, a compiled file, host-language parsers, and the
output schema.

- One optimization setting, `Optimize`, holds a profile (`Speed`, `Memory` or `Size`) and overrides of individual flags.
  The closure backend, the VM and the generators all receive it, through the option `WithOptimize`. The table from
  profile names to values is in the driver.
- A compiled file is made in one of three modes: parse, recognize or document.

## Tool Packages

- **`pkg/lint`** runs checks over the grammar AST and returns findings. Each finding has a check name, a severity, a
  source range and, where one exists, a suggested rewrite. It also gives the complexity class, which is the public way
  to reach the classification.
- **`pkg/sample`** generates inputs that a grammar accepts, with a coverage-guided mode.
- **`pkg/editfuzz`** generates, runs, replays and minimizes edit sequences, which are versioned JSON.

## The Bytecode Package

`pkg/bytecode` holds the module, the instruction set and its encoding, the compiled-file format and its validation. The
instruction set is public, and its fields are documented as unstable until the release.

## The Runtime Package and Its Subpackages

`pkg/runtime` is the parser interface. A parser parses an input, recognizes it, parses a stream and emits each top-level
element, creates a Document for edits, and gives the output schema. Parse options set limits, the context, the position
unit, expectations, missing tokens, tracing, profiling and diagnostics.

Recognition is expressed only by the `Recognize` method of a parser; no option switches a parse into a recognition. The
API of generated parsers offers the same choice with the same meaning.

- **`input`**: position units, locations, text edits and the kinds of change of a Document.
- **`value`**: nodes, cursors, traversal, JSON and syntax errors. A recovered error is a node value, and a failure in a
  trace event carries a syntax error too.
- **`value/impl`**: the node representation that the runtime and generated parsers share.
- **`schema`**: the types of the output schema, its JSON form, validation and comparison.
- **`query`**: structural queries, compiled against an output schema. The core is a programmatic pattern checked
  against the schema; a text syntax is a later front end to it.
- **`instrument`**: the model of trace events and profiles; see [Observability](observability.md).
- **`vm`**: loads a compiled file into a parser, without the grammar and without the compiler.
- **`golib`**: support declarations that only generated Go parsers use. It is not an API that users call.

## The Pass Infrastructure

The pass infrastructure is internal, in `internal/pass`. Its model is in [The Pass Pipeline](pass-pipeline.md).

## Related Topics

- [Architecture](architecture.md): the overview and the package layout.
- [Package Layering](layering.md): the rules on which packages may import which, including the runtime side.
- [The Pass Pipeline](pass-pipeline.md): the pass model that the pass infrastructure implements.
- [Observability](observability.md): the run-time observation model.
- [Record 072](records/072-release-package-layout.md): the decision on the public layer, its decided interfaces and its
  alternatives.
