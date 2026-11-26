package radar

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"netpulse/internal/config"
)

// ServicePort represents one probed standard LAN service port on a single target host.
type ServicePort struct {
	Port  int     `json:"port"`
	Name  string  `json:"name"`
	Open  bool    `json:"open"`
	RTTMs float64 `json:"rttMs"`
}

// DeviceServicesResult summarizes gentle on-demand service checks for one device.
type DeviceServicesResult struct {
	DeviceKey string        `json:"deviceKey"`
	IP        string        `json:"ip"`
	CheckedAt string        `json:"checkedAt"`
	Services  []ServicePort `json:"services"`
}

var gentleServicePorts = []struct {
	port int
	name string
}{
	{22, "SSH"},
	{53, "DNS"},
	{80, "HTTP"},
	{443, "HTTPS"},
	{445, "SMB / Archivos"},
	{548, "AFP (macOS)"},
	{3389, "RDP (Escritorio Remoto)"},
	{8080, "Web / Proxy Alt"},
	{9100, "Impresora JetDirect"},
}

// InspectDeviceServices probes 9 standard LAN ports on a single host with a 280ms timeout.
// Limiting to 9 ports on one user-selected host avoids mass-sweep AV/IoT issues.
func InspectDeviceServices(ctx context.Context, deviceKey, ip string) DeviceServicesResult {
	res := DeviceServicesResult{
		DeviceKey: deviceKey,
		IP:        ip,
		CheckedAt: time.Now().Format(time.RFC3339),
		Services:  make([]ServicePort, 0),
	}
	if net.ParseIP(strings.TrimSpace(ip)) == nil {
		return res
	}

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		open = make([]ServicePort, 0, len(gentleServicePorts))
	)

	for _, sp := range gentleServicePorts {
		wg.Add(1)
		go func(port int, name string) {
			defer wg.Done()
			if rtt, ok := probeTCPPort(ctx, ip, port, 280*time.Millisecond); ok {
				mu.Lock()
				open = append(open, ServicePort{
					Port:  port,
					Name:  name,
					Open:  true,
					RTTMs: rtt,
				})
				mu.Unlock()
			}
		}(sp.port, sp.name)
	}
	wg.Wait()

	sort.Slice(open, func(i, j int) bool {
		return open[i].Port < open[j].Port
	})
	res.Services = open
	return res
}

func probeTCPPort(ctx context.Context, ip string, port int, timeout time.Duration) (float64, bool) {
	if ctx.Err() != nil {
		return 0, false
	}
	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ip, strconv.Itoa(port)))
	elapsed := time.Since(start)
	if err != nil {
		return 0, false
	}
	_ = conn.Close()

	rtt := math.Round(float64(elapsed.Microseconds())/100.0) / 10.0
	if rtt < 0.1 {
		rtt = 0.1
	}
	return rtt, true
}

// BuildMagicPacket constructs a standard 102-byte Wake-on-LAN Magic Packet for mac.
func BuildMagicPacket(mac string) ([]byte, error) {
	norm := config.NormalizeMAC(mac)
	parts := strings.Split(norm, ":")
	if len(parts) != 6 {
		return nil, fmt.Errorf("dirección MAC inválida para Wake-on-LAN: %q", mac)
	}

	var macBytes [6]byte
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("octeto MAC inválido %q", p)
		}
		macBytes[i] = byte(v)
	}

	pkt := make([]byte, 102)
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	for rep := 0; rep < 16; rep++ {
		copy(pkt[6+rep*6:6+(rep+1)*6], macBytes[:])
	}
	return pkt, nil
}

// SendWakeOnLAN broadcasts a Wake-on-LAN Magic Packet over UDP port 9.
func SendWakeOnLAN(mac, localIP string, prefixLen uint8) error {
	pkt, err := BuildMagicPacket(mac)
	if err != nil {
		return err
	}

	targets := []string{"255.255.255.255:9"}
	if bcast := subnetBroadcastIP(localIP, prefixLen); bcast != "" {
		targets = append([]string{net.JoinHostPort(bcast, "9")}, targets...)
	}

	var bindIP net.IP
	if parsed := net.ParseIP(localIP).To4(); parsed != nil {
		bindIP = parsed
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		return fmt.Errorf("no se pudo abrir socket UDP para Wake-on-LAN: %w", err)
	}
	defer conn.Close()

	sent := false
	var lastErr error
	for _, target := range targets {
		udpAddr, err := net.ResolveUDPAddr("udp4", target)
		if err != nil {
			lastErr = err
			continue
		}
		if _, err := conn.WriteToUDP(pkt, udpAddr); err != nil {
			lastErr = err
			continue
		}
		sent = true
	}

	if !sent {
		if lastErr != nil {
			return lastErr
		}
		return errors.New("no se pudo enviar el paquete Wake-on-LAN")
	}
	return nil
}

func subnetBroadcastIP(localIP string, prefixLen uint8) string {
	ip := net.ParseIP(localIP).To4()
	if ip == nil {
		return ""
	}
	if prefixLen == 0 || prefixLen > 30 {
		prefixLen = 24
	}
	mask := net.CIDRMask(int(prefixLen), 32)
	netInt := binary.BigEndian.Uint32(ip.Mask(mask))
	maskInt := binary.BigEndian.Uint32(mask)
	bcastInt := netInt | ^maskInt
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], bcastInt)
	return net.IP(b[:]).String()
}
