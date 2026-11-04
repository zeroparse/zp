package complexitytest

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"zeroparse.dev/zp"
)

// TestDepthBoundProbe measures the cost of depth-proportional families at a fixed nesting depth
// with a growing input, and at a fixed input tail with a growing depth. Run with
// ZP_DEPTH_PROBE=<output file>.
func TestDepthBoundProbe(t *testing.T) {
	out := os.Getenv("ZP_DEPTH_PROBE")
	if out == "" {
		t.Skip("set ZP_DEPTH_PROBE")
	}
	file := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	type probe struct {
		name, src string
		control   bool
		build     func(d, m int) string
	}
	ws := func(m int) string { return rep(" ", m) }
	probes := []probe{
		{"calc nested negations", file("../../examples/calculator/calc.zp"), false, func(d, m int) string { return rep("-", d) + "1" + ws(m) }},
		{"calc nested powers", file("../../examples/calculator/calc.zp"), false, func(d, m int) string { return rep("2^", d) + "1" + ws(m) }},
		{"calc k blocks (m=k*8 blocks)", file("../../examples/calculator/calc.zp"), false, func(d, m int) string {
			return rep(rep("-", d)+"1"+ws(8)+"+", m/8) + "1"
		}},
		{"minilang nested negations", file("../../examples/minilang/minilang.zp"), false, func(d, m int) string { return rep("-", d) + "a" + ws(m) + ";" }},
		{"cue nested negations", file("../../testdata/grammars/cue/cue.zp"), false, func(d, m int) string { return "a: " + rep("-", d) + "1" + ws(m) + "\n" }},
		{"cel nested negations", file("../../testdata/grammars/cel/cel.zp"), false, func(d, m int) string { return rep("-", d) + "a" + ws(m) }},
		{"control e = ( e? (ws ))?", "rule main = e ws $$\nrule e = \"(\" e? (ws \")\")?\nrule ws = [ ]*\n", true, func(d, m int) string { return rep("(", d) + ws(m) }},
		{"control right operands ws ^", "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    level { infix right ws \"^\" }\n}\nrule ws = [ ]*\n", true, func(d, m int) string { return rep("a^", d) + "a" + ws(m) }},
		{"control skip nested prefix", "rule main = x ws $$\nrule x = #operators {\n    skip ws\n    operand \"a\"\n    level { prefix \"(\"\n        postfix \")\" }\n}\nrule ws = [ ]*\n", true, func(d, m int) string { return rep("(", d) + "a" + ws(m) }},
		{"control operator parts that nest (stays)", "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    level { postfix \"(\" x? }\n    level { postfix ws \"!\" }\n}\nrule ws = [ ]*\n", true, func(d, m int) string { return rep("a(", d) + "a" + ws(m) }},
	}
	var b strings.Builder
	slope := func(xs []uint64) string {
		var s []string
		for i := 1; i < len(xs); i++ {
			s = append(s, fmt.Sprintf("%.2f", math.Log2(float64(xs[i])/float64(xs[i-1]))))
		}
		return strings.Join(s, " ")
	}
	for _, pr := range probes {
		var p *zp.Parser
		var err error
		if pr.control {
			p, err = Compile(pr.src)
		} else {
			p, err = zp.CompileSource(pr.src, "")
		}
		if err != nil {
			t.Fatalf("%s: %v", pr.name, err)
		}
		for _, be := range Backends {
			var byM, byD []uint64
			for _, m := range []int{500, 1000, 2000, 4000} {
				c, err := Steps(p, pr.build(8, m), be, 2_000_000_000)
				if err != nil {
					t.Fatalf("%s %s m=%d: %v", pr.name, BackendName(be), m, err)
				}
				byM = append(byM, c.Steps)
			}
			for _, d := range []int{8, 16, 32, 64} {
				c, err := Steps(p, pr.build(d, 2000), be, 2_000_000_000)
				if err != nil {
					t.Fatalf("%s %s d=%d: %v", pr.name, BackendName(be), d, err)
				}
				byD = append(byD, c.Steps)
			}
			fmt.Fprintf(&b, "%-42s %-9s D=8,m=500..4000: %v slopes %s | m=2000,D=8..64: %v slopes %s\n", pr.name, BackendName(be), byM, slope(byM), byD, slope(byD))
		}
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
