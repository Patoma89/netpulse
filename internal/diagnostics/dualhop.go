package diagnostics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"netpulse/internal/ping"
)

const (
	defaultDualHopInterval = 500 * time.Millisecond
	defaultDualHopTimeout  = 900 * time.Millisecond
	defaultMaxSamples      = 120
	maxIncidents           = 50
	spikeCooldownMs        = 5000
)

// DualHopMonitor continuously probes the local router and an internet target concurrently.
type DualHopMonitor struct {
	mu                sync.RWMutex
	pinger            ping.Pinger
	interval          time.Duration
	maxSamples        int
	routerTarget      string
	internetTarget    string
	samples           []DualHopSample
	incidents         []StabilityIncident
	seq               int
	nextIncidentID    int
	routerLossRun     int
	routerLossStart   int64
	internetLossRun   int
	internetLossStart int64
	lastRouterSpike   int64
	lastInetSpike     int64
	running           bool
	cancel            context.CancelFunc
	doneCh            chan struct{}
	onUpdate          func(DualHopSnapshot)
	onIncident        func(StabilityIncident)
}

// NewDualHopMonitor builds a DualHopMonitor using the provided Pinger and sample rate.
func NewDualHopMonitor(pinger ping.Pinger, interval time.Duration, onUpdate func(DualHopSnapshot)) *DualHopMonitor {
	if interval <= 0 {
		interval = defaultDualHopInterval
	}
	return &DualHopMonitor{
		pinger:     pinger,
		interval:   interval,
		maxSamples: defaultMaxSamples,
		samples:    make([]DualHopSample, 0, defaultMaxSamples),
		incidents:  make([]StabilityIncident, 0, 16),
		onUpdate:   onUpdate,
	}
}

// SetOnUpdate sets or replaces the snapshot update callback.
func (m *DualHopMonitor) SetOnUpdate(cb func(DualHopSnapshot)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate = cb
}

// SetOnIncident sets a callback invoked whenever a new stability incident is detected.
func (m *DualHopMonitor) SetOnIncident(cb func(StabilityIncident)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onIncident = cb
}

// Start begins concurrent Dual-Hop probing, stopping any prior active loop cleanly.
func (m *DualHopMonitor) Start(routerIP, internetIP string) {
	m.Stop()

	m.mu.Lock()
	if m.routerTarget != routerIP || m.internetTarget != internetIP {
		m.samples = m.samples[:0]
		m.incidents = m.incidents[:0]
		m.seq = 0
		m.resetIncidentTrackersLocked()
	}
	m.routerTarget = routerIP
	m.internetTarget = internetIP
	m.running = true

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})
	m.cancel = cancel
	m.doneCh = doneCh
	m.mu.Unlock()

	go m.loop(ctx, doneCh, routerIP, internetIP)
}

// Stop halts the probing loop and waits for its goroutine to exit.
func (m *DualHopMonitor) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	doneCh := m.doneCh
	m.cancel = nil
	m.doneCh = nil
	m.running = false
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if doneCh != nil {
		<-doneCh
	}
}

// Reset clears accumulated samples and incidents while keeping current targets and running state.
func (m *DualHopMonitor) Reset() DualHopSnapshot {
	m.mu.Lock()
	m.samples = m.samples[:0]
	m.incidents = m.incidents[:0]
	m.seq = 0
	m.resetIncidentTrackersLocked()
	snap := m.snapshotLocked()
	cb := m.onUpdate
	m.mu.Unlock()

	if cb != nil {
		cb(snap)
	}
	return snap
}

func (m *DualHopMonitor) resetIncidentTrackersLocked() {
	m.routerLossRun = 0
	m.routerLossStart = 0
	m.internetLossRun = 0
	m.internetLossStart = 0
	m.lastRouterSpike = 0
	m.lastInetSpike = 0
}

// IsRunning reports whether the monitor loop is active.
func (m *DualHopMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// Snapshot returns a thread-safe copy of the current Dual-Hop state, stats, incidents, and verdict.
func (m *DualHopMonitor) Snapshot() DualHopSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshotLocked()
}

func (m *DualHopMonitor) loop(ctx context.Context, doneCh chan struct{}, routerIP, internetIP string) {
	defer close(doneCh)

	m.probeOnce(ctx, routerIP, internetIP)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.probeOnce(ctx, routerIP, internetIP)
		}
	}
}

func (m *DualHopMonitor) probeOnce(ctx context.Context, routerIP, internetIP string) {
	if ctx.Err() != nil {
		return
	}

	probeCtx, cancel := context.WithTimeout(ctx, defaultDualHopTimeout)
	defer cancel()

	var (
		wg          sync.WaitGroup
		routerRes   ping.Result
		internetRes ping.Result
	)

	if routerIP != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routerRes = m.pinger.Ping(probeCtx, routerIP, defaultDualHopTimeout)
		}()
	}

	if internetIP != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			internetRes = m.pinger.Ping(probeCtx, internetIP, defaultDualHopTimeout)
		}()
	}

	wg.Wait()

	if ctx.Err() != nil {
		return
	}

	routerMs := -1.0
	if routerRes.Success {
		routerMs = routerRes.RTTMs
	}
	internetMs := -1.0
	if internetRes.Success {
		internetMs = internetRes.RTTMs
	}

	now := time.Now()
	nowMs := now.UnixMilli()

	m.mu.Lock()
	m.seq++
	sample := DualHopSample{
		Timestamp:  nowMs,
		Seq:        m.seq,
		RouterMs:   routerMs,
		RouterOK:   routerRes.Success,
		InternetMs: internetMs,
		InternetOK: internetRes.Success,
	}
	m.samples = append(m.samples, sample)
	if len(m.samples) > m.maxSamples {
		overflow := len(m.samples) - m.maxSamples
		copy(m.samples, m.samples[overflow:])
		m.samples = m.samples[:m.maxSamples]
	}

	newIncidents := m.trackIncidentsLocked(now, nowMs, routerIP != "", internetIP != "", routerRes, internetRes)

	snap := m.snapshotLocked()
	cb := m.onUpdate
	incCb := m.onIncident
	m.mu.Unlock()

	if incCb != nil {
		for _, inc := range newIncidents {
			incCb(inc)
		}
	}
	if cb != nil {
		cb(snap)
	}
}

func (m *DualHopMonitor) trackIncidentsLocked(
	now time.Time,
	nowMs int64,
	hasRouter bool,
	hasInternet bool,
	routerRes ping.Result,
	internetRes ping.Result,
) []StabilityIncident {
	var created []StabilityIncident
	timeLabel := now.Format("15:04:05")
	stepMs := m.interval.Milliseconds()
	if stepMs <= 0 {
		stepMs = 500
	}

	// 1. Router consecutive loss & spikes
	if hasRouter {
		if !routerRes.Success {
			m.routerLossRun++
			if m.routerLossRun == 1 {
				m.routerLossStart = nowMs
			} else if m.routerLossRun == 2 {
				dur := stepMs * 2
				inc := m.prependIncidentLocked(StabilityIncident{
					Timestamp:  nowMs,
					TimeLabel:  timeLabel,
					Hop:        "router",
					Kind:       "loss",
					Severity:   "rose",
					DurationMs: dur,
					Message:    fmt.Sprintf("Microcorte en Wi-Fi / Red Local (%.1fs sin respuesta al router)", float64(dur)/1000.0),
				})
				created = append(created, inc)
			} else if len(m.incidents) > 0 && m.incidents[0].Hop == "router" && m.incidents[0].Kind == "loss" {
				dur := (nowMs - m.routerLossStart) + stepMs
				if dur < stepMs*2 {
					dur = stepMs * int64(m.routerLossRun)
				}
				m.incidents[0].DurationMs = dur
				m.incidents[0].Message = fmt.Sprintf("Microcorte en Wi-Fi / Red Local (%.1fs sin respuesta al router)", float64(dur)/1000.0)
			}
		} else {
			m.routerLossRun = 0
			if routerRes.RTTMs >= 40.0 && nowMs-m.lastRouterSpike >= spikeCooldownMs {
				m.lastRouterSpike = nowMs
				inc := m.prependIncidentLocked(StabilityIncident{
					Timestamp: nowMs,
					TimeLabel: timeLabel,
					Hop:       "router",
					Kind:      "spike",
					Severity:  "amber",
					PeakMs:    routerRes.RTTMs,
					Message:   fmt.Sprintf("Pico de latencia local al router (%.1f ms)", routerRes.RTTMs),
				})
				created = append(created, inc)
			}
		}
	}

	// 2. Internet/ISP consecutive loss & spikes (only attribute to ISP when router hop is healthy)
	if hasInternet && (!hasRouter || routerRes.Success) {
		if !internetRes.Success {
			m.internetLossRun++
			if m.internetLossRun == 1 {
				m.internetLossStart = nowMs
			} else if m.internetLossRun == 2 {
				dur := stepMs * 2
				inc := m.prependIncidentLocked(StabilityIncident{
					Timestamp:  nowMs,
					TimeLabel:  timeLabel,
					Hop:        "internet",
					Kind:       "loss",
					Severity:   "rose",
					DurationMs: dur,
					Message:    fmt.Sprintf("Microcorte de salida ISP (%.1fs sin respuesta externa)", float64(dur)/1000.0),
				})
				created = append(created, inc)
			} else if len(m.incidents) > 0 && m.incidents[0].Hop == "internet" && m.incidents[0].Kind == "loss" {
				dur := (nowMs - m.internetLossStart) + stepMs
				if dur < stepMs*2 {
					dur = stepMs * int64(m.internetLossRun)
				}
				m.incidents[0].DurationMs = dur
				m.incidents[0].Message = fmt.Sprintf("Microcorte de salida ISP (%.1fs sin respuesta externa)", float64(dur)/1000.0)
			}
		} else {
			m.internetLossRun = 0
			delta := internetRes.RTTMs - routerRes.RTTMs
			if !hasRouter {
				delta = internetRes.RTTMs
			}
			if delta >= 120.0 && nowMs-m.lastInetSpike >= spikeCooldownMs {
				m.lastInetSpike = nowMs
				inc := m.prependIncidentLocked(StabilityIncident{
					Timestamp: nowMs,
					TimeLabel: timeLabel,
					Hop:       "internet",
					Kind:      "spike",
					Severity:  "amber",
					PeakMs:    internetRes.RTTMs,
					Message:   fmt.Sprintf("Pico de latencia en Proveedor / ISP (%.1f ms)", internetRes.RTTMs),
				})
				created = append(created, inc)
			}
		}
	} else if hasInternet && !routerRes.Success {
		m.internetLossRun = 0
	}
	return created
}

func (m *DualHopMonitor) prependIncidentLocked(inc StabilityIncident) StabilityIncident {
	m.nextIncidentID++
	inc.ID = m.nextIncidentID
	m.incidents = append([]StabilityIncident{inc}, m.incidents...)
	if len(m.incidents) > maxIncidents {
		m.incidents = m.incidents[:maxIncidents]
	}
	return inc
}

func (m *DualHopMonitor) snapshotLocked() DualHopSnapshot {
	n := len(m.samples)
	samplesCopy := make([]DualHopSample, n)
	copy(samplesCopy, m.samples)

	incCopy := make([]StabilityIncident, len(m.incidents))
	copy(incCopy, m.incidents)

	rVals := make([]float64, n)
	rOKs := make([]bool, n)
	iVals := make([]float64, n)
	iOKs := make([]bool, n)

	for i, s := range samplesCopy {
		rVals[i] = s.RouterMs
		rOKs[i] = s.RouterOK
		iVals[i] = s.InternetMs
		iOKs[i] = s.InternetOK
	}

	routerStats := ComputeHopStats(m.routerTarget, rVals, rOKs)
	internetStats := ComputeHopStats(m.internetTarget, iVals, iOKs)
	verdict := EvaluateVerdict(routerStats, internetStats)

	return DualHopSnapshot{
		Running:        m.running,
		RouterTarget:   m.routerTarget,
		InternetTarget: m.internetTarget,
		Router:         routerStats,
		Internet:       internetStats,
		Verdict:        verdict,
		Samples:        samplesCopy,
		Incidents:      incCopy,
	}
}
