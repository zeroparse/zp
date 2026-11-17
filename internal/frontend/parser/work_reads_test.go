package parser

import (
	"go/ast"
	"go/build"
	"go/importer"
	goparser "go/parser"
	gotoken "go/token"
	"go/types"
	"os"
	"strings"
	"testing"
)

// countedFields are the fields whose reads are counted as work (see charge): the source of the lexer
// and of the builder, and the tokens of the parser and of the error messages.
var countedFields = map[string]bool{"lexer.src": true, "canonical.src": true, "parser.toks": true, "messages.toks": true}

// workReaders are the functions that may read the counted fields directly: the accessors, which count
// each read, and the loops that count what they read as a whole before they read it.
var workReaders = map[string]bool{
	"lexer.at": true, "lexer.runes": true,
	"canonical.at": true, "canonical.runes": true,
	"parser.at": true, "parser.tok": true, "messages.at": true,
	"messages.lineEndsStatement": true, // counts len(m.toks)
}

// TestWorkReadsGoThroughAccessors checks that every read of a counted field goes through the
// accessors that count it as work, so that a loop that rescans the source or the tokens cannot
// bypass the counters that the scaling tests read. Outside workReaders, a counted field may only be
// the operand of len or be assigned. The package is type-checked, so a field is found through any
// variable, and a copy of it into a local variable is a read too.
func TestWorkReadsGoThroughAccessors(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := gotoken.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := build.Default.MatchFile(".", name); err != nil {
			t.Fatal(err)
		} else if !ok {
			continue
		}
		f, err := goparser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("zeroparse.dev/zp/internal/frontend/parser", fset, files, info); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			reader := workReaders[receiverName(fn)+"."+fn.Name.Name]
			allowed := map[ast.Expr]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.CallExpr:
					if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "len" && len(n.Args) == 1 {
						allowed[n.Args[0]] = true
					}
				case *ast.AssignStmt:
					for _, l := range n.Lhs {
						allowed[l] = true
					}
				case *ast.SelectorExpr:
					field := countedField(info.Selections[n])
					if field == "" {
						break
					}
					seen[field]++
					if !reader && !allowed[n] {
						t.Errorf("%s: %s is read outside its accessor", fset.Position(n.Pos()), field)
					}
				}
				return true
			})
		}
	}
	for field := range countedFields {
		if seen[field] == 0 {
			t.Errorf("%s is never used: the check does not see it", field)
		}
	}
}

// countedField returns the name of the counted field that s selects, or "".
func countedField(s *types.Selection) string {
	if s == nil || s.Kind() != types.FieldVal {
		return ""
	}
	recv := s.Recv()
	if p, ok := recv.(*types.Pointer); ok {
		recv = p.Elem()
	}
	named, ok := recv.(*types.Named)
	if !ok {
		return ""
	}
	name := named.Obj().Name() + "." + s.Obj().Name()
	if !countedFields[name] {
		return ""
	}
	return name
}

// receiverName returns the name of the type of the receiver of fn, or "" for a function.
func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
