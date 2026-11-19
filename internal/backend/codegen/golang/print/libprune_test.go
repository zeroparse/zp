package print

import (
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
)

// pruneSrc is a library in miniature: guards of each form, a user type, an implicit method, a type
// only a dropped guard names and an import only that guard uses.
const pruneSrc = `package golib

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
	files := []libFile{{"lib.go", []byte(pruneSrc)}, {"features.go", []byte("package golib\n")}}
	// usedFeatures lives in lib.go here; move its declaration to features.go, as in the library.
	files[0].data = []byte(strings.Replace(pruneSrc, usedFeaturesDecl, "", 1))
	files[1].data = []byte("package golib\n\n" + usedFeaturesDecl)
	roots := [][]byte{[]byte("package p\nvar p rt.Parser\nfunc f() { p.Run() }\n"), []byte("package p\ntype Document = rt.Document\n")}
	out, err := pruneLib(files, common.FeatureSet(common.FeatureDocument), roots, roots[1:])
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
	all, err := pruneLib(files, common.FeatureSet(common.AllFeatures), roots, roots[1:])
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
		src := "package golib\n\nfunc (p *Parser) f() {\n\t" + body + "\n}\n"
		files := []libFile{{"lib.go", []byte(src)}, {"features.go", []byte("package golib\n\n" + usedFeaturesDecl)}}
		if _, err := pruneLib(files, 0, nil, nil); err == nil {
			t.Errorf("%s: no error", body)
		}
	}
}

// TestImplicitMethodsCoverCore checks that implicitMethods names every method of the interfaces of
// the runtime packages and value/impl, which the library's types may satisfy without naming the methods.
// runtime.Parser and runtime.Document are left out: the VM and the reference parser implement
// them, and neither the library nor generated code does.
func TestImplicitMethodsCoverCore(t *testing.T) {
	notLibrary := map[string]bool{"Parser": true, "Document": true}
	for _, dir := range []string{"../../../../../pkg/runtime", "../../../../../pkg/runtime/input", "../../../../../pkg/runtime/value", "../../../../../pkg/runtime/value/impl", "../../../../../pkg/runtime/instrument"} {
		pkgs, err := goparser.ParseDir(token.NewFileSet(), dir, func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pkgs {
			goast.Inspect(p, func(n goast.Node) bool {
				ts, ok := n.(*goast.TypeSpec)
				if !ok {
					return true
				}
				it, ok := ts.Type.(*goast.InterfaceType)
				if !ok || p.Name == "runtime" && notLibrary[ts.Name.Name] {
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

// dropUnusedImports cuts whole lines and leaves at most one blank line, between two groups, in the
// import blocks that it shortens: a cut that leaves indentation, a trailing comment or the blank
// line of an emptied group behind splits or pads the import group (the import rule of
// docs/development.md, "Go Style").
func TestDropUnusedImportsKeepsGroups(t *testing.T) {
	t.Parallel()
	const body = "\nvar _, _ = fmt.Sprint, strings.ToUpper\n" // the source never names os
	for _, c := range []struct{ name, src, want string }{
		{"indented import in a group", "import (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n", "import (\n\t\"fmt\"\n\t\"strings\"\n)\n"},
		{"first group emptied", "import (\n\t\"os\"\n\n\t\"fmt\"\n)\n", "import (\n\t\"fmt\"\n)\n"},
		{"last group emptied", "import (\n\t\"fmt\"\n\n\t\"os\"\n)\n", "import (\n\t\"fmt\"\n)\n"},
		{"middle group emptied", "import (\n\t\"fmt\"\n\n\t\"os\"\n\n\t\"strings\"\n)\n", "import (\n\t\"fmt\"\n\n\t\"strings\"\n)\n"},
		{"trailing comment goes with the import", "import (\n\t\"fmt\"\n\t\"os\" // for the tests\n\t\"strings\"\n)\n", "import (\n\t\"fmt\"\n\t\"strings\"\n)\n"},
		{"trailing comment of an emptied group", "import (\n\t\"os\" // for the tests\n\n\t\"fmt\"\n)\n", "import (\n\t\"fmt\"\n)\n"},
		{"single import", "import \"os\"\nimport \"fmt\"\nimport \"strings\"\n", "import \"fmt\"\nimport \"strings\"\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := dropUnusedImports("x.go", []byte("package x\n\n"+c.src+body))
			if err != nil {
				t.Fatal(err)
			}
			want := "package x\n\n" + c.want + body
			if string(got) != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// Only the import declarations are rewritten: a line "import (" in a raw string, and the blank lines
// of a function or of the raw string, stay as they are.
func TestDropUnusedImportsLeavesOtherText(t *testing.T) {
	t.Parallel()
	src := "package x\n\nimport (\n\t\"fmt\"\n)\n\nvar text = `\nimport (\n\n\n)\n`\n\nvar _ = fmt.Sprint\n"
	got, err := dropUnusedImports("x.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Errorf("got:\n%s\nwant it unchanged", got)
	}
}
