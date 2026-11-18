package input

import (
	"testing"
)

func TestPositionUnitString(t *testing.T) {
	if CodePoints.String() != "codepoints" || Bytes.String() != "bytes" {
		t.Errorf("String() = %q, %q", CodePoints, Bytes)
	}
	if CodePoints != 0 {
		t.Error("CodePoints is not the zero value")
	}
}
