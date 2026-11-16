package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// This file implements the base protocol of LSP: JSON-RPC 2.0 messages, each preceded by a header
// with its Content-Length.

// MaxMessage is the size of the largest message read. A larger Content-Length is taken to be a
// corrupt header rather than allocated.
const MaxMessage = 64 << 20

// MaxHeader is the size of the largest message header read, including its line ends and the
// blank line. Base-protocol headers have one or two short fields.
const MaxHeader = 8 << 10

var ErrHeaderTooLarge = fmt.Errorf("message header exceeds %d bytes", MaxHeader)

// JSON-RPC and LSP error codes.
const (
	CodeParseError           = -32700
	CodeInvalidRequest       = -32600
	CodeMethodNotFound       = -32601
	CodeInvalidParams        = -32602
	CodeInternalError        = -32603
	CodeServerNotInitialized = -32002
	CodeRequestFailed        = -32803
	CodeRequestCancelled     = -32800
	CodeContentModified      = -32801
	CodeServerCancelled      = -32802
)

// RPCError is the error of a response.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

// Incoming is a message received from the client: a request (with an id), a notification
// (without one), or a response to a request of the server.
type Incoming struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

// HasID reports whether the message has a non-null id.
func (m *Incoming) HasID() bool { return len(m.ID) > 0 && string(m.ID) != "null" }

// readHeaderLine reads one header line, counting its bytes in used. A header line without an end,
// or a header without its blank line, is rejected once it exceeds maxHeader instead of being
// accumulated.
func readHeaderLine(r *bufio.Reader, used *int) (string, error) {
	var line []byte
	for {
		chunk, err := r.ReadSlice('\n')
		*used += len(chunk)
		if *used > MaxHeader {
			return "", ErrHeaderTooLarge
		}
		line = append(line, chunk...)
		if err != bufio.ErrBufferFull {
			return string(line), err
		}
	}
}

// ReadMessage reads the content of the next message. It returns io.EOF at the end of the input
// before a message starts.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	first := true
	headerBytes := 0
	for {
		line, err := readHeaderLine(r, &headerBytes)
		if err != nil {
			if errors.Is(err, ErrHeaderTooLarge) {
				return nil, err
			}
			if err == io.EOF && first && line == "" {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("reading header: %w", noEOF(err))
		}
		first = false
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header line %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid Content-Length %q", strings.TrimSpace(value))
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("message without Content-Length")
	}
	if length > MaxMessage {
		return nil, fmt.Errorf("message of %d bytes is too large", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("reading message: %w", noEOF(err))
	}
	return body, nil
}

func noEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// Conn writes messages to the client. It is safe for concurrent use.
type Conn struct {
	mu sync.Mutex
	w  io.Writer
}

// NewConn returns a Conn that writes to w.
func NewConn(w io.Writer) *Conn { return &Conn{w: w} }

func (c *Conn) Write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var b bytes.Buffer
	fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n", len(data))
	b.Write(data)
	_, err = c.w.Write(b.Bytes())
	return err
}

// Reply sends the response to the request id: the result, or the error if err is not nil.
func (c *Conn) Reply(id json.RawMessage, result any, err *RPCError) error {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	if err != nil {
		return c.Write(struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *RPCError       `json:"error"`
		}{"2.0", id, err})
	}
	// A successful response always has a result, null if there is none.
	return c.Write(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{"2.0", id, result})
}

// Notify sends a notification.
func (c *Conn) Notify(method string, params any) error {
	return c.Write(struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", method, params})
}
