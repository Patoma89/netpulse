package privacy

import (
	"net"
	"strings"
)

// MaskIP redacts the host portions of IPv4 and IPv6 addresses for safe sharing.
func MaskIP(ip string) string {
	trimmed := strings.TrimSpace(ip)
	if trimmed == "" {
		return ""
	}

	parsed := net.ParseIP(trimmed)
	if parsed == nil {
		return "•••.•••.•••.•••"
	}

	if v4 := parsed.To4(); v4 != nil {
		parts := strings.Split(v4.String(), ".")
		if len(parts) == 4 {
			return parts[0] + "." + parts[1] + ".•••.•••"
		}
	}

	// IPv6: preserve first 2 hextets when present and mask remaining interface/subnet segments.
	full := parsed.String()
	segments := strings.Split(full, ":")
	if len(segments) >= 2 {
		return segments[0] + ":" + segments[1] + ":••••:••••:••••"
	}
	return "••••:••••:••••"
}

// MaskMAC preserves the OUI vendor prefix (first 3 octets) and masks the device identifier.
func MaskMAC(mac string) string {
	trimmed := strings.TrimSpace(mac)
	if trimmed == "" {
		return ""
	}

	normalized := strings.ReplaceAll(strings.ToUpper(trimmed), "-", ":")
	normalized = strings.ReplaceAll(normalized, ".", ":")
	parts := strings.Split(normalized, ":")
	if len(parts) == 6 && len(parts[0]) == 2 && len(parts[1]) == 2 && len(parts[2]) == 2 {
		return parts[0] + ":" + parts[1] + ":" + parts[2] + ":••:••:••"
	}

	// Handle Cisco-style 3-group MACs (AABB.CCDD.EEFF)
	if len(parts) == 3 && len(parts[0]) == 4 && len(parts[1]) == 4 {
		return parts[0][:2] + ":" + parts[0][2:] + ":" + parts[1][:2] + ":••:••:••"
	}

	return "••:••:••:••:••:••"
}

// MaskSSID hides the middle characters of a Wi-Fi network name to avoid location leaks.
func MaskSSID(ssid string) string {
	trimmed := strings.TrimSpace(ssid)
	if trimmed == "" {
		return ""
	}

	runes := []rune(trimmed)
	n := len(runes)
	if n <= 4 {
		return strings.Repeat("•", n)
	}

	prefix := string(runes[:2])
	suffix := string(runes[n-1:])
	maskedLen := n - 3
	if maskedLen > 8 {
		maskedLen = 8
	}
	return prefix + strings.Repeat("•", maskedLen) + suffix
}
