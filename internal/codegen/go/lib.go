package gogen

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/format"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"zeroparse.dev/zp/internal/codegen"
)

// The Go library of generated parsers is the package zeroparse.dev/zp/codegen/go/lib, whose exported
// names that are documented as for generated parsers the generated code reaches, together with the
// package core, which declares the result types (Node, Limits, SyntaxError, ...) that generated
// parsers share with the engine. A generated package imports core and either imports the library or
// carries a copy of its files (embed mode), which the generator reads from the source in
// GenOptions.GoLib; the declarations of core come from the index coreAPI.

// modulePath is the path of this module, libImportPath the import path of the library in it, and
// coreImportPath that of the package core.
const (
	modulePath     = "zeroparse.dev/zp"
	libImportPath  = modulePath + "/codegen/go/lib"
	coreImportPath = modulePath + "/core"
)

// libAlias is the name generated code gives to the library, coreAlias the name it gives to the
// package core, and typesAlias the name it gives to the types package of a typed parser.
const (
	libAlias   = "rt"
	coreAlias  = "core"
	TypesAlias = "types"
)

// libCache memoizes the parsed library of each source, so that the many generations of one
// process parse it once. The key is the source value of GenOptions.GoLib, used only for a
// source whose dynamic type is a string or a pointer (os.DirFS, an embedded directory): such a value
// is always a valid map key. Any other source is parsed on every call.
var libCache struct {
	mu   sync.Mutex
	info map[fs.FS]*libInfo
}

// libFile is a file of the library: name is its path relative to the library's
// directory.
type libFile struct {
	name string
	data []byte
}

// libDecl is an exported top-level declaration of the library or of core.
type libDecl struct {
	kind token.Token // token.TYPE, CONST, VAR or FUNC
	doc  string
	spec goast.Node
	// pkg is the name generated code gives to the package that declares it: libAlias or
	// coreAlias.
	pkg string
}

type libInfo struct {
	files []libFile
	Decls map[string]*libDecl
	// protocol is the name of the constant that declares the newest protocol version the library
	// implements (ProtocolN), to which a parser that imports the library refers.
	protocol string
}

// loadLib parses the library source of opts (once per source) and returns its files and
// exported declarations.
func loadLib(opts codegen.GenOptions) (*libInfo, error) {
	fsys := opts.GoLib
	if fsys == nil {
		return nil, fmt.Errorf("Go generation needs the library source: set GenOptions.GoLib")
	}
	k := reflect.TypeOf(fsys).Kind()
	cacheable := k == reflect.String || k == reflect.Pointer
	libCache.mu.Lock()
	defer libCache.mu.Unlock()
	if cacheable {
		if info, ok := libCache.info[fsys]; ok {
			return info, nil
		}
	}
	info, err := parseLib(fsys)
	if err != nil {
		return nil, err
	}
	if cacheable {
		if libCache.info == nil {
			libCache.info = map[fs.FS]*libInfo{}
		}
		libCache.info[fsys] = info
	}
	return info, nil
}

// parseLib reads the files of the library in fsys and its exported declarations, and adds
// those of core from the index.
func parseLib(fsys fs.FS) (*libInfo, error) {
	info := &libInfo{Decls: map[string]*libDecl{}}
	fset := token.NewFileSet()
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		info.files = append(info.files, libFile{name, data})
		f, err := goparser.ParseFile(fset, name, data, goparser.ParseComments|goparser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		if err := addLibDecls(info, f, libAlias); err != nil {
			return nil, err
		}
	}
	if err := addCoreDecls(info, coreAPI); err != nil {
		return nil, err
	}
	if len(info.files) == 0 {
		return nil, fmt.Errorf("the library source has no files")
	}
	best := 0
	for name, d := range info.Decls {
		if n, ok := strings.CutPrefix(name, "Protocol"); ok && d.kind == token.CONST && d.pkg == libAlias {
			if v, err := strconv.Atoi(n); err == nil && v > best {
				best, info.protocol = v, name
			}
		}
	}
	if info.protocol == "" {
		return nil, fmt.Errorf("the library source declares no protocol version (a constant ProtocolN)")
	}
	sort.Slice(info.files, func(i, j int) bool { return info.files[i].name < info.files[j].name })
	return info, nil
}

// addLibDecls adds the exported top-level declarations of f, a file of the package that
// generated code names pkg, to info. A name that both the library and core declare is an error:
// generated code could not tell which it names.
func addLibDecls(info *libInfo, f *goast.File, pkg string) error {
	add := func(name string, d *libDecl) error {
		if prev, ok := info.Decls[name]; ok && prev.pkg != pkg {
			return fmt.Errorf("the library source declares %s in both %s and %s", name, prev.pkg, pkg)
		}
		d.pkg = pkg
		info.Decls[name] = d
		return nil
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *goast.FuncDecl:
			if d.Recv == nil && d.Name.IsExported() {
				if err := add(d.Name.Name, &libDecl{kind: token.FUNC, doc: d.Doc.Text(), spec: d}); err != nil {
					return err
				}
			}
		case *goast.GenDecl:
			for _, s := range d.Specs {
				doc := d.Doc
				switch s := s.(type) {
				case *goast.TypeSpec:
					if s.Doc != nil {
						doc = s.Doc
					}
					if s.Name.IsExported() {
						if err := add(s.Name.Name, &libDecl{kind: token.TYPE, doc: doc.Text(), spec: s}); err != nil {
							return err
						}
					}
				case *goast.ValueSpec:
					if s.Doc != nil {
						doc = s.Doc
					}
					for _, n := range s.Names {
						if n.IsExported() {
							if err := add(n.Name, &libDecl{kind: d.Tok, doc: doc.Text(), spec: s}); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// libFiles returns the copy of the library that LibEmbed writes, in internal/libgo. The copy
// imports core and core/noderep of this module, as generated code does.
func libFiles(opts codegen.GenOptions) ([]codegen.GeneratedFile, error) {
	info, err := loadLib(opts)
	if err != nil {
		return nil, err
	}
	var out []codegen.GeneratedFile
	for _, f := range info.files {
		out = append(out, codegen.GeneratedFile{Path: path.Join(embeddedLibDir, f.name), Content: []byte(codegen.Header() + "\n" + string(f.data))})
	}
	return out, nil
}

// embeddedLibDir is the directory of the copy of the library in a package generated with LibEmbed.
const embeddedLibDir = "internal/libgo"

// libImport returns the import path of the library for a generated package and for the copy of
// the library that embed and only write.
func libImport(opts codegen.GenOptions) string {
	if opts.Lib == codegen.LibEmbed || opts.Lib == codegen.LibOnly {
		return path.Join(opts.Module, embeddedLibDir)
	}
	return libImportPath
}

// stdImports are the standard-library packages generated code may use, by the name it uses them
// with.
var stdImports = map[string]string{
	"atomic": "sync/atomic", "bufio": "bufio", "bytes": "bytes", "context": "context", "errors": "errors",
	"fmt": "fmt", "io": "io", "iter": "iter", "math": "math", "slices": "slices", "sort": "sort",
	"strconv": "strconv", "strings": "strings", "sync": "sync", "unicode": "unicode", "unsafe": "unsafe",
	"utf8": "unicode/utf8",
}

// typesPackage describes the types package of a typed parser for qualifyLib.
type typesPackage struct {
	ImportPath string
	// Declared reports whether the types package declares the name besides the types of the
	// grammar, which the parser package refers to as types.Name (a type of the grammar is
	// written so in the source).
	Declared func(name string) bool
	// Types holds the type declarations of the types package, which resolve the literals of
	// types.Name.
	Types map[string]goast.Expr
	// schema is the name of the constant of the types package that ties it to the parser package.
	schema string
}

// qualifyLib finishes the generated source body of a package. Every identifier that the body
// leaves unresolved and that is an exported name of the library or of core is that package's, and
// is qualified with it; one that the types package tp (if any) declares is qualified with the types
// package. The standard-library packages the body uses are imported, the library, whose import path
// is rtImport, core if the body names it, and the types package.
func qualifyLib(rtImport, body string, info *libInfo, tp *typesPackage) (string, error) {
	const prefix = "package p\n"
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "body.go", prefix+body, goparser.ParseComments)
	if err != nil {
		return "", err
	}
	// The offsets of the names to qualify, with the qualifier of each.
	type insertion struct {
		off       int
		qualifier string
	}
	var inserts []insertion
	qualify := func(id *goast.Ident, d *libDecl) {
		inserts = append(inserts, insertion{fset.Position(id.Pos()).Offset - len(prefix), d.pkg + "."})
	}
	std := map[string]bool{}
	selectorBase := map[*goast.Ident]bool{}
	goast.Inspect(f, func(n goast.Node) bool {
		switch n := n.(type) {
		case *goast.SelectorExpr:
			if id, ok := n.X.(*goast.Ident); ok {
				selectorBase[id] = true
			}
		}
		return true
	})
	var typeOffsets []int
	for _, id := range f.Unresolved {
		if tp != nil && tp.Declared(id.Name) {
			typeOffsets = append(typeOffsets, fset.Position(id.Pos()).Offset-len(prefix))
			continue
		}
		if d, ok := info.Decls[id.Name]; ok {
			qualify(id, d)
			continue
		}
		if selectorBase[id] {
			if p, ok := stdImports[id.Name]; ok {
				std[p] = true
			}
			if tp != nil && id.Name == TypesAlias {
				typeOffsets = append(typeOffsets, -1)
			}
		}
	}
	// A key of a composite literal that is not of a struct is an expression, which may name a
	// runtime constant; the parser does not report it as unresolved, since a key may name a field.
	// The kind of the literal comes from its type, resolved through the type declarations of the
	// body and of the library, and for a literal with its type elided, from the enclosing one.
	types := map[string]goast.Expr{}
	for _, d := range f.Decls {
		if d, ok := d.(*goast.GenDecl); ok && d.Tok == token.TYPE {
			for _, s := range d.Specs {
				ts := s.(*goast.TypeSpec)
				types[ts.Name.Name] = ts.Type
			}
		}
	}
	var qualifyKeys func(lit *goast.CompositeLit, typ goast.Expr)
	qualifyKeys = func(lit *goast.CompositeLit, typ goast.Expr) {
		if lit.Type != nil {
			typ = lit.Type
		}
		kind, key, elem := literalType(typ, types, info, tp)
		for _, e := range lit.Elts {
			kv, isKV := e.(*goast.KeyValueExpr)
			if !isKV {
				if sub, ok := e.(*goast.CompositeLit); ok && sub.Type == nil {
					qualifyKeys(sub, elem)
				}
				continue
			}
			if id, ok := kv.Key.(*goast.Ident); ok && kind != 's' && id.Obj == nil {
				if d, ok := info.Decls[id.Name]; ok && (d.kind == token.CONST || d.kind == token.VAR) {
					qualify(id, d)
				}
			}
			if sub, ok := kv.Key.(*goast.CompositeLit); ok && sub.Type == nil {
				qualifyKeys(sub, key)
			}
			if sub, ok := kv.Value.(*goast.CompositeLit); ok && sub.Type == nil {
				qualifyKeys(sub, elem)
			}
		}
	}
	goast.Inspect(f, func(n goast.Node) bool {
		if lit, ok := n.(*goast.CompositeLit); ok && lit.Type != nil {
			qualifyKeys(lit, nil)
		}
		return true
	})
	coreUsed := false
	for _, in := range inserts {
		coreUsed = coreUsed || in.qualifier == coreAlias+"."
	}
	typesUsed := false
	for _, off := range typeOffsets {
		typesUsed = true
		if off >= 0 {
			inserts = append(inserts, insertion{off, TypesAlias + "."})
		}
	}
	sort.Slice(inserts, func(i, j int) bool { return inserts[i].off < inserts[j].off })
	// One pass over the insertion offsets in ascending order; a name found twice is qualified once.
	var out strings.Builder
	out.Grow(len(body) + len(inserts)*(len(libAlias)+1))
	done, last := 0, -1
	for _, in := range inserts {
		if in.off == last {
			continue
		}
		last = in.off
		out.WriteString(body[done:in.off])
		out.WriteString(in.qualifier)
		done = in.off
	}
	out.WriteString(body[done:])
	var imports strings.Builder
	var paths []string
	for p := range std {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	imports.WriteString("import (\n")
	for _, p := range paths {
		fmt.Fprintf(&imports, "\t%q\n", p)
	}
	fmt.Fprintf(&imports, "\n\t%s %q\n", libAlias, rtImport)
	if coreUsed {
		fmt.Fprintf(&imports, "\t%q\n", coreImportPath)
	}
	if typesUsed {
		fmt.Fprintf(&imports, "\t%s %q\n", TypesAlias, tp.ImportPath)
	}
	imports.WriteString(")\n\n")
	return imports.String() + out.String(), nil
}

// literalType returns the kind of the composite literal of type typ ('s' for a struct, 'm' for a
// map, 'a' for an array or slice, 0 if unknown) and the types of its keys and elements (nil if
// unknown). Named types are resolved through types, the type declarations of the body, and then
// through the library's.
func literalType(typ goast.Expr, types map[string]goast.Expr, info *libInfo, tp *typesPackage) (kind byte, key, elem goast.Expr) {
	for depth := 0; typ != nil && depth < 32; depth++ {
		switch t := typ.(type) {
		case *goast.ParenExpr:
			typ = t.X
		case *goast.StarExpr:
			typ = t.X
		case *goast.IndexExpr:
			typ = t.X
		case *goast.IndexListExpr:
			typ = t.X
		case *goast.StructType:
			return 's', nil, nil
		case *goast.SelectorExpr:
			// A type of the types package.
			if id, ok := t.X.(*goast.Ident); ok && id.Name == TypesAlias && tp != nil {
				if next, ok := tp.Types[t.Sel.Name]; ok {
					typ = next
					continue
				}
			}
			return 0, nil, nil
		case *goast.MapType:
			return 'm', t.Key, t.Value
		case *goast.ArrayType:
			return 'a', nil, t.Elt
		case *goast.Ident:
			if next, ok := types[t.Name]; ok {
				typ = next
			} else if d, ok := info.Decls[t.Name]; ok && d.kind == token.TYPE {
				typ = d.spec.(*goast.TypeSpec).Type
			} else {
				return 0, nil, nil
			}
		default:
			return 0, nil, nil
		}
	}
	return 0, nil, nil
}

// declaredNames returns the top-level names (other than methods) that the Go source declares.
func declaredNames(src string) ([]string, error) {
	f, err := goparser.ParseFile(token.NewFileSet(), "src.go", src, goparser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *goast.FuncDecl:
			if d.Recv == nil && d.Name.Name != "init" {
				out = append(out, d.Name.Name)
			}
		case *goast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *goast.TypeSpec:
					out = append(out, s.Name.Name)
				case *goast.ValueSpec:
					for _, n := range s.Names {
						out = append(out, n.Name)
					}
				}
			}
		}
	}
	return out, nil
}

// apiNames returns the library names that the generated package re-exports for its users, in
// declaration order by group.
func apiNames(opts codegen.GenOptions) []string {
	names := []string{
		"Node", "NodeKind", "KindString", "KindTuple", "KindList", "KindError", "KindStruct",
		"KindTupleValue", "KindListValue", "KindValue", "SyntaxError", "SyntaxErrors", "PositionUnit", "CodePoints", "Bytes",
		"ParseOption", "WithUnit", "WithLimits", "Limits", "LimitKind", "LimitError", "ErrLimit", "LimitCompute", "LimitInput",
		"LimitDepth", "LimitMemory", "Unlimited",
		"Cursor", "CycleError", "WalkAction", "WalkContinue", "WalkSkip", "WalkStop", "WalkVisit", "WalkEdge", "WalkEdgeKind",
		"WalkRoot", "WalkChild", "WalkField", "WalkListElement", "Walk",
	}
	if opts.Expectations {
		names = append(names, "WithExpectations")
	}
	if opts.LimitControls {
		names = append(names, "WithContext")
	}
	if opts.Observability {
		names = append(names, "TraceKind", "TraceEnter", "TraceExit", "TraceSkipped", "TraceEvent", "WithTrace", "WithTraceFilter", "WithTraceMaxDepth",
			"WithProfile", "Profile", "ProfileSnapshot", "RuleProfile", "ProfileLocation", "SkipReasonCount")
	}
	if opts.Stream {
		names = append(names, "StreamIterator", "ErrStreamClosed", "WithDiagnosticCallback", "WithDiagnosticLimit", "TruncatedSyntaxErrors")
	}
	if opts.Document {
		names = append(names, "ParseStats")
		if !opts.Types {
			names = append(names, "Document")
		}
	}
	if opts.Types {
		names = append(names, "Span")
	}
	return names
}

// apiSource returns the source of the file of the generated package that re-exports the names of
// the library and of core that apiNames lists. Types, constants and variables are aliases;
// functions are wrappers that the compiler inlines.
func apiSource(opts codegen.GenOptions, info *libInfo) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\npackage %s\n\n", codegen.Header(), opts.Package)
	b.WriteString("import (\n\t\"context\"\n\n")
	fmt.Fprintf(&b, "\t%q\n", coreImportPath)
	fmt.Fprintf(&b, "\t%s %q\n)\n\n", libAlias, libImport(opts))
	b.WriteString("var _ context.Context\n\n")
	for _, name := range apiNames(opts) {
		d := info.Decls[name]
		if d == nil {
			return "", fmt.Errorf("internal error: the library has no %s", name)
		}
		pkg := d.pkg
		doc := d.doc
		for _, line := range strings.Split(strings.TrimRight(doc, "\n"), "\n") {
			if line == "" && doc == "" {
				break
			}
			if line == "" {
				b.WriteString("//\n")
			} else {
				b.WriteString("// " + line + "\n")
			}
		}
		switch d.kind {
		case token.TYPE:
			fmt.Fprintf(&b, "type %s = %s.%s\n\n", name, pkg, name)
		case token.CONST:
			fmt.Fprintf(&b, "const %s = %s.%s\n\n", name, pkg, name)
		case token.VAR:
			fmt.Fprintf(&b, "var %s = %s.%s\n\n", name, pkg, name)
		case token.FUNC:
			fn := d.spec.(*goast.FuncDecl)
			if fn.Type.TypeParams != nil {
				return "", fmt.Errorf("internal error: the generic function %s cannot be re-exported", name)
			}
			var sig bytes.Buffer
			t := *fn.Type
			if err := printer.Fprint(&sig, token.NewFileSet(), &t); err != nil {
				return "", err
			}
			var args []string
			i := 0
			params := &goast.FieldList{}
			for _, f := range fn.Type.Params.List {
				nf := *f
				if len(f.Names) == 0 {
					nf.Names = []*goast.Ident{goast.NewIdent(fmt.Sprintf("a%d", i))}
				}
				for _, n := range nf.Names {
					a := n.Name
					if _, ok := f.Type.(*goast.Ellipsis); ok {
						a += "..."
					}
					args = append(args, a)
				}
				i++
				params.List = append(params.List, &nf)
			}
			ft := *fn.Type
			ft.Params = params
			var sigb bytes.Buffer
			if err := printer.Fprint(&sigb, token.NewFileSet(), &ft); err != nil {
				return "", err
			}
			call := fmt.Sprintf("%s.%s(%s)", pkg, name, strings.Join(args, ", "))
			ret := ""
			if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
				ret = "return "
			}
			fmt.Fprintf(&b, "func %s%s { %s%s }\n\n", name, strings.TrimPrefix(sigb.String(), "func"), ret, call)
		}
	}
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return "", fmt.Errorf("generated API does not compile: %w\n%s", err, b.String())
	}
	return string(src), nil
}
