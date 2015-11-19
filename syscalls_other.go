// +build !linux

package water

import "errors"

func newTAP(ifName string) (ifce *Interface, err error) {
	return nil, errors.New("tap interface not implemented on this platform")
}

func newTUN(ifName string) (ifce *Interface, err error) {
	return nil, errors.New("tap interface not implemented on this platform")
}
