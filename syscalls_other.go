// +build !linux

package water

func newTAP(ifName string) (ifce *Interface, err error) {
	panic("water: tap interface not implemented on this platform")
}

func newTUN(ifName string) (ifce *Interface, err error) {
	panic("water: tap interface not implemented on this platform")
}
