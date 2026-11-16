// Package archtest checks the import graph of the module against the dependency rules of the
// release package layout:
//
//   - Rule 1: a package of the runtime side (pkg/runtime and its subpackages, pkg/bytecode and
//     internal/runtime/**) depends only on runtime-side packages of the module and on no
//     third-party package.
//   - Rule 1a: the library of generated Go parsers (pkg/runtime/golib) imports only pkg/runtime,
//     its input, value, value/impl, instrument and schema packages, and a generated Go parser,
//     in either library mode, links neither the VM (pkg/runtime/vm) nor internal/runtime/**.
//   - Rule 2: the layers of the compiler side. internal/pass imports nothing of the module;
//     internal/ir/** imports only internal/ir/**, internal/pass, internal/cancel and pkg/grammar,
//     and within it internal/ir/code imports no dialect, code/flow imports only code and ir/core,
//     code/action and code/exec import only code, and the packages under code/exec also code/exec
//     and code/action; the front end imports no middle end, back end, analysis or LSP package,
//     the middle end no back end, analysis or LSP package, the back end no analysis or LSP
//     package; only cmd/** and internal/testing/** import internal/lsp/**; and only
//     internal/testing/** imports internal/testing/** outside tests.
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
