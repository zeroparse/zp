package core

import (
	"testing"
	"unsafe"

	"zeroparse.dev/zp/core/impl"
)

// The parsers convert a *Node to a *coreimpl.Data[Node] and back; that needs Data to be the only
// field of Node.
func TestNodeLayout(t *testing.T) {
	var n Node
	if unsafe.Sizeof(n) != unsafe.Sizeof(coreimpl.Data[Node]{}) || unsafe.Offsetof(n.d) != 0 {
		t.Fatalf("Node is %d bytes with its Data at %d; Data is %d bytes", unsafe.Sizeof(n), unsafe.Offsetof(n.d), unsafe.Sizeof(coreimpl.Data[Node]{}))
	}
	if unsafe.Sizeof(n) != 104 && unsafe.Sizeof(uintptr(0)) == 8 {
		t.Errorf("Node is %d bytes, want 104", unsafe.Sizeof(n))
	}
}
