package water

import "io"

// Interface is a TUN/TAP interface.
type Interface struct {
	isTAP bool
	io.ReadWriteCloser
	name string
}

// NewTAP creates a new TAP interface whose name is ifName. If ifName is empty, a
// default name (tap0, tap1, ... ) will be assigned. ifName should not exceed
// 16 bytes. TAP interfaces are not supported on darwin.
func NewTAP(ifName string) (ifce *Interface, err error) {
	return newTAP(ifName)
}

// NewTUN creates a new TUN interface whose name is ifName. If ifName is empty, a
// default name (tap0, tap1, ... ) will be assigned. ifName should not exceed
// 16 bytes. Setting interface name is NOT supported on darwin.
func NewTUN(ifName string) (ifce *Interface, err error) {
	return newTUN(ifName)
}

// IsTUN returns true if ifce is a TUN interface.
func (ifce *Interface) IsTUN() bool {
	return !ifce.isTAP
}

// IsTAP returns true if ifce is a TAP interface.
func (ifce *Interface) IsTAP() bool {
	return ifce.isTAP
}

// Name returns the interface name of ifce, e.g. tun0, tap1, tun0, etc..
func (ifce *Interface) Name() string {
	return ifce.name
}
