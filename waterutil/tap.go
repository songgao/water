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

func MACDestination(macFrame []byte) net.HardwareAddr {
	return net.HardwareAddr(macFrame[:6])
}

func MACSource(macFrame []byte) net.HardwareAddr {
	return net.HardwareAddr(macFrame[6:12])
}

func MACTagging(macFrame []byte) Tagging {
	if macFrame[12] == 0x81 && macFrame[13] == 0x00 {
		return Tagged
	} else if macFrame[12] == 0x88 && macFrame[13] == 0xa8 {
		return DoubleTagged
	}
	return NotTagged
}

func MACEthertype(macFrame []byte) Ethertype {
	ethertypePos := 12 + MACTagging(macFrame)
	return Ethertype{macFrame[ethertypePos], macFrame[ethertypePos+1]}
}

func MACPayload(macFrame []byte) []byte {
	return macFrame[12+MACTagging(macFrame)+2:]
}
