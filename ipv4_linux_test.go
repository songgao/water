package water

import (
	"net"
	"os/exec"
	"testing"
)

func startPing(t *testing.T, dst net.IP, dashB bool) {
	params := []string{"-c", "4", dst.String()}
	if dashB {
		params = append([]string{"-b"}, params...)
	}
	if err := exec.Command("ping", params...).Start(); err != nil {
		t.Fatal(err)
	}
}

func setupIfce(t *testing.T, ipNet net.IPNet, dev string) {
	if err := exec.Command("ip", "link", "set", dev, "up").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("ip", "addr", "add", ipNet.String(), "dev", dev).Run(); err != nil {
		t.Fatal(err)
	}
}

func teardownIfce(t *testing.T, ifce Interface) {
	if err := ifce.Close(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("ip", "link", "set", ifce.Name(), "down").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestBroadcastTAP(t *testing.T) {
	var (
		self = net.IPv4(10, 0, 42, 1)
		mask = net.IPv4Mask(255, 255, 255, 0)
		brd  = net.IPv4(10, 0, 42, 255)
	)

	ifce, err := New(Config{DeviceType: TAP})
	if err != nil {
		t.Fatalf("creating TAP error: %v\n", err)
	}
	defer teardownIfce(t, ifce)

	dataCh, errCh := startRead(t, ifce)

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startPing(t, brd, true)

	waitForPingOrBust(t, true, true, self, brd, dataCh, errCh)
}

func TestBroadcastTUN(t *testing.T) {
	var (
		self = net.IPv4(10, 0, 42, 1)
		mask = net.IPv4Mask(255, 255, 255, 0)
		brd  = net.IPv4(10, 0, 42, 255)
	)

	ifce, err := New(Config{DeviceType: TUN})
	if err != nil {
		t.Fatalf("creating TUN error: %v\n", err)
	}
	defer teardownIfce(t, ifce)

	dataCh, errCh := startRead(t, ifce)

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startPing(t, brd, true)

	waitForPingOrBust(t, false, true, self, brd, dataCh, errCh)
}

func TestUnicastTUN(t *testing.T) {
	var (
		self   = net.IPv4(10, 0, 42, 1)
		mask   = net.IPv4Mask(255, 255, 255, 0)
		remote = net.IPv4(10, 0, 42, 2)
	)

	ifce, err := New(Config{DeviceType: TUN})
	if err != nil {
		t.Fatalf("creating TUN error: %v\n", err)
	}
	defer teardownIfce(t, ifce)

	dataCh, errCh := startRead(t, ifce)

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startPing(t, remote, false)

	waitForPingOrBust(t, false, false, self, remote, dataCh, errCh)
}
