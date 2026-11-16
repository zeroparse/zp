package tsgen

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/codegen"
)

// The library embedded in a TypeScript parser (LibEmbed) is narrowed to what the parser uses, as
// the Go generator narrows its copy of the Go library: shakeLib keeps the declarations of lib.ts
// that the parser's files name, directly or through other kept text, and the methods of the parsing
// machine's classes (shakenClasses) that kept text names. Names are read lexically, so a name that
// only occurs in an unrelated place keeps its declaration too: the shaker may keep more than the
// parser needs, never less. It decides no feature itself.

// shakenClasses are the classes of the library whose methods are kept only when kept text names
// them: the parsing machine, which users of a parser never see. The methods of every other class
// stay with their class, as users may call them.
var shakenClasses = map[string]bool{"Parser": true, "DocumentParser": true}

// implicitMembers are the methods that JavaScript calls without naming them.
var implicitMembers = map[string]bool{"constructor": true, "toString": true, "toJSON": true, "valueOf": true}

// tsTok is a token of TypeScript source: an identifier (ident) or any other token, with its
// offsets in the source and the brace depth at its start.
type tsTok struct {
	ident      bool
	text       string
	start, end int
	depth      int
}

// lexTS returns the tokens of src outside comments, strings, regular expression literals and the
// text of template literals; each literal is one token. A slash starts a regular expression where
// an operand can start, as in JavaScript.
func lexTS(src string) ([]tsTok, error) {
	var toks []tsTok
	depth := 0
	// subst counts, for each open substitution of a template literal, the braces open inside it.
	var subst []int
	i := 0
	template := func() error {
		for i < len(src) {
			switch src[i] {
			case '\\':
				i += 2
			case '`':
				i++
				return nil
			case '$':
				if i+1 < len(src) && src[i+1] == '{' {
					i += 2
					subst = append(subst, 0)
					return nil
				}
				i++
			default:
				i++
			}
		}
		return fmt.Errorf("unterminated template literal")
	}
	regexAfter := func() bool {
		if len(toks) == 0 {
			return true
		}
		t := toks[len(toks)-1]
		if t.ident {
			switch t.text {
			case "return", "typeof", "case", "do", "else", "in", "of", "new", "delete", "void", "throw", "instanceof":
				return true
			}
			return false
		}
		return !strings.ContainsAny(t.text, ")]") && !(t.text[0] >= '0' && t.text[0] <= '9') && t.text != "\""
	}
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				return nil, fmt.Errorf("unterminated comment at offset %d", i)
			}
			i += j + 4
		case c == '/' && regexAfter():
			// A regular expression literal: up to the slash outside a class, then its flags.
			start := i
			class := false
			for i++; i < len(src) && (src[i] != '/' || class); i++ {
				switch src[i] {
				case '\\':
					i++
				case '[':
					class = true
				case ']':
					class = false
				case '\n':
					return nil, fmt.Errorf("unterminated regular expression at offset %d", start)
				}
			}
			for i++; i < len(src) && (unicode.IsLetter(rune(src[i]))); i++ {
			}
			toks = append(toks, tsTok{text: "\"", start: start, end: i, depth: depth})
		case c == '"' || c == '\'':
			start := i
			i++
			for i < len(src) && src[i] != c {
				if src[i] == '\\' {
					i++
				} else if src[i] == '\n' {
					return nil, fmt.Errorf("unterminated string at offset %d", start)
				}
				i++
			}
			i++
			toks = append(toks, tsTok{text: "\"", start: start, end: i, depth: depth})
		case c == '`':
			start := i
			i++
			if err := template(); err != nil {
				return nil, err
			}
			toks = append(toks, tsTok{text: "\"", start: start, end: i, depth: depth})
		case c == '{':
			toks = append(toks, tsTok{text: "{", start: i, end: i + 1, depth: depth})
			depth++
			if n := len(subst); n > 0 {
				subst[n-1]++
			}
			i++
		case c == '}':
			if n := len(subst); n > 0 {
				if subst[n-1] == 0 {
					subst = subst[:n-1]
					i++
					if err := template(); err != nil {
						return nil, err
					}
					continue
				}
				subst[n-1]--
			}
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced brace at offset %d", i)
			}
			toks = append(toks, tsTok{text: "}", start: i, end: i + 1, depth: depth})
			i++
		default:
			r, size := utf8.DecodeRuneInString(src[i:])
			if r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				j := i + size
				for j < len(src) {
					r, size := utf8.DecodeRuneInString(src[j:])
					if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
						break
					}
					j += size
				}
				toks = append(toks, tsTok{ident: !unicode.IsDigit(r), text: src[i:j], start: i, end: j, depth: depth})
				i = j
				continue
			}
			toks = append(toks, tsTok{text: src[i : i+size], start: i, end: i + size, depth: depth})
			i += size
		}
	}
	if depth != 0 || len(subst) != 0 {
		return nil, fmt.Errorf("unbalanced braces at the end")
	}
	return toks, nil
}

// libSeg is a segment of lib.ts: a top-level declaration of name (none for any other statement,
// which is always kept), or within the class of a declaration, a method member, which is kept only
// when kept text names it, or other text of the class (member empty), kept with the class.
type libSeg struct {
	start, end   int
	name, member string
	ids          []string
}

// declKeywords start a top-level declaration, after export, declare or abstract.
var declKeywords = map[string]bool{"class": true, "function": true, "const": true, "let": true, "var": true, "interface": true, "type": true, "enum": true}

// methodModifiers may precede the name of a class member.
var methodModifiers = map[string]bool{"private": true, "protected": true, "public": true, "static": true, "readonly": true, "override": true, "async": true, "abstract": true, "declare": true}

// libSegs splits src into its segments. A segment starts at a line whose first token begins a
// top-level statement or a member of a class body (the comment lines before it included) and ends
// where the next starts.
func libSegs(src string, toks []tsTok) ([]libSeg, error) {
	// lineStart returns the offset of the start of the line of offset o, moved back over the comment
	// and blank lines before it that follow the previous token.
	lineStart := func(o, prevEnd int) int {
		s := strings.LastIndexByte(src[:o], '\n') + 1
		for s > 0 {
			p := strings.LastIndexByte(src[:s-1], '\n') + 1
			line := strings.TrimSpace(src[p : s-1])
			if p < prevEnd || !(line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "/*")) {
				break
			}
			s = p
		}
		return s
	}
	// startsAt reports whether toks[k] is the first token of its line, indented by indent spaces.
	startsAt := func(k, indent int) bool {
		s := strings.LastIndexByte(src[:toks[k].start], '\n') + 1
		return toks[k].start-s == indent && strings.TrimLeft(src[s:toks[k].start], " ") == "" && (k == 0 || toks[k-1].end <= s)
	}
	var segs []libSeg
	cut := func(at int, name, member string) {
		if n := len(segs); n > 0 {
			segs[n-1].end = at
		}
		segs = append(segs, libSeg{start: at, name: name, member: member})
	}
	cut(0, "", "")
	class := ""   // the class whose body the scan is in
	classAt := -1 // the depth of the class body
	for k := 0; k < len(toks); k++ {
		t := toks[k]
		prevEnd := 0
		if k > 0 {
			prevEnd = toks[k-1].end
		}
		switch {
		case class != "" && t.depth == 0 && t.text == "}" && startsAt(k, 0):
			// The end of the class body: the rest of the class is not a member.
			cut(lineStart(t.start, prevEnd), class, "")
			class, classAt = "", -1
		case t.depth == 0 && startsAt(k, 0):
			at := lineStart(t.start, prevEnd)
			j := k
			for j < len(toks) && (toks[j].text == "export" || toks[j].text == "declare" || toks[j].text == "abstract") {
				j++
			}
			if j+1 < len(toks) && declKeywords[toks[j].text] && toks[j+1].ident {
				name := toks[j+1].text
				if (toks[j].text == "const" || toks[j].text == "let" || toks[j].text == "var") && declaresMore(toks, j+1) {
					return nil, fmt.Errorf("the declaration of %s at offset %d declares several names", name, t.start)
				}
				cut(at, name, "")
				class, classAt = "", -1
				if toks[j].text == "class" && shakenClasses[name] {
					class, classAt = name, 1
				}
			} else if t.text != "}" && t.text != ")" && t.text != "]" && t.text != ";" {
				cut(at, "", "")
				class, classAt = "", -1
			}
		case class != "" && t.depth == classAt && startsAt(k, 2) && t.text != "}":
			at := lineStart(t.start, prevEnd)
			j := k
			for j < len(toks) && methodModifiers[toks[j].text] {
				j++
			}
			member := ""
			if j+1 < len(toks) && toks[j].ident && (toks[j+1].text == "(" || toks[j+1].text == "<") && !implicitMembers[toks[j].text] {
				member = toks[j].text
			}
			cut(at, class, member)
		}
	}
	segs[len(segs)-1].end = len(src)
	k := 0
	for i := range segs {
		for k < len(toks) && toks[k].start < segs[i].start {
			k++
		}
		for ; k < len(toks) && toks[k].start < segs[i].end; k++ {
			if toks[k].ident {
				segs[i].ids = append(segs[i].ids, toks[k].text)
			}
		}
	}
	return segs, nil
}

// declaresMore reports whether the const, let or var statement whose name is toks[k] declares
// another name: whether a comma outside brackets, before the statement ends, is followed by a name
// and = or :.
func declaresMore(toks []tsTok, k int) bool {
	nest := 0
	for ; k+2 < len(toks); k++ {
		switch toks[k].text {
		case "(", "[", "{":
			nest++
		case ")", "]", "}":
			nest--
		case ";":
			if nest == 0 {
				return false
			}
		case ",":
			if nest == 0 && toks[k+1].ident && (toks[k+2].text == "=" || toks[k+2].text == ":") {
				return true
			}
		}
	}
	return false
}

// shakeLib returns src without the declarations and the methods of shakenClasses that neither the
// roots (the text of the parser's files) nor kept text name.
func shakeLib(src string, roots []string) (string, error) {
	toks, err := lexTS(src)
	if err != nil {
		return "", fmt.Errorf("the TypeScript library: %v", err)
	}
	segs, err := libSegs(src, toks)
	if err != nil {
		return "", fmt.Errorf("the TypeScript library: %v", err)
	}
	named := map[string]bool{}
	name := func(id string) { named[id] = true }
	for _, r := range roots {
		rt, err := lexTS(r)
		if err != nil {
			return "", fmt.Errorf("a generated file: %v", err)
		}
		for _, t := range rt {
			if t.ident {
				name(t.text)
			}
		}
	}
	kept := make([]bool, len(segs))
	keep := func(i int) {
		if !kept[i] {
			kept[i] = true
			for _, id := range segs[i].ids {
				name(id)
			}
		}
	}
	for i, s := range segs {
		if s.name == "" {
			keep(i)
		}
	}
	for more := true; more; {
		more = false
		for i, s := range segs {
			if !kept[i] && named[s.name] && (s.member == "" || named[s.member]) {
				keep(i)
				more = true
			}
		}
	}
	var b strings.Builder
	for i, s := range segs {
		if kept[i] {
			b.WriteString(src[s.start:s.end])
		}
	}
	return b.String(), nil
}

// usedFeaturesDecl is the declaration of the features of a copy of the library in lib.ts.
const usedFeaturesDecl = "export const usedFeatures: number = allFeatures;\n"

// narrowLib returns the library src with the features fs: usedFeatures declares fs, and each if
// statement guarded by the conjunct (usedFeatures & featureX) !== 0 of a feature X outside fs loses
// its condition and first branch, so that its else branch, if any, takes its place. Any other use
// of usedFeatures is an error, so that no code guarded otherwise stays in a copy without its
// feature unnoticed.
func narrowLib(src string, fs codegen.FeatureSet) (string, error) {
	if strings.Count(src, usedFeaturesDecl) != 1 {
		return "", fmt.Errorf("the TypeScript library does not declare usedFeatures once as %q", strings.TrimSpace(usedFeaturesDecl))
	}
	src = strings.Replace(src, usedFeaturesDecl, fmt.Sprintf("export const usedFeatures: number = %d; // %s\n", fs, fs), 1)
	toks, err := lexTS(src)
	if err != nil {
		return "", fmt.Errorf("the TypeScript library: %v", err)
	}
	bits := map[string]codegen.Feature{}
	for i, name := range codegen.FeatureNames() {
		bits["feature"+strings.ToUpper(name[:1])+name[1:]] = codegen.Feature(1) << i
	}
	// match returns the index of the token that closes the bracket opened at toks[k].
	match := func(k int) (int, error) {
		open, closing := toks[k].text, map[string]string{"(": ")", "{": "}"}[toks[k].text]
		n := 0
		for j := k; j < len(toks); j++ {
			switch toks[j].text {
			case open:
				n++
			case closing:
				if n--; n == 0 {
					return j, nil
				}
			}
		}
		return 0, fmt.Errorf("the TypeScript library: unclosed %s at offset %d", open, toks[k].start)
	}
	uses, guards := 0, 0
	for _, t := range toks {
		if t.text == "usedFeatures" {
			uses++
		}
	}
	var b strings.Builder
	last := 0  // the offset of src up to which b holds the result
	skip := -1 // the index of the last token of removed text
	for k := 0; k+1 < len(toks); k++ {
		if toks[k].text != "if" || toks[k+1].text != "(" {
			continue
		}
		cond, err := match(k + 1)
		if err != nil {
			return "", err
		}
		off := false
		for _, c := range splitAnd(toks[k+2 : cond]) {
			var text strings.Builder
			mentions := false
			for _, t := range c {
				text.WriteString(t.text)
				mentions = mentions || t.text == "usedFeatures"
			}
			if !mentions {
				continue
			}
			s := text.String()
			bit, ok := bits[strings.TrimSuffix(strings.TrimPrefix(s, "(usedFeatures&"), ")!==0")]
			if !ok || !strings.HasPrefix(s, "(usedFeatures&") || !strings.HasSuffix(s, ")!==0") {
				return "", fmt.Errorf("the TypeScript library: the condition of the if at offset %d uses usedFeatures in %q, not as a conjunct (usedFeatures & featureX) !== 0", toks[k].start, s)
			}
			guards++
			off = off || !fs.Has(bit)
		}
		if !off || k <= skip {
			continue
		}
		if toks[cond+1].text != "{" {
			return "", fmt.Errorf("the TypeScript library: the guarded if at offset %d has no block", toks[k].start)
		}
		end, err := match(cond + 1)
		if err != nil {
			return "", err
		}
		start, to := toks[k].start, toks[end].end
		if end+2 < len(toks) && toks[end+1].text == "else" {
			// The else branch, a block or an if, takes the place of the if.
			to = toks[end+2].start
			skip = end + 1
		} else {
			// The statement goes, with its line when nothing else is on it.
			ls := strings.LastIndexByte(src[:start], '\n') + 1
			if le := strings.IndexByte(src[to:], '\n'); strings.TrimSpace(src[ls:start]) == "" && le >= 0 && strings.TrimSpace(src[to:to+le]) == "" {
				start, to = ls, to+le+1
			}
			skip = end
		}
		b.WriteString(src[last:start])
		last = to
	}
	b.WriteString(src[last:])
	if uses != guards+1 {
		return "", fmt.Errorf("the TypeScript library uses usedFeatures %d times besides its declaration, in %d guards: each use must be a conjunct (usedFeatures & featureX) !== 0 of an if condition", uses-1, guards)
	}
	return b.String(), nil
}

// splitAnd splits the tokens of a condition at its && operators outside brackets.
func splitAnd(toks []tsTok) [][]tsTok {
	var out [][]tsTok
	n, from := 0, 0
	for i, t := range toks {
		switch t.text {
		case "(", "[", "{":
			n++
		case ")", "]", "}":
			n--
		case "&":
			if n == 0 && i+1 < len(toks) && toks[i+1].text == "&" && toks[i+1].start == t.end {
				out = append(out, toks[from:i])
				from = i + 2
			}
		}
	}
	return append(out, toks[from:])
}
