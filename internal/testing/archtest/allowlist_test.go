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
	// The printer tests of the Exec dialect build specialized functions with a middle-end pass.
	"rule 2: internal/ir/exec/printtest -> internal/ir/exec/pass",
}
