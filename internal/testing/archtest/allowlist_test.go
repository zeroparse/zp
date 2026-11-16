package archtest

// allowlist holds the import edges that break a rule today, in the form of violation.key. The
// test fails when a build has an edge that the list does not hold, and when the list holds an
// edge that no build has: an edge leaves the list in the change that removes it, and none is
// added.
var allowlist = []string{
	// The runtime entry points, the parser state, the Document and the VM still compile grammars
	// and lower action expressions at run time: they read the checked program, the Core IR and
	// the grammar AST. They go when the runtime gets a run-time program of its own and the
	// compiler entry moves to the driver.
	"rule 1: internal/engine -> grammar",
	"rule 1: internal/engine -> internal/cancel",
	"rule 1: internal/engine -> internal/compile",
	"rule 1: internal/engine -> internal/complexity",
	"rule 1: internal/engine -> internal/desugar",
	"rule 1: internal/engine -> internal/diag",
	"rule 1: internal/engine -> internal/engine/closure",
	"rule 1: internal/engine -> internal/ir/core",
	"rule 1: internal/engine/document -> internal/compile",
	"rule 1: internal/engine/state -> grammar",
	"rule 1: internal/engine/state -> internal/compile",
	"rule 1: internal/engine/state -> internal/ir/core",
	"rule 1: internal/engine/state -> internal/ir/exec",
	"rule 1: internal/engine/state -> internal/ir/exec/lower",
	"rule 1: internal/engine/state -> internal/ir/exec/ops",
	"rule 1: internal/engine/vm -> grammar",
	"rule 1: internal/engine/vm -> internal/compile",
	"rule 1: internal/engine/vm -> internal/ir/core",
	"rule 1: internal/engine/vm -> internal/ir/core/build",

	// The front end computes the rule facts of the Core IR, which belong to the middle end.
	"rule 2: internal/frontend -> internal/ir/core/facts",
	// The generators read the optimization registry and profiles, which go to internal/pass
	// (registrations beside each pass) and the driver (profiles, pipeline driving).
	"rule 2: internal/codegen -> internal/ir/opt",
	"rule 2: internal/codegen/go -> internal/ir/opt",
	"rule 2: internal/codegen/ts -> internal/ir/opt",
	// The Go and TypeScript generators drive the lowering to Flow and print Exec; the driver
	// takes over the lowering and the printers read Exec only.
	"rule 2: internal/codegen/go -> dialects exec, flow",
	"rule 2: internal/codegen/ts -> dialects exec, flow",
	// The flag names and fixture suites use the backend choice, Parser and parse options of
	// engine, which go to pkg/runtime (the backend choice goes away).
	"rule 2: internal/cliflag -> engine",
	"rule 2: internal/fixture -> engine",
}
