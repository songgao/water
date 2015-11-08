package water

import (
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/songgao/water/waterutil"
)

const BUFFERSIZE = 1522

func startTimeout(ch chan<- bool, t time.Duration) {
	go func() {
		time.Sleep(t)
		ch <- true
	}()
}

func startRead(ch chan<- []byte, ifce *Interface) {
	go func() {
		for {
			buffer := make([]byte, BUFFERSIZE)
			_, err := ifce.Read(buffer)
			if err == nil {
				ch <- buffer
			}
		}
	}()
}

func startBroadcast(t *testing.T, dst net.IP) {
	if err := exec.Command("ping", "-b", "-c", "2", dst.String()).Start(); err != nil {
		t.Fatal(err)
	}
}

func TestBroadcast(t *testing.T) {
	var (
		self = net.IPv4(10, 0, 42, 1)
		mask = net.IPv4Mask(255, 255, 255, 0)
		brd  = net.IPv4(10, 0, 42, 255)
	)

	ifce, err := NewTAP("test")
	if err != nil {
		t.Fatal(err)
	}

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startBroadcast(t, brd)

	dataCh := make(chan []byte, 8)
	startRead(dataCh, ifce)

	timeout := make(chan bool, 1)
	startTimeout(timeout, time.Second*8)

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
			break readFrame
		case <-timeout:
			t.Fatal("Waiting for broadcast packet timeout")
		}
	}
}
