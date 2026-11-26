package diagnostics

import (
	"fmt"
	"math"
	"sort"
)

// DualHopSample represents one simultaneous probe of Hop 1 (router) and Hop 2 (internet).
type DualHopSample struct {
	Timestamp  int64   `json:"timestamp"`
	Seq        int     `json:"seq"`
	RouterMs   float64 `json:"routerMs"`
	RouterOK   bool    `json:"routerOk"`
	InternetMs float64 `json:"internetMs"`
	InternetOK bool    `json:"internetOk"`
}

// HopStats holds aggregated latency, jitter, and packet-loss metrics for a single hop.
type HopStats struct {
	Target    string  `json:"target"`
	CurrentMs float64 `json:"currentMs"`
	AvgMs     float64 `json:"avgMs"`
	MinMs     float64 `json:"minMs"`
	MaxMs     float64 `json:"maxMs"`
	P95Ms     float64 `json:"p95Ms"`
	JitterMs  float64 `json:"jitterMs"`
	LossPct   float64 `json:"lossPct"`
	Sent      int     `json:"sent"`
	Lost      int     `json:"lost"`
	LastOK    bool    `json:"lastOk"`
}

// Verdict attributes observed network degradation to Wi-Fi/LAN, ISP, both, or optimal health.
type Verdict struct {
	Code     string `json:"code"`     // "warming_up" | "offline" | "optimal" | "wifi_issue" | "isp_issue" | "both_issue"
	Severity string `json:"severity"` // "emerald" | "cyan" | "amber" | "rose"
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Score    int    `json:"score"`
}

// StabilityIncident records a detected packet-loss burst or severe latency spike.
type StabilityIncident struct {
	ID         int     `json:"id"`
	Timestamp  int64   `json:"timestamp"`
	TimeLabel  string  `json:"timeLabel"`
	Hop        string  `json:"hop"`      // "router" | "internet"
	Kind       string  `json:"kind"`     // "loss" | "spike"
	Severity   string  `json:"severity"` // "amber" | "rose"
	DurationMs int64   `json:"durationMs"`
	PeakMs     float64 `json:"peakMs"`
	Message    string  `json:"message"`
}

// DualHopSnapshot is the complete UI-ready state emitted by DualHopMonitor.
type DualHopSnapshot struct {
	Running        bool                `json:"running"`
	RouterTarget   string              `json:"routerTarget"`
	InternetTarget string              `json:"internetTarget"`
	Router         HopStats            `json:"router"`
	Internet       HopStats            `json:"internet"`
	Verdict        Verdict             `json:"verdict"`
	Samples        []DualHopSample     `json:"samples"`
	Incidents      []StabilityIncident `json:"incidents"`
}

func round1(v float64) float64 {
	return math.Round(v*10.0) / 10.0
}

// ComputeHopStats calculates latency statistics from parallel value/success slices.
func ComputeHopStats(target string, values []float64, oks []bool) HopStats {
	n := len(values)
	if len(oks) < n {
		n = len(oks)
	}
	stats := HopStats{
		Target: target,
		Sent:   n,
	}
	if n == 0 {
		return stats
	}

	valid := make([]float64, 0, n)
	var sum float64
	var jitterSum float64
	var jitterCount int
	var prevValid float64
	hasPrev := false

	for i := 0; i < n; i++ {
		ok := oks[i]
		val := values[i]
		if !ok || val < 0 {
			stats.Lost++
			continue
		}
		valid = append(valid, val)
		sum += val
		if hasPrev {
			jitterSum += math.Abs(val - prevValid)
			jitterCount++
		}
		prevValid = val
		hasPrev = true
	}

	stats.LastOK = oks[n-1]
	if stats.LastOK && values[n-1] >= 0 {
		stats.CurrentMs = round1(values[n-1])
	}
	stats.LossPct = round1(float64(stats.Lost) * 100.0 / float64(n))

	if len(valid) == 0 {
		return stats
	}

	sorted := make([]float64, len(valid))
	copy(sorted, valid)
	sort.Float64s(sorted)

	stats.MinMs = round1(sorted[0])
	stats.MaxMs = round1(sorted[len(sorted)-1])
	stats.AvgMs = round1(sum / float64(len(valid)))

	p95Idx := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	if p95Idx < 0 {
		p95Idx = 0
	}
	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}
	stats.P95Ms = round1(sorted[p95Idx])

	if jitterCount > 0 {
		stats.JitterMs = round1(jitterSum / float64(jitterCount))
	}

	return stats
}

// EvaluateVerdict classifies whether lag originates in local Wi-Fi/LAN or the ISP.
func EvaluateVerdict(router, internet HopStats) Verdict {
	if router.Sent < 3 && internet.Sent < 3 {
		return Verdict{
			Code:     "warming_up",
			Severity: "cyan",
			Title:    "Calibrando Dual-Hop...",
			Detail:   "Recolectando las primeras muestras simultáneas al router e internet.",
			Score:    100,
		}
	}

	if router.Sent > 0 && router.LossPct >= 50 {
		return Verdict{
			Code:     "offline",
			Severity: "rose",
			Title:    "Enlace local interrumpido",
			Detail:   fmt.Sprintf("Tu PC pierde %.1f%% de paquetes antes de llegar al router local.", router.LossPct),
			Score:    0,
		}
	}

	routerBad := router.Sent >= 3 && (router.LossPct >= 3 || router.JitterMs >= 8 || router.P95Ms >= 20 || router.AvgMs >= 15)

	ispDelta := math.Max(0, internet.AvgMs-router.AvgMs)
	ispLossExcess := math.Max(0, internet.LossPct-router.LossPct)
	ispJitterExcess := math.Max(0, internet.JitterMs-router.JitterMs)
	ispP95Delta := math.Max(0, internet.P95Ms-router.P95Ms)

	ispBad := internet.Sent >= 3 && ((!routerBad && internet.LossPct >= 3) ||
		ispLossExcess >= 3 ||
		ispJitterExcess >= 15 ||
		ispDelta >= 90 ||
		ispP95Delta >= 110)

	score := computeHealthScore(router, internet, ispDelta, ispLossExcess, ispJitterExcess)

	switch {
	case routerBad && ispBad:
		return Verdict{
			Code:     "both_issue",
			Severity: "rose",
			Title:    "Inestabilidad en Red Local y Proveedor",
			Detail: fmt.Sprintf(
				"Tu enlace al router tiene %.1f ms (jitter %.1f ms, %.1f%% pérdida) y tu ISP suma %.1f ms extra con %.1f%% pérdida adicional.",
				router.AvgMs, router.JitterMs, router.LossPct, ispDelta, ispLossExcess,
			),
			Score: score,
		}
	case routerBad:
		return Verdict{
			Code:     "wifi_issue",
			Severity: "amber",
			Title:    "El lag está en tu Wi-Fi / Red Local",
			Detail: fmt.Sprintf(
				"Tu router responde en %.1f ms promedio (P95 %.1f ms, jitter %.1f ms, %.1f%% pérdida). Revisá canal Wi-Fi o probá cable Ethernet.",
				router.AvgMs, router.P95Ms, router.JitterMs, router.LossPct,
			),
			Score: score,
		}
	case ispBad:
		return Verdict{
			Code:     "isp_issue",
			Severity: "amber",
			Title:    "Tu Wi-Fi está bien; el problema es tu Proveedor (ISP)",
			Detail: fmt.Sprintf(
				"Tu router responde estable en %.1f ms (%.1f%% pérdida), pero el salto externo sube %.1f ms (jitter extra %.1f ms, pérdida ISP %.1f%%).",
				router.AvgMs, router.LossPct, ispDelta, ispJitterExcess, ispLossExcess,
			),
			Score: score,
		}
	default:
		severity := "emerald"
		if score < 85 {
			severity = "cyan"
		}
		return Verdict{
			Code:     "optimal",
			Severity: severity,
			Title:    "Red Estable y Óptima",
			Detail: fmt.Sprintf(
				"Enlace local limpio (%.1f ms, jitter %.1f ms) y salida a internet estable (%.1f ms promedio, %.1f%% pérdida).",
				router.AvgMs, router.JitterMs, internet.AvgMs, internet.LossPct,
			),
			Score: score,
		}
	}
}

func computeHealthScore(router, internet HopStats, ispDelta, ispLossExcess, ispJitterExcess float64) int {
	penalty := 0.0

	// Local hop penalties
	penalty += router.LossPct * 4.0
	if router.JitterMs > 3 {
		penalty += (router.JitterMs - 3) * 1.6
	}
	if router.AvgMs > 8 {
		penalty += (router.AvgMs - 8) * 0.8
	}

	// ISP hop penalties
	penalty += ispLossExcess * 3.5
	if ispJitterExcess > 6 {
		penalty += (ispJitterExcess - 6) * 1.1
	}
	if ispDelta > 45 {
		penalty += (ispDelta - 45) * 0.25
	}
	if internet.LossPct >= 50 && router.LossPct < 5 {
		penalty += 45
	}

	score := int(math.Round(100.0 - penalty))
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
