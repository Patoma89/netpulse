package radar

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"netpulse/internal/config"
	"netpulse/internal/netinfo"
)

const arpWorkerCount = 64

// DiscoveryHooks abstracts active LAN discovery protocols for clean unit testing.
type DiscoveryHooks struct {
	DiscoverMDNS func(ctx context.Context, localIP string, duration time.Duration) map[string]MDNSInfo
	DiscoverSSDP func(ctx context.Context, localIP string, duration time.Duration) map[string]SSDPInfo
	QueryNetBIOS func(ctx context.Context, ip string, timeout time.Duration) string
	LookupAddr   func(ctx context.Context, ip string) string
}

// Scanner performs non-intrusive concurrent LAN discovery via ARP, mDNS, SSDP, and NetBIOS.
type Scanner struct {
	arp   ARPScanner
	hooks DiscoveryHooks
}

// NewScanner builds a production LAN Scanner backed by Win32 ARP and Go discovery protocols.
func NewScanner() *Scanner {
	return NewScannerWithDeps(NewWinARPScanner(), DiscoveryHooks{
		DiscoverMDNS: DiscoverMDNS,
		DiscoverSSDP: DiscoverSSDP,
		QueryNetBIOS: QueryNetBIOSName,
		LookupAddr:   defaultReverseLookup,
	})
}

// NewScannerWithDeps constructs a Scanner with injectable ARP and protocol hooks for testing.
func NewScannerWithDeps(arp ARPScanner, hooks DiscoveryHooks) *Scanner {
	return &Scanner{
		arp:   arp,
		hooks: hooks,
	}
}

type discoveredHost struct {
	ip        string
	mac       string
	rttMs     float64
	isLocal   bool
	isGateway bool
	protocols map[string]struct{}
	services  []string
	hostname  string
	netbios   string
	mdns      string
	ssdp      string
}

// Scan executes a multi-protocol LAN scan and returns the classified ScanResult along with a DeviceKey->MAC map.
func (s *Scanner) Scan(
	ctx context.Context,
	summary netinfo.NetworkSummary,
	knownDevices map[string]config.KnownDevice,
	onProgress func(ScanProgress),
) (ScanResult, map[string]string, error) {
	if !summary.Connected || summary.LocalIP == "" {
		err := errors.New("no hay una conexión de red local activa para escanear")
		emitRadarProgress(onProgress, ScanProgress{Stage: "error", ProgressPct: 0, Message: err.Error()})
		return ScanResult{}, nil, err
	}

	var prefixLen uint8 = 24
	var ifIndex uint32
	if summary.Primary != nil {
		prefixLen = summary.Primary.PrefixLen
		ifIndex = summary.Primary.IfIndex
	}

	cidr, hosts, err := SubnetHostIPs(summary.LocalIP, prefixLen)
	if err != nil {
		emitRadarProgress(onProgress, ScanProgress{Stage: "error", ProgressPct: 0, Message: err.Error()})
		return ScanResult{}, nil, err
	}

	started := time.Now()
	emitRadarProgress(onProgress, ScanProgress{
		Stage:       "arp",
		ProgressPct: 10,
		FoundCount:  1,
		Message:     "Leyendo caché ARP y sondeando subred local...",
	})

	hostMap := make(map[string]*discoveredHost)
	var hostMu sync.Mutex

	ensureHost := func(ip string) *discoveredHost {
		h, ok := hostMap[ip]
		if !ok {
			h = &discoveredHost{
				ip:        ip,
				protocols: make(map[string]struct{}),
			}
			hostMap[ip] = h
		}
		return h
	}

	// Always seed local PC
	localNormMAC := config.NormalizeMAC(summary.AdapterMAC)
	if localNormMAC != "" {
		localHost := ensureHost(summary.LocalIP)
		localHost.mac = localNormMAC
		localHost.isLocal = true
		localHost.rttMs = 0.1
		localHost.protocols["Local"] = struct{}{}
		if hn, err := os.Hostname(); err == nil && hn != "" {
			localHost.hostname = hn
		}
	}

	// Seed existing Windows ARP cache
	if s.arp != nil {
		if entries, err := s.arp.ReadARPTable(ifIndex); err == nil {
			for _, e := range entries {
				norm := config.NormalizeMAC(e.MAC)
				if norm == "" {
					continue
				}
				h := ensureHost(e.IP)
				if h.mac == "" {
					h.mac = norm
				}
				h.protocols["ARP"] = struct{}{}
				if e.IP == summary.GatewayIP {
					h.isGateway = true
				}
			}
		}
	}

	// Concurrent SendARP sweep
	jobs := make(chan string, len(hosts))
	for _, ip := range hosts {
		if ip == summary.LocalIP {
			continue
		}
		jobs <- ip
	}
	close(jobs)

	var completed atomic.Int32
	totalToProbe := len(hosts)
	if totalToProbe == 0 {
		totalToProbe = 1
	}

	var arpWG sync.WaitGroup
	workers := arpWorkerCount
	if len(hosts) < workers {
		workers = len(hosts)
	}
	if workers < 1 {
		workers = 1
	}

	for w := 0; w < workers; w++ {
		arpWG.Add(1)
		go func() {
			defer arpWG.Done()
			for ip := range jobs {
				if ctx.Err() != nil {
					return
				}
				if s.arp != nil {
					mac, rtt, ok := s.arp.ProbeARP(ctx, ip, summary.LocalIP)
					if ok && mac != "" {
						norm := config.NormalizeMAC(mac)
						hostMu.Lock()
						h := ensureHost(ip)
						h.mac = norm
						if h.rttMs == 0 || rtt < h.rttMs {
							h.rttMs = rtt
						}
						h.protocols["ARP"] = struct{}{}
						if ip == summary.GatewayIP {
							h.isGateway = true
						}
						count := len(hostMap)
						hostMu.Unlock()

						done := int(completed.Add(1))
						if done%24 == 0 {
							pct := 10 + (done*50)/totalToProbe
							emitRadarProgress(onProgress, ScanProgress{
								Stage:       "arp",
								ProgressPct: pct,
								FoundCount:  count,
								Message:     "Sondeando dispositivos en la red...",
							})
						}
						continue
					}
				}
				done := int(completed.Add(1))
				if done%32 == 0 {
					hostMu.Lock()
					count := len(hostMap)
					hostMu.Unlock()
					pct := 10 + (done*50)/totalToProbe
					emitRadarProgress(onProgress, ScanProgress{
						Stage:       "arp",
						ProgressPct: pct,
						FoundCount:  count,
						Message:     "Sondeando dispositivos en la red...",
					})
				}
			}
		}()
	}
	arpWG.Wait()

	if ctx.Err() != nil {
		return ScanResult{}, nil, ctx.Err()
	}

	hostMu.Lock()
	foundIPs := make([]string, 0, len(hostMap))
	for ip := range hostMap {
		foundIPs = append(foundIPs, ip)
	}
	hostMu.Unlock()

	emitRadarProgress(onProgress, ScanProgress{
		Stage:       "discovery",
		ProgressPct: 65,
		FoundCount:  len(foundIPs),
		Message:     "Consultando mDNS, SSDP/UPnP y NetBIOS...",
	})

	// Parallel discovery phase: mDNS, SSDP, NetBIOS, Reverse DNS
	var (
		discWG   sync.WaitGroup
		mdnsMap  map[string]MDNSInfo
		ssdpMap  map[string]SSDPInfo
		nbMap    = make(map[string]string)
		rdnsMap  = make(map[string]string)
		nbMu     sync.Mutex
		rdnsMu   sync.Mutex
	)

	if s.hooks.DiscoverMDNS != nil {
		discWG.Add(1)
		go func() {
			defer discWG.Done()
			mdnsMap = s.hooks.DiscoverMDNS(ctx, summary.LocalIP, 850*time.Millisecond)
		}()
	}

	if s.hooks.DiscoverSSDP != nil {
		discWG.Add(1)
		go func() {
			defer discWG.Done()
			ssdpMap = s.hooks.DiscoverSSDP(ctx, summary.LocalIP, 850*time.Millisecond)
		}()
	}

	if s.hooks.QueryNetBIOS != nil {
		discWG.Add(1)
		go func() {
			defer discWG.Done()
			var nbWG sync.WaitGroup
			for _, ip := range foundIPs {
				nbWG.Add(1)
				go func(targetIP string) {
					defer nbWG.Done()
					if name := s.hooks.QueryNetBIOS(ctx, targetIP, 250*time.Millisecond); name != "" {
						nbMu.Lock()
						nbMap[targetIP] = name
						nbMu.Unlock()
					}
				}(ip)
			}
			nbWG.Wait()
		}()
	}

	if s.hooks.LookupAddr != nil {
		discWG.Add(1)
		go func() {
			defer discWG.Done()
			var rdnsWG sync.WaitGroup
			for _, ip := range foundIPs {
				rdnsWG.Add(1)
				go func(targetIP string) {
					defer rdnsWG.Done()
					if name := s.hooks.LookupAddr(ctx, targetIP); name != "" {
						rdnsMu.Lock()
						rdnsMap[targetIP] = name
						rdnsMu.Unlock()
					}
				}(ip)
			}
			rdnsWG.Wait()
		}()
	}

	discWG.Wait()

	emitRadarProgress(onProgress, ScanProgress{
		Stage:       "classify",
		ProgressPct: 90,
		FoundCount:  len(hostMap),
		Message:     "Identificando fabricantes OUI y verificando dispositivos conocidos...",
	})

	for ip, info := range mdnsMap {
		if h, ok := hostMap[ip]; ok {
			if info.Hostname != "" {
				h.mdns = info.Hostname
			}
			if len(info.Services) > 0 {
				h.services = mergeUniqueStrings(h.services, info.Services)
			}
			h.protocols["mDNS"] = struct{}{}
		}
	}
	for ip, info := range ssdpMap {
		if h, ok := hostMap[ip]; ok {
			if info.FriendlyName != "" {
				h.ssdp = info.FriendlyName
			} else if info.Server != "" {
				h.ssdp = info.Server
			}
			if info.ServiceType != "" {
				h.services = mergeUniqueStrings(h.services, []string{info.ServiceType})
			}
			h.protocols["SSDP"] = struct{}{}
		}
	}
	for ip, name := range nbMap {
		if h, ok := hostMap[ip]; ok {
			h.netbios = name
			h.protocols["NetBIOS"] = struct{}{}
		}
	}
	for ip, name := range rdnsMap {
		if h, ok := hostMap[ip]; ok && h.hostname == "" {
			h.hostname = name
		}
	}

	nowISO := time.Now().Format(time.RFC3339)
	devices := make([]Device, 0, len(hostMap))
	macByKey := make(map[string]string, len(hostMap))
	trustedCount := 0
	unknownCount := 0
	newCount := 0

	for _, h := range hostMap {
		if h.mac == "" {
			continue
		}
		vendor, isRand := LookupVendor(h.mac)
		key := DeviceKey(h.mac)
		macByKey[key] = h.mac

		saved, hasSaved := knownDevices[h.mac]
		firstSeen := nowISO
		if hasSaved && saved.FirstSeen != "" {
			firstSeen = saved.FirstSeen
		}
		lastSeen := nowISO
		if hasSaved && saved.LastSeen != "" {
			lastSeen = saved.LastSeen
		}
		seenCount := saved.SeenCount
		if seenCount <= 0 {
			seenCount = 1
		}

		trusted := hasSaved && saved.Trusted
		if h.isLocal {
			trusted = true
		}
		isNew := !h.isLocal && !hasSaved

		dev := Device{
			Key:             key,
			IP:              h.ip,
			MAC:             h.mac,
			Hostname:        h.hostname,
			NetBIOSName:     h.netbios,
			MDNSName:        h.mdns,
			SSDPName:        h.ssdp,
			Vendor:          vendor,
			IsRandomizedMAC: isRand,
			Alias:           saved.Alias,
			Trusted:         trusted,
			IsNew:           isNew,
			IsLocal:         h.isLocal,
			IsGateway:       h.isGateway,
			RTTMs:           h.rttMs,
			Protocols:       sortedProtocolKeys(h.protocols),
			Services:        h.services,
			FirstSeen:       firstSeen,
			LastSeen:        lastSeen,
			SeenCount:       seenCount,
			Online:          true,
			LastTransition:  saved.LastTransition,
		}

		if saved.Category != "" {
			dev.Category = saved.Category
		} else {
			dev.Category = InferCategory(dev)
		}

		if dev.Trusted {
			trustedCount++
		} else {
			unknownCount++
		}
		if dev.IsNew {
			newCount++
		}

		devices = append(devices, dev)
	}

	SortDevices(devices)

	res := ScanResult{
		Subnet:       cidr,
		ScannedHosts: len(hosts),
		DurationMs:   time.Since(started).Milliseconds(),
		CompletedAt:  nowISO,
		TotalDevices: len(devices),
		TrustedCount: trustedCount,
		UnknownCount: unknownCount,
		NewCount:     newCount,
		Devices:      devices,
	}

	emitRadarProgress(onProgress, ScanProgress{
		Stage:       "done",
		ProgressPct: 100,
		FoundCount:  len(devices),
		Message:     "Escaneo LAN completado",
	})

	return res, macByKey, nil
}

// SortDevices orders gateway first, then local PC, then untrusted/new devices, then ascending IPv4.
func SortDevices(devices []Device) {
	sort.Slice(devices, func(i, j int) bool {
		a, b := devices[i], devices[j]
		if a.IsGateway != b.IsGateway {
			return a.IsGateway
		}
		if a.IsLocal != b.IsLocal {
			return a.IsLocal
		}
		if a.Trusted != b.Trusted {
			return !a.Trusted
		}
		if a.IsNew != b.IsNew {
			return a.IsNew
		}
		return ipv4ToUint32(a.IP) < ipv4ToUint32(b.IP)
	})
}

func ipv4ToUint32(ipStr string) uint32 {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
}

func sortedProtocolKeys(m map[string]struct{}) []string {
	order := []string{"Local", "ARP", "mDNS", "SSDP", "NetBIOS"}
	out := make([]string, 0, len(m))
	for _, k := range order {
		if _, ok := m[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

func mergeUniqueStrings(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	out := make([]string, 0, len(existing)+len(incoming))
	for _, s := range existing {
		if _, ok := seen[s]; !ok && s != "" {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	for _, s := range incoming {
		if _, ok := seen[s]; !ok && s != "" {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

func defaultReverseLookup(ctx context.Context, ip string) string {
	lookupCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(lookupCtx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(names[0]), ".")
}

func emitRadarProgress(cb func(ScanProgress), p ScanProgress) {
	if cb != nil {
		cb(p)
	}
}
