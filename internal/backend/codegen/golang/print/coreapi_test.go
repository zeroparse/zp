package print_test

import (
	"bytes"
	"os"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/golang/print"
)

// TestCoreAPICurrent checks that coreapi_gen.go, the index of the declarations of core that the
// generator reads, is what go generate writes from the package core as it is.
func TestCoreAPICurrent(t *testing.T) {
	want, err := print.CoreAPISource(os.DirFS("../../../../../core"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("coreapi_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("coreapi_gen.go is out of date with the package core; run go generate ./internal/backend/codegen/golang/print")
	}
}
