package cliflag

import (
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/pkg/runtime/input"
)

func TestParseBackend(t *testing.T) {
	for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		if got, err := ParseBackend(b.String()); err != nil || got != b {
			t.Errorf("ParseBackend(%q) = %v, %v", b.String(), got, err)
		}
	}
	for _, name := range []string{"", "nope", "Closure", engine.DefaultBackend.String()} {
		if _, err := ParseBackend(name); err == nil {
			t.Errorf("ParseBackend(%q) accepted an unknown backend", name)
		}
	}
}

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
