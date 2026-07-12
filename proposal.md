# Make Windows work: Wintun support, a `Close()` that actually closes, and a `go.mod`

## Summary

water is the library the Go ecosystem reaches for when it needs a TUN/TAP interface, and on
Linux it earns that. On Windows it does not: the device can be opened, but it cannot be shut
down. `Close()` cannot release a `Read()` that is parked waiting for a packet, so any program
whose main loop is `for { ifce.Read(buf) }` — which is every program that uses this library —
has no way to stop. That, plus the fact that the only supported driver requires installing
OpenVPN, is why "water doesn't work on Windows" is the common wisdom.

This PR makes Windows a first-class platform:

* adds **[Wintun](https://www.wintun.net)** support — the layer 3 driver written for
  WireGuard — as the default TUN backend, falling back to tap-windows6 when `wintun.dll` is
  not present;
* rewrites the **tap-windows6** backend on top of `golang.org/x/sys/windows`, fixing a
  deadlock, two handle leaks per interface, an undefined-behaviour `DeviceIoControl`, and a
  driver-mode race;
* makes `Close()` interrupt in-flight I/O on both backends, so it behaves the way it already
  does on Linux;
* adds a **`go.mod`** — the repository currently has none, so it cannot be built by modern Go
  tooling outside `GOPATH`;
* replaces the dead Travis config with GitHub Actions that build all eight target
  platform/arch pairs and **run the test suite on Linux, macOS and Windows** (with Wintun
  actually installed on the Windows runner).

It also fixes the most serious bug in the repository, which is not a Windows bug at all: every
accessor in **`waterutil` panics on a malformed packet**, so a single crafted packet off the
wire crashes the process on *every* platform. See [Beyond Windows](#beyond-windows).

**The public API on Linux and macOS is unchanged.** Verified mechanically: every exported
signature and struct field for `GOOS=linux` is identical before and after, and the only
`go doc` diff at all is comment text. Existing Linux users can take this upgrade without
touching a line of their code. The Windows `PlatformSpecificParams` gains two fields but keeps
`ComponentID`, `InterfaceName` and `Network`, so existing Windows code compiles and behaves as
before.

---

## The bugs

### 1. `Close()` deadlocks against a pending `Read()` — the headline bug

`wfile.Close()` was:

```go
func (f *wfile) Close() error {
	return syscall.Close(f.fd)
}
```

Meanwhile `Read()` sits inside `GetOverlappedResult(..., bWait=TRUE)`, parked until a packet
arrives. Nothing cancels that wait. On a quiet interface no packet ever arrives, so:

* the reader blocks forever, and
* the handle is closed out from under a live overlapped operation, which is undefined
  behaviour — the kernel may complete the I/O into a buffer belonging to a handle that has
  since been recycled.

`TestCloseUnblockPendingRead` has been in the tree since 2019 asserting exactly this
behaviour. It passes on Linux (where the fd is `O_NONBLOCK` and the runtime poller handles
it) and could never have passed on Windows.

The fix is the standard one, and it is what `Close()` has to do on any platform where a read
parks in the kernel: refuse new I/O, actively **interrupt** the I/O already in flight, wait
for it to come back, and only then release the device. That sequencing is shared between both
Windows backends as `ioGate` (`syscalls_windows.go`); the two drivers supply the interrupt:

* tap-windows6 → `CancelIoEx(fd, nil)`, which cancels every request this process has in
  flight on the handle;
* Wintun → `SetEvent` on the read-wait event, which wakes the parked reader so it can notice
  the gate is closing.

There is a narrower race hiding underneath, which `ioGate` alone does not close: a `Read()`
that has already passed the gate but has not yet reached `ReadFile()` when `Close()` runs
`CancelIoEx` — the cancel finds nothing, the read then parks forever, and `Close()`
deadlocks behind it in `running.Wait()`. `tapDevice.await()` closes it by re-checking the
gate once the operation is in flight, so that whichever of the two observers wins, the
operation still ends up cancelled. (The Wintun path is safe from the analogous race for a
reason worth stating: its read-wait event is auto-reset and is only ever reset by a wait that
consumes it, never by `WintunReceivePacket`, so a `SetEvent` landing in that window cannot be
lost. There is a comment to that effect.)

### 2. Two HANDLE leaks per interface, forever

`newOverlapped()` created an event with `CreateEventW` for each of the read and write
overlapped structures. Nothing ever closed them. Every interface a process opened leaked two
kernel handles for the lifetime of the process. `Close()` now closes both, along with the
device handle.

### 3. `DeviceIoControl` with a NULL `OVERLAPPED` on an overlapped handle

The device is opened `FILE_FLAG_OVERLAPPED`. `setStatus()` and `setTUN()` both called
`DeviceIoControl` with `lpOverlapped = nil`, which on such a handle is undefined: the call is
permitted to return `ERROR_IO_PENDING` and go on writing into the input and output buffers
after it has returned — buffers which, here, are Go slices the caller is about to drop. It
works today only because tap-windows6 happens to complete these ioctls synchronously. The new
`tapIoctl()` supplies an `OVERLAPPED` and waits for completion properly.

### 4. The link came up before the driver was told it was a TUN

`openDev()` called `setStatus(up)` *then* `setTUN()`. Connecting the virtual cable is what
lets the network stack start pushing frames at the adapter, and until `CONFIG_TUN` lands the
driver is still in TAP mode — so frames arriving in that window are delivered as Ethernet
frames to a caller expecting raw IP. OpenVPN itself issues `CONFIG_TUN` before
`SET_MEDIA_STATUS`; we now do the same.

### 5. Registry handle leaks in `getdeviceid()`

`continue` inside the interface-name branch skipped the `key.Close()` below it, and `defer
k2.Close()` sat inside a loop, so the key stayed open until the whole function returned. The
lookup is rewritten as `findTapDevice()` / `inspectAdapter()`, one key per call, each closed
on every path. It now also returns the adapter's friendly name, which we previously went and
re-derived by scanning every system interface for a matching MAC.

### 6. Setting one config field silently blanked the others

`New()` substitutes `defaultPlatformSpecificParams()` only when `PlatformSpecificParams` is
*entirely* zero. So:

```go
water.New(water.Config{
	DeviceType:             water.TUN,
	PlatformSpecificParams: water.PlatformSpecificParams{InterfaceName: "Ethernet 3"},
})
```

leaves `ComponentID` empty, and the registry walk then looks for an adapter whose
`ComponentId` is `""` — which matches nothing, so this can never succeed. Defaults are now
applied per field on Windows, so filling in one field no longer blanks the rest.

### 7. Hand-rolled `LoadLibrary` + `syscall.Syscall`, with a `panic` in `init()`

`init()` did `LoadLibrary("kernel32.dll")` and `GetProcAddress` for `CreateEventW`,
`ResetEvent` and `GetOverlappedResult` — all of which `golang.org/x/sys/windows` already
wraps — and **panicked** on failure, at package-init time, in a library. All of it is gone in
favour of `x/sys/windows`. (`getOverlappedResult` also passed a `*int` where the API wants an
`LPDWORD`; harmless on 64-bit little-endian by luck, not by design.)

---

## Beyond Windows

### `waterutil` panics on any malformed packet — a remote crash, on every platform

This one is the most serious bug in the repository, and it has nothing to do with Windows.

Every accessor in `waterutil` indexes into the packet without checking its length. The
package's entire job is parsing frames that arrived from the network, so those lengths are
attacker-controlled:

```go
func IPv4Source(packet []byte) net.IP {
	return net.IPv4(packet[12], packet[13], packet[14], packet[15])  // panics on a runt packet
}

func IPv4Payload(packet []byte) []byte {
	ihl := packet[0] & 0x0F
	return packet[ihl*4:]   // ihl comes from the packet; a lying IHL slices out of range
}
```

A two-byte runt panics `IPv4Source`, `IPv4Protocol`, `MACSource` and `MACEthertype`. Worse,
`IPv4Payload` believes the packet's own IHL field: a **perfectly well-formed-length 20-byte
IPv4 packet whose IHL claims a 60-byte header** panics with `slice bounds out of range
[60:20]`. That is one crafted packet, from the network, taking down the process — and
`IPv4Payload` is what `IPv4SourcePort`, `IPv4DestinationPort` and both port setters are built
on.

Every accessor now bounds-checks. Signatures are unchanged: a packet too short to hold the
field being asked for yields the zero value (`0`, or `nil` for the slice-valued ones), and the
setters leave a too-short packet alone. `IPv4Payload` only believes the IHL when it describes
a header that is at least the 20-byte minimum *and* actually fits inside the packet.

`IsIPv4` and `IsIPv6` now also require the packet to be long enough to hold the header they
claim, rather than reading `packet[0]` and hoping — so they can serve as the "is this safe to
parse" check the accessors' zero values are distinguished by. This is the one behaviour change
in the package: `IsIPv4` on a 5-byte buffer beginning `0x45` used to say true and now says
false. That seems clearly right, but it is a change, and worth a maintainer's eye.

Backed by `waterutil/waterutil_test.go`, including `FuzzAccessors`, which asserts that no byte
string can panic any accessor. It survives 7.5M executions on the fixed code, and it fails on
the current code **on a seed corpus entry**, before fuzzing even begins. The package had no
tests at all before this.

### Linux

Linux is in good shape and I did not find much, which is presumably why it is the platform
people trust. Two real bugs:

* **fd leak on the error path.** `openDev()` opened `/dev/net/tun` and, if `setupFd()` then
  failed (a bad name, an ioctl refusing), returned without closing it.
* **interface names were silently truncated.** `req.Name` is `IFNAMSIZ` bytes and must be
  NUL-terminated, and `copy()` just truncates. Ask for a 20-character name and you would get
  an interface with a *different*, shorter name than the one you asked for, and `Config.Name`
  would not match `ifce.Name()`. Now an error.

And one documentation bug worth calling out because it documents an unreachable feature:
`DevicePermissions.Owner` and `.Group` are `uint`, but both are documented as *"if set to a
negative value, the value will not be changed"*. You cannot set a `uint` to a negative value,
so that escape hatch does not exist. I fixed the comment rather than the type, since widening
it to `int` would break the API. Flagging it in case you would rather change the type in a
major version.

### macOS: `tunReadCloser` could return a negative read count

```go
n, err := t.f.Read(t.rBuf)
copy(to, t.rBuf[4:])
return n - 4, err   // n == 0 on a failed read  ->  returns -4
```

A negative `n` violates the `io.Reader` contract and panics `io.Copy` and `bufio`
(`"invalid Read result"`). `Write` had the same shape. Both now guard.

---

## Wintun

tap-windows6 is a poor foundation for TUN on Windows. It is an Ethernet device, so its TUN
mode is an emulation in which the driver answers ARP on the kernel's behalf — which is why
`Network` exists at all, why it is documented as unable to notice DHCP or user-driven address
changes, and why every water-on-Windows tutorial starts with "first, install OpenVPN".

Wintun is the driver WireGuard uses, and it is what everyone reaching for a Windows TUN today
actually wants:

| | tap-windows6 | Wintun |
| --- | --- | --- |
| Layer | 2, TUN emulated over ARP | native 3 |
| Install | pre-install the adapter (OpenVPN) | ship `wintun.dll` next to the binary |
| Adapter lifecycle | pre-created, persistent | created on `New()`, removed on `Close()` |
| Transport | overlapped file I/O | shared-memory rings |
| `Network` CIDR needed | yes | no |
| TAP | yes | no |

`wintun_windows.go` binds the driver's C API directly (`LoadLibraryEx` + `GetProcAddress`, no
cgo) and implements `io.ReadWriteCloser` over its ring buffers. The DLL is deliberately loaded
with `LOAD_LIBRARY_SEARCH_APPLICATION_DIR | SYSTEM32 | USER_DIRS` rather than a bare
`LoadLibrary`, which would also search the current working directory — a DLL-hijacking route
for a library that, by its nature, runs elevated.

### Driver selection

`PlatformSpecificParams.Driver` is new, and its zero value is `WindowsDriverAuto`:

* **TUN** → Wintun when `wintun.dll` loads, tap-windows6 otherwise;
* **TAP** → always tap-windows6 (Wintun has no layer 2 mode).

Auto only falls back when the driver is *absent*. If Wintun is present but fails for a real
reason — not elevated, name already taken — that error is returned as-is rather than being
replaced by a confusing tap-windows6 error about a missing `tap0901` adapter. When neither
driver is available, the error names both, so the user knows what to install.

This means **existing Windows users see no behaviour change**: they don't ship `wintun.dll`,
so Auto resolves to tap-windows6 exactly as today. Wintun is opt-in by deployment.

---

## Testing

`.github/workflows/ci.yml` replaces `.travis.yml` (which pinned Go 1.10/1.12 and has not run
in years):

* **vet + gofmt** across `GOOS=linux,darwin,windows`;
* **build** for all 8 supported platform/arch pairs, including `windows/arm64`;
* **test on Linux, macOS and Windows runners**, under `-race`. The Windows job downloads
  `wintun.dll` and places it beside the test binary — the same thing a real deployment does —
  so the Wintun path is genuinely exercised, not just compiled. Tests whose driver is missing
  skip themselves rather than failing, so the tap-windows6 cases stay green on a runner
  without OpenVPN.

New tests:

* `close_test.go` — pins down what `Close()` promises on every platform: it releases a parked
  `Read()`, it is idempotent, I/O after it fails rather than touching a released device, and
  it is safe to call concurrently with in-flight readers. This is the regression suite for the
  headline bug, and it is cross-platform, so Windows is now held to the same contract Linux is.
* `ipv4_windows_test.go` — end-to-end TUN on Windows (create, address with `netsh`, ping,
  read the packet back), one variant pinning Wintun explicitly, plus a check that
  Wintun + TAP is rejected.
* `wintun_windows_test.go` — ring-capacity validation, and the `CTL_CODE` arithmetic checked
  against the constants in `tap-windows.h`, since getting those wrong means the driver
  silently misinterprets our ioctls.
* `if_test.go` — config defaulting, no privileges needed.

### What I verified, and what I did not

Verified by running: the Linux suite passes under `-race` (including the four new close
tests); `waterutil` passes under `-race` and survives 7.5M fuzz executions. All eight
platform/arch pairs build, and `go vet` is clean for all three OSes with tests included.

Not verified by running: **I have not executed the Windows code on Windows.** It is
cross-compiled, vetted and reviewed, and the CI added here is designed to be the thing that
actually proves it — a maintainer merging this should let the Windows job run first. The
Wintun binding follows the same structure as WireGuard's, and the tap-windows6 ioctl sequence
follows OpenVPN's, but that is an argument for plausibility, not a substitute for a green
Windows run.

---

## Notes for review

**`go vet -unsafeptr=false`.** `ringMemory()` in `wintun_windows.go` converts the address
Wintun hands back for a packet in its shared rings into a slice. That memory belongs to the
driver's mapping, not to the Go heap: it never moves and the collector has no stake in it, so
the conversion is safe — but `vet`'s `unsafeptr` check cannot tell foreign memory from a stale
Go pointer, and every Go binding to a driver like this (WireGuard's included) has the same
shape. The `Makefile` and CI turn that one check off, with the reasoning recorded at the call
site. I preferred that to obfuscating the conversion into something `vet` happens not to
recognise. Happy to take a different call here.

**Go floor.** `go.mod` declares `go 1.19` and pins `golang.org/x/sys v0.20.0` (whose own floor
is 1.18), so the module stays usable on older toolchains. This is what let the pre-1.11 build
tags go: `syscalls_{linux,darwin}_legacy.go` existed to support Go < 1.11, and the darwin one
disabled non-blocking mode with the comment *"Close() won't be able to unblock a pending read,
but that's better than being broken"* — a caveat that no longer needs to exist.

**`waterutil/doc.go`** is reformatted by `gofmt -s`. That is purely the Go 1.19 doc-comment
rules indenting the ASCII diagrams as code blocks; it makes them render as intended on
pkg.go.dev instead of being reflowed. No content change, but it is why that file appears in
the diff.

## File map

| File | |
| --- | --- |
| `go.mod`, `go.sum` | new — the repo had no module definition |
| `syscalls_windows.go` | rewritten: driver selection + the shared `ioGate` close protocol |
| `wintun_windows.go` | new — Wintun driver binding |
| `tapwindows6_windows.go` | new — the rewritten tap-windows6 backend |
| `params_windows.go` | `Driver` and `RingCapacity` added; existing fields kept |
| `syscalls_linux.go` | merged in `openDev`; fd leak on error; reject over-long names |
| `syscalls_darwin.go` | merged in `setNonBlock`; fixed the negative `Read`/`Write` counts |
| `waterutil/*.go` | bounds-checked every accessor — the malformed-packet crash |
| `waterutil/waterutil_test.go` | new — the package had no tests; includes `FuzzAccessors` |
| `close_test.go` | the `Close()` contract, cross-platform |
| `.github/workflows/ci.yml` | new — replaces `.travis.yml` |
