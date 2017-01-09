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
}

func defaultPlatformSpecificParams() PlatformSpecificParams {
	return PlatformSpecificParams{
		ComponentId: "tap0901",
	}
}
