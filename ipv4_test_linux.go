package water

import (
	"net"
	"os/exec"
	"testing"
)

func setupIfce(t *testing.T, ipNet net.IPNet, dev string) {
	if err := exec.Command("ip", "link", "set", dev, "up").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("ip", "addr", "add", ipNet.String(), "dev", dev).Run(); err != nil {
		t.Fatal(err)
	}
}
