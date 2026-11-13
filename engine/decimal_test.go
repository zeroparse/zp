package engine_test

import (
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
)

func TestInvalidSourceDecimalReferences(t *testing.T) {
	for _, digits := range []string{"١", "１２", "1١", strconv.FormatUint(uint64(^uint(0)>>1)+1, 10)} {
		source := `def main="a" -> $` + digits
		if g, err := engine.ParseGrammar(source); g != nil || err == nil || !strings.Contains(err.Error(), "invalid positional reference") {
			t.Fatalf("ParseGrammar accepted %q: %v, %v", source, g, err)
		}
		if p, err := compileTestSource(source, "main"); p != nil || err == nil || !strings.Contains(err.Error(), "invalid positional reference") {
			t.Fatalf("CompileSource accepted %q: %v, %v", source, p, err)
		}
	}
}

func TestDecimalReferenceValues(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{`rule main = v1:"a" v2:"b" => list($v1, $v2)`, `["a" "b"]`},
		{`rule main = v1:"a" v2:"b" => list($v1, $v2)`, `["a" "b"]`},
		{`rule main = v1:"a" "b" => $v1`, `"a"`},
		{`rule main = "a" v2:"b" => $v2`, `"b"`},
	} {
		p, err := compileTestSource(tc.source, "main")
		if err != nil {
			t.Fatal(err)
		}
		for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
			for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
				n, err := p.Parse("ab", engine.WithBackend(backend), engine.WithUnit(unit))
				if err != nil || n.String() != tc.want {
					t.Fatalf("%s backend=%v unit=%v: %v, %v", tc.source, backend, unit, n, err)
				}
			}
		}
	}
}
