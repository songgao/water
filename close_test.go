package water

import (
	"testing"
	"time"
)

// The tests here pin down what Close() promises on every platform:
//
//   - it releases a Read() that is parked waiting for a packet that may never
//     arrive, rather than dead-locking behind it,
//   - it can be called more than once, and
//   - I/O attempted after it fails instead of touching a released device.
//
// None of the three held on Windows before Close() learned to cancel the I/O it
// was waiting on, which made a water Interface there impossible to shut down
// cleanly.

// newTestTUN creates a TUN interface, skipping the test when the platform cannot
// provide one -- no driver installed, or not enough privilege.
func newTestTUN(t *testing.T) *Interface {
	t.Helper()

	ifce, err := New(Config{DeviceType: TUN})
	if err != nil {
		t.Skipf("cannot create a TUN interface here, skipping: %v", err)
	}
	return ifce
}

func TestCloseUnblockPendingRead(t *testing.T) {
	ifce := newTestTUN(t)

	readReturned := make(chan error, 1)
	go func() {
		_, err := ifce.Read(make([]byte, 1<<16))
		readReturned <- err
	}()

	// Give the Read time to actually park in the OS before closing under it.
	time.Sleep(time.Second)

	if err := ifce.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-readReturned:
		t.Logf("pending Read unblocked with: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not unblock the pending Read")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	ifce := newTestTUN(t)

	if err := ifce.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// A second Close must not panic, hang, or double-release the device. It is
	// allowed to report an error, but it has to return.
	done := make(chan struct{})
	go func() {
		ifce.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("second Close hung")
	}
}

func TestReadAfterCloseDoesNotHang(t *testing.T) {
	ifce := newTestTUN(t)

	if err := ifce.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := ifce.Read(make([]byte, 1<<16))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Read on a closed interface returned a nil error")
		}
		t.Logf("Read after Close returned: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Read on a closed interface hung")
	}
}

// TestConcurrentCloseAndRead is the race the ioGate on Windows exists to make
// safe: Close tearing the device down while a Read is in flight. Run it under
// -race.
func TestConcurrentCloseAndRead(t *testing.T) {
	ifce := newTestTUN(t)

	const readers = 4
	done := make(chan struct{}, readers)
	for i := 0; i < readers; i++ {
		go func() {
			for {
				if _, err := ifce.Read(make([]byte, 1<<16)); err != nil {
					done <- struct{}{}
					return
				}
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	if err := ifce.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for i := 0; i < readers; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Close did not unblock every pending Read")
		}
	}
}
