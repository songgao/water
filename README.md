# water

`water` is a native Go library for [TUN/TAP](http://en.wikipedia.org/wiki/TUN/TAP) interfaces.

`water` is designed to be simple and efficient. It

* wraps almost only syscalls and uses only Go standard types;
* exposes standard interfaces; plays well with standard packages like `io`, `bufio`, etc..
* does not handle memory management (allocating/destructing slice). It's up to user to decide whether/how to reuse buffers.

~~`water/waterutil` has some useful functions to interpret MAC frame headers and IP packet headers. It also contains some constants such as protocol numbers and ethernet frame types.~~

See https://github.com/songgao/packets for functions for parsing various packets.

## Supported Platforms

| Platform | TUN | TAP | Driver |
| --- | --- | --- | --- |
| Linux | yes | yes | built into the kernel |
| Windows | yes | yes | [Wintun](https://www.wintun.net) for TUN, [tap-windows6](https://github.com/OpenVPN/tap-windows6) for TAP and as a TUN fallback |
| macOS | yes (point-to-point) | via [tuntaposx](https://sourceforge.net/p/tuntaposx) | `utun` built into the system |

## Installation
```
go get github.com/songgao/water
```

## Documentation
[http://godoc.org/github.com/songgao/water](http://godoc.org/github.com/songgao/water)

## Example

### TAP on Linux:

```go
package main

import (
	"log"

	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
)

func main() {
	config := water.Config{
		DeviceType: water.TAP,
	}
	config.Name = "O_O"

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
	}
	var frame ethernet.Frame

	for {
		frame.Resize(1500)
		n, err := ifce.Read([]byte(frame))
		if err != nil {
			log.Fatal(err)
		}
		frame = frame[:n]
		log.Printf("Dst: %s\n", frame.Destination())
		log.Printf("Src: %s\n", frame.Source())
		log.Printf("Ethertype: % x\n", frame.Ethertype())
		log.Printf("Payload: % x\n", frame.Payload())
	}
}
```

This piece of code creates a `TAP` interface, and prints some header information for every frame. After pull up the `main.go`, you'll need to bring up the interface and assign an IP address. All of these need root permission.

```bash
sudo go run main.go
```

In a new terminal:

```bash
sudo ip addr add 10.1.0.10/24 dev O_O
sudo ip link set dev O_O up
```

Wait until the output `main.go` terminal, try sending some ICMP broadcast message:
```bash
ping -c1 -b 10.1.0.255
```

You'll see output containing the IPv4 ICMP frame:
```
2016/10/24 03:18:16 Dst: ff:ff:ff:ff:ff:ff
2016/10/24 03:18:16 Src: 72:3c:fc:29:1c:6f
2016/10/24 03:18:16 Ethertype: 08 00
2016/10/24 03:18:16 Payload: 45 00 00 54 00 00 40 00 40 01 25 9f 0a 01 00 0a 0a 01 00 ff 08 00 01 c1 08 49 00 01 78 7d 0d 58 00 00 00 00 a2 4c 07 00 00 00 00 00 10 11 12 13 14 15 16 17 18 19 1a 1b 1c 1d 1e 1f 20 21 22 23 24 25 26 27 28 29 2a 2b 2c 2d 2e 2f 30 31 32 33 34 35 36 37
```

### TUN on macOS

```go
package main

import (
	"log"

	"github.com/songgao/water"
)

func main() {
	ifce, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Interface Name: %s\n", ifce.Name())

	packet := make([]byte, 2000)
	for {
		n, err := ifce.Read(packet)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Packet Received: % x\n", packet[:n])
	}
}
```

Run it!

```bash
$ sudo go run main.go
```

This is a point-to-point only interface. Use `ifconfig` to see its attributes. You need to bring it up and assign IP addresses (apparently replace `utun2` if needed):

```bash
$ sudo ifconfig utun2 10.1.0.10 10.1.0.20 up
```

Now send some ICMP packets to the interface:

```bash
$ ping 10.1.0.20
```

You'd see the ICMP packets printed out:

```
2017/03/20 21:17:30 Interface Name: utun2
2017/03/20 21:17:40 Packet Received: 45 00 00 54 e9 1d 00 00 40 01 7d 6c 0a 01 00 0a 0a 01 00 14 08 00 ee 04 21 15 00 00 58 d0 a9 64 00 08 fb a5 08 09 0a 0b 0c 0d 0e 0f 10 11 12 13 14 15 16 17 18 19 1a 1b 1c 1d 1e 1f 20 21 22 23 24 25 26 27 28 29 2a 2b 2c 2d 2e 2f 30 31 32 33 34 35 36 37
```

#### Caveats

1. Only Point-to-Point user TUN devices are supported. TAP devices are *not* supported natively by macOS.
2. Custom interface names are not supported by macOS. Interface names are automatically generated serially, using the `utun<#>` naming convention.

### Windows

water speaks to two drivers on Windows, and picks between them for you:

* **[Wintun](https://www.wintun.net)** — the layer 3 driver written for WireGuard, and the
  right choice for **TUN**. It is kernel-signed, needs no adapter to be installed ahead of
  time (water creates the adapter on `New()` and removes it again on `Close()`), and is a
  real layer 3 device rather than an emulation on top of Ethernet. Ship `wintun.dll` next to
  your executable — that is the whole install. It cannot do TAP.
* **[tap-windows6](https://github.com/OpenVPN/tap-windows6)** — the driver that comes with
  the [OpenVPN client](https://github.com/OpenVPN/openvpn), and the only way to get a **TAP**
  device. It is also the TUN fallback when `wintun.dll` is not present. The adapter has to be
  installed beforehand.

By default (`WindowsDriverAuto`) a TUN device uses Wintun when `wintun.dll` can be loaded and
tap-windows6 otherwise, and a TAP device always uses tap-windows6. Set
`PlatformSpecificParams.Driver` to `WindowsDriverWintun` or `WindowsDriverTapWindows6` to pin
one explicitly.

Creating an interface requires **administrator rights** either way.

#### TUN on Windows

Download `wintun.dll` from [wintun.net](https://www.wintun.net) and put the one matching your
architecture (`bin/amd64/wintun.dll`, say) next to your executable. Then:

```go
package main

import (
	"log"

	"github.com/songgao/water"
)

func main() {
	ifce, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer ifce.Close()

	log.Printf("Interface Name: %s\n", ifce.Name())

	packet := make([]byte, 2000)
	for {
		n, err := ifce.Read(packet)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Packet Received: % x\n", packet[:n])
	}
}
```

Run it from an **administrator** prompt, then give the interface an address from a second
administrator prompt (`ifce.Name()` prints the name to use):

```dos
netsh interface ip set address name="water0" source=static addr=10.1.0.10 mask=255.255.255.0 gateway=none
```

and send it some traffic:

```dos
ping 10.1.0.20
```

#### TAP on Windows

TAP needs the tap-windows6 driver installed (installing the OpenVPN client does that). This is
compatible with the Linux code:

```go
package main

import (
	"log"

	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
)

func main() {
	ifce, err := water.New(water.Config{
		DeviceType: water.TAP,
	})
	if err != nil {
		log.Fatal(err)
	}
	var frame ethernet.Frame

	for {
		frame.Resize(1500)
		n, err := ifce.Read([]byte(frame))
		if err != nil {
			log.Fatal(err)
		}
		frame = frame[:n]
		log.Printf("Dst: %s\n", frame.Destination())
		log.Printf("Src: %s\n", frame.Source())
		log.Printf("Ethertype: % x\n", frame.Ethertype())
		log.Printf("Payload: % x\n", frame.Payload())
	}
}
```

Same as Linux version, but you don't need to bring up the device by hand, the only thing you need is to assign an IP address to it.

```dos
go run main.go
```

It will output a lot of lines because of some windows services and dhcp.
You will need admin right to assign IP.

In a new cmd (admin right):

```dos
# Replace with your device name, it can be achieved by ifce.Name().
netsh interface ip set address name="Ehternet 2" source=static addr=10.1.0.10 mask=255.255.255.0 gateway=none
```

The `main.go` terminal should be silenced after IP assignment, try sending some ICMP broadcast message:

```dos
ping 10.1.0.255
```

You'll see output containing the IPv4 ICMP frame same as the Linux version.

#### Specifying interface name

If you are going to use multiple TAP devices on the Windows, there is a way to specify an interface name to select the exact device that you need:

```go
	ifce, err := water.New(water.Config{
		DeviceType: water.TAP,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID:   "tap0901",
			InterfaceName: "Ethernet 3",
			Network:       "192.168.1.10/24",
		},
	})
```

For Wintun, `InterfaceName` instead *names* the adapter that water creates, rather than
selecting an existing one. Left empty, water picks the first free name of the form `water0`,
`water1`, ... Either way `ifce.Name()` reports the name the adapter ended up with, which is
the name to hand to `netsh`.

#### Closing an interface

`Close()` releases a `Read()` that is parked waiting for a packet, on every platform, so the
usual shape works on Windows too:

```go
go func() {
	<-ctx.Done()
	ifce.Close() // unblocks the Read below
}()

for {
	n, err := ifce.Read(packet)
	if errors.Is(err, os.ErrClosed) {
		return
	}
	...
}
```

## TODO
* tuntaposx for TAP on Darwin

## Alternatives
`tuntap`: [https://code.google.com/p/tuntap/](https://code.google.com/p/tuntap/)
