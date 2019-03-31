// +build go1.11

package water

import (
	"context"
	"testing"
	"time"
)

func TestCloseUnblockPendingRead(t *testing.T) {
	ifce, err := New(Config{DeviceType: TUN})
	if err != nil {
		t.Fatalf("creating TUN error: %v\n", err)
	}

	c := make(chan struct{})
	go func() {
		ifce.Read(make([]byte, 1<<16))
		close(c)
	}()

	// make sure ifce.Close() happens after ifce.Read() blocks
	time.Sleep(1 * time.Second)

	ifce.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	select {
	case <-c:
		t.Log("Pending Read unblocked")
	case <-ctx.Done():
		t.Fatal("Timeouted, pending read blocked")
	}
}
