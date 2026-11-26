package report

import (
	"fmt"
	"strings"
	"time"

	"netpulse/internal/diagnostics"
	"netpulse/internal/netinfo"
	"netpulse/internal/optimizer"
	"netpulse/internal/privacy"
	"netpulse/internal/radar"
	"netpulse/internal/router"
	"netpulse/internal/wifi"
)

// ReportInput aggregates current module states to generate a shareable Discord report.
type ReportInput struct {
	Version     string
	ForceRedact bool
	Network     netinfo.NetworkSummary
	DualHop     diagnostics.DualHopSnapshot
	Bufferbloat *diagnostics.BufferbloatResult
	Radar       radar.ScanResult
	WiFi        wifi.ScanResult
	DNS         optimizer.DNSBenchmarkResult
	UPnP        optimizer.UPnPScanResult
	Router      router.RouterProfile
}

// DiscordReport contains structured summary fields and ready-to-paste Markdown/PlainText.
type DiscordReport struct {
	GeneratedAt      string   `json:"generatedAt"`
	Version          string   `json:"version"`
	Redacted         bool     `json:"redacted"`
	HealthScore      int      `json:"healthScore"`
	VerdictTitle     string   `json:"verdictTitle"`
	VerdictSeverity  string   `json:"verdictSeverity"`
	LinkLabel        string   `json:"linkLabel"`
	RouterLatency    string   `json:"routerLatency"`
	InternetLatency  string   `json:"internetLatency"`
	BufferbloatGrade string   `json:"bufferbloatGrade"`
	WiFiSummary      string   `json:"wifiSummary"`
	RadarSummary     string   `json:"radarSummary"`
	DNSSummary       string   `json:"dnsSummary"`
	UPnPSummary      string   `json:"upnpSummary"`
	RouterSummary    string   `json:"routerSummary"`
	RedactedFields   []string `json:"redactedFields"`
	Markdown         string   `json:"markdown"`
	PlainText        string   `json:"plainText"`
}

// BuildDiscordReport constructs a safe, structured network health report for Discord sharing.
// When ForceRedact is true, zero raw IPs, MACs/BSSIDs, SSIDs, or LAN hostnames appear in output.
func BuildDiscordReport(in ReportInput) DiscordReport {
	ver := strings.TrimSpace(in.Version)
	if ver == "" {
		ver = "1.0.0"
	}

	redact := in.ForceRedact
	linkLabel := "Sin conexión activa"
	if in.Network.Connected {
		kind := "Red Local"
		switch in.Network.LinkType {
		case "wifi":
			kind = "Wi-Fi"
		case "ethernet":
			kind = "Ethernet"
		}
		if in.Network.LinkSpeedMbps > 0 {
			linkLabel = fmt.Sprintf("%s · %d Mbps", kind, in.Network.LinkSpeedMbps)
		} else {
			linkLabel = kind
		}
	}

	score := in.DualHop.Verdict.Score
	verdictTitle := in.DualHop.Verdict.Title
	verdictSeverity := in.DualHop.Verdict.Severity
	if verdictTitle == "" {
		verdictTitle = "Diagnóstico pendiente"
		verdictSeverity = "cyan"
	}

	routerLat := "Sin muestras"
	if in.DualHop.Router.Sent > 0 {
		routerLat = fmt.Sprintf(
			"%.1f ms prom · P95 %.1f ms · jitter %.1f ms · %.1f%% pérdida",
			in.DualHop.Router.AvgMs, in.DualHop.Router.P95Ms, in.DualHop.Router.JitterMs, in.DualHop.Router.LossPct,
		)
	}

	inetLat := "Sin muestras"
	if in.DualHop.Internet.Sent > 0 {
		targetLabel := in.DualHop.InternetTarget
		if targetLabel == "" {
			targetLabel = "1.1.1.1"
		}
		inetLat = fmt.Sprintf(
			"%.1f ms prom · P95 %.1f ms · jitter %.1f ms · %.1f%% pérdida (%s)",
			in.DualHop.Internet.AvgMs, in.DualHop.Internet.P95Ms, in.DualHop.Internet.JitterMs, in.DualHop.Internet.LossPct, targetLabel,
		)
	}

	bloatGrade := "No ejecutado"
	bloatLine := "No ejecutado aún"
	if in.Bufferbloat != nil && in.Bufferbloat.Grade != "" {
		bloatGrade = in.Bufferbloat.Grade
		if in.Bufferbloat.UploadSkipped {
			bloatLine = fmt.Sprintf(
				"Nota %s (Reposo %.1f ms · Descarga +%.1f ms @ %.1f Mbps · Subida omitida)",
				in.Bufferbloat.Grade, in.Bufferbloat.IdleMedianMs, in.Bufferbloat.DownloadDeltaMs, in.Bufferbloat.DownloadMbps,
			)
		} else {
			bloatLine = fmt.Sprintf(
				"Nota %s (Reposo %.1f ms · Descarga +%.1f ms @ %.1f Mbps · Subida +%.1f ms @ %.1f Mbps)",
				in.Bufferbloat.Grade, in.Bufferbloat.IdleMedianMs,
				in.Bufferbloat.DownloadDeltaMs, in.Bufferbloat.DownloadMbps,
				in.Bufferbloat.UploadDeltaMs, in.Bufferbloat.UploadMbps,
			)
		}
	}

	wifiSummary := summarizeWiFi(in.WiFi, redact)
	radarSummary := summarizeRadar(in.Radar)
	dnsSummary := summarizeDNS(in.DNS)
	upnpSummary := summarizeUPnP(in.UPnP)
	routerSummary := summarizeRouter(in.Router, redact)

	redactedFields := []string{}
	if redact {
		redactedFields = []string{
			"IP pública e IPs locales / puerta de enlace",
			"Direcciones MAC y BSSID de radios",
			"Nombre de red Wi-Fi (SSID)",
			"Hostnames y alias de equipos en Radar LAN",
			"Credenciales de router (almacenadas solo en Windows Vault)",
		}
	}

	nowStr := time.Now().Format("2006-01-02 15:04")

	var md strings.Builder
	md.WriteString(fmt.Sprintf("### NetPulse v%s · Reporte de Salud de Red\n", ver))
	if redact {
		md.WriteString("> 🔒 *Modo Discord activo: IPs, MAC/BSSID, SSID y nombres de equipos censurados.*\n\n")
	} else {
		md.WriteString("> ⚠️ *Modo sin censura: revisá antes de publicar en canales públicos.*\n\n")
	}
	md.WriteString(fmt.Sprintf("- **Puntaje / Veredicto:** `%d/100` — **%s**\n", score, verdictTitle))
	md.WriteString(fmt.Sprintf("- **Enlace Principal:** `%s`\n", linkLabel))
	md.WriteString(fmt.Sprintf("- **Salto 1 (Router):** %s\n", routerLat))
	md.WriteString(fmt.Sprintf("- **Salto 2 (Internet):** %s\n", inetLat))
	md.WriteString(fmt.Sprintf("- **Bufferbloat:** %s\n", bloatLine))
	md.WriteString(fmt.Sprintf("- **Espectro Wi-Fi:** %s\n", wifiSummary))
	md.WriteString(fmt.Sprintf("- **Radar LAN:** %s\n", radarSummary))
	md.WriteString(fmt.Sprintf("- **DNS Turbo:** %s\n", dnsSummary))
	md.WriteString(fmt.Sprintf("- **Auditoría UPnP:** %s\n", upnpSummary))
	if routerSummary != "" {
		md.WriteString(fmt.Sprintf("- **Gateway / Router:** %s\n", routerSummary))
	}
	md.WriteString(fmt.Sprintf("\n_Generado 100%% offline el %s con NetPulse v%s · 0%% telemetría_", nowStr, ver))

	markdown := md.String()
	plain := strings.ReplaceAll(markdown, "### ", "")
	plain = strings.ReplaceAll(plain, "**", "")
	plain = strings.ReplaceAll(plain, "`", "")
	plain = strings.ReplaceAll(plain, "> ", "")
	plain = strings.ReplaceAll(plain, "_", "")

	return DiscordReport{
		GeneratedAt:      time.Now().Format(time.RFC3339),
		Version:          ver,
		Redacted:         redact,
		HealthScore:      score,
		VerdictTitle:     verdictTitle,
		VerdictSeverity:  verdictSeverity,
		LinkLabel:        linkLabel,
		RouterLatency:    routerLat,
		InternetLatency:  inetLat,
		BufferbloatGrade: bloatGrade,
		WiFiSummary:      wifiSummary,
		RadarSummary:     radarSummary,
		DNSSummary:       dnsSummary,
		UPnPSummary:      upnpSummary,
		RouterSummary:    routerSummary,
		RedactedFields:   redactedFields,
		Markdown:         markdown,
		PlainText:        plain,
	}
}

func summarizeWiFi(w wifi.ScanResult, redact bool) string {
	switch w.Status {
	case "":
		return "No escaneado aún"
	case "no_wifi_adapter":
		return "Sin radio Wi-Fi (Conexión por cable Ethernet)"
	case "location_denied":
		return "Bloqueado por permiso de Ubicación en Windows 11"
	case "error":
		return "Error al leer adaptador WLAN"
	}

	ssidPart := ""
	if w.Connected && w.CurrentSSID != "" {
		if redact {
			ssidPart = fmt.Sprintf("Red %s · ", privacy.MaskSSID(w.CurrentSSID))
		} else {
			ssidPart = fmt.Sprintf("Red %s · ", w.CurrentSSID)
		}
	}

	rec := w.Rec24GHz
	if w.CurrentBand == "5GHz" || (w.Rec5GHz.RecommendedChannel > 0 && w.CurrentBand != "2.4GHz") {
		rec = w.Rec5GHz
	}

	if w.CurrentChannel > 0 {
		return fmt.Sprintf(
			"%s%s Ch %d (%d MHz, %d dBm) · Canal recomendado: Ch %d (%d%% cong.) · %d redes vecinas",
			ssidPart, w.CurrentBand, w.CurrentChannel, w.CurrentWidthMHz, w.CurrentRSSI,
			rec.RecommendedChannel, rec.BestCongestion, len(w.AccessPoints),
		)
	}
	return fmt.Sprintf("%d redes cercanas · Mejor canal %s: Ch %d", len(w.AccessPoints), rec.Band, rec.RecommendedChannel)
}

func summarizeRadar(r radar.ScanResult) string {
	if r.CompletedAt == "" && r.TotalDevices == 0 {
		return "No escaneado aún"
	}
	return fmt.Sprintf(
		"%d equipos detectados (%d confiables, %d sin confirmar) en %.2fs",
		r.TotalDevices, r.TrustedCount, r.UnknownCount, float64(r.DurationMs)/1000.0,
	)
}

func summarizeDNS(d optimizer.DNSBenchmarkResult) string {
	if d.CompletedAt == "" || len(d.Results) == 0 {
		return "Benchmark no ejecutado aún"
	}
	best := d.Results[0]
	if best.Reachable {
		return fmt.Sprintf("Más rápido: %s (%.1f ms mediana, jitter %.1f ms)", best.Provider.Name, best.MedianMs, best.JitterMs)
	}
	return "Sin respuestas DNS válidas"
}

func summarizeUPnP(u optimizer.UPnPScanResult) string {
	switch u.Status {
	case "":
		return "No auditado aún"
	case "upnp_disabled":
		return "UPnP desactivado o protegido (0 puertos expuestos)"
	case "ok":
		if len(u.Mappings) == 0 {
			return "UPnP activo con 0 puertos abiertos"
		}
		return fmt.Sprintf("%d puertos abiertos (%d riesgo alto)", len(u.Mappings), u.HighRiskCount)
	default:
		return "Error al auditar UPnP"
	}
}

func summarizeRouter(prof router.RouterProfile, redact bool) string {
	if !prof.Detected {
		return ""
	}
	if redact {
		return fmt.Sprintf("%s (%s)", prof.Vendor, privacy.MaskIP(prof.GatewayIP))
	}
	return fmt.Sprintf("%s (%s)", prof.Vendor, prof.GatewayIP)
}
