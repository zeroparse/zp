package print_test

import (
	"bytes"
	"os"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/golang/print"
)

// TestCoreAPICurrent checks that coreapi_gen.go, the index of the declarations of the runtime packages that the
// generator reads, is what go generate writes from those packages as they are.
func TestCoreAPICurrent(t *testing.T) {
	want, err := print.CoreAPISource(os.DirFS("../../../../../pkg/runtime"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("coreapi_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("coreapi_gen.go is out of date with the runtime packages; run go generate ./internal/backend/codegen/golang/print")
	}
}
