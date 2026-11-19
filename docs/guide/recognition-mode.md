# Recognition Mode

Recognition decides whether the input matches the grammar without building a tree. It returns the same success, failure and syntax errors as a parse, recovered errors included. Predicates, and the actions they read, run; the run-time errors of other actions and typed conversions are not checked.

A parser's `Recognize` method recognizes the input; it applies to every [backend](backends.md). This guide shows what it returns, what it still evaluates and what it cannot be combined with.

The example uses `pairs.zp` and a parser `p` compiled as in [Running Parsers](running-parsers.md):

```zp
type Pair = { Key node[string]; Value node[string] }

@start
rule main = ws ps:pair+ $$ => $ps
rule pair Pair = k:key "=" v:value ";"? ws => Pair{Key: $k, Value: $v}
rule key = [a-zあ-ん]+ #atomic
rule value = [0-9]+ #atomic
rule ws = [ \t\n]*
```

With it, `Recognize` returns only an error, the same syntax errors as a full parse, and builds no node. It is faster and allocates less: the
benchmarks measure recognition at 1.3–1.6× faster than a full parse on the closure backend.

```go
err := p.Recognize("abc=12")
fmt.Println(err) // <nil>
err = p.Recognize("abc=", runtime.WithExpectations())
fmt.Println(err) // 1:5: syntax error: expected [0-9]
```

What it does not do:

- **Actions are conditionally evaluated.** Actions whose values are not needed to evaluate predicates are skipped. Actions needed by predicates still run and can report run-time errors. With the grammar
  `rule main Ratio =  a:[0-9]+ #atomic "/" b:[0-9]+ #atomic $$ => Ratio{Num: len($a) / (len($b) - len($b))} `, the input `12/34`
  fails a full parse with `action in main: division by zero` but is accepted by recognition (`zp parse -recognize` prints
  `ok`). Use recognition for syntax checks, and a full parse where action errors matter.
- Captures that [predicates](../../spec/language/predicates.md) read are still built, since a predicate can depend on them.
- A compiled grammar recognizes when it is saved in recognize mode (`zp compile -mode recognize`, or
  `compiler.Marshal(src, vm.CompiledRecognize)`), on the bytecode backends. A file in another mode returns an error that names
  the mode it needs, and traced or profiled recognition needs the grammar source. The [compiled grammars
  guide](compiled-grammars.md#modes) explains the modes.
- It has no stream or Document form: `Recognize` reads one complete input, and `ParseStream` and `NewDocument` always build
  values.

Typical uses are checking the syntax of uploads or configuration before doing real work, linting loops, and pre-filtering large
inputs. On the command line: `zp parse -g grammar.zp -recognize < input`, which prints `ok` or the syntax error (and
exits with status 1).

## Related Topics

- [Choosing a Backend](backends.md): the engines that recognition runs on.
- [Compiled Grammars](compiled-grammars.md): which saved grammars can recognize.
- [Generated Go API](generated-go-api.md#recognition--recognize): recognition in generated parsers.
- [Running Parsers](running-parsers.md): the other options of a parse.
