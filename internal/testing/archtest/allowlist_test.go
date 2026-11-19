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
	// The Go and TypeScript generators drive the lowering to Flow and print Exec. With Flow
	// and Action as dialects and the control decisions in Flow passes (S7a, S7b), the driver runs
	// flowlower and the printers print the Exec stage only.
	"rule 2: internal/backend/codegen/golang/print -> dialects action, exec, flow",
	"rule 2: internal/backend/codegen/typescript/print -> dialects action, exec, flow",
}
