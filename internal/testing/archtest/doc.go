// Package archtest checks the import graph of the module against the dependency rules of the
// release package layout:
//
//   - Rule 1: a package of the runtime side (pkg/runtime and its subpackages, pkg/bytecode and
//     internal/runtime/**) depends only on runtime-side packages of the module and on no
//     third-party package.
//   - Rule 1a: the library of generated Go parsers (pkg/runtime/golib) imports only pkg/runtime,
//     its input, value, value/impl, instrument and schema packages, and a generated Go parser,
//     in either library mode, links neither the VM (pkg/runtime/vm) nor internal/runtime/**.
//   - Rule 2: the layers of the compiler side. Each layer imports only the layers that the
//     layer graph puts below it: cmd over the public tool API and the LSP, the public tool API
//     over the driver, the LSP and the driver over analysis, analysis over the back end, the back
//     end over the middle end, the middle end over the front end, and those over internal/ir,
//     which is over internal/pass and pkg/grammar; every layer may import the runtime side and
//     the utilities (diagnostic, cancel, cliflag, fixture), which import only pkg/grammar. Test
//     support and the packages outside the graph (benchmarks, examples, the specification's
//     tests) may import any layer except the LSP, and only test support imports test support
//     outside tests.
//     internal/pass imports nothing of the module; internal/ir/** imports only internal/ir/**,
//     internal/pass, internal/cancel and pkg/grammar, and within it internal/ir/code imports no
//     dialect, code/flow imports only code and ir/core, code/action and code/exec import only
//     code, and the packages of a dialect's subtree also each other and, under code/exec,
//     code/action. Only the conversions (flowlower, execlower, vmlower) and test support import
//     more than one dialect.
//
// The rules are stated on the paths of the release layout. The test places every package of the
// module at its path in that layout (layout_test.go), so it checks the rules while the packages
// are still being moved there; a package the table does not place fails it. The edges that
// break a rule today are listed in allowlist_test.go, which must match them exactly: a listed
// edge that disappears fails the test too, so the list only shrinks.
//
// The test reads the graph with go list for the default build and for every build tag that
// hack/test-reference-tags.sh checks, and checks the imports of non-test files only. The
// package has no non-test code.
package archtest
