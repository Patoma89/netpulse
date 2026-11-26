package radar

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"

	"netpulse/internal/config"
)

// Device represents a single discovered host on the local area network.
type Device struct {
	Key             string   `json:"key"`
	IP              string   `json:"ip"`
	MAC             string   `json:"mac"`
	Hostname        string   `json:"hostname"`
	NetBIOSName     string   `json:"netbiosName"`
	MDNSName        string   `json:"mdnsName"`
	SSDPName        string   `json:"ssdpName"`
	Vendor          string   `json:"vendor"`
	IsRandomizedMAC bool     `json:"isRandomizedMac"`
	Category        string   `json:"category"` // "router" | "pc" | "phone" | "tv" | "console" | "printer" | "iot" | "unknown"
	Alias           string   `json:"alias"`
	Trusted         bool     `json:"trusted"`
	IsNew           bool     `json:"isNew"`
	IsLocal         bool     `json:"isLocal"`
	IsGateway       bool     `json:"isGateway"`
	RTTMs           float64  `json:"rttMs"`
	Protocols       []string `json:"protocols"`
	Services        []string `json:"services"`
	FirstSeen       string   `json:"firstSeen"`
	LastSeen        string   `json:"lastSeen"`
	SeenCount       int      `json:"seenCount"`
	Online          bool     `json:"online"`
	LastTransition  string   `json:"lastTransition"`
}

// GatewayMACAlert warns when the gateway's ARP MAC changed unexpectedly within the same network profile.
type GatewayMACAlert struct {
	Active      bool   `json:"active"`
	ProfileID   string `json:"profileId"`
	PreviousMAC string `json:"previousMac"`
	CurrentMAC  string `json:"currentMac"`
	DetectedAt  string `json:"detectedAt"`
}

// ScanProgress streams real-time LAN discovery progress to the UI.
type ScanProgress struct {
	Stage       string `json:"stage"` // "arp" | "discovery" | "classify" | "done" | "error"
	ProgressPct int    `json:"progressPct"`
	FoundCount  int    `json:"foundCount"`
	Message     string `json:"message"`
}

// ScanResult contains the full output and summary counters of a LAN radar scan.
type ScanResult struct {
	Subnet          string           `json:"subnet"`
	ScannedHosts    int              `json:"scannedHosts"`
	DurationMs      int64            `json:"durationMs"`
	CompletedAt     string           `json:"completedAt"`
	TotalDevices    int              `json:"totalDevices"`
	TrustedCount    int              `json:"trustedCount"`
	UnknownCount    int              `json:"unknownCount"`
	NewCount        int              `json:"newCount"`
	ProfileID       string           `json:"profileId"`
	ProfileName     string           `json:"profileName"`
	GatewayMACAlert *GatewayMACAlert `json:"gatewayMacAlert,omitempty"`
	Devices         []Device         `json:"devices"`
}

// ARPEntry represents an entry from the Windows ARP cache.
type ARPEntry struct {
	IP      string
	MAC     string
	IfIndex uint32
	Type    uint32
}

// DeviceKey returns a deterministic 16-char hex identifier for a normalized MAC address.
// This lets the frontend reference devices without exposing raw MACs in Discord Privacy Mode.
func DeviceKey(mac string) string {
	norm := config.NormalizeMAC(mac)
	if norm == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("netpulse-device:" + norm))
	return hex.EncodeToString(sum[:8])
}

// SubnetHostIPs computes all usable IPv4 host addresses in the local subnet.
// Guardrail: subnets broader than /24 are capped to the /24 containing localIP so scans finish in <2s.
func SubnetHostIPs(localIP string, prefixLen uint8) (string, []string, error) {
	ip := net.ParseIP(localIP).To4()
	if ip == nil {
		return "", nil, fmt.Errorf("invalid IPv4 address %q", localIP)
	}

	effectivePrefix := prefixLen
	if effectivePrefix == 0 || effectivePrefix > 30 || effectivePrefix < 24 {
		effectivePrefix = 24
	}

	mask := net.CIDRMask(int(effectivePrefix), 32)
	networkIP := ip.Mask(mask)
	cidr := fmt.Sprintf("%s/%d", networkIP.String(), effectivePrefix)

	netInt := binary.BigEndian.Uint32(networkIP)
	maskInt := binary.BigEndian.Uint32(mask)
	bcastInt := netInt | ^maskInt

	hosts := make([]string, 0, int(bcastInt-netInt))
	for cur := netInt + 1; cur < bcastInt; cur++ {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], cur)
		hosts = append(hosts, net.IP(b[:]).String())
	}

	return cidr, hosts, nil
}
