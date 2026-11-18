package archtest

// allowlist holds the import edges that break a rule today, in the form of violation.key. The
// test fails when a build has an edge that the list does not hold, and when the list holds an
// edge that no build has: an edge leaves the list in the change that removes it. An entry is
// added only for a dependence that already exists, when a move renames or splits it, and it names
// the stage that removes it.
var allowlist = []string{
	// The front end computes the rule facts of the Core IR, which now live in corepass, and
	// reports the structural errors from them. The edge goes when the facts are corepass analyses
	// that the driver asks for, and the front end no longer computes them.
	"rule 2: internal/frontend -> internal/middleend/corepass",
	// The generators resolve the optimization profile and run the pass pipelines through the
	// driver, against the arrow from the driver to the back end. The back-end regroup (S4, #279)
	// moves them under internal/backend, and the runtime separation (S5, #280) moves the compile
	// entry into the driver, which then resolves the settings and calls the back ends; the edges
	// go with it.
	"rule 2: internal/backend/codegen/common -> internal/driver",
	"rule 2: internal/backend/codegen/golang/print -> internal/driver",
	"rule 2: internal/backend/codegen/typescript/print -> internal/driver",
	// The Go and TypeScript generators drive the lowering to Flow and print Exec. With Flow
	// and Action as dialects and the control decisions in Flow passes (S7a, S7b), the driver runs
	// flowlower and the printers print the Exec stage only.
	"rule 2: internal/backend/codegen/golang/print -> dialects exec, flow",
	"rule 2: internal/backend/codegen/typescript/print -> dialects exec, flow",
	// The language server compiles the grammars it serves and reads their checked programs. It
	// did so through engine, which held the compile entry on the runtime side; the runtime
	// separation (S5, #280) moved the entry to the driver, which the layer graph does not place
	// below the LSP. The edge goes when the LSP compiles through the compiler API of the runtime
	// and compiler API stage (S6b), or the layer graph places the LSP above the driver.
	"rule 2: internal/lsp/zpls -> internal/driver/entry",
	// The flag names use the backend choice and the parse options of engine. The runtime and
	// compiler API stage (S6b) removes engine: the options move to pkg/runtime, and the backend
	// choice goes away (the constructor chooses the backend).
	"rule 2: internal/cliflag -> engine",
}
