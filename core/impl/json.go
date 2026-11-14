package coreimpl

import (
	"encoding/json"
	"io"
	"strconv"
)

// JSONOutput coalesces the small fragments of a node's JSON while preserving writer errors and
// the already-written prefix when an unsupported value is encountered.
type JSONOutput struct {
	W   io.Writer
	Buf []byte
}

// Flush writes the buffered bytes to W.
func (out *JSONOutput) Flush() error {
	if len(out.Buf) == 0 {
		return nil
	}
	if err := writeAll(out.W, out.Buf); err != nil {
		return err
	}
	out.Buf = out.Buf[:0]
	return nil
}

// Bytes writes data, which may be longer than the buffer.
func (out *JSONOutput) Bytes(data []byte) error {
	if len(out.Buf)+len(data) > cap(out.Buf) {
		if err := out.Flush(); err != nil {
			return err
		}
	}
	if len(data) > cap(out.Buf) {
		return writeAll(out.W, data)
	}
	out.Buf = append(out.Buf, data...)
	return nil
}

// Raw writes s, which is JSON text.
func (out *JSONOutput) Raw(s string) error {
	if len(out.Buf)+len(s) > cap(out.Buf) {
		if err := out.Flush(); err != nil {
			return err
		}
	}
	out.Buf = append(out.Buf, s...)
	return nil
}

// Number writes n as a JSON number.
func (out *JSONOutput) Number(n int32) error {
	if len(out.Buf)+11 > cap(out.Buf) {
		if err := out.Flush(); err != nil {
			return err
		}
	}
	out.Buf = strconv.AppendInt(out.Buf, int64(n), 10)
	return nil
}

// Encoded writes value as encoding/json encodes it.
func (out *JSONOutput) Encoded(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return out.Bytes(data)
}

func writeAll(w io.Writer, data []byte) error {
	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}
