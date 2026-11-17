// Package grammar loads, validates, formats and converts ZeroParse grammars.
//
// A grammar source (Source) is one grammar input: a named value made from bytes, a string, a
// reader or a path in a file system, in grammar syntax or in JSON. Load gives the grammar AST of a
// source, which package ast defines; Format writes an AST as grammar syntax, and Convert writes a
// source in a given encoding. A program that builds an AST by hand checks it with Validate.
//
// Like all of the module API at v0.x, this package may change in any release.
package grammar

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sync"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Encoding is the encoding of a grammar source: grammar syntax or JSON.
type Encoding int

const (
	// EncodingAuto decides the encoding from the source: the extension of its name (.zp or
	// .json), then its content, where JSON is content whose first byte other than a space, tab,
	// carriage return or line feed is '{', and anything else is grammar syntax.
	EncodingAuto Encoding = iota
	// EncodingZP is grammar syntax.
	EncodingZP
	// EncodingJSON is the JSON form of the grammar AST.
	EncodingJSON
)

// String returns the name of the encoding: "auto", "zp" or "json".
func (e Encoding) String() string {
	switch e {
	case EncodingAuto:
		return "auto"
	case EncodingZP:
		return "zp"
	case EncodingJSON:
		return "json"
	}
	return fmt.Sprintf("Encoding(%d)", int(e))
}

// ErrCompiled is the error, wrapped, of loading a compiled grammar (the output of zp compile),
// which holds the bytecode of a parser and not the grammar.
var ErrCompiled = errors.New("a compiled grammar holds the bytecode of a parser, not the grammar source")

// Source is one grammar input, a named value with an encoding. Its fields are not exported, so a
// source has exactly one supplier: the constructor that made it. The zero Source is empty and
// unnamed.
type Source struct {
	name     string
	encoding Encoding
	// read gives the content of the source; nil for the zero Source.
	read func() ([]byte, error)
}

// FromBytes returns the source named name with the content data. The source reads data when it
// is loaded, so the caller must not modify data until then.
func FromBytes(name string, data []byte) Source {
	return Source{name: name, read: func() ([]byte, error) { return data, nil }}
}

// FromString returns the source named name with the content text.
func FromString(name, text string) Source {
	return Source{name: name, read: func() ([]byte, error) { return []byte(text), nil }}
}

// FromReader returns the source named name that r supplies. The source reads r in full when it
// is first loaded, because a diagnostic may need the text of any line, and keeps what it read, so
// later loads of the source and of its copies give the same content (or the same read error).
func FromReader(name string, r io.Reader) Source {
	return Source{name: name, read: sync.OnceValues(func() ([]byte, error) { return io.ReadAll(r) })}
}

// FromFS returns the source at the path name in the file system fsys. The name of the source is
// name, and the source reads the file when it is loaded.
func FromFS(fsys fs.FS, name string) Source {
	return Source{name: name, read: func() ([]byte, error) { return fs.ReadFile(fsys, name) }}
}

// As returns s with the encoding e, which Load then uses instead of deciding one; As(EncodingAuto)
// restores the decision from the name and the content.
func (s Source) As(e Encoding) Source {
	s.encoding = e
	return s
}

// Name returns the name of s: the name given to its constructor.
func (s Source) Name() string { return s.name }

// Encoding returns the encoding of s that As set, or EncodingAuto.
func (s Source) Encoding() Encoding { return s.encoding }

// Decide returns the encoding in which Load reads s: the one that As set, or else the one that
// EncodingAuto decides from the name and the content. It reads s as Load does and returns the same
// errors of reading, including ErrCompiled.
func (s Source) Decide() (Encoding, error) {
	_, e, err := s.load()
	return e, err
}

// load reads s and decides its encoding.
func (s Source) load() ([]byte, Encoding, error) {
	var data []byte
	if s.read != nil {
		var err error
		if data, err = s.read(); err != nil {
			return nil, 0, err
		}
	}
	if bytecode.IsCompiled(data) {
		return nil, 0, ErrCompiled
	}
	return data, s.decide(data), nil
}

// decide returns the encoding of s with the content data.
func (s Source) decide(data []byte) Encoding {
	if s.encoding != EncodingAuto {
		return s.encoding
	}
	switch path.Ext(s.name) {
	case ".zp":
		return EncodingZP
	case ".json":
		return EncodingJSON
	}
	if t := bytes.TrimLeft(data, " \t\r\n"); len(t) > 0 && t[0] == '{' {
		return EncodingJSON
	}
	return EncodingZP
}

// Load returns the grammar AST of s, in grammar syntax or in JSON. JSON is decoded strictly, as
// ast.DecodeJSON does: an unknown field, a duplicate key or trailing data is an error, as is a
// tree that ast.Validate rejects. Load returns an error wrapping ErrCompiled for a compiled
// grammar, the error of reading s, or the errors of the source with their positions; the errors
// do not name s, so a caller that reports them adds Name.
func Load(s Source) (*ast.Grammar, error) {
	data, enc, err := s.load()
	if err != nil {
		return nil, err
	}
	switch enc {
	case EncodingZP:
		return parser.Parse(string(data))
	case EncodingJSON:
		return ast.DecodeJSON(data)
	}
	return nil, fmt.Errorf("unknown encoding %v", enc)
}

// Validate checks the structure of g, an AST that a program has built, as ast.Validate does.
func Validate(g *ast.Grammar) error { return ast.Validate(g) }

// Convert returns the grammar of s in the encoding to: grammar syntax as Format writes it, or the
// JSON form of the AST. It returns an error for EncodingAuto and the errors of Load.
func Convert(s Source, to Encoding) ([]byte, error) {
	if to != EncodingZP && to != EncodingJSON {
		return nil, fmt.Errorf("convert: the target encoding must be zp or json, not %v", to)
	}
	g, err := Load(s)
	if err != nil {
		return nil, err
	}
	if to == EncodingZP {
		return []byte(Format(g)), nil
	}
	return ast.MarshalJSON(g)
}
