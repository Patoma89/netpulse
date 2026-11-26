package ping

import (
	"context"
	"net"
	"time"
)

// Result holds the outcome of a single ICMP echo probe.
type Result struct {
	Target    string  `json:"target"`
	RTTMs     float64 `json:"rttMs"`
	TTL       uint8   `json:"ttl"`
	Success   bool    `json:"success"`
	Status    uint32  `json:"status"`
	Error     string  `json:"error,omitempty"`
	Timestamp int64   `json:"timestamp"`
}

// Pinger sends ICMP echo requests to a target host.
type Pinger interface {
	Ping(ctx context.Context, target string, timeout time.Duration) Result
}

// ParseIPv4ToUint32 converts an IPv4 address into the little-endian Win32 IPAddr representation.
func ParseIPv4ToUint32(ip net.IP) (uint32, bool) {
	if ip == nil {
		return 0, false
	}
	v4 := ip.To4()
	if v4 == nil {
		return 0, false
	}
	return uint32(v4[0]) | uint32(v4[1])<<8 | uint32(v4[2])<<16 | uint32(v4[3])<<24, true
}
