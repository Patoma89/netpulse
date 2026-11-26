package radar

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"time"
)

// nbstatQuery is a standard RFC 1002 NetBIOS Node Status (NBSTAT *) UDP query packet.
var nbstatQuery = []byte{
	0xA2, 0x48, // Transaction ID
	0x00, 0x00, // Flags: standard query
	0x00, 0x01, // Questions: 1
	0x00, 0x00, // Answer RRs
	0x00, 0x00, // Authority RRs
	0x00, 0x00, // Additional RRs
	0x20, // Name length: 32
	'C', 'K', 'A', 'A', 'A', 'A', 'A', 'A',
	'A', 'A', 'A', 'A', 'A', 'A', 'A', 'A',
	'A', 'A', 'A', 'A', 'A', 'A', 'A', 'A',
	'A', 'A', 'A', 'A', 'A', 'A', 'A', 'A',
	0x00,       // Root label terminator
	0x00, 0x21, // Type: NBSTAT (0x0021)
	0x00, 0x01, // Class: IN (0x0001)
}

// QueryNetBIOSName queries UDP port 137 on a single LAN host to retrieve its NetBIOS machine name.
func QueryNetBIOSName(ctx context.Context, ip string, timeout time.Duration) string {
	if ctx.Err() != nil {
		return ""
	}
	if timeout <= 0 {
		timeout = 250 * time.Millisecond
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp4", net.JoinHostPort(ip, "137"))
	if err != nil {
		return ""
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(nbstatQuery); err != nil {
		return ""
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || n < 57 {
		return ""
	}
	return ParseNetBIOSResponse(buf[:n])
}

// ParseNetBIOSResponse extracts the primary unique host name from a raw NBSTAT UDP reply.
func ParseNetBIOSResponse(pkt []byte) string {
	if len(pkt) < 57 {
		return ""
	}
	// Verify at least 1 answer RR (bytes 6..7)
	ansCount := binary.BigEndian.Uint16(pkt[6:8])
	if ansCount == 0 {
		return ""
	}

	numNames := int(pkt[56])
	offset := 57
	if numNames <= 0 || offset+numNames*18 > len(pkt) {
		return ""
	}

	var fallback string
	for i := 0; i < numNames; i++ {
		rec := pkt[offset : offset+18]
		offset += 18

		rawName := strings.TrimRight(string(rec[:15]), " \x00\t")
		suffix := rec[15]
		flags := binary.BigEndian.Uint16(rec[16:18])
		isGroup := flags&0x8000 != 0

		if rawName == "" || isGroup || strings.HasPrefix(rawName, "\x01\x02__MSBROWSE__") {
			continue
		}
		if suffix == 0x00 {
			return rawName
		}
		if suffix == 0x20 && fallback == "" {
			fallback = rawName
		}
	}
	return fallback
}
