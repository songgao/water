package water

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// water supports two drivers on Windows:
//
//   - Wintun (https://www.wintun.net), the layer 3 driver written for
//     WireGuard. It is the better option by a wide margin: it is kernel-signed,
//     needs no adapter to be installed ahead of time, and is a native TUN
//     device rather than an emulation on top of Ethernet. It cannot do TAP.
//
//   - tap-windows6 (https://github.com/OpenVPN/tap-windows6), the driver
//     shipped with OpenVPN, which is what water has always used. It is the only
//     way to get a TAP device, and it is the fallback for TUN when wintun.dll
//     is not around.
//
// Config.Driver picks between them; the default is to pick automatically.

// errDriverUnavailable wraps the errors reported when a driver is not usable on
// this machine at all -- as opposed to being usable but failing. Only the
// former makes WindowsDriverAuto fall back to the other driver, because falling
// back on the latter would replace an accurate error with a confusing one.
var errDriverUnavailable = errors.New("driver not available")

func openDev(config Config) (*Interface, error) {
	// Fill in per-field defaults. Config.New() only substitutes
	// defaultPlatformSpecificParams() when PlatformSpecificParams is entirely
	// zero, so a caller who sets just one field -- InterfaceName, say -- would
	// otherwise end up with an empty ComponentID that can never match an
	// adapter.
	if config.ComponentID == "" {
		config.ComponentID = defaultComponentID
	}
	if config.Network == "" {
		config.Network = defaultNetwork
	}

	switch config.Driver {
	case WindowsDriverWintun:
		if config.DeviceType != TUN {
			return nil, errors.New("water: Wintun supports TUN devices only; use WindowsDriverTapWindows6 for TAP")
		}
		return openDevWintun(config)

	case WindowsDriverTapWindows6:
		return openDevTapWindows6(config)

	case WindowsDriverAuto:
		// Wintun cannot do layer 2, so TAP always means tap-windows6.
		if config.DeviceType != TUN {
			return openDevTapWindows6(config)
		}
		ifce, err := openDevWintun(config)
		if err == nil || !errors.Is(err, errDriverUnavailable) {
			return ifce, err
		}
		// Wintun is not installed. Fall back to tap-windows6, and if that is
		// missing too, report both so the user knows what to install.
		noWintun := err
		ifce, err = openDevTapWindows6(config)
		if errors.Is(err, errDriverUnavailable) {
			return nil, fmt.Errorf("water: no usable TUN driver on this system.\n"+
				"  wintun:       %v\n"+
				"  tap-windows6: %v", noWintun, err)
		}
		return ifce, err

	default:
		return nil, fmt.Errorf("water: unknown Windows driver %d", config.Driver)
	}
}

// ioGate coordinates in-flight Read and Write calls with Close.
//
// Closing a TUN/TAP device is not simply a matter of closing the handle: a
// reader is normally parked inside the OS waiting for a packet that may never
// come, and yanking the handle out from under it is undefined behaviour. So
// Close has to (1) refuse new I/O, (2) actively interrupt the I/O already in
// flight, (3) wait for it to come back, and only then (4) tear the device down.
// ioGate is steps 1 to 3; the driver supplies the interrupt and the teardown.
type ioGate struct {
	mu      sync.RWMutex // held for read by enter, for write by shutdown
	closed  bool
	running sync.WaitGroup
	once    sync.Once
}

// enter registers an I/O operation, or reports os.ErrClosed if the device is
// already closing. Every successful enter must be matched by a leave.
//
// Taking mu for reading around the closed check and the Add is what makes the
// WaitGroup safe: it means no Add can start after shutdown has published
// closed == true, and therefore no Add can race with running.Wait.
func (g *ioGate) enter() error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed {
		return os.ErrClosed
	}
	g.running.Add(1)
	return nil
}

func (g *ioGate) leave() { g.running.Done() }

// closing reports whether Close has been called. Read loops that park in the OS
// consult it after waking up, to tell "a packet arrived" from "Close woke me".
func (g *ioGate) closing() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.closed
}

// shutdown is idempotent, and safe to call while Read or Write is blocked.
//
// interrupt must unblock any I/O currently in flight; it is called with no lock
// held, and the operations it interrupts are expected to return promptly.
// teardown then releases the device, and runs only once every operation has
// returned, so it may assume it has exclusive access.
func (g *ioGate) shutdown(interrupt func(), teardown func() error) error {
	var err error
	g.once.Do(func() {
		g.mu.Lock()
		g.closed = true
		g.mu.Unlock()

		interrupt()
		g.running.Wait()
		err = teardown()
	})
	return err
}
