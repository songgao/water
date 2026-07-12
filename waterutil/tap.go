package waterutil

import (
	"net"
)

type Tagging int

// Indicating whether/how a MAC frame is tagged. The value is number of bytes taken by tagging.
const (
	NotTagged    Tagging = 0
	Tagged       Tagging = 4
	DoubleTagged Tagging = 8
)

// macHeaderLen is the length of an untagged Ethernet header: two addresses and
// the ethertype.
const macHeaderLen = 14

// As in tun_ipv4.go, these read frames straight off the wire and treat every
// length as hostile: a frame too short to hold the field being asked for yields
// the zero value rather than panicking.

// MACDestination returns the destination address, or nil if macFrame is too
// short to have one.
func MACDestination(macFrame []byte) net.HardwareAddr {
	if len(macFrame) < 6 {
		return nil
	}
	return net.HardwareAddr(macFrame[:6])
}

// MACSource returns the source address, or nil if macFrame is too short to have
// one.
func MACSource(macFrame []byte) net.HardwareAddr {
	if len(macFrame) < 12 {
		return nil
	}
	return net.HardwareAddr(macFrame[6:12])
}

// MACTagging reports how macFrame is tagged. A frame too short to carry a tag is
// NotTagged.
func MACTagging(macFrame []byte) Tagging {
	if len(macFrame) < macHeaderLen {
		return NotTagged
	}
	if macFrame[12] == 0x81 && macFrame[13] == 0x00 {
		return Tagged
	} else if macFrame[12] == 0x88 && macFrame[13] == 0xa8 {
		return DoubleTagged
	}
	return NotTagged
}

// MACEthertype returns the frame's ethertype, or the zero Ethertype if macFrame
// is too short -- including the case of a frame that claims a VLAN tag it is not
// long enough to actually carry.
func MACEthertype(macFrame []byte) Ethertype {
	ethertypePos := 12 + int(MACTagging(macFrame))
	if len(macFrame) < ethertypePos+2 {
		return Ethertype{}
	}
	return Ethertype{macFrame[ethertypePos], macFrame[ethertypePos+1]}
}

// MACPayload returns the frame's payload, or nil if macFrame is too short to
// have one.
func MACPayload(macFrame []byte) []byte {
	payloadPos := 12 + int(MACTagging(macFrame)) + 2
	if len(macFrame) < payloadPos {
		return nil
	}
	return macFrame[payloadPos:]
}

// IsBroadcast reports whether addr is the broadcast address. A short or nil addr
// is not.
func IsBroadcast(addr net.HardwareAddr) bool {
	if len(addr) < 6 {
		return false
	}
	return addr[0] == 0xff && addr[1] == 0xff && addr[2] == 0xff && addr[3] == 0xff && addr[4] == 0xff && addr[5] == 0xff
}

// IsIPv4Multicast reports whether addr is an IPv4 multicast address. A short or
// nil addr is not.
func IsIPv4Multicast(addr net.HardwareAddr) bool {
	if len(addr) < 3 {
		return false
	}
	return addr[0] == 0x01 && addr[1] == 0x00 && addr[2] == 0x5e
}
