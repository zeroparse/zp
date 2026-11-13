package engine

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/value"
)

func TestAliasNodeIdentity(t *testing.T) {
	t.Parallel()
	for _, c := range codegentest.AliasCases {
		t.Run(c.Name, func(t *testing.T) {
			for _, options := range []compile.Options{{}, {DisableMemo: true}} {
				prog := mustCompile(t, c.Src, options)
				for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
					n, err := prog.ParseWith("main", "éé", ParseOptions{Unit: unit})
					if err != nil {
						t.Fatal(err)
					}
					wantType := "Num"
					switch c.Name {
					case "alias terminals in structs":
						wantType = "Box"
					case "alias constructors", "alias singleton unions":
						wantType = "P"
					}
					if n.Type() != wantType {
						t.Errorf("%s: got type %s, want %s", unit, n.Type(), wantType)
					}
					terminal := n
					if v, ok := value.FieldOf(n, "Value"); ok {
						terminal, _ = v.(*core.Node)
					}
					if terminal == nil || terminal.Type() != "Num" || value.D(terminal).Text != "éé" {
						t.Errorf("%s: expected a Num terminal, got %s", unit, codegentest.ResultJSON(terminal, nil))
					}
				}
				for _, input := range c.Inputs {
					checkBackends(t, prog, "main", input)
				}
				for _, mode := range valueModes {
					loaded := saveLoad(t, prog, "main", mode)
					for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
						if backend == Closure {
							_, err := loaded.ParseWith("main", c.Inputs[0], servedBy(mode, ParseOptions{Backend: backend}))
							wantClosureRefused(t, err)
							continue
						}
						for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
							for _, input := range c.Inputs {
								want, wantErr := prog.ParseWith("main", input, ParseOptions{Unit: unit})
								got, gotErr := loaded.ParseWith("main", input, servedBy(mode, ParseOptions{Backend: backend, Unit: unit}))
								if codegentest.ResultJSON(got, gotErr) != codegentest.ResultJSON(want, wantErr) {
									t.Errorf("loaded %s/%s on %q: got %s, want %s", backend, unit, input, codegentest.ResultJSON(got, gotErr), codegentest.ResultJSON(want, wantErr))
								}
							}
						}
					}
				}
			}
		})
	}
}
