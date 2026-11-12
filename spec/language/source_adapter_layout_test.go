package language_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/grammar"
)

// The formatter reads the raw comment tokens and line breaks that the parser
// keeps beside the source metadata (the fields that the JSON form leaves out).
// The canonical frontend must build them as the hand-written parser does, so
// that Format writes the same text for the grammar of either.

// rawLayout returns every exported field under v, with its path, including the
// fields that the JSON form leaves out. A comment is written whole at each
// place that holds it.
func rawLayout(lines []string, path string, v reflect.Value) []string {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return lines
		}
		if c, ok := v.Interface().(*grammar.Comment); ok {
			return append(lines, fmt.Sprintf("%s = %d:%d-%d:%d %q blank=%v header=%d:%d run=%d", path, c.Pos.Line, c.Pos.Col,
				c.End.Line, c.End.Col, c.Text, c.Blank, c.Header.Line, c.Header.Col, len(c.Run)))
		}
		return rawLayout(lines, path, v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return lines
		}
		return rawLayout(lines, path+"("+v.Elem().Type().String()+")", v.Elem())
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			lines = rawLayout(lines, fmt.Sprintf("%s[%d]", path, i), v.Index(i))
		}
		return lines
	case reflect.Map:
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
		for _, k := range keys {
			lines = rawLayout(lines, fmt.Sprintf("%s[%v]", path, k), v.MapIndex(k))
		}
		return lines
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Type().Field(i); f.IsExported() {
				lines = rawLayout(lines, path+"."+f.Name, v.Field(i))
			}
		}
		return lines
	case reflect.Bool, reflect.Int, reflect.Int64, reflect.Uint64, reflect.String, reflect.Float64, reflect.Uint8, reflect.Int32:
		if !v.IsZero() {
			lines = append(lines, fmt.Sprintf("%s = %v", path, v.Interface()))
		}
		return lines
	}
	return append(lines, fmt.Sprintf("%s = %v", path, v.Interface()))
}

var layoutIndex = regexp.MustCompile(`\[\d+\]`)

// layoutDifference describes where the hand-written parser and the canonical
// frontend build a different grammar for src, raw layout included. class is
// the first place that differs, as its path without list indexes, or
// "format" when only the text that Format writes differs. class is "" when the
// two agree, or when either rejects src.
func layoutDifference(src string) (class, detail string) {
	manual, err := zp.ParseGrammar(src)
	if err != nil {
		return "", ""
	}
	got, err := adaptCanonical(src)
	if err != nil {
		return "", ""
	}
	want, have := rawLayout(nil, "", reflect.ValueOf(manual)), rawLayout(nil, "", reflect.ValueOf(got))
	for i := 0; i < max(len(want), len(have)); i++ {
		var w, h string
		if i < len(want) {
			w = want[i]
		}
		if i < len(have) {
			h = have[i]
		}
		if w == h {
			continue
		}
		path := w
		if path == "" {
			path = h
		}
		path, _, _ = strings.Cut(path, " = ")
		return layoutIndex.ReplaceAllString(path, "[]"), fmt.Sprintf("\n manual:    %s\n canonical: %s", w, h)
	}
	if a, b := grammar.Format(manual), grammar.Format(got); a != b {
		return "format", fmt.Sprintf("\nmanual:\n%s\ncanonical:\n%s", a, b)
	}
	return "", ""
}

// withComment inserts a comment at a pseudo-random offset of src that is
// outside strings, classes and comments: before a token, after it, or on a
// line of its own.
func withComment(rng *rand.Rand, src string) string {
	rs := []rune(src)
	var places []int
	in := rune(0)
	for i := 0; i <= len(rs); i++ {
		if in == 0 && (i == 0 || i == len(rs) || rs[i] == ' ' || rs[i] == '\n' || rs[i-1] == ' ' || rs[i-1] == '\n' || strings.ContainsRune("()[]{},;=/", rs[i])) {
			places = append(places, i)
		}
		if i == len(rs) {
			break
		}
		switch r := rs[i]; {
		case in == 0 && (r == '"' || r == '['):
			in = r
		case in == '"' && r == '\\', in == '[' && r == '\\':
			i++
		case in == '"' && r == '"', in == '[' && r == ']':
			in = 0
		case in == 0 && r == '/' && i+1 < len(rs) && (rs[i+1] == '/' || rs[i+1] == '*'):
			return src // a source with comments keeps them as they are
		}
	}
	k := places[rng.Intn(len(places))]
	comments := []string{"/* c */", "// c\n", "\n// c\n", "\n/* c */\n", "/* a\n b */", "\n\n// c\n\n"}
	return string(rs[:k]) + comments[rng.Intn(len(comments))] + string(rs[k:])
}

func TestCanonicalLayout(t *testing.T) {
	corpus := mutationCorpus(t)
	inputs := append([]string(nil), corpus...)
	rng := rand.New(rand.NewSource(70))
	n := 3000
	if testing.Short() {
		n = 600
	}
	for range n {
		inputs = append(inputs, mutatedSource(rng, corpus))
		base := corpus[rng.Intn(len(corpus))]
		if len(base) < 2000 {
			inputs = append(inputs, withComment(rng, base), withComment(rng, withComment(rng, base)))
		}
	}
	classes := map[string]int{}
	examples := map[string]string{}
	compared := 0
	for _, src := range inputs {
		if _, err := zp.ParseGrammar(src); err == nil {
			compared++
		}
		class, detail := layoutDifference(src)
		if class == "" {
			continue
		}
		classes[class]++
		if _, ok := examples[class]; !ok {
			examples[class] = fmt.Sprintf("%q%s", src, detail)
		}
	}
	keys := make([]string, 0, len(classes))
	for k := range classes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return classes[keys[i]] > classes[keys[j]] })
	total := 0
	for _, k := range keys {
		total += classes[k]
		t.Errorf("the canonical frontend builds a different raw layout or format for %d sources at %s, as for\n%s", classes[k], k, examples[k])
	}
	t.Logf("%d of %d accepted sources differ", total, compared)
}
