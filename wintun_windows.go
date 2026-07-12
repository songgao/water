package water

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Wintun support. Wintun (https://www.wintun.net) is the layer 3 TUN driver
// written for WireGuard. Compared with tap-windows6 it is kernel-signed, needs
// no adapter to be installed beforehand -- water creates the adapter on New()
// and removes it again on Close() -- and moves packets over a pair of shared
// memory rings rather than through overlapped file I/O.
//
// Deploying it means shipping wintun.dll next to the executable; the driver
// itself is embedded in the DLL and installed on first use. Creating an adapter
// requires administrator rights.
//
// The API bound here is documented at https://git.zx2c4.com/wintun/about/.

const (
	// Ring capacity must be a power of two within these bounds.
	wintunMinRingCapacity     = 0x20000   // 128 KiB
	wintunMaxRingCapacity     = 0x4000000 // 64 MiB
	wintunDefaultRingCapacity = 0x400000  // 4 MiB

	// wintunMaxPacketSize is the largest packet the driver will carry.
	wintunMaxPacketSize = 0xffff

	// wintunTunnelType is what Windows reports for adapters water creates.
	wintunTunnelType = "water"

	// wintunMaxAdapterName is MAX_ADAPTER_NAME from wintun.h, in UTF-16 code
	// units, including the terminating NUL.
	wintunMaxAdapterName = 128
)

// wintunDLL holds the entry points of a loaded wintun.dll.
type wintunDLL struct {
	createAdapter        uintptr
	closeAdapter         uintptr
	getRunningVersion    uintptr
	startSession         uintptr
	endSession           uintptr
	getReadWaitEvent     uintptr
	receivePacket        uintptr
	releaseReceivePacket uintptr
	allocateSendPacket   uintptr
	sendPacket           uintptr
}

var (
	wintunOnce sync.Once
	wintunAPI  *wintunDLL
	wintunErr  error
)

// loadWintun loads wintun.dll once per process and resolves its entry points.
//
// The DLL is searched for in the application directory, System32 and the
// directories registered with AddDllDirectory -- deliberately not in the current
// working directory, which a bare LoadLibrary would also search and which an
// attacker may be able to write to.
func loadWintun() (*wintunDLL, error) {
	wintunOnce.Do(func() {
		const searchFlags = windows.LOAD_LIBRARY_SEARCH_APPLICATION_DIR |
			windows.LOAD_LIBRARY_SEARCH_SYSTEM32 |
			windows.LOAD_LIBRARY_SEARCH_USER_DIRS

		module, err := windows.LoadLibraryEx("wintun.dll", 0, searchFlags)
		if err != nil {
			wintunErr = fmt.Errorf("%w: loading wintun.dll: %v "+
				"(download it from https://www.wintun.net and place wintun.dll next to your executable)",
				errDriverUnavailable, err)
			return
		}

		api := &wintunDLL{}
		for _, proc := range []struct {
			name string
			addr *uintptr
		}{
			{"WintunCreateAdapter", &api.createAdapter},
			{"WintunCloseAdapter", &api.closeAdapter},
			{"WintunGetRunningDriverVersion", &api.getRunningVersion},
			{"WintunStartSession", &api.startSession},
			{"WintunEndSession", &api.endSession},
			{"WintunGetReadWaitEvent", &api.getReadWaitEvent},
			{"WintunReceivePacket", &api.receivePacket},
			{"WintunReleaseReceivePacket", &api.releaseReceivePacket},
			{"WintunAllocateSendPacket", &api.allocateSendPacket},
			{"WintunSendPacket", &api.sendPacket},
		} {
			addr, err := windows.GetProcAddress(module, proc.name)
			if err != nil {
				wintunErr = fmt.Errorf("%w: wintun.dll has no %s, it is probably too old: %v",
					errDriverUnavailable, proc.name, err)
				return
			}
			*proc.addr = addr
		}
		wintunAPI = api
	})
	return wintunAPI, wintunErr
}

// errnoOr turns a failed Win32 call into an error, covering the case where the
// call reported failure but left GetLastError at zero.
func errnoOr(errno syscall.Errno, fallback string) error {
	if errno != 0 {
		return errno
	}
	return errors.New(fallback)
}

func (d *wintunDLL) CreateAdapter(name string) (windows.Handle, error) {
	name16, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	type16, err := windows.UTF16PtrFromString(wintunTunnelType)
	if err != nil {
		return 0, err
	}
	// The third argument is a requested adapter GUID; NULL asks Wintun to derive
	// a stable one from the name.
	adapter, _, errno := syscall.SyscallN(d.createAdapter,
		uintptr(unsafe.Pointer(name16)),
		uintptr(unsafe.Pointer(type16)),
		0)
	runtime.KeepAlive(name16)
	runtime.KeepAlive(type16)
	if adapter == 0 {
		return 0, errnoOr(errno, "WintunCreateAdapter failed")
	}
	return windows.Handle(adapter), nil
}

// CloseAdapter ends the adapter's lifetime, removing it from the system.
func (d *wintunDLL) CloseAdapter(adapter windows.Handle) {
	syscall.SyscallN(d.closeAdapter, uintptr(adapter))
}

func (d *wintunDLL) RunningDriverVersion() (major, minor uint16) {
	version, _, _ := syscall.SyscallN(d.getRunningVersion)
	return uint16(version >> 16), uint16(version & 0xffff)
}

func (d *wintunDLL) StartSession(adapter windows.Handle, capacity uint32) (windows.Handle, error) {
	session, _, errno := syscall.SyscallN(d.startSession, uintptr(adapter), uintptr(capacity))
	if session == 0 {
		return 0, errnoOr(errno, "WintunStartSession failed")
	}
	return windows.Handle(session), nil
}

func (d *wintunDLL) EndSession(session windows.Handle) {
	syscall.SyscallN(d.endSession, uintptr(session))
}

// ReadWaitEvent returns the event Wintun signals when the receive ring goes from
// empty to non-empty.
func (d *wintunDLL) ReadWaitEvent(session windows.Handle) windows.Handle {
	event, _, _ := syscall.SyscallN(d.getReadWaitEvent, uintptr(session))
	return windows.Handle(event)
}

// ReceivePacket returns a pointer into the receive ring's shared memory, along
// with the length of the packet sitting there. The caller must copy what it
// needs and then hand the pointer back to ReleaseReceivePacket.
func (d *wintunDLL) ReceivePacket(session windows.Handle) (packet uintptr, size uint32, err error) {
	packet, _, errno := syscall.SyscallN(d.receivePacket,
		uintptr(session),
		uintptr(unsafe.Pointer(&size)))
	if packet == 0 {
		return 0, 0, errnoOr(errno, "WintunReceivePacket failed")
	}
	return packet, size, nil
}

func (d *wintunDLL) ReleaseReceivePacket(session windows.Handle, packet uintptr) {
	syscall.SyscallN(d.releaseReceivePacket, uintptr(session), packet)
}

// AllocateSendPacket reserves space in the send ring and returns a pointer to
// it. Fill it in, then hand the pointer to SendPacket.
func (d *wintunDLL) AllocateSendPacket(session windows.Handle, size int) (uintptr, error) {
	packet, _, errno := syscall.SyscallN(d.allocateSendPacket,
		uintptr(session),
		uintptr(uint32(size)))
	if packet == 0 {
		return 0, errnoOr(errno, "WintunAllocateSendPacket failed")
	}
	return packet, nil
}

func (d *wintunDLL) SendPacket(session windows.Handle, packet uintptr) {
	syscall.SyscallN(d.sendPacket, uintptr(session), packet)
}

// ringMemory turns a pointer into one of Wintun's rings into a slice.
//
// The address comes from the driver, and points into a shared mapping rather
// than into the Go heap: it never moves and the collector has no stake in it, so
// the conversion is safe even though it is the pattern `go vet -unsafeptr` warns
// about. It cannot tell foreign memory from a stale Go address, which is why the
// Makefile's vet target turns that one check off. The slice must not outlive the
// Release or Send call that hands the packet back to the driver.
func ringMemory(packet uintptr, size int) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(packet)), size)
}

// wintunDevice is a Wintun adapter with an open session.
type wintunDevice struct {
	api     *wintunDLL
	adapter windows.Handle
	session windows.Handle

	// readWait is signalled by the driver when a packet arrives, and by Close to
	// release a reader waiting for one that never will.
	readWait windows.Handle

	rMu  sync.Mutex
	wMu  sync.Mutex
	gate ioGate
}

func openDevWintun(config Config) (*Interface, error) {
	api, err := loadWintun()
	if err != nil {
		return nil, err
	}

	capacity, err := ringCapacity(config.RingCapacity)
	if err != nil {
		return nil, err
	}

	adapter, name, err := createAdapter(api, config.InterfaceName)
	if err != nil {
		return nil, err
	}

	session, err := api.StartSession(adapter, capacity)
	if err != nil {
		api.CloseAdapter(adapter)
		return nil, fmt.Errorf("water: starting a Wintun session on %q: %w", name, err)
	}

	return &Interface{
		isTAP: false,
		name:  name,
		ReadWriteCloser: &wintunDevice{
			api:      api,
			adapter:  adapter,
			session:  session,
			readWait: api.ReadWaitEvent(session),
		},
	}, nil
}

// createAdapter creates the adapter, choosing a name if the caller did not.
func createAdapter(api *wintunDLL, requested string) (windows.Handle, string, error) {
	if requested != "" {
		if len(requested) >= wintunMaxAdapterName {
			return 0, "", fmt.Errorf("water: InterfaceName %q is too long, the limit is %d characters",
				requested, wintunMaxAdapterName-1)
		}
		adapter, err := api.CreateAdapter(requested)
		if err != nil {
			return 0, "", adapterError(api, err)
		}
		return adapter, requested, nil
	}

	// A Wintun adapter is owned by the process that created it, so a name that
	// is already taken normally means another live process is using it. Step
	// over those rather than failing.
	var err error
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("water%d", i)
		var adapter windows.Handle
		if adapter, err = api.CreateAdapter(name); err == nil {
			return adapter, name, nil
		}
	}
	return 0, "", adapterError(api, err)
}

func adapterError(api *wintunDLL, err error) error {
	major, minor := api.RunningDriverVersion()
	return fmt.Errorf("water: creating a Wintun adapter failed (driver %d.%d): %w "+
		"(creating an adapter requires administrator rights)", major, minor, err)
}

// ringCapacity validates the requested ring size, which Wintun requires to be a
// power of two within a fixed range.
func ringCapacity(requested int) (uint32, error) {
	if requested == 0 {
		return wintunDefaultRingCapacity, nil
	}
	if requested < wintunMinRingCapacity || requested > wintunMaxRingCapacity {
		return 0, fmt.Errorf("water: RingCapacity %d is out of range, it must be between %d and %d",
			requested, wintunMinRingCapacity, wintunMaxRingCapacity)
	}
	if requested&(requested-1) != 0 {
		return 0, fmt.Errorf("water: RingCapacity %d is not a power of two", requested)
	}
	return uint32(requested), nil
}

// Read returns the next packet from the adapter, blocking until one arrives.
//
// b should be at least as large as the interface MTU. A packet too big to fit is
// dropped and io.ErrShortBuffer returned along with the prefix that did fit,
// rather than being silently truncated into a corrupt packet.
func (d *wintunDevice) Read(b []byte) (int, error) {
	if err := d.gate.enter(); err != nil {
		return 0, err
	}
	defer d.gate.leave()

	d.rMu.Lock()
	defer d.rMu.Unlock()

	for {
		if d.gate.closing() {
			return 0, os.ErrClosed
		}

		packet, size, err := d.api.ReceivePacket(d.session)
		if err == nil {
			n := copy(b, ringMemory(packet, int(size)))
			d.api.ReleaseReceivePacket(d.session, packet)
			if n < int(size) {
				return n, io.ErrShortBuffer
			}
			return n, nil
		}

		switch {
		case errors.Is(err, windows.ERROR_NO_MORE_ITEMS):
			// The receive ring is empty. Park until the driver signals that a
			// packet arrived -- or until Close signals the same event to wake us.
			//
			// Close's signal cannot be lost in the window between the check
			// above and this wait: the event is auto-reset and only ever reset
			// by a wait that consumes it, never by ReceivePacket. So a SetEvent
			// that lands in that window leaves the event signalled, this wait
			// returns at once, and the next trip round the loop sees the gate
			// closing.
			if _, err := windows.WaitForSingleObject(d.readWait, windows.INFINITE); err != nil {
				return 0, err
			}
		case errors.Is(err, windows.ERROR_HANDLE_EOF):
			return 0, os.ErrClosed
		case errors.Is(err, windows.ERROR_INVALID_DATA):
			return 0, errors.New("water: the Wintun receive ring is corrupt")
		default:
			return 0, fmt.Errorf("water: reading from Wintun: %w", err)
		}
	}
}

// Write hands a packet to the adapter.
//
// When the send ring is full the packet is dropped and Write reports success, as
// a saturated link on any other platform would. Reporting a short write instead
// would abort an io.Copy over what is a routine and transient condition.
func (d *wintunDevice) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if len(b) > wintunMaxPacketSize {
		return 0, fmt.Errorf("water: packet of %d bytes is larger than Wintun's %d byte limit",
			len(b), wintunMaxPacketSize)
	}

	if err := d.gate.enter(); err != nil {
		return 0, err
	}
	defer d.gate.leave()

	d.wMu.Lock()
	defer d.wMu.Unlock()

	packet, err := d.api.AllocateSendPacket(d.session, len(b))
	if err != nil {
		switch {
		case errors.Is(err, windows.ERROR_BUFFER_OVERFLOW):
			return len(b), nil // ring full: drop, as a congested link does
		case errors.Is(err, windows.ERROR_HANDLE_EOF):
			return 0, os.ErrClosed
		case errors.Is(err, windows.ERROR_INVALID_DATA):
			return 0, errors.New("water: the Wintun send ring is corrupt")
		}
		return 0, fmt.Errorf("water: writing to Wintun: %w", err)
	}

	copy(ringMemory(packet, len(b)), b)
	d.api.SendPacket(d.session, packet)
	return len(b), nil
}

// Close ends the session and removes the adapter. It may be called while a Read
// or a Write is in flight, and it may be called more than once.
func (d *wintunDevice) Close() error {
	return d.gate.shutdown(
		// Signalling the read-wait event is what releases a Read parked on an
		// idle interface: it wakes, sees the gate closing, and returns
		// os.ErrClosed instead of waiting for a packet that is never coming.
		func() { windows.SetEvent(d.readWait) },
		func() error {
			// Safe now that no Read or Write is touching the rings.
			d.api.EndSession(d.session)
			d.api.CloseAdapter(d.adapter)
			return nil
		},
	)
}
