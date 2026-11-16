package archtest

// allowlist holds the import edges that break a rule today, in the form of violation.key. The
// test fails when a build has an edge that the list does not hold, and when the list holds an
// edge that no build has: an edge leaves the list in the change that removes it, and none is
// added.
var allowlist = []string{
	// The runtime entry points, the parser state, the Document and the VM still compile grammars
	// and lower action expressions at run time: they read the checked program, the Core IR and
	// the grammar AST. They go when the runtime gets a run-time program of its own and the
	// compiler entry moves to the driver (the runtime separation stage, S5).
	"rule 1: internal/engine -> grammar",
	"rule 1: internal/engine -> internal/cancel",
	"rule 1: internal/engine -> internal/compile",
	"rule 1: internal/engine -> internal/complexity",
	"rule 1: internal/engine -> internal/desugar",
	"rule 1: internal/engine -> internal/diagnostic",
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

	// The front end computes the rule facts of the Core IR. The IR regroup (S3) moves the facts
	// to corepass analyses, and the driver asks for them.
	"rule 2: internal/frontend -> internal/ir/core/facts",
	// The generators read the optimization registry and profiles, which the IR regroup (S3)
	// splits into registrations beside each pass (internal/pass) and the driver.
	"rule 2: internal/codegen -> internal/ir/opt",
	"rule 2: internal/codegen/go -> internal/ir/opt",
	"rule 2: internal/codegen/ts -> internal/ir/opt",
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
