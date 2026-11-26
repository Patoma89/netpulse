package privacy

import "testing"

func TestMaskIP(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"192.168.1.45", "192.168.•••.•••"},
		{"181.45.220.19", "181.45.•••.•••"},
		{"2800:810:486:1a2b::1", "2800:810:••••:••••:••••"},
		{"not-an-ip", "•••.•••.•••.•••"},
	}

	for _, tc := range cases {
		got := MaskIP(tc.in)
		if got != tc.want {
			t.Errorf("MaskIP(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskMAC(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"aa:bb:cc:11:22:33", "AA:BB:CC:••:••:••"},
		{"A4-B1-C2-D3-E4-F5", "A4:B1:C2:••:••:••"},
		{"aabb.ccdd.eeff", "AA:BB:CC:••:••:••"},
		{"invalid", "••:••:••:••:••:••"},
	}

	for _, tc := range cases {
		got := MaskMAC(tc.in)
		if got != tc.want {
			t.Errorf("MaskMAC(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskSSID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"Home", "••••"},
		{"WiFi", "••••"},
		{"MiWifi_5G", "Mi••••••G"},
		{"Fibertel_Casa_5GHz", "Fi••••••••z"},
	}

	for _, tc := range cases {
		got := MaskSSID(tc.in)
		if got != tc.want {
			t.Errorf("MaskSSID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
