//go:build windows

package diagnostics

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	defaultMTRInterval = 1500 * time.Millisecond
	defaultMTRMaxHops  = 15
)

// MTRHopStats holds rolling latency, packet loss, and jitter stats for one TTL hop in continuous MTR mode.
type MTRHopStats struct {
	TTL       int     `json:"ttl"`
	IP        string  `json:"ip"`
	Sent      int     `json:"sent"`
	Lost      int     `json:"lost"`
	LossPct   float64 `json:"lossPct"`
	LastMs    float64 `json:"lastMs"`
	BestMs    float64 `json:"bestMs"`
	AvgMs     float64 `json:"avgMs"`
	WorstMs   float64 `json:"worstMs"`
	JitterMs  float64 `json:"jitterMs"`
	IsTarget  bool    `json:"isTarget"`
	IsPrivate bool    `json:"isPrivate"`
}

// MTRSnapshot represents the full rolling state emitted by MTRMonitor.
type MTRSnapshot struct {
	Running       bool          `json:"running"`
	Target        string        `json:"target"`
	StartedAt     string        `json:"startedAt"`
	Cycles        int           `json:"cycles"`
	ReachedTarget bool          `json:"reachedTarget"`
	WorstLossTTL  int           `json:"worstLossTtl"`
	WorstLossPct  float64       `json:"worstLossPct"`
	Hops          []MTRHopStats `json:"hops"`
	Summary       string        `json:"summary"`
}

type mtrHopAccumulator struct {
	ttl         int
	ip          string
	sent        int
	lost        int
	lastMs      float64
	bestMs      float64
	worstMs     float64
	validSum    float64
	validCount  int
	hasPrev     bool
	prevMs      float64
	jitterSum   float64
	jitterCount int
	isTarget    bool
	isPrivate   bool
}

// MTRMonitor continuously probes TTL 1..N (WinMTR style) without Admin rights via Win32 ICMP.
type MTRMonitor struct {
	mu       sync.RWMutex
	interval time.Duration
	onUpdate func(MTRSnapshot)

	running   bool
	target    string
	startedAt string
	maxHops   int
	cycles    int
	reached   bool
	hops      map[int]*mtrHopAccumulator
	lastTTL   int

	cancel context.CancelFunc
	doneCh chan struct{}
}

// NewMTRMonitor creates a continuous MTR monitor.
func NewMTRMonitor(onUpdate func(MTRSnapshot)) *MTRMonitor {
	return &MTRMonitor{
		interval: defaultMTRInterval,
		onUpdate: onUpdate,
		hops:     make(map[int]*mtrHopAccumulator),
	}
}

// SetOnUpdate sets the callback emitted after each MTR cycle completes.
func (m *MTRMonitor) SetOnUpdate(cb func(MTRSnapshot)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate = cb
}

// Start begins continuous MTR probing to target. Any existing session is stopped cleanly.
func (m *MTRMonitor) Start(target string, maxHops int) error {
	m.Stop()

	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		trimmed = "1.1.1.1"
	}
	if maxHops <= 0 || maxHops > traceMaxHops {
		maxHops = defaultMTRMaxHops
	}

	resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 3*time.Second)
	destAddr, err := resolveTraceIPv4(resolveCtx, trimmed)
	resolveCancel()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	m.mu.Lock()
	m.running = true
	m.target = trimmed
	m.startedAt = time.Now().Format("15:04:05")
	m.maxHops = maxHops
	m.cycles = 0
	m.reached = false
	m.lastTTL = 0
	m.hops = make(map[int]*mtrHopAccumulator)
	m.cancel = cancel
	m.doneCh = doneCh
	cb := m.onUpdate
	snap := m.snapshotLocked()
	m.mu.Unlock()

	if cb != nil {
		cb(snap)
	}

	go m.loop(ctx, doneCh, trimmed, destAddr, maxHops)
	return nil
}

// Stop halts the continuous MTR monitor and waits for its goroutine to exit.
func (m *MTRMonitor) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	doneCh := m.doneCh
	wasRunning := m.running
	m.cancel = nil
	m.doneCh = nil
	m.running = false
	cb := m.onUpdate
	snap := m.snapshotLocked()
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if doneCh != nil {
		<-doneCh
	}
	if wasRunning && cb != nil {
		cb(snap)
	}
}

// IsRunning reports whether continuous MTR probing is active.
func (m *MTRMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// Snapshot returns a defensive copy of the current MTR hop table and summary.
func (m *MTRMonitor) Snapshot() MTRSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshotLocked()
}

func (m *MTRMonitor) loop(ctx context.Context, doneCh chan struct{}, target string, destAddr uint32, maxHops int) {
	defer close(doneCh)

	m.runCycle(ctx, destAddr, maxHops)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runCycle(ctx, destAddr, maxHops)
		}
	}
}

func (m *MTRMonitor) runCycle(ctx context.Context, destAddr uint32, maxHops int) {
	if ctx.Err() != nil {
		return
	}

	handle, _, _ := procTraceIcmpCreateFile.Call()
	if handle == uintptr(windows.InvalidHandle) || handle == 0 {
		return
	}
	defer procTraceIcmpCloseHandle.Call(handle)

	type probeSample struct {
		ttl     int
		ip      string
		rttMs   float64
		ok      bool
		reached bool
	}

	samples := make([]probeSample, 0, maxHops)
	consecTimeouts := 0
	reachedInCycle := false

	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			return
		}
		ip, rtt, ok, reached := probeSingleMTRHop(handle, destAddr, ttl)
		samples = append(samples, probeSample{
			ttl:     ttl,
			ip:      ip,
			rttMs:   rtt,
			ok:      ok,
			reached: reached,
		})
		if reached {
			reachedInCycle = true
			break
		}
		if !ok {
			consecTimeouts++
			if consecTimeouts >= 4 {
				break
			}
		} else {
			consecTimeouts = 0
		}
	}

	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.cycles++
	if reachedInCycle {
		m.reached = true
	}

	for _, s := range samples {
		acc, ok := m.hops[s.ttl]
		if !ok {
			acc = &mtrHopAccumulator{ttl: s.ttl, ip: "*"}
			m.hops[s.ttl] = acc
		}
		if s.ttl > m.lastTTL {
			m.lastTTL = s.ttl
		}
		acc.sent++
		if !s.ok {
			acc.lost++
			continue
		}
		if s.ip != "" {
			acc.ip = s.ip
			acc.isPrivate = IsPrivateIPv4(s.ip) || IsCGNATIP(s.ip)
		}
		if s.reached {
			acc.isTarget = true
		}
		acc.lastMs = s.rttMs
		acc.validSum += s.rttMs
		acc.validCount++
		if acc.bestMs == 0 || s.rttMs < acc.bestMs {
			acc.bestMs = s.rttMs
		}
		if s.rttMs > acc.worstMs {
			acc.worstMs = s.rttMs
		}
		if acc.hasPrev {
			acc.jitterSum += math.Abs(s.rttMs - acc.prevMs)
			acc.jitterCount++
		}
		acc.prevMs = s.rttMs
		acc.hasPrev = true
	}

	snap := m.snapshotLocked()
	cb := m.onUpdate
	m.mu.Unlock()

	if cb != nil {
		cb(snap)
	}
}

func probeSingleMTRHop(handle uintptr, destAddr uint32, ttl int) (string, float64, bool, bool) {
	replySize := int(unsafe.Sizeof(traceICMPEchoReply{})) + len(tracePayload) + 64
	replyBuf := make([]byte, replySize)
	opts := traceIPOptionInformation{TTL: uint8(ttl)}

	start := time.Now()
	ret, _, _ := procTraceIcmpSendEcho.Call(
		handle,
		uintptr(destAddr),
		uintptr(unsafe.Pointer(&tracePayload[0])),
		uintptr(len(tracePayload)),
		uintptr(unsafe.Pointer(&opts)),
		uintptr(unsafe.Pointer(&replyBuf[0])),
		uintptr(replySize),
		uintptr(650),
	)
	elapsed := time.Since(start)

	reply := (*traceICMPEchoReply)(unsafe.Pointer(&replyBuf[0]))
	if ret == 0 || (reply.Status != traceIPSuccess && reply.Status != traceIPTTLExpired) {
		return "", 0, false, false
	}

	hopIP := net.IPv4(
		byte(reply.Address),
		byte(reply.Address>>8),
		byte(reply.Address>>16),
		byte(reply.Address>>24),
	).String()

	rtt := round1(float64(elapsed.Microseconds()) / 1000.0)
	if rtt < 0.1 {
		rtt = 0.1
	}
	if reply.RoundTripTime > 0 && float64(reply.RoundTripTime) > rtt+2 {
		rtt = float64(reply.RoundTripTime)
	}
	return hopIP, rtt, true, reply.Status == traceIPSuccess
}

func (m *MTRMonitor) snapshotLocked() MTRSnapshot {
	hops := make([]MTRHopStats, 0, m.lastTTL)
	for ttl := 1; ttl <= m.lastTTL; ttl++ {
		acc, ok := m.hops[ttl]
		if !ok {
			continue
		}
		lossPct := 0.0
		if acc.sent > 0 {
			lossPct = round1(float64(acc.lost) * 100.0 / float64(acc.sent))
		}
		avgMs := 0.0
		if acc.validCount > 0 {
			avgMs = round1(acc.validSum / float64(acc.validCount))
		}
		jitterMs := 0.0
		if acc.jitterCount > 0 {
			jitterMs = round1(acc.jitterSum / float64(acc.jitterCount))
		}
		hops = append(hops, MTRHopStats{
			TTL:       acc.ttl,
			IP:        acc.ip,
			Sent:      acc.sent,
			Lost:      acc.lost,
			LossPct:   lossPct,
			LastMs:    round1(acc.lastMs),
			BestMs:    round1(acc.bestMs),
			AvgMs:     avgMs,
			WorstMs:   round1(acc.worstMs),
			JitterMs:  jitterMs,
			IsTarget:  acc.isTarget,
			IsPrivate: acc.isPrivate,
		})
	}

	worstTTL, worstLoss, summary := BuildMTRSummary(m.target, m.cycles, m.reached, hops)
	return MTRSnapshot{
		Running:       m.running,
		Target:        m.target,
		StartedAt:     m.startedAt,
		Cycles:        m.cycles,
		ReachedTarget: m.reached,
		WorstLossTTL:  worstTTL,
		WorstLossPct:  worstLoss,
		Hops:          hops,
		Summary:       summary,
	}
}

// BuildMTRSummary analyzes continuous MTR hop stats to pinpoint real packet-loss hops vs rate-limited intermediate routers.
func BuildMTRSummary(target string, cycles int, reached bool, hops []MTRHopStats) (worstTTL int, worstLoss float64, summary string) {
	if cycles == 0 || len(hops) == 0 {
		return 0, 0, fmt.Sprintf("Iniciando trazado continuo MTR hacia %s...", target)
	}

	targetLoss := 0.0
	if reached {
		for _, h := range hops {
			if h.IsTarget {
				targetLoss = h.LossPct
				break
			}
		}
	}

	for i, h := range hops {
		// Intermediate nodes showing 100% timeout while later hops still reply are simply ICMP-silent routers.
		hasDownstream := false
		for j := i + 1; j < len(hops); j++ {
			if hops[j].LossPct < 100 {
				hasDownstream = true
				break
			}
		}
		if !h.IsTarget && h.IP == "*" && h.LossPct == 100 && hasDownstream {
			continue
		}
		if h.LossPct > worstLoss {
			worstLoss = h.LossPct
			worstTTL = h.TTL
		}
	}

	switch {
	case worstTTL > 0 && worstLoss >= 5:
		summary = fmt.Sprintf("Pérdida real de %.1f%% observada en Salto %d tras %d ciclos.", worstLoss, worstTTL, cycles)
	case reached:
		summary = fmt.Sprintf("Ruta estable hacia %s (%d ciclos · pérdida final %.1f%%).", target, cycles, targetLoss)
	default:
		summary = fmt.Sprintf("MTR activo (%d ciclos): algunos saltos no responden ICMP.", cycles)
	}
	return worstTTL, round1(worstLoss), summary
}
