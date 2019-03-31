package water

import (
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/songgao/water/waterutil"
)

func startBroadcast(t *testing.T, dst net.IP) {
	if err := exec.Command("ping", "-b", "-c", "4", dst.String()).Start(); err != nil {
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

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startBroadcast(t, brd)

	dataCh := make(chan []byte)
	errCh := make(chan error)
	startRead(t, ifce, dataCh, errCh)

	timeout := time.NewTimer(8 * time.Second).C

readFrame:
	for {
		select {
		case buffer := <-dataCh:
			ethertype := waterutil.MACEthertype(buffer)
			if ethertype != waterutil.IPv4 {
				continue readFrame
			}
			if !waterutil.IsBroadcast(waterutil.MACDestination(buffer)) {
				continue readFrame
			}
			packet := waterutil.MACPayload(buffer)
			if !waterutil.IsIPv4(packet) {
				continue readFrame
			}
			if !waterutil.IPv4Source(packet).Equal(self) {
				continue readFrame
			}
			if !waterutil.IPv4Destination(packet).Equal(brd) {
				continue readFrame
			}
			if waterutil.IPv4Protocol(packet) != waterutil.ICMP {
				continue readFrame
			}
			t.Logf("received broadcast frame: %#v\n", buffer)
			break readFrame
		case err := <-errCh:
			t.Fatalf("read error: %v", err)
		case <-timeout:
			t.Fatal("Waiting for broadcast packet timeout")
		}
	}
}
