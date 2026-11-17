package schemaexport

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/internal/testing/compiletest"
	"zeroparse.dev/zp/pkg/runtime/schema"
)

// compileError compiles src, which must not compile, and returns the error.
func compileError(t *testing.T, src string) string {
	t.Helper()
	g, err := compiletest.Parse(src)
	if err != nil {
		t.Fatalf("syntax error: %v\n%s", err, src)
	}
	if _, err := compiletest.Compile(g, compile.Options{}); err != nil {
		return err.Error()
	}
	t.Fatalf("expected a compile error\n%s", src)
	return ""
}

func TestOutputSchemaEmptyJoinRoundTrip(t *testing.T) {
	// A field cannot have a type without values, but a rule can: one that never succeeds.
	if message := compileError(t, `type Holder = { Never _|_ }
rule main = "a"`); !strings.Contains(message, "no values") {
		t.Fatalf("field without values: %s", message)
	}
	empty, err := shapeOf(types.TyBottom)
	if err != nil {
		t.Fatal(err)
	}
	sch := validInMemoryOutputSchema()
	sch.Types[1].Shape = empty
	if err := sch.Validate(); err != nil {
		t.Fatalf("valid empty join schema: %v", err)
	}
	data, err := json.Marshal(sch)
	if err != nil {
		t.Fatal(err)
	}
	var decoded schema.Output
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode valid empty join: %v", err)
	}
	if !reflect.DeepEqual(sch, decoded) || decoded.Types[1].Shape.Kind != "join" || len(decoded.Types[1].Shape.Alternatives) != 0 {
		t.Fatalf("empty join round trip changed schema: before=%+v after=%+v", sch, decoded)
	}
}

func TestOutputSchemaInferredCaptureAndDynamic(t *testing.T) {
	shape, err := shapeOf(types.NodeTy{
		Value: types.TupleTy{Elems: []types.Ty{types.TyStringNode, types.TyTop, types.OptTy{Elem: types.ListTy{Elem: types.TyString}}}},
		Caps: []types.CapIndex{
			{Name: "z", Paths: [][]int{{1}}, T: types.TyTop},
			{Name: "a", Paths: [][]int{{2}, {0}}, T: types.OptTy{Elem: types.ListTy{Elem: types.TyString}}},
			{Name: "s", Side: true, T: types.TyStringNode},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Captures are listed by name, whatever the order of the places in the node.
	if shape.Kind != "node" || shape.Value.Kind != "tuple" || len(shape.Value.Elements) != 3 || shape.Value.Elements[1].Kind != "top" ||
		len(shape.Captures) != 3 || shape.Captures[0].Name != "a" || shape.Captures[0].Shape.Kind != "optional" ||
		!reflect.DeepEqual(shape.Captures[0].Paths, [][]int{{0}, {2}}) || shape.Captures[1].Name != "s" || !shape.Captures[1].Side ||
		len(shape.Captures[1].Paths) != 0 || shape.Captures[2].Name != "z" || shape.Captures[2].Shape.Kind != "top" {
		t.Fatalf("shape %+v", shape)
	}
	joinShape, err := shapeOf(types.JoinTy{Alts: []types.Ty{types.TyString, types.TyInt}})
	if err != nil || len(joinShape.Alternatives) != 2 {
		t.Fatalf("join %+v %v", joinShape, err)
	}
}

func TestOutputSchemaRejectsUnresolvedShapes(t *testing.T) {
	p := &compile.Program{Model: &check.Model{Types: map[string]grammar.TypeDecl{"Known": &grammar.TypeDef{Name: "Known", Type: &grammar.StructType{}}}}}
	// The checker reports undefined names, which resolve to _ so that checking can go on; a
	// checked program has none. *_ is _, since _ includes nil.
	for _, expr := range []grammar.TypeExpr{
		&grammar.TypeRef{Name: "UnknownShape"},
		&grammar.ListType{Elem: &grammar.OptionalType{Elem: &grammar.TypeRef{Name: "UnknownShape"}}},
		&grammar.JoinType{Types: []grammar.TypeExpr{&grammar.TypeRef{Name: "Known"}, &grammar.TypeRef{Name: "UnknownShape"}}},
		&grammar.TypeGroup{},
	} {
		shape, err := declared(p, expr)
		if err != nil || shape.Kind != "top" && (shape.Kind != "list" || shape.Element.Kind != "top") {
			data, _ := json.Marshal(shape)
			t.Errorf("unresolved declared shape %T: %s %v", expr, data, err)
		}
	}
	if message := compileError(t, "type T = { F UnknownShape }\nrule main = \"a\""); !strings.Contains(message, "UnknownShape") {
		t.Errorf("undefined field type: %s", message)
	}
	for _, inferred := range []types.Ty{types.BasicTy("other"), types.NamedTy{Name: "Known", Kind: '?'}, (*types.ListTy)(nil)} {
		if shape, err := shapeOf(inferred); err == nil {
			t.Errorf("accepted unresolved inferred shape %T: %+v", inferred, shape)
		}
	}
	if shape, err := shapeOf(types.TyBottom); err != nil || shape.Kind != "join" || len(shape.Alternatives) != 0 {
		t.Errorf("_|_ is not the empty join: %+v %v", shape, err)
	}
}

// TestOutputSchemaCaptureOrderIndependent checks that the capture entries of a node do not depend on
// the order in which the captures are listed.
func TestOutputSchemaCaptureOrderIndependent(t *testing.T) {
	caps := []types.CapIndex{
		{Name: "b", Paths: [][]int{{1}}, T: types.TyString},
		{Name: "a", Paths: [][]int{{0}}, T: types.ListTy{Elem: types.TyString}},
		{Name: "a", Paths: [][]int{{2}}, T: types.TyString},
	}
	reversed := []types.CapIndex{caps[2], caps[1], caps[0]}
	render := func(caps []types.CapIndex) string {
		v, err := shapeOf(types.NodeTy{Value: types.TyString, Caps: caps})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if a, b := render(caps), render(reversed); a != b {
		t.Fatalf("order changes the shape:\n%s\n%s", a, b)
	}
}
