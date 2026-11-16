package gogen

import (
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
)

// pruneSrc is a library in miniature: guards of each form, a user type, an implicit method, a type
// only a dropped guard names and an import only that guard uses.
const pruneSrc = `package libgo

import (
	"fmt"
	"strings"
)

type feature uint32

const (
	featureDocument feature = 1 << iota
	featureStream
	featureObserve
	featureBudget

	allFeatures = 1<<iota - 1
)

const usedFeatures = allFeatures

type Parser struct{ n int }

type Document struct{ p *Parser }

// Edit is called by users of a Document.
func (d *Document) Edit() *Span { return nil }

type Span struct{}

func (s *Span) Len() int { return 0 }

func (s *Span) private() {}

type budget struct{}

func (b *budget) spend() { fmt.Println(strings.ToUpper("x")) }

func (p *Parser) Run() {
	if usedFeatures&featureBudget != 0 && p.n > 0 {
		var b budget
		b.spend()
	}
	if p.n == 1 {
		p.n++
	} else if usedFeatures&featureStream != 0 {
		p.n--
	}
	if usedFeatures&featureObserve != 0 {
		p.n = 3
	} else {
		p.n = 4
	}
}

func (p *Parser) unused() {}
`

func TestPruneLib(t *testing.T) {
	files := []libFile{{"lib.go", []byte(pruneSrc)}, {"features.go", []byte("package libgo\n")}}
	// usedFeatures lives in lib.go here; move its declaration to features.go, as in the library.
	files[0].data = []byte(strings.Replace(pruneSrc, usedFeaturesDecl, "", 1))
	files[1].data = []byte("package libgo\n\n" + usedFeaturesDecl)
	roots := [][]byte{[]byte("package p\nvar p rt.Parser\nfunc f() { p.Run() }\n"), []byte("package p\ntype Document = rt.Document\n")}
	out, err := pruneLib(files, codegen.FeatureSet(codegen.FeatureDocument), roots, roots[1:])
	if err != nil {
		t.Fatal(err)
	}
	var lib string
	for _, f := range out {
		if f.name == "lib.go" {
			lib = string(f.data)
		}
	}
	for _, want := range []string{"func (d *Document) Edit()", "type Span", "func (s *Span) Len()", "p.n = 4", "p.n++\n\t}\n"} {
		if !strings.Contains(lib, want) {
			t.Errorf("the pruned library lacks %q:\n%s", want, lib)
		}
	}
	for _, gone := range []string{"budget", "spend", "\"fmt\"", "\"strings\"", "p.n--", "p.n = 3", "private", "unused()"} {
		if strings.Contains(lib, gone) {
			t.Errorf("the pruned library keeps %q:\n%s", gone, lib)
		}
	}
	// Every feature on: the guards stay, and so does what they name.
	all, err := pruneLib(files, codegen.FeatureSet(codegen.AllFeatures), roots, roots[1:])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(all[0].data), "b.spend()") || !strings.Contains(string(all[0].data), "func (b *budget) spend()") {
		t.Errorf("the library with every feature lost a guarded statement:\n%s", all[0].data)
	}
}

// TestPruneLibRejectsUnreadableGuards checks that a use of usedFeatures that is not a conjunct
// usedFeatures&featureX != 0, or a guard whose else holds another, is an error rather than a
// guard left in place or copied unpruned.
func TestPruneLibRejectsUnreadableGuards(t *testing.T) {
	for _, body := range []string{
		"if usedFeatures&featureBudget == 0 { p.n = 1 }",
		"if p.n > 0 || usedFeatures&featureBudget != 0 { p.n = 1 }",
		"if usedFeatures&featureBudget != 0 { p.n = 1 } else if usedFeatures&featureStream != 0 { p.n = 2 }",
	} {
		src := "package libgo\n\nfunc (p *Parser) f() {\n\t" + body + "\n}\n"
		files := []libFile{{"lib.go", []byte(src)}, {"features.go", []byte("package libgo\n\n" + usedFeaturesDecl)}}
		if _, err := pruneLib(files, 0, nil, nil); err == nil {
			t.Errorf("%s: no error", body)
		}
	}
}

// TestImplicitMethodsCoverCore checks that implicitMethods names every method of the interfaces of
// package core and core/impl, which the library's types may satisfy without naming the methods.
func TestImplicitMethodsCoverCore(t *testing.T) {
	for _, dir := range []string{"../../../core", "../../../core/impl"} {
		pkgs, err := goparser.ParseDir(token.NewFileSet(), dir, func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pkgs {
			goast.Inspect(p, func(n goast.Node) bool {
				it, ok := n.(*goast.InterfaceType)
				if !ok {
					return true
				}
				for _, m := range it.Methods.List {
					for _, name := range m.Names {
						if name.IsExported() && !implicitMethods[name.Name] {
							t.Errorf("%s: interface method %s is not in implicitMethods", dir, name.Name)
						}
					}
				}
				return true
			})
		}
	}
}
