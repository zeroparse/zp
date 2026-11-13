package tsgen

import (
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/codegen"
)

// TsGeneratedHeader starts every generated TypeScript file. The command line tool removes stale
// files of an earlier generation only if they start with it.
func TsGeneratedHeader() string { return codegen.Header() + "\n\n" }

// tsLibFileName is the file that holds the library: in the library source, and next to the parser
// (LibEmbed).
const tsLibFileName = "lib.ts"

// tsEmbeddedLibSpecifier is how parser.ts and types.ts import the library they sit next to. The .ts
// extension is the file's own name, which libraries that strip types resolve as it is.
const tsEmbeddedLibSpecifier = "./lib.ts"

// tsTypesSpecifier is how parser.ts imports types.ts.
const tsTypesSpecifier = "./types.ts"

// tsPublicLib lists the library declarations that parser.ts re-exports: the names a user of the
// parser sees. The other exports of lib.ts are the contract between the library and the code
// generated from a grammar.
var tsPublicLib = strings.Fields(`
	Value Unit CodePoints Bytes NodeField NodeJSON Node marshal SyntaxError SyntaxErrors ParseResult
	ParseOptions Limits ParseInterruptionKind ParseInterruptionError Unlimited
	TraceKind TraceEvent Profile ProfileLocation SkipReasonCount RuleProfile ProfileSnapshot`)

// tsPublicControls lists the library declarations of the limit controls that parser.ts re-exports
// only with -limits.
var tsPublicControls = strings.Fields(`CancellationSignal`)

// substituteMethods replaces methods of the Parser class in the library source src by the printed
// methods ms: each method, with the comment lines before it, from its first line at the class
// body's indentation to the closing brace at that indentation.
func substituteMethods(src string, ms []tsMethod) string {
	for _, m := range ms {
		head := "\n  " + m.Name + "("
		i := strings.Index(src, head)
		if i < 0 || strings.Count(src, head) != 1 {
			panic("tsgen: the library has not exactly one method " + m.Name)
		}
		start := i + 1
		for {
			j := strings.LastIndex(src[:start-1], "\n") + 1
			if !strings.HasPrefix(src[j:start-1], "  //") {
				break
			}
			start = j
		}
		k := strings.Index(src[i:], "\n  }\n")
		if k < 0 {
			panic("tsgen: the method " + m.Name + " of the library has no closing brace at the class body's indentation")
		}
		end := i + k + len("\n  }\n")
		src = src[:start] + m.Text + src[end:]
	}
	return src
}

// tsFiles assembles the files of a parser from the code generated from the grammar (body) and the
// declarations of its types (types, empty without opts.Types).
func tsFiles(opts codegen.GenOptions, body, types string) ([]codegen.GeneratedFile, error) {
	lib, err := loadLib(opts.TSLib)
	if err != nil {
		return nil, err
	}
	spec := tsEmbeddedLibSpecifier
	if opts.Lib == codegen.LibImport {
		spec = opts.LibModule
	}
	var files []codegen.GeneratedFile
	var parser strings.Builder
	parser.WriteString(TsGeneratedHeader())
	parser.WriteString("// A parser generated from a ZeroParse grammar; see parse below. It imports its runtime.\n\n")
	parser.WriteString("/* eslint-disable */\n\n")
	used := tsIdentifiers(body)
	exports := lib.exports
	var names []string
	for name := range used {
		if _, ok := exports[name]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	parser.WriteString("import {" + tsImportNames(names, exports) + "} from " + TsString(spec) + ";\n")
	if types != "" {
		parser.WriteString("import type * as Types from " + TsString(tsTypesSpecifier) + ";\n")
	}
	parser.WriteString("\n// The names of the library that users of the parser need.\n")
	var values, typs []string
	public := slices.Clone(tsPublicLib)
	if opts.LimitControls {
		public = append(public, tsPublicControls...)
	}
	for _, name := range public {
		export := name
		if !opts.LimitControls && (name == "ParseOptions" || name == "Limits") {
			// Without limit controls the options have no cancellation, and the limits no compute limit.
			export = "Plain" + name + " as " + name
			name = "Plain" + name
		}
		if exports[name] {
			typs = append(typs, export)
		} else {
			values = append(values, export)
		}
	}
	if len(values) > 0 {
		parser.WriteString("export {" + strings.Join(values, ", ") + "} from " + TsString(spec) + ";\n")
	}
	if len(typs) > 0 {
		parser.WriteString("export type {" + strings.Join(typs, ", ") + "} from " + TsString(spec) + ";\n")
	}
	if types != "" {
		// parser.ts stays the single entry point: the types of the grammar are exported from it too.
		parser.WriteString("\n// The types of the grammar.\nexport type * from " + TsString(tsTypesSpecifier) + ";\n")
	}
	parser.WriteString("\n")
	parser.WriteString(body)
	if opts.Lib != codegen.LibImport {
		files = append(files, codegen.GeneratedFile{Path: tsLibFileName, Content: []byte(lib.text)})
	}
	files = append(files, codegen.GeneratedFile{Path: "parser.ts", Content: []byte(parser.String())})
	if types != "" {
		var t strings.Builder
		t.WriteString(TsGeneratedHeader())
		t.WriteString("// The types of a ZeroParse grammar, as the typed parseValue of parser.ts returns them.\n\n")
		t.WriteString("/* eslint-disable */\n\n")
		t.WriteString("import type {Node as RuntimeNode} from " + TsString(spec) + ";\n\n")
		t.WriteString(types)
		files = append(files, codegen.GeneratedFile{Path: "types.ts", Content: []byte(t.String())})
	}
	return files, nil
}

// tsImportNames renders the names of an import list; names only used as types are imported as types.
func tsImportNames(names []string, exports map[string]bool) string {
	parts := make([]string, len(names))
	for i, name := range names {
		if exports[name] {
			parts[i] = "type " + name
		} else {
			parts[i] = name
		}
	}
	return strings.Join(parts, ", ")
}

// tsIdentifiers returns the identifiers that code uses: those outside comments and strings (the
// parts of template literals included) that do not follow a dot, which only names a property.
func tsIdentifiers(code string) map[string]bool {
	ids := map[string]bool{}
	// braces counts, for each open template literal substitution, the unclosed braces inside it.
	var braces []int
	i := 0
	template := func() {
		// i is after a backtick or a closing brace of a substitution: read template text.
		for i < len(code) {
			switch code[i] {
			case '\\':
				i += 2
			case '`':
				i++
				return
			case '$':
				if i+1 < len(code) && code[i+1] == '{' {
					i += 2
					braces = append(braces, 0)
					return
				}
				i++
			default:
				i++
			}
		}
	}
	for i < len(code) {
		c := code[i]
		switch {
		case c == '/' && i+1 < len(code) && code[i+1] == '/':
			for i < len(code) && code[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(code) && code[i+1] == '*':
			j := strings.Index(code[i+2:], "*/")
			if j < 0 {
				return ids
			}
			i += j + 4
		case c == '"' || c == '\'':
			i++
			for i < len(code) && code[i] != c {
				if code[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '`':
			i++
			template()
		case c == '{':
			if n := len(braces); n > 0 {
				braces[n-1]++
			}
			i++
		case c == '}':
			if n := len(braces); n > 0 {
				if braces[n-1] == 0 {
					braces = braces[:n-1]
					i++
					template()
					continue
				}
				braces[n-1]--
			}
			i++
		default:
			r, size := utf8.DecodeRuneInString(code[i:])
			if r == '_' || r == '$' || unicode.IsLetter(r) {
				j := i + size
				for j < len(code) {
					r, size := utf8.DecodeRuneInString(code[j:])
					if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
						break
					}
					j += size
				}
				// A property name follows a dot, but a spread (...x) names a variable.
				prop := i > 0 && code[i-1] == '.' && !(i > 2 && code[i-2] == '.' && code[i-3] == '.')
				if !prop {
					ids[code[i:j]] = true
				}
				i = j
				continue
			}
			if unicode.IsDigit(r) {
				// A number, including its suffixes and exponent: not an identifier.
				for i < len(code) && (unicode.IsLetter(rune(code[i])) || unicode.IsDigit(rune(code[i])) || code[i] == '_' || code[i] == '.') {
					i++
				}
				continue
			}
			i += size
		}
	}
	return ids
}
