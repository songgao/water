// +build linux

package water

// PlatformSpecificParams defines parameters in Config that are specific to
// Linux. A zero-value of such type is valid, yielding an interface
// with OS defined name.
type PlatformSpecificParams struct {
	// Name is the name to be set for the interface to be created. This overrides
	// the default name assigned by OS such as tap0 or tun0. A zero-value of this
	// field, i.e. an empty string, indicates that the default name should be
	// used.
	Name string

	// Enable or disable persistence mode for the interface device.
	Persist bool

	// ID of the user which will be granted ownership of the device.
	// The default value of -1 specifies that any user may use the device.
	Owner int

	// ID of the group which will be granted access to the device.
	// The default value of -1 specifies that any group may use the device.
	Group int
}

func defaultPlatformSpecificParams() PlatformSpecificParams {
	return PlatformSpecificParams{
		Owner: -1,
		Group: -1,
	}
}
