//go:build windows

package diagnostics

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ipFlagDF          uint8  = 0x02
	ipPacketTooBig    uint32 = 11009
	ipv4ICMPOverhead         = 28
	standardEthernetM        = 1500
)

// NATStatus summarizes whether the internet uplink sits behind CGNAT, Double NAT, or a clean single NAT.
type NATStatus struct {
	Kind           string     `json:"kind"` // "public" | "double_nat" | "cgnat" | "unknown"
	Label          string     `json:"label"`
	Detail         string     `json:"detail"`
	CGNATHopIP     string     `json:"cgnatHopIp"`
	DoubleNATHopIP string     `json:"doubleNatHopIp"`
	FirstHops      []TraceHop `json:"firstHops"`
}

// MTUStatus holds Path MTU Discovery (DF bit) results for IPv4 internet traffic.
type MTUStatus struct {
	PathMTU     int    `json:"pathMtu"`
	StandardMTU int    `json:"standardMtu"`
	Fragmented  bool   `json:"fragmented"`
	Target      string `json:"target"`
	Detail      string `json:"detail"`
}

// IPv6Status checks whether the machine has a global IPv6 address and real end-to-end IPv6 connectivity.
type IPv6Status struct {
	HasLocalIPv6  bool    `json:"hasLocalIpv6"`
	HasGlobalIPv6 bool    `json:"hasGlobalIpv6"`
	Reachable     bool    `json:"reachable"`
	RTTMs         float64 `json:"rttMs"`
	Detail        string  `json:"detail"`
}

// NetworkAuditResult bundles NAT/CGNAT, Path MTU, and IPv6 checks for gaming & ISP diagnostics.
type NetworkAuditResult struct {
	CompletedAt string     `json:"completedAt"`
	NAT         NATStatus  `json:"nat"`
	MTU         MTUStatus  `json:"mtu"`
	IPv6        IPv6Status `json:"ipv6"`
	OverallNote string     `json:"overallNote"`
}

var cgnatCIDR = func() *net.IPNet {
	_, n, _ := net.ParseCIDR("100.64.0.0/10")
	return n
}()

// IsCGNATIP reports whether an IPv4 address belongs to RFC 6598 Shared Address Space (100.64.0.0/10).
func IsCGNATIP(ipStr string) bool {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil || cgnatCIDR == nil {
		return false
	}
	return cgnatCIDR.Contains(ip)
}

// IsPrivateIPv4 reports whether an IPv4 address belongs to RFC 1918 private space.
func IsPrivateIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return false
	}
	return ip.IsPrivate()
}

// ClassifyNATFromHops infers CGNAT, Double NAT, or single public NAT from early traceroute hops.
func ClassifyNATFromHops(hops []TraceHop) NATStatus {
	out := NATStatus{
		Kind:      "unknown",
		Label:     "No concluyente",
		Detail:    "Algunos saltos iniciales del ISP bloquean ICMP.",
		FirstHops: make([]TraceHop, len(hops)),
	}
	copy(out.FirstHops, hops)

	var privateHops []string
	seenPrivate := make(map[string]bool)
	firstPublic := ""

	for _, h := range hops {
		if h.TimedOut || h.IP == "" || h.IP == "*" {
			continue
		}
		if IsCGNATIP(h.IP) {
			out.Kind = "cgnat"
			out.Label = "CGNAT detectado"
			out.CGNATHopIP = h.IP
			out.Detail = fmt.Sprintf("El salto %d (%s) pertenece al rango CGNAT 100.64.0.0/10: IP compartida por tu ISP (puede causar NAT estricta o impedir abrir puertos).", h.TTL, h.IP)
			return out
		}
		if IsPrivateIPv4(h.IP) {
			if !seenPrivate[h.IP] {
				seenPrivate[h.IP] = true
				privateHops = append(privateHops, h.IP)
			}
		} else if firstPublic == "" {
			firstPublic = h.IP
		}
	}

	if len(privateHops) >= 2 {
		out.Kind = "double_nat"
		out.Label = "Doble NAT detectado"
		out.DoubleNATHopIP = privateHops[1]
		out.Detail = fmt.Sprintf("Se detectaron 2 saltos privados consecutivos (%s → %s): tu router está conectado detrás del módem del ISP sin modo Bridge.", privateHops[0], privateHops[1])
		return out
	}

	if len(privateHops) == 1 && firstPublic != "" {
		out.Kind = "public"
		out.Label = "NAT simple (Salida directa)"
		out.Detail = fmt.Sprintf("Un solo router local (%s) antes de la salida pública (%s): topología ideal sin Doble NAT ni CGNAT.", privateHops[0], firstPublic)
		return out
	}

	if len(privateHops) == 0 && firstPublic != "" {
		out.Kind = "public"
		out.Label = "Salida pública directa"
		out.Detail = fmt.Sprintf("El primer salto (%s) ya es público.", firstPublic)
		return out
	}

	return out
}

// RunNetworkAudit runs CGNAT/Double-NAT detection, Path MTU Discovery, and IPv6 connectivity check concurrently.
func RunNetworkAudit(ctx context.Context) NetworkAuditResult {
	var (
		wg      sync.WaitGroup
		natRes  NATStatus
		mtuRes  MTUStatus
		ipv6Res IPv6Status
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		natRes = probeEarlyNAT(ctx, "1.1.1.1")
	}()
	go func() {
		defer wg.Done()
		mtuRes = DiscoverPathMTU(ctx, "1.1.1.1")
	}()
	go func() {
		defer wg.Done()
		ipv6Res = CheckIPv6Connectivity(ctx)
	}()
	wg.Wait()

	note := "Conexión saludable para gaming y videollamadas."
	if natRes.Kind == "cgnat" {
		note = "Tu proveedor usa CGNAT: si jugás P2P o hosteás servidores, consultá por IPv4 pública o usá IPv6."
	} else if natRes.Kind == "double_nat" {
		note = "Doble NAT detectado: activá modo Bridge o DMZ en el módem del ISP hacia tu router principal."
	} else if mtuRes.Fragmented {
		note = fmt.Sprintf("MTU reducido (%d bytes): ajustá MSS/MTU en tu router si notás páginas lentas.", mtuRes.PathMTU)
	}

	return NetworkAuditResult{
		CompletedAt: time.Now().Format(time.RFC3339),
		NAT:         natRes,
		MTU:         mtuRes,
		IPv6:        ipv6Res,
		OverallNote: note,
	}
}

func probeEarlyNAT(ctx context.Context, target string) NATStatus {
	destAddr, err := resolveTraceIPv4(ctx, target)
	if err != nil {
		return NATStatus{Kind: "unknown", Label: "Sin ruta IPv4", Detail: err.Error(), FirstHops: []TraceHop{}}
	}

	handle, _, err := procTraceIcmpCreateFile.Call()
	if handle == uintptr(windows.InvalidHandle) || handle == 0 {
		return NATStatus{Kind: "unknown", Label: "Error ICMP", Detail: fmt.Sprint(err), FirstHops: []TraceHop{}}
	}
	defer procTraceIcmpCloseHandle.Call(handle)

	hops := make([]TraceHop, 0, 4)
	for ttl := 1; ttl <= 4; ttl++ {
		if ctx.Err() != nil {
			break
		}
		hop := probeTraceHop(handle, destAddr, ttl)
		hops = append(hops, hop)
		if hop.IsTarget {
			break
		}
	}
	return ClassifyNATFromHops(hops)
}

// DiscoverPathMTU finds the maximum unfragmented IPv4 MTU (including 28-byte IP+ICMP headers) using DF bit.
func DiscoverPathMTU(ctx context.Context, target string) MTUStatus {
	out := MTUStatus{
		StandardMTU: standardEthernetM,
		Target:      target,
	}

	destAddr, err := resolveTraceIPv4(ctx, target)
	if err != nil {
		out.Detail = fmt.Sprintf("No se pudo medir MTU: %v", err)
		return out
	}

	handle, _, err := procTraceIcmpCreateFile.Call()
	if handle == uintptr(windows.InvalidHandle) || handle == 0 {
		out.Detail = fmt.Sprintf("No se pudo abrir ICMP handle: %v", err)
		return out
	}
	defer procTraceIcmpCloseHandle.Call(handle)

	probePayload := func(payloadLen int) bool {
		if ctx.Err() != nil {
			return false
		}
		payload := make([]byte, payloadLen)
		for i := range payload {
			payload[i] = byte('A' + (i % 26))
		}
		// Buffer must hold header + full payload + ICMP error padding to avoid IP_BUF_TOO_SMALL.
		replySize := int(unsafe.Sizeof(traceICMPEchoReply{})) + payloadLen + 64
		replyBuf := make([]byte, replySize)
		opts := traceIPOptionInformation{
			TTL:   64,
			Flags: ipFlagDF,
		}

		ret, _, _ := procTraceIcmpSendEcho.Call(
			handle,
			uintptr(destAddr),
			uintptr(unsafe.Pointer(&payload[0])),
			uintptr(len(payload)),
			uintptr(unsafe.Pointer(&opts)),
			uintptr(unsafe.Pointer(&replyBuf[0])),
			uintptr(replySize),
			uintptr(700),
		)
		if ret == 0 {
			return false
		}
		reply := (*traceICMPEchoReply)(unsafe.Pointer(&replyBuf[0]))
		if reply.Status == ipPacketTooBig {
			return false
		}
		return reply.Status == traceIPSuccess
	}

	// Fast-path standard 1500-byte Ethernet MTU (1472 payload + 28 header).
	if probePayload(1472) {
		out.PathMTU = 1500
		out.Fragmented = false
		out.Detail = "MTU 1500 bytes óptimo sin fragmentación IPv4."
		return out
	}

	low, high := 1172, 1471
	bestPayload := 0
	for low <= high {
		if ctx.Err() != nil {
			break
		}
		mid := (low + high) / 2
		if probePayload(mid) {
			bestPayload = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	if bestPayload > 0 {
		out.PathMTU = bestPayload + ipv4ICMPOverhead
		out.Fragmented = out.PathMTU < 1500
		if out.PathMTU >= 1480 && out.PathMTU <= 1492 {
			out.Detail = fmt.Sprintf("MTU detectado %d bytes (típico de enlaces PPPoE/ADSL/fibra encapsulada).", out.PathMTU)
		} else if out.Fragmented {
			out.Detail = fmt.Sprintf("MTU limitado a %d bytes (< 1500): posible túnel VPN o encapsulamiento.", out.PathMTU)
		} else {
			out.Detail = fmt.Sprintf("MTU detectado: %d bytes.", out.PathMTU)
		}
		return out
	}

	out.Detail = "El destino o firewall intermedio bloqueó paquetes grandes con flag DF."
	return out
}

// CheckIPv6Connectivity verifies local global IPv6 assignment and live IPv6 routing to Cloudflare DNS.
func CheckIPv6Connectivity(ctx context.Context) IPv6Status {
	out := IPv6Status{}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.IP.To4() != nil || ipNet.IP.IsLoopback() {
				continue
			}
			if ipNet.IP.IsLinkLocalUnicast() {
				out.HasLocalIPv6 = true
				continue
			}
			if ipNet.IP.IsGlobalUnicast() {
				out.HasLocalIPv6 = true
				out.HasGlobalIPv6 = true
			}
		}
	}

	v6Ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	dialer := net.Dialer{}
	start := time.Now()
	conn, dialErr := dialer.DialContext(v6Ctx, "tcp6", "[2606:4700:4700::1111]:443")
	if dialErr == nil {
		_ = conn.Close()
		out.Reachable = true
		out.RTTMs = round1(float64(time.Since(start).Microseconds()) / 1000.0)
		out.Detail = fmt.Sprintf("IPv6 operativo de extremo a extremo (%.1f ms).", out.RTTMs)
		return out
	}

	if !out.HasGlobalIPv6 {
		out.Detail = "Sin dirección IPv6 global asignada (tu red opera solo con IPv4)."
	} else {
		out.Detail = "Tienes IPv6 asignada localmente, pero sin salida a Internet IPv6."
	}
	return out
}
