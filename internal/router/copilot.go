package router

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"netpulse/internal/netinfo"
	"netpulse/internal/radar"
)

var reHTMLTitle = regexp.MustCompile(`(?is)<title[^>]*>\s*([^<]+?)\s*</title>`)

// DetectRouterProfile fingerprints the local default gateway via ARP OUI + safe HTTP header/title probe.
func DetectRouterProfile(
	ctx context.Context,
	summary netinfo.NetworkSummary,
	rec24Ch int,
	rec5Ch int,
	recDNSName string,
	recDNSPrimary string,
) RouterProfile {
	profile := RouterProfile{
		Detected:      summary.Connected && summary.GatewayIP != "",
		GatewayIP:     summary.GatewayIP,
		Rec24Channel:  rec24Ch,
		Rec5Channel:   rec5Ch,
		RecDNSName:    recDNSName,
		RecDNSPrimary: recDNSPrimary,
	}
	if !profile.Detected {
		profile.MenuGuide = "Conectate a tu red local para detectar la puerta de enlace y generar la guía del router."
		return profile
	}

	profile.AdminURL = "http://" + summary.GatewayIP

	if ip := net.ParseIP(summary.GatewayIP); ip != nil && ip.IsPrivate() {
		scanner := radar.NewWinARPScanner()
		if mac, _, ok := scanner.ProbeARP(ctx, summary.GatewayIP, summary.LocalIP); ok && mac != "" {
			profile.GatewayMAC = mac
			vendor, _ := radar.LookupVendor(mac)
			profile.Vendor = vendor
		}

		title, server := probeGatewayHTTP(ctx, profile.AdminURL)
		profile.HTTPTitle = title
		profile.ServerHeader = server
	}

	if profile.Vendor == "" {
		profile.Vendor = "Router / Módem Local"
	}

	profile.SuggestedDriver = inferAutoPilotDriver(profile.Vendor, profile.HTTPTitle, profile.ServerHeader)
	profile.AutoPilotSupported = profile.SuggestedDriver != ""
	profile.MenuGuide = buildBrandMenuGuide(profile.Vendor, profile.HTTPTitle, profile.SuggestedDriver)

	return profile
}

func probeGatewayHTTP(ctx context.Context, adminURL string) (title, server string) {
	reqCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer cancel()

	client := &http.Client{Timeout: 1200 * time.Millisecond}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, adminURL, nil)
	if err != nil {
		return "", ""
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	server = strings.TrimSpace(resp.Header.Get("Server"))
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err == nil && len(body) > 0 {
		if m := reHTMLTitle.FindStringSubmatch(string(body)); len(m) > 1 {
			title = strings.TrimSpace(m[1])
		}
	}
	return title, server
}

func inferAutoPilotDriver(vendor, title, server string) string {
	combined := strings.ToLower(vendor + " " + title + " " + server)
	switch {
	case strings.Contains(combined, "openwrt") || strings.Contains(combined, "luci") || strings.Contains(combined, "gl.inet"):
		return "openwrt"
	case strings.Contains(combined, "mikrotik") || strings.Contains(combined, "routeros"):
		return "mikrotik"
	case strings.Contains(combined, "fritz") || strings.Contains(combined, "avm"):
		return "tr064"
	default:
		return ""
	}
}

func buildBrandMenuGuide(vendor, title, driver string) string {
	combined := strings.ToLower(vendor + " " + title)
	switch {
	case driver == "openwrt":
		return "Detectado firmware OpenWrt / LuCI: en el panel web entrá a Network → Wireless → Edit → Channel, o usá la pestaña Auto-Piloto Seguro (SSH/UCI)."
	case driver == "mikrotik":
		return "Detectado MikroTik RouterOS: en WinBox/WebFig entrá a Wireless / WiFi → Frequency, o usá la pestaña Auto-Piloto Seguro (REST API)."
	case driver == "tr064":
		return "Detectado AVM FRITZ!Box: en fritz.box entrá a Wi-Fi → Radio Channel, o usá la pestaña Auto-Piloto Seguro (TR-064)."
	case strings.Contains(combined, "tp-link"):
		return "En TP-Link: ingresá a Avanzado → Inalámbrico → Configuración inalámbrica → Canal (elegí el canal recomendado y guardá)."
	case strings.Contains(combined, "huawei"):
		return "En módems Huawei (ISP): buscá WLAN / Red Local → Configuración WLAN básica → Canal (desactivá 'Automático' y elegí el canal recomendado)."
	case strings.Contains(combined, "zte"):
		return "En módems ZTE (ISP): entrá a Local Network → WLAN → WLAN Basic → Channel (pasá de Auto al canal recomendado)."
	case strings.Contains(combined, "sagemcom"):
		return "En módems Sagemcom (ISP): entrá a Wi-Fi → Radio 2.4G / 5G → Canal Manual y seleccioná el canal recomendado."
	case strings.Contains(combined, "arris"):
		return "En módems ARRIS: entrá a Wireless → 2.4 GHz / 5 GHz → Control Channel y seleccioná el canal recomendado."
	case strings.Contains(combined, "technicolor"):
		return "En módems Technicolor: entrá a Wireless → Configuration → Channel y seleccioná el canal recomendado."
	case strings.Contains(combined, "asus"):
		return "En ASUS: entrá a Wireless → General → Control Channel y aplicá el canal recomendado (evitá canales DFS si usás 5 GHz)."
	case strings.Contains(combined, "ubiquiti") || strings.Contains(combined, "unifi"):
		return "En UniFi: entrá a UniFi Devices → tu AP → Settings → Radios → Channel."
	default:
		return "Abrí el panel del módem/router con el botón de arriba, buscá la sección Wi-Fi / WLAN → Canal (Channel), cambiá de Automático al canal recomendado y luego presioná 'Medir Después'."
	}
}

// CompareSnapshots computes before/after deltas for the Co-Pilot verification workflow.
func CompareSnapshots(before, after BeforeAfterSnapshot) BeforeAfterReport {
	deltaRouter := math.Round((after.RouterAvgMs-before.RouterAvgMs)*10.0) / 10.0
	deltaJitter := math.Round((after.RouterJitterMs-before.RouterJitterMs)*10.0) / 10.0
	deltaCong := after.WiFiCongestion - before.WiFiCongestion
	deltaScore := after.Score - before.Score

	var verdict string
	switch {
	case deltaScore > 0 || deltaRouter < -1.0 || deltaJitter < -0.5 || deltaCong < -5:
		verdict = fmt.Sprintf(
			"Mejora confirmada: puntaje %+d, latencia router %+.1f ms, jitter %+.1f ms, congestión Wi-Fi %+d%%.",
			deltaScore, deltaRouter, deltaJitter, deltaCong,
		)
	case deltaScore == 0 && math.Abs(deltaRouter) <= 1.0 && deltaCong == 0:
		verdict = "Sin cambios apreciables entre la medición base y la actual."
	default:
		verdict = fmt.Sprintf(
			"Variación medida: puntaje %+d, latencia router %+.1f ms, congestión %+d%%. Si empeoró, restaurá el canal anterior.",
			deltaScore, deltaRouter, deltaCong,
		)
	}

	afterCopy := after
	return BeforeAfterReport{
		HasBaseline:     true,
		Baseline:        before,
		After:           &afterCopy,
		DeltaRouterMs:   deltaRouter,
		DeltaJitterMs:   deltaJitter,
		DeltaCongestion: deltaCong,
		DeltaScore:      deltaScore,
		Verdict:         verdict,
	}
}

// OpenAdminPanel safely opens the local private router admin HTTP/HTTPS URL via Win32 ShellExecute.
func OpenAdminPanel(rawURL, expectedGatewayIP string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("URL de panel de router inválida")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsPrivate() {
		return errors.New("por seguridad, solo se permite abrir paneles de IPs privadas locales")
	}
	if expectedGatewayIP != "" && u.Hostname() != expectedGatewayIP {
		return errors.New("la URL no coincide con la puerta de enlace detectada")
	}

	verb := windows.StringToUTF16Ptr("open")
	target := windows.StringToUTF16Ptr(u.String())
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}
