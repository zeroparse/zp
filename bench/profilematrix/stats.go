package main

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"text/tabwriter"
)

// metric is a quantity that a cell records. Smaller is better for all of them.
type metric struct {
	name string
	unit string
}

// metrics lists what a run records, in the order of its output.
var metrics = []metric{
	{"ns_op", "ns per pass over the inputs, the minimum of the rounds"},
	{"b_op", "bytes allocated per pass"},
	{"allocs_op", "allocations per pass"},
	{"retained", "live heap held by the results of a pass, after three collections, in bytes"},
	{"rss", "peak resident set size of the measuring process, in bytes"},
	{"gc_share", "share of the CPU time that went to garbage collection"},
	{"src_bytes", "size of the generated parser.go"},
	{"bin_bytes", "size of a stripped program that calls ParseAST"},
	{"compile_ms", "wall time of compiling and linking that program with the parser package not cached"},
	{"ts_raw", "size of the generated parser.ts (with -ts)"},
	{"ts_gzip", "size of parser.ts compressed with gzip -9 (with -ts)"},
}

// cells holds the recorded values by grammar, variant and metric.
type cells map[string]map[string]map[string]float64

func (c cells) set(grammar, variant, metric string, v float64) {
	if c[grammar] == nil {
		c[grammar] = map[string]map[string]float64{}
	}
	if c[grammar][variant] == nil {
		c[grammar][variant] = map[string]float64{}
	}
	c[grammar][variant][metric] = v
}

func (c cells) get(grammar, variant, metric string) (float64, bool) {
	v, ok := c[grammar][variant][metric]
	return v, ok
}

// ratio returns variant/base for one grammar and metric. It reports false if either value is missing
// or the base is zero.
func (c cells) ratio(grammar, variant, base, metric string) (float64, bool) {
	v, ok1 := c.get(grammar, variant, metric)
	b, ok2 := c.get(grammar, base, metric)
	if !ok1 || !ok2 || b == 0 {
		return 0, false
	}
	return v / b, true
}

// geomean returns the geometric mean of the ratios. It is 1 for no ratios.
func geomean(r []float64) float64 {
	if len(r) == 0 {
		return 1
	}
	var sum float64
	for _, x := range r {
		sum += math.Log(x)
	}
	return math.Exp(sum / float64(len(r)))
}

// summary is the effect of a variant on one metric across the grammars, as ratios to the baseline.
type summary struct {
	Ratios  map[string]float64 // by grammar
	Geomean float64
	Worst   float64 // the largest ratio
	Better  int     // grammars where the ratio is below 1-threshold
}

// summarize computes the summary of variant against base for a metric over the grammars that have both
// values. threshold is the relative improvement that counts as better, for example 0.03: a ratio counts
// only if it is strictly below 1-threshold, so with 0 a grammar that did not change is not better.
func (c cells) summarize(grammars []string, variant, base, metric string, threshold float64) summary {
	s := summary{Ratios: map[string]float64{}}
	var all []float64
	for _, g := range grammars {
		r, ok := c.ratio(g, variant, base, metric)
		if !ok {
			continue
		}
		s.Ratios[g] = r
		all = append(all, r)
		s.Worst = max(s.Worst, r)
		if r < 1-threshold {
			s.Better++
		}
	}
	s.Geomean = geomean(all)
	return s
}

// printCells writes one line per recorded value.
func (c cells) printCells(w io.Writer, grammars, variants []string) {
	for _, g := range grammars {
		for _, v := range variants {
			for _, m := range metrics {
				if x, ok := c.get(g, v, m.name); ok {
					fmt.Fprintf(w, "cell %s %s %s %s\n", g, v, m.name, formatValue(x))
				}
			}
		}
	}
}

// printRatios writes, for every metric with values, a table of the ratios of each variant to the
// baseline: one row per variant, one column per grammar, then the geometric mean and the worst ratio.
func (c cells) printRatios(w io.Writer, grammars, variants []string) {
	base := variants[0]
	for _, m := range metrics {
		var rows [][]string
		for _, v := range variants[1:] {
			s := c.summarize(grammars, v, base, m.name, 0)
			if len(s.Ratios) == 0 {
				continue
			}
			row := []string{v}
			for _, g := range grammars {
				if r, ok := s.Ratios[g]; ok {
					row = append(row, fmt.Sprintf("%.3f", r))
				} else {
					row = append(row, "-")
				}
			}
			rows = append(rows, append(row, fmt.Sprintf("%.3f", s.Geomean), fmt.Sprintf("%.3f", s.Worst)))
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(w, "\nratio to %s: %s (%s)\n", base, m.name, m.unit)
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, strings.Join(append(append([]string{"variant"}, grammars...), "geomean", "worst"), "\t"))
		for _, r := range rows {
			fmt.Fprintln(tw, strings.Join(r, "\t"))
		}
		tw.Flush()
	}
}

// formatValue prints integers without a fraction and other values with six significant digits.
func formatValue(x float64) string {
	if x == math.Trunc(x) && math.Abs(x) < 1e15 {
		return fmt.Sprintf("%d", int64(x))
	}
	return fmt.Sprintf("%.6g", x)
}

// rotate returns the variants starting at index round modulo their number, so that each variant is
// measured in every position over the rounds.
func rotate(variants []string, round int) []string {
	if len(variants) == 0 {
		return nil
	}
	k := round % len(variants)
	return append(slices.Clone(variants[k:]), variants[:k]...)
}
