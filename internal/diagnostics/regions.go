package diagnostics

import (
	"context"
	"math"
	"net"
	"sort"
	"sync"
	"time"
)

// RegionEndpoint defines a TCP-443 latency probe endpoint representing a gaming/voice region.
type RegionEndpoint struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Region   string `json:"region"`
	Category string `json:"category"`
	Address  string `json:"address"`
}

// RegionPingResult holds latency and jitter metrics for a single tested region.
type RegionPingResult struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Region    string  `json:"region"`
	Category  string  `json:"category"`
	Host      string  `json:"host"`
	MinMs     float64 `json:"minMs"`
	AvgMs     float64 `json:"avgMs"`
	JitterMs  float64 `json:"jitterMs"`
	Reachable bool    `json:"reachable"`
	Grade     string  `json:"grade"` // "great" | "good" | "fair" | "high"
}

// RegionBenchmarkResult aggregates ranked region latency results.
type RegionBenchmarkResult struct {
	CompletedAt string             `json:"completedAt"`
	BestRegion  string             `json:"bestRegion"`
	Results     []RegionPingResult `json:"results"`
}

// DialProbeFunc measures TCP handshake RTT in milliseconds to address (host:port).
type DialProbeFunc func(ctx context.Context, address string, timeout time.Duration) (float64, error)

// DefaultRegionEndpoints returns major gaming/voice and regional cloud endpoints.
func DefaultRegionEndpoints() []RegionEndpoint {
	return []RegionEndpoint{
		{ID: "cf-local", Name: "Cloudflare Edge", Region: "Nodo Local / Anycast", Category: "Edge", Address: "1.1.1.1:443"},
		{ID: "google-edge", Name: "Google Edge", Region: "Nodo Local / Anycast", Category: "Edge", Address: "8.8.8.8:443"},
		{ID: "discord", Name: "Discord Voice / Gateway", Region: "Voz & Chat", Category: "Gaming", Address: "discord.com:443"},
		{ID: "steam", Name: "Steam / Valve Edge", Region: "Plataforma Gaming", Category: "Gaming", Address: "store.steampowered.com:443"},
		{ID: "br-sp", Name: "Brasil (São Paulo)", Region: "LATAM Sur / BR", Category: "Región", Address: "dynamodb.sa-east-1.amazonaws.com:443"},
		{ID: "us-east", Name: "USA Este (Virginia)", Region: "Norteamérica Este", Category: "Región", Address: "dynamodb.us-east-1.amazonaws.com:443"},
		{ID: "us-west", Name: "USA Oeste (California)", Region: "Norteamérica Oeste", Category: "Región", Address: "dynamodb.us-west-1.amazonaws.com:443"},
		{ID: "eu-frankfurt", Name: "Europa (Frankfurt)", Region: "Europa Central", Category: "Región", Address: "dynamodb.eu-central-1.amazonaws.com:443"},
	}
}

// RegionBenchmarker measures TCP-443 RTT and jitter to major gaming and regional nodes.
type RegionBenchmarker struct {
	dialFn    DialProbeFunc
	endpoints []RegionEndpoint
	timeout   time.Duration
}

// NewRegionBenchmarker creates a production RegionBenchmarker using TCP 443 dial probes.
func NewRegionBenchmarker() *RegionBenchmarker {
	return NewRegionBenchmarkerWithDialer(defaultTCPDialProbe)
}

// NewRegionBenchmarkerWithDialer creates a RegionBenchmarker with an injectable dialer for testing.
func NewRegionBenchmarkerWithDialer(fn DialProbeFunc) *RegionBenchmarker {
	return &RegionBenchmarker{
		dialFn:    fn,
		endpoints: DefaultRegionEndpoints(),
		timeout:   1800 * time.Millisecond,
	}
}

// Run probes all configured endpoints in parallel (3 samples each) and sorts fastest first.
func (b *RegionBenchmarker) Run(ctx context.Context) RegionBenchmarkResult {
	results := make([]RegionPingResult, len(b.endpoints))
	var wg sync.WaitGroup

	for idx, ep := range b.endpoints {
		wg.Add(1)
		go func(i int, endpoint RegionEndpoint) {
			defer wg.Done()
			results[i] = b.probeEndpoint(ctx, endpoint)
		}(idx, ep)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Reachable != results[j].Reachable {
			return results[i].Reachable
		}
		if results[i].AvgMs != results[j].AvgMs {
			return results[i].AvgMs < results[j].AvgMs
		}
		return results[i].JitterMs < results[j].JitterMs
	})

	best := ""
	for _, r := range results {
		if r.Reachable && r.Category == "Región" {
			best = r.Name
			break
		}
	}
	if best == "" && len(results) > 0 && results[0].Reachable {
		best = results[0].Name
	}

	return RegionBenchmarkResult{
		CompletedAt: time.Now().Format(time.RFC3339),
		BestRegion:  best,
		Results:     results,
	}
}

func (b *RegionBenchmarker) probeEndpoint(ctx context.Context, ep RegionEndpoint) RegionPingResult {
	rtts := make([]float64, 0, 3)
	for s := 0; s < 3; s++ {
		if ctx.Err() != nil {
			break
		}
		rtt, err := b.dialFn(ctx, ep.Address, b.timeout)
		if err == nil && rtt > 0 {
			rtts = append(rtts, rtt)
		}
	}

	host, _, err := net.SplitHostPort(ep.Address)
	if err != nil {
		host = ep.Address
	}

	res := RegionPingResult{
		ID:       ep.ID,
		Name:     ep.Name,
		Region:   ep.Region,
		Category: ep.Category,
		Host:     host,
		Grade:    "high",
	}
	if len(rtts) == 0 {
		return res
	}

	minV := rtts[0]
	var sum, jitSum float64
	for i, v := range rtts {
		sum += v
		if v < minV {
			minV = v
		}
		if i > 0 {
			jitSum += math.Abs(v - rtts[i-1])
		}
	}

	avg := round1(sum / float64(len(rtts)))
	jit := 0.0
	if len(rtts) > 1 {
		jit = round1(jitSum / float64(len(rtts)-1))
	}

	res.Reachable = true
	res.MinMs = round1(minV)
	res.AvgMs = avg
	res.JitterMs = jit
	res.Grade = gradeRegionLatency(avg)
	return res
}

func gradeRegionLatency(avgMs float64) string {
	switch {
	case avgMs <= 45:
		return "great"
	case avgMs <= 90:
		return "good"
	case avgMs <= 165:
		return "fair"
	default:
		return "high"
	}
}

func defaultTCPDialProbe(ctx context.Context, address string, timeout time.Duration) (float64, error) {
	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp4", address)
	elapsed := time.Since(start)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()

	rttMs := math.Round(float64(elapsed.Microseconds())/100.0) / 10.0
	if rttMs < 0.1 {
		rttMs = 0.1
	}
	return rttMs, nil
}
