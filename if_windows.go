// +build windows

package water

import "errors"

func newDev(config Config) (ifce *Interface, err error) {
	if config.DeviceType != TAP && config.DeviceType != TUN {
		return nil, errors.New("unknown device type")
	}
	return openDev(config)
}
