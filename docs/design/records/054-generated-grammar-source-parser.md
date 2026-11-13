# 054. Generated Grammar Source Parser

- **Status**: Accepted
- **Author**: @ornew
- **Date**: 2026-10-13

## Summary

Parse ZeroParse grammar source with a typed Go parser generated from ZeroParse's
own complete grammar. It is the only frontend of the source APIs and commands:
`ParseGrammar`, `CompileSource`, `parse`, `compile`, `gen`, `fmt`, `lint` and
the language server. The hand-written recursive-descent parser stays in the
repository only as the oracle of the differential tests; no API or command
selects it. The
generated frontend accepts and rejects the same sources as the oracle, also at
the nesting limit of the language specification, which both apply through the
same lexer, and builds the same grammar ASTs. On a rejected source it reports the first error
at the place the oracle does or, in the forms named in the
[Diagnostic Contract](#diagnostic-contract), at another place of the same
mistake, and it reports no error at a declaration without a mistake. Message
text may differ. The speed of the generated frontend is recovered in the
generator and the optimizer, measured against the hand-written baseline below,
not by keeping a second frontend.

## Motivation

A parser described by ZeroParse's own grammar removes the duplicated syntax
logic of a hand-written parser and makes grammar evolution inspectable: the
grammar of the language is the executable source of its frontend. It also
makes the language's frontend a workload of the generator, so improvements of
generated parsers apply to it. The hand-written parser remains as an
independent oracle of acceptance, ASTs, source metadata and diagnostics.

## Goals

- Describe the complete grammar-file syntax in a canonical root `.zp` file.
- Generate a typed Go parser from that grammar without requiring users to run
  generation before building or using ZeroParse.
- Use one frontend for every source-consuming API and command, with no
  selection and no mutable global configuration.
- Hold the generated frontend to the oracle: outputs, source metadata,
  acceptance and the Diagnostic Contract.
- Recover the speed of the generated frontend in the generator and the
  optimizer, measured against the hand-written baseline.

## Non-Goals

This proposal does not replace the grammar compiler, type checker, formatter,
semantic analysis, or the `Compile(*grammar.Grammar, ...)` AST entry point.
It does not keep a selectable second frontend. It does not claim ZeroParse can generate its parser without a checked-in bootstrap
artifact.

## Proposed Design

### Grammar and Generated Parser

Keep one complete, normative source grammar at the repository root. Generate a
typed Go parser that constructs a concrete source AST, including nodes such as
`File`, `Def`, `RuleExpr`, `Type`, `ActionExpr` and `Comment`. A separate adapter
converts that source AST to the existing public `grammar.Grammar` API. Keep
source positions, comments and layout on the source-AST path for formatter,
LSP and diagnostic consumers. The public source metadata contract is defined
in [Source Metadata and Declaration Comments](057-source-info-and-comment-ownership.md)
for documentation and source tools, independently of frontend selection. The
grammar must preserve
contextual words, hard-newline rules, action and predicate blocks, Operators
Rules and malformed-input recovery.

#### Struct Definition Field Separators

Fields in a struct type definition (`type a = { ... }`) use semicolons or hard newlines.
Constructor fields and function arguments use commas. The source parser and
adapter must preserve field order, comments, positions and diagnostics while
checking separator parity against the manual parser. This changes source
syntax rather than field semantics in the public AST or compiled type table.

The source grammar accepts identifiers beginning with `_` at parse time.
Reserved-name checks belong to validation and compilation, so parsing such a
name succeeds while validation reports the reserved-name error. Attribute
names are lexically adjacent to `@`: both `@name` and `@name(...)` are valid,
while `@ name` is not. The root grammar's `Comments` field is always a list;
its surrounding whitespace is discarded by composing existing grammar
constructs. This local construction does not introduce general filtering or
typed-list collection semantics; those remain a separate proposal.

The checked-in generated Go source is the bootstrap artifact used during an
ordinary build. Regeneration must be reproducible and must not invoke the
parser being regenerated to construct itself at runtime. Keep the generator
and generated parser in package boundaries that avoid an import cycle through
`zp.CompileSource` or grammar compilation. A generation check fails when the
generated source is stale.

### Incremental Source Span Construction

The canonical source tree constructs normalized package-clause metadata in
`Package.Source`. Type and rule declarations, struct fields, operators lines and
constructor field initializers each retain a syntax-only `SourceSpan` (a typed
body node) and a `Source` boundary record. A type declaration's span runs from
the `type` keyword through the completed type specification. Leading and trailing boundary comments,
statement separators and following whitespace stay outside that span. Comments
inside a continuous type specification, including nested fields and grouping,
stay inside it. A separate typed whole-rule body production provides the parsed
name and specification. Both package clauses and type declarations use the
actual body node's range, so error recovery runs once. Speculative recognition of
a recovery-bearing body can duplicate diagnostics and must not be used to
construct its span. No inline action or untyped sequence-field lookup is required.

The internal offset adapter maps zero-based rune offsets to public one-based
`grammar.Pos` values. Its span-only update retains existing normalized ownership,
diagnostic token positions and formatter layout. A completed item's
boundary explicitly supplies the next item's leading and detached groups
(`Next`); a list's first item takes them from the boundary after its opening
delimiter (`Head`). `Positions.Items` joins these records. Reclassifying a raw
comment list in the adapter would duplicate or move ownership and is not part of
this design.

Expressions and types have no statement boundary to record. The canonical tree
keeps each comment-bearing gap between two tokens in `File.Gaps`, with its
groups, and the adapter assigns a gap's groups to the outermost node that starts
or ends at its token (`Positions.Claim`), in the order the hand-written parser
completes nodes. The grammar adds no wrapper node per expression.

`internal/syntax.FromSourceTree` builds the public `grammar.Grammar` from the
canonical tree: it decodes literals and character classes, flattens chains,
resolves lambda parameters, merges attribute runs and applies the checks that
the grammar does not make. The raw formatter state (`SourceInfo.Layout`, the
line breaks and the comment tokens they hold) depends on where the hand-written
parser takes each comment, so the adapter replays it: after the canonical
ownership is assigned, it reads the tokens of the source with the lexer once
more and walks the grammar in source order, calling the parser's own layout
primitives at the tokens where the parser calls them. The grammar already
decides every branch, so the replay needs no lookahead; the cost is one more
lexing pass per parse. A layout differential test compares every exported
field, the formatter-only ones included, and the text that `Format` writes, for
the same corpus, mutations and comment insertions. The differential tests compare, for every
repository grammar, documentation example, comment insertion, line-ending
variant and a seeded set of mutated sources, whether both frontends accept the
source and, when they do, the JSON form of the AST (fields and source metadata)
together with the Go type of each node. Differential span cases cover aliases, terminals,
structs, multiline groups, comments, separators, Unicode and CR/LF/CRLF. The
existing generated-consumer gate also compares typed type spans with the runtime
source tree. The Diagnostic Contract and formatting parity hold for the
frontend as a whole.

### Diagnostic Contract

The hand-written parser is the reference of the differential tests. The
generated parser keeps four guarantees against it. It does not have to produce the same message text, so a
diagnostic that the grammar cannot name dynamically needs no runtime support.
The adapter reports the checks the grammar does not make with the reference
messages where that is direct.

A syntax error says what is expected and what was found, the token at the
error: `expected an expression, found end of file`. The grammar names the
operands of rule expressions and of values, so a list of the tokens that start
one reads `an expression` or `a value`; white space and comment openers are
not listed, because they may stand anywhere. Explicit messages of the grammar
cover a pragma that does not exist (`unknown pragma #foo`), `#repeat` without
a count, a group or block that is not closed, and the old spellings that the
reference named; the adapter adds the name of the type to the messages of old
type definitions, as the reference does. A value
on the line after its `=>`, or after an operator where a line break ends the
statement, is reported as the reference reports it. Message differences that remain: `#ignorecase(...)` is
reported as a list of what may follow the expression, at the `#`, not as an
argument it does not take; the reference's messages about a pragma or its
arguments that do not follow directly, a `nil` lambda parameter, attributes
on a declaration other than a rule, and an Operators Rule followed by a pragma
other than `#ignorecase` are lists of what is expected; after a complete value
or expression, an error lists the operators that may follow (`=> 0x`), where
the reference names a missing separator; an attribute value lists the
literals besides `an expression`; and `#repeat(1,` expects the closing
parenthesis, where the reference expects the bound, which may be left out.

1. **Acceptance.** Both parsers accept exactly the same sources and build the
   same grammar for each.
2. **No misleading error.** The generated parser reports an error at a mistake
   of the source, not at a declaration that has none, and none before the first
   error of the reference unless that place holds a mistake too. This is a
   property of its recovery that is measured, not proved. The mutation tests
   count the sources that break it (the classes `extra` and `other earlier` of
   the [Mutation Measurement](#mutation-measurement)) and bound them by a cap.
   They are 90 and 15 of the 908,697 rejected sources of 12 seeds (about 1.2
   in 10,000), so the property holds for all but those, not for every source.
   An error on a declaration that no edit touched and where the reference
   reports none counts as `extra`, also at the start of the declaration after
   a broken one, unless it is the only error that the generated parser reports
   for the broken one (see `end`). Tables of valid grammars that use `rule`,
   `type` and `var` as names, with and without a following mistake, require
   that the valid text is accepted and, after a mistake, unflagged. A string
   or a character class that is not terminated is one error: the grammar
   failure that lies in it is dropped, and the lexical error stays. So is the
   failure exactly at the start of the declaration after it, if the literal
   holds an opening bracket that no later line closes or swallows the closing
   bracket of its group. A declaration here is a line that starts in the first
   column with `rule`, `type`, `var` or `@`, or an indented line that has the
   shape of a declaration, and none starts between the literal and the
   failure; an indented line with a keyword but no shape is a statement or a
   field of a group. A failure after the declaration's first token, such as an
   attribute without a name, is kept. A literal that holds a closing bracket
   which a later line closes again is not covered: the stray bracket is one
   more error. Nor are two mistakes in one block, a literal and a broken line
   that has the shape of a declaration or holds one after a semicolon: in `rule a = {\n x "{\n var = 1\n}\nrule c = d\n`
   and `rule a = {\n x "{\n}\n  rule b; rule c = d\n` the grammar also fails at
   `rule c`, which is valid, because a declaration starts after the literal.
3. **First error.** The first error is at the offset of the reference's first
   error, or in one of the forms below. Every form is bounded by a distance or
   by a check that the two errors belong to the same mistake, and the
   mutation tests cap the share of the sources in it.
4. **Recovery.** After a mistake, both parsers skip to the next declaration
   and go on. Inside a declaration, the fields of a struct, the items of an
   Operators Rule and the statements of an action block recover on their own,
   and a semicolon on a line ends a skipped statement. An independent broken
   declaration that follows is reported by both when the generated parser
   resumes at it (below); the cases that it does not are the `missed` classes
   of the measurement.

The forms of a first error at another place:

| Form | The generated parser reports | The reference reports | Bound |
| --- | --- | --- | --- |
| near | the next token or two of the same mistake | the first token | at most 16 code points apart, on the same line or the next, in the same declaration |
| shadowed | an error that the reference also reports | an earlier error of its own that a recovery inside a declaration that fails later drops | the same declaration |
| hint | the next error of the declaration, or an error the reference reports too | a lexical check that the grammar cannot make (a capture written with `.$`, an index bracket after a space, a character class after a name, an unknown pragma, an invalid number, range or escape, a `node` without its type argument) | later in the same declaration, or at an error the reference reports too |
| end | where a closing delimiter is missing, at the end of the declaration | an earlier mistake in the unclosed list | the declaration holds nothing but blanks after the error |
| head | the line, or the statement after a semicolon, that has the shape of a declaration inside a brace or parenthesis that is open | the keyword read as an item, failing later | the flagged text has a bare `=` or `~` after `rule`, `type` or `var`, outside strings, classes and comments |
| late | the mistake itself | a check of the whole declaration, made at its end (a `node` type with two type arguments) | earlier in the same declaration |
| unclosed | where the end of a block or group is missing, in the declaration or in a later one that the construct swallows | the mistake inside it | a brace or parenthesis that is open at the reference's error is still open at the generated parser's |
| dropped | the next error of the declaration | a mistake inside a part that the recovery drops because the declaration fails after it | later in the same declaration, with nothing open at the reference's error |

A hint is a check that the adapter makes after the grammar accepts the
source, so hints are reported only when the whole source is accepted by the
grammar: a syntax error anywhere in the source loses the hints of every
declaration, not only of the broken one, until it is fixed.

A recovery inside an attempt that fails later is dropped, so a mistake inside
an unclosed block is reported where the block's end is missing; see
[Recovery](../../guide/recovery.md). After a mistake the recovery skips from
the start of the failed declaration to the next declaration, following the
brackets that the declaration opens:

- At the top level (no bracket open), a line, or a statement after a
  semicolon, that starts with `rule`, `type` or `var` as a whole word is the
  next declaration, with or without an `=`; so are the attributes of a
  rule and an attribute named like a keyword.
- A group `(...)`, `{...}` or `[...]`, and the arguments of an attribute,
  that closes before a place with the shape of a declaration (below) is
  skipped whole: a line inside it is not taken for a declaration, whatever it
  starts with.
- A bracket that is not closed before such a place is not skipped whole. From
  its opening on, only a place with the shape of a declaration resumes.
- Strings, classes and block comments are skipped as units, so that a bracket
  or a keyword inside them counts for nothing.
- A run of attribute lines where no declaration starts is skipped as a unit
  with the lines of attributes that follow it, because the attributes reach
  the same end from each of its lines. At the top level the run stops before a
  line that starts with an attribute named like a keyword, which starts a
  declaration.

The shape of a declaration is `rule`, `type` or `var` at the start of a line
or after a semicolon, then a bare `=` or `~` on the line before any `;`, `{`,
`}`, `(` or `)` (what stands between them is not checked, so a missing or
wrong name still counts), or the word and at most a name at the end of the
source, or the attributes before a rule that has it. An `=` is bare when it is
not part of an operator (`:=`, `==`, `!=`, `<=`, `>=`, `+=`, `=>`) and not in
a string, class or block comment. A field, a statement or a name that is
called `rule`, `type` or `var` (`type string`, `var := 1`, `var / t`,
`var != "="`, `rule Foo`) has no such shape, so a body that is valid is not
read as declarations when a later mistake makes the parser recover. A named
argument of an attribute or a call that is called like a keyword
(`type = "x"`) has the shape; it is read as a declaration only inside a
bracket that is left open, because a group that closes is skipped whole.

The reference resynchronizes at a keyword anywhere on a line, also inside a
bracket. The generated parser does not, so a broken declaration in a
bracket that closes, or without an `=` inside one that does not, is not
resumed at. The cost is the `missed` classes of the measurement: an
independent mistake in such a declaration is reported once the earlier one is
fixed. A body that uses these words as names is ambiguous with such a broken
declaration; the reference reads it as a declaration (and may report errors in
the valid body), the generated parser does not. A declaration that starts in
the middle of a line is likewise not resumed at.

The recovery is linear. A test parses failing sources of 21 families (nested
groups, brackets, attributes, attribute lines, strings and comments) with
`Parse` and its limits, at a size where the parse takes over 20 ms and at 8
times that size, and requires that the time grows less than 24 times; the
measured ratios are 5.3 to 16.9 on a loaded machine. Under the default work limit of a generated
parser, a parse that grows faster than the source would end early with a limit
error, at a cost that grows with the source, and hide the growth. Looking for
the next declaration at each line of a run of attribute lines, each time to
the end of the run, grows with the square of the run: 4,000 such lines in an
open group take over a second, 32,000 lines over five minutes. The skip of attribute
lines above keeps it linear; the four families of attribute lines fail without
it, with ratios of 58 to 268.

### One Frontend

The source APIs and commands parse with the generated parser and its adapter;
there is no per-call or process-wide parser selection. `Compile` of an already
constructed `grammar.Grammar` is unaffected. A partial parse for editors runs
the parser with recovery and keeps the declarations that are complete (see
[Diagnostic Contract](#diagnostic-contract)); it observes the caller's context
for cancellation. The lexer applies the nesting limit of the
[language specification](../../../spec/language/lexical.md#limits), which the
oracle shares, so that a malformed or hostile source cannot build a tree that
the consumers of the grammar AST cannot walk. It reports `nesting too deep` at
the token that opens the level beyond the limit, with the syntax errors of the
source. The generated parser keeps its default bound on nested recursive rule
calls, far above what the limit allows, only to bound its recursion on a source
that the lexer rejects. The work of a parse is not limited: the recovery is
linear, and a caller stops a long parse with its context.

The generated parser's concrete source-AST shape is checked against the typed
nodes declared by the root grammar. Its adapter is then checked against the
public `grammar.Grammar` result of the oracle. Preserve meaningful public
source forms such as explicit `Group` nodes, statement `Block` nodes, ordered
positional attribute arguments including null slots, named arguments, and the
unified binary/unary operator representation. Do not normalize these
distinctions away to make the parsers appear equal.

## Alternatives Considered

- **Keep only the hand-written parser.** This avoids bootstrap and duplicate
  implementations, but leaves the grammar description and implementation as
  separate sources of truth.
- **Remove the hand-written parser entirely.** This leaves one parser in the
  repository, but removes the independent comparison path that the
  differential tests need.
- **Keep the hand-written parser as the default until the generated one is
  faster.** This keeps today's speed, but keeps two frontends whose behavior
  must be kept equal, and puts the speed work in a parser that no other
  grammar benefits from.
- **Generate an untyped parser and convert its generic tree.** This may be
  easier to generate, but adds allocations and conversion logic and risks
  losing typed AST and source-layout information.
- **Select a parser per call or at runtime.** This could tailor each call, but
  adds policy complexity to a deterministic source API and makes behavior
  harder to reproduce.
- **Keep the display grammar in Markdown only.** This is easy to read, but
  cannot serve as a direct generator input or be checked against the source
  parser as an executable grammar artifact.

## Compatibility and Bootstrap

Existing `.zp`, JSON grammar AST, compiled grammar and generated parser formats
remain unchanged; the generated frontend is only a frontend change. The root
grammar and checked-in generated Go parser must be updated together. Bootstrap
and generation checks must work from a clean checkout without network access or
a prior generated artifact outside the repository.

## Acceptance Tests

- Differentially parse every repository `.zp` file, grammar fixture and
  documentation example with both parsers. For the generated path, compare
  the concrete source AST with the root grammar's declared node structure,
  then compare its adapter output with the manual parser's public
  `grammar.Grammar` result. Preserve `Group`, `Block`, ordered positional
  arguments and null slots, named arguments, and the common operator-node
  representation. Also compare source positions and comments/layout, check
  that both reject the same sources and that the errors keep the Diagnostic
  Contract; message text is not compared. A normalized AST may be compared only
  as a separate semantic check, never as a substitute for these source and
  public-AST checks.
- Cover valid, malformed and incomplete input; CR/LF/CRLF; nested comments;
  hard newlines and delimiters; contextual identifiers; type declarations;
  attributes and pragmas; action blocks; Operators Rules; Unicode literals and
  character classes; and reserved or invalid names. Verify that leading-`_`
  names parse but fail at the validation/compile stage, and that whitespace
  between `@` and an attribute name is rejected.
- Fuzz bounded source strings and grammar mutations, requiring identical
  accept/reject results and classifying the errors of every rejected source
  as the [Mutation Measurement](#mutation-measurement) does. Preserve stable
  partial results for editor recovery where supported.
- Compare formatter parse/format/parse stability and LSP diagnostics and
  source ranges of the generated frontend with those of the oracle.
- Enumerate every sequence of up to two tokens (three in the `zp_long` tier) of
  a fixed alphabet after each of twelve heads and tails (declarations, type
  bodies, Operators Rules, action blocks, attribute arguments, groups and
  index brackets), joined by a space and directly, and require the same
  acceptance and grammar. The long tier covers about 3.8 million sources, of
  which about 3.1 million are rejected by both; none of those has an error in
  the classes `extra` or `other earlier`, and the others stay in the forms
  above. The adapter reads the typed values of the checked-in generated
  parser. Run the canonical grammar as a generated Go parser on the sources of
  the mutation corpus and require the engine's trees, positions and errors.
  Changed copies of the source grammar run on the engine, whose tree the tests
  move into the typed values by type and field name; the unchanged grammar run
  this way must agree with the generated parser on the corpora. Deliberately
  wrong typed values (lost comment gaps, reversed choices, shifted spans,
  changed repeat bounds and capture names) must each make the comparison fail.
- Verify clean-checkout bootstrap, deterministic generation, root Go tests,
  all parser modules, CLI commands, site examples and generated parser checks.
- Benchmark parse-only time, bytes/op and allocations on small common
  grammars, large grammars, comment-heavy files, deeply nested forms and
  malformed/incomplete editor input, with identical inputs, a pinned Go
  version and repeated alternating samples of the hand-written baseline and
  the generated parser plus adapter. Report paired ranges and compare only
  equivalent public AST and diagnostic work. Measure full
  source-to-compiled-parser cost separately.

### Mutation Measurement

The mutation tests take windows of whole declarations (up to about 400 code
points) from the repository `.zp` files, the parity-test corpora, a set of
valid grammars that use `rule`, `type` and `var` as names, and 100 valid
grammars that a generator writes from the forms of those names (fields, rule
references, variables and attributes named like a keyword, in groups, blocks
and brackets). They apply one or two edits (delete a character or a range of
up to 12, insert or replace by a token of the syntax, swap two characters,
truncate) and write a quarter of the windows with CRLF line breaks. For each
source, both parsers must agree on acceptance and on the grammar of an
accepted source. On a source that both reject, the first error is classified
as `same` or by one of the forms in the Diagnostic Contract, and the errors of
the two parsers are matched by declaration. A class that is not `same` has a
cap, in parts per 10,000 of the rejected sources. Controls break the
comparison on purpose and must be detected. The default tier runs 3 seeds of
3,000 sources; the `zp_long` tier runs 8 seeds of 100,000.

The caps are drift alarms, not bounds that the design guarantees. Each is the
mean share that a class took in 12 seeds of 100,000 sources (the 8 of the long
tier and 4 more, 9 to 12) plus three standard deviations of the share in one
seed (binomial, about 75,000 rejected sources), rounded up, so it describes
what this generator produces on this grammar and corpus. A change that moves
a class past its cap is looked at, and the cap is not raised to make it pass.
The long tier is not an independent check, because its seeds are among those
that set the caps. Ten seeds that did not (13 to 22, 1,000,000 sources) were
run once as a check. The first caps were the largest share of any of the 12
seeds; seed 13 passed one of them (`near` at 524.44 against 524), which showed
that the largest of 12 underestimates the spread, so the caps became the mean
plus three standard deviations. Seeds 13 and 14 were used to choose that
formula, so 15 to 22 are the check proper. With the caps every class of the ten
seeds is within its cap (the largest are `near` at 524.64 against 534,
`shadowed` at 36.92 against 39 and `missed: other` at 19.22 against 22).

Over the 12 seeds, 908,697 of the 1,200,000 sources are rejected by both
parsers (the others are accepted by both, with the same grammar). Over the
rejected sources, with the largest distance in lines between the two first
errors in any of the 12 seeds:

| First error | Sources | Share | Cap per 10,000 | Lines apart |
| --- | ---: | ---: | ---: | ---: |
| same offset | 848,569 | 93.38% | | |
| near | 46,308 | 5.10% | 534 | 1 |
| end | 8,423 | 0.927% | 104 | 12 |
| shadowed | 2,954 | 0.325% | 39 | 9 |
| head | 976 | 0.107% | 15 | 5 |
| unclosed | 645 | 0.071% | 11 | 12 |
| dropped | 643 | 0.071% | 10 | 10 |
| hint | 142 | 0.016% | 3 | 12 |
| late | 12 | 0.001% | 1 | 2 |
| other later | 10 | 0.001% | 1 | not counted |
| other earlier | 15 | 0.002% | 1 | 3 |

The corpus has many short sources with action blocks, whose errors the
reference reports at a token and the generated parser at the token next to it
(`near`), so `near` is a larger share than it is for a corpus of grammars
alone. `other later` and `other earlier` are the errors that
no form names; the tests list their sources with `ZP_HARM_EXAMPLES`. The
later ones are a rule written without `=` before a `{` block, which the
grammar reads as a rule header and its body, so that the recovered mistakes
inside the block are dropped when the missing `=` makes the rule fail, and a
mistake inside an Operators Rule body; the generated parser reports where the
construct fails, more than 64 code points later. The earlier ones are an
attribute inside an unclosed group, up to 3 lines before the reference's
error. The lines apart are given for the forms that have a distance in lines;
`other later` is bounded in code points by the classification, not in lines.
`unclosed` requires that a brace or parenthesis is still open at the generated
parser's error; a later error with nothing open in the same declaration is
`dropped`.
A mistake that is recovered inside a block is dropped likewise when a later
construct of the same declaration fails: in `rule x = operators {...}
#ignorevarcase` the reference reports the mistake in the block, and the
generated parser reports the unknown pragma, at the first token after the
declaration as the reference does for the pragma alone, so that the error is
on the declaration after it, which is valid.

The errors of the two parsers are matched by declaration. The generated parser
reports 90 errors in a declaration where the reference reports none (`extra`;
cap 3 per 10,000). Most are a real mistake that the reference does not
report after an earlier one: the window ends in the middle of an attribute
(`@x(`) after a block that failed, or a mistyped keyword in a later
declaration. An error on a declaration that no edit touched and where the
reference reports none is `extra` also at the start of the declaration after
a broken one, unless it is the only error that the generated parser reports
for the broken one (its report of what the broken one lacks at its end).

The reference reports an error in a declaration where the generated parser
reports none for 1,563 errors: 20 of a `hint` (made once the first mistake is
fixed, see above) and 1,543 of another kind (`missed: other`), 0.17% of the
rejected sources and 20 in 10,000 at most for a seed (cap 22). A sample of the 517
missed errors in 4 seeds (403 sources) shows where they are:

| Where the missed error is | Errors | Share |
| --- | ---: | ---: |
| on a line that starts with `rule`, `type` or `var`, inside a bracket | 386 | 75% |
| on another line inside a bracket | 100 | 19% |
| at the top level | 31 | 6% |

They are the independent keyword lines and statements that the reference
reads as declarations after a mistake in a bracket that the generated parser
skips whole or in which the line has no shape (`rule := 1`, `type / var`,
`c d )`, an argument `var = "x"` of an attribute whose closing bracket is
reached). The generated parser does not resume at them by design, and is
right not to flag the valid statements among them. Resuming at every line
that starts with a keyword would report them, and would flag the valid
statements of a body that is named like a keyword, which is the more harmful
error. The 31 at the top level are of three kinds: hints (checks that the
reference makes while it reads tokens), an argument named like a keyword of
an attribute that follows a mistake, and a mistyped keyword after an
attribute list.

The full list of error offsets is equal for 70.2% of the rejected sources, and
the second error is at the same offset for 33.2% of the 319,806 sources in
which the reference reports two. These shares are lower than the share of the
first error because a source with two edits often has two mistakes inside one
declaration, where a recovery that resumes at a later place cannot match the
reference. The default tier measures its classes within the caps with the
allowance for its size.

Cost. On valid grammars the recovery does not run, but the lists end at a
place with the shape of a declaration, which the parser looks ahead for. On
the grammars of the benchmark (`go test -bench CanonicalFrontend`, parsing
and building the grammar, 8 alternating rounds of 40 iterations against the
tree at the previous revision of the recovery), the median time changes by
+0.5% for `golang.zp` (57.31 and 57.58 ms) and by +0.2% for `python.zp`, and
the bytes allocated per iteration by +6.1% and +1.5%. A run of 7 iterations
had shown +6.6% in time and +16% and +14% in bytes; that was a measure at too
small a count, because the memo chunks a parse takes are kept for the next
parse, so the bytes of an iteration depend on how many iterations run. With a
memory profile of 300 iterations of `golang.zp`, the bytes allocated in the
adapter's parse grow by 132 MB of 10,698 MB (+1.2%), of which 105 MB is in
`MemoTable.Alloc`, 14 MB in the first call of a memoized rule and 3 MB in
`MemoPut`; every other function allocates the same. The measured grammar has 264
rules instead of 252 (it has 261 now, without three rules that no start rule reached). The growth is in the memo entries, which more memoized rules
and the added lookaheads make; which of the two causes it is was not
measured.

### Frontend Baseline

`BenchmarkFrontend` of `internal/syntax` parses each source with each frontend,
and [`bench/sourceparser.sh`](../../../bench/sourceparser.sh) runs it in 20
rounds of 500 ms per source and frontend, rotating the order of the frontends
from round to round. `manual` is the hand-written lexer and parser returning the
public grammar AST, `manual-aa` the same function again, whose difference from
`manual` is the noise of the run. The generated columns of the table below are
`ParseValue` of the checked-in parser generated from the source grammar, without
the adapter to the grammar AST, so they do less than the full frontend does and
are a lower bound of its cost. The `generated` and `generated-exp` frontends of
the benchmark add the adapter, and `generated-exp` also collects what each
syntax error expects. Compilation and code generation are
outside the timed region. The sources are a one-line grammar
(`rule main = [a-z]+ #atomic`), 64 commented rules, the source grammar, the
grammars of `testdata/grammars` (those of the ready-made parsers) and a copy of
the Go grammar in which every tenth rule line ends with a stray parenthesis, so
that both parsers recover.

The medians below were measured on Go 1.27.1, darwin/arm64, Apple M3 Max,
`GOMAXPROCS=16`, at source revision `f3d46fc`; every interval is within ±2% and the
difference between `manual` and `manual-aa` is −0.04% in the geometric mean of
time. The generated parser converts the node tree it builds into the typed
values, because actions of the source grammar read fields of struct values that
have no plain form; see [Typed Values](../../guide/generated-typed-values.md).

| Source | Bytes | Manual time | Manual B/op | Manual allocs | Generated time | Generated B/op | Generated allocs |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| small | 26 | 1.56 µs | 3.3 KiB | 19 | 17.6 µs | 48.2 KiB | 35 |
| comments | 5,198 | 99.0 µs | 180 KiB | 2,004 | 1.24 ms | 1,981 KiB | 3,090 |
| source grammar | 54,731 | 2.26 ms | 6.05 MiB | 17,420 | 21.8 ms | 15.3 MiB | 26,200 |
| cel | 11,556 | 416 µs | 953 KiB | 3,685 | 3.51 ms | 3,484 KiB | 5,700 |
| csv | 1,720 | 39.9 µs | 97 KiB | 334 | 304 µs | 463 KiB | 685 |
| cue | 35,513 | 1.35 ms | 3.51 MiB | 11,320 | 11.8 ms | 9.57 MiB | 19,610 |
| duckdb | 192,980 | 8.16 ms | 27.4 MiB | 77,710 | 81.1 ms | 94.0 MiB | 130,080 |
| golang | 56,479 | 2.52 ms | 7.72 MiB | 20,060 | 20.6 ms | 16.4 MiB | 32,980 |
| json | 1,564 | 76.2 µs | 188 KiB | 704 | 695 µs | 546 KiB | 1,085 |
| python | 72,104 | 3.28 ms | 9.99 MiB | 25,350 | 27.5 ms | 33.1 MiB | 49,960 |
| typescript | 127,682 | 4.69 ms | 13.4 MiB | 36,140 | 46.7 ms | 49.0 MiB | 67,420 |
| xml | 14,928 | 494 µs | 1.28 MiB | 4,282 | 4.38 ms | 3.88 MiB | 6,644 |
| yaml | 34,840 | 913 µs | 2.04 MiB | 7,593 | 8.02 ms | 7.24 MiB | 11,265 |
| malformed Go | 56,541 | 2.57 ms | 7.70 MiB | 19,940 | 20.9 ms | 16.2 MiB | 31,670 |
| geometric mean | | 548 µs | 1.40 MiB | 4,996 | 5.03 ms | 5.26 MiB | 8,354 |

The generated parser takes 9.2 times the time, 3.8 times the bytes and 1.7
times the allocations of the hand-written one in the geometric mean, before the
adapter adds its own work. The bytes of a parse that reuses the memory of
earlier parses depend on the number of iterations, so its bytes vary more
between rounds (up to ±9%) than its time. The speed recovery of the generated
frontend is measured the same way, with the adapter in the generated column.

## Limitations and Open Questions

The generated frontend is slower than the hand-written baseline; the speed is
recovered in the generator and the optimizer. The layout replay of the adapter
uses the layout primitives of the hand-written parser, so that parser stays in
the repository with the oracle. Messages of the generated frontend are those of the
grammar's labels and expectations, which name fewer specific mistakes than
the messages of the hand-written parser.
