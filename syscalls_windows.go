// +build windows

// To use it with windows, you need a tap driver installed on windows.
// https://github.com/OpenVPN/tap-windows6
// or just install OpenVPN
// https://github.com/OpenVPN/openvpn
package water

import (
	"bytes"
	"errors"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var (
	IfceNameNotFound  = errors.New("Failed to find the name of interface.")
	TapDeviceNotFound = errors.New("Failed to find the tap device with specified ComponentId in registry, TAP driver may not installed.")
	RegistryOpenErr   = errors.New("Failed to open the adapter registry, TAP driver may not installed.")
	// Device Control Codes
	tap_win_ioctl_get_mac             = tap_control_code(1, 0)
	tap_win_ioctl_get_version         = tap_control_code(2, 0)
	tap_win_ioctl_get_mtu             = tap_control_code(3, 0)
	tap_win_ioctl_get_info            = tap_control_code(4, 0)
	tap_ioctl_config_point_to_point   = tap_control_code(5, 0)
	tap_ioctl_set_media_status        = tap_control_code(6, 0)
	tap_win_ioctl_config_dhcp_masq    = tap_control_code(7, 0)
	tap_win_ioctl_get_log_line        = tap_control_code(8, 0)
	tap_win_ioctl_config_dhcp_set_opt = tap_control_code(9, 0)
	tap_ioctl_config_tun              = tap_control_code(10, 0)
	// w32 api
	file_device_unknown = uint32(0x00000022)
	// Driver maker specified ComponentId
	// ComponentId is defined here: https://github.com/OpenVPN/tap-windows6/blob/master/version.m4#L5
	componentId = "tap0901"
)

func ctl_code(device_type, function, method, access uint32) uint32 {
	return (device_type << 16) | (access << 14) | (function << 2) | method
}

func tap_control_code(request, method uint32) uint32 {
	return ctl_code(file_device_unknown, request, method, 0)
}

// getdeviceid finds out a TAP device from registry, it *may* requires privileged right to prevent some weird issue.
func getdeviceid() (string, error) {
	// TAP driver key location
	regkey := `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, regkey, registry.READ)
	if err != nil {
		return "", RegistryOpenErr
	}
	defer k.Close()
	// read all subkeys, it should not return an err here
	keys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return "", err
	}
	// find the one matched ComponentId
	for _, v := range keys {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, regkey+"\\"+v, registry.READ)
		if err != nil {
			continue
		}
		val, _, err := key.GetStringValue("ComponentId")
		if err != nil {
			key.Close()
			continue
		}
		if val == componentId {
			val, _, err = key.GetStringValue("NetCfgInstanceId")
			if err != nil {
				key.Close()
				continue
			}
			key.Close()
			return val, nil
		}
		key.Close()
	}
	return "", TapDeviceNotFound
}

// openDev find and open an interface.
func openDev(isTAP bool) (ifce *Interface, err error) {
	// ifName won't work
	// find the device in registry.
	deviceid, err := getdeviceid()
	if err != nil {
		return nil, err
	}
	path := "\\\\.\\Global\\" + deviceid + ".tap"
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// type Handle uintptr
	file, err := syscall.CreateFile(pathp, syscall.GENERIC_READ|syscall.GENERIC_WRITE, uint32(syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE), nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_SYSTEM, 0)
	// if err hanppens, close the interface.
	defer func() {
		if err != nil {
			syscall.Close(file)
		}
		if err := recover(); err != nil {
			syscall.Close(file)
		}
	}()
	if err != nil {
		return nil, err
	}
	var bytesReturned uint32
	rdbbuf := make([]byte, syscall.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)

	//TUN
	if !isTAP {
		code2 := []byte{0x0a, 0x03, 0x00, 0x01, 0x0a, 0x03, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00}
		err = syscall.DeviceIoControl(file, tap_ioctl_config_tun, &code2[0], uint32(12), &rdbbuf[0], uint32(len(rdbbuf)), &bytesReturned, nil)
		if err != nil {
			return
		}
	}

	// find the mac address of tap device, use this to find the name of interface
	mac := make([]byte, 6)
	err = syscall.DeviceIoControl(file, tap_win_ioctl_get_mac, &mac[0], uint32(len(mac)), &mac[0], uint32(len(mac)), &bytesReturned, nil)
	if err != nil {
		return nil, err
	}

	fd := os.NewFile(uintptr(file), path)
	ifce = &Interface{isTAP: isTAP, ReadWriteCloser: fd}

	// bring up device.
	code := []byte{0x01, 0x00, 0x00, 0x00}
	err = syscall.DeviceIoControl(file, tap_ioctl_set_media_status, &code[0], uint32(4), &rdbbuf[0], uint32(len(rdbbuf)), &bytesReturned, nil)
	if err != nil {
		return
	}
	// find the name of tap interface(u need it to set the ip or other command)
	ifces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, v := range ifces {
		if bytes.Equal(v.HardwareAddr[:6], mac[:6]) {
			ifce.name = v.Name
			return
		}
	}

	err = IfceNameNotFound
	return
}

func newTAP(ifName string) (ifce *Interface, err error) {
	// ifName won't work
	return openDev(true)
}

func newTUN(ifName string) (ifce *Interface, err error) {
	// ifName won't work
	return openDev(false)
}
