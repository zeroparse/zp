// Package action declares the Action dialect of Code IR: the operations that evaluate the action
// expressions of a grammar (actions, predicates and the values of bindings) on the values of terms.
// The dialect is legal at the Flow and the Exec stage.
package action

import "zeroparse.dev/zp/internal/ir/code"

// Dialect is the name of the dialect, the Dialect of its operations.
const Dialect = "action"

// Names of the state of the abstract machine that operations of the dialect refer to. The module that
// declares the dialect (Declare) declares them: the Exec dialect names them exec.Host, exec.Pos and
// exec.Datum.
const (
	// Host is the component of the host.
	Host = "host"
	// Pos is the register of the position.
	Pos = "pos"
	// Datum is the handle type of a value that a variable holds or a term produces.
	Datum = "Datum"
)

// Declare adds the components, handle types and operations of the dialect to m, which declares the
// state of the abstract machine they refer to (Host, Pos and Datum).
func Declare(m *code.Module) {
	n := len(m.Ops)
	addValue(m)
	addTermCode(m)
	for _, o := range m.Ops[n:] {
		o.Dialect = Dialect
	}
}
