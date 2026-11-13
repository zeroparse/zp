// Package probe generates the parsers of testdata/grammars with the options of the repository and
// measures what they leave behind: the size of the generated source, of a stripped program that
// calls ParseValue, and of the generated TypeScript. bench/report and bench/profilematrix share it.
package probe

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Grammars lists the language grammars of testdata/grammars that the size and profile reports cover.
var Grammars = []string{"json", "csv", "xml", "yaml", "cel", "cue", "golang", "python", "typescript", "duckdb"}

// ExecOut runs a command in dir, with extra environment variables, and returns its standard output.
func ExecOut(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(slices.Clone(args), " "), err, stderr.String())
	}
	return string(out), nil
}

// BuildZP builds the zp command of the repository at root into the work directory.
func BuildZP(root, work string) (string, error) {
	out := filepath.Join(work, "zp")
	if _, err := ExecOut(root, []string{"GOFLAGS=" + os.Getenv("GOFLAGS")}, "go", "build", "-trimpath", "-o", out, "./cmd/zp"); err != nil {
		return "", err
	}
	return out, nil
}

// GenOptions returns the options of testdata/grammars/generate.txt for grammar name.
func GenOptions(root, name string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "testdata", "grammars", "generate.txt"))
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && f[0] == name {
			return f[1:], nil
		}
	}
	return nil, fmt.Errorf("testdata/grammars/generate.txt has no line for %s", name)
}

// GenTSArgs returns the options with which testdata/grammars/generate.txt generates parser name,
// adapted to TypeScript: the Go-only options (-module, -runtime import, -nodoc, -typed-only) are dropped.
func GenTSArgs(root, name string) ([]string, error) {
	f, err := GenOptions(root, name)
	if err != nil {
		return nil, err
	}
	out := []string{"-g", filepath.Join(root, "testdata", "grammars", name, name+".zp")}
	for i := 0; i < len(f); i++ {
		switch f[i] {
		case "-module", "-runtime", "-o":
			i++
		case "-nodoc", "-typed-only":
		default:
			out = append(out, f[i])
		}
	}
	return out, nil
}

// GenerateGo generates the Go parser of grammar name into dir with the options of
// testdata/grammars/generate.txt followed by extra (for example -optimize or -no-opt).
func GenerateGo(root, zpBin, dir, name string, extra ...string) error {
	f, err := GenOptions(root, name)
	if err != nil {
		return err
	}
	args := append([]string{"gen", "-g", filepath.Join(root, "testdata", "grammars", name, name+".zp"), "-module", "sizeprobe/gen"}, f...)
	args = append(args, extra...)
	_, err = ExecOut(root, nil, zpBin, append(args, "-o", dir)...)
	return err
}

// GenerateTS generates the TypeScript parser of grammar name into dir, with extra options after those
// of GenTSArgs.
func GenerateTS(root, zpBin, dir, name string, extra ...string) error {
	args, err := GenTSArgs(root, name)
	if err != nil {
		return err
	}
	_, err = ExecOut(root, nil, zpBin, append(append(append([]string{"gen", "-lang", "ts"}, args...), extra...), "-o", dir)...)
	return err
}

// SizeOfFile returns the size of a file and of its gzip -9 compression.
func SizeOfFile(path string) (raw, gz int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := w.Write(data); err != nil {
		return 0, 0, err
	}
	if err := w.Close(); err != nil {
		return 0, 0, err
	}
	return len(data), buf.Len(), nil
}

// SizeMain is a program that calls ParseValue of the parser in the package sizeprobe/gen.
const SizeMain = `package main

import (
	"os"

	p "sizeprobe/gen"
)

func main() {
	if _, err := p.ParseValue(os.Args[1]); err != nil {
		os.Exit(1)
	}
}
`

// BuildProgram builds a program with the source main, which imports the parser generated into genDir as
// sizeprobe/gen, in a module of its own in dir, and returns the path of the stripped binary (built with
// -trimpath) and its size. The module replaces the repository's by root.
//
// buildFlags are added to the go build command.
func BuildProgram(root, dir, genDir, main string, buildFlags ...string) (path string, size int, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	mod := fmt.Sprintf(`module sizeprobe

go 1.27.1

require %[1]s v0.0.0

replace %[1]s => %[2]s
`, "zeroparse.dev/zp", root)
	if err := os.CopyFS(filepath.Join(dir, "gen"), os.DirFS(genDir)); err != nil {
		return "", 0, err
	}
	for file, content := range map[string]string{"go.mod": mod, "main.go": main} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			return "", 0, err
		}
	}
	out := filepath.Join(dir, "probe")
	args := append([]string{"build", "-trimpath", "-ldflags=-s -w"}, buildFlags...)
	if _, err := ExecOut(dir, []string{"GOWORK=off", "GOFLAGS=-mod=mod"}, "go", append(args, "-o", out, ".")...); err != nil {
		return "", 0, err
	}
	st, err := os.Stat(out)
	if err != nil {
		return "", 0, err
	}
	return out, int(st.Size()), nil
}
