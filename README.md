# water
`water` is a native Go library for [TUN/TAP](http://en.wikipedia.org/wiki/TUN/TAP) interfaces.

`water` is designed to be simple and efficient. It

* wraps almost only syscalls and uses only Go standard types;
* exposes standard interfaces; plays well with standard packages like `io`, `bufio`, etc..
* does not handle memory management (allocating/destructing slice). It's up to user to decide how to deal with buffers; whether to use GC.

`water/waterutil` has some useful functions to interpret MAC frame headers and IP packet headers. It also contains some constants such as protocol numbers and ethernet frame types.

## Installation
```
go get -u github.com/songgao/water
go get -u github.com/songgao/water/waterutil
```

## Documentation
[http://godoc.org/github.com/songgao/water](http://godoc.org/github.com/songgao/water)

[http://godoc.org/github.com/songgao/water/waterutil](http://godoc.org/github.com/songgao/water/waterutil)

## Example

```go
package main

import (
	"github.com/songgao/water"
	"github.com/songgao/water/waterutil"
	"fmt"
)

const BUFFERSIZE = 1522

func main() {
	ifce, err := water.NewTAP("")
	fmt.Printf("%v, %v\n\n", err, ifce)
	buffer := make([]byte, BUFFERSIZE)
	for {
		_, err = ifce.Read(buffer)
		if err != nil {
			break
		}
		ethertype := waterutil.MACEthertype(buffer)
		if ethertype == waterutil.IPv4 {
			packet := waterutil.MACPayload(buffer)
			if waterutil.IsIPv4(packet) {
				fmt.Printf("Source:      %v [%v]\n", waterutil.MACSource(buffer), waterutil.IPv4Source(packet))
				fmt.Printf("Destination: %v [%v]\n", waterutil.MACDestination(buffer), waterutil.IPv4Destination(packet))
				fmt.Printf("Protocol:    %v\n\n", waterutil.IPv4Protocol(packet))
			}
		}
	}
}
```

This piece of code creates a `TAP` interface, and prints some header information for every IPv4 packet. After pull up the `main.go`, you'll need to bring up the interface and assign IP address. All of these need root permission.

```bash
sudo go run main.go
```

```bash
sudo ip link set dev tap0 up
sudo ip addr add 10.0.0.1/24 dev tap0
```

Now, try sending some ICMP broadcast message:
```bash
ping -b 10.0.0.255
```

You'll see the `main.go` print something like:
```
<nil>, &{true 0xf84003f058 tap0}

Source:      42:35:da:af:2b:00 [10.0.0.1]
Destination: ff:ff:ff:ff:ff:ff [10.0.0.255]
Protocol:    1

Source:      42:35:da:af:2b:00 [10.0.0.1]
Destination: ff:ff:ff:ff:ff:ff [10.0.0.255]
Protocol:    1

Source:      42:35:da:af:2b:00 [10.0.0.1]
Destination: ff:ff:ff:ff:ff:ff [10.0.0.255]
Protocol:    1
```

## TODO
* IPv6 Support in `waterutil`
* Darwin(Mac) Support

## LICENSE
[BSD 3-Clause License](http://opensource.org/licenses/BSD-3-Clause)

## Alternatives
`tuntap`: [https://code.google.com/p/tuntap/](https://code.google.com/p/tuntap/)
