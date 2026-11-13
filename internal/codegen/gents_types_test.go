package codegen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
)

// A join with the error member must not gain an open-node member, whether it is named by an alias
// or inferred for a field: both forms share one TypeScript union and one recipe.
func TestGeneratedTypedErrorJoinHasNoOpenMember(t *testing.T) {
	t.Parallel()
	if codegen.CstJoin(compile.JoinTy{Alts: []compile.Ty{compile.NamedTy{Name: "A", Kind: 's'}, compile.TyErr}}) {
		t.Error("cstJoin counts the error member as an open node")
	}
	if !codegen.CstJoin(compile.JoinTy{Alts: []compile.Ty{compile.NamedTy{Name: "A", Kind: 's'}, compile.TyInt}}) {
		t.Error("cstJoin must keep counting a scalar member as an open node")
	}
	// The raw spelling of node[_] is an open member, with or without error next to it.
	if !codegen.CstJoin(compile.JoinTy{Alts: []compile.Ty{compile.NamedTy{Name: "A", Kind: 's'}, compile.NodeTy{Value: compile.TyTop}}}) {
		t.Error("cstJoin does not count a raw node[_] member as an open node")
	}
	if !codegen.CstJoin(compile.JoinTy{Alts: []compile.Ty{compile.NamedTy{Name: "A", Kind: 's'}, compile.NodeTy{Value: compile.TyTop}, compile.TyErr}}) {
		t.Error("cstJoin does not count a raw node[_] member next to error as an open node")
	}
	src := `type Block = { Body []Stmt }
type Let = { N string }
type Stmt ~ Let | Block | error
rule main Block = body:stmts => Block{Body: $body}
rule stmts = ss:(s:stmt)* => map($ss, (x) => x.$s)
rule stmt Stmt = s:(let / block) #recover(skip=[^;{}]+ ";"?) => $s
rule block Block = "{" body:stmts "}" => Block{Body: $body}
rule let Let = n:([a-z]+) ";" => Let{N: text($n)}`
	g, err := parseTest(src)
	if err != nil {
		t.Fatal(err)
	}
	files, err := codegentest.GenerateTS(g, codegen.GenOptions{Types: true, Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "types.ts" && strings.Contains(string(f.Content), "Block | TNode<unknown>") {
			t.Errorf("types.ts adds an open node to the join with error:\n%s", f.Content)
		}
		if f.Path == "parser.ts" && strings.Contains(string(f.Content), "fallback:") {
			t.Errorf("parser.ts has an open-node fallback for the join with error")
		}
	}
	goFiles, err := codegentest.Generate(t, g, codegen.GenOptions{Types: true, Start: "main", Module: "example.com/m", Package: "p"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range goFiles {
		if strings.Contains(string(f.Content), "TNode[any]) isStmt") {
			t.Errorf("%s makes the open node a member of the join with error", f.Path)
		}
	}
}
