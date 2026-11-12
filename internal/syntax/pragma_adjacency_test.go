package syntax

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPragmaAdjacency(t *testing.T) {
	for _, pragma := range []string{"atomic", "stream", "repeat(2)", `recover(skip=";")`} {
		for _, gap := range []string{" ", "\t", "/* gap */", "\n"} {
			src := "rule main = (\"x\" #" + gap + pragma + ")"
			_, err := Parse(src)
			if err == nil || !strings.HasPrefix(err.Error(), positionOf(src, "#"+gap)+": ") {
				t.Fatalf("gap %q before %s: %v", gap, pragma, err)
			}
		}
	}
	for _, pragma := range []string{"repeat", "recover"} {
		args := "(2)"
		if pragma == "recover" {
			args = `(skip=";")`
		}
		for _, gap := range []string{" ", "\t", "/* gap */", "\n"} {
			src := "rule main = (\"x\" #" + pragma + gap + args + ")"
			_, err := Parse(src)
			if err == nil || !strings.HasPrefix(err.Error(), positionOf(src, pragma+gap)+": ") {
				t.Fatalf("gap %q after %s: %v", gap, pragma, err)
			}
		}
	}
	for _, source := range []string{
		`rule main = "x" #atomic()`, `rule main = "x" #stream()`,
		`rule main = "x" #repeat( /* inside */ 2 )`,
		`rule main = "x" #recover( /* inside */ skip = ";" )`,
		`rule main = "x" #atomic ("y")`,
		`rule main = "x" #stream ("y")`,
	} {
		if _, err := Parse(source); err != nil {
			t.Fatalf("valid source %q: %v", source, err)
		}
	}
}

// positionOf returns the line and column, as in an error, of the character after the first
// occurrence of prefix in src: the place where the parse of a source with a gap cannot go on.
func positionOf(src, prefix string) string {
	before := src[:strings.Index(src, prefix)+len(prefix)]
	line := strings.Count(before, "\n") + 1
	col := utf8.RuneCountInString(before[strings.LastIndex(before, "\n")+1:]) + 1
	return fmt.Sprintf("%d:%d", line, col)
}
