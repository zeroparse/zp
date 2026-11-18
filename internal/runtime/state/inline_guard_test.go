package state

import (
	"os/exec"
	"strings"
	"testing"
)

// TestStrictHotHelpersInline keeps the helpers that every parse runs inlinable. They sit at or near
// the compiler's inlining budget, so a branch added to one of them silently costs strict parses time.
// The allocators count memory in their refill branches (budget_memory.go) and must stay inlined
// with that addition; the memo chunks and the first-call bit set are counted from their state, so
// Alloc and MarkSeen carry none. Every call records its recovered errors (recSince, recAdd) and
// every recovery one error (PushRecovered), almost always none or one.
func TestStrictHotHelpersInline(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the compiler")
	}
	out, err := exec.Command("go", "build", "-gcflags=-m=2", "-o", "/dev/null", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	for _, name := range []string{"(*Parser).ExprLoop", "(*Parser).NudBegin", "(*Parser).TriviaBegin", "(*Parser).Mark", "(*Parser).SpendN", "(*Parser).NewNode", "(*Parser).Nodes", "(*Parser).fields", "(*MemoTable).Alloc", "(*MemoTable).MarkSeen", "(*Parser).recSince", "(*Parser).recAdd", "(*Parser).PushRecovered"} {
		if !strings.Contains(string(out), "can inline "+name+" with cost") {
			t.Errorf("%s is no longer inlinable", name)
		}
	}
}
