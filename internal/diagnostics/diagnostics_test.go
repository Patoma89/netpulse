package diagnostics

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"netpulse/internal/ping"
)

type mockPinger struct {
	mu      sync.Mutex
	results map[string][]ping.Result
	idx     map[string]int
}

func newMockPinger(results map[string][]ping.Result) *mockPinger {
	return &mockPinger{
		results: results,
		idx:     make(map[string]int),
	}
}

func (m *mockPinger) Ping(ctx context.Context, target string, timeout time.Duration) ping.Result {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.results[target]
	if len(seq) == 0 {
		return ping.Result{Target: target, Success: true, RTTMs: 10.0, Timestamp: time.Now().UnixMilli()}
	}
	i := m.idx[target]
	m.idx[target] = i + 1
	res := seq[i%len(seq)]
	res.Target = target
	res.Timestamp = time.Now().UnixMilli()
	return res
}

func TestComputeHopStats(t *testing.T) {
	vals := []float64{2.0, 4.0, -1, 6.0, 8.0}
	oks := []bool{true, true, false, true, true}

	stats := ComputeHopStats("192.168.1.1", vals, oks)
	if stats.Sent != 5 || stats.Lost != 1 {
		t.Fatalf("unexpected sent/lost: %+v", stats)
	}
	if stats.LossPct != 20.0 {
		t.Fatalf("expected LossPct 20.0, got %v", stats.LossPct)
	}
	if stats.AvgMs != 5.0 || stats.MinMs != 2.0 || stats.MaxMs != 8.0 || stats.P95Ms != 8.0 {
		t.Fatalf("unexpected latency stats: %+v", stats)
	}
	if stats.JitterMs != 2.0 {
		t.Fatalf("expected JitterMs 2.0, got %v", stats.JitterMs)
	}
	if stats.CurrentMs != 8.0 || !stats.LastOK {
		t.Fatalf("expected CurrentMs 8.0 and LastOK true, got %+v", stats)
	}

	allLost := ComputeHopStats("1.1.1.1", []float64{-1, -1}, []bool{false, false})
	if allLost.LossPct != 100.0 || allLost.AvgMs != 0 || allLost.LastOK {
		t.Fatalf("unexpected all-lost stats: %+v", allLost)
	}
}

func TestEvaluateVerdictAllStates(t *testing.T) {
	// 1. Warming up
	v := EvaluateVerdict(HopStats{Sent: 2}, HopStats{Sent: 2})
	if v.Code != "warming_up" || v.Score != 100 {
		t.Fatalf("expected warming_up, got %+v", v)
	}

	// 2. Offline
	v = EvaluateVerdict(HopStats{Sent: 10, LossPct: 60}, HopStats{Sent: 10, LossPct: 60})
	if v.Code != "offline" || v.Score != 0 {
		t.Fatalf("expected offline, got %+v", v)
	}

	// 3. Optimal
	v = EvaluateVerdict(
		HopStats{Sent: 20, AvgMs: 1.8, P95Ms: 3.1, JitterMs: 0.6, LossPct: 0},
		HopStats{Sent: 20, AvgMs: 18.5, P95Ms: 24.0, JitterMs: 1.9, LossPct: 0},
	)
	if v.Code != "optimal" || v.Severity != "emerald" || v.Score < 85 || v.Score > 100 {
		t.Fatalf("expected optimal emerald verdict, got %+v", v)
	}

	// 4. Wi-Fi / LAN issue
	v = EvaluateVerdict(
		HopStats{Sent: 20, AvgMs: 22.0, P95Ms: 48.0, JitterMs: 14.0, LossPct: 5.0},
		HopStats{Sent: 20, AvgMs: 36.0, P95Ms: 62.0, JitterMs: 15.0, LossPct: 5.0},
	)
	if v.Code != "wifi_issue" || v.Score < 0 || v.Score > 100 {
		t.Fatalf("expected wifi_issue, got %+v", v)
	}

	// 5. ISP issue
	v = EvaluateVerdict(
		HopStats{Sent: 20, AvgMs: 1.5, P95Ms: 2.5, JitterMs: 0.4, LossPct: 0},
		HopStats{Sent: 20, AvgMs: 135.0, P95Ms: 180.0, JitterMs: 24.0, LossPct: 8.0},
	)
	if v.Code != "isp_issue" || v.Score < 0 || v.Score > 100 {
		t.Fatalf("expected isp_issue, got %+v", v)
	}

	// 6. Both issue
	v = EvaluateVerdict(
		HopStats{Sent: 20, AvgMs: 19.0, P95Ms: 38.0, JitterMs: 11.0, LossPct: 5.0},
		HopStats{Sent: 20, AvgMs: 150.0, P95Ms: 220.0, JitterMs: 32.0, LossPct: 15.0},
	)
	if v.Code != "both_issue" || v.Severity != "rose" || v.Score < 0 || v.Score > 100 {
		t.Fatalf("expected both_issue, got %+v", v)
	}
}

func TestDualHopMonitorLifecycle(t *testing.T) {
	pinger := newMockPinger(map[string][]ping.Result{
		"192.168.1.1": {{Success: true, RTTMs: 2.1}, {Success: true, RTTMs: 2.5}},
		"1.1.1.1":     {{Success: true, RTTMs: 18.2}, {Success: true, RTTMs: 20.4}},
	})

	var (
		mu      sync.Mutex
		updates int
	)
	mon := NewDualHopMonitor(pinger, 20*time.Millisecond, func(s DualHopSnapshot) {
		mu.Lock()
		updates++
		mu.Unlock()
	})

	mon.Start("192.168.1.1", "1.1.1.1")
	if !mon.IsRunning() {
		t.Fatalf("expected monitor to report running")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := mon.Snapshot()
		if len(snap.Samples) >= 4 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	snap := mon.Snapshot()
	if len(snap.Samples) < 4 {
		t.Fatalf("expected >=4 samples, got %d", len(snap.Samples))
	}
	if snap.Verdict.Code != "optimal" {
		t.Fatalf("expected optimal verdict, got %+v", snap.Verdict)
	}

	resetSnap := mon.Reset()
	if len(resetSnap.Samples) != 0 {
		t.Fatalf("expected 0 samples after Reset, got %d", len(resetSnap.Samples))
	}

	mon.Stop()
	if mon.IsRunning() {
		t.Fatalf("expected monitor to stop")
	}

	mu.Lock()
	countAtStop := updates
	mu.Unlock()
	time.Sleep(60 * time.Millisecond)
	mu.Lock()
	countLater := updates
	mu.Unlock()
	if countLater != countAtStop {
		t.Fatalf("expected no updates after Stop: before=%d after=%d", countAtStop, countLater)
	}
}

func TestGradeBufferbloat(t *testing.T) {
	cases := []struct {
		dlDelta float64
		ulDelta float64
		loss    float64
		want    string
	}{
		{2.5, 3.0, 0, "A+"},
		{15.0, 10.0, 0, "A"},
		{45.0, 20.0, 0, "B"},
		{120.0, 30.0, 0, "C"},
		{250.0, 80.0, 0, "D"},
		{450.0, 90.0, 0, "F"},
		{4.0, 4.0, 6.0, "C"},
		{4.0, 4.0, 20.0, "F"},
	}

	for _, tc := range cases {
		got, _, _, _ := GradeBufferbloat(tc.dlDelta, tc.ulDelta, tc.loss)
		if got != tc.want {
			t.Fatalf("GradeBufferbloat(%v, %v, %v) = %q, want %q", tc.dlDelta, tc.ulDelta, tc.loss, got, tc.want)
		}
	}
}

func TestBufferbloatRunnerWithMocksAndCancel(t *testing.T) {
	pinger := newMockPinger(map[string][]ping.Result{
		"1.1.1.1": {{Success: true, RTTMs: 12.0}},
	})

	runner := NewBufferbloatRunner(pinger)
	runner.idleDur = 120 * time.Millisecond
	runner.downloadDur = 150 * time.Millisecond
	runner.uploadDur = 150 * time.Millisecond
	runner.pingInterval = 30 * time.Millisecond
	runner.downloadLoad = func(ctx context.Context, onBytes func(int64)) error {
		onBytes(1_500_000)
		<-ctx.Done()
		return nil
	}
	runner.uploadLoad = func(ctx context.Context, onBytes func(int64)) error {
		onBytes(750_000)
		<-ctx.Done()
		return nil
	}

	var phases []string
	res, err := runner.Run(context.Background(), "1.1.1.1", func(p BufferbloatProgress) {
		phases = append(phases, p.Phase)
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if res.Grade != "A+" {
		t.Fatalf("expected grade A+, got %s (%+v)", res.Grade, res)
	}
	if res.DownloadMbps <= 0 || res.UploadMbps <= 0 {
		t.Fatalf("expected positive Mbps metrics, got dl=%v ul=%v", res.DownloadMbps, res.UploadMbps)
	}
	if len(phases) == 0 || phases[len(phases)-1] != "done" {
		t.Fatalf("expected final progress phase 'done', got %v", phases)
	}
	if runner.LastResult() == nil || runner.LastResult().Grade != "A+" {
		t.Fatalf("expected LastResult to be cached")
	}

	// Test cancellation mid-run
	runner.idleDur = 600 * time.Millisecond
	errCh := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(context.Background(), "1.1.1.1", nil)
		errCh <- runErr
	}()
	time.Sleep(60 * time.Millisecond)
	runner.Cancel()
	if cancelErr := <-errCh; !errors.Is(cancelErr, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", cancelErr)
	}
}

func TestDualHopIncidentLogDetection(t *testing.T) {
	pinger := newMockPinger(map[string][]ping.Result{
		"192.168.1.1": {
			{Success: true, RTTMs: 2.0},
			{Success: true, RTTMs: 55.0}, // Router spike >= 40ms
			{Success: true, RTTMs: 2.1},
		},
		"1.1.1.1": {
			{Success: false, Error: "timeout"}, // ISP loss 1
			{Success: false, Error: "timeout"}, // ISP loss 2 -> incident
			{Success: true, RTTMs: 18.0},
		},
	})

	mon := NewDualHopMonitor(pinger, 15*time.Millisecond, nil)
	mon.Start("192.168.1.1", "1.1.1.1")

	deadline := time.Now().Add(450 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := mon.Snapshot()
		if len(snap.Incidents) >= 2 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	mon.Stop()

	snap := mon.Snapshot()
	if len(snap.Incidents) < 2 {
		t.Fatalf("expected at least 2 stability incidents (router spike + ISP loss), got %+v", snap.Incidents)
	}

	resetSnap := mon.Reset()
	if len(resetSnap.Incidents) != 0 {
		t.Fatalf("expected incidents to clear on Reset, got %d", len(resetSnap.Incidents))
	}
}

func TestRegionBenchmarkerWithMockDialer(t *testing.T) {
	bench := NewRegionBenchmarkerWithDialer(func(ctx context.Context, addr string, timeout time.Duration) (float64, error) {
		switch addr {
		case "1.1.1.1:443":
			return 12.0, nil
		case "dynamodb.sa-east-1.amazonaws.com:443":
			return 34.0, nil
		case "dynamodb.eu-central-1.amazonaws.com:443":
			return 190.0, nil
		default:
			return 75.0, nil
		}
	})

	res := bench.Run(context.Background())
	if len(res.Results) != len(DefaultRegionEndpoints()) {
		t.Fatalf("expected %d region results, got %d", len(DefaultRegionEndpoints()), len(res.Results))
	}
	if res.Results[0].ID != "cf-local" || res.Results[0].Grade != "great" {
		t.Fatalf("expected fastest cf-local great grade, got %+v", res.Results[0])
	}
	if res.BestRegion != "Brasil (São Paulo)" {
		t.Fatalf("expected BestRegion 'Brasil (São Paulo)', got %q", res.BestRegion)
	}
}

func TestSummarizeTraceroute(t *testing.T) {
	hops := []TraceHop{
		{TTL: 1, IP: "192.168.1.1", RTTMs: 1.5},
		{TTL: 2, IP: "10.20.0.1", RTTMs: 9.0},
		{TTL: 3, TimedOut: true, IP: "*"},
		{TTL: 4, IP: "200.16.1.10", RTTMs: 78.5},
		{TTL: 5, IP: "1.1.1.1", RTTMs: 81.0, IsTarget: true},
	}

	res := SummarizeTraceroute("1.1.1.1", hops)
	if !res.ReachedTarget {
		t.Fatalf("expected ReachedTarget=true")
	}
	if res.WorstHopTTL != 4 || res.WorstJumpMs != 69.5 {
		t.Fatalf("expected worst jump at TTL 4 (+69.5 ms), got TTL=%d jump=%v", res.WorstHopTTL, res.WorstJumpMs)
	}
}

func TestClassifyNATAndCGNAT(t *testing.T) {
	if !IsCGNATIP("100.64.12.5") || !IsCGNATIP("100.127.255.1") || IsCGNATIP("100.128.0.1") {
		t.Fatalf("IsCGNATIP boundaries failed")
	}
	if !IsPrivateIPv4("192.168.1.1") || !IsPrivateIPv4("10.0.0.1") || IsPrivateIPv4("1.1.1.1") {
		t.Fatalf("IsPrivateIPv4 check failed")
	}

	cgnat := ClassifyNATFromHops([]TraceHop{
		{TTL: 1, IP: "192.168.1.1"},
		{TTL: 2, IP: "100.65.10.1"},
		{TTL: 3, IP: "1.1.1.1"},
	})
	if cgnat.Kind != "cgnat" || cgnat.CGNATHopIP != "100.65.10.1" {
		t.Fatalf("expected cgnat classification, got %+v", cgnat)
	}

	doubleNAT := ClassifyNATFromHops([]TraceHop{
		{TTL: 1, IP: "192.168.0.1"},
		{TTL: 2, IP: "192.168.100.1"},
		{TTL: 3, IP: "200.16.1.1"},
	})
	if doubleNAT.Kind != "double_nat" || doubleNAT.DoubleNATHopIP != "192.168.100.1" {
		t.Fatalf("expected double_nat classification, got %+v", doubleNAT)
	}

	publicNAT := ClassifyNATFromHops([]TraceHop{
		{TTL: 1, IP: "192.168.1.1"},
		{TTL: 2, IP: "200.40.1.1"},
	})
	if publicNAT.Kind != "public" {
		t.Fatalf("expected public classification, got %+v", publicNAT)
	}
}

func TestBuildMTRSummary(t *testing.T) {
	hops := []MTRHopStats{
		{TTL: 1, IP: "192.168.1.1", Sent: 10, Lost: 0, LossPct: 0},
		{TTL: 2, IP: "*", Sent: 10, Lost: 10, LossPct: 100}, // Intermediate rate-limiter should be ignored
		{TTL: 3, IP: "200.16.5.1", Sent: 10, Lost: 2, LossPct: 20},
		{TTL: 4, IP: "1.1.1.1", Sent: 10, Lost: 2, LossPct: 20, IsTarget: true},
	}
	worstTTL, worstLoss, summary := BuildMTRSummary("1.1.1.1", 10, true, hops)
	if worstTTL != 3 || worstLoss != 20.0 || summary == "" {
		t.Fatalf("expected worstTTL=3 worstLoss=20.0, got ttl=%d loss=%v summary=%q", worstTTL, worstLoss, summary)
	}
}
