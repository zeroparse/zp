package archtest

// allowlist holds the import edges that break a rule today, in the form of violation.key. The
// test fails when a build has an edge that the list does not hold, and when the list holds an
// edge that no build has: an edge leaves the list in the change that removes it. An entry is
// added only for a dependence that already exists, when a move renames or splits it, and it names
// the stage that removes it.
var allowlist = []string{
	// The runtime entry points, the parser state, the Document and the VM still compile grammars
	// and lower action expressions at run time: they read the checked program, the Core IR and
	// the grammar AST. They go when the runtime gets a run-time program of its own and the
	// compiler entry moves to the driver (the runtime separation stage, S5).
	"rule 1: internal/engine -> grammar",
	"rule 1: internal/engine -> internal/cancel",
	"rule 1: internal/engine -> internal/compile",
	"rule 1: internal/engine -> internal/analysis/complexity",
	"rule 1: internal/engine -> internal/frontend/desugar",
	// The runtime checks a grammar through the front end's entry (frontend.Check), which takes
	// the rule facts and the site numbers from the Core IR; S5 moves it to the driver with the rest.
	"rule 1: internal/engine -> internal/frontend",
	"rule 1: internal/engine -> internal/diagnostic",
	"rule 1: internal/engine -> internal/engine/closure",
	"rule 1: internal/engine -> internal/ir/core",
	"rule 1: internal/engine/document -> internal/compile",
	"rule 1: internal/engine/state -> grammar",
	"rule 1: internal/engine/state -> internal/compile",
	"rule 1: internal/engine/state -> internal/ir/core",
	"rule 1: internal/engine/state -> internal/ir/code",
	"rule 1: internal/engine/state -> internal/ir/exec/lower",
	"rule 1: internal/engine/state -> internal/ir/code/exec",
	"rule 1: internal/engine/vm -> grammar",
	"rule 1: internal/engine/vm -> internal/compile",
	"rule 1: internal/engine/vm -> internal/ir/core",
	"rule 1: internal/engine/vm -> internal/frontend/elaborate",
	"rule 1: internal/engine/vm -> internal/middleend/corepass",

	// The front end computes the rule facts of the Core IR, which now live in corepass, and
	// reports the structural errors from them. The edge goes when the facts are corepass analyses
	// that the driver asks for, and the front end no longer computes them.
	"rule 2: internal/frontend -> internal/middleend/corepass",
	// The generators resolve the optimization profile and run the pass pipelines through the
	// driver, against the arrow from the driver to the back end. The back-end regroup (S4, #279)
	// moves them under internal/backend, and the runtime separation (S5, #280) moves the compile
	// entry into the driver, which then resolves the settings and calls the back ends; the edges
	// go with it.
	"rule 2: internal/codegen -> internal/driver",
	"rule 2: internal/codegen/go -> internal/driver",
	"rule 2: internal/codegen/ts -> internal/driver",
	// The Go and TypeScript generators drive the lowering to Flow and print Exec. With Flow
	// and Action as dialects and the control decisions in Flow passes (S7a, S7b), the driver runs
	// flowlower and the printers print the Exec stage only.
	"rule 2: internal/codegen/go -> dialects exec, flow",
	"rule 2: internal/codegen/ts -> dialects exec, flow",
	// The flag names use the backend choice and the parse options of engine. The runtime and
	// compiler API stage (S6b) removes engine: the options move to pkg/runtime, and the backend
	// choice goes away (the constructor chooses the backend).
	"rule 2: internal/cliflag -> engine",
}
