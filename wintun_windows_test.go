package water

import "testing"

// TestNameAndInterfaceName pins down how the portable Name spelling and the
// older Windows-only InterfaceName spelling combine, since cross-platform code
// wants the former and existing Windows code uses the latter.
func TestNameAndInterfaceName(t *testing.T) {
	for _, tc := range []struct {
		name          string
		params        PlatformSpecificParams
		wantInterface string
	}{
		{
			name:          "Name alone is used",
			params:        PlatformSpecificParams{Name: "portable"},
			wantInterface: "portable",
		},
		{
			name:          "InterfaceName alone still works",
			params:        PlatformSpecificParams{InterfaceName: "windows-only"},
			wantInterface: "windows-only",
		},
		{
			name:          "InterfaceName wins when both are set",
			params:        PlatformSpecificParams{Name: "portable", InterfaceName: "windows-only"},
			wantInterface: "windows-only",
		},
		{
			name:          "neither leaves it empty, so a driver default is picked",
			params:        PlatformSpecificParams{},
			wantInterface: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := Config{DeviceType: TUN, PlatformSpecificParams: tc.params}

			if got := config.interfaceName(); got != tc.wantInterface {
				t.Errorf("interfaceName() = %q, want %q", got, tc.wantInterface)
			}
		})
	}
}

func TestRingCapacity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested int
		want      uint32
		wantErr   bool
	}{
		{name: "zero means the default", requested: 0, want: wintunDefaultRingCapacity},
		{name: "minimum", requested: wintunMinRingCapacity, want: wintunMinRingCapacity},
		{name: "maximum", requested: wintunMaxRingCapacity, want: wintunMaxRingCapacity},
		{name: "power of two in range", requested: 1 << 20, want: 1 << 20},
		{name: "below the minimum", requested: 1 << 16, wantErr: true},
		{name: "above the maximum", requested: 1 << 27, wantErr: true},
		{name: "not a power of two", requested: 0x300000, wantErr: true},
		{name: "negative", requested: -1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ringCapacity(tc.requested)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ringCapacity(%d) = %d, want an error", tc.requested, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ringCapacity(%d): %v", tc.requested, err)
			}
			if got != tc.want {
				t.Errorf("ringCapacity(%d) = %d, want %d", tc.requested, got, tc.want)
			}
		})
	}
}

// TestTapControlCodes checks the CTL_CODE arithmetic against the values
// tap-windows.h defines, since getting these wrong means the driver silently
// misinterprets our ioctls.
func TestTapControlCodes(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  uint32
		want uint32
	}{
		{"TAP_WIN_IOCTL_GET_MAC", tapIoctlGetMac, 0x00220004},
		{"TAP_WIN_IOCTL_GET_VERSION", tapIoctlGetVersion, 0x00220008},
		{"TAP_WIN_IOCTL_SET_MEDIA_STATUS", tapIoctlSetMediaStatus, 0x00220018},
		{"TAP_WIN_IOCTL_CONFIG_TUN", tapIoctlConfigTun, 0x00220028},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %#08x, want %#08x", tc.name, tc.got, tc.want)
		}
	}
}
