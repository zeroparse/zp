// Command profilematrix measures what generation options do to the generated parsers of the ten
// language grammars of testdata/grammars. It generates every grammar once per variant, builds each
// parser into two programs (one for the size of a stripped binary, one to measure), runs the
// measuring program in fresh processes in rotated order, and prints what it recorded:
//
//	go run ./bench/profilematrix \
//	    -variant speed= -variant size='-optimize size' -variant nosharing='-no-opt method-sharing' \
//	    -control -rounds 8
//
// A variant is NAME=ARGS, where ARGS are options of zp gen added to the options of
// testdata/grammars/generate.txt (they may be empty). The first variant is the baseline; the ratios are
// relative to it. -control adds a copy of the baseline under the name "control": its ratios show the
// noise of the machine, and the deterministic metrics must equal the baseline's.
//
// The program is measured by parsing every input of the grammar that parses without an error: the
// .txt files of testdata/grammars/NAME, and for json, csv and xml also a generated input of about
// 256 KiB. One pass parses all of them. Time is the minimum over the rounds, which makes it robust
// against other load but not immune; run it under the benchmark lock of the machine.
//
//	ns_op, b_op, allocs_op   testing.Benchmark of one pass
//	retained                 live heap held by the results of a pass, after three collections
//	rss                      peak resident set size of the measuring process
//	gc_share                 share of the CPU time that went to garbage collection
//	src_bytes, bin_bytes     size of parser.go and of a stripped program calling ParseAST
//	compile_ms               wall time of building that program with the parser not cached
//	ts_raw, ts_gzip          size of parser.ts (with -ts), generated with the same ARGS
//
// The output has one "cell GRAMMAR VARIANT METRIC VALUE" line per value, then a table of ratios to the
// baseline per metric.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type variantFlags []string

func (v *variantFlags) String() string     { return strings.Join(*v, " ") }
func (v *variantFlags) Set(s string) error { *v = append(*v, s); return nil }

func main() {
	var variants variantFlags
	flag.Var(&variants, "variant", "NAME=ARGS: a variant, with options of zp gen in ARGS (repeatable; the first is the baseline)")
	root := flag.String("root", ".", "the root of the repository")
	grammars := flag.String("grammars", "", "grammars to measure, comma-separated (default: the ten language grammars)")
	rounds := flag.Int("rounds", 8, "rounds of measurements; the minimum time is kept")
	benchtime := flag.String("benchtime", "300ms", "the -benchtime of each measurement")
	control := flag.Bool("control", false, "add a copy of the baseline named control")
	ts := flag.Bool("ts", false, "also record the sizes of the generated TypeScript parsers")
	work := flag.String("work", "", "directory for generated files (default: a temporary one, removed afterwards)")
	flag.Parse()
	cfg := config{
		Root: *root, Rounds: *rounds, Benchtime: *benchtime, Control: *control, TS: *ts, Work: *work,
	}
	if *grammars != "" {
		cfg.Grammars = strings.Split(*grammars, ",")
	}
	for _, v := range variants {
		name, args, ok := strings.Cut(v, "=")
		if !ok || name == "" {
			fmt.Fprintf(os.Stderr, "profilematrix: -variant %q: want NAME=ARGS\n", v)
			os.Exit(2)
		}
		cfg.Variants = append(cfg.Variants, variant{Name: name, Args: strings.Fields(args)})
	}
	if err := run(cfg, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "profilematrix:", err)
		os.Exit(1)
	}
}
