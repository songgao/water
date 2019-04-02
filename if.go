package water

import (
	"errors"
	"io"
)

// Interface represents a TUN/TAP interface.
type Interface interface {
	// Use Read() and Write() methods to read from and write into this TUN/TAP
	// interface. In TAP interface, each call corresponds to an Ethernet frame.
	// In TUN mode, each call corresponds to an IP packet.
	io.ReadWriteCloser

	// IsTUN returns true if ifce is a TUN interface.
	IsTUN() bool
	// IsTAP returns true if ifce is a TAP interface.
	IsTAP() bool
	// DeviceType returns the interface's device type.
	Type() DeviceType
	// Name returns the name of the interface.
	Name() string
	// Sys returns the underlying system interface for the interface. This is
	// useful if caller needs to perform system calls directly on the tun/tap
	// device.
	//
	// On Unix systems, this returns a file descriptor of uintptr type. On
	// Windows, this returns a syscall.Handle.
	Sys() interface{}
}

// DeviceType is the type for specifying device types.
type DeviceType int

// Constants for TUN and TAP interfaces.
const (
	_ DeviceType = iota
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
func New(config Config) (ifce Interface, err error) {
	if zeroConfig == config {
		config = defaultConfig()
	}
	if config.PlatformSpecificParams == zeroConfig.PlatformSpecificParams {
		config.PlatformSpecificParams = defaultPlatformSpecificParams()
	}
	switch config.DeviceType {
	case TUN, TAP:
		return openDev(config)
	default:
		return nil, errors.New("unknown device type")
	}
}
