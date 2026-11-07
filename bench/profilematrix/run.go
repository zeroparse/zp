package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"zeroparse.dev/zp/internal/bench/inputs"
	"zeroparse.dev/zp/internal/bench/probe"
)

type variant struct {
	Name string
	Args []string
}

type config struct {
	Root      string
	Grammars  []string // default probe.Grammars
	Variants  []variant
	Rounds    int
	Benchtime string
	Control   bool
	TS        bool
	Work      string // default: a temporary directory that is removed
}

// scaledInputs are the grammars that also get a generated input of about 256 KiB.
var scaledInputs = map[string]func() string{
	"json": func() string { return inputs.JSONInput(256 << 10) },
	"csv":  func() string { return inputs.CSVInput(5000) },
	"xml":  func() string { return inputs.XMLInput(256 << 10) },
}

// built is a generated parser with the programs built for it.
type built struct {
	grammar, variant string
	bench            string // the measuring program
}

// measurement is what the measuring program printed, and what the parent observed of the process.
type measurement struct {
	Inputs   int     `json:"inputs"`
	NS       float64 `json:"ns"`
	Bytes    int64   `json:"bytes"`
	Allocs   int64   `json:"allocs"`
	Retained int64   `json:"retained"`
	GCShare  float64 `json:"gc_share"`
	RSS      int64   `json:"-"`
}

func run(cfg config, out, log io.Writer) error {
	if len(cfg.Variants) == 0 {
		return fmt.Errorf("no -variant: give at least the baseline, for example -variant speed=")
	}
	if cfg.Rounds < 1 {
		return fmt.Errorf("-rounds must be at least 1")
	}
	grammars := cfg.Grammars
	if len(grammars) == 0 {
		grammars = probe.Grammars
	}
	variants := slices.Clone(cfg.Variants)
	if cfg.Control {
		variants = append(variants, variant{Name: "control", Args: variants[0].Args})
	}
	seen := map[string]bool{}
	for _, v := range variants {
		if seen[v.Name] {
			return fmt.Errorf("variant %q is given twice", v.Name)
		}
		seen[v.Name] = true
	}
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return err
	}
	work := cfg.Work
	if work == "" {
		if work, err = os.MkdirTemp("", "zp-profilematrix-"); err != nil {
			return err
		}
		defer os.RemoveAll(work)
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	zpBin, err := probe.BuildZP(root, work)
	if err != nil {
		return err
	}
	names := make([]string, len(variants))
	for i, v := range variants {
		names[i] = v.Name
	}
	c := cells{}
	programs := map[string]built{}
	for _, g := range grammars {
		if err := writeInputs(root, work, g); err != nil {
			return err
		}
		for _, v := range variants {
			fmt.Fprintf(log, "build %s %s\n", g, v.Name)
			b, err := build(root, zpBin, work, g, v, cfg.TS, c)
			if err != nil {
				return fmt.Errorf("%s %s: %w", g, v.Name, err)
			}
			programs[g+"\x00"+v.Name] = b
		}
	}
	best := map[string]measurement{} // by grammar and variant
	for round := 0; round < cfg.Rounds; round++ {
		for _, g := range grammars {
			files, err := filepath.Glob(filepath.Join(work, "inputs", g, "*"))
			if err != nil {
				return err
			}
			for _, name := range rotate(names, round) {
				fmt.Fprintf(log, "round %d %s %s\n", round+1, g, name)
				m, err := measure(programs[g+"\x00"+name].bench, cfg.Benchtime, files)
				if err != nil {
					return fmt.Errorf("%s %s: %w", g, name, err)
				}
				key := g + "\x00" + name
				if prev, ok := best[key]; ok {
					m = keepBest(prev, m)
				}
				best[key] = m
			}
		}
	}
	for key, m := range best {
		g, v, _ := strings.Cut(key, "\x00")
		c.set(g, v, "ns_op", m.NS)
		c.set(g, v, "b_op", float64(m.Bytes))
		c.set(g, v, "allocs_op", float64(m.Allocs))
		c.set(g, v, "retained", float64(m.Retained))
		c.set(g, v, "rss", float64(m.RSS))
		c.set(g, v, "gc_share", m.GCShare)
	}
	c.printCells(out, grammars, names)
	c.printRatios(out, grammars, names)
	return nil
}

// writeInputs writes the inputs of a grammar into work/inputs/NAME: the .txt files of its directory,
// and the generated large input for the grammars that have one.
func writeInputs(root, work, grammar string) error {
	dir := filepath.Join(work, "inputs", grammar)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	txt, err := filepath.Glob(filepath.Join(root, "testdata", "grammars", grammar, "*.txt"))
	if err != nil {
		return err
	}
	for _, f := range txt {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.Base(f))
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	if gen, ok := scaledInputs[grammar]; ok {
		dst := filepath.Join(dir, "scaled.in")
		if err := os.WriteFile(dst, []byte(gen()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// build generates a variant of a grammar, records its sizes in c and builds the measuring program.
func build(root, zpBin, work, grammar string, v variant, ts bool, c cells) (built, error) {
	genDir := filepath.Join(work, "gen", grammar, v.Name)
	if err := probe.GenerateGo(root, zpBin, genDir, grammar, v.Args...); err != nil {
		return built{}, err
	}
	raw, _, err := probe.SizeOfFile(filepath.Join(genDir, "parser.go"))
	if err != nil {
		return built{}, err
	}
	c.set(grammar, v.Name, "src_bytes", float64(raw))
	// The directory name of -D is unique to this build: the flag changes nothing in the output, but it
	// is part of the cache key of the package, so the package is compiled and not taken from the cache.
	uniq := fmt.Sprintf("-gcflags=sizeprobe/gen=-D=./%d", time.Now().UnixNano())
	start := time.Now()
	_, size, err := probe.BuildProgram(root, filepath.Join(work, "size", grammar, v.Name), genDir, probe.SizeMain, uniq)
	if err != nil {
		return built{}, err
	}
	c.set(grammar, v.Name, "compile_ms", float64(time.Since(start).Milliseconds()))
	c.set(grammar, v.Name, "bin_bytes", float64(size))
	bench, _, err := probe.BuildProgram(root, filepath.Join(work, "bench", grammar, v.Name), genDir, benchMain)
	if err != nil {
		return built{}, err
	}
	if ts {
		tsDir := filepath.Join(work, "ts", grammar, v.Name)
		if err := probe.GenerateTS(root, zpBin, tsDir, grammar, v.Args...); err != nil {
			return built{}, err
		}
		raw, gz, err := probe.SizeOfFile(filepath.Join(tsDir, "parser.ts"))
		if err != nil {
			return built{}, err
		}
		c.set(grammar, v.Name, "ts_raw", float64(raw))
		c.set(grammar, v.Name, "ts_gzip", float64(gz))
	}
	return built{grammar: grammar, variant: v.Name, bench: bench}, nil
}

// keepBest combines two measurements of the same program: the minimum of each quantity, and the
// garbage collection share of the faster run.
func keepBest(a, b measurement) measurement {
	r := a
	if b.NS < a.NS {
		r.NS, r.GCShare = b.NS, b.GCShare
	}
	r.Bytes, r.Allocs, r.Retained = min(a.Bytes, b.Bytes), min(a.Allocs, b.Allocs), min(a.Retained, b.Retained)
	r.RSS = min(a.RSS, b.RSS)
	return r
}

// measure runs the measuring program once in a fresh process.
func measure(program, benchtime string, files []string) (measurement, error) {
	cmd := exec.Command(program, append([]string{benchtime}, files...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return measurement{}, fmt.Errorf("%s: %w\n%s", program, err, stderr.String())
	}
	var m measurement
	if err := json.Unmarshal([]byte(stdout.String()), &m); err != nil {
		return measurement{}, fmt.Errorf("%s printed %q: %w", program, stdout.String(), err)
	}
	if m.NS <= 0 || math.IsNaN(m.NS) {
		return measurement{}, fmt.Errorf("%s measured %v ns per pass", program, m.NS)
	}
	m.RSS = peakRSS(cmd.ProcessState)
	return m, nil
}
