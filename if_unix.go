// +build linux darwin

package water

import "errors"

func newDev(config Config) (ifce *Interface, err error) {
	switch config.DeviceType {
	case TUN:
		return newTUN(config.Name)
	case TAP:
		return newTAP(config.Name)
	default:
		return nil, errors.New("unknown device type")
	}
}
