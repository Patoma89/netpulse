package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"netpulse/internal/autostart"
	"netpulse/internal/config"
	"netpulse/internal/diagnostics"
	"netpulse/internal/history"
	"netpulse/internal/netinfo"
	"netpulse/internal/optimizer"
	"netpulse/internal/ping"
	"netpulse/internal/privacy"
	"netpulse/internal/processes"
	"netpulse/internal/radar"
	"netpulse/internal/report"
	"netpulse/internal/router"
	"netpulse/internal/tray"
	"netpulse/internal/updater"
	"netpulse/internal/vault"
	"netpulse/internal/wifi"
)

const appVersion = "1.2.0"

// AppInfo describes static runtime metadata and local security status.
type AppInfo struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	ConfigPath    string `json:"configPath"`
	ZeroTelemetry bool   `json:"zeroTelemetry"`
	VaultReady    bool   `json:"vaultReady"`
	VaultError    string `json:"vaultError"`
}

// RedactedPreview holds both raw and Discord-safe masked telemetry samples.
type RedactedPreview struct {
	IP   string `json:"ip"`
	MAC  string `json:"mac"`
	SSID string `json:"ssid"`
}

// App is the main Wails backend bound to the React frontend.
type App struct {
	ctx             context.Context
	store           *config.Store
	vault           *vault.Vault
	configPath      string
	pinger            ping.Pinger
	monitor           *diagnostics.DualHopMonitor
	bloat             *diagnostics.BufferbloatRunner
	regionBench       *diagnostics.RegionBenchmarker
	mtr               *diagnostics.MTRMonitor
	history           *history.Store
	procMon           *processes.Monitor
	auditMu           sync.RWMutex
	lastAudit         diagnostics.NetworkAuditResult
	radarMu           sync.RWMutex
	radarScanner      *radar.Scanner
	lastRadar         radar.ScanResult
	radarMACByKey     map[string]string
	lastAlertedMACs   map[string]bool
	activeProfileID   string
	activeProfileName string
	pendingGatewayMAC string
	wifiMu            sync.RWMutex
	lastWifi        wifi.ScanResult
	optMu           sync.RWMutex
	dnsBench        *optimizer.DNSBenchmarker
	lastDNS         optimizer.DNSBenchmarkResult
	lastUPnP        optimizer.UPnPScanResult
	routerMu        sync.RWMutex
	routerProfile   router.RouterProfile
	routerBaseline  *router.BeforeAfterSnapshot
	routerAfter     *router.BeforeAfterSnapshot
	autoPilot       *router.AutoPilotManager
	trayMgr         *tray.Manager
	watcherCancel   context.CancelFunc
	quitting        atomic.Bool
	lastNetSig      string
}

// NewApp initializes the local config store, Windows Credential Manager vault, and network engines.
func NewApp() *App {
	store, err := config.NewStore()
	cfgPath := ""
	if err == nil {
		cfgPath = store.Path()
	}

	v := vault.New()
	pinger := ping.NewWinPinger()
	monitor := diagnostics.NewDualHopMonitor(pinger, 0, nil)
	bloat := diagnostics.NewBufferbloatRunner(pinger)
	histStore, _ := history.NewStore()

	return &App{
		store:           store,
		vault:           v,
		configPath:      cfgPath,
		pinger:          pinger,
		monitor:         monitor,
		bloat:           bloat,
		regionBench:     diagnostics.NewRegionBenchmarker(),
		mtr:             diagnostics.NewMTRMonitor(nil),
		history:         histStore,
		procMon:         processes.NewMonitor(),
		radarScanner:    radar.NewScanner(),
		lastRadar:       radar.ScanResult{Devices: []radar.Device{}},
		radarMACByKey:   make(map[string]string),
		lastAlertedMACs: make(map[string]bool),
		lastWifi: wifi.ScanResult{
			AccessPoints: []wifi.AccessPoint{},
			Scores24GHz:  []wifi.ChannelScore{},
			Scores5GHz:   []wifi.ChannelScore{},
			Scores6GHz:   []wifi.ChannelScore{},
		},
		dnsBench:  optimizer.NewDNSBenchmarker(),
		lastDNS:   optimizer.DNSBenchmarkResult{CurrentDNS: []string{}, Results: []optimizer.DNSResult{}},
		lastUPnP:  optimizer.UPnPScanResult{Mappings: []optimizer.UPnPMapping{}},
		autoPilot: router.NewAutoPilotManager(v, pinger, store),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.monitor.SetOnUpdate(func(snap diagnostics.DualHopSnapshot) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "dualhop:sample", a.redactDualHopSnapshot(snap))
		}
	})
	a.monitor.SetOnIncident(func(inc diagnostics.StabilityIncident) {
		if a.history == nil {
			return
		}
		a.radarMu.RLock()
		netName := a.activeProfileName
		a.radarMu.RUnlock()
		if netName == "" {
			netName = "Red Principal"
		}
		_ = a.history.AppendIncident(history.IncidentRecord{
			Timestamp:   inc.Timestamp,
			NetworkName: netName,
			Hop:         inc.Hop,
			Kind:        inc.Kind,
			Severity:    inc.Severity,
			DurationMs:  inc.DurationMs,
			PeakMs:      inc.PeakMs,
			Message:     inc.Message,
		})
	})
	if a.mtr != nil {
		a.mtr.SetOnUpdate(func(snap diagnostics.MTRSnapshot) {
			if a.ctx != nil {
				wailsruntime.EventsEmit(a.ctx, "mtr:update", a.redactMTRSnapshot(snap))
			}
		})
	}

	summary, err := netinfo.GetNetworkSummary()
	if err == nil {
		a.lastNetSig = netSignature(summary)
		if summary.Connected && summary.GatewayIP != "" {
			target := a.GetConfig().PingTarget
			if target == "" {
				target = "1.1.1.1"
			}
			a.monitor.Start(summary.GatewayIP, target)
		}
	}

	a.trayMgr = tray.New(tray.Callbacks{
		OnOpen: func() {
			if a.ctx != nil {
				wailsruntime.WindowUnminimise(a.ctx)
				wailsruntime.WindowShow(a.ctx)
			}
		},
		OnScan: func() {
			res, scanErr := a.ScanLAN()
			if scanErr == nil && a.trayMgr != nil {
				a.trayMgr.Notify(
					"NetPulse · Radar LAN",
					fmt.Sprintf("Escaneo completo: %d equipos (%d sin confirmar).", res.TotalDevices, res.UnknownCount),
				)
			}
		},
		OnQuit: func() {
			a.quitting.Store(true)
			if a.ctx != nil {
				wailsruntime.Quit(a.ctx)
			}
		},
	})
	a.trayMgr.Start()

	watchCtx, cancel := context.WithCancel(ctx)
	a.watcherCancel = cancel
	go a.backgroundWatchers(watchCtx)
}

func (a *App) beforeClose(ctx context.Context) bool {
	if !a.quitting.Load() && a.GetConfig().MinimizeToTray {
		wailsruntime.WindowHide(ctx)
		return true
	}
	return false
}

func (a *App) onSecondInstance(_ options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
}

func (a *App) shutdown(ctx context.Context) {
	if a.watcherCancel != nil {
		a.watcherCancel()
	}
	if a.trayMgr != nil {
		a.trayMgr.Stop()
	}
	if a.monitor != nil {
		a.monitor.Stop()
	}
	if a.bloat != nil {
		a.bloat.Cancel()
	}
	if a.mtr != nil {
		a.mtr.Stop()
	}
}

func (a *App) backgroundWatchers(ctx context.Context) {
	netTicker := time.NewTicker(4 * time.Second)
	radarTicker := time.NewTicker(5 * time.Minute)
	defer netTicker.Stop()
	defer radarTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-netTicker.C:
			summary, err := netinfo.GetNetworkSummary()
			if err != nil {
				continue
			}
			sig := netSignature(summary)
			if sig != a.lastNetSig {
				a.lastNetSig = sig
				if a.monitor.IsRunning() {
					if summary.Connected && summary.GatewayIP != "" {
						target := a.GetConfig().PingTarget
						if target == "" {
							target = "1.1.1.1"
						}
						a.monitor.Start(summary.GatewayIP, target)
					} else {
						a.monitor.Stop()
					}
				}
				if a.ctx != nil {
					wailsruntime.EventsEmit(a.ctx, "network:changed", a.redactNetworkSummary(summary))
				}
			}
		case <-radarTicker.C:
			if !a.GetConfig().BackgroundRadarAlert {
				continue
			}
			res, err := a.ScanLAN()
			if err != nil {
				continue
			}
			for _, dev := range res.Devices {
				if dev.IsNew && !dev.Trusted && !a.lastAlertedMACs[dev.Key] {
					a.lastAlertedMACs[dev.Key] = true
					if a.trayMgr != nil {
						a.trayMgr.Notify(
							"NetPulse: Dispositivo nuevo detectado",
							fmt.Sprintf("Se conectó %s (%s) a tu red local.", dev.Vendor, dev.Category),
						)
					}
					if a.ctx != nil {
						wailsruntime.EventsEmit(a.ctx, "radar:intruder", dev)
					}
				}
			}
		}
	}
}

func netSignature(s netinfo.NetworkSummary) string {
	return fmt.Sprintf("%v|%s|%s|%s", s.Connected, s.GatewayIP, s.LocalIP, s.LinkType)
}

// GetAppInfo returns runtime environment and local-first security verification details.
func (a *App) GetAppInfo() AppInfo {
	vaultErr := a.vault.HealthCheck()
	vaultErrMsg := ""
	if vaultErr != nil {
		vaultErrMsg = vaultErr.Error()
	}

	return AppInfo{
		Name:          "NetPulse",
		Version:       appVersion,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		ConfigPath:    a.configPath,
		ZeroTelemetry: true,
		VaultReady:    vaultErr == nil,
		VaultError:    vaultErrMsg,
	}
}

// GetConfig returns the current local preferences.
func (a *App) GetConfig() config.AppConfig {
	if a.store == nil {
		return config.DefaultConfig()
	}
	return a.store.Get()
}

// SetPrivacyMode toggles Discord Privacy Mode and persists it locally.
func (a *App) SetPrivacyMode(enabled bool) (config.AppConfig, error) {
	var cfg config.AppConfig
	var err error
	if a.store == nil {
		cfg = config.DefaultConfig()
		cfg.PrivacyMode = enabled
	} else {
		cfg, err = a.store.SetPrivacyMode(enabled)
		if err != nil {
			return cfg, err
		}
	}

	if a.ctx != nil && a.monitor != nil {
		wailsruntime.EventsEmit(a.ctx, "dualhop:sample", a.redactDualHopSnapshot(a.monitor.Snapshot()))
	}
	return cfg, nil
}

// SetTrayPreferences updates Windows tray minimization and background intruder alerts.
func (a *App) SetTrayPreferences(minimizeToTray, backgroundRadarAlert bool) (config.AppConfig, error) {
	if a.store == nil {
		cfg := config.DefaultConfig()
		cfg.MinimizeToTray = minimizeToTray
		cfg.BackgroundRadarAlert = backgroundRadarAlert
		return cfg, nil
	}
	return a.store.SetTrayPreferences(minimizeToTray, backgroundRadarAlert)
}

// SetUIPreferences persists user language ("es" | "en") and UI theme ("dark" | "light").
func (a *App) SetUIPreferences(language, theme string) (config.AppConfig, error) {
	if a.store == nil {
		cfg := config.DefaultConfig()
		cfg.Language = language
		cfg.Theme = theme
		return cfg, nil
	}
	return a.store.SetUIPreferences(language, theme)
}

// SetStartWithWindows toggles automatic minimized startup in HKCU\...\Run without requiring Admin rights.
func (a *App) SetStartWithWindows(enabled bool) (config.AppConfig, error) {
	if err := autostart.SetEnabled(enabled); err != nil {
		return a.GetConfig(), err
	}
	if a.store == nil {
		cfg := config.DefaultConfig()
		cfg.StartWithWindows = enabled
		return cfg, nil
	}
	return a.store.SetAutostartPreference(enabled)
}

// GetStartWithWindows reports whether NetPulse currently has a Windows autostart entry registered.
func (a *App) GetStartWithWindows() bool {
	return autostart.IsEnabled()
}

// CheckForUpdates runs a single opt-in GitHub Releases check ONLY when invoked explicitly by the user.
func (a *App) CheckForUpdates(repo string) updater.UpdateCheckResult {
	targetRepo := strings.TrimSpace(repo)
	if targetRepo == "" {
		targetRepo = a.GetConfig().UpdateRepo
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	return updater.CheckLatestRelease(ctx, appVersion, targetRepo, "")
}

// PreviewRedaction applies Go-side Discord privacy masking when PrivacyMode is enabled.
func (a *App) PreviewRedaction(ip, mac, ssid string) RedactedPreview {
	if a.GetConfig().PrivacyMode {
		return RedactedPreview{
			IP:   privacy.MaskIP(ip),
			MAC:  privacy.MaskMAC(mac),
			SSID: privacy.MaskSSID(ssid),
		}
	}
	return RedactedPreview{
		IP:   ip,
		MAC:  mac,
		SSID: ssid,
	}
}

// GetNetworkSummary returns detected Windows network adapters and primary gateway details.
func (a *App) GetNetworkSummary() netinfo.NetworkSummary {
	summary, err := netinfo.GetNetworkSummary()
	if err != nil {
		return netinfo.NetworkSummary{Adapters: []netinfo.AdapterInfo{}}
	}
	return a.redactNetworkSummary(summary)
}

// GetDualHopSnapshot returns the current Dual-Hop telemetry snapshot, respecting PrivacyMode.
func (a *App) GetDualHopSnapshot() diagnostics.DualHopSnapshot {
	if a.monitor == nil {
		return diagnostics.DualHopSnapshot{}
	}
	return a.redactDualHopSnapshot(a.monitor.Snapshot())
}

// StartDualHop refreshes the local gateway, persists the internet target, and starts monitoring.
func (a *App) StartDualHop(target string) (diagnostics.DualHopSnapshot, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return diagnostics.DualHopSnapshot{}, errors.New("el destino de ping no puede estar vacío")
	}

	if a.store != nil {
		if _, err := a.store.SetPingTarget(trimmed); err != nil {
			return diagnostics.DualHopSnapshot{}, err
		}
	}

	summary, err := netinfo.GetNetworkSummary()
	if err != nil {
		return diagnostics.DualHopSnapshot{}, err
	}

	gatewayIP := summary.GatewayIP
	a.monitor.Start(gatewayIP, trimmed)
	return a.redactDualHopSnapshot(a.monitor.Snapshot()), nil
}

// StopDualHop pauses the Dual-Hop probe loop.
func (a *App) StopDualHop() diagnostics.DualHopSnapshot {
	if a.monitor == nil {
		return diagnostics.DualHopSnapshot{}
	}
	a.monitor.Stop()
	return a.redactDualHopSnapshot(a.monitor.Snapshot())
}

// ResetDualHop clears accumulated Dual-Hop samples and incidents and returns the reset snapshot.
func (a *App) ResetDualHop() diagnostics.DualHopSnapshot {
	if a.monitor == nil {
		return diagnostics.DualHopSnapshot{}
	}
	return a.redactDualHopSnapshot(a.monitor.Reset())
}

// RunBufferbloatTest runs the Idle -> Download -> Upload latency-under-load test.
func (a *App) RunBufferbloatTest() (diagnostics.BufferbloatResult, error) {
	if a.bloat == nil {
		return diagnostics.BufferbloatResult{}, errors.New("motor de bufferbloat no inicializado")
	}

	target := a.GetConfig().PingTarget
	if target == "" {
		target = "1.1.1.1"
	}

	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	return a.bloat.Run(parentCtx, target, func(p diagnostics.BufferbloatProgress) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "bufferbloat:progress", p)
		}
	})
}

// CancelBufferbloatTest cancels an active Bufferbloat test if running.
func (a *App) CancelBufferbloatTest() {
	if a.bloat != nil {
		a.bloat.Cancel()
	}
}

// GetLastBufferbloatResult returns the latest completed Bufferbloat test result, if available.
func (a *App) GetLastBufferbloatResult() *diagnostics.BufferbloatResult {
	if a.bloat == nil {
		return nil
	}
	return a.bloat.LastResult()
}

// RunRegionBenchmark measures latency & jitter to major gaming, voice, and cloud regions.
func (a *App) RunRegionBenchmark() diagnostics.RegionBenchmarkResult {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.regionBench.Run(ctx)
}

// RunTraceroute runs a hop-by-hop ICMP traceroute without Admin privileges, respecting PrivacyMode.
func (a *App) RunTraceroute(target string) diagnostics.TracerouteResult {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(target) == "" {
		target = a.GetConfig().PingTarget
	}
	res := diagnostics.RunTraceroute(ctx, target)

	if a.GetConfig().PrivacyMode {
		for i := range res.Hops {
			if res.Hops[i].TimedOut || res.Hops[i].IP == "*" {
				continue
			}
			parsed := net.ParseIP(res.Hops[i].IP)
			if res.Hops[i].TTL <= 2 || (parsed != nil && parsed.IsPrivate()) {
				res.Hops[i].IP = privacy.MaskIP(res.Hops[i].IP)
			}
		}
	}
	return res
}

// StartMTR starts continuous WinMTR-style hop probing toward target.
func (a *App) StartMTR(target string) error {
	if a.mtr == nil {
		return errors.New("motor MTR no inicializado")
	}
	if strings.TrimSpace(target) == "" {
		target = a.GetConfig().PingTarget
	}
	return a.mtr.Start(target, 15)
}

// StopMTR stops continuous MTR probing.
func (a *App) StopMTR() diagnostics.MTRSnapshot {
	if a.mtr == nil {
		return diagnostics.MTRSnapshot{}
	}
	a.mtr.Stop()
	return a.redactMTRSnapshot(a.mtr.Snapshot())
}

// GetMTRSnapshot returns the current continuous MTR snapshot, respecting PrivacyMode.
func (a *App) GetMTRSnapshot() diagnostics.MTRSnapshot {
	if a.mtr == nil {
		return diagnostics.MTRSnapshot{}
	}
	return a.redactMTRSnapshot(a.mtr.Snapshot())
}

// RunNetworkAudit performs CGNAT / Double-NAT detection, Path MTU discovery, and IPv6 check concurrently.
func (a *App) RunNetworkAudit() diagnostics.NetworkAuditResult {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	auditCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	res := diagnostics.RunNetworkAudit(auditCtx)
	a.auditMu.Lock()
	a.lastAudit = res
	a.auditMu.Unlock()
	return a.redactNetworkAudit(res)
}

// GetLastNetworkAudit returns the latest cached NAT/MTU/IPv6 network audit, respecting PrivacyMode.
func (a *App) GetLastNetworkAudit() diagnostics.NetworkAuditResult {
	a.auditMu.RLock()
	res := a.lastAudit
	a.auditMu.RUnlock()
	return a.redactNetworkAudit(res)
}

// GetIncidentHistory returns stored stability incidents over the last N days (up to 30 days).
func (a *App) GetIncidentHistory(days int) history.HistorySnapshot {
	if a.history == nil {
		return history.HistorySnapshot{
			RetentionDays: history.MaxRetentionDays,
			Incidents:     []history.IncidentRecord{},
			Daily:         []history.DailySummary{},
		}
	}
	return a.redactHistorySnapshot(a.history.Snapshot(days))
}

// ClearIncidentHistory clears all persisted incident history entries.
func (a *App) ClearIncidentHistory() error {
	if a.history == nil {
		return nil
	}
	return a.history.Clear()
}

// SaveISPReportFile opens a native Windows Save dialog to export 30-day stability history as CSV or HTML.
func (a *App) SaveISPReportFile(format string, days int) (string, error) {
	if a.ctx == nil {
		return "", errors.New("ventana principal no inicializada")
	}
	var snap history.HistorySnapshot
	if a.history != nil {
		snap = a.history.Snapshot(days)
	}
	a.radarMu.RLock()
	netLabel := a.activeProfileName
	a.radarMu.RUnlock()
	if netLabel == "" {
		netLabel = "Red Principal"
	}
	redact := a.GetConfig().PrivacyMode

	ext := "html"
	defaultName := "netpulse-evidencia-isp.html"
	filters := []wailsruntime.FileFilter{{DisplayName: "Informe HTML (*.html)", Pattern: "*.html"}}
	var content string
	if strings.EqualFold(strings.TrimSpace(format), "csv") {
		ext = "csv"
		defaultName = "netpulse-evidencia-isp.csv"
		filters = []wailsruntime.FileFilter{{DisplayName: "CSV Excel (*.csv)", Pattern: "*.csv"}}
		content = history.BuildISPReportCSV(snap, netLabel, redact)
	} else {
		content = history.BuildISPReportHTML(snap, netLabel, appVersion, redact)
	}

	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           fmt.Sprintf("Exportar Evidencia ISP (.%s)", ext),
		DefaultFilename: defaultName,
		Filters:         filters,
	})
	if err != nil {
		return "", err
	}
	if savePath == "" {
		return "", nil
	}
	if err := os.WriteFile(savePath, []byte(content), 0o600); err != nil {
		return "", err
	}
	return savePath, nil
}

// GetProcessConnections captures active per-process TCP/UDP sockets and I/O rate via Win32 IP Helper without Admin.
func (a *App) GetProcessConnections() (processes.ProcessSnapshot, error) {
	if a.procMon == nil {
		return processes.ProcessSnapshot{Processes: []processes.ProcessNetUsage{}}, nil
	}
	snap, err := a.procMon.Snapshot()
	if err != nil {
		return processes.ProcessSnapshot{}, err
	}
	return a.redactProcessSnapshot(snap), nil
}

// ScanLAN executes a concurrent ARP + mDNS + SSDP + NetBIOS LAN radar scan.
func (a *App) ScanLAN() (radar.ScanResult, error) {
	if a.radarScanner == nil {
		return radar.ScanResult{}, errors.New("motor de radar LAN no inicializado")
	}

	summary, err := netinfo.GetNetworkSummary()
	if err != nil {
		return radar.ScanResult{}, err
	}

	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	cfg := a.GetConfig()
	hadProfilesBefore := len(cfg.NetworkProfiles) > 0
	res, macByKey, err := a.radarScanner.Scan(parentCtx, summary, cfg.KnownDevices, func(p radar.ScanProgress) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "radar:progress", p)
		}
	})
	if err != nil {
		return radar.ScanResult{}, err
	}

	rawProfileID, defaultProfName := a.resolveActiveProfileMeta(summary, res.Subnet)
	var gatewayMAC string
	onlineMACs := make([]string, 0, len(res.Devices))
	migratedTrusted := make([]string, 0, len(res.Devices))
	for _, d := range res.Devices {
		rawMAC := macByKey[d.Key]
		if rawMAC == "" {
			continue
		}
		onlineMACs = append(onlineMACs, rawMAC)
		if d.IsGateway {
			gatewayMAC = rawMAC
		}
		if d.Trusted && !d.IsLocal {
			migratedTrusted = append(migratedTrusted, rawMAC)
		}
	}

	var (
		prof          config.NetworkProfile
		gwChanged     bool
		prevGW        string
		presenceError error
	)
	if a.store != nil {
		prof, gwChanged, prevGW, presenceError = a.store.RecordNetworkPresence(
			rawProfileID,
			defaultProfName,
			summary.GatewayIP,
			gatewayMAC,
			res.Subnet,
			onlineMACs,
		)
		if presenceError == nil && !hadProfilesBefore && len(migratedTrusted) > 0 {
			_ = a.store.TrustAllForProfile(prof.ID, migratedTrusted)
			prof = a.store.Get().NetworkProfiles[prof.ID]
		}
	}

	res.ProfileID = opaqueProfileID(prof.ID)
	res.ProfileName = prof.Name
	if res.ProfileName == "" {
		res.ProfileName = defaultProfName
	}

	updatedCfg := a.GetConfig()
	for i := range res.Devices {
		rawMAC := macByKey[res.Devices[i].Key]
		if kd, ok := updatedCfg.KnownDevices[rawMAC]; ok {
			res.Devices[i].LastSeen = kd.LastSeen
			res.Devices[i].SeenCount = kd.SeenCount
			res.Devices[i].Online = kd.Online
			res.Devices[i].LastTransition = kd.LastTransition
		}
		if prof.ID != "" && !res.Devices[i].IsLocal {
			res.Devices[i].Trusted = prof.TrustedMACs[rawMAC]
		}
	}
	recomputeRadarCounts(&res)

	var alertPtr *radar.GatewayMACAlert
	if gwChanged {
		alertPtr = &radar.GatewayMACAlert{
			Active:      true,
			ProfileID:   res.ProfileID,
			PreviousMAC: prevGW,
			CurrentMAC:  gatewayMAC,
			DetectedAt:  time.Now().Format("15:04:05"),
		}
		res.GatewayMACAlert = alertPtr
	}

	a.radarMu.Lock()
	a.lastRadar = res
	a.radarMACByKey = macByKey
	a.activeProfileID = prof.ID
	a.activeProfileName = res.ProfileName
	if gwChanged {
		a.pendingGatewayMAC = gatewayMAC
	} else {
		a.pendingGatewayMAC = ""
	}
	a.radarMu.Unlock()

	redacted := a.redactRadarResult(res)
	if gwChanged && a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "radar:gateway-alert", redacted.GatewayMACAlert)
		if a.trayMgr != nil {
			a.trayMgr.Notify(
				"NetPulse: Alerta MAC de Gateway",
				"Cambió la dirección MAC del router en esta red. Revisá Radar LAN.",
			)
		}
	}

	return redacted, nil
}

// GetLastRadarScan returns the most recent LAN radar scan result, respecting PrivacyMode.
func (a *App) GetLastRadarScan() radar.ScanResult {
	a.radarMu.RLock()
	res := a.lastRadar
	a.radarMu.RUnlock()
	return a.redactRadarResult(res)
}

// UpdateRadarDevice saves an alias, category, and trusted status for a discovered LAN device.
func (a *App) UpdateRadarDevice(deviceKey, alias, category string, trusted bool) (radar.ScanResult, error) {
	a.radarMu.Lock()
	defer a.radarMu.Unlock()

	rawMAC, ok := a.radarMACByKey[deviceKey]
	if !ok || rawMAC == "" {
		return a.redactRadarResultLocked(a.lastRadar), errors.New("dispositivo no encontrado en el último escaneo")
	}

	cleanCategory := strings.TrimSpace(category)
	if cleanCategory == "" {
		cleanCategory = "unknown"
	}

	var firstSeen string
	for _, d := range a.lastRadar.Devices {
		if d.Key == deviceKey {
			firstSeen = d.FirstSeen
			break
		}
	}

	if a.store != nil {
		if _, err := a.store.UpsertKnownDevice(rawMAC, config.KnownDevice{
			Alias:     strings.TrimSpace(alias),
			Category:  cleanCategory,
			Trusted:   trusted,
			FirstSeen: firstSeen,
		}); err != nil {
			return a.redactRadarResultLocked(a.lastRadar), err
		}
		if a.activeProfileID != "" {
			_ = a.store.SetDeviceTrustedForProfile(a.activeProfileID, rawMAC, trusted)
		}
	}

	for i := range a.lastRadar.Devices {
		if a.lastRadar.Devices[i].Key == deviceKey {
			a.lastRadar.Devices[i].Alias = strings.TrimSpace(alias)
			a.lastRadar.Devices[i].Category = cleanCategory
			a.lastRadar.Devices[i].Trusted = trusted
			a.lastRadar.Devices[i].IsNew = false
			break
		}
	}

	recomputeRadarCounts(&a.lastRadar)
	return a.redactRadarResultLocked(a.lastRadar), nil
}

// TrustAllCurrentDevices marks every device in the latest radar scan as trusted in local config.
func (a *App) TrustAllCurrentDevices() (radar.ScanResult, error) {
	a.radarMu.Lock()
	defer a.radarMu.Unlock()

	if len(a.lastRadar.Devices) == 0 {
		return a.redactRadarResultLocked(a.lastRadar), nil
	}

	batch := make(map[string]config.KnownDevice, len(a.lastRadar.Devices))
	for _, d := range a.lastRadar.Devices {
		rawMAC := a.radarMACByKey[d.Key]
		if rawMAC == "" {
			continue
		}
		batch[rawMAC] = config.KnownDevice{
			Alias:     d.Alias,
			Category:  d.Category,
			Trusted:   true,
			FirstSeen: d.FirstSeen,
		}
	}

	if a.store != nil && len(batch) > 0 {
		if _, err := a.store.TrustDevices(batch); err != nil {
			return a.redactRadarResultLocked(a.lastRadar), err
		}
		if a.activeProfileID != "" {
			macList := make([]string, 0, len(batch))
			for m := range batch {
				macList = append(macList, m)
			}
			_ = a.store.TrustAllForProfile(a.activeProfileID, macList)
		}
	}

	for i := range a.lastRadar.Devices {
		a.lastRadar.Devices[i].Trusted = true
		a.lastRadar.Devices[i].IsNew = false
	}

	recomputeRadarCounts(&a.lastRadar)
	return a.redactRadarResultLocked(a.lastRadar), nil
}

// ConfirmGatewayMACChange updates the trusted baseline MAC of the current router after user verification.
func (a *App) ConfirmGatewayMACChange() (radar.ScanResult, error) {
	a.radarMu.Lock()
	defer a.radarMu.Unlock()

	if a.store == nil || a.activeProfileID == "" || a.pendingGatewayMAC == "" {
		return a.redactRadarResultLocked(a.lastRadar), nil
	}
	if err := a.store.ConfirmGatewayMAC(a.activeProfileID, a.pendingGatewayMAC); err != nil {
		return a.redactRadarResultLocked(a.lastRadar), err
	}
	a.pendingGatewayMAC = ""
	a.lastRadar.GatewayMACAlert = nil
	return a.redactRadarResultLocked(a.lastRadar), nil
}

// RenameActiveNetworkProfile renames the active Radar LAN profile (e.g., "Casa", "Oficina").
func (a *App) RenameActiveNetworkProfile(name string) (radar.ScanResult, error) {
	a.radarMu.Lock()
	defer a.radarMu.Unlock()

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return a.redactRadarResultLocked(a.lastRadar), errors.New("el nombre del perfil no puede estar vacío")
	}
	if a.store != nil && a.activeProfileID != "" {
		if _, err := a.store.RenameNetworkProfile(a.activeProfileID, trimmed); err != nil {
			return a.redactRadarResultLocked(a.lastRadar), err
		}
	}
	a.activeProfileName = trimmed
	a.lastRadar.ProfileName = trimmed
	return a.redactRadarResultLocked(a.lastRadar), nil
}

func (a *App) resolveActiveProfileMeta(summary netinfo.NetworkSummary, subnet string) (string, string) {
	if summary.LinkType == "wifi" {
		a.wifiMu.RLock()
		ssid := strings.TrimSpace(a.lastWifi.CurrentSSID)
		a.wifiMu.RUnlock()
		if ssid != "" {
			return "wifi:" + ssid + "/" + summary.GatewayIP, "Wi-Fi Principal"
		}
		return "gw:" + summary.GatewayIP + "/" + subnet, "Red Wi-Fi"
	}
	if summary.LinkType == "ethernet" {
		return "gw:" + summary.GatewayIP + "/" + subnet, "Red Ethernet"
	}
	return "gw:" + summary.GatewayIP + "/" + subnet, "Red Local"
}

func opaqueProfileID(rawID string) string {
	trimmed := strings.TrimSpace(rawID)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("netpulse-profile:" + trimmed))
	return "prof-" + hex.EncodeToString(sum[:6])
}

// InspectRadarDeviceServices checks 9 standard LAN ports on a single discovered device.
func (a *App) InspectRadarDeviceServices(deviceKey string) (radar.DeviceServicesResult, error) {
	a.radarMu.RLock()
	var rawIP string
	for _, d := range a.lastRadar.Devices {
		if d.Key == deviceKey {
			rawIP = d.IP
			break
		}
	}
	a.radarMu.RUnlock()

	if rawIP == "" {
		return radar.DeviceServicesResult{}, errors.New("dispositivo no encontrado en el último escaneo")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	res := radar.InspectDeviceServices(ctx, deviceKey, rawIP)
	if a.GetConfig().PrivacyMode {
		res.IP = privacy.MaskIP(res.IP)
	}
	return res, nil
}

// WakeRadarDevice sends a Wake-on-LAN Magic Packet to a discovered LAN device's MAC address.
func (a *App) WakeRadarDevice(deviceKey string) error {
	a.radarMu.RLock()
	rawMAC, ok := a.radarMACByKey[deviceKey]
	a.radarMu.RUnlock()
	if !ok || rawMAC == "" {
		return errors.New("MAC del dispositivo no disponible en el último escaneo")
	}

	summary, _ := netinfo.GetNetworkSummary()
	var prefixLen uint8 = 24
	if summary.Primary != nil && summary.Primary.PrefixLen > 0 {
		prefixLen = summary.Primary.PrefixLen
	}
	return radar.SendWakeOnLAN(rawMAC, summary.LocalIP, prefixLen)
}

func recomputeRadarCounts(res *radar.ScanResult) {
	trusted := 0
	unknown := 0
	newCount := 0
	for _, d := range res.Devices {
		if d.Trusted {
			trusted++
		} else {
			unknown++
		}
		if d.IsNew {
			newCount++
		}
	}
	res.TotalDevices = len(res.Devices)
	res.TrustedCount = trusted
	res.UnknownCount = unknown
	res.NewCount = newCount
	radar.SortDevices(res.Devices)
}

// ScanWiFi scans nearby Wi-Fi networks via Win32 wlanapi.dll and computes channel recommendations.
func (a *App) ScanWiFi(activeScan bool) wifi.ScanResult {
	res := wifi.ScanWiFi(activeScan)

	a.wifiMu.Lock()
	a.lastWifi = res
	a.wifiMu.Unlock()

	return a.redactWiFiResult(res)
}

// GetLastWiFiScan returns the most recent cached Wi-Fi spectrum scan, respecting PrivacyMode.
func (a *App) GetLastWiFiScan() wifi.ScanResult {
	a.wifiMu.RLock()
	res := a.lastWifi
	a.wifiMu.RUnlock()
	return a.redactWiFiResult(res)
}

// OpenWindowsLocationSettings opens Windows Location Privacy settings for Win11 24H2+ Wi-Fi access.
func (a *App) OpenWindowsLocationSettings() error {
	return wifi.OpenLocationSettings()
}

// RunDNSBenchmark benchmarks built-in DNS resolvers and the active system DNS in parallel.
func (a *App) RunDNSBenchmark() optimizer.DNSBenchmarkResult {
	summary, err := netinfo.GetNetworkSummary()
	var adapterName string
	var ifIndex uint32
	var currentDNS []string
	if err == nil && summary.Primary != nil {
		adapterName = summary.Primary.Name
		ifIndex = summary.Primary.IfIndex
		currentDNS = summary.Primary.DNSServers
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	res := a.dnsBench.Run(ctx, adapterName, ifIndex, currentDNS)
	a.optMu.Lock()
	a.lastDNS = res
	a.optMu.Unlock()

	return a.redactDNSBenchmark(res)
}

// GetLastDNSBenchmark returns the most recent DNS benchmark result, respecting PrivacyMode.
func (a *App) GetLastDNSBenchmark() optimizer.DNSBenchmarkResult {
	a.optMu.RLock()
	res := a.lastDNS
	a.optMu.RUnlock()
	return a.redactDNSBenchmark(res)
}

// ApplyDNS sets primary/secondary IPv4 DNS on the active Windows adapter via on-demand UAC.
func (a *App) ApplyDNS(primary, secondary string) (netinfo.NetworkSummary, error) {
	summary, err := netinfo.GetNetworkSummary()
	if err != nil || !summary.Connected || summary.Primary == nil {
		return a.GetNetworkSummary(), errors.New("no hay un adaptador de red principal activo")
	}
	if err := optimizer.ApplyStaticDNS(summary.Primary.IfIndex, primary, secondary); err != nil {
		return a.GetNetworkSummary(), err
	}
	return a.GetNetworkSummary(), nil
}

// ResetDNSToDHCP restores automatic DHCP DNS settings on the active Windows adapter via UAC.
func (a *App) ResetDNSToDHCP() (netinfo.NetworkSummary, error) {
	summary, err := netinfo.GetNetworkSummary()
	if err != nil || !summary.Connected || summary.Primary == nil {
		return a.GetNetworkSummary(), errors.New("no hay un adaptador de red principal activo")
	}
	if err := optimizer.ResetDNSToDHCP(summary.Primary.IfIndex); err != nil {
		return a.GetNetworkSummary(), err
	}
	return a.GetNetworkSummary(), nil
}

// FlushDNSCache clears the Windows DNS resolver cache without Admin privileges.
func (a *App) FlushDNSCache() error {
	return optimizer.FlushDNSCache()
}

// ScanUPnPPorts audits UPnP IGD port mappings exposed by the local router.
func (a *App) ScanUPnPPorts() optimizer.UPnPScanResult {
	summary, _ := netinfo.GetNetworkSummary()
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	res := optimizer.ScanUPnP(ctx, summary.LocalIP, summary.GatewayIP)

	a.optMu.Lock()
	a.lastUPnP = res
	a.optMu.Unlock()

	return a.redactUPnPResult(res)
}

// GetLastUPnPScan returns the latest UPnP port audit result, respecting PrivacyMode.
func (a *App) GetLastUPnPScan() optimizer.UPnPScanResult {
	a.optMu.RLock()
	res := a.lastUPnP
	a.optMu.RUnlock()
	return a.redactUPnPResult(res)
}

// DeleteUPnPMapping closes an UPnP port mapping on the router and updates the cached scan.
func (a *App) DeleteUPnPMapping(protocol string, externalPort int) (optimizer.UPnPScanResult, error) {
	a.optMu.RLock()
	ctrlURL := a.lastUPnP.ControlURL
	svcType := a.lastUPnP.ServiceType
	a.optMu.RUnlock()

	if ctrlURL == "" || svcType == "" {
		return a.GetLastUPnPScan(), errors.New("ejecutá un escaneo UPnP antes de cerrar puertos")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := optimizer.DeleteUPnPMapping(ctx, ctrlURL, svcType, protocol, externalPort); err != nil {
		return a.GetLastUPnPScan(), err
	}

	a.optMu.Lock()
	filtered := make([]optimizer.UPnPMapping, 0, len(a.lastUPnP.Mappings))
	highRisk := 0
	for _, m := range a.lastUPnP.Mappings {
		if m.ExternalPort == externalPort && strings.EqualFold(m.Protocol, protocol) {
			continue
		}
		filtered = append(filtered, m)
		if m.Risk == "high" {
			highRisk++
		}
	}
	a.lastUPnP.Mappings = filtered
	a.lastUPnP.HighRiskCount = highRisk
	updated := a.lastUPnP
	a.optMu.Unlock()

	return a.redactUPnPResult(updated), nil
}

// DetectRouterProfile fingerprints the default gateway and recommends Wi-Fi/DNS settings for Co-Pilot.
func (a *App) DetectRouterProfile() router.RouterProfile {
	summary, _ := netinfo.GetNetworkSummary()

	a.wifiMu.RLock()
	rec24 := a.lastWifi.Rec24GHz.RecommendedChannel
	rec5 := a.lastWifi.Rec5GHz.RecommendedChannel
	a.wifiMu.RUnlock()

	a.optMu.RLock()
	recDNSName := a.lastDNS.FastestName
	var recDNSPrimary string
	for _, r := range a.lastDNS.Results {
		if r.Provider.ID == a.lastDNS.FastestID {
			recDNSPrimary = r.Provider.Primary
			break
		}
	}
	a.optMu.RUnlock()

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	prof := router.DetectRouterProfile(ctx, summary, rec24, rec5, recDNSName, recDNSPrimary)

	a.routerMu.Lock()
	a.routerProfile = prof
	a.routerMu.Unlock()

	return a.redactRouterProfile(prof)
}

// GetLastRouterProfile returns the cached router Co-Pilot profile, respecting PrivacyMode.
func (a *App) GetLastRouterProfile() router.RouterProfile {
	a.routerMu.RLock()
	prof := a.routerProfile
	a.routerMu.RUnlock()
	return a.redactRouterProfile(prof)
}

// OpenRouterAdminPanel opens the detected local private gateway admin panel in the user's browser.
func (a *App) OpenRouterAdminPanel() error {
	a.routerMu.RLock()
	adminURL := a.routerProfile.AdminURL
	gwIP := a.routerProfile.GatewayIP
	a.routerMu.RUnlock()

	if adminURL == "" || gwIP == "" {
		summary, err := netinfo.GetNetworkSummary()
		if err != nil || !summary.Connected || summary.GatewayIP == "" {
			return errors.New("no se detectó puerta de enlace activa")
		}
		gwIP = summary.GatewayIP
		adminURL = "http://" + gwIP
	}
	return router.OpenAdminPanel(adminURL, gwIP)
}

// CaptureRouterBaseline snapshots current Dual-Hop + Wi-Fi metrics as the "Before" reference.
func (a *App) CaptureRouterBaseline() router.BeforeAfterReport {
	snap := a.capturePointSnapshot()
	a.routerMu.Lock()
	a.routerBaseline = &snap
	a.routerAfter = nil
	a.routerMu.Unlock()

	return router.BeforeAfterReport{
		HasBaseline: true,
		Baseline:    snap,
	}
}

// CompareRouterAfter snapshots current metrics and compares them against the saved baseline.
func (a *App) CompareRouterAfter() router.BeforeAfterReport {
	after := a.capturePointSnapshot()

	a.routerMu.Lock()
	defer a.routerMu.Unlock()

	if a.routerBaseline == nil {
		a.routerBaseline = &after
		return router.BeforeAfterReport{
			HasBaseline: true,
			Baseline:    after,
		}
	}

	a.routerAfter = &after
	return router.CompareSnapshots(*a.routerBaseline, after)
}

// GetRouterBeforeAfter returns the current Before/After comparison state.
func (a *App) GetRouterBeforeAfter() router.BeforeAfterReport {
	a.routerMu.RLock()
	defer a.routerMu.RUnlock()

	if a.routerBaseline == nil {
		return router.BeforeAfterReport{}
	}
	if a.routerAfter == nil {
		return router.BeforeAfterReport{
			HasBaseline: true,
			Baseline:    *a.routerBaseline,
		}
	}
	return router.CompareSnapshots(*a.routerBaseline, *a.routerAfter)
}

func (a *App) capturePointSnapshot() router.BeforeAfterSnapshot {
	dh := a.monitor.Snapshot()

	a.wifiMu.RLock()
	w := a.lastWifi
	a.wifiMu.RUnlock()

	cong := 0
	if w.CurrentBand == "2.4GHz" {
		cong = w.Rec24GHz.CurrentCongestion
	} else if w.CurrentBand == "5GHz" {
		cong = w.Rec5GHz.CurrentCongestion
	}

	return router.BeforeAfterSnapshot{
		CapturedAt:     time.Now().Format("15:04:05"),
		RouterAvgMs:    dh.Router.AvgMs,
		RouterJitterMs: dh.Router.JitterMs,
		RouterLossPct:  dh.Router.LossPct,
		InternetAvgMs:  dh.Internet.AvgMs,
		WiFiBand:       w.CurrentBand,
		WiFiChannel:    w.CurrentChannel,
		WiFiCongestion: cong,
		Score:          dh.Verdict.Score,
	}
}

// GetSavedRouterConfig loads non-secret Auto-Pilot connection settings from Windows Credential Manager.
func (a *App) GetSavedRouterConfig(driver string) router.AutoPilotConfig {
	cfg := a.autoPilot.LoadSavedConfig(driver)
	if a.GetConfig().PrivacyMode && cfg.Host != "" {
		cfg.Host = privacy.MaskIP(cfg.Host)
	}
	return cfg
}

// ClearSavedRouterCredentials deletes stored Auto-Pilot credentials from Windows Credential Manager.
func (a *App) ClearSavedRouterCredentials(driver string) error {
	return a.autoPilot.ClearCredentials(driver)
}

// ClearRouterSSHFingerprint removes the stored TOFU SSH fingerprint for host:port.
func (a *App) ClearRouterSSHFingerprint(host string, port int) error {
	if a.store == nil {
		return nil
	}
	if port <= 0 {
		port = 22
	}
	resolved := a.resolveRouterHost("openwrt", host)
	return a.store.ClearSSHHostFingerprint(net.JoinHostPort(resolved, strconv.Itoa(port)))
}

// TestRouterAutoPilot validates connectivity to a supported router driver and lists its Wi-Fi radios.
func (a *App) TestRouterAutoPilot(cfg router.AutoPilotConfig) (router.AutoPilotStatus, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cfg.Host = a.resolveRouterHost(cfg.Driver, cfg.Host)
	return a.autoPilot.TestConnection(ctx, cfg)
}

// ApplyRouterChannel safely applies a Wi-Fi channel via Auto-Pilot after guardrails and backup.
func (a *App) ApplyRouterChannel(req router.ChannelChangeRequest) (router.ChannelChangeResult, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req.Host = a.resolveRouterHost(req.Driver, req.Host)
	return a.autoPilot.ApplyChannel(ctx, req)
}

// RollbackRouterChannel restores the previous channel captured before the last ApplyRouterChannel.
func (a *App) RollbackRouterChannel() (router.ChannelChangeResult, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.autoPilot.Rollback(ctx)
}

// GetLastRouterBackup returns the latest Auto-Pilot channel backup, if available.
func (a *App) GetLastRouterBackup() *router.ChannelBackup {
	b := a.autoPilot.LastBackup()
	if b != nil && a.GetConfig().PrivacyMode {
		b.Host = privacy.MaskIP(b.Host)
	}
	return b
}

func (a *App) resolveRouterHost(driver, provided string) string {
	trimmed := strings.TrimSpace(provided)
	if trimmed != "" && !strings.Contains(trimmed, "•") {
		return trimmed
	}
	saved := a.autoPilot.LoadSavedConfig(driver)
	if saved.Host != "" {
		return saved.Host
	}
	a.routerMu.RLock()
	gw := a.routerProfile.GatewayIP
	a.routerMu.RUnlock()
	if gw != "" {
		return gw
	}
	summary, err := netinfo.GetNetworkSummary()
	if err == nil && summary.GatewayIP != "" {
		return summary.GatewayIP
	}
	return trimmed
}

func (a *App) redactNetworkSummary(s netinfo.NetworkSummary) netinfo.NetworkSummary {
	if !a.GetConfig().PrivacyMode {
		return s
	}

	s.GatewayIP = privacy.MaskIP(s.GatewayIP)
	s.LocalIP = privacy.MaskIP(s.LocalIP)
	s.AdapterMAC = privacy.MaskMAC(s.AdapterMAC)

	if s.Primary != nil {
		p := *s.Primary
		p.IPv4 = privacy.MaskIP(p.IPv4)
		p.Gateway = privacy.MaskIP(p.Gateway)
		p.MAC = privacy.MaskMAC(p.MAC)
		if len(p.DNSServers) > 0 {
			maskedDNS := make([]string, len(p.DNSServers))
			for i, d := range p.DNSServers {
				maskedDNS[i] = maskPrivateIPOnly(d)
			}
			p.DNSServers = maskedDNS
		}
		s.Primary = &p
	}

	if len(s.Adapters) > 0 {
		masked := make([]netinfo.AdapterInfo, len(s.Adapters))
		for i, ad := range s.Adapters {
			ad.IPv4 = privacy.MaskIP(ad.IPv4)
			ad.Gateway = privacy.MaskIP(ad.Gateway)
			ad.MAC = privacy.MaskMAC(ad.MAC)
			masked[i] = ad
		}
		s.Adapters = masked
	}
	return s
}

func (a *App) redactDualHopSnapshot(snap diagnostics.DualHopSnapshot) diagnostics.DualHopSnapshot {
	if !a.GetConfig().PrivacyMode {
		return snap
	}
	snap.RouterTarget = privacy.MaskIP(snap.RouterTarget)
	snap.Router.Target = privacy.MaskIP(snap.Router.Target)
	return snap
}

func (a *App) redactRadarResult(res radar.ScanResult) radar.ScanResult {
	return a.redactRadarResultLocked(res)
}

func (a *App) redactRadarResultLocked(res radar.ScanResult) radar.ScanResult {
	devicesCopy := make([]radar.Device, len(res.Devices))
	copy(devicesCopy, res.Devices)
	res.Devices = devicesCopy

	if !a.GetConfig().PrivacyMode {
		return res
	}

	if res.Subnet != "" {
		if idx := strings.IndexByte(res.Subnet, '/'); idx > 0 {
			res.Subnet = privacy.MaskIP(res.Subnet[:idx]) + res.Subnet[idx:]
		} else {
			res.Subnet = privacy.MaskIP(res.Subnet)
		}
	}

	if res.GatewayMACAlert != nil {
		alertCopy := *res.GatewayMACAlert
		alertCopy.PreviousMAC = privacy.MaskMAC(alertCopy.PreviousMAC)
		alertCopy.CurrentMAC = privacy.MaskMAC(alertCopy.CurrentMAC)
		res.GatewayMACAlert = &alertCopy
	}

	for i := range res.Devices {
		res.Devices[i].IP = privacy.MaskIP(res.Devices[i].IP)
		res.Devices[i].MAC = privacy.MaskMAC(res.Devices[i].MAC)
	}
	return res
}

func (a *App) redactMTRSnapshot(snap diagnostics.MTRSnapshot) diagnostics.MTRSnapshot {
	hopsCopy := make([]diagnostics.MTRHopStats, len(snap.Hops))
	copy(hopsCopy, snap.Hops)
	snap.Hops = hopsCopy

	if !a.GetConfig().PrivacyMode {
		return snap
	}
	for i := range snap.Hops {
		if snap.Hops[i].IP == "" || snap.Hops[i].IP == "*" {
			continue
		}
		if snap.Hops[i].TTL <= 2 || snap.Hops[i].IsPrivate {
			snap.Hops[i].IP = privacy.MaskIP(snap.Hops[i].IP)
		}
	}
	return snap
}

func (a *App) redactNetworkAudit(res diagnostics.NetworkAuditResult) diagnostics.NetworkAuditResult {
	hopsCopy := make([]diagnostics.TraceHop, len(res.NAT.FirstHops))
	copy(hopsCopy, res.NAT.FirstHops)
	res.NAT.FirstHops = hopsCopy

	if !a.GetConfig().PrivacyMode {
		return res
	}
	for i := range res.NAT.FirstHops {
		ip := res.NAT.FirstHops[i].IP
		if ip == "" || ip == "*" {
			continue
		}
		if res.NAT.FirstHops[i].TTL <= 2 || diagnostics.IsPrivateIPv4(ip) || diagnostics.IsCGNATIP(ip) {
			res.NAT.FirstHops[i].IP = privacy.MaskIP(ip)
		}
	}
	if res.NAT.DoubleNATHopIP != "" {
		res.NAT.DoubleNATHopIP = privacy.MaskIP(res.NAT.DoubleNATHopIP)
	}
	if res.NAT.CGNATHopIP != "" {
		res.NAT.CGNATHopIP = privacy.MaskIP(res.NAT.CGNATHopIP)
	}
	if res.NAT.Detail != "" && (res.NAT.DoubleNATHopIP != "" || res.NAT.CGNATHopIP != "") {
		res.NAT.Detail = maskIPsInText(res.NAT.Detail)
	}
	return res
}

func (a *App) redactProcessSnapshot(snap processes.ProcessSnapshot) processes.ProcessSnapshot {
	procsCopy := make([]processes.ProcessNetUsage, len(snap.Processes))
	copy(procsCopy, snap.Processes)
	snap.Processes = procsCopy

	if !a.GetConfig().PrivacyMode {
		return snap
	}

	for i := range snap.Processes {
		if snap.Processes[i].ExePath != "" {
			// Avoid leaking Windows username in C:\Users\<user>\... paths when Discord Privacy Mode is ON.
			snap.Processes[i].ExePath = filepath.Base(snap.Processes[i].ExePath)
		}
		remotes := make([]processes.ConnectionEndpoint, len(snap.Processes[i].RemoteSample))
		copy(remotes, snap.Processes[i].RemoteSample)
		for j := range remotes {
			if remotes[j].RemoteIP != "" && remotes[j].RemoteIP != "0.0.0.0" {
				remotes[j].RemoteIP = privacy.MaskIP(remotes[j].RemoteIP)
			}
		}
		snap.Processes[i].RemoteSample = remotes
	}
	return snap
}

func (a *App) redactHistorySnapshot(snap history.HistorySnapshot) history.HistorySnapshot {
	incCopy := make([]history.IncidentRecord, len(snap.Incidents))
	copy(incCopy, snap.Incidents)
	snap.Incidents = incCopy

	if !a.GetConfig().PrivacyMode {
		return snap
	}
	for i := range snap.Incidents {
		if snap.Incidents[i].Message != "" {
			snap.Incidents[i].Message = maskIPsInText(snap.Incidents[i].Message)
		}
		if snap.Incidents[i].NetworkName != "" {
			snap.Incidents[i].NetworkName = maskIPsInText(snap.Incidents[i].NetworkName)
		}
	}
	return snap
}

func maskIPsInText(s string) string {
	words := strings.Fields(s)
	changed := false
	for i, w := range words {
		clean := strings.Trim(w, "(),.;:")
		if ip := net.ParseIP(clean); ip != nil && ip.To4() != nil {
			words[i] = strings.Replace(w, clean, privacy.MaskIP(clean), 1)
			changed = true
		}
	}
	if !changed {
		return s
	}
	return strings.Join(words, " ")
}

func (a *App) redactWiFiResult(res wifi.ScanResult) wifi.ScanResult {
	apsCopy := make([]wifi.AccessPoint, len(res.AccessPoints))
	copy(apsCopy, res.AccessPoints)
	res.AccessPoints = apsCopy

	if !a.GetConfig().PrivacyMode {
		return res
	}

	if res.CurrentSSID != "" {
		res.CurrentSSID = privacy.MaskSSID(res.CurrentSSID)
	}
	if res.CurrentBSSID != "" {
		res.CurrentBSSID = privacy.MaskMAC(res.CurrentBSSID)
	}

	for i := range res.AccessPoints {
		if !res.AccessPoints[i].Hidden && res.AccessPoints[i].SSID != "" {
			res.AccessPoints[i].SSID = privacy.MaskSSID(res.AccessPoints[i].SSID)
		}
		if res.AccessPoints[i].BSSID != "" {
			res.AccessPoints[i].BSSID = privacy.MaskMAC(res.AccessPoints[i].BSSID)
		}
	}
	return res
}

func (a *App) redactDNSBenchmark(res optimizer.DNSBenchmarkResult) optimizer.DNSBenchmarkResult {
	resCopy := res
	curCopy := make([]string, len(res.CurrentDNS))
	copy(curCopy, res.CurrentDNS)
	resCopy.CurrentDNS = curCopy

	resultsCopy := make([]optimizer.DNSResult, len(res.Results))
	copy(resultsCopy, res.Results)
	resCopy.Results = resultsCopy

	if !a.GetConfig().PrivacyMode {
		return resCopy
	}

	for i, ip := range resCopy.CurrentDNS {
		resCopy.CurrentDNS[i] = maskPrivateIPOnly(ip)
	}
	for i := range resCopy.Results {
		if resCopy.Results[i].Provider.ID == "current-isp" {
			resCopy.Results[i].Provider.Primary = maskPrivateIPOnly(resCopy.Results[i].Provider.Primary)
			resCopy.Results[i].Provider.Secondary = maskPrivateIPOnly(resCopy.Results[i].Provider.Secondary)
		}
	}
	return resCopy
}

func (a *App) redactUPnPResult(res optimizer.UPnPScanResult) optimizer.UPnPScanResult {
	mappingsCopy := make([]optimizer.UPnPMapping, len(res.Mappings))
	copy(mappingsCopy, res.Mappings)
	res.Mappings = mappingsCopy

	if !a.GetConfig().PrivacyMode {
		return res
	}

	if res.GatewayIP != "" {
		res.GatewayIP = privacy.MaskIP(res.GatewayIP)
	}
	if res.ControlURL != "" {
		if u, err := url.Parse(res.ControlURL); err == nil && u.Hostname() != "" {
			portSuffix := ""
			if u.Port() != "" {
				portSuffix = ":" + u.Port()
			}
			res.ControlURL = u.Scheme + "://" + privacy.MaskIP(u.Hostname()) + portSuffix + u.Path
		}
	}
	for i := range res.Mappings {
		res.Mappings[i].InternalClient = privacy.MaskIP(res.Mappings[i].InternalClient)
	}
	return res
}

func (a *App) redactRouterProfile(prof router.RouterProfile) router.RouterProfile {
	if !a.GetConfig().PrivacyMode {
		return prof
	}
	if prof.GatewayIP != "" {
		prof.GatewayIP = privacy.MaskIP(prof.GatewayIP)
	}
	if prof.GatewayMAC != "" {
		prof.GatewayMAC = privacy.MaskMAC(prof.GatewayMAC)
	}
	if prof.AdminURL != "" {
		if u, err := url.Parse(prof.AdminURL); err == nil && u.Hostname() != "" {
			prof.AdminURL = u.Scheme + "://" + privacy.MaskIP(u.Hostname())
		}
	}
	return prof
}

func maskPrivateIPOnly(ipStr string) string {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip != nil && ip.IsPrivate() {
		return privacy.MaskIP(ipStr)
	}
	return ipStr
}

// GenerateDiscordReport builds an anonymized (or raw if unchecked & PrivacyMode off) report for Discord.
func (a *App) GenerateDiscordReport(forceRedact bool) report.DiscordReport {
	netSum, _ := netinfo.GetNetworkSummary()
	dhSnap := a.monitor.Snapshot()
	bloatRes := a.bloat.LastResult()

	a.radarMu.RLock()
	radRes := a.lastRadar
	a.radarMu.RUnlock()

	a.wifiMu.RLock()
	wifiRes := a.lastWifi
	a.wifiMu.RUnlock()

	a.optMu.RLock()
	dnsRes := a.lastDNS
	upnpRes := a.lastUPnP
	a.optMu.RUnlock()

	a.routerMu.RLock()
	rProf := a.routerProfile
	a.routerMu.RUnlock()

	effectiveRedact := forceRedact || a.GetConfig().PrivacyMode

	return report.BuildDiscordReport(report.ReportInput{
		Version:     appVersion,
		ForceRedact: effectiveRedact,
		Network:     netSum,
		DualHop:     dhSnap,
		Bufferbloat: bloatRes,
		Radar:       radRes,
		WiFi:        wifiRes,
		DNS:         dnsRes,
		UPnP:        upnpRes,
		Router:      rProf,
	})
}

// SaveDiscordReportFile opens a native Windows save dialog to export the Discord Markdown report.
func (a *App) SaveDiscordReportFile(content string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("ventana principal no inicializada")
	}
	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Guardar Reporte NetPulse para Discord",
		DefaultFilename: "netpulse-reporte-discord.md",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Markdown (*.md)", Pattern: "*.md"},
			{DisplayName: "Texto plano (*.txt)", Pattern: "*.txt"},
		},
	})
	if err != nil {
		return "", err
	}
	if savePath == "" {
		return "", nil
	}
	if err := os.WriteFile(savePath, []byte(content), 0o600); err != nil {
		return "", err
	}
	return savePath, nil
}
