//go:build windows

package netinfo

import (
	"fmt"
	"net"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ifTypeEthernetCsmacd  uint32 = 6
	ifTypeSoftwareLoop    uint32 = 24
	ifTypeIEEE80211       uint32 = 71
	ifTypeTunnel          uint32 = 131
	ifOperStatusUp        uint32 = 1
	unknownLinkSpeedSentinel     = ^uint64(0)
)

type adapterCandidate struct {
	index  int
	metric uint32
}

// GetNetworkSummary queries native Win32 GetAdaptersAddresses without shelling out to netsh/ipconfig.
func GetNetworkSummary() (NetworkSummary, error) {
	flags := uint32(windows.GAA_FLAG_INCLUDE_GATEWAYS | windows.GAA_FLAG_INCLUDE_PREFIX)
	bufSize := uint32(16 * 1024)
	var buf []byte

	for attempt := 0; attempt < 3; attempt++ {
		buf = make([]byte, bufSize)
		aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_INET, flags, 0, aa, &bufSize)
		if err == nil {
			return parseAdapters(aa), nil
		}
		if err != windows.ERROR_BUFFER_OVERFLOW {
			return NetworkSummary{}, fmt.Errorf("GetAdaptersAddresses: %w", err)
		}
	}

	return NetworkSummary{}, fmt.Errorf("GetAdaptersAddresses: buffer overflow after retries")
}

func parseAdapters(head *windows.IpAdapterAddresses) NetworkSummary {
	summary := NetworkSummary{
		Adapters: make([]AdapterInfo, 0),
	}

	var candidates []adapterCandidate

	for cur := head; cur != nil; cur = cur.Next {
		if cur.IfType == ifTypeSoftwareLoop {
			continue
		}

		name := windows.UTF16PtrToString(cur.FriendlyName)
		desc := windows.UTF16PtrToString(cur.Description)
		mac := formatMAC(cur.PhysicalAddress[:], cur.PhysicalAddressLength)
		adapterType := classifyAdapterType(cur.IfType)
		isVirtual := isVirtualAdapter(cur.IfType, name, desc)
		isUp := cur.OperStatus == ifOperStatusUp
		speedMbps := computeLinkSpeedMbps(cur.TransmitLinkSpeed, cur.ReceiveLinkSpeed)

		ipv4, prefixLen := extractPrimaryUnicastIPv4(cur.FirstUnicastAddress)
		gateway := extractFirstIPv4Gateway(cur.FirstGatewayAddress)
		dnsServers := extractIPv4DNS(cur.FirstDnsServerAddress)

		if !isVirtual && adapterType == "wifi" {
			summary.HasWiFiAdapter = true
		}

		info := AdapterInfo{
			Name:          name,
			Description:   desc,
			MAC:           mac,
			IPv4:          ipv4,
			PrefixLen:     prefixLen,
			Gateway:       gateway,
			DNSServers:    dnsServers,
			Type:          adapterType,
			LinkSpeedMbps: speedMbps,
			IfIndex:       cur.IfIndex,
			IsUp:          isUp,
			IsPrimary:     false,
			IsVirtual:     isVirtual,
		}

		idx := len(summary.Adapters)
		summary.Adapters = append(summary.Adapters, info)

		if isUp && ipv4 != "" && gateway != "" && gateway != "0.0.0.0" {
			candidates = append(candidates, adapterCandidate{
				index:  idx,
				metric: cur.Ipv4Metric,
			})
		}
	}

	if len(candidates) > 0 {
		best := candidates[0]
		for _, c := range candidates[1:] {
			if isBetterPrimary(summary.Adapters[c.index], c.metric, summary.Adapters[best.index], best.metric) {
				best = c
			}
		}

		summary.Adapters[best.index].IsPrimary = true
		primaryCopy := summary.Adapters[best.index]
		summary.Connected = true
		summary.Primary = &primaryCopy
		summary.GatewayIP = primaryCopy.Gateway
		summary.LocalIP = primaryCopy.IPv4
		summary.AdapterMAC = primaryCopy.MAC
		summary.AdapterName = primaryCopy.Name
		summary.LinkType = primaryCopy.Type
		summary.LinkSpeedMbps = primaryCopy.LinkSpeedMbps
	}

	return summary
}

func isBetterPrimary(a AdapterInfo, aMetric uint32, b AdapterInfo, bMetric uint32) bool {
	if a.IsVirtual != b.IsVirtual {
		return !a.IsVirtual
	}
	if aMetric != bMetric {
		return aMetric < bMetric
	}
	return a.LinkSpeedMbps > b.LinkSpeedMbps
}

func classifyAdapterType(ifType uint32) string {
	switch ifType {
	case ifTypeIEEE80211:
		return "wifi"
	case ifTypeEthernetCsmacd:
		return "ethernet"
	default:
		return "other"
	}
}

func isVirtualAdapter(ifType uint32, name, desc string) bool {
	if ifType == ifTypeTunnel {
		return true
	}
	combined := strings.ToLower(name + " " + desc)
	keywords := []string{
		"virtual",
		"vethernet",
		"vmware",
		"virtualbox",
		"hyper-v",
		"tap-windows",
		"wintun",
		"wireguard",
		"tailscale",
		"zerotier",
		"loopback",
		"bluetooth",
	}
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func formatMAC(addr []byte, length uint32) string {
	if length != 6 || len(addr) < 6 {
		return ""
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		addr[0], addr[1], addr[2], addr[3], addr[4], addr[5])
}

func computeLinkSpeedMbps(tx, rx uint64) uint64 {
	var bps uint64
	if tx > 0 && tx != unknownLinkSpeedSentinel {
		bps = tx
	} else if rx > 0 && rx != unknownLinkSpeedSentinel {
		bps = rx
	}
	if bps == 0 {
		return 0
	}
	return bps / 1_000_000
}

func sockaddrToIPv4(sa windows.SocketAddress) string {
	if sa.Sockaddr == nil {
		return ""
	}
	raw := (*windows.RawSockaddrAny)(unsafe.Pointer(sa.Sockaddr))
	if raw.Addr.Family != windows.AF_INET {
		return ""
	}
	in4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(sa.Sockaddr))
	return net.IP(in4.Addr[:]).String()
}

func extractPrimaryUnicastIPv4(head *windows.IpAdapterUnicastAddress) (string, uint8) {
	for cur := head; cur != nil; cur = cur.Next {
		ip := sockaddrToIPv4(cur.Address)
		if ip != "" && ip != "0.0.0.0" {
			return ip, cur.OnLinkPrefixLength
		}
	}
	return "", 0
}

func extractFirstIPv4Gateway(head *windows.IpAdapterGatewayAddress) string {
	for cur := head; cur != nil; cur = cur.Next {
		ip := sockaddrToIPv4(cur.Address)
		if ip != "" && ip != "0.0.0.0" {
			return ip
		}
	}
	return ""
}

func extractIPv4DNS(head *windows.IpAdapterDnsServerAdapter) []string {
	out := make([]string, 0)
	for cur := head; cur != nil; cur = cur.Next {
		ip := sockaddrToIPv4(cur.Address)
		if ip != "" && ip != "0.0.0.0" {
			out = append(out, ip)
		}
	}
	return out
}
