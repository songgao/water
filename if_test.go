package water

import "testing"

func TestNewRejectsUnknownDeviceType(t *testing.T) {
	ifce, err := New(Config{DeviceType: DeviceType(42)})
	if err == nil {
		ifce.Close()
		t.Fatal("New with an unknown DeviceType succeeded, want an error")
	}
}

// TestZeroConfigIsValid documents the promise made in Config's doc comment. It
// only checks that a zero Config is filled in rather than rejected out of hand,
// so it does not need a driver or any privilege.
func TestZeroConfigIsValid(t *testing.T) {
	config := Config{}
	if zeroConfig != config {
		t.Fatal("a zero Config is not equal to zeroConfig")
	}

	if got := defaultConfig(); got.DeviceType != TUN {
		t.Errorf("defaultConfig().DeviceType = %v, want TUN", got.DeviceType)
	}
}
