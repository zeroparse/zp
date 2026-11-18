package print_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/internal/testing/codegentest"
	zprt "zeroparse.dev/zp/pkg/runtime/golib"
	"zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/value"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// Exercise the exported APIs against the same hand-built DAG, including nil,
// nested lists and a shared value reached through different owning parents.
func TestTreeLibParity(t *testing.T) {
	t.Parallel()
	leaf := nodes.Ptr(nodes.Data{Kind: valueimpl.MatchKind, Terminal: true, Text: "é<&>", Start: 1, End: 3})
	gLeaf := zprt.NewNode(value.KindString, 1, 3, nodes.D(leaf).Text, nil, "", nil, nil)
	child := nodes.Ptr(nodes.Data{Children: []*value.Node{leaf}, Fields: valueimpl.Fields{{Name: "list", Value: []any{leaf, nil, []byte{1, 2}}}}})
	gChild := zprt.NewNode(value.KindStruct, 0, 0, "", []*value.Node{gLeaf}, "", valueimpl.Fields{{Name: "list", Value: []any{gLeaf, nil, []byte{1, 2}}}}, nil)
	root := nodes.Ptr(nodes.Data{Children: []*value.Node{child, nil, leaf}, Fields: valueimpl.Fields{{Name: "z", Value: true}, {Name: "a", Value: leaf}}})
	gRoot := zprt.NewNode(value.KindStruct, 0, 0, "", []*value.Node{gChild, nil, gLeaf}, "", valueimpl.Fields{{Name: "z", Value: true}, {Name: "a", Value: gLeaf}}, nil)
	core, err := root.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	generated, err := gRoot.MarshalJSON()
	if err != nil || !bytes.Equal(core, generated) {
		t.Fatalf("JSON parity: %s != %s, %v", core, generated, err)
	}
	var a, b []string
	describe := func(depth int, kind uint8, child int, field string, list int, nodeText, parentText string) string {
		return fmt.Sprintf("%d/%d/%d/%s/%d/%s/%s", depth, kind, child, field, list, nodeText, parentText)
	}
	ca := func(v value.WalkVisit) (value.WalkAction, error) {
		n, p := "", ""
		if v.Node != nil {
			n = nodes.D(v.Node).Text
		}
		if v.Parent != nil {
			p = nodes.D(v.Parent).Text
		}
		a = append(a, describe(v.Depth, uint8(v.Edge.Kind), v.Edge.ChildIndex, v.Edge.FieldName, v.Edge.ListIndex, n, p))
		return value.WalkContinue, nil
	}
	cb := func(v value.WalkVisit) (value.WalkAction, error) {
		n, p := "", ""
		if v.Node != nil {
			n = v.Node.Text()
		}
		if v.Parent != nil {
			p = v.Parent.Text()
		}
		b = append(b, describe(v.Depth, uint8(v.Edge.Kind), v.Edge.ChildIndex, v.Edge.FieldName, v.Edge.ListIndex, n, p))
		return value.WalkContinue, nil
	}
	if err := value.Walk(root, ca); err != nil {
		t.Fatal(err)
	}
	if err := value.Walk(gRoot, cb); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("walk parity: %v != %v", a, b)
	}
	for _, unit := range []input.PositionUnit{input.CodePoints, input.Bytes} {
		for _, span := range [][2]int32{{0, 0}, {1, 2}, {3, 4}} {
			a, b = nil, nil
			if err := (value.Cursor{Root: root, Unit: unit}).Range(span[0], span[1], ca); err != nil {
				t.Fatal(err)
			}
			if err := (value.Cursor{Root: gRoot, Unit: input.PositionUnit(unit)}).Range(span[0], span[1], cb); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("cursor parity: %v != %v", a, b)
			}
		}
	}
	nodes.D(root).Children = []*value.Node{root}
	zprt.SetNodeElems(gRoot, []*value.Node{gRoot})
	var c *value.CycleError
	var gc *value.CycleError
	if !errors.As(value.Walk(root, ca), &c) || !errors.As(value.Walk(gRoot, cb), &gc) {
		t.Fatal("missing cycle errors")
	}
	if len(c.Path) != len(gc.Path) || uint8(c.Path[1].Kind) != uint8(gc.Path[1].Kind) {
		t.Fatal("cycle path parity")
	}
}

// Compile the emitted single-file parser and exercise the same depth, cycle,
// encoding and cursor regressions used by the core. This catches imports and
// typed-name collisions that direct library package tests cannot establish.
func TestGeneratedTreeTraversal(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTreeTraversal)
}

func testGeneratedTreeTraversal(t *testing.T) {
	if testing.Short() {
		t.Skip("runs generated code")
	}
	var source strings.Builder
	names := []string{"Walk", "WalkAction", "WalkContinue", "WalkSkip", "WalkStop", "WalkVisit", "WalkEdge", "WalkEdgeKind", "WalkRoot", "WalkChild", "WalkField", "WalkListElement", "CycleError", "Cursor"}
	for _, name := range names {
		fmt.Fprintf(&source, "type %s = { Value int }\n", name)
	}
	source.WriteString("rule main = \"é\"\n")
	grammar, err := parseTest(source.String())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	codegentest.GeneratePackage(t, dir, "tree", grammar, common.GenOptions{Start: "main", Types: true})
	typesSource, err := os.ReadFile(filepath.Join(dir, "tree", "types", "types.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !strings.Contains(string(typesSource), "type "+name+" struct") {
			t.Fatalf("grammar type %s is not declared under its name", name)
		}
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte(codegentest.GoMod("codegentest")))
	// The traversal itself is the library's and is tested there; this checks that the generated
	// package declares the colliding grammar types beside the library's names and walks a tree.
	var consumer strings.Builder
	consumer.WriteString("package tree\n\nimport (\n\t\"testing\"\n\n\t\"codegentest/tree/types\"\n\tzprt \"zeroparse.dev/zp/pkg/runtime/golib\"\n\tzpvalue \"zeroparse.dev/zp/pkg/runtime/value\"\n)\n\nfunc TestWalk(t *testing.T) {\n")
	for _, name := range names {
		fmt.Fprintf(&consumer, "\t_ = types.%s{Value: 1}\n", name)
	}
	consumer.WriteString(`	leaf := zprt.NewNode(zpvalue.KindString, 0, 2, "é", nil, "", nil, nil)
	root := zprt.NewNode(zpvalue.KindTuple, 0, 2, "", []*zpvalue.Node{leaf}, "", nil, nil)
	visits := 0
	if err := zpvalue.Walk(root, func(v zpvalue.WalkVisit) (zpvalue.WalkAction, error) {
		visits++
		return zpvalue.WalkContinue, nil
	}); err != nil || visits != 2 {
		t.Fatalf("visits=%d err=%v", visits, err)
	}
}
`)
	write("tree/tree_test.go", []byte(consumer.String()))
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated tree checks: %v\n%s", err, out)
	}
}

// DuckDB supplies Inspect(any, func(any) bool) for its typed AST. Compile its actual
// helper beside a fresh generated parser and exercise both traversal surfaces.
func TestGeneratedDuckDBInspectCompatibility(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedDuckDBInspectCompatibility)
}

func testGeneratedDuckDBInspectCompatibility(t *testing.T) {
	grammar, err := parseTest("type Leaf = { Text string }\nrule main Leaf = t:(\"é\") #atomic => Leaf{Text: text($t)}\n")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "testdata", "grammars", "duckdb", "inspect.go"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte(codegentest.GoMod("codegentest")))
	codegentest.GeneratePackage(t, dir, "duckdb", grammar, common.GenOptions{Start: "main", Types: true})
	write("duckdb/inspect.go", helper)
	write("duckdb/walk_test.go", []byte(`package duckdb
import (
	"testing"
)
func TestBothWalkAPIs(t *testing.T) {
 node, err := Parse("é")
 if err != nil { t.Fatal(err) }
 generic := 0
 // A text struct node and its Text field value.
 if err := Walk(node,func(v WalkVisit)(WalkAction,error){generic++;return WalkContinue,nil});err!=nil||generic!=2 {t.Fatal(generic,err)}
 generic=0
 if err := (Cursor{Root:node,Unit:CodePoints}).At(0,func(v WalkVisit)(WalkAction,error){generic++;return WalkContinue,nil});err!=nil||generic!=1 {t.Fatal(generic,err)}
 typed, err := ParseValue("é")
 if err != nil { t.Fatal(err) }
 count:=0
 Inspect(typed,func(any)bool{count++;return true})
 if count!=1 {t.Fatalf("typed Inspect visits %d, want 1",count)}
 if span,ok:=SpanOf(typed);!ok||span.Start!=0||span.End!=1 {t.Fatal(span,ok)}
}
`))
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("DuckDB walk compatibility: %v\n%s", err, out)
	}
}
