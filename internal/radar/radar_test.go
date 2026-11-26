package radar

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
	"unsafe"

	"netpulse/internal/config"
	"netpulse/internal/netinfo"
)

func TestSubnetHostIPs(t *testing.T) {
	cidr, hosts, err := SubnetHostIPs("192.168.1.42", 24)
	if err != nil {
		t.Fatalf("SubnetHostIPs /24 error: %v", err)
	}
	if cidr != "192.168.1.0/24" || len(hosts) != 254 {
		t.Fatalf("unexpected /24 result: cidr=%s len=%d", cidr, len(hosts))
	}
	if hosts[0] != "192.168.1.1" || hosts[len(hosts)-1] != "192.168.1.254" {
		t.Fatalf("unexpected /24 bounds: first=%s last=%s", hosts[0], hosts[len(hosts)-1])
	}

	cidr29, hosts29, err := SubnetHostIPs("10.0.0.10", 29)
	if err != nil {
		t.Fatalf("SubnetHostIPs /29 error: %v", err)
	}
	if cidr29 != "10.0.0.8/29" || len(hosts29) != 6 {
		t.Fatalf("unexpected /29 result: cidr=%s len=%d", cidr29, len(hosts29))
	}

	// /16 must be capped safely to /24
	cidr16, hosts16, err := SubnetHostIPs("172.16.5.20", 16)
	if err != nil {
		t.Fatalf("SubnetHostIPs /16 error: %v", err)
	}
	if cidr16 != "172.16.5.0/24" || len(hosts16) != 254 {
		t.Fatalf("expected /16 to cap to /24, got cidr=%s len=%d", cidr16, len(hosts16))
	}
}

func TestLookupVendorAndRandomizedMAC(t *testing.T) {
	cases := []struct {
		mac      string
		want     string
		wantRand bool
	}{
		{"00:03:93:11:22:33", "Apple", false},
		{"50-C7-BF-AA-BB-CC", "TP-Link", false},
		{"24:5A:4C:01:02:03", "Ubiquiti", false},
		{"24:6F:28:11:22:33", "Espressif (IoT)", false},
		{"70:9E:29:44:55:66", "Sony PlayStation", false},
		{"1C:69:7A:10:20:30", "Intel", false},
		{"02:11:22:33:44:55", "MAC Aleatoria (Privada)", true},
		{"DA:A1:19:01:02:03", "MAC Aleatoria (Privada)", true},
		{"00:FE:ED:01:02:03", "Fabricante desconocido", false},
	}

	for _, tc := range cases {
		gotVendor, gotRand := LookupVendor(tc.mac)
		if gotVendor != tc.want || gotRand != tc.wantRand {
			t.Fatalf("LookupVendor(%q) = (%q, %v), want (%q, %v)", tc.mac, gotVendor, gotRand, tc.want, tc.wantRand)
		}
	}
}

func TestInferCategory(t *testing.T) {
	cases := []struct {
		name string
		dev  Device
		want string
	}{
		{"gateway", Device{IsGateway: true, Vendor: "Intel"}, "router"},
		{"local pc", Device{IsLocal: true, Vendor: "Realtek"}, "pc"},
		{"ps5", Device{Vendor: "Sony PlayStation"}, "console"},
		{"chromecast", Device{MDNSName: "Living-Room-Chromecast", Services: []string{"_googlecast"}}, "tv"},
		{"printer", Device{Vendor: "HP", Services: []string{"_printer"}}, "printer"},
		{"iot", Device{Vendor: "Espressif (IoT)"}, "iot"},
		{"randomized phone", Device{IsRandomizedMAC: true}, "phone"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := InferCategory(tc.dev); got != tc.want {
				t.Fatalf("InferCategory() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseNetBIOSResponse(t *testing.T) {
	pkt := make([]byte, 57+18*2)
	binary.BigEndian.PutUint16(pkt[6:8], 1) // 1 answer
	pkt[56] = 2                             // 2 names

	// Record 0: group name WORKGROUP<00> (should be skipped)
	copy(pkt[57:72], "WORKGROUP      ")
	pkt[72] = 0x00
	binary.BigEndian.PutUint16(pkt[73:75], 0x8400)

	// Record 1: unique workstation NICOPC<00>
	copy(pkt[75:90], "NICOPC         ")
	pkt[90] = 0x00
	binary.BigEndian.PutUint16(pkt[91:93], 0x0400)

	got := ParseNetBIOSResponse(pkt)
	if got != "NICOPC" {
		t.Fatalf("ParseNetBIOSResponse() = %q, want 'NICOPC'", got)
	}
}

func TestParseSSDPResponse(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age=1800\r\n" +
		"LOCATION: http://192.168.1.50:8008/ssdp/device-desc.xml\r\n" +
		"SERVER: Linux/3.8 UPnP/1.0 Chromecast/1.0\r\n" +
		"ST: urn:dial-multiscreen-org:service:dial:1\r\n\r\n"

	info := ParseSSDPResponse(raw)
	if info.Location != "http://192.168.1.50:8008/ssdp/device-desc.xml" {
		t.Fatalf("unexpected Location: %q", info.Location)
	}
	if info.ServiceType != "urn:dial-multiscreen-org:service:dial:1" {
		t.Fatalf("unexpected ServiceType: %q", info.ServiceType)
	}
	if info.Server == "" {
		t.Fatalf("expected non-empty Server")
	}
}

func TestParseMDNSPacketAndPointerSafety(t *testing.T) {
	// Synthetic mDNS response with 1 A record for "living-tv.local" and 1 PTR for "_googlecast._tcp.local"
	pkt := make([]byte, 12)
	binary.BigEndian.PutUint16(pkt[0:2], 0)
	binary.BigEndian.PutUint16(pkt[2:4], 0x8400) // response
	binary.BigEndian.PutUint16(pkt[6:8], 2)      // 2 answers

	// Answer 1: living-tv.local IN A 192.168.1.50
	pkt = append(pkt, 9)
	pkt = append(pkt, []byte("living-tv")...)
	pkt = append(pkt, 5)
	pkt = append(pkt, []byte("local")...)
	pkt = append(pkt, 0)
	pkt = append(pkt, 0, 1, 0, 1, 0, 0, 0, 120, 0, 4, 192, 168, 1, 50)

	// Answer 2: _googlecast._tcp.local IN PTR Living._googlecast._tcp.local
	pkt = append(pkt, 11)
	pkt = append(pkt, []byte("_googlecast")...)
	pkt = append(pkt, 4)
	pkt = append(pkt, []byte("_tcp")...)
	pkt = append(pkt, 5)
	pkt = append(pkt, []byte("local")...)
	pkt = append(pkt, 0)
	// PTR to offset 12? Let's encode full rdata cleanly:
	rdata := []byte{6}
	rdata = append(rdata, []byte("Living")...)
	rdata = append(rdata, 11)
	rdata = append(rdata, []byte("_googlecast")...)
	rdata = append(rdata, 4)
	rdata = append(rdata, []byte("_tcp")...)
	rdata = append(rdata, 5)
	rdata = append(rdata, []byte("local")...)
	rdata = append(rdata, 0)
	pkt = append(pkt, 0, 12, 0, 1, 0, 0, 0, 120, 0, byte(len(rdata)))
	pkt = append(pkt, rdata...)

	info := ParseMDNSPacket(pkt)
	if info.Hostname != "living-tv" {
		t.Fatalf("expected hostname 'living-tv', got %q", info.Hostname)
	}
	if len(info.Services) == 0 || info.Services[0] != "_googlecast" {
		t.Fatalf("expected '_googlecast' service, got %v", info.Services)
	}

	// Self-referential pointer loop must terminate safely without panic/hang
	malicious := []byte{
		0, 0, 0x84, 0, 0, 1, 0, 0, 0, 0, 0, 0,
		0xC0, 0x0C, // pointer to offset 12 (itself)
		0, 1, 0, 1,
	}
	_ = ParseMDNSPacket(malicious)
}

func TestMIBIPNetRowLayout(t *testing.T) {
	if size := unsafe.Sizeof(mibIpNetRow{}); size != 24 {
		t.Fatalf("unexpected mibIpNetRow size: got %d, want 24", size)
	}
}

type mockARPScanner struct {
	table []ARPEntry
	hosts map[string]string
}

func (m *mockARPScanner) ProbeARP(ctx context.Context, destIP, srcIP string) (string, float64, bool) {
	mac, ok := m.hosts[destIP]
	if !ok {
		return "", 0, false
	}
	return mac, 1.2, true
}

func (m *mockARPScanner) ReadARPTable(ifIndex uint32) ([]ARPEntry, error) {
	return m.table, nil
}

func TestScannerEndToEndWithMocks(t *testing.T) {
	arp := &mockARPScanner{
		table: []ARPEntry{
			{IP: "192.168.1.1", MAC: "50:C7:BF:01:02:03", IfIndex: 7, Type: mibIpNetTypeDynamic},
		},
		hosts: map[string]string{
			"192.168.1.1":  "50:C7:BF:01:02:03",
			"192.168.1.50": "70:9E:29:AA:BB:CC", // PlayStation (trusted in knownDevices)
			"192.168.1.99": "24:6F:28:11:22:33", // Espressif (new/untrusted)
		},
	}

	scanner := NewScannerWithDeps(arp, DiscoveryHooks{
		DiscoverMDNS: func(ctx context.Context, localIP string, duration time.Duration) map[string]MDNSInfo {
			return map[string]MDNSInfo{
				"192.168.1.99": {Hostname: "sensor-patio", Services: []string{"_hap"}},
			}
		},
		DiscoverSSDP: func(ctx context.Context, localIP string, duration time.Duration) map[string]SSDPInfo {
			return map[string]SSDPInfo{
				"192.168.1.50": {FriendlyName: "PS5-Living"},
			}
		},
		QueryNetBIOS: func(ctx context.Context, ip string, timeout time.Duration) string {
			return ""
		},
		LookupAddr: func(ctx context.Context, ip string) string {
			return ""
		},
	})

	summary := netinfo.NetworkSummary{
		Connected:   true,
		LocalIP:     "192.168.1.20",
		GatewayIP:   "192.168.1.1",
		AdapterMAC:  "1C:69:7A:00:11:22",
		AdapterName: "Ethernet",
		Primary: &netinfo.AdapterInfo{
			IPv4:      "192.168.1.20",
			PrefixLen: 24,
			Gateway:   "192.168.1.1",
			MAC:       "1C:69:7A:00:11:22",
			IfIndex:   7,
		},
	}

	known := map[string]config.KnownDevice{
		"70:9E:29:AA:BB:CC": {
			Alias:     "Mi PS5",
			Trusted:   true,
			Category:  "console",
			FirstSeen: "2026-01-01T00:00:00Z",
		},
	}

	var stages []string
	res, macByKey, err := scanner.Scan(context.Background(), summary, known, func(p ScanProgress) {
		stages = append(stages, p.Stage)
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if res.TotalDevices != 4 {
		t.Fatalf("expected 4 devices (local + gateway + ps5 + iot), got %d", res.TotalDevices)
	}
	if res.TrustedCount != 2 || res.UnknownCount != 2 || res.NewCount != 2 {
		t.Fatalf("unexpected counts: %+v", res)
	}
	if len(stages) == 0 || stages[len(stages)-1] != "done" {
		t.Fatalf("expected final progress stage 'done', got %v", stages)
	}

	// Verify sort order: Gateway -> Local PC -> Untrusted/New -> Trusted
	if !res.Devices[0].IsGateway {
		t.Fatalf("expected gateway first, got %+v", res.Devices[0])
	}
	if !res.Devices[1].IsLocal {
		t.Fatalf("expected local PC second, got %+v", res.Devices[1])
	}
	if res.Devices[2].IP != "192.168.1.99" || !res.Devices[2].IsNew || res.Devices[2].Category != "iot" {
		t.Fatalf("expected untrusted Espressif third, got %+v", res.Devices[2])
	}
	if res.Devices[3].Alias != "Mi PS5" || !res.Devices[3].Trusted {
		t.Fatalf("expected trusted PS5 fourth, got %+v", res.Devices[3])
	}

	ps5Key := DeviceKey("70:9E:29:AA:BB:CC")
	if macByKey[ps5Key] != "70:9E:29:AA:BB:CC" {
		t.Fatalf("expected macByKey entry for PS5 key %q", ps5Key)
	}
}

func TestGzippedExtraOUILookup(t *testing.T) {
	if v, isRand := LookupVendor("74:24:9F:11:22:33"); v != "SpaceX Starlink" || isRand {
		t.Fatalf("expected SpaceX Starlink from gzipped OUI DB, got (%q, %v)", v, isRand)
	}
	if v, isRand := LookupVendor("E8:D8:D1:AA:BB:CC"); v != "Valve / Steam Deck" || isRand {
		t.Fatalf("expected Valve / Steam Deck from gzipped OUI DB, got (%q, %v)", v, isRand)
	}
}

func TestBuildMagicPacketAndInspectServices(t *testing.T) {
	pkt, err := BuildMagicPacket("aa-bb-cc-dd-ee-ff")
	if err != nil {
		t.Fatalf("BuildMagicPacket failed: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("expected 102-byte magic packet, got %d", len(pkt))
	}
	if !bytes.Equal(pkt[:6], []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) {
		t.Fatalf("missing 6-byte 0xFF sync header")
	}
	wantMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for i := 0; i < 16; i++ {
		if !bytes.Equal(pkt[6+i*6:12+i*6], wantMAC) {
			t.Fatalf("MAC repetition %d mismatch", i)
		}
	}

	if _, err := BuildMagicPacket("invalid"); err == nil {
		t.Fatalf("expected error for invalid MAC")
	}

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	if _, ok := probeTCPPort(context.Background(), "127.0.0.1", port, 300*time.Millisecond); !ok {
		t.Fatalf("expected probeTCPPort to detect local listener on port %d", port)
	}
}
