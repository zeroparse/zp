package cliflag

import (
	"testing"

	"zeroparse.dev/zp"
)

func TestParseBackend(t *testing.T) {
	for _, b := range []zp.Backend{zp.Closure, zp.Bytecode, zp.BytecodeIterative} {
		if got, err := ParseBackend(b.String()); err != nil || got != b {
			t.Errorf("ParseBackend(%q) = %v, %v", b.String(), got, err)
		}
	}
	for _, name := range []string{"", "nope", "Closure", zp.DefaultBackend.String()} {
		if _, err := ParseBackend(name); err == nil {
			t.Errorf("ParseBackend(%q) accepted an unknown backend", name)
		}
	}
}

func TestParseUnit(t *testing.T) {
	for _, u := range []zp.PositionUnit{zp.CodePoints, zp.Bytes} {
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
