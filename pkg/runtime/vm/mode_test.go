package vm

import (
	"testing"

	"zeroparse.dev/zp/internal/runtime/engine"
)

// TestCompiledModesMatchTheEngine checks that each mode is the engine's mode of the same name,
// which the conversions between them rely on.
func TestCompiledModesMatchTheEngine(t *testing.T) {
	for _, c := range []struct {
		vm     CompiledMode
		engine engine.CompiledMode
	}{{CompiledParse, engine.CompiledParse}, {CompiledRecognize, engine.CompiledRecognize}, {CompiledDocument, engine.CompiledDocument}} {
		if engine.CompiledMode(c.vm) != c.engine || c.vm.String() != c.engine.String() {
			t.Errorf("vm mode %d (%s) is not the engine mode %d (%s)", c.vm, c.vm, c.engine, c.engine)
		}
	}
}
