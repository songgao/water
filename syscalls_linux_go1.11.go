// +build linux,go1.11

package water

import (
	"os"
	"syscall"
)

func openDev(config Config) (Interface, error) {
	var fdInt int
	var err error
	if fdInt, err = syscall.Open(
		"/dev/net/tun", os.O_RDWR|syscall.O_NONBLOCK, 0); err != nil {
		return nil, err
	}

	name, err := setupFd(config, uintptr(fdInt))
	if err != nil {
		return nil, err
	}

	return &ifce{
		deviceType:      config.DeviceType,
		fd:              uintptr(fdInt),
		name:            name,
		ReadWriteCloser: os.NewFile(uintptr(fdInt), "tun"),
	}, nil
}
