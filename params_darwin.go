package water

const (
	// SystemDriver refers to the default P2P driver
	SystemDriver = 0
	// TunTapOSXDriver refers to the third-party tuntaposx driver
	// see https://sourceforge.net/p/tuntaposx
	TunTapOSXDriver = 1
)

// PlatformSpecificParams defines parameters in Config that are specific to
// macOS. A zero-value of such type is valid, yielding an interface
// with OS defined name.
// Currently it is not possible to set the interface name in macOS.
type PlatformSpecificParams struct {
	// Name is the name for the interface to be used.
	// e.g. "tap0"
	// Only valid if using TunTapOSXDriver.
	Name string
	// Driver should be set if an alternative driver is desired
	// e.g. TunTapOSXDriver
	Driver int
}

func defaultPlatformSpecificParams() PlatformSpecificParams {
	return PlatformSpecificParams{}
}
