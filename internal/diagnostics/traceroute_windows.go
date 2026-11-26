//go:build windows

package diagnostics

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"netpulse/internal/ping"
)

var (
	modTraceIphlpapi         = windows.NewLazySystemDLL("iphlpapi.dll")
	procTraceIcmpCreateFile  = modTraceIphlpapi.NewProc("IcmpCreateFile")
	procTraceIcmpSendEcho    = modTraceIphlpapi.NewProc("IcmpSendEcho")
	procTraceIcmpCloseHandle = modTraceIphlpapi.NewProc("IcmpCloseHandle")
)

const (
	traceIPSuccess    uint32 = 0
	traceIPTTLExpired uint32 = 11013
	traceMaxHops             = 18
)

type traceIPOptionInformation struct {
	TTL         uint8
	Tos         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

type traceICMPEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       traceIPOptionInformation
}

// TraceHop describes one IP router hop along the path to an external target.
type TraceHop struct {
	TTL      int     `json:"ttl"`
	IP       string  `json:"ip"`
	RTTMs    float64 `json:"rttMs"`
	TimedOut bool    `json:"timedOut"`
	IsTarget bool    `json:"isTarget"`
	DeltaMs  float64 `json:"deltaMs"`
}

// TracerouteResult summarizes hop-by-hop latency and the largest latency jump.
type TracerouteResult struct {
	Target        string     `json:"target"`
	CompletedAt   string     `json:"completedAt"`
	ReachedTarget bool       `json:"reachedTarget"`
	WorstHopTTL   int        `json:"worstHopTtl"`
	WorstJumpMs   float64    `json:"worstJumpMs"`
	Hops          []TraceHop `json:"hops"`
	Summary       string     `json:"summary"`
}

var tracePayload = []byte("NetPulseTraceRoutePayload0123456")

// RunTraceroute performs a lightweight Win32 ICMP TTL-incrementing traceroute without Admin rights.
func RunTraceroute(ctx context.Context, target string) TracerouteResult {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		trimmed = "1.1.1.1"
	}

	destAddr, err := resolveTraceIPv4(ctx, trimmed)
	if err != nil {
		return TracerouteResult{
			Target:      trimmed,
			CompletedAt: time.Now().Format(time.RFC3339),
			Hops:        []TraceHop{},
			Summary:     fmt.Sprintf("No se pudo resolver el destino %s: %v", trimmed, err),
		}
	}

	handle, _, callErr := procTraceIcmpCreateFile.Call()
	if handle == uintptr(windows.InvalidHandle) || handle == 0 {
		return TracerouteResult{
			Target:      trimmed,
			CompletedAt: time.Now().Format(time.RFC3339),
			Hops:        []TraceHop{},
			Summary:     fmt.Sprintf("No se pudo abrir ICMP handle: %v", callErr),
		}
	}
	defer procTraceIcmpCloseHandle.Call(handle)

	hops := make([]TraceHop, 0, traceMaxHops)
	consecutiveTimeouts := 0

	for ttl := 1; ttl <= traceMaxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}

		hop := probeTraceHop(handle, destAddr, ttl)
		hops = append(hops, hop)

		if hop.IsTarget {
			break
		}
		if hop.TimedOut {
			consecutiveTimeouts++
			if consecutiveTimeouts >= 4 {
				break
			}
		} else {
			consecutiveTimeouts = 0
		}
	}

	return SummarizeTraceroute(trimmed, hops)
}

func probeTraceHop(handle uintptr, destAddr uint32, ttl int) TraceHop {
	replySize := int(unsafe.Sizeof(traceICMPEchoReply{})) + len(tracePayload) + 64
	replyBuf := make([]byte, replySize)
	opts := traceIPOptionInformation{TTL: uint8(ttl)}

	var bestIP string
	var rtts []float64
	reached := false

	for attempt := 0; attempt < 2; attempt++ {
		start := time.Now()
		ret, _, _ := procTraceIcmpSendEcho.Call(
			handle,
			uintptr(destAddr),
			uintptr(unsafe.Pointer(&tracePayload[0])),
			uintptr(len(tracePayload)),
			uintptr(unsafe.Pointer(&opts)),
			uintptr(unsafe.Pointer(&replyBuf[0])),
			uintptr(replySize),
			uintptr(750),
		)
		elapsed := time.Since(start)

		reply := (*traceICMPEchoReply)(unsafe.Pointer(&replyBuf[0]))
		if ret > 0 && (reply.Status == traceIPSuccess || reply.Status == traceIPTTLExpired) {
			hopIP := net.IPv4(
				byte(reply.Address),
				byte(reply.Address>>8),
				byte(reply.Address>>16),
				byte(reply.Address>>24),
			).String()
			if bestIP == "" {
				bestIP = hopIP
			}
			rtt := math.Round(float64(elapsed.Microseconds())/100.0) / 10.0
			if rtt < 0.1 {
				rtt = 0.1
			}
			if reply.RoundTripTime > 0 && float64(reply.RoundTripTime) > rtt+2 {
				rtt = float64(reply.RoundTripTime)
			}
			rtts = append(rtts, rtt)
			if reply.Status == traceIPSuccess {
				reached = true
			}
		}
	}

	if len(rtts) == 0 {
		return TraceHop{TTL: ttl, IP: "*", TimedOut: true}
	}

	var sum float64
	for _, v := range rtts {
		sum += v
	}
	avgRTT := round1(sum / float64(len(rtts)))

	return TraceHop{
		TTL:      ttl,
		IP:       bestIP,
		RTTMs:    avgRTT,
		TimedOut: false,
		IsTarget: reached,
	}
}

// SummarizeTraceroute computes per-hop latency jumps and highlights the worst bottleneck hop.
func SummarizeTraceroute(target string, hops []TraceHop) TracerouteResult {
	out := TracerouteResult{
		Target:      target,
		CompletedAt: time.Now().Format(time.RFC3339),
		Hops:        make([]TraceHop, len(hops)),
	}
	copy(out.Hops, hops)

	var prevRTT float64
	hasPrev := false
	worstTTL := 0
	worstJump := 0.0

	for i := range out.Hops {
		h := &out.Hops[i]
		if h.IsTarget {
			out.ReachedTarget = true
		}
		if h.TimedOut {
			continue
		}
		if hasPrev {
			jump := round1(math.Max(0, h.RTTMs-prevRTT))
			h.DeltaMs = jump
			if jump > worstJump {
				worstJump = jump
				worstTTL = h.TTL
			}
		} else {
			h.DeltaMs = h.RTTMs
		}
		prevRTT = h.RTTMs
		hasPrev = true
	}

	out.WorstHopTTL = worstTTL
	out.WorstJumpMs = worstJump

	switch {
	case len(out.Hops) == 0:
		out.Summary = "No se obtuvieron saltos de ruta."
	case worstTTL > 0 && worstJump >= 25:
		out.Summary = fmt.Sprintf("Mayor salto de latencia en el Salto %d (+%.1f ms).", worstTTL, worstJump)
	case out.ReachedTarget:
		out.Summary = fmt.Sprintf("Ruta completa hasta %s en %d saltos sin saltos anómalos.", target, len(out.Hops))
	default:
		out.Summary = fmt.Sprintf("Trazado parcial (%d saltos): algunos routers intermedios bloquean ICMP.", len(out.Hops))
	}

	return out
}

func resolveTraceIPv4(ctx context.Context, target string) (uint32, error) {
	if ip := net.ParseIP(target); ip != nil {
		addr, ok := ping.ParseIPv4ToUint32(ip)
		if !ok {
			return 0, fmt.Errorf("%q no es IPv4", target)
		}
		return addr, nil
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", target)
	if err != nil {
		return 0, err
	}
	for _, ip := range ips {
		if addr, ok := ping.ParseIPv4ToUint32(ip); ok {
			return addr, nil
		}
	}
	return 0, fmt.Errorf("sin dirección IPv4 para %q", target)
}
