package water

// DevicePermissions determines the owner and group owner for the newly created
// interface.
//
// To leave both as Linux sets them -- owner and group -1, meaning any user and
// any group -- leave Config.Permissions nil. These fields are unsigned, so there
// is no in-band way to say "do not change this one"; setting Permissions at all
// sets both.
type DevicePermissions struct {
	// Owner is the ID of the user which will be granted ownership of the
	// device.
	Owner uint

	// Group is the ID of the group which will be granted access to the device.
	Group uint
}

// PlatformSpecificParams defines parameters in Config that are specific to
// Linux. A zero-value of such type is valid, yielding an interface
// with OS defined name.
type PlatformSpecificParams struct {
	// Name is the name to be set for the interface to be created. This overrides
	// the default name assigned by OS such as tap0 or tun0. A zero-value of this
	// field, i.e. an empty string, indicates that the default name should be
	// used.
	Name string

	// Persist specifies whether persistence mode for the interface device
	// should be enabled or disabled.
	Persist bool

	// Permissions, if non-nil, specifies the owner and group owner for the
	// interface.  A zero-value of this field, i.e. nil, indicates that no
	// changes to owner or group will be made.
	Permissions *DevicePermissions

	// MultiQueue specifies whether the multiqueue flag should be set on the
	// interface.  From version 3.8, Linux supports multiqueue tuntap which can
	// uses multiple file descriptors (queues) to parallelize packets sending
	// or receiving.
	MultiQueue bool
}

func defaultPlatformSpecificParams() PlatformSpecificParams {
	return PlatformSpecificParams{}
}
