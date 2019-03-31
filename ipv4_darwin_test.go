package water

import (
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/songgao/water/waterutil"
)

func startPing(t *testing.T, dst net.IP) {
	if err := exec.Command("ping", "-c", "4", dst.String()).Start(); err != nil {
		t.Fatal(err)
	}
}

func setupIfce(t *testing.T, self net.IP, remote net.IP, dev string) {
	if err := exec.Command("ifconfig", dev, "inet", self.String(), remote.String(), "up").Run(); err != nil {
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

	dataCh := make(chan []byte)
	errCh := make(chan error)
	startRead(t, ifce, dataCh, errCh)

	setupIfce(t, self, remote, ifce.Name())
	startPing(t, remote)

	timeout := time.NewTimer(8 * time.Second).C

readFrame:
	for {
		select {
		case packet := <-dataCh:
			if !waterutil.IsIPv4(packet) {
				continue readFrame
			}
			if !waterutil.IPv4Source(packet).Equal(self) {
				continue readFrame
			}
			if !waterutil.IPv4Destination(packet).Equal(remote) {
				continue readFrame
			}
			if waterutil.IPv4Protocol(packet) != waterutil.ICMP {
				continue readFrame
			}
			t.Logf("received broadcast packet: %#v\n", packet)
			break readFrame
		case err := <-errCh:
			t.Fatalf("read error: %v", err)
		case <-timeout:
			t.Fatal("Waiting for broadcast packet timeout")
		}
	}
}
