package optimizer

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// DNSProvider describes a DNS resolver pair and whether it is currently active on the PC.
type DNSProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Tagline   string `json:"tagline"`
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	IsCurrent bool   `json:"isCurrent"`
}

// DNSResult summarizes parallel UDP query latency and reliability for one DNS provider.
type DNSResult struct {
	Provider     DNSProvider `json:"provider"`
	MedianMs     float64     `json:"medianMs"`
	MinMs        float64     `json:"minMs"`
	AvgMs        float64     `json:"avgMs"`
	JitterMs     float64     `json:"jitterMs"`
	SuccessCount int         `json:"successCount"`
	TotalQueries int         `json:"totalQueries"`
	Reachable    bool        `json:"reachable"`
	Recommended  bool        `json:"recommended"`
}

// DNSBenchmarkResult holds the ranked DNS benchmark output for the active adapter.
type DNSBenchmarkResult struct {
	AdapterName string      `json:"adapterName"`
	IfIndex     uint32      `json:"ifIndex"`
	CurrentDNS  []string    `json:"currentDns"`
	FastestID   string      `json:"fastestId"`
	FastestName string      `json:"fastestName"`
	CompletedAt string      `json:"completedAt"`
	Results     []DNSResult `json:"results"`
}

// DNSQueryFunc resolves domain against serverIP and returns RTT in milliseconds.
type DNSQueryFunc func(ctx context.Context, serverIP, domain string, timeout time.Duration) (float64, error)

var benchmarkDomains = []string{
	"cloudflare.com",
	"google.com",
	"discord.com",
	"steampowered.com",
	"wikipedia.org",
}

// DefaultDNSProviders returns the built-in fast & privacy-respecting DNS resolvers.
func DefaultDNSProviders() []DNSProvider {
	return []DNSProvider{
		{
			ID:        "cloudflare",
			Name:      "Cloudflare",
			Tagline:   "Ultra rápido y privado (1.1.1.1)",
			Primary:   "1.1.1.1",
			Secondary: "1.0.0.1",
		},
		{
			ID:        "cloudflare-sec",
			Name:      "Cloudflare Malware Block",
			Tagline:   "Bloquea malware y phishing (1.1.1.2)",
			Primary:   "1.1.1.2",
			Secondary: "1.0.0.2",
		},
		{
			ID:        "google",
			Name:      "Google Public DNS",
			Tagline:   "Baja latencia global y alta estabilidad",
			Primary:   "8.8.8.8",
			Secondary: "8.8.4.4",
		},
		{
			ID:        "quad9",
			Name:      "Quad9",
			Tagline:   "Seguridad contra amenazas sin registro",
			Primary:   "9.9.9.9",
			Secondary: "149.112.112.112",
		},
		{
			ID:        "adguard",
			Name:      "AdGuard DNS",
			Tagline:   "Filtra publicidad y rastreadores por DNS",
			Primary:   "94.140.14.14",
			Secondary: "94.140.15.15",
		},
		{
			ID:        "opendns",
			Name:      "Cisco OpenDNS",
			Tagline:   "Resolución confiable con caché distribuida",
			Primary:   "208.67.222.222",
			Secondary: "208.67.220.220",
		},
	}
}

// DNSBenchmarker executes parallel UDP DNS latency benchmarks.
type DNSBenchmarker struct {
	queryFn DNSQueryFunc
	domains []string
	timeout time.Duration
}

// NewDNSBenchmarker constructs a production DNSBenchmarker using UDP port 53 queries.
func NewDNSBenchmarker() *DNSBenchmarker {
	return NewDNSBenchmarkerWithQuery(QueryDNSServer)
}

// NewDNSBenchmarkerWithQuery constructs a DNSBenchmarker with an injectable query function for tests.
func NewDNSBenchmarkerWithQuery(fn DNSQueryFunc) *DNSBenchmarker {
	return &DNSBenchmarker{
		queryFn: fn,
		domains: benchmarkDomains,
		timeout: 1200 * time.Millisecond,
	}
}

// Run benchmarks all built-in resolvers plus the current system/ISP DNS in parallel.
func (b *DNSBenchmarker) Run(ctx context.Context, adapterName string, ifIndex uint32, currentDNS []string) DNSBenchmarkResult {
	providers := DefaultDNSProviders()

	currentSet := make(map[string]bool, len(currentDNS))
	for _, ip := range currentDNS {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			currentSet[trimmed] = true
		}
	}

	matchedCurrent := false
	for i := range providers {
		if currentSet[providers[i].Primary] || currentSet[providers[i].Secondary] {
			providers[i].IsCurrent = true
			matchedCurrent = true
		}
	}

	if !matchedCurrent && len(currentDNS) > 0 && strings.TrimSpace(currentDNS[0]) != "" {
		sec := ""
		if len(currentDNS) > 1 {
			sec = strings.TrimSpace(currentDNS[1])
		}
		providers = append(providers, DNSProvider{
			ID:        "current-isp",
			Name:      "Actual (ISP / Router)",
			Tagline:   "Servidor DNS configurado actualmente en tu PC",
			Primary:   strings.TrimSpace(currentDNS[0]),
			Secondary: sec,
			IsCurrent: true,
		})
	}

	results := make([]DNSResult, len(providers))
	var wg sync.WaitGroup
	for idx, prov := range providers {
		wg.Add(1)
		go func(i int, p DNSProvider) {
			defer wg.Done()
			results[i] = b.benchmarkOne(ctx, p)
		}(idx, prov)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		a, bRes := results[i], results[j]
		if a.Reachable != bRes.Reachable {
			return a.Reachable
		}
		if a.SuccessCount != bRes.SuccessCount && (a.SuccessCount < 3 || bRes.SuccessCount < 3) {
			return a.SuccessCount > bRes.SuccessCount
		}
		if a.MedianMs != bRes.MedianMs {
			return a.MedianMs < bRes.MedianMs
		}
		return a.JitterMs < bRes.JitterMs
	})

	fastestID := ""
	fastestName := ""
	for i := range results {
		if results[i].Reachable && results[i].SuccessCount >= 3 {
			results[i].Recommended = true
			fastestID = results[i].Provider.ID
			fastestName = results[i].Provider.Name
			break
		}
	}

	cleanCurrent := make([]string, 0, len(currentDNS))
	for _, ip := range currentDNS {
		if t := strings.TrimSpace(ip); t != "" {
			cleanCurrent = append(cleanCurrent, t)
		}
	}

	return DNSBenchmarkResult{
		AdapterName: adapterName,
		IfIndex:     ifIndex,
		CurrentDNS:  cleanCurrent,
		FastestID:   fastestID,
		FastestName: fastestName,
		CompletedAt: time.Now().Format(time.RFC3339),
		Results:     results,
	}
}

func (b *DNSBenchmarker) benchmarkOne(ctx context.Context, p DNSProvider) DNSResult {
	rtts := make([]float64, 0, len(b.domains))
	for _, domain := range b.domains {
		if ctx.Err() != nil {
			break
		}
		rtt, err := b.queryFn(ctx, p.Primary, domain, b.timeout)
		if err == nil && rtt > 0 {
			rtts = append(rtts, rtt)
		}
	}

	res := DNSResult{
		Provider:     p,
		TotalQueries: len(b.domains),
		SuccessCount: len(rtts),
		Reachable:    len(rtts) > 0,
	}
	if len(rtts) == 0 {
		return res
	}

	var sum, jitterSum float64
	for i, v := range rtts {
		sum += v
		if i > 0 {
			jitterSum += math.Abs(v - rtts[i-1])
		}
	}

	sorted := make([]float64, len(rtts))
	copy(sorted, rtts)
	sort.Float64s(sorted)

	res.MinMs = round1(sorted[0])
	res.AvgMs = round1(sum / float64(len(rtts)))
	if len(sorted)%2 == 1 {
		res.MedianMs = round1(sorted[len(sorted)/2])
	} else {
		mid := len(sorted) / 2
		res.MedianMs = round1((sorted[mid-1] + sorted[mid]) / 2.0)
	}
	if len(rtts) > 1 {
		res.JitterMs = round1(jitterSum / float64(len(rtts)-1))
	}
	return res
}

// BuildDNSQuery encodes a standard RFC 1035 A-record UDP query packet.
func BuildDNSQuery(domain string, txID uint16) []byte {
	pkt := make([]byte, 12, 64)
	binary.BigEndian.PutUint16(pkt[0:2], txID)
	binary.BigEndian.PutUint16(pkt[2:4], 0x0100) // Standard query, recursion desired
	binary.BigEndian.PutUint16(pkt[4:6], 1)      // QDCOUNT = 1

	for _, label := range strings.Split(strings.TrimSuffix(strings.TrimSpace(domain), "."), ".") {
		if label == "" || len(label) > 63 {
			continue
		}
		pkt = append(pkt, byte(len(label)))
		pkt = append(pkt, []byte(label)...)
	}
	pkt = append(pkt, 0x00)       // root label
	pkt = append(pkt, 0x00, 0x01) // QTYPE = A
	pkt = append(pkt, 0x00, 0x01) // QCLASS = IN
	return pkt
}

// QueryDNSServer measures real UDP port 53 resolution latency for a domain against serverIP.
func QueryDNSServer(ctx context.Context, serverIP, domain string, timeout time.Duration) (float64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp4", net.JoinHostPort(serverIP, "53"))
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	txID := uint16(time.Now().UnixNano() & 0xFFFF)
	if txID == 0 {
		txID = 0x4E50
	}
	query := BuildDNSQuery(domain, txID)

	_ = conn.SetDeadline(time.Now().Add(timeout))
	start := time.Now()
	if _, err := conn.Write(query); err != nil {
		return 0, err
	}

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	elapsed := time.Since(start)
	if err != nil {
		return 0, err
	}
	if n < 12 || binary.BigEndian.Uint16(buf[0:2]) != txID {
		return 0, errors.New("invalid DNS response")
	}

	rttMs := math.Round(float64(elapsed.Microseconds())/100.0) / 10.0
	if rttMs < 0.1 {
		rttMs = 0.1
	}
	return rttMs, nil
}

func round1(v float64) float64 {
	return math.Round(v*10.0) / 10.0
}
