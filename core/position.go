package core

// PositionUnit is the unit of positions in the input.
type PositionUnit int

const (
	// CodePoints counts positions in Unicode code points. It is the default.
	CodePoints PositionUnit = iota
	// Bytes counts positions in UTF-8 bytes.
	Bytes
)

// String returns "codepoints" or "bytes".
func (u PositionUnit) String() string {
	if u == Bytes {
		return "bytes"
	}
	return "codepoints"
}
