package water

import "io"

// Interface is a TUN/TAP interface.
type Interface struct {
	isTAP bool
	io.ReadWriteCloser
	name string
}

// DeviceType is the type for specifying device types.
type DeviceType int

// TUN and TAP device types.
const (
	_ = iota
	TUN
	TAP
)

// Config defines parameters required to create a TUN/TAP interface. It's only
// used when the device is initialized. A zero-value Config is a valid
// configuration.
type Config struct {
	// DeviceType specifies whether the device is a TUN or TAP interface. A
	// zero-value is treated as TUN.
	DeviceType DeviceType

	// PlatformSpecificParams defines parameters that differ on different
	// platforms. See comments for the type for more details.
	PlatformSpecificParams
}

func defaultConfig() Config {
	return Config{
		DeviceType:             TUN,
		PlatformSpecificParams: defaultPlatformSpecificParams(),
	}
}

var zeroConfig Config

// New creates a new TUN/TAP interface using config.
func New(config Config) (ifce *Interface, err error) {
	if zeroConfig == config {
		config = defaultConfig()
	}
	return newDev(config)
}

// NewTAP creates a new TAP interface whose name is ifName. If ifName is empty, a
// default name (tap0, tap1, ... ) will be assigned. ifName should not exceed
// 16 bytes. TAP interfaces are not supported on darwin.
//
// Note: this function is deprecated and will be removed from the library.
// Please use New() instead.
func NewTAP(ifName string) (ifce *Interface, err error) {
	return newTAP(ifName)
}

// NewTUN creates a new TUN interface whose name is ifName. If ifName is empty, a
// default name (tap0, tap1, ... ) will be assigned. ifName should not exceed
//
// Note: this function is deprecated and will be removed from the library.
// Please use New() instead.
// 16 bytes. Setting interface name is NOT supported on darwin.
func NewTUN(ifName string) (ifce *Interface, err error) {
	return newTUN(ifName)
}

// IsTUN returns true if ifce is a TUN interface.
func (ifce *Interface) IsTUN() bool {
	return !ifce.isTAP
}

// IsTAP returns true if ifce is a TAP interface.
func (ifce *Interface) IsTAP() bool {
	return ifce.isTAP
}

// Name returns the interface name of ifce, e.g. tun0, tap1, tun0, etc..
func (ifce *Interface) Name() string {
	return ifce.name
}
