package probe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenOptionsAndGenTSArgs(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	got, err := GenOptions(root, "json")
	if err != nil {
		t.Fatal(err)
	}
	if !slicesContain(got, "-recognize") || !slicesContain(got, "-module") && !slicesContain(got, "-lib") {
		t.Errorf("options of json = %v", got)
	}
	ts, err := GenTSArgs(root, "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range ts {
		if a == "-lib" || a == "import" || a == "-nodoc" || a == "-typed-only" {
			t.Errorf("TypeScript arguments keep the Go-only %q: %v", a, ts)
		}
	}
	if len(ts) < 2 || ts[0] != "-g" || !strings.HasSuffix(ts[1], filepath.Join("json", "json.zp")) {
		t.Errorf("TypeScript arguments = %v", ts)
	}
	if _, err := GenOptions(root, "nosuch"); err == nil {
		t.Errorf("an unknown grammar has options")
	}
}

func slicesContain(s []string, x string) bool {
	return strings.Contains(" "+strings.Join(s, " ")+" ", " "+x+" ")
}

func TestSizeOfFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte(strings.Repeat("abc", 1000)), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, gz, err := SizeOfFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 3000 || gz <= 0 || gz >= 100 {
		t.Errorf("raw %d, gzip %d: a repetitive file of 3000 bytes should compress to under 100", raw, gz)
	}
	if _, _, err := SizeOfFile(path + ".none"); err == nil {
		t.Errorf("a missing file has a size")
	}
}
