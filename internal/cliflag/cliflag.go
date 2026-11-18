// Package cliflag parses the names that command-line flags and serialized files use for the
// values of the zp options: position units. The zp command and the editfuzz sequence format
// share it.
package cliflag

import (
	"fmt"

	"zeroparse.dev/zp/pkg/runtime/input"
)

// ParseUnit returns the unit named codepoints or bytes.
func ParseUnit(name string) (input.PositionUnit, error) {
	for _, u := range []input.PositionUnit{input.CodePoints, input.Bytes} {
		if u.String() == name {
			return u, nil
		}
	}
	return 0, fmt.Errorf("unknown unit %q", name)
}
