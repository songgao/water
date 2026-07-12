package waterutil

// Header lengths, in bytes, of the fixed part of each IP header.
const (
	ipv4HeaderLen = 20
	ipv6HeaderLen = 40
)

// IsIPv4 reports whether packet is a well-formed-looking IPv4 packet: long
// enough to hold an IPv4 header, and carrying version 4. A packet too short to
// tell is not IPv4.
func IsIPv4(packet []byte) bool {
	if len(packet) < ipv4HeaderLen {
		return false
	}
	return 4 == (packet[0] >> 4)
}

// IsIPv6 reports whether packet is a well-formed-looking IPv6 packet: long
// enough to hold an IPv6 header, and carrying version 6. A packet too short to
// tell is not IPv6.
func IsIPv6(packet []byte) bool {
	if len(packet) < ipv6HeaderLen {
		return false
	}
	return 6 == (packet[0] >> 4)
}
