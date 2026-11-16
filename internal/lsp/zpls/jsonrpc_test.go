package zpls

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/lsp/protocol"
)

func TestOversizedHeaderStopsServer(t *testing.T) {
	c := newInitialized(t)
	// The reader checks the limit as its 4096-byte buffer fills, so send one fill past it.
	go c.w.Write([]byte(strings.Repeat("a", protocol.MaxHeader+4096)))
	select {
	case err := <-c.done:
		if !errors.Is(err, protocol.ErrHeaderTooLarge) {
			t.Errorf("error %v", err)
		}
	case <-timeoutC():
		t.Fatal("the server did not stop")
	}
}
