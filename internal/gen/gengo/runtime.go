package gengo

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

	"zeroparse.dev/zp/internal/gen"
)

// The Go runtime of generated parsers is the module zeroparse.dev/zp/runtime: the package runtime,
// whose exported names that are documented as for generated parsers the generated code reaches, the
// package parse, which declares the result types (Node, Limits, SyntaxError, ...) that generated
// parsers share with the engine, and the node representation noderep, which both use. A generated
// package either imports the runtime module or carries a copy of the files of its packages (embed
// mode), which the generator reads from the source in GenOptions.RuntimeSource: the runtime package
// at its root, and the others in the subdirectories runtimeSubpackages names.

// runtimeImportPath is the import path of the runtime package in this module.
const runtimeImportPath = "zeroparse.dev/zp/runtime"

// runtimeAlias is the name generated code gives to the runtime package, parseAlias the name it gives
// to the package parse, and typesAlias the name it gives to the types package of a typed parser.
const (
	runtimeAlias = "rt"
	parseAlias   = "parse"
	TypesAlias   = "types"
)

// runtimeSubpackages are the packages of the runtime module besides the runtime package, by their
// directory under the runtime's, which is also their package name. Generated code names the
// declarations of parse (parseAlias); the runtime and parse import noderep.
var runtimeSubpackages = []string{"noderep", parseAlias}

// runtimeCache memoizes the parsed runtime of each source, so that the many generations of one
// process parse it once. The key is the source value of GenOptions.RuntimeSource, used only for a
// source whose dynamic type is a string or a pointer (os.DirFS, an embedded directory): such a value
// is always a valid map key. Any other source is parsed on every call.
var runtimeCache struct {
	mu   sync.Mutex
	info map[fs.FS]*runtimeInfo
}

// runtimeFile is a file of the runtime module: name is its path relative to the runtime's
// directory.
type runtimeFile struct {
	name string
	data []byte
}

// runtimeDecl is an exported top-level declaration of the runtime or of parse.
type runtimeDecl struct {
	kind token.Token // token.TYPE, CONST, VAR or FUNC
	doc  string
	spec goast.Node
	// pkg is the name generated code gives to the package that declares it: runtimeAlias or
	// parseAlias.
	pkg string
}

type runtimeInfo struct {
	files []runtimeFile
	Decls map[string]*runtimeDecl
	// protocol is the name of the constant that declares the newest protocol version the runtime
	// implements (ProtocolN), to which a parser that imports the runtime refers.
	protocol string
}

// loadRuntime parses the runtime source of opts (once per source) and returns its files and
// exported declarations.
func loadRuntime(opts gen.GenOptions) (*runtimeInfo, error) {
	fsys := opts.RuntimeSource
	if fsys == nil {
		return nil, fmt.Errorf("Go generation needs the runtime source: set GenOptions.RuntimeSource")
	}
	k := reflect.TypeOf(fsys).Kind()
	cacheable := k == reflect.String || k == reflect.Pointer
	runtimeCache.mu.Lock()
	defer runtimeCache.mu.Unlock()
	if cacheable {
		if info, ok := runtimeCache.info[fsys]; ok {
			return info, nil
		}
	}
	info, err := parseRuntime(fsys)
	if err != nil {
		return nil, err
	}
	if cacheable {
		if runtimeCache.info == nil {
			runtimeCache.info = map[fs.FS]*runtimeInfo{}
		}
		runtimeCache.info[fsys] = info
	}
	return info, nil
}

// parseRuntime reads the files of the runtime package in fsys, and of the packages of
// runtimeSubpackages in its subdirectories, and the exported declarations of the runtime and
// parse.
func parseRuntime(fsys fs.FS) (*runtimeInfo, error) {
	info := &runtimeInfo{Decls: map[string]*runtimeDecl{}}
	fset := token.NewFileSet()
	for _, dir := range append([]string{"."}, runtimeSubpackages...) {
		entries, err := fs.ReadDir(fsys, dir)
		if err != nil {
			return nil, err
		}
		pkg := runtimeAlias
		if dir != "." {
			pkg = dir
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			name = path.Join(dir, name)
			data, err := fs.ReadFile(fsys, name)
			if err != nil {
				return nil, err
			}
			info.files = append(info.files, runtimeFile{name, data})
			if pkg != runtimeAlias && pkg != parseAlias {
				continue
			}
			f, err := goparser.ParseFile(fset, name, data, goparser.ParseComments|goparser.SkipObjectResolution)
			if err != nil {
				return nil, err
			}
			if err := addRuntimeDecls(info, f, pkg); err != nil {
				return nil, err
			}
		}
	}
	if len(info.files) == 0 {
		return nil, fmt.Errorf("the runtime source has no files")
	}
	best := 0
	for name, d := range info.Decls {
		if n, ok := strings.CutPrefix(name, "Protocol"); ok && d.kind == token.CONST && d.pkg == runtimeAlias {
			if v, err := strconv.Atoi(n); err == nil && v > best {
				best, info.protocol = v, name
			}
		}
	}
	if info.protocol == "" {
		return nil, fmt.Errorf("the runtime source declares no protocol version (a constant ProtocolN)")
	}
	sort.Slice(info.files, func(i, j int) bool { return info.files[i].name < info.files[j].name })
	return info, nil
}

// addRuntimeDecls adds the exported top-level declarations of f, a file of the package that
// generated code names pkg, to info. A name that both the runtime and parse declare is an error:
// generated code could not tell which it names.
func addRuntimeDecls(info *runtimeInfo, f *goast.File, pkg string) error {
	add := func(name string, d *runtimeDecl) error {
		if prev, ok := info.Decls[name]; ok && prev.pkg != pkg {
			return fmt.Errorf("the runtime source declares %s in both %s and %s", name, prev.pkg, pkg)
		}
		d.pkg = pkg
		info.Decls[name] = d
		return nil
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *goast.FuncDecl:
			if d.Recv == nil && d.Name.IsExported() {
				if err := add(d.Name.Name, &runtimeDecl{kind: token.FUNC, doc: d.Doc.Text(), spec: d}); err != nil {
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
						if err := add(s.Name.Name, &runtimeDecl{kind: token.TYPE, doc: doc.Text(), spec: s}); err != nil {
							return err
						}
					}
				case *goast.ValueSpec:
					if s.Doc != nil {
						doc = s.Doc
					}
					for _, n := range s.Names {
						if n.IsExported() {
							if err := add(n.Name, &runtimeDecl{kind: d.Tok, doc: doc.Text(), spec: s}); err != nil {
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

// runtimeFiles returns the copy of the runtime module that RuntimeEmbed writes: the runtime package
// in internal/runtime, and the other packages in its subdirectories, whose imports of each other
// name their copies.
func runtimeFiles(opts gen.GenOptions) ([]gen.GeneratedFile, error) {
	info, err := loadRuntime(opts)
	if err != nil {
		return nil, err
	}
	base := runtimeImport(opts)
	var paths []string
	for _, sub := range runtimeSubpackages {
		paths = append(paths, strconv.Quote(runtimeImportPath+"/"+sub), strconv.Quote(base+"/"+sub))
	}
	imports := strings.NewReplacer(paths...)
	var out []gen.GeneratedFile
	for _, f := range info.files {
		data := imports.Replace(string(f.data))
		out = append(out, gen.GeneratedFile{Path: path.Join("internal", "runtime", f.name), Content: []byte(gen.Header() + "\n" + data)})
	}
	return out, nil
}

// runtimeImport returns the import path of the runtime for a generated package and for the copy of
// the runtime that embed and only write.
func runtimeImport(opts gen.GenOptions) string {
	if opts.Runtime == gen.RuntimeEmbed || opts.Runtime == gen.RuntimeOnly {
		return path.Join(opts.Module, "internal", "runtime")
	}
	return runtimeImportPath
}

// parseImport returns the import path of the package parse for a generated package.
func parseImport(opts gen.GenOptions) string { return runtimeImport(opts) + "/" + parseAlias }

// stdImports are the standard-library packages generated code may use, by the name it uses them
// with.
var stdImports = map[string]string{
	"atomic": "sync/atomic", "bufio": "bufio", "bytes": "bytes", "context": "context", "errors": "errors",
	"fmt": "fmt", "io": "io", "iter": "iter", "math": "math", "slices": "slices", "sort": "sort",
	"strconv": "strconv", "strings": "strings", "sync": "sync", "unicode": "unicode", "unsafe": "unsafe",
	"utf8": "unicode/utf8",
}

// typesPackage describes the types package of a typed parser for qualifyRuntime.
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

// qualifyRuntime finishes the generated source body of a package. Every identifier that the body
// leaves unresolved and that is an exported name of the runtime or of parse is that package's, and
// is qualified with it; one that the types package tp (if any) declares is qualified with the types
// package. The standard-library packages the body uses are imported, the runtime, whose import path
// is rtImport, parse (the subdirectory parse of the runtime's) if the body names it, and the types
// package.
func qualifyRuntime(rtImport, body string, info *runtimeInfo, tp *typesPackage) (string, error) {
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
	qualify := func(id *goast.Ident, d *runtimeDecl) {
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
	// body and of the runtime, and for a literal with its type elided, from the enclosing one.
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
	parseUsed := false
	for _, in := range inserts {
		parseUsed = parseUsed || in.qualifier == parseAlias+"."
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
	out.Grow(len(body) + len(inserts)*(len(runtimeAlias)+1))
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
	fmt.Fprintf(&imports, "\n\t%s %q\n", runtimeAlias, rtImport)
	if parseUsed {
		fmt.Fprintf(&imports, "\t%q\n", rtImport+"/"+parseAlias)
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
// through the runtime's.
func literalType(typ goast.Expr, types map[string]goast.Expr, info *runtimeInfo, tp *typesPackage) (kind byte, key, elem goast.Expr) {
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

// apiNames returns the runtime names that the generated package re-exports for its users, in
// declaration order by group.
func apiNames(opts gen.GenOptions) []string {
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
// the runtime and of parse that apiNames lists. Types, constants and variables are aliases;
// functions are wrappers that the compiler inlines.
func apiSource(opts gen.GenOptions, info *runtimeInfo) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\npackage %s\n\n", gen.Header(), opts.Package)
	b.WriteString("import (\n\t\"context\"\n\n")
	fmt.Fprintf(&b, "\t%q\n", parseImport(opts))
	fmt.Fprintf(&b, "\t%s %q\n)\n\n", runtimeAlias, runtimeImport(opts))
	b.WriteString("var _ context.Context\n\n")
	for _, name := range apiNames(opts) {
		d := info.Decls[name]
		if d == nil {
			return "", fmt.Errorf("internal error: the runtime has no %s", name)
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
