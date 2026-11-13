//go:build zp_long

package gogen_test

import (
	"regexp"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegentest"
)

// TestFlowStreamObservationMutants checks that the reports of stream parses notice a stream body that
// forgets to start the stream, drops the delivery of an element, delivers at the wrong position or delivers
// after the zero-width check instead of before.
func TestFlowStreamObservationMutants(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	codegentest.PinOrdinary(t)
	cases := streamCases(t)
	form := streamForms[2]
	d, _ := runStreamProbes(t, cases, form, nil)
	if len(d) == 0 {
		t.Fatal("no reports")
	}
	startStmt := regexp.MustCompile(`(?m)^\t+p\.StreamStart\(\)\n`)
	emitStmt := regexp.MustCompile(`(?m)^\t+p\.EmitStream\(.*\)\n`)
	reorderEmit := regexp.MustCompile(`(?m)^(\t+)(p\.EmitStream\(.*\)\n)(\t+if p\.Pos == x\d+\.Pos \{\n\t+break\n\t+\}\n)`)
	for _, m := range []struct {
		name   string
		mutate func(string) string
	}{
		{"no stream start", func(s string) string { return startStmt.ReplaceAllString(s, "") }},
		{"no delivery", func(s string) string { return emitStmt.ReplaceAllString(s, "") }},
		{"delivery after the zero-width check", func(s string) string {
			return reorderEmit.ReplaceAllString(s, "$3$2")
		}},
		{"delivery at the wrong position", func(s string) string {
			return emitStmt.ReplaceAllStringFunc(s, func(l string) string { return strings.Replace(l, ", p.Pos)", ", p.Pos+1)", 1) })
		}},
	} {
		t.Run(m.name, func(t *testing.T) {
			mf, _ := runStreamProbes(t, cases, form, func(code []byte) []byte {
				out := m.mutate(string(code))
				return []byte(out)
			})
			if mf == nil {
				return // the consumer of the mutant failed
			}
			if diff := streamReportDiffs(t, mf, d); len(diff) == 0 {
				t.Errorf("the mutant %q changes no report: the stream is not compared", m.name)
			}
		})
	}
}
