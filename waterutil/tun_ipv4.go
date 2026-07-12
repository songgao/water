package waterutil

import (
	"net"
)

// The accessors here read packets straight off the wire, so they treat every
// length as hostile. A packet too short to hold the field being asked for yields
// the zero value -- 0, or nil for the slice-valued ones -- and the setters leave
// a too-short packet alone, rather than panicking with an index out of range.
//
// Use IsIPv4 to tell a malformed packet from one that genuinely carries zeroes.

func IPv4DSCP(packet []byte) byte {
	if len(packet) < ipv4HeaderLen {
		return 0
	}
	return packet[1] >> 2
}

func IPv4ECN(packet []byte) byte {
	if len(packet) < ipv4HeaderLen {
		return 0
	}
	return packet[1] & 0x03
}

func IPv4Identification(packet []byte) [2]byte {
	if len(packet) < ipv4HeaderLen {
		return [2]byte{}
	}
	return [2]byte{packet[4], packet[5]}
}

func IPv4TTL(packet []byte) byte {
	if len(packet) < ipv4HeaderLen {
		return 0
	}
	return packet[8]
}

func IPv4Protocol(packet []byte) IPProtocol {
	if len(packet) < ipv4HeaderLen {
		return 0
	}
	return IPProtocol(packet[9])
}

// IPv4Source returns the source address, or nil if packet is too short to have
// one.
func IPv4Source(packet []byte) net.IP {
	if len(packet) < ipv4HeaderLen {
		return nil
	}
	return net.IPv4(packet[12], packet[13], packet[14], packet[15])
}

// SetIPv4Source writes source into packet, and does nothing if packet is too
// short to hold an IPv4 header or source is not an IPv4 address.
func SetIPv4Source(packet []byte, source net.IP) {
	if len(packet) < ipv4HeaderLen {
		return
	}
	copy(packet[12:16], source.To4())
}

// IPv4Destination returns the destination address, or nil if packet is too short
// to have one.
func IPv4Destination(packet []byte) net.IP {
	if len(packet) < ipv4HeaderLen {
		return nil
	}
	return net.IPv4(packet[16], packet[17], packet[18], packet[19])
}

// SetIPv4Destination writes dest into packet, and does nothing if packet is too
// short to hold an IPv4 header or dest is not an IPv4 address.
func SetIPv4Destination(packet []byte, dest net.IP) {
	if len(packet) < ipv4HeaderLen {
		return
	}
	copy(packet[16:20], dest.To4())
}

// IPv4Payload returns the packet's payload, or nil if packet is malformed.
//
// The header length is taken from the packet's own IHL field, which is
// attacker-controlled: it is only believed when it describes a header that is at
// least the minimum size and that actually fits inside the packet.
func IPv4Payload(packet []byte) []byte {
	if len(packet) < ipv4HeaderLen {
		return nil
	}
	headerLen := int(packet[0]&0x0F) * 4
	if headerLen < ipv4HeaderLen || headerLen > len(packet) {
		return nil
	}
	return packet[headerLen:]
}

// For TCP/UDP

func IPv4SourcePort(packet []byte) uint16 {
	payload := IPv4Payload(packet)
	if len(payload) < 4 {
		return 0
	}
	return (uint16(payload[0]) << 8) | uint16(payload[1])
}

func IPv4DestinationPort(packet []byte) uint16 {
	payload := IPv4Payload(packet)
	if len(payload) < 4 {
		return 0
	}
	return (uint16(payload[2]) << 8) | uint16(payload[3])
}

// SetIPv4SourcePort writes port into the packet's TCP/UDP header, and does
// nothing if the packet is too short to have one.
func SetIPv4SourcePort(packet []byte, port uint16) {
	payload := IPv4Payload(packet)
	if len(payload) < 4 {
		return
	}
	payload[0] = byte(port >> 8)
	payload[1] = byte(port & 0xFF)
}

// SetIPv4DestinationPort writes port into the packet's TCP/UDP header, and does
// nothing if the packet is too short to have one.
func SetIPv4DestinationPort(packet []byte, port uint16) {
	payload := IPv4Payload(packet)
	if len(payload) < 4 {
		return
	}
	payload[2] = byte(port >> 8)
	payload[3] = byte(port & 0xFF)
}
