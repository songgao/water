// +build linux darwin

package water

import "io"

type ifce struct {
	deviceType DeviceType
	fd         uintptr
	name       string
	io.ReadWriteCloser
}

func (i *ifce) IsTUN() bool {
	return i.deviceType == TUN
}

func (i *ifce) IsTAP() bool {
	return i.deviceType == TAP
}

func (i *ifce) Type() DeviceType {
	return i.deviceType
}

func (i *ifce) Name() string {
	return i.name
}

func (i *ifce) Sys() interface{} {
	return i.fd
}
