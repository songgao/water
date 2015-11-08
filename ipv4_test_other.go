// +build !linux

package water

import (
	"net"
	"testing"
)

func setupIfce(t *testing.T, ipNet net.IPNet, dev string) {
	t.Fatal("unsupported platform")
}
