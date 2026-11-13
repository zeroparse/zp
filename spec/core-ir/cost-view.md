# Cost View

The complexity classifier decides the
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost) on the
[cost view](../runtime/parsing.md#cost-view) of a grammar. This chapter defines the cost view as a Core IR module:
what it is built from, what it shares with that module, and how its nodes lead back to the grammar as written. The
rewrites themselves, their conditions and the value conditions are those of
[Cost View](../runtime/parsing.md#cost-view); the proof that they keep the cost within a constant factor is
[Cost View](../proofs/cost-view.md) in the proofs.

## Source Module

The view is derived from the **source module**: the module of the grammar that
[Cost View](../runtime/parsing.md#cost-view) starts from (inline value actions replaced by calls of their hidden
rules, built-in attributes replaced by the constructs that they denote), with each rule once. The source module is
built without types: it has the control structure, the calls, the captures and the variable reads of the rules, every
rule builds its value, no repetition projects, and it has no [policy](modules.md#policy) and no
[value-free twins](modules.md#value-free-twins). Its [rule facts](attributes.md#rule-facts) are those of the typed
module of the same rules, because the facts read only that structure.

Every node of the source module has an **origin**: the expression of the grammar that it is built from. A node that
an attribute builds around an expression (a label, a recovery, a stream, `#missing`) has the attributed expression
as its origin, and the body of an Operators Rule has the Operators Rule.

## The View Is an Analysis

The view is a module derived by an analysis, not a pass: it is never run, never lowered, and the source module is
not changed by building it. The rewrites MAY change values, which a pass MUST NOT do
([Invariants](invariants.md)); the value conditions of
[Cost View](../runtime/parsing.md#cost-view) say where a rewrite that changes values is allowed.

The view has the rules of the source module, in the same order and with the same identities. A node that no
rewrite changes is the node of the source module, shared, with its origin. A node that a rewrite builds is new and
has no origin; for it the view records the source text of the expressions that it stands for, so that a message
about the view quotes the grammar as it is written.

What the runtime decides by the grammar as written is read from the source module and its facts, not from the
view: which rules are memoized, the left-recursion cycles and their leaders, the variables that key the memo, the
run sites and the scopes of captures.

## Options

The cost view applies every rewrite of [Cost View](../runtime/parsing.md#cost-view). Two further rewrites exist for
analyses of matching only, which compare what a parse matches and where it ends and not what it costs:

| Rewrite | Replaces | Why the cost view leaves it out |
|:--|:--|:--|
| N7, for labels | a label (`@error`, `@diagnostic`) by its body | it changes the expectations and the run sites |
| N13 | a call by the body of the called rule | it changes which evaluations the memo answers |

Of N7, the view applies only the part that removes labels. The law also removes `#atomic`, discards and captures that
no predicate reads; the view keeps them.

## Writing the View as a Grammar

The view can be written back as grammar rules: a shared node is its origin, a node that the view builds by giving
a node of the source module new parts (a repetition, an option, a lookahead, a capture, a discard, an atomic
expression, a label, a recovery or a stream with a rewritten body) is the origin of that node with the new parts,
and any other node that the view builds is written in the grammar syntax of its kind (a sequence, a choice, a
repetition, an option, a lookahead, a class, a cut, `_` or `_|_`). Written so, the view shows the rewrites of
[Cost View](../runtime/parsing.md#cost-view) applied to the grammar's rule bodies, in the text that a message quotes.
