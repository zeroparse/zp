package compile

import (
	"os"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/desugar"
)

var normalizationBenchmarkGrammar *grammar.Grammar

var normalizationBenchmarkProgram *Program

func BenchmarkAttributeNormalization(b *testing.B) {
	sources := map[string]string{
		"plain":      `rule main = "a"? (&"b" / !"x") "b"+ $$`,
		"attributes": `rule main = ("a" @error("as")) "b" @error(message="bs") $$`,
		"pragmas":    `rule main = ("a" #repeat(1, 3) #atomic ";") #recover(skip=[^;]+ ";") $$`,
		"operators":  `rule main = #operators { skip " "*; operand [0-9]+; operand "(" main ")"; level { infix left "+" / "-" }; level product { infix left "*" / "/" } }`,
	}
	source, err := os.ReadFile("../../zp.zp")
	if err != nil {
		b.Fatal(err)
	}
	sources["source-grammar"] = string(source)
	for name, path := range map[string]string{"json": "../../testdata/grammars/json/json.zp", "minilang": "../../examples/minilang/minilang.zp"} {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		sources[name] = string(data)
	}
	for name, source := range sources {
		g, err := parseTest(source)
		if err != nil {
			b.Fatal(err)
		}
		for _, eager := range []bool{true, false} {
			mode := "copy-on-change"
			if eager {
				mode = "eager"
			}
			b.Run(name+"/"+mode+"/normalize", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var err error
					normalizationBenchmarkGrammar, err = desugar.NormalizeAttributesWithCopies(g, eager)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(name+"/"+mode+"/compile", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var err error
					normalizationBenchmarkProgram, err = compileStatic(g, Options{EagerAttributeCopies: eager}, nil)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(name+"/"+mode+"/source-prepare", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					ast, err := parseTest(source)
					if err != nil {
						b.Fatal(err)
					}
					normalizationBenchmarkProgram, err = compileStatic(ast, Options{EagerAttributeCopies: eager}, nil)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
