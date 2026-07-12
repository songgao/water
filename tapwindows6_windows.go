package water

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// tap-windows6 support. The driver ships with OpenVPN; it can also be installed
// on its own from https://github.com/OpenVPN/tap-windows6.

const (
	defaultComponentID = "tap0901"
	defaultNetwork     = "192.168.1.10/24"

	// tapDriverKey is the location of the TAP driver key.
	tapDriverKey = `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`
	// netConfigKey is the location of the TAP adapter's network config.
	netConfigKey = `SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}`

	fileDeviceUnknown = uint32(0x00000022)
)

// Device control codes, as defined by tap-windows6's tap-windows.h.
var (
	tapIoctlGetMac         = tapControlCode(1, 0)
	tapIoctlGetVersion     = tapControlCode(2, 0)
	tapIoctlSetMediaStatus = tapControlCode(6, 0)
	tapIoctlConfigTun      = tapControlCode(10, 0)
)

func ctlCode(deviceType, function, method, access uint32) uint32 {
	return (deviceType << 16) | (access << 14) | (function << 2) | method
}

func tapControlCode(request, method uint32) uint32 {
	return ctlCode(fileDeviceUnknown, request, method, 0)
}

// tapDevice is a tap-windows6 adapter opened for overlapped I/O.
type tapDevice struct {
	fd windows.Handle

	// Read and Write each own an OVERLAPPED with its own event, so that a read
	// and a write may be in flight at the same time. Concurrent readers are
	// serialised by rMu, and concurrent writers by wMu, because an OVERLAPPED
	// may only describe one operation at a time.
	rMu sync.Mutex
	ro  *windows.Overlapped
	wMu sync.Mutex
	wo  *windows.Overlapped

	gate ioGate
}

// tapIoctl issues a synchronous DeviceIoControl on an overlapped handle.
//
// The device is opened with FILE_FLAG_OVERLAPPED, and passing a NULL OVERLAPPED
// to DeviceIoControl on such a handle is undefined behaviour: the call may
// return ERROR_IO_PENDING and go on writing into the buffers after it has
// returned. So supply an OVERLAPPED and wait for the completion ourselves.
func tapIoctl(fd windows.Handle, code uint32, in, out []byte) error {
	event, err := windows.CreateEvent(nil, 1 /* manual reset */, 0 /* non-signalled */, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(event)
	overlapped := &windows.Overlapped{HEvent: event}

	var inPtr, outPtr *byte
	if len(in) > 0 {
		inPtr = &in[0]
	}
	if len(out) > 0 {
		outPtr = &out[0]
	}

	var returned uint32
	err = windows.DeviceIoControl(fd, code,
		inPtr, uint32(len(in)),
		outPtr, uint32(len(out)),
		&returned, overlapped)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		err = windows.GetOverlappedResult(fd, overlapped, &returned, true /* wait */)
	}
	return err
}

// setMediaStatus connects or disconnects the adapter's virtual cable, which is
// what makes Windows show the adapter as up.
func setMediaStatus(fd windows.Handle, up bool) error {
	status := make([]byte, 4)
	if up {
		status[0] = 0x01
	}
	// The driver expects the same buffer for input and output, as OpenVPN does.
	return tapIoctl(fd, tapIoctlSetMediaStatus, status, status)
}

// configureTun puts the adapter into TUN mode. tap-windows6 is an Ethernet
// device, so to emulate a point-to-point layer 3 device it answers ARP on the
// kernel's behalf, and for that it needs to be told the local address and the
// subnet it is pretending to be attached to.
func configureTun(fd windows.Handle, network string) error {
	localIP, remoteNet, err := net.ParseCIDR(network)
	if err != nil {
		return fmt.Errorf("water: parsing Network %q: %w", network, err)
	}
	local4, remote4, mask4 := localIP.To4(), remoteNet.IP.To4(), net.IP(remoteNet.Mask).To4()
	if local4 == nil || remote4 == nil || mask4 == nil {
		return fmt.Errorf("water: Network %q is not IPv4; tap-windows6 TUN mode requires an IPv4 CIDR", network)
	}

	ep := make([]byte, 0, 12)
	ep = append(ep, local4...)
	ep = append(ep, remote4...)
	ep = append(ep, mask4...)

	return tapIoctl(fd, tapIoctlConfigTun, ep, ep)
}

// driverVersion reports the tap-windows6 version, and doubles as a check that
// the handle really does belong to a TAP adapter.
func driverVersion(fd windows.Handle) (major, minor uint32, err error) {
	version := make([]byte, 12)
	if err := tapIoctl(fd, tapIoctlGetVersion, version, version); err != nil {
		return 0, 0, err
	}
	le := func(b []byte) uint32 {
		return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	}
	return le(version[0:4]), le(version[4:8]), nil
}

// findTapDevice locates an installed tap-windows6 adapter in the registry and
// returns its NetCfgInstanceId -- the GUID that names the device object -- along
// with the adapter's friendly name, when it has one.
func findTapDevice(componentID, interfaceName string) (guid, name string, err error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, tapDriverKey, registry.READ)
	if err != nil {
		return "", "", fmt.Errorf("%w: opening the adapter registry key, tap-windows6 may not be installed: %v",
			errDriverUnavailable, err)
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return "", "", fmt.Errorf("water: enumerating network adapters: %w", err)
	}

	for _, subkey := range subkeys {
		guid, name, ok := inspectAdapter(subkey, componentID, interfaceName)
		if ok {
			return guid, name, nil
		}
	}

	if interfaceName != "" {
		return "", "", fmt.Errorf("%w: no adapter with ComponentId %q and name %q; "+
			"the adapter may not be installed, or the name may be wrong "+
			"(check `netsh interface show interface`)",
			errDriverUnavailable, componentID, interfaceName)
	}
	return "", "", fmt.Errorf("%w: no adapter with ComponentId %q; tap-windows6 may not be installed",
		errDriverUnavailable, componentID)
}

// inspectAdapter reports whether one adapter subkey is the adapter we want.
func inspectAdapter(subkey, componentID, interfaceName string) (guid, name string, ok bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, tapDriverKey+`\`+subkey, registry.READ)
	if err != nil {
		return "", "", false
	}
	defer key.Close()

	if id, _, err := key.GetStringValue("ComponentId"); err != nil || id != componentID {
		return "", "", false
	}
	guid, _, err = key.GetStringValue("NetCfgInstanceId")
	if err != nil {
		return "", "", false
	}

	// An adapter that is mid-install may not have a Connection\Name yet. That is
	// only fatal if the caller asked for an adapter by name.
	name, err = adapterName(guid)
	if err != nil {
		return guid, "", interfaceName == ""
	}
	return guid, name, interfaceName == "" || name == interfaceName
}

// adapterName reads an adapter's friendly name -- the name shown in Control
// Panel, and the name netsh expects.
func adapterName(guid string) (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, netConfigKey+`\`+guid+`\Connection`, registry.READ)
	if err != nil {
		return "", err
	}
	defer k.Close()

	name, _, err := k.GetStringValue("Name")
	return name, err
}

// adapterNameByMAC finds the adapter's name by matching the MAC address the
// driver reports against the system's interface list. It is a fallback for when
// the registry has no friendly name for the adapter.
func adapterNameByMAC(fd windows.Handle) (string, error) {
	mac := make([]byte, 6)
	if err := tapIoctl(fd, tapIoctlGetMac, mac, mac); err != nil {
		return "", fmt.Errorf("water: reading the adapter's MAC address: %w", err)
	}

	ifces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("water: listing network interfaces: %w", err)
	}
	for _, ifce := range ifces {
		// Miniport and VPN adapters sometimes report an empty HardwareAddr.
		if len(ifce.HardwareAddr) >= 6 && bytes.Equal(ifce.HardwareAddr[:6], mac) {
			return ifce.Name, nil
		}
	}
	return "", fmt.Errorf("water: no network interface has the adapter's MAC address %x", mac)
}

func openDevTapWindows6(config Config) (*Interface, error) {
	guid, name, err := findTapDevice(config.ComponentID, config.InterfaceName)
	if err != nil {
		return nil, err
	}

	path, err := windows.UTF16PtrFromString(`\\.\Global\` + guid + `.tap`)
	if err != nil {
		return nil, err
	}
	fd, err := windows.CreateFile(path,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_SYSTEM|windows.FILE_FLAG_OVERLAPPED,
		0)
	if err != nil {
		return nil, fmt.Errorf("water: opening adapter %q: %w "+
			"(the adapter may already be in use by another process, and creating one needs administrator rights)",
			guid, err)
	}

	// Unwind everything opened here unless we hand it to the caller.
	var cleanup []func()
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := len(cleanup) - 1; i >= 0; i-- {
			cleanup[i]()
		}
	}()
	cleanup = append(cleanup, func() { windows.CloseHandle(fd) })

	if _, _, err := driverVersion(fd); err != nil {
		return nil, fmt.Errorf("water: %q does not look like a tap-windows6 adapter: %w", guid, err)
	}

	// Set the mode before bringing the link up, not after: connecting the cable
	// is what lets the stack start pushing frames at us, and until CONFIG_TUN
	// lands the driver is still in TAP mode and would hand them to us as
	// Ethernet frames.
	if config.DeviceType == TUN {
		if err := configureTun(fd, config.Network); err != nil {
			return nil, fmt.Errorf("water: putting the adapter into TUN mode: %w", err)
		}
	}
	if err := setMediaStatus(fd, true); err != nil {
		return nil, fmt.Errorf("water: bringing the adapter up: %w", err)
	}
	cleanup = append(cleanup, func() { setMediaStatus(fd, false) })

	if name == "" {
		if name, err = adapterNameByMAC(fd); err != nil {
			return nil, err
		}
	}

	ro, err := newOverlapped()
	if err != nil {
		return nil, err
	}
	cleanup = append(cleanup, func() { windows.CloseHandle(ro.HEvent) })

	wo, err := newOverlapped()
	if err != nil {
		return nil, err
	}
	cleanup = append(cleanup, func() { windows.CloseHandle(wo.HEvent) })

	committed = true
	return &Interface{
		isTAP:           config.DeviceType == TAP,
		ReadWriteCloser: &tapDevice{fd: fd, ro: ro, wo: wo},
		name:            name,
	}, nil
}

func newOverlapped() (*windows.Overlapped, error) {
	event, err := windows.CreateEvent(nil, 1 /* manual reset */, 0 /* non-signalled */, nil)
	if err != nil {
		return nil, fmt.Errorf("water: creating an I/O completion event: %w", err)
	}
	return &windows.Overlapped{HEvent: event}, nil
}

func (d *tapDevice) Read(b []byte) (int, error) {
	if err := d.gate.enter(); err != nil {
		return 0, err
	}
	defer d.gate.leave()

	d.rMu.Lock()
	defer d.rMu.Unlock()

	if err := windows.ResetEvent(d.ro.HEvent); err != nil {
		return 0, err
	}

	var done uint32
	err := windows.ReadFile(d.fd, b, &done, d.ro)
	return d.await(d.ro, err, &done)
}

func (d *tapDevice) Write(b []byte) (int, error) {
	if err := d.gate.enter(); err != nil {
		return 0, err
	}
	defer d.gate.leave()

	d.wMu.Lock()
	defer d.wMu.Unlock()

	if err := windows.ResetEvent(d.wo.HEvent); err != nil {
		return 0, err
	}

	var done uint32
	err := windows.WriteFile(d.fd, b, &done, d.wo)
	return d.await(d.wo, err, &done)
}

// await blocks until an overlapped operation completes, and reports how much of
// it got through.
//
// Re-checking the gate once the operation is in flight is what closes the race
// between Close and an operation that had already passed the gate but had not
// yet reached the OS. Without it, Close's CancelIoEx could run in that window,
// find nothing to cancel, and then dead-lock in running.Wait() behind a read
// that parks forever waiting for a packet nobody will send.
//
// The two orderings both end with the operation cancelled. Either this call sees
// the gate closing and cancels the operation itself, or it does not -- and then
// its check preceded Close's, so the operation was already in flight by the time
// Close ran CancelIoEx, which therefore found and cancelled it.
func (d *tapDevice) await(o *windows.Overlapped, err error, done *uint32) (int, error) {
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		if d.gate.closing() {
			windows.CancelIoEx(d.fd, o)
		}
		err = windows.GetOverlappedResult(d.fd, o, done, true /* wait */)
	}
	if err != nil {
		return int(*done), readWriteError(err)
	}
	return int(*done), nil
}

// readWriteError maps the ways the OS reports "the device went away underneath
// you" onto os.ErrClosed, so that callers can treat a closed device uniformly
// with errors.Is(err, os.ErrClosed) on every platform.
func readWriteError(err error) error {
	switch {
	case errors.Is(err, windows.ERROR_OPERATION_ABORTED): // CancelIoEx, from Close
		return os.ErrClosed
	case errors.Is(err, windows.ERROR_INVALID_HANDLE):
		return os.ErrClosed
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND): // adapter unplugged
		return os.ErrClosed
	}
	return err
}

// Close brings the adapter down and releases it. It may be called while a Read
// or a Write is in flight, and it may be called more than once.
func (d *tapDevice) Close() error {
	return d.gate.shutdown(
		// CancelIoEx with a nil OVERLAPPED cancels every I/O request this
		// process has in flight on the handle, whichever thread issued it. That
		// is what releases a Read parked on an interface that is not receiving
		// any traffic -- without it, Close would block behind a packet that may
		// never arrive.
		func() { windows.CancelIoEx(d.fd, nil) },
		func() error {
			setMediaStatus(d.fd, false)
			err := windows.CloseHandle(d.fd)
			windows.CloseHandle(d.ro.HEvent)
			windows.CloseHandle(d.wo.HEvent)
			return err
		},
	)
}
