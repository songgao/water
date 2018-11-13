package water

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/songgao/water/waterutil"
)

const BUFFERSIZE = 1522

func startRead(ch chan<- []byte, ifce *Interface) {
	go func() {
		for {
			buffer := make([]byte, BUFFERSIZE)
			n, err := ifce.Read(buffer)
			if err == nil {
				buffer = buffer[:n:n]
				ch <- buffer
			}
		}
	}()
}

func TestBroadcast(t *testing.T) {
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

	dataCh := make(chan []byte, 8)
	startRead(dataCh, ifce)

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
		case <-timeout:
			t.Fatal("Waiting for broadcast packet timeout")
		}
	}
}

func TestCloseUnblockPendingRead(t *testing.T) {
	var (
		self = net.IPv4(192, 168, 150, 1)
		mask = net.IPv4Mask(255, 255, 255, 0)
	)

	ifce, err := New(Config{DeviceType: TUN})
	if err != nil {
		t.Fatalf("creating TUN error: %v\n", err)
	}

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	c := make(chan struct{})
	go func() {
		ifce.Read(make([]byte, 1<<16))
		close(c)
	}()

	// make sure ifce.Close() happens after ifce.Read()
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
