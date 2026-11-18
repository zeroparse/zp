// Package generate generates Go and TypeScript parser source from grammar syntax trees with the
// libraries of generated parsers. Package compiler generates through it from grammar sources;
// programs and tests of this module that hold a syntax tree call it directly.
package generate

import (
	"io/fs"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/backend/codegen/golang/libsrc"
	goprint "zeroparse.dev/zp/internal/backend/codegen/golang/print"
	tslib "zeroparse.dev/zp/internal/backend/codegen/typescript/lib"
	tsprint "zeroparse.dev/zp/internal/backend/codegen/typescript/print"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Go generates the files of a Go parser for g with o, and the embedded copy of the Go library
// (pkg/runtime/golib) with LibEmbed and LibOnly. g is not read with LibOnly and may be nil.
func Go(g *ast.Grammar, o common.GenOptions) ([]common.GeneratedFile, error) {
	o.GoLib = goLib
	return goprint.Generate(g, Compile, o)
}

// TypeScript generates the files of a TypeScript parser for g with o, and lib.ts with LibEmbed
// and LibOnly. g is not read with LibOnly and may be nil.
func TypeScript(g *ast.Grammar, o common.GenOptions) ([]common.GeneratedFile, error) {
	o.TSLib = tsLib
	return tsprint.Generate(g, Compile, o)
}

// Compile is the common.Compiler that the generators compile grammars with: it checks g to its
// static program, which is all a generator reads, and checks the recognizer of the program (as
// the engine's Program.Recognizer makes it) on first use. No backend of the engine is built.
func Compile(g *ast.Grammar, opts compile.Options) (*compile.Program, func() (*compile.Program, error), error) {
	prog, err := check(g, opts, nil)
	if err != nil {
		return nil, nil, err
	}
	recognizer := func() (*compile.Program, error) {
		source := prog.Grammar
		if prog.Source != nil {
			source = prog.Source
		}
		o := prog.Opts
		o.Recognition = compile.Recognize
		return check(source, o, prog.Flags())
	}
	return prog, recognizer, nil
}

// check checks g with opts and, if not nil, the rule flags flags, naming the hidden rules of
// inline actions in errors as written.
func check(g *ast.Grammar, opts compile.Options, flags []compile.RuleFlags) (*compile.Program, error) {
	names := desugar.InlineNames{}
	prog, err := frontend.Check(g, opts, flags, names, nil)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return prog, nil
}

// goLib and tsLib are the libraries of generated parsers: Go generation copies the Go library
// into packages generated with LibEmbed and reads it to qualify the names generated code uses
// from it; TypeScript generation assembles lib.ts from the TypeScript library. The copies live in
// the back end rather than in the runtime so that only programs that generate code carry them.
var (
	goLib fs.FS = libsrc.FS
	tsLib fs.FS = tslib.FS
)
