// +build !linux

package water

const (
	cIFF_TUN   = 0
	cIFF_TAP   = 0
	cIFF_NO_PI = 0
)

type ifReq struct {
	Name  [0]byte
	Flags uint16
	pad   [0]byte
}

func createInterface(fd uintptr, ifName string, flags uint16) (createdIFName string, err error) {
	panic("water: createInterface not implemented on this platform")
}