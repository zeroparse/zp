package runtimetest

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// hotInline lists the methods and functions that generated parsers call on their hot paths. A
// generated parser calls them across the package boundary, so they only cost a few instructions
// if the compiler can inline them; the test fails when a change to the runtime makes one of them
// too costly to inline. Each entry is as the compiler names it.
var hotInline = []string{
	"(*BudgetControl).refill", // inside the runtime: a stream's default budget refills whenever the parser reads further
	"(*Parser).AtEndBatch",
	"(*Parser).AtEndBatchOrdinary",
	"(*Parser).AtEndOrdinary",
	"(*Parser).AtLineStartBatch",
	"(*Parser).AtLineStartBatchOrdinary",
	"(*Parser).DropKids",
	"(*Parser).Enter",
	"(*Parser).ExpectOrdinary",
	"(*Parser).LiteralText",
	"(*Parser).Mark",
	"(*Parser).NewBranchOrdinary",
	"(*Parser).NewNodeOrdinary",
	"(*Parser).NewTerminalOrdinary",
	"(*Parser).Nodes",
	"(*Parser).PushRecovered",
	"(*Parser).Restore",
	"(*Parser).SetCapture",
	"(*Parser).Spend",
	"(*Parser).spendReadOrdinary",
	"(*Node).End",
	"(*Node).Rule",
	"(*Node).Start",
	"(*Node).Type",
	"(*TMatch).TFresh",
	"(*TMatch).TKids",
	"(*TMatch).TSetFresh",
	"(*TMatch).TStruct",
	"(*TMatch).TText",
	"(*TParser).ClassFast",
}

func TestHotPathHooksInline(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the compiler")
	}
	cmd := exec.Command("go", "build", "-gcflags=-m=2", ".", "./parse")
	cmd.Dir = filepath.Join(gentest.ModuleRoot(), "runtime")
	cmd.Env = append(os.Environ(), "GOFLAGS="+os.Getenv("GOFLAGS"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	can := map[string]bool{}
	for _, m := range regexp.MustCompile(`can inline (\S+) with cost`).FindAllSubmatch(out, -1) {
		can[string(m[1])] = true
	}
	for _, name := range hotInline {
		if !can[name] {
			t.Errorf("%s can no longer be inlined", name)
		}
	}
}
