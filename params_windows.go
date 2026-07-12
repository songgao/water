package water

// WindowsDriverProvider enumerates the TUN/TAP drivers that water can use on
// Windows.
type WindowsDriverProvider int

const (
	// WindowsDriverAuto lets water pick a driver. A TUN device is created with
	// Wintun when wintun.dll can be loaded, and with tap-windows6 otherwise. A
	// TAP device is always created with tap-windows6, as Wintun cannot do
	// layer 2.
	//
	// This is the zero value, so it is what a zero-value Config gets.
	WindowsDriverAuto WindowsDriverProvider = iota

	// WindowsDriverWintun uses the Wintun driver (https://www.wintun.net), the
	// TUN driver developed for WireGuard. It is layer 3 only: asking for a TAP
	// device with this driver is an error.
	//
	// Wintun needs wintun.dll to be loadable at run time and the process to be
	// running with administrator rights. Unlike tap-windows6 it needs no
	// pre-installed adapter: the adapter is created by New() and removed again
	// by Close().
	WindowsDriverWintun

	// WindowsDriverTapWindows6 uses the OpenVPN tap-windows6 driver, which is
	// the driver water has always used on Windows. It supports both TAP and
	// TUN, but the adapter has to be installed beforehand (installing OpenVPN
	// does that), and its TUN mode is emulated on top of layer 2, which is why
	// Network has to be set for it.
	WindowsDriverTapWindows6
)

// PlatformSpecificParams defines parameters in Config that are specific to
// Windows. A zero-value of such type is valid.
type PlatformSpecificParams struct {
	// ComponentID associates with the virtual adapter that exists in Windows.
	// This is usually configured when driver for the adapter is installed. A
	// zero-value of this field, i.e., an empty string, causes the interface to
	// use the default ComponentId. The default ComponentId is set to tap0901,
	// the one used by OpenVPN.
	//
	// Only used by the tap-windows6 driver.
	ComponentID string

	// InterfaceName names the network adapter.
	//
	// For tap-windows6 it selects among the installed adapters: it is the
	// friendly name of the adapter as shown in Control Panel, which matters
	// when several tap0901 adapters exist on the same system. An empty value
	// picks the first adapter matching ComponentID.
	//
	// For Wintun it is the name given to the adapter that water creates. An
	// empty value picks the first free name of the form water0, water1, ...
	//
	// Either way Interface.Name() reports the name the adapter ended up with,
	// which is the name to hand to `netsh interface ip set address name=...`.
	InterfaceName string

	// Network is required when creating a TUN interface with tap-windows6. The
	// library will call net.ParseCIDR() to parse this string into LocalIP,
	// RemoteNetaddr, RemoteNetmask. The underlying driver will need those to
	// generate ARP response to Windows kernel, to emulate an TUN interface.
	// Please note that it cannot perceive the IP changes caused by DHCP, user
	// configuration to the adapter and etc,. If IP changed, please reconfigure
	// the adapter using syscall, just like openDev().
	// For detail, please refer
	// https://github.com/OpenVPN/tap-windows6/blob/master/src/device.c#L431
	// and https://github.com/songgao/water/pull/13#issuecomment-270341777
	//
	// Wintun is a real layer 3 device and ignores this field.
	Network string

	// Driver selects the Windows driver backend. A zero-value means
	// WindowsDriverAuto.
	Driver WindowsDriverProvider

	// RingCapacity is the size in bytes of each of Wintun's send and receive
	// ring buffers. It must be a power of two between 128KiB and 64MiB. A
	// zero-value means 4MiB.
	//
	// Bigger rings absorb bigger bursts before packets start being dropped, at
	// the cost of non-paged pool. Only used by the Wintun driver.
	RingCapacity int
}

func defaultPlatformSpecificParams() PlatformSpecificParams {
	return PlatformSpecificParams{
		ComponentID: defaultComponentID,
		Network:     defaultNetwork,
	}
}
