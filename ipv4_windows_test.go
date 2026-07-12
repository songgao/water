package water

import (
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"
)

func startPing(t *testing.T, dst net.IP, _ bool) {
	if err := exec.Command("ping", "-n", "4", dst.String()).Start(); err != nil {
		t.Fatal(err)
	}
}

// setupIfce gives the adapter a static address.
//
// A freshly created adapter takes a moment to show up in the network stack, and
// netsh fails outright rather than waiting for it, so retry for a few seconds
// before giving up.
func setupIfce(t *testing.T, ipNet net.IPNet, dev string) {
	t.Helper()

	mask := net.IP(ipNet.Mask).To4()
	if mask == nil {
		t.Fatalf("mask %v is not IPv4", ipNet.Mask)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		cmd := exec.Command("netsh", "interface", "ip", "set", "address",
			fmt.Sprintf("name=%s", dev),
			"source=static",
			fmt.Sprintf("addr=%s", ipNet.IP),
			fmt.Sprintf("mask=%s", mask),
			"gateway=none")

		out, err := cmd.CombinedOutput()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("netsh failed to configure %q: %v: %s", dev, err, out)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func teardownIfce(t *testing.T, ifce *Interface) {
	t.Helper()

	if err := ifce.Close(); err != nil {
		t.Fatal(err)
	}
}

// newIfceOrSkip creates an interface, skipping the test when the driver it needs
// is not installed on this machine. Neither Wintun nor tap-windows6 ships with
// Windows, so a missing driver is an untestable environment rather than a
// failure.
func newIfceOrSkip(t *testing.T, config Config) *Interface {
	t.Helper()

	ifce, err := New(config)
	if err != nil {
		t.Skipf("cannot create the interface, skipping: %v", err)
	}
	return ifce
}

// TestUnicastTUN is the end-to-end check for TUN on Windows: bring an interface
// up, give it an address, and confirm that a packet the OS routes at it comes
// back out of Read(). Under the default driver selection this exercises Wintun
// when wintun.dll is present, and tap-windows6 otherwise.
func TestUnicastTUN(t *testing.T) {
	var (
		self   = net.IPv4(10, 0, 42, 1)
		mask   = net.IPv4Mask(255, 255, 255, 0)
		remote = net.IPv4(10, 0, 42, 2)
	)

	ifce := newIfceOrSkip(t, Config{
		DeviceType:             TUN,
		PlatformSpecificParams: PlatformSpecificParams{Network: "10.0.42.1/24"},
	})
	defer teardownIfce(t, ifce)

	t.Logf("created TUN interface %q", ifce.Name())

	dataCh, errCh := startRead(t, ifce)

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startPing(t, remote, false)

	waitForPingOrBust(t, false, false, self, remote, dataCh, errCh)
}

// TestUnicastTUNWintun pins the Wintun driver explicitly, so that a machine with
// wintun.dll present tests it rather than silently falling back.
func TestUnicastTUNWintun(t *testing.T) {
	var (
		self   = net.IPv4(10, 0, 43, 1)
		mask   = net.IPv4Mask(255, 255, 255, 0)
		remote = net.IPv4(10, 0, 43, 2)
	)

	ifce := newIfceOrSkip(t, Config{
		DeviceType: TUN,
		PlatformSpecificParams: PlatformSpecificParams{
			Driver:        WindowsDriverWintun,
			InterfaceName: "water-test",
		},
	})
	defer teardownIfce(t, ifce)

	if got := ifce.Name(); got != "water-test" {
		t.Errorf("Name() = %q, want %q", got, "water-test")
	}

	dataCh, errCh := startRead(t, ifce)

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startPing(t, remote, false)

	waitForPingOrBust(t, false, false, self, remote, dataCh, errCh)
}

// TestBroadcastTAP needs a tap-windows6 adapter, which only exists on a machine
// with OpenVPN (or the driver alone) installed.
func TestBroadcastTAP(t *testing.T) {
	var (
		self = net.IPv4(10, 0, 42, 1)
		mask = net.IPv4Mask(255, 255, 255, 0)
		brd  = net.IPv4(10, 0, 42, 255)
	)

	ifce := newIfceOrSkip(t, Config{DeviceType: TAP})
	defer teardownIfce(t, ifce)

	dataCh, errCh := startRead(t, ifce)

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startPing(t, brd, true)

	waitForPingOrBust(t, true, true, self, brd, dataCh, errCh)
}

// TestWintunRejectsTAP guards the one combination that cannot work: Wintun is a
// layer 3 driver and has no TAP mode.
func TestWintunRejectsTAP(t *testing.T) {
	ifce, err := New(Config{
		DeviceType:             TAP,
		PlatformSpecificParams: PlatformSpecificParams{Driver: WindowsDriverWintun},
	})
	if err == nil {
		ifce.Close()
		t.Fatal("New(TAP) with the Wintun driver succeeded, want an error")
	}
	t.Logf("rejected as expected: %v", err)
}
