package radar

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"time"
)

// MDNSInfo holds hostname and service identifiers extracted from mDNS responses.
type MDNSInfo struct {
	Hostname string
	Services []string
}

var mdnsServiceQueries = []string{
	"_services._dns-sd._udp.local",
	"_googlecast._tcp.local",
	"_airplay._tcp.local",
	"_hap._tcp.local",
	"_printer._tcp.local",
	"_smb._tcp.local",
	"_companion-link._tcp.local",
}

// BuildMDNSQueryPacket builds a standard DNS-SD PTR query packet for a service name.
func BuildMDNSQueryPacket(service string) []byte {
	pkt := make([]byte, 12, 64)
	binary.BigEndian.PutUint16(pkt[4:6], 1) // 1 question

	for _, label := range strings.Split(strings.TrimSuffix(service, "."), ".") {
		if label == "" || len(label) > 63 {
			continue
		}
		pkt = append(pkt, byte(len(label)))
		pkt = append(pkt, []byte(label)...)
	}
	pkt = append(pkt, 0x00)       // root
	pkt = append(pkt, 0x00, 0x0C) // Type PTR
	pkt = append(pkt, 0x00, 0x01) // Class IN
	return pkt
}

// ParseMDNSPacket safely decodes a raw mDNS UDP response into hostname and service tags.
func ParseMDNSPacket(pkt []byte) MDNSInfo {
	var info MDNSInfo
	if len(pkt) < 12 {
		return info
	}

	qdCount := int(binary.BigEndian.Uint16(pkt[4:6]))
	anCount := int(binary.BigEndian.Uint16(pkt[6:8]))
	nsCount := int(binary.BigEndian.Uint16(pkt[8:10]))
	arCount := int(binary.BigEndian.Uint16(pkt[10:12]))

	offset := 12
	for i := 0; i < qdCount && offset < len(pkt); i++ {
		_, next, ok := readDNSName(pkt, offset)
		if !ok || next+4 > len(pkt) {
			return info
		}
		offset = next + 4
	}

	seenSvc := make(map[string]struct{})
	addSvc := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seenSvc[s]; !ok {
			seenSvc[s] = struct{}{}
			info.Services = append(info.Services, s)
		}
	}

	totalRRs := anCount + nsCount + arCount
	for i := 0; i < totalRRs && offset < len(pkt); i++ {
		rrName, next, ok := readDNSName(pkt, offset)
		if !ok || next+10 > len(pkt) {
			break
		}
		rrType := binary.BigEndian.Uint16(pkt[next : next+2])
		rdLen := int(binary.BigEndian.Uint16(pkt[next+8 : next+10]))
		rdataStart := next + 10
		rdataEnd := rdataStart + rdLen
		if rdataEnd > len(pkt) {
			break
		}

		switch rrType {
		case 1: // A record
			if clean := cleanMDNSHost(rrName); clean != "" && info.Hostname == "" {
				info.Hostname = clean
			}
		case 12: // PTR
			if svc := extractServiceTag(rrName); svc != "" {
				addSvc(svc)
			}
			if target, _, ok := readDNSName(pkt, rdataStart); ok {
				if svc := extractServiceTag(target); svc != "" {
					addSvc(svc)
				}
				if info.Hostname == "" && !strings.HasPrefix(target, "_") {
					if idx := strings.Index(target, "._"); idx > 0 {
						info.Hostname = strings.TrimSpace(target[:idx])
					}
				}
			}
		case 33: // SRV
			if rdataStart+6 <= rdataEnd {
				if target, _, ok := readDNSName(pkt, rdataStart+6); ok {
					if clean := cleanMDNSHost(target); clean != "" {
						info.Hostname = clean
					}
				}
			}
		}

		offset = rdataEnd
	}

	return info
}

// DiscoverMDNS sends lightweight multicast DNS-SD PTR probes and collects responder metadata.
func DiscoverMDNS(ctx context.Context, localIP string, duration time.Duration) map[string]MDNSInfo {
	out := make(map[string]MDNSInfo)
	if ctx.Err() != nil {
		return out
	}
	if duration <= 0 {
		duration = 800 * time.Millisecond
	}

	var bindIP net.IP
	if parsed := net.ParseIP(localIP).To4(); parsed != nil {
		bindIP = parsed
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		return out
	}
	defer conn.Close()

	dst := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	for _, svc := range mdnsServiceQueries {
		_, _ = conn.WriteToUDP(BuildMDNSQueryPacket(svc), dst)
	}

	deadline := time.Now().Add(duration)
	_ = conn.SetReadDeadline(deadline)

	buf := make([]byte, 4096)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		n, addr, readErr := conn.ReadFromUDP(buf)
		if n > 12 && addr != nil {
			ip := addr.IP.String()
			parsed := ParseMDNSPacket(buf[:n])
			if parsed.Hostname == "" && len(parsed.Services) == 0 {
				continue
			}
			cur := out[ip]
			if cur.Hostname == "" && parsed.Hostname != "" {
				cur.Hostname = parsed.Hostname
			}
			cur.Services = mergeUniqueStrings(cur.Services, parsed.Services)
			out[ip] = cur
		}
		if readErr != nil {
			break
		}
	}
	return out
}

func readDNSName(pkt []byte, offset int) (string, int, bool) {
	if offset < 0 || offset >= len(pkt) {
		return "", offset, false
	}

	var labels []string
	cur := offset
	nextAfterFirstPtr := -1
	jumps := 0

	for jumps < 16 {
		if cur >= len(pkt) {
			return "", cur, false
		}
		length := int(pkt[cur])
		if length == 0 {
			if nextAfterFirstPtr == -1 {
				nextAfterFirstPtr = cur + 1
			}
			return strings.Join(labels, "."), nextAfterFirstPtr, true
		}
		if length&0xC0 == 0xC0 {
			if cur+1 >= len(pkt) {
				return "", cur, false
			}
			ptr := int(binary.BigEndian.Uint16(pkt[cur:cur+2]) & 0x3FFF)
			if ptr >= len(pkt) {
				return "", cur, false
			}
			if nextAfterFirstPtr == -1 {
				nextAfterFirstPtr = cur + 2
			}
			cur = ptr
			jumps++
			continue
		}
		if length > 63 || cur+1+length > len(pkt) {
			return "", cur, false
		}
		labels = append(labels, string(pkt[cur+1:cur+1+length]))
		cur += 1 + length
	}
	return "", cur, false
}

func cleanMDNSHost(name string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(name), ".")
	if trimmed == "" || strings.HasPrefix(trimmed, "_") || strings.Contains(trimmed, ".in-addr.arpa") {
		return ""
	}
	trimmed = strings.TrimSuffix(trimmed, ".local")
	if strings.Contains(trimmed, "._") {
		return ""
	}
	return trimmed
}

func extractServiceTag(name string) string {
	for _, part := range strings.Split(name, ".") {
		if strings.HasPrefix(part, "_") && part != "_tcp" && part != "_udp" && part != "_services" && part != "_dns-sd" {
			return part
		}
	}
	return ""
}
