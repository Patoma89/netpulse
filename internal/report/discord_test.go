package report

import (
	"strings"
	"testing"

	"netpulse/internal/diagnostics"
	"netpulse/internal/netinfo"
	"netpulse/internal/optimizer"
	"netpulse/internal/radar"
	"netpulse/internal/router"
	"netpulse/internal/wifi"
)

func TestBuildDiscordReportRedactsAllSensitiveData(t *testing.T) {
	in := ReportInput{
		Version:     "1.0.0",
		ForceRedact: true,
		Network: netinfo.NetworkSummary{
			Connected:     true,
			LinkType:      "wifi",
			LinkSpeedMbps: 866,
			LocalIP:       "192.168.1.42",
			GatewayIP:     "192.168.1.1",
			AdapterMAC:    "A4:B1:C2:D3:E4:F5",
		},
		DualHop: diagnostics.DualHopSnapshot{
			RouterTarget:   "192.168.1.1",
			InternetTarget: "1.1.1.1",
			Router: diagnostics.HopStats{
				Sent:     20,
				AvgMs:    2.1,
				P95Ms:    3.4,
				JitterMs: 0.5,
				LossPct:  0,
			},
			Internet: diagnostics.HopStats{
				Sent:     20,
				AvgMs:    18.4,
				P95Ms:    23.0,
				JitterMs: 1.4,
				LossPct:  0,
			},
			Verdict: diagnostics.Verdict{
				Code:     "optimal",
				Severity: "emerald",
				Title:    "Red Estable y Óptima",
				Score:    96,
			},
		},
		Bufferbloat: &diagnostics.BufferbloatResult{
			Grade:           "A",
			IdleMedianMs:    17.5,
			DownloadDeltaMs: 12.0,
			DownloadMbps:    310.5,
			UploadDeltaMs:   18.2,
			UploadMbps:      48.0,
		},
		Radar: radar.ScanResult{
			CompletedAt:  "2026-11-25T13:00:00Z",
			TotalDevices: 8,
			TrustedCount: 7,
			UnknownCount: 1,
			DurationMs:   1420,
			Devices: []radar.Device{
				{
					IP:       "192.168.1.42",
					MAC:      "A4:B1:C2:D3:E4:F5",
					Hostname: "PC-Privada-Juan",
					Alias:    "MiPCDeCasa",
				},
			},
		},
		WiFi: wifi.ScanResult{
			Status:          "ok",
			Connected:       true,
			CurrentSSID:     "MiWifiSecreta5G",
			CurrentBSSID:    "C0:25:E9:11:22:33",
			CurrentBand:     "5GHz",
			CurrentChannel:  36,
			CurrentWidthMHz: 80,
			CurrentRSSI:     -52,
			AccessPoints:    []wifi.AccessPoint{{SSID: "Vecino"}, {SSID: "MiWifiSecreta5G"}},
			Rec5GHz: wifi.BandRecommendation{
				Band:               "5GHz",
				RecommendedChannel: 149,
				BestCongestion:     4,
			},
		},
		DNS: optimizer.DNSBenchmarkResult{
			CompletedAt: "2026-11-25T13:00:00Z",
			Results: []optimizer.DNSResult{
				{
					Provider:  optimizer.DNSProvider{ID: "cloudflare", Name: "Cloudflare", Primary: "1.1.1.1"},
					MedianMs:  5.4,
					JitterMs:  0.8,
					Reachable: true,
				},
			},
		},
		UPnP: optimizer.UPnPScanResult{
			Status: "upnp_disabled",
		},
		Router: router.RouterProfile{
			Detected:  true,
			Vendor:    "TP-Link",
			GatewayIP: "192.168.1.1",
		},
	}

	rep := BuildDiscordReport(in)
	if !rep.Redacted || rep.HealthScore != 96 || rep.BufferbloatGrade != "A" {
		t.Fatalf("unexpected report summary: %+v", rep)
	}
	if len(rep.RedactedFields) != 5 {
		t.Fatalf("expected 5 redacted field descriptions, got %d", len(rep.RedactedFields))
	}

	forbidden := []string{
		"192.168.1.42",
		"192.168.1.1",
		"A4:B1:C2:D3:E4:F5",
		"C0:25:E9:11:22:33",
		"MiWifiSecreta5G",
		"PC-Privada-Juan",
		"MiPCDeCasa",
	}
	for _, secret := range forbidden {
		if strings.Contains(rep.Markdown, secret) || strings.Contains(rep.PlainText, secret) {
			t.Fatalf("sensitive string %q leaked into Discord report:\n%s", secret, rep.Markdown)
		}
	}

	expected := []string{
		"96/100",
		"Red Estable y Óptima",
		"Nota A",
		"Ch 149",
		"8 equipos detectados",
		"Cloudflare",
		"UPnP desactivado",
		"0% telemetría",
	}
	for _, want := range expected {
		if !strings.Contains(rep.Markdown, want) {
			t.Fatalf("expected %q in report Markdown:\n%s", want, rep.Markdown)
		}
	}
}

func TestBuildDiscordReportDefaultsWhenEmpty(t *testing.T) {
	rep := BuildDiscordReport(ReportInput{ForceRedact: true})
	if rep.BufferbloatGrade != "No ejecutado" {
		t.Fatalf("expected 'No ejecutado', got %q", rep.BufferbloatGrade)
	}
	if !strings.Contains(rep.Markdown, "No escaneado aún") {
		t.Fatalf("expected graceful empty summaries, got:\n%s", rep.Markdown)
	}
}
