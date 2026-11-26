package radar

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SSDPInfo holds metadata discovered via SSDP/UPnP M-SEARCH replies.
type SSDPInfo struct {
	FriendlyName string
	Server       string
	ServiceType  string
	Location     string
}

var (
	reFriendlyName = regexp.MustCompile(`(?is)<friendlyName>\s*([^<]+?)\s*</friendlyName>`)
	reModelName    = regexp.MustCompile(`(?is)<modelName>\s*([^<]+?)\s*</modelName>`)
)

// ParseSSDPResponse parses HTTP-like SSDP M-SEARCH response headers.
func ParseSSDPResponse(raw string) SSDPInfo {
	var info SSDPInfo
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(line[:colon]))
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "SERVER":
			info.Server = val
		case "ST":
			info.ServiceType = val
		case "LOCATION":
			info.Location = val
		}
	}
	return info
}

// DiscoverSSDP sends an SSDP M-SEARCH multicast probe and gathers device descriptions.
func DiscoverSSDP(ctx context.Context, localIP string, duration time.Duration) map[string]SSDPInfo {
	out := make(map[string]SSDPInfo)
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

	msearch := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 1\r\n" +
		"ST: ssdp:all\r\n\r\n"

	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	_, _ = conn.WriteToUDP([]byte(msearch), dst)

	deadline := time.Now().Add(duration)
	_ = conn.SetReadDeadline(deadline)

	buf := make([]byte, 4096)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		n, addr, readErr := conn.ReadFromUDP(buf)
		if n > 0 && addr != nil {
			ip := addr.IP.String()
			parsed := ParseSSDPResponse(string(buf[:n]))
			existing := out[ip]
			if existing.Location == "" && parsed.Location != "" {
				existing.Location = parsed.Location
			}
			if existing.Server == "" && parsed.Server != "" {
				existing.Server = parsed.Server
			}
			if existing.ServiceType == "" && parsed.ServiceType != "" {
				existing.ServiceType = parsed.ServiceType
			}
			out[ip] = existing
		}
		if readErr != nil {
			break
		}
	}

	// Fetch <friendlyName> / <modelName> concurrently with strict local-IP SSRF guard and 350ms timeout.
	client := &http.Client{Timeout: 350 * time.Millisecond}
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for ip, info := range out {
		if info.Location == "" || !isSafeLocalLocation(info.Location, ip) {
			continue
		}
		wg.Add(1)
		go func(targetIP, loc string) {
			defer wg.Done()
			name := fetchSSDPFriendlyName(ctx, client, loc)
			if name != "" {
				mu.Lock()
				cur := out[targetIP]
				cur.FriendlyName = name
				out[targetIP] = cur
				mu.Unlock()
			}
		}(ip, info.Location)
	}
	wg.Wait()

	return out
}

func isSafeLocalLocation(rawURL, expectedIP string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	if host != expectedIP {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

func fetchSSDPFriendlyName(ctx context.Context, client *http.Client, rawURL string) string {
	reqCtx, cancel := context.WithTimeout(ctx, 350*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil || len(body) == 0 {
		return ""
	}
	text := string(body)
	if m := reFriendlyName.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := reModelName.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
