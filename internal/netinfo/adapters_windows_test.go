//go:build windows

package netinfo

import "testing"

func TestIsVirtualAdapter(t *testing.T) {
	cases := []struct {
		name     string
		ifType   uint32
		iface    string
		desc     string
		expected bool
	}{
		{
			name:     "real wifi adapter",
			ifType:   ifTypeIEEE80211,
			iface:    "Wi-Fi",
			desc:     "Intel(R) Wi-Fi 6E AX211 160MHz",
			expected: false,
		},
		{
			name:     "real ethernet adapter",
			ifType:   ifTypeEthernetCsmacd,
			iface:    "Ethernet",
			desc:     "Realtek PCIe GbE Family Controller",
			expected: false,
		},
		{
			name:     "hyper-v vethernet",
			ifType:   ifTypeEthernetCsmacd,
			iface:    "vEthernet (WSL)",
			desc:     "Hyper-V Virtual Ethernet Adapter",
			expected: true,
		},
		{
			name:     "wireguard tunnel",
			ifType:   ifTypeTunnel,
			iface:    "wg0",
			desc:     "WireGuard Tunnel",
			expected: true,
		},
		{
			name:     "tailscale adapter",
			ifType:   ifTypeEthernetCsmacd,
			iface:    "Tailscale",
			desc:     "Tailscale Tunnel",
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isVirtualAdapter(tc.ifType, tc.iface, tc.desc)
			if got != tc.expected {
				t.Fatalf("isVirtualAdapter(%d, %q, %q) = %v, want %v", tc.ifType, tc.iface, tc.desc, got, tc.expected)
			}
		})
	}
}

func TestFormatMAC(t *testing.T) {
	mac := formatMAC([]byte{0xA4, 0xB1, 0xC2, 0x03, 0xE4, 0xF5}, 6)
	if mac != "A4:B1:C2:03:E4:F5" {
		t.Fatalf("unexpected MAC: %q", mac)
	}
	if empty := formatMAC([]byte{0x01, 0x02}, 2); empty != "" {
		t.Fatalf("expected empty MAC for invalid length, got %q", empty)
	}
}

func TestGetNetworkSummarySmoke(t *testing.T) {
	summary, err := GetNetworkSummary()
	if err != nil {
		t.Fatalf("GetNetworkSummary() error = %v", err)
	}
	if summary.Connected {
		if summary.Primary == nil {
			t.Fatalf("Connected is true but Primary is nil")
		}
		if summary.GatewayIP == "" || summary.LocalIP == "" {
			t.Fatalf("Connected is true but GatewayIP (%q) or LocalIP (%q) is empty", summary.GatewayIP, summary.LocalIP)
		}
	}
}
