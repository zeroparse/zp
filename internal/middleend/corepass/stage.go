package corepass

import (
	"fmt"
	"io"

	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/pass"
)

// IR describes the Core IR module of a table to the manager of the pipeline of a compilation: it is
// dumped as the listing of the module, labeled with its table. It has no Verify: the Core verifier
// reads the attributes analysis, which the Pipeline computes before it verifies (VerifyRequires).
var IR = pass.IR[*core.Module]{Stage: Stage, Print: Print, Copy: (*core.Module).Clone}

// Print writes the dump of the Core IR module m, labeled l, after the step after.
func Print(w io.Writer, l pass.Label, after string, m *core.Module) {
	fmt.Fprintf(w, "-- Core IR of the %s table after %s --\n%s\n", l.Table, after, core.Print(m, true))
}
