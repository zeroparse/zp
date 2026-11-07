package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zeroparse.dev/zp/internal/bench/inputs"
	"zeroparse.dev/zp/internal/bench/probe"
)

// tsWorkload is a workload measured on the generated TypeScript parser: the grammar and the input
// that the Go benchmarks of the same name use. The recovery workload is left out because a
// recovering parse reports an error, which the measuring script treats as a failed run.
type tsWorkload struct {
	name, grammar string
	input         func() string
}

var tsWorkloads = []tsWorkload{
	{"JSON", "testdata/grammars/json/json.zp", func() string { return inputs.JSONInput(256 << 10) }},
	{"CSV", "testdata/grammars/csv/csv.zp", func() string { return inputs.CSVInput(5000) }},
	{"XML", "testdata/grammars/xml/xml.zp", func() string { return inputs.XMLInput(256 << 10) }},
	{"Arith_OperatorsRule", "examples/calculator/calc.zp", func() string { return inputs.ArithInput(20000) }},
	{"Arith_LeftRec", "examples/calculator/calc_lr.zp", func() string { return inputs.ArithInput(20000) }},
	{"Minilang", "examples/minilang/minilang.zp", func() string { return inputs.MinilangInput(300, 0) }},
	{"Outline", "examples/outline/outline.zp", func() string { return inputs.OutlineInput(5000) }},
}

// tsModes are the entry points of a generated TypeScript parser that are measured, by the name
// they have in the results.
var tsModes = []struct{ name, entry string }{
	{"Parse", "parse"},
	{"ParseAST", "parseAST"},
	{"Recognize", "recognize"},
}

// captureTS generates a TypeScript parser for every workload, measures each entry point in count
// rounds of fresh Node processes (bench/ts-parse.mjs) and returns the results with a header.
func captureTS(root, commit, work, node string, count int) ([]byte, error) {
	zpBin, err := buildZP(root, work)
	if err != nil {
		return nil, err
	}
	version, err := execOut(root, nil, node, "--version")
	if err != nil {
		return nil, err
	}
	v8, err := execOut(root, nil, node, "-p", "process.versions.v8")
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	writeHeader(&b, commit)
	fmt.Fprintf(&b, "# node: %s\n# v8: %s\n# count: %d\n", strings.TrimSpace(version), strings.TrimSpace(v8), count)
	inputs := filepath.Join(work, "ts-inputs")
	if err := os.MkdirAll(inputs, 0o755); err != nil {
		return nil, err
	}
	for _, w := range tsWorkloads {
		if err := os.WriteFile(filepath.Join(inputs, w.name+".txt"), []byte(w.input()), 0o644); err != nil {
			return nil, err
		}
		args := []string{"gen", "-lang", "ts", "-types", "-recognize", "-g", filepath.Join(root, w.grammar), "-o", filepath.Join(work, "ts", w.name)}
		if _, err := execOut(root, nil, zpBin, args...); err != nil {
			return nil, err
		}
	}
	// Alternate the runs of the rounds so that drift of the machine affects every workload alike.
	script := filepath.Join(root, "bench", "ts-parse.mjs")
	for round := 0; round < count; round++ {
		for _, w := range tsWorkloads {
			for _, m := range tsModes {
				line, err := execOut(root, nil, node, "--experimental-strip-types", script,
					"--module", filepath.Join(work, "ts", w.name, "parser.ts"),
					"--input", filepath.Join(inputs, w.name+".txt"),
					"--name", m.name+"/"+w.name, "--entry", m.entry)
				if err != nil {
					return nil, fmt.Errorf("%s %s: %w", m.name, w.name, err)
				}
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "BenchmarkTSParse/") {
					return nil, fmt.Errorf("%s %s: unexpected output %q", m.name, w.name, line)
				}
				fmt.Fprintf(os.Stderr, "%s\n", line)
				fmt.Fprintf(&b, "BenchmarkTS/%s\n", strings.TrimPrefix(line, "BenchmarkTSParse/"))
			}
		}
	}
	return b.Bytes(), nil
}

// sizeParsers are the grammars whose generated size is recorded, with the options of
// testdata/grammars/generate.txt (the options of the ready-made parsers).
var sizeParsers = probe.Grammars

// genOptions returns the options of testdata/grammars/generate.txt for grammar name.
func genOptions(root, name string) ([]string, error) { return probe.GenOptions(root, name) }

// parserGenArgs returns the options with which testdata/grammars/generate.txt generates parser name,
// adapted to TypeScript.
func parserGenArgs(root, name string) ([]string, error) { return probe.TSGenArgs(root, name) }

// captureSizes records, for each parser of sizeParsers, the size of the generated Go parser.go,
// of a binary that calls its ParseAST (built with -trimpath and stripped of symbols and DWARF), of
// the generated TypeScript parser.ts and of that file compressed with gzip -9. The embedded
// TypeScript runtime is common to all parsers and is recorded once as parser "runtime".
func captureSizes(root, commit, work string) ([]byte, error) {
	zpBin, err := buildZP(root, work)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	writeHeader(&b, commit)
	fmt.Fprintln(&b, "# go binary: a main that calls ParseAST; go build -trimpath -ldflags=\"-s -w\"")
	for _, name := range sizeParsers {
		genDir, err := generateGo(root, zpBin, work, name)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(filepath.Join(genDir, "parser.go"))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "size %s go-source %d\n", name, st.Size())
		bin, err := buildBinary(root, work, name, genDir)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "size %s go-binary %d\n", name, bin)
		args, err := parserGenArgs(root, name)
		if err != nil {
			return nil, err
		}
		out := filepath.Join(work, "size-ts", name)
		if _, err := execOut(root, nil, zpBin, append(append([]string{"gen", "-lang", "ts"}, args...), "-o", out)...); err != nil {
			return nil, err
		}
		raw, gz, err := sizeOfFile(filepath.Join(out, "parser.ts"))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "size %s ts-raw %d\nsize %s ts-gzip %d\n", name, raw, name, gz)
		if name == sizeParsers[0] {
			raw, gz, err := sizeOfFile(filepath.Join(out, "runtime.ts"))
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&b, "size runtime ts-raw %d\nsize runtime ts-gzip %d\n", raw, gz)
		}
	}
	return b.Bytes(), nil
}

// sizeOfFile returns the size of a file and of its gzip -9 compression.
func sizeOfFile(path string) (raw, gz int, err error) { return probe.SizeOfFile(path) }

// generateGo generates the Go parser of grammar name with the options of testdata/grammars/generate.txt
// into the work directory and returns the directory.
func generateGo(root, zpBin, work, name string) (string, error) {
	dir := filepath.Join(work, "gen-"+name)
	return dir, probe.GenerateGo(root, zpBin, dir, name)
}

// buildBinary builds a program that calls ParseAST of the parser generated into genDir, in a module of
// its own in the work directory, and returns the size of the binary.
func buildBinary(root, work, name, genDir string) (int, error) {
	_, size, err := probe.BuildProgram(root, filepath.Join(work, "bin-"+name), genDir, probe.SizeMain)
	return size, err
}

// buildZP builds the zp command of the repository into the work directory.
func buildZP(root, work string) (string, error) { return probe.BuildZP(root, work) }

// execOut runs a command in dir, with extra environment variables, and returns its standard output.
func execOut(dir string, env []string, name string, args ...string) (string, error) {
	return probe.ExecOut(dir, env, name, args...)
}
