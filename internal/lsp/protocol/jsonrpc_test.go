package protocol

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestReadMessageHeaderLimit(t *testing.T) {
	body := `{}`
	tail := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n"
	padding := "X-Padding: " + strings.Repeat("a", MaxHeader-len(tail)-len("X-Padding: \r\n")) + "\r\n"
	if len(padding)+len(tail) != MaxHeader {
		t.Fatalf("header of %d bytes", len(padding)+len(tail))
	}
	got, err := ReadMessage(bufio.NewReader(strings.NewReader(padding + tail + body)))
	if err != nil || string(got) != body {
		t.Fatalf("header at the limit: %q, %v", got, err)
	}
	_, err = ReadMessage(bufio.NewReader(strings.NewReader("X" + padding + tail + body)))
	if !errors.Is(err, ErrHeaderTooLarge) {
		t.Fatalf("header over the limit: %v", err)
	}

	// Neither an unterminated line nor endless short lines are accumulated.
	for name, line := range map[string]string{"unterminated line": "a", "short fields": "X: y\r\n"} {
		r := &repeatReader{line: line}
		_, err := ReadMessage(bufio.NewReader(r))
		if !errors.Is(err, ErrHeaderTooLarge) {
			t.Fatalf("%s: %v", name, err)
		}
		// The limit plus at most one buffer fill.
		if r.read > MaxHeader+4096 {
			t.Errorf("%s: read %d bytes", name, r.read)
		}
	}
}

// repeatReader yields one line endlessly and counts how much was read.
type repeatReader struct {
	line string
	off  int
	read int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.line[r.off]
		r.off = (r.off + 1) % len(r.line)
	}
	r.read += len(p)
	return len(p), nil
}
