package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/complexity"
	"zeroparse.dev/zp/lint"
)

// lintOutput is the JSON document that zp lint -f json prints.
type lintOutput struct {
	Grammar  string        `json:"grammar"`
	Start    string        `json:"start"`
	Findings []lintFinding `json:"findings"`
	// Classification is present with -classify.
	Classification *lintClassification `json:"classification,omitempty"`
}

// lintClassification is the classification of the cost of the grammar from the start rule.
type lintClassification struct {
	// Class is linear, linear-env, unknown, polynomial or exponential.
	Class string `json:"class"`
	// Degree is the degree of a polynomial class.
	Degree int `json:"degree,omitempty"`
	// WitnessedDegree is the largest degree of the polynomial witnesses of an unknown class, a
	// lower estimate rather than a bound.
	WitnessedDegree int `json:"witnessedDegree,omitempty"`
	// Combinations bounds K for a linear class whose variables with finite values have more than
	// one combination of values.
	Combinations int `json:"combinations,omitempty"`
	// Witnesses are the polynomial and exponential causes, which an unknown class can have too.
	Witnesses []lintCause `json:"witnesses"`
	// Reasons are the conditions for linear cost that the classifier cannot establish.
	Reasons []lintCause `json:"reasons"`
}

type lintCause struct {
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	Rule    string `json:"rule,omitempty"`
	Message string `json:"message"`
}

// classNames are the names of the classes in the output of zp lint -classify.
var classNames = map[complexity.ComplexityClass]string{
	complexity.ClassLinear:    "linear",
	complexity.ClassLinearEnv: "linear-env",
	complexity.ClassUnknown:   "unknown",
	complexity.ClassPoly:      "polynomial",
	complexity.ClassExp:       "exponential",
}

// classifyShown is the number of witnesses and of reasons that the text output of -classify shows.
const classifyShown = 3

func lintClassificationOf(c *complexity.Complexity) *lintClassification {
	causes := func(cs []complexity.ComplexityCause) []lintCause {
		out := []lintCause{}
		for _, x := range cs {
			out = append(out, lintCause{x.Pos.Line, x.Pos.Col, x.Rule, x.Message})
		}
		return out
	}
	out := &lintClassification{Class: classNames[c.Class], Degree: c.Degree, WitnessedDegree: c.WitnessedDegree, Witnesses: causes(c.Causes), Reasons: causes(c.Reasons)}
	if c.Class == complexity.ClassLinear && c.Combinations > 1 {
		out.Combinations = c.Combinations
	}
	return out
}

// printClassification prints the class of the cost from start, and the first witnesses and
// reasons.
func printClassification(w io.Writer, path, start string, c *lintClassification) {
	class := c.Class
	if c.Degree > 0 {
		class = fmt.Sprintf("%s (n^%d)", class, c.Degree)
	}
	if c.WitnessedDegree > 0 {
		class = fmt.Sprintf("%s (witnessed n^%d)", class, c.WitnessedDegree)
	}
	if c.Combinations > 0 {
		class = fmt.Sprintf("%s (K <= %d)", class, c.Combinations)
	}
	fmt.Fprintf(w, "%s: the cost from start rule %s is %s\n", path, start, class)
	for _, list := range []struct {
		what   string
		causes []lintCause
	}{{"witness", c.Witnesses}, {"reason", c.Reasons}} {
		for i, x := range list.causes {
			if i == classifyShown {
				fmt.Fprintf(w, "\t... and %d more\n", len(list.causes)-i)
				break
			}
			where := path
			if x.Line > 0 {
				where = fmt.Sprintf("%s:%d:%d", where, x.Line, x.Col)
			}
			fmt.Fprintf(w, "\t%s: %s: %s\n", list.what, where, x.Message)
		}
	}
}

type lintFinding struct {
	Line     int           `json:"line,omitempty"`
	Col      int           `json:"col,omitempty"`
	Severity lint.Severity `json:"severity"`
	Check    string        `json:"check"`
	Rule     string        `json:"rule,omitempty"`
	Message  string        `json:"message"`
	Fix      string        `json:"fix,omitempty"`
}

// checkList is a flag that collects check names, given comma-separated or in several flags.
type checkList []string

func (l *checkList) String() string { return strings.Join(*l, ",") }

func (l *checkList) Set(s string) error {
	for _, name := range strings.Split(s, ",") {
		if name = strings.TrimSpace(name); name != "" {
			*l = append(*l, name)
		}
	}
	return nil
}

func lintCmd(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	grammarPath := fs.String("g", "", "grammar file (.zp or .json)")
	start := fs.String("s", "", "start rule (default: the grammar's default start rule)")
	format := fs.String("f", "text", "output format: text or json")
	strict := fs.Bool("strict", false, "fail on warnings too, not only on errors")
	list := fs.Bool("list", false, "list the checks and exit")
	classify := fs.Bool("classify", false, "classify the cost of the grammar from the start rule and run the super-linear check")
	var disable checkList
	fs.Var(&disable, "disable", "comma-separated checks not to run (the flag can be repeated)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *list {
		w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		for _, c := range lint.Checks() {
			fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, c.Severity, c.Summary)
		}
		return w.Flush()
	}
	if *grammarPath == "" {
		return errors.New("-g is required")
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unknown format %q", *format)
	}
	g, err := loadGrammar(*grammarPath, "lint")
	if err != nil {
		return err
	}
	if *start == "" {
		p, err := engine.Compile(g, "")
		if err != nil {
			return fileError(*grammarPath, err)
		}
		*start = p.Start()
	}
	// The super-linear check classifies the grammar; the class and its witnesses come from that
	// classification, or from one of their own when the check is disabled.
	var classified *complexity.Complexity
	ctx := complexity.WithComplexitySink(context.Background(), func(c *complexity.Complexity) { classified = c })
	findings, err := (lint.Config{Disable: disable, Classify: *classify}).Run(ctx, g, *start)
	if err != nil {
		return fileError(*grammarPath, err)
	}
	var classification *lintClassification
	if *classify {
		if classified == nil {
			classified, err = complexity.ClassifyComplexity(context.Background(), g, *start)
			if err != nil {
				return fileError(*grammarPath, err)
			}
		}
		classification = lintClassificationOf(classified)
	}
	counts := map[lint.Severity]int{}
	out := lintOutput{Grammar: *grammarPath, Start: *start, Findings: []lintFinding{}, Classification: classification}
	for _, f := range findings {
		counts[f.Severity]++
		out.Findings = append(out.Findings, lintFinding{f.Pos.Line, f.Pos.Col, f.Severity, f.Check, f.Rule, f.Message, f.Fix})
	}
	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return err
		}
	} else {
		for _, f := range findings {
			where := *grammarPath
			if f.Pos.IsValid() {
				where = fmt.Sprintf("%s:%d:%d", where, f.Pos.Line, f.Pos.Col)
			}
			fmt.Fprintf(stdout, "%s: %s: %s [%s]\n", where, f.Severity, f.Message, f.Check)
			if f.Fix != "" {
				fmt.Fprintf(stdout, "\tfix: %s\n", f.Fix)
			}
		}
		if classification != nil {
			printClassification(stdout, *grammarPath, *start, classification)
		}
	}
	failed := counts[lint.Error]
	if *strict {
		failed += counts[lint.Warning]
	}
	if failed > 0 {
		return fmt.Errorf("lint: %s, %s, %s", plural(counts[lint.Error], "error"),
			plural(counts[lint.Warning], "warning"), plural(counts[lint.Hint], "hint"))
	}
	return nil
}

func plural(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}
