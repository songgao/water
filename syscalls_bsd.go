// +build openbsd freebsd netbsd

package water

import (
	"errors"
	"os"
)

func openDev(config Config) (ifce *Interface, err error) {
	switch config.Name[:8] {
	case "/dev/tap":
		return newTAP(config)
	case "/dev/tun":
		return newTUN(config)
	default:
		return nil, errors.New("unrecognized driver")
	}
}

func newTAP(config Config) (ifce *Interface, err error) {
	if config.Name[:8] != "/dev/tap" {
		panic("TUN/TAP name must be in format /dev/tunX or /dev/tapX")
	}

	file, err := os.OpenFile(config.Name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	ifce = &Interface{isTAP: true, ReadWriteCloser: file, name: config.Name[5:]}
	return
}

func newTUN(config Config) (ifce *Interface, err error) {
	if config.Name[:8] != "/dev/tun" {
		panic("TUN/TAP name must be in format /dev/tunX or /dev/tapX")
	}

	file, err := os.OpenFile(config.Name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	ifce = &Interface{isTAP: false, ReadWriteCloser: file, name: config.Name[5:]}
	return
}
