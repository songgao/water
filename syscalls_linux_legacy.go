// +build linux,!go1.11

package water

import (
	"os"
)

func openDev(config Config) (Interface, error) {
	var file *os.File
	var err error
	if file, err = os.OpenFile(
		"/dev/net/tun", os.O_RDWR, 0); err != nil {
		return nil, err
	}

	name, err := setupFd(config, file.Fd())
	if err != nil {
		return nil, err
	}

	return &ifce{
		deviceType:      config.DeviceType,
		fd:              file.Fd(),
		name:            name,
		ReadWriteCloser: file,
	}, nil
}
