package cliflag

import (
	"testing"

	"zeroparse.dev/zp/pkg/runtime/input"
)

func TestParseUnit(t *testing.T) {
	for _, u := range []input.PositionUnit{input.CodePoints, input.Bytes} {
		if got, err := ParseUnit(u.String()); err != nil || got != u {
			t.Errorf("ParseUnit(%q) = %v, %v", u.String(), got, err)
		}
	}
	for _, name := range []string{"", "nope", "Bytes"} {
		if _, err := ParseUnit(name); err == nil {
			t.Errorf("ParseUnit(%q) accepted an unknown unit", name)
		}
	}
}
