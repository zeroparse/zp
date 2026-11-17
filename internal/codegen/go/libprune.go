package gogen

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/format"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"

	"zeroparse.dev/zp/internal/codegen"
)

// The copy of the library that a parser carries (LibEmbed) holds only what the parser can run. The
// library names each feature's code with a guard on the constant usedFeatures (features.go), which
// the checked-in library sets to every feature. The copy sets it to the features of the parser
// (codegen.ParserFeatures), drops the statements whose guard is then false, and then drops the
// declarations that nothing kept names. Neither step decides a feature: the guards follow the one
// analysis, and the second step follows only names.
//
// A guard is an if statement whose condition is a conjunction with a conjunct of the form
// usedFeatures&featureX != 0. Such a statement is replaced by its else branch when X is not a
// feature of the parser. The library writes guards so that the statement can go: a variable that
// only the guarded statement reads is declared inside it.
//
// A declaration is kept when kept text names it, the text of the parser included: names are
// lexical, as the Go scanner reads identifiers (comments and strings do not keep anything), so a
// declaration that the compiler needs is always kept. A method is kept with its receiver type when
// it is exported (a user or an interface of the standard library may call it) or kept text names
// it. A const block is one unit, since dropping a spec would change iota.

// usedFeaturesDecl is the declaration of the features of the checked-in library, which the copy
// replaces with those of its parser.
const usedFeaturesDecl = "const usedFeatures = allFeatures\n"

// libUnit is a top-level declaration of the library.
type libUnit struct {
	file       int
	start, end int // byte range in the file, doc comment included
	names      []string
	recv       string // the receiver type of a method
	exported   bool   // a method with an exported name
	sig        string // the signature of a method
	isType     bool   // a type declaration
	surface    string // what a user of a type declaration sees: all but its unexported fields
	root       bool   // init, blank declarations
	text       string
}

// pruneLib returns the files of the library for a parser with the features fs whose own files are
// roots, of which api are those that its users call (shakeLib).
func pruneLib(files []libFile, fs codegen.FeatureSet, roots, api [][]byte) ([]libFile, error) {
	out := make([]libFile, len(files))
	copy(out, files)
	found := false
	for i, f := range out {
		if f.name != "features.go" {
			continue
		}
		if !bytes.Contains(f.data, []byte(usedFeaturesDecl)) {
			return nil, fmt.Errorf("the library file features.go lacks %q", strings.TrimSpace(usedFeaturesDecl))
		}
		decl := fmt.Sprintf("const usedFeatures feature = %d // %s\n", uint32(fs), fs)
		out[i].data = bytes.Replace(f.data, []byte(usedFeaturesDecl), []byte(decl), 1)
		found = true
	}
	if !found {
		return nil, fmt.Errorf("the library source has no features.go")
	}
	for i := range out {
		data, err := dropGuarded(out[i], fs)
		if err != nil {
			return nil, err
		}
		out[i].data = data
	}
	return shakeLib(out, roots, api)
}

// featureBits maps the names of the library's feature constants to their bits.
func featureBits() map[string]codegen.Feature {
	m := map[string]codegen.Feature{}
	for i, n := range codegen.FeatureNames() {
		m["feature"+strings.ToUpper(n[:1])+n[1:]] = codegen.Feature(1) << i
	}
	return m
}

// dropGuarded returns the file without the statements whose guard is false for fs.
func dropGuarded(f libFile, fs codegen.FeatureSet) ([]byte, error) {
	if !bytes.Contains(f.data, []byte("usedFeatures&")) {
		return f.data, nil
	}
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, f.name, f.data, goparser.ParseComments|goparser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	bits := featureBits()
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	var guardErr error
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	// parentOf maps an if statement that is the else branch of another to that other.
	parentOf := map[*goast.IfStmt]*goast.IfStmt{}
	goast.Inspect(file, func(n goast.Node) bool {
		if s, ok := n.(*goast.IfStmt); ok {
			if e, ok := s.Else.(*goast.IfStmt); ok {
				parentOf[e] = s
			}
		}
		return true
	})
	goast.Inspect(file, func(n goast.Node) bool {
		s, ok := n.(*goast.IfStmt)
		if !ok {
			return true
		}
		on, isGuard, err := guardValue(s.Cond, bits, fs)
		if err != nil && guardErr == nil {
			guardErr = fmt.Errorf("%s: %w", fset.Position(s.Pos()), err)
		}
		if !isGuard || on {
			return true
		}
		if s.Init != nil && guardErr == nil {
			guardErr = fmt.Errorf("%s: a guard has an init statement", fset.Position(s.Pos()))
		}
		if s.Else != nil && bytes.Contains(f.data[off(s.Else.Pos()):off(s.Else.End())], []byte("usedFeatures")) && guardErr == nil {
			// The else replaces the statement as it is, so it must hold no guard of its own.
			guardErr = fmt.Errorf("%s: the else of a guard holds a guard", fset.Position(s.Pos()))
		}
		switch parent := parentOf[s]; {
		case s.Else != nil:
			edits = append(edits, edit{off(s.Pos()), off(s.End()), string(f.data[off(s.Else.Pos()):off(s.Else.End())])})
		case parent != nil:
			// The last branch of an if-else chain goes with its else.
			edits = append(edits, edit{off(parent.Body.End()), off(s.End()), ""})
		default:
			edits = append(edits, edit{off(s.Pos()), off(s.End()), ""})
		}
		return false
	})
	if guardErr != nil {
		return nil, guardErr
	}
	data := f.data
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		data = append(append(append([]byte(nil), data[:e.start]...), e.text...), data[e.end:]...)
	}
	return data, nil
}

// guardValue reports whether cond is a guard and, if so, whether it can be true for fs.
func guardValue(cond goast.Expr, bits map[string]codegen.Feature, fs codegen.FeatureSet) (on, isGuard bool, err error) {
	on = true
	for _, c := range conjuncts(cond) {
		f, ok, err := featureTest(c, bits)
		if err != nil {
			return false, false, err
		}
		if ok {
			isGuard = true
			if !fs.Has(f) {
				on = false
			}
		}
	}
	return on, isGuard, nil
}

func conjuncts(e goast.Expr) []goast.Expr {
	if p, ok := e.(*goast.ParenExpr); ok {
		return conjuncts(p.X)
	}
	if b, ok := e.(*goast.BinaryExpr); ok && b.Op == token.LAND {
		return append(conjuncts(b.X), conjuncts(b.Y)...)
	}
	return []goast.Expr{e}
}

// featureTest reports whether e is usedFeatures&featureX != 0, and X. Any other use of
// usedFeatures in a condition is an error, so that a guard the copy cannot read does not pass
// unnoticed.
func featureTest(e goast.Expr, bits map[string]codegen.Feature) (codegen.Feature, bool, error) {
	if !mentions(e, "usedFeatures") {
		return 0, false, nil
	}
	b, ok := e.(*goast.BinaryExpr)
	if ok && b.Op == token.NEQ {
		if and, ok := b.X.(*goast.BinaryExpr); ok && and.Op == token.AND {
			x, xok := and.X.(*goast.Ident)
			y, yok := and.Y.(*goast.Ident)
			z, zok := b.Y.(*goast.BasicLit)
			if xok && yok && zok && x.Name == "usedFeatures" && z.Value == "0" {
				if f, ok := bits[y.Name]; ok {
					return f, true, nil
				}
			}
		}
	}
	return 0, false, fmt.Errorf("a condition uses usedFeatures other than as a conjunct usedFeatures&featureX != 0")
}

func mentions(e goast.Expr, name string) bool {
	found := false
	goast.Inspect(e, func(n goast.Node) bool {
		if id, ok := n.(*goast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

// identifiers returns the identifiers of src as the Go scanner reads them.
func identifiers(src []byte, add func(string)) {
	var s scanner.Scanner
	fset := token.NewFileSet()
	s.Init(fset.AddFile("", -1, len(src)), src, nil, 0)
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return
		}
		if tok == token.IDENT {
			add(lit)
		}
	}
}

// shakeLib drops the top-level declarations of files that nothing kept names, then the imports
// that the rest no longer uses, and the files left without declarations. roots are the files of the
// parser, api the parts of them that its users call (api.go, types.go and the userSurface of
// parser.go): the exported methods of a library type that api names, or that the declaration or the
// exported methods of such a type name, are kept, since a user can call them without naming them in
// the parser.
func shakeLib(files []libFile, roots, api [][]byte) ([]libFile, error) {
	fset := token.NewFileSet()
	var units []*libUnit
	for i, f := range files {
		file, err := goparser.ParseFile(fset, f.name, f.data, goparser.ParseComments|goparser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		off := func(p token.Pos) int { return fset.Position(p).Offset }
		for _, d := range file.Decls {
			u := &libUnit{file: i}
			start := d.Pos()
			switch d := d.(type) {
			case *goast.FuncDecl:
				if d.Doc != nil {
					start = d.Doc.Pos()
				}
				u.names = []string{d.Name.Name}
				if d.Recv != nil {
					u.recv = recvName(d.Recv.List[0].Type)
					u.exported = d.Name.IsExported()
					u.sig = string(f.data[off(d.Type.Pos()):off(d.Type.End())])
				} else if d.Name.Name == "init" {
					u.root = true
				}
			case *goast.GenDecl:
				if d.Tok == token.IMPORT {
					continue
				}
				if d.Doc != nil {
					start = d.Doc.Pos()
				}
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *goast.TypeSpec:
						u.names = append(u.names, s.Name.Name)
						u.isType = true
						u.surface += typeSurface(f.data, off, s)
					case *goast.ValueSpec:
						for _, n := range s.Names {
							u.names = append(u.names, n.Name)
						}
					}
				}
				if slices.Contains(u.names, "_") {
					u.root = true
				}
			}
			u.start, u.end = off(start), off(d.End())
			u.text = string(f.data[u.start:u.end])
			units = append(units, u)
		}
	}
	byName := map[string][]*libUnit{}
	isType := map[string]bool{}
	methods := map[string][]*libUnit{}      // by receiver type
	methodsNamed := map[string][]*libUnit{} // by method name
	for _, u := range units {
		if u.recv != "" {
			methods[u.recv] = append(methods[u.recv], u)
			methodsNamed[u.names[0]] = append(methodsNamed[u.names[0]], u)
			continue
		}
		for _, n := range u.names {
			byName[n] = append(byName[n], u)
			isType[n] = isType[n] || u.isType
		}
	}
	kept := map[*libUnit]bool{}
	keptType := map[string]bool{}
	user := map[string]bool{}
	named := map[string]bool{}
	var queue []*libUnit
	keep := func(u *libUnit) {
		if !kept[u] {
			kept[u] = true
			queue = append(queue, u)
		}
	}
	// keepMethods keeps the methods of the kept type t that a caller can reach.
	keepMethods := func(t string) {
		for _, m := range methods[t] {
			n := m.names[0]
			if named[n] || m.exported && (user[t] || implicitMethods[n]) {
				keep(m)
			}
		}
	}
	var markUser func(string)
	markUser = func(t string) {
		if !isType[t] || user[t] || !goast.IsExported(t) {
			return
		}
		user[t] = true
		if keptType[t] {
			for _, u := range byName[t] {
				identifiers([]byte(u.surface), markUser)
			}
			keepMethods(t)
		}
	}
	name := func(n string) {
		if named[n] {
			return
		}
		named[n] = true
		for _, u := range byName[n] {
			keep(u)
		}
		for _, m := range methodsNamed[n] {
			if keptType[m.recv] {
				keep(m)
			}
		}
	}
	for _, r := range roots {
		identifiers(r, name)
	}
	for _, r := range api {
		identifiers(r, markUser)
	}
	for _, u := range units {
		if u.root {
			keep(u)
		}
	}
	for len(queue) > 0 {
		u := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		identifiers([]byte(u.text), name)
		switch {
		case u.recv != "":
			if u.exported && user[u.recv] {
				identifiers([]byte(u.sig), markUser)
			}
		case u.isType:
			for _, t := range u.names {
				keptType[t] = true
				if user[t] {
					identifiers([]byte(u.surface), markUser)
				}
				keepMethods(t)
			}
		}
	}
	out := make([]libFile, 0, len(files))
	for i, f := range files {
		var b bytes.Buffer
		last := 0
		any := false
		for _, u := range units {
			if u.file != i {
				continue
			}
			if kept[u] {
				any = true
				continue
			}
			b.Write(f.data[last:u.start])
			last = u.end
		}
		if !any {
			continue
		}
		b.Write(f.data[last:])
		data, err := dropUnusedImports(f.name, b.Bytes())
		if err != nil {
			return nil, err
		}
		out = append(out, libFile{f.name, data})
	}
	return out, nil
}

// userSurface returns the parts of src, a generated Go file of the parser's package, that a user of
// the package reaches without naming the library: the signatures of its exported functions and of
// the exported methods of its exported types, the surfaces of its exported types (typeSurface), and
// the declarations of its exported constants and variables. A type of the parser that embeds a
// library type, such as the typed Document, thus makes the exported methods of that type user API.
func userSurface(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "parser.go", src, goparser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	var b bytes.Buffer
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *goast.FuncDecl:
			if !d.Name.IsExported() || d.Recv != nil && !goast.IsExported(recvName(d.Recv.List[0].Type)) {
				continue
			}
			b.Write(src[off(d.Type.Pos()):off(d.Type.End())])
			b.WriteByte('\n')
		case *goast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *goast.TypeSpec:
					if s.Name.IsExported() {
						b.WriteString(typeSurface(src, off, s))
						b.WriteByte('\n')
					}
				case *goast.ValueSpec:
					if slices.ContainsFunc(s.Names, (*goast.Ident).IsExported) {
						b.Write(src[off(s.Pos()):off(s.End())])
						b.WriteByte('\n')
					}
				}
			}
		}
	}
	return b.Bytes(), nil
}

// typeSurface returns the text of the type of s that a user of the type can reach: all of it but
// the unexported fields of a struct.
func typeSurface(src []byte, off func(token.Pos) int, s *goast.TypeSpec) string {
	st, ok := s.Type.(*goast.StructType)
	if !ok {
		return string(src[off(s.Type.Pos()):off(s.Type.End())])
	}
	var b strings.Builder
	for _, f := range st.Fields.List {
		exported := len(f.Names) == 0 // an embedded field
		for _, n := range f.Names {
			exported = exported || n.IsExported()
		}
		if exported {
			b.Write(src[off(f.Type.Pos()):off(f.Type.End())])
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// implicitMethods are the names of the methods that code outside the library can call through an
// interface without naming them in the library or the parser: those of the interfaces of the
// standard library that the library's types satisfy, and those of the interfaces of package core
// (TestImplicitMethodsCoverCore).
var implicitMethods = map[string]bool{
	"Error": true, "String": true, "GoString": true, "Format": true, "Unwrap": true, "Is": true, "As": true,
	"MarshalJSON": true, "UnmarshalJSON": true, "MarshalText": true, "UnmarshalText": true,
	"Len": true, "Less": true, "Swap": true, "Push": true, "Pop": true, "Read": true, "Write": true, "Close": true,
	"LineCol": true, "Text": true, "Failure": true, "Recovered": true,
}

func recvName(e goast.Expr) string {
	switch e := e.(type) {
	case *goast.StarExpr:
		return recvName(e.X)
	case *goast.IndexExpr:
		return recvName(e.X)
	case *goast.IndexListExpr:
		return recvName(e.X)
	case *goast.Ident:
		return e.Name
	}
	return ""
}

// importNames are the package names of the imports of the library whose last path element is not
// their name.
var importNames = map[string]string{"zeroparse.dev/zp/core/impl": "coreimpl"}

// dropUnusedImports returns src without the imports whose package it no longer names.
func dropUnusedImports(name string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, name, src, goparser.ParseComments|goparser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	goast.Inspect(file, func(n goast.Node) bool {
		if s, ok := n.(*goast.SelectorExpr); ok {
			if id, ok := s.X.(*goast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	type cut struct{ start, end int }
	var cuts []cut
	single := map[*goast.ImportSpec]*goast.GenDecl{}
	for _, d := range file.Decls {
		if g, ok := d.(*goast.GenDecl); ok && g.Tok == token.IMPORT && !g.Lparen.IsValid() {
			single[g.Specs[0].(*goast.ImportSpec)] = g
		}
	}
	for _, im := range file.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		n := path.Base(p)
		if alt, ok := importNames[p]; ok {
			n = alt
		}
		if im.Name != nil {
			if im.Name.Name == "_" || im.Name.Name == "." {
				continue
			}
			n = im.Name.Name
		}
		if !used[n] {
			var node goast.Node = im
			if g, ok := single[im]; ok {
				node = g
			}
			cuts = append(cuts, cut{fset.Position(node.Pos()).Offset, fset.Position(node.End()).Offset})
		}
	}
	for i := len(cuts) - 1; i >= 0; i-- {
		c := cuts[i]
		src = append(append([]byte(nil), src[:c.start]...), src[c.end:]...)
	}
	return format.Source(src)
}
