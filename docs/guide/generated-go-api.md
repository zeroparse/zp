# The Generated Go API

A Go parser written by [`zp gen`](code-generation.md) has a small API: `parser.go` defines the functions that parse, `api.go` re-exports the result types of the `core` package (`zeroparse.dev/zp/core`) and the options of the library package under their own names (generated code imports the library as `rt "<module>/internal/libgo"`, its copy, or as `rt "zeroparse.dev/zp/codegen/go/lib"` with `-lib import`), and, with `-types`, `types/types.go` defines the Go types of the grammar. This guide describes
it, the call options that all entry points share, and recognition. Options that add typed values, incremental documents
and streams have their own guides:

| Option | Guide |
|:--|:--|
| `-types` | [Typed Values in Generated Go](generated-typed-values.md) |
| `-document` | [Incremental Documents in Generated Parsers](generated-documents.md) |
| `-stream` | [Streams in Generated Go](generated-streams.md) |
| `-recognize` | [Recognition](#recognition--recognize), below |

It assumes you have generated a parser as in the [quick start](code-generation.md#quick-start).

## The Generated API

Every generated package has the same small API, spread over `parser.go` and `api.go`:

```go
// Parse parses the whole input with the start rule. Options select position units and the nested rule-call limit.
func Parse(input string, opts ...ParseOption) (*Node, error)

// ParseRule parses the whole input with the start rule name, a rule marked @start; an empty
// name selects the grammar's default start rule.
func ParseRule(name, input string, opts ...ParseOption) (*Node, error)

type ParseOption = rt.ParseOption // an alias; the options are internal to the library
func WithUnit(unit PositionUnit) ParseOption
func WithLimits(limits Limits) ParseOption // Limits.Depth in every package; Compute and Input need -limits

type PositionUnit int
const (
	CodePoints PositionUnit = iota // the default
	Bytes
)

type Node = core.Node // Start, End, Text, Kind, Type, Rule, Len, Elem, Elems, Field, Fields, Value, IsTerminal, String, Clone; JSON as the engine's

type SyntaxError struct { Pos, Line, Col int; Expected, Messages []string }
func (e *SyntaxError) Error() string
func (e *SyntaxError) Message() string
type SyntaxErrors []*SyntaxError
```

Compared with the engine's API:

- **Expectations are recorded only if the parser was generated with `-expectations` and the call passes
  `WithExpectations()`.** Otherwise `SyntaxError.Expected` and `Messages` are empty (see
  [recording expectations](error-messages.md#recording-expectations)). A parser generated without the flag has no
  `WithExpectations`.
- **`Parse` and `ParseRule` replace `Parser.Parse`.** Position units and rule-call limits use
  functional options (`Parse(input, pairsparser.WithUnit(pairsparser.Bytes))`), like the engine. `ParseRule(name, input)` plays the role of
  `Parser.WithStart(name).Parse(input)`, and like it accepts only rules marked `@start`: another rule is an error,
  `rule pair is not a start rule; parsing can begin only at a rule marked @start`, and so is an unknown name,
  `rule nope is not defined`. An empty name selects the grammar's default start rule, as for
  `Parser.WithStart`. Like the engine, both require the rule to consume the whole input.
- **The types are those of the `core` package.** `pairsparser.Node` is `core.Node` of `zeroparse.dev/zp/core`, the
  type the engine returns, and likewise for the errors, limits and traversal, with the embedded library and with
  `-lib import`: a tree from either can be passed where the other is expected, and `errors.As` and `errors.Is` work
  with the types and values of either.
- **Fields.** As with the engine, a field value is a `*Node`, `int`, `string`, `bool` or `nil`: assert it to
  `*pairsparser.Node` (as the example does with `Field("Key")`), which is `*core.Node` only with `-lib import`.
- **Errors** carry the same positions and messages. Recovered errors work too, and the error value that a
  `#recover` produces in a typed result has the range of the skipped input (`Span`) and the `Message`.
  With this grammar, `rec.zp`:

  ```zp
  @start
  rule main =  stmt* $$
  rule stmt =  s:("let" " " name ";") #recover(skip=[^;]* ";") => $s
  rule name =  [a-z]+ #atomic
  ```

  generated into a package `recparser` with `-expectations` and parsed with `WithExpectations()`, the node is returned together with the recovered errors:

  ```go
  node, err := recparser.Parse("let a;let 1;let b;", recparser.WithExpectations())
  fmt.Println(node)
  var errs recparser.SyntaxErrors
  if errors.As(err, &errs) {
  	for _, e := range errs {
  		fmt.Println(e.Line, e.Col, e.Message())
  	}
  }
  ```

  ```text
  ([("let" " " "a" ";") error("1:11: syntax error: expected [a-z]") ("let" " " "b" ";")])
  1 11 syntax error: expected [a-z]
  ```

- A generated parser is **safe for concurrent use**: each call has its own state, and the tables are read-only after
  start-up. (Checked here with sixteen goroutines under the race detector.)
- Runtime errors in actions (such as division by zero) and exceeding the nesting limit return plain errors, as in the
  engine.

### What Is Inside the Package

| Part | Contents |
|:--|:--|
| Library | The parser state, memoization, left recursion, Operators Rule loop, attribute handling and the built-in functions of actions. It is **the same for every grammar**: the package `zeroparse.dev/zp/codegen/go/lib`, which the generated package imports (`-lib import`) or carries a copy of in `internal/libgo` (`-lib embed`, the default). Either way it depends only on `zeroparse.dev/zp/core`. |
| `api.go` | The names of `core` and the library that the users of the parser need (`Node`, `SyntaxError`, the options and so on), as aliases and wrappers. It is the same for grammars generated with the same options. |
| `types/types.go` | With `-types`, the package `<module>/types`: a Go type for each type of the grammar, `TNode`, `Error` and the tuple types. |
| Rule table | For each rule, the results of the engine's static analysis (memoized? left-recursion leader? capture names?). |
| Parser expressions | One method per expression, with literals, character classes and rule indices embedded as constants, and one per rule that is never memoized and has no captures, which calls the rule's body directly. |
| Value Actions and Predicate Actions | Go expressions; lambdas become Go function literals. |

The library is not part of `parser.go`, so the size of `parser.go` is that of the per-grammar parts: it grows with the
grammar, and a tiny grammar produces a file of a few kilobytes. Options such as `-types` and `-document` add code to
`parser.go`.

## Recognition (`-recognize`)

With `-recognize` (`Recognize` of `codegen.Go`), the generated package also has

```go
// Recognize checks that the whole input matches the start rule without building a tree, and returns the
// syntax errors Parse would return (SyntaxErrors for errors recovered with #recover).
func Recognize(input string, opts ...ParseOption) error
```

It is the engine's [`RecognizeOnly`](recognition-mode.md) in generated form: the grammar is compiled once more
with no rule building a value (except those whose values predicates read), into a second rule table in the same file.
Actions whose values are not needed by predicates are skipped. Actions needed by predicates still run and can report runtime errors. On the benchmark workloads it
takes about half the time of `Parse` (JSON 8.8 → 4.9 ms on a 262 KB input, CSV 4.0 → 1.8 ms), and it is the fastest
way ZeroParse has to validate input. The price is a larger file: the second table adds about as much code as the first.

## Invocation Options

```go
node, err := pairsparser.Parse(input,
    pairsparser.WithUnit(pairsparser.Bytes), pairsparser.WithLimits(pairsparser.Limits{Depth: 600_000}))
```

`Parse`, `ParseRule`, `Recognize` (when generated) and `ParseValue`/`ParseRuleValue` (with `-types`) accept the same functional options.
With `-typed-only` (`TypedOnly` of `codegen.Go`, which needs `-types`) the package has only `ParseValue` and `ParseRuleValue`, and `Recognize` with `-recognize`: `Parse`, `ParseRule` and the code that builds `*Node` values are not generated, and `-document` and `-stream` are rejected. With `-recognize-only` (`RecognizeOnly`) the package has only `Recognize`; it cannot be combined with `-types`, `-document` or `-stream`. When a typed entry point converts the Node of its rule (an action reads a field of a struct that has no plain Go form, such as a node, other than to copy it into a field of the same type, or a result type has no Go type of its own), the generated package keeps the unexported Node rule functions that conversion runs, so the saving is smaller for such a grammar.
No options preserves CodePoints and the generated depth default, normally 100,000. A positive `Limits.Depth`
overrides that default for one invocation, including typed direct parsing and node conversion. Zero selects the
generated default. Options apply in order; `Depth: Unlimited` returns a configuration error before matching, actions
or named-rule lookup, because generated code nests rule calls on the Go stack. A deeper parse fails with a
`*LimitError` of kind `LimitDepth`. A nil option panics when applied, as engine options do.
Parser pooling and concurrent calls keep their limits independent. Limits count nested recursive rule calls rather than
brackets or AST nodes, and raising the limit does not guarantee host-stack capacity.

## Cancellation and Compute Limits

A Go parser generated with `-limits` also accepts invocation-local
cancellation and compute and memory limits. Use the generated package's options and error
types; a package generated without the flag exports no `WithContext`, and a parse
that passes a context or a finite `Compute` or `Memory` limit returns an error saying that the parser was generated
without limit controls (a nil context and `Compute` and `Memory` limits that are zero or `Unlimited` set no control
and are accepted, as are any `Input` and `Depth`, which every parser checks):

```go
node, err := pairsparser.Parse(input,
    pairsparser.WithContext(ctx),
    pairsparser.WithLimits(pairsparser.Limits{
        Compute: 5_000_000,
    }))
```

A nil context has no deadline or cancellation signal. A zero limit selects its default: a zero `Compute` is
`max(2^20, 1024 * n)` for an input of `n` units, so every parse of a controlled parser has a budget. The generated
package exports `Unlimited`, which disables a limit; `Limits{Compute: Unlimited}` with no context runs without
a control. `Limits.Input` bounds the input length in the position unit; its default is `Unlimited`, and a longer input fails
before any parsing with a `*LimitError` of kind `LimitInput` (a generated stream fails when the parser reaches a unit beyond it).
`Limits.Memory` bounds the bytes of storage the parse takes into use, typed values included, and fails the parse with
kind `LimitMemory`; its default is `Unlimited`, and it works as in the engine
([Bounding Memory](cancellation-and-limits.md#bounding-memory)). Options apply once in
order; the last context wins, and a nonzero field of a later `WithLimits` overrides the same field of an earlier
one while a zero field leaves it unchanged. The counter
resets for every call and remain independent during concurrent parsing. The
same options work with named rules, recognition, direct typed parsing, and
Node-to-typed conversion.

`Compute` counts cumulative spend in one counter, one unit each: inspected
input elements, including input preparation, repeated scans and text
extraction, and execution and per-element spend in rule calls, loop iterations,
actions, memo lookup, graph/list construction, capture rollback, diagnostics
and typed conversion. A loop that only scans input, such as a repeated
character class, charges for every element it reads, so `Compute` bounds it.
Counts depend on the generated
artifact and position unit; a compute limit is not a character count, memory cap, or
elapsed-time limit. Backtracking never refunds spend. `Depth` bounds nesting
independently of the other limits and needs no control.

Use `errors.Is(err, context.Canceled)` or `context.DeadlineExceeded` for
cancellation, and `errors.Is(err, pairsparser.ErrLimit)` for an exhausted
limit. `errors.As` can retrieve a `*pairsparser.LimitError` with `Kind`,
`Limit`, `Used` and `Requested`. Interrupted entry points return their zero
result; recovery cannot turn an interruption into a successful partial tree.
Previously returned results stay valid, and a later invocation can retry.
Cancellation is cooperative during engine execution and is checked before input
preparation and before publishing a result.

## Type Names

With `-types`, the types of the grammar are declared in the package `<module>/types`, not in the parser package, so they
share no namespace with `parser.go` and `api.go`: a grammar type may be called `Node`, `ParseOption`, `Walk` or like any
other name of the parser package, and is then used as `types.Node`. The types package declares `TNode`, `Error`,
`ZPHooks` and, where used, `Unit`, `TupleN`, `ZPNodeTypes` itself and the constant `ZPSchema_<digest>` (which ties the parser to this package), and generation fails if a grammar type has one
of those names. Declarations that you add by hand to the generated packages must not reuse the names they declare. The
[generated parser contract](../../spec/runtime/generated-parsers.md) lists the rule.

## Related Topics

- [Generating Parsers](code-generation.md): the command, what is supported and keeping generated code up to date.
- [Reading Trees](reading-trees.md): the `Node` that the untyped entry points return.
- [Incremental Parsing](incremental.md) and [Streaming](streaming.md): the engine's versions of the same features.
