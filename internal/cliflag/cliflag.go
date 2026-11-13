// Package cliflag parses the names that command-line flags and serialized files use for the
// values of the zp options: backends and position units. The zp command and the editfuzz sequence
// format share it.
package cliflag

import (
	"fmt"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
)

// ParseBackend returns the backend named closure, bytecode or bytecode-iterative.
func ParseBackend(name string) (engine.Backend, error) {
	for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		if b.String() == name {
			return b, nil
		}
	}
	return 0, fmt.Errorf("unknown backend %q", name)
}

// ParseUnit returns the unit named codepoints or bytes.
func ParseUnit(name string) (core.PositionUnit, error) {
	for _, u := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		if u.String() == name {
			return u, nil
		}
	}
	return 0, fmt.Errorf("unknown unit %q", name)
}
