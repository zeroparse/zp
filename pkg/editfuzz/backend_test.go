package editfuzz

import "testing"

func TestParseBackend(t *testing.T) {
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		if got, err := ParseBackend(string(b)); err != nil || got != b {
			t.Errorf("ParseBackend(%q) = %v, %v", b, got, err)
		}
	}
	for _, name := range []string{"", "nope", "Closure", "default"} {
		if _, err := ParseBackend(name); err == nil {
			t.Errorf("ParseBackend(%q) accepted an unknown backend", name)
		}
	}
}
