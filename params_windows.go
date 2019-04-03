package water

// PlatformSpecificParams defines parameters in Config that are specific to
// Windows. A zero-value of such type is valid.
type PlatformSpecificParams struct {
	// ComponentID associates with the virtual adapter that exists in Windows.
	// This is usually configured when driver for the adapter is installed. A
	// zero-value of this field, i.e., an empty string, causes the interface to
	// use the default ComponentId. The default ComponentId is set to tap0901,
	// the one used by OpenVPN.
	ComponentID string
	// InterfaceName is a friendly name of the network adapter as set in Control Panel.
	// Of course, you may have multiple tap0901 adapters on the system, in which
	// case we need a friendlier way to identify them.
	InterfaceName string
	// Network is required when creating a TUN interface. The library will call
	// net.ParseCIDR() to parse this string into LocalIP, RemoteNetaddr,
	// RemoteNetmask. The underlying driver will need those to generate ARP
	// response to Windows kernel, to emulate an TUN interface.
	// Please note that Network must be same with IP and Mask that configured manually.
	Network string

	DHCPServer string
	DNS1       string
	DNS2       string
	// Configure IP and DNS by device DHCP
	IsDHCP bool
}

func defaultPlatformSpecificParams() PlatformSpecificParams {
	return PlatformSpecificParams{
		ComponentID: "tap0901",
		Network:     "10.1.0.10/24",
	}
}
