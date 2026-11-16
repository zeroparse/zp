//go:build zp_long

package gogen_test

import (
	"regexp"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
)

// examinedMutant leaves out one record of the examined end on one route of an observed parse: in the
// generated code (code) or in a file of the library (file), it replaces each copy of old with new.
// The exactness test must then report a difference on each of the entries.
type examinedMutant struct {
	name, file, old, new string
	code                 bool
	entries              []string
}

var examinedMutants = []examinedMutant{
	// The reads of the traced typed and recognizer bodies.
	{name: "unrecorded character read of a typed or recognizer body", code: true, old: "p.Peek()", new: "p.PeekOrdinary()",
		entries: []string{"value", "recognize"}},
	// The typed bodies compare literals through Peek.
	{name: "unrecorded literal read of a recognizer body", code: true, old: "p.MatchLiteral(", new: "p.MatchLiteralOrdinary(",
		entries: []string{"recognize"}},
	// The reads of a stream in the traced Node bodies.
	{name: "unrecorded character read of a stream", file: "stream.go", old: "\tp.documentTouch(end)\n\treturn r, size, ok\n", new: "\t_ = end\n\treturn r, size, ok\n",
		entries: []string{"stream"}},
	{name: "unrecorded literal match of a stream", file: "stream.go", old: "\tp.documentTouch(p.Pos)\n\tif !build {\n\t\treturn nil, true\n\t}\n\treturn p.NewNode(Data{Kind: coreimpl.MatchKind, Start: int32(start), End: int32(p.Pos), Text: p.LiteralText(start, text)",
		new:     "\tif !build {\n\t\treturn nil, true\n\t}\n\treturn p.NewNode(Data{Kind: coreimpl.MatchKind, Start: int32(start), End: int32(p.Pos), Text: p.LiteralText(start, text)",
		entries: []string{"stream"}},
	// The records that the library replays for a result it reuses.
	{name: "a Node memo hit records nothing", file: "call_exec.go", old: "p.documentTouch(e.examined)", new: "p.documentTouch(0)",
		entries: []string{"parse", "recognize"}},
	{name: "a typed memo hit records nothing", file: "call_exec.go", old: "p.documentTouch(p.memoExamined[e])", new: "p.documentTouch(0)",
		entries: []string{"value"}},
	{name: "a context replay records nothing", file: "context_memo.go", old: "\tp.Observation.memoHit()\n\tp.documentTouch(e.examined)\n", new: "\tp.Observation.memoHit()\n",
		entries: []string{"parse"}},
	{name: "a skip record reused records nothing", file: "skip_runs.go", old: "p.documentTouch(r.examined)", new: "p.documentTouch(0)",
		entries: []string{"parse"}},
	{name: "a lookahead drops what it examined", file: "context_memo.go", old: "p.docExamined = max(l.examined, p.docExamined)", new: "p.docExamined = l.examined",
		entries: []string{"parse"}},
}

// TestObservedExaminedEndsMutants checks that TestObservedExaminedEndsAreExact notices a record of the
// examined end left out on each route: each mutant must change the examined end of some exit of each of
// its entries.
func TestObservedExaminedEndsMutants(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	codegentest.PinOrdinary(t)
	cases := examinedMutantCases(t)
	for _, m := range examinedMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			replaced := 0
			lib := codegentest.LibCopy()
			var mutate func([]byte) []byte
			if m.code {
				mutate = func(code []byte) []byte {
					replaced += strings.Count(string(code), m.old)
					return []byte(strings.ReplaceAll(string(code), m.old, m.new))
				}
			} else {
				lib = codegentest.LibCopyWith(t, func(name, src string) string {
					if name != m.file {
						return src
					}
					replaced += strings.Count(src, m.old)
					return strings.ReplaceAll(src, m.old, m.new)
				}, nil)
			}
			reports := runExaminedProbes(t, cases, lib, mutate)
			if replaced == 0 {
				t.Fatalf("no %q in the code", m.old)
			}
			_, diffs := examinedMismatches(t, cases, reports)
			for _, entry := range m.entries {
				if len(diffs[entry]) == 0 {
					t.Errorf("the mutant changes no examined end of %s", entry)
				}
			}
		})
	}
}

// examinedMutantShapes selects the cases of the generated corpus that the mutants need: memoized
// rules, lookaheads, left recursion, cuts and recovery.
var examinedMutantShapes = regexp.MustCompile(`(?i)leader|recurs|look|recover|memo|cut|LR`)

// examinedMutantCases are the cases the mutants run on: those of the generated corpus that
// examinedMutantShapes selects, the shapes of the observation tests and the stream grammars, a
// smaller set than examinedCases, as each mutant builds every case again.
func examinedMutantCases(t *testing.T) []codegentest.Case {
	t.Helper()
	var cases []codegentest.Case
	for _, c := range codegentest.GenCorpus(t) {
		if examinedMutantShapes.MatchString(c.Name) && !strings.Contains(c.Src, "#stream") {
			cases = append(cases, c)
		}
	}
	cases = append(cases, observeShapes...)
	cases = append(cases, examinedExtras()...)
	for _, c := range streamCases(t) {
		if strings.Contains(c.Src, "#stream") && len(c.Inputs) > 0 {
			cases = append(cases, c)
		}
	}
	return cases
}
