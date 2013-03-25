package waterutil

import (
	"net"
)

func IPv4DSCP(packet []byte) byte {
	return packet[1] >> 2
}

func IPv4ECN(packet []byte) byte {
	return packet[1] & 0x03
}

func IPv4Identification(packet []byte) [2]byte {
	return [2]byte{packet[4], packet[5]}
}

func IPv4TTL(packet []byte) byte {
	return packet[8]
}

func IPv4Protocol(packet []byte) IPProtocol {
	return IPProtocol(packet[9])
}

func IPv4Source(packet []byte) net.IP {
	return net.IPv4(packet[12], packet[13], packet[14], packet[15])
}

func IPv4Destination(packet []byte) net.IP {
	return net.IPv4(packet[16], packet[17], packet[18], packet[19])
}

func IPv4Payload(packet []byte) []byte {
	ihl := packet[0] & 0x0F
	return packet[ihl*4:]
}
