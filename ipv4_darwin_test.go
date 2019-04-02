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

func setupIfce(t *testing.T, self net.IP, remote net.IP, dev string) {
	if err := exec.Command("ifconfig", dev, "inet", self.String(), remote.String(), "up").Run(); err != nil {
		t.Fatal(err)
	}
}

func teardownIfce(t *testing.T, ifce Interface) {
	if err := ifce.Close(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("ifconfig", ifce.Name(), "down").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestP2PTUN(t *testing.T) {
	var (
		self   = net.IPv4(10, 0, 42, 1)
		remote = net.IPv4(10, 0, 42, 2)
	)

	ifce, err := New(Config{DeviceType: TUN})
	if err != nil {
		t.Fatalf("creating TUN error: %v\n", err)
	}
	defer teardownIfce(t, ifce)

	dataCh, errCh := startRead(t, ifce)

	setupIfce(t, self, remote, ifce.Name())
	startPing(t, remote, false)

	waitForPingOrBust(t, false, false, self, remote, dataCh, errCh)
}
