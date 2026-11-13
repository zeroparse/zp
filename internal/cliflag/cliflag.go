// Package cliflag parses the names that command-line flags and serialized files use for the
// values of the zp options: backends and position units. The zp command and the editfuzz sequence
// format share it.
package cliflag

import (
	"fmt"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/runtime/parse"
)

// ParseBackend returns the backend named closure, bytecode or bytecode-iterative.
func ParseBackend(name string) (zp.Backend, error) {
	for _, b := range []zp.Backend{zp.Closure, zp.Bytecode, zp.BytecodeIterative} {
		if b.String() == name {
			return b, nil
		}
	}
	return 0, fmt.Errorf("unknown backend %q", name)
}

// ParseUnit returns the unit named codepoints or bytes.
func ParseUnit(name string) (parse.PositionUnit, error) {
	for _, u := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
		if u.String() == name {
			return u, nil
		}
	}
	return 0, fmt.Errorf("unknown unit %q", name)
}
