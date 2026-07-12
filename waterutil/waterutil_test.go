package waterutil

import (
	"net"
	"testing"
)

// callEverything runs every exported accessor over the same buffer. The
// accessors parse data that came straight off the wire, so none of them may
// panic, whatever the input.
func callEverything(b []byte) {
	IsIPv4(b)
	IsIPv6(b)

	IPv4DSCP(b)
	IPv4ECN(b)
	IPv4Identification(b)
	IPv4TTL(b)
	IPv4Protocol(b)
	IPv4Source(b)
	IPv4Destination(b)
	IPv4Payload(b)
	IPv4SourcePort(b)
	IPv4DestinationPort(b)

	SetIPv4Source(b, net.IPv4(1, 2, 3, 4))
	SetIPv4Destination(b, net.IPv4(1, 2, 3, 4))
	SetIPv4SourcePort(b, 1234)
	SetIPv4DestinationPort(b, 5678)

	MACDestination(b)
	MACSource(b)
	MACTagging(b)
	MACEthertype(b)
	MACPayload(b)

	IsBroadcast(net.HardwareAddr(b))
	IsIPv4Multicast(net.HardwareAddr(b))
}

// TestMalformedInputDoesNotPanic covers the shapes a hostile peer can put on the
// wire. Every one of these used to panic with an index out of range, which meant
// a single crafted packet could take down any process using this package.
func TestMalformedInputDoesNotPanic(t *testing.T) {
	lyingIHL := make([]byte, ipv4HeaderLen)
	lyingIHL[0] = 0x4f // version 4, IHL 15: claims a 60-byte header in a 20-byte packet

	shortIHL := make([]byte, ipv4HeaderLen)
	shortIHL[0] = 0x40 // version 4, IHL 0: claims a header shorter than the minimum

	claimsVLAN := make([]byte, 13)
	claimsVLAN[12] = 0x81 // truncated frame that claims to carry a VLAN tag

	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"one byte", []byte{0x45}},
		{"two bytes", []byte{0x45, 0x00}},
		{"header minus one", make([]byte, ipv4HeaderLen-1)},
		{"IHL claims more than the packet holds", lyingIHL},
		{"IHL below the minimum", shortIHL},
		{"frame claims a VLAN tag it cannot hold", claimsVLAN},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked on %v: %v", tc.input, r)
				}
			}()
			callEverything(tc.input)
		})
	}
}

// TestWellFormedPacketStillParses guards against the bounds checks being so
// eager that they break the happy path.
func TestWellFormedPacketStillParses(t *testing.T) {
	// A 20-byte IPv4 header (IHL 5) followed by 4 bytes of TCP ports.
	packet := []byte{
		0x45, 0x00, 0x00, 0x18,
		0xab, 0xcd, 0x00, 0x00,
		0x40, 0x06, 0x00, 0x00,
		10, 0, 0, 1,
		10, 0, 0, 2,
		0x1f, 0x90, 0x00, 0x50, // ports 8080 -> 80
	}

	if !IsIPv4(packet) {
		t.Fatal("IsIPv4 = false, want true")
	}
	if got, want := IPv4TTL(packet), byte(0x40); got != want {
		t.Errorf("IPv4TTL = %d, want %d", got, want)
	}
	if got, want := IPv4Protocol(packet), IPProtocol(TCP); got != want {
		t.Errorf("IPv4Protocol = %d, want %d", got, want)
	}
	if got, want := IPv4Source(packet), net.IPv4(10, 0, 0, 1); !got.Equal(want) {
		t.Errorf("IPv4Source = %v, want %v", got, want)
	}
	if got, want := IPv4Destination(packet), net.IPv4(10, 0, 0, 2); !got.Equal(want) {
		t.Errorf("IPv4Destination = %v, want %v", got, want)
	}
	if got, want := IPv4Identification(packet), [2]byte{0xab, 0xcd}; got != want {
		t.Errorf("IPv4Identification = %v, want %v", got, want)
	}
	if got, want := len(IPv4Payload(packet)), 4; got != want {
		t.Errorf("len(IPv4Payload) = %d, want %d", got, want)
	}
	if got, want := IPv4SourcePort(packet), uint16(8080); got != want {
		t.Errorf("IPv4SourcePort = %d, want %d", got, want)
	}
	if got, want := IPv4DestinationPort(packet), uint16(80); got != want {
		t.Errorf("IPv4DestinationPort = %d, want %d", got, want)
	}

	SetIPv4SourcePort(packet, 443)
	if got, want := IPv4SourcePort(packet), uint16(443); got != want {
		t.Errorf("after SetIPv4SourcePort, IPv4SourcePort = %d, want %d", got, want)
	}
}

func TestMACFrameStillParses(t *testing.T) {
	frame := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // broadcast destination
		0x3e, 0x22, 0x1a, 0x17, 0x93, 0x9b, // source
		0x08, 0x00, // ethertype IPv4
		0xde, 0xad, 0xbe, 0xef, // payload
	}

	if !IsBroadcast(MACDestination(frame)) {
		t.Error("IsBroadcast(MACDestination) = false, want true")
	}
	if got := MACTagging(frame); got != NotTagged {
		t.Errorf("MACTagging = %v, want NotTagged", got)
	}
	if got := MACEthertype(frame); got != IPv4 {
		t.Errorf("MACEthertype = %v, want IPv4", got)
	}
	if got, want := len(MACPayload(frame)), 4; got != want {
		t.Errorf("len(MACPayload) = %d, want %d", got, want)
	}
}

// FuzzAccessors is the general statement: no byte string, however malformed,
// makes any accessor panic.
func FuzzAccessors(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x45})
	f.Add([]byte{0x4f, 0x00, 0x00, 0x18})
	f.Add(make([]byte, 14))
	f.Add(make([]byte, 20))

	f.Fuzz(func(t *testing.T, packet []byte) {
		callEverything(packet)
	})
}
