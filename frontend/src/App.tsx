import { useEffect, useState } from 'react';
import {
  ApplyDNS,
  ApplyRouterChannel,
  CancelBufferbloatTest,
  CaptureRouterBaseline,
  CheckForUpdates,
  ClearIncidentHistory,
  ClearRouterSSHFingerprint,
  ClearSavedRouterCredentials,
  CompareRouterAfter,
  ConfirmGatewayMACChange,
  DeleteUPnPMapping,
  DetectRouterProfile,
  FlushDNSCache,
  GenerateDiscordReport,
  GetAppInfo,
  GetConfig,
  GetDualHopSnapshot,
  GetIncidentHistory,
  GetLastBufferbloatResult,
  GetLastDNSBenchmark,
  GetLastNetworkAudit,
  GetLastRadarScan,
  GetLastRouterBackup,
  GetLastRouterProfile,
  GetLastUPnPScan,
  GetLastWiFiScan,
  GetMTRSnapshot,
  GetNetworkSummary,
  GetProcessConnections,
  GetRouterBeforeAfter,
  GetSavedRouterConfig,
  GetStartWithWindows,
  InspectRadarDeviceServices,
  OpenRouterAdminPanel,
  OpenWindowsLocationSettings,
  RenameActiveNetworkProfile,
  ResetDNSToDHCP,
  ResetDualHop,
  RollbackRouterChannel,
  RunBufferbloatTest,
  RunDNSBenchmark,
  RunNetworkAudit,
  RunRegionBenchmark,
  RunTraceroute,
  SaveDiscordReportFile,
  SaveISPReportFile,
  ScanLAN,
  ScanUPnPPorts,
  ScanWiFi,
  SetPrivacyMode,
  SetStartWithWindows,
  SetTrayPreferences,
  SetUIPreferences,
  StartDualHop,
  StartMTR,
  StopDualHop,
  StopMTR,
  TestRouterAutoPilot,
  TrustAllCurrentDevices,
  UpdateRadarDevice,
  WakeRadarDevice,
} from '../wailsjs/go/main/App';
import { BrowserOpenURL, EventsOn } from '../wailsjs/runtime/runtime';
import {
  diagnostics,
  history,
  main,
  netinfo,
  optimizer,
  processes,
  radar,
  report,
  router,
  updater,
  wifi,
} from '../wailsjs/go/models';
import { Sidebar } from './components/Sidebar';
import { TopBar } from './components/TopBar';
import { PatchNotesModal } from './components/PatchNotesModal';
import type { BufferbloatProgress } from './components/BufferbloatCard';
import { DashboardView } from './views/DashboardView';
import { RadarView, type RadarScanProgress } from './views/RadarView';
import { WifiView } from './views/WifiView';
import { ProcessesView } from './views/ProcessesView';
import { OptimizerView } from './views/OptimizerView';
import { RouterView } from './views/RouterView';
import { DiscordReportView } from './views/DiscordReportView';
import { SECTIONS, type NavSection } from './types/navigation';
import type { UILang, UITheme } from './i18n';

function App() {
  const [activeSection, setActiveSection] = useState<NavSection>('dashboard');
  const [appInfo, setAppInfo] = useState<main.AppInfo | null>(null);
  const [privacyMode, setPrivacyMode] = useState<boolean>(true);
  const [savingPrivacy, setSavingPrivacy] = useState<boolean>(false);
  const [lang, setLang] = useState<UILang>('es');
  const [theme, setTheme] = useState<UITheme>('dark');
  const [minimizeToTray, setMinimizeToTray] = useState<boolean>(false);
  const [backgroundRadarAlert, setBackgroundRadarAlert] = useState<boolean>(false);
  const [startWithWindows, setStartWithWindows] = useState<boolean>(false);
  const [updateResult, setUpdateResult] = useState<updater.UpdateCheckResult | null>(null);
  const [checkingUpdates, setCheckingUpdates] = useState<boolean>(false);
  const [patchNotesOpen, setPatchNotesOpen] = useState<boolean>(false);

  const [networkSummary, setNetworkSummary] = useState<netinfo.NetworkSummary | null>(null);
  const [dualHop, setDualHop] = useState<diagnostics.DualHopSnapshot | null>(null);
  const [selectedTarget, setSelectedTarget] = useState<string>('1.1.1.1');

  const [bloatRunning, setBloatRunning] = useState<boolean>(false);
  const [bloatProgress, setBloatProgress] = useState<BufferbloatProgress | null>(null);
  const [bloatResult, setBloatResult] = useState<diagnostics.BufferbloatResult | null>(null);
  const [bloatError, setBloatError] = useState<string | null>(null);

  const [regionResult, setRegionResult] = useState<diagnostics.RegionBenchmarkResult | null>(null);
  const [regionRunning, setRegionRunning] = useState<boolean>(false);
  const [traceResult, setTraceResult] = useState<diagnostics.TracerouteResult | null>(null);
  const [traceRunning, setTraceRunning] = useState<boolean>(false);
  const [mtrSnapshot, setMtrSnapshot] = useState<diagnostics.MTRSnapshot | null>(null);
  const [networkAudit, setNetworkAudit] = useState<diagnostics.NetworkAuditResult | null>(null);
  const [auditRunning, setAuditRunning] = useState<boolean>(false);
  const [incidentHistory, setIncidentHistory] = useState<history.HistorySnapshot | null>(null);

  const [procSnapshot, setProcSnapshot] = useState<processes.ProcessSnapshot | null>(null);
  const [procLoading, setProcLoading] = useState<boolean>(false);

  const [radarScanning, setRadarScanning] = useState<boolean>(false);
  const [radarProgress, setRadarProgress] = useState<RadarScanProgress | null>(null);
  const [radarResult, setRadarResult] = useState<radar.ScanResult | null>(null);
  const [radarError, setRadarError] = useState<string | null>(null);

  const [wifiScanning, setWifiScanning] = useState<boolean>(false);
  const [wifiResult, setWifiResult] = useState<wifi.ScanResult | null>(null);

  const [dnsBenchmark, setDnsBenchmark] = useState<optimizer.DNSBenchmarkResult | null>(null);
  const [dnsRunning, setDnsRunning] = useState<boolean>(false);
  const [dnsApplyingId, setDnsApplyingId] = useState<string | null>(null);
  const [dnsFeedback, setDnsFeedback] = useState<{ ok: boolean; message: string } | null>(null);

  const [upnpResult, setUpnpResult] = useState<optimizer.UPnPScanResult | null>(null);
  const [upnpScanning, setUpnpScanning] = useState<boolean>(false);
  const [upnpDeletingKey, setUpnpDeletingKey] = useState<string | null>(null);
  const [upnpError, setUpnpError] = useState<string | null>(null);

  const [routerProfile, setRouterProfile] = useState<router.RouterProfile | null>(null);
  const [detectingRouter, setDetectingRouter] = useState<boolean>(false);
  const [beforeAfter, setBeforeAfter] = useState<router.BeforeAfterReport | null>(null);
  const [lastBackup, setLastBackup] = useState<router.ChannelBackup | null>(null);

  const [discordReport, setDiscordReport] = useState<report.DiscordReport | null>(null);
  const [discordForceRedact, setDiscordForceRedact] = useState<boolean>(true);
  const [discordLoading, setDiscordLoading] = useState<boolean>(false);

  useEffect(() => {
    let active = true;

    async function hydrate() {
      try {
        const [
          info,
          cfg,
          netSum,
          dhSnap,
          lastBloat,
          lastRadar,
          lastWifi,
          lastDns,
          lastUpnp,
          lastRouter,
          baReport,
          backup,
          histSnap,
          lastAudit,
          lastMTR,
          autoWin,
        ] = await Promise.all([
          GetAppInfo(),
          GetConfig(),
          GetNetworkSummary(),
          GetDualHopSnapshot(),
          GetLastBufferbloatResult(),
          GetLastRadarScan(),
          GetLastWiFiScan(),
          GetLastDNSBenchmark(),
          GetLastUPnPScan(),
          GetLastRouterProfile(),
          GetRouterBeforeAfter(),
          GetLastRouterBackup(),
          GetIncidentHistory(30),
          GetLastNetworkAudit(),
          GetMTRSnapshot(),
          GetStartWithWindows(),
        ]);
        if (!active) return;
        setAppInfo(info);
        setPrivacyMode(cfg.privacyMode);
        setLang(cfg.language === 'en' ? 'en' : 'es');
        setTheme(cfg.theme === 'light' ? 'light' : 'dark');
        setMinimizeToTray(Boolean(cfg.minimizeToTray));
        setBackgroundRadarAlert(Boolean(cfg.backgroundRadarAlert));
        setStartWithWindows(Boolean(autoWin));
        setIncidentHistory(histSnap);
        if (lastAudit && lastAudit.completedAt) setNetworkAudit(lastAudit);
        if (lastMTR && (lastMTR.running || lastMTR.cycles > 0)) setMtrSnapshot(lastMTR);
        if (cfg.pingTarget) {
          setSelectedTarget(cfg.pingTarget);
        }
        setNetworkSummary(netSum);
        setDualHop(dhSnap);
        if (lastBloat && lastBloat.grade) setBloatResult(lastBloat);
        if (lastRadar && (lastRadar.totalDevices > 0 || lastRadar.completedAt)) setRadarResult(lastRadar);
        if (lastWifi && lastWifi.completedAt) setWifiResult(lastWifi);
        if (lastDns && lastDns.completedAt) setDnsBenchmark(lastDns);
        if (lastUpnp && lastUpnp.completedAt) setUpnpResult(lastUpnp);
        if (lastRouter && lastRouter.detected) setRouterProfile(lastRouter);
        if (baReport && baReport.hasBaseline) setBeforeAfter(baReport);
        if (backup) setLastBackup(backup);
      } catch (err) {
        console.error('Error initializing NetPulse state:', err);
      }
    }

    hydrate();

    const offDualHop = EventsOn('dualhop:sample', (snap: diagnostics.DualHopSnapshot) => {
      if (active) setDualHop(snap);
    });
    const offMTR = EventsOn('mtr:update', (snap: diagnostics.MTRSnapshot) => {
      if (active) setMtrSnapshot(snap);
    });
    const offBloat = EventsOn('bufferbloat:progress', (p: BufferbloatProgress) => {
      if (active) setBloatProgress(p);
    });
    const offRadar = EventsOn('radar:progress', (p: RadarScanProgress) => {
      if (active) setRadarProgress(p);
    });
    const offNetChanged = EventsOn('network:changed', (sum: netinfo.NetworkSummary) => {
      if (active) setNetworkSummary(sum);
    });
    const offIntruder = EventsOn('radar:intruder', async () => {
      if (active) {
        const latest = await GetLastRadarScan();
        setRadarResult(latest);
      }
    });
    const offGWAlert = EventsOn('radar:gateway-alert', async () => {
      if (active) {
        const latest = await GetLastRadarScan();
        setRadarResult(latest);
      }
    });

    return () => {
      active = false;
      offDualHop();
      offMTR();
      offBloat();
      offRadar();
      offNetChanged();
      offIntruder();
      offGWAlert();
    };
  }, []);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'd') {
        e.preventDefault();
        handleTogglePrivacy();
        return;
      }
      if (e.key === 'F5') {
        e.preventDefault();
        if (activeSection === 'radar') handleScanLAN();
        else if (activeSection === 'wifi') handleScanWiFi(true);
        else if (activeSection === 'processes') handleRefreshProcesses();
        else if (activeSection === 'optimizer') handleRunDNSBenchmark();
        else if (activeSection === 'router') handleDetectRouterProfile();
        else if (activeSection === 'discord') handleGenerateDiscordReport(discordForceRedact);
        else handleResetMonitor();
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  });

  useEffect(() => {
    if (activeSection === 'wifi' && !wifiResult && !wifiScanning) {
      handleScanWiFi(false);
    }
    if (activeSection === 'processes' && !procSnapshot && !procLoading) {
      handleRefreshProcesses();
    }
    if (activeSection === 'router' && !routerProfile && !detectingRouter) {
      handleDetectRouterProfile();
    }
    if (activeSection === 'discord') {
      handleGenerateDiscordReport(discordForceRedact);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeSection]);

  async function handleTogglePrivacy() {
    if (savingPrivacy) return;
    setSavingPrivacy(true);
    try {
      const updated = await SetPrivacyMode(!privacyMode);
      setPrivacyMode(updated.privacyMode);
      const [netSum, dhSnap, lastRadar, lastWifi, lastDns, lastUpnp, lastRouter, backup, histSnap, lastAud, lastMtr] =
        await Promise.all([
          GetNetworkSummary(),
          GetDualHopSnapshot(),
          GetLastRadarScan(),
          GetLastWiFiScan(),
          GetLastDNSBenchmark(),
          GetLastUPnPScan(),
          GetLastRouterProfile(),
          GetLastRouterBackup(),
          GetIncidentHistory(30),
          GetLastNetworkAudit(),
          GetMTRSnapshot(),
        ]);
      setNetworkSummary(netSum);
      setDualHop(dhSnap);
      setIncidentHistory(histSnap);
      if (lastAud && lastAud.completedAt) setNetworkAudit(lastAud);
      if (lastMtr && (lastMtr.running || lastMtr.cycles > 0)) setMtrSnapshot(lastMtr);
      if (lastRadar && (lastRadar.totalDevices > 0 || lastRadar.completedAt)) setRadarResult(lastRadar);
      if (lastWifi && lastWifi.completedAt) setWifiResult(lastWifi);
      if (lastDns && lastDns.completedAt) setDnsBenchmark(lastDns);
      if (lastUpnp && lastUpnp.completedAt) setUpnpResult(lastUpnp);
      if (lastRouter && lastRouter.detected) setRouterProfile(lastRouter);
      setLastBackup(backup);
      if (activeSection === 'processes') {
        await handleRefreshProcesses();
      }

      if (activeSection === 'discord' || discordReport) {
        const rep = await GenerateDiscordReport(discordForceRedact || updated.privacyMode);
        setDiscordReport(rep);
      }
    } catch (err) {
      console.error('Error toggling privacy mode:', err);
    } finally {
      setSavingPrivacy(false);
    }
  }

  async function handleChangeTrayPrefs(minTray: boolean, bgAlert: boolean) {
    setMinimizeToTray(minTray);
    setBackgroundRadarAlert(bgAlert);
    try {
      const updated = await SetTrayPreferences(minTray, bgAlert);
      setMinimizeToTray(Boolean(updated.minimizeToTray));
      setBackgroundRadarAlert(Boolean(updated.backgroundRadarAlert));
    } catch (err) {
      console.error('Error saving tray prefs:', err);
    }
  }

  async function handleToggleLang() {
    const next: UILang = lang === 'es' ? 'en' : 'es';
    setLang(next);
    try {
      await SetUIPreferences(next, theme);
    } catch (err) {
      console.error('Error saving language:', err);
    }
  }

  async function handleToggleTheme() {
    const next: UITheme = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    try {
      await SetUIPreferences(lang, next);
    } catch (err) {
      console.error('Error saving theme:', err);
    }
  }

  async function handleToggleAutostart(enabled: boolean) {
    setStartWithWindows(enabled);
    try {
      const cfg = await SetStartWithWindows(enabled);
      setStartWithWindows(Boolean(cfg.startWithWindows));
    } catch (err) {
      console.error('Error updating Windows autostart:', err);
    }
  }

  async function handleCheckUpdates() {
    if (checkingUpdates) return;
    setCheckingUpdates(true);
    try {
      setUpdateResult(await CheckForUpdates(''));
    } finally {
      setCheckingUpdates(false);
    }
  }

  async function handleRunNetworkAudit() {
    if (auditRunning) return;
    setAuditRunning(true);
    try {
      setNetworkAudit(await RunNetworkAudit());
    } finally {
      setAuditRunning(false);
    }
  }

  async function handleToggleMTR() {
    try {
      if (mtrSnapshot?.running) {
        setMtrSnapshot(await StopMTR());
      } else {
        await StartMTR(selectedTarget);
        setMtrSnapshot(await GetMTRSnapshot());
      }
    } catch (err) {
      console.error('Error toggling MTR:', err);
    }
  }

  async function handleClearHistory() {
    await ClearIncidentHistory();
    setIncidentHistory(await GetIncidentHistory(30));
  }

  async function handleExportISPReport(format: 'csv' | 'html') {
    await SaveISPReportFile(format, 30);
    setIncidentHistory(await GetIncidentHistory(30));
  }

  async function handleRefreshProcesses() {
    if (procLoading) return;
    setProcLoading(true);
    try {
      setProcSnapshot(await GetProcessConnections());
    } catch (err) {
      console.error('Error loading process connections:', err);
    } finally {
      setProcLoading(false);
    }
  }

  async function handleConfirmGatewayMAC() {
    setRadarResult(await ConfirmGatewayMACChange());
  }

  async function handleRenameRadarProfile(name: string) {
    setRadarResult(await RenameActiveNetworkProfile(name));
  }

  async function handleChangeTarget(target: string) {
    setSelectedTarget(target);
    try {
      setDualHop(await StartDualHop(target));
    } catch (err) {
      console.error('Error changing ping target:', err);
    }
  }

  async function handleToggleMonitor() {
    try {
      const snap = dualHop?.running
        ? await StopDualHop()
        : await StartDualHop(selectedTarget);
      setDualHop(snap);
    } catch (err) {
      console.error('Error toggling Dual-Hop monitor:', err);
    }
  }

  async function handleResetMonitor() {
    try {
      setDualHop(await ResetDualHop());
    } catch (err) {
      console.error('Error resetting Dual-Hop monitor:', err);
    }
  }

  async function handleRunBufferbloat() {
    if (bloatRunning) return;
    setBloatRunning(true);
    setBloatError(null);
    setBloatProgress({
      phase: 'idle',
      progressPct: 2,
      currentPingMs: 0,
      currentMbps: 0,
      message: 'Iniciando test de Bufferbloat...',
    });

    try {
      setBloatResult(await RunBufferbloatTest());
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (!msg.toLowerCase().includes('cancel')) setBloatError(msg);
    } finally {
      setBloatRunning(false);
    }
  }

  async function handleCancelBufferbloat() {
    try {
      await CancelBufferbloatTest();
    } catch (err) {
      console.error('Error cancelling Bufferbloat test:', err);
    }
  }

  async function handleRunRegions() {
    if (regionRunning) return;
    setRegionRunning(true);
    try {
      setRegionResult(await RunRegionBenchmark());
    } finally {
      setRegionRunning(false);
    }
  }

  async function handleRunTraceroute() {
    if (traceRunning) return;
    setTraceRunning(true);
    try {
      setTraceResult(await RunTraceroute(selectedTarget));
    } finally {
      setTraceRunning(false);
    }
  }

  async function handleScanLAN() {
    if (radarScanning) return;
    setRadarScanning(true);
    setRadarError(null);
    setRadarProgress({
      stage: 'arp',
      progressPct: 5,
      foundCount: 0,
      message: 'Preparando escaneo de red local...',
    });

    try {
      setRadarResult(await ScanLAN());
    } catch (err) {
      setRadarError(err instanceof Error ? err.message : String(err));
    } finally {
      setRadarScanning(false);
    }
  }

  async function handleUpdateRadarDevice(
    deviceKey: string,
    alias: string,
    category: string,
    trusted: boolean,
  ) {
    try {
      setRadarResult(await UpdateRadarDevice(deviceKey, alias, category, trusted));
    } catch (err) {
      setRadarError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleTrustAllDevices() {
    try {
      setRadarResult(await TrustAllCurrentDevices());
    } catch (err) {
      setRadarError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleScanWiFi(activeScan: boolean) {
    if (wifiScanning) return;
    setWifiScanning(true);
    try {
      setWifiResult(await ScanWiFi(activeScan));
    } catch (err) {
      console.error('Error scanning Wi-Fi spectrum:', err);
    } finally {
      setWifiScanning(false);
    }
  }

  async function handleOpenLocationSettings() {
    try {
      await OpenWindowsLocationSettings();
    } catch (err) {
      console.error('Error opening Windows location settings:', err);
    }
  }

  async function handleRunDNSBenchmark() {
    if (dnsRunning) return;
    setDnsRunning(true);
    setDnsFeedback(null);
    try {
      setDnsBenchmark(await RunDNSBenchmark());
    } catch (err) {
      setDnsFeedback({ ok: false, message: err instanceof Error ? err.message : String(err) });
    } finally {
      setDnsRunning(false);
    }
  }

  async function handleApplyDNS(id: string, primary: string, secondary: string) {
    setDnsApplyingId(id);
    setDnsFeedback(null);
    try {
      const updatedNet = await ApplyDNS(primary, secondary);
      setNetworkSummary(updatedNet);
      setDnsFeedback({ ok: true, message: `DNS aplicado (${primary}) y caché DNS limpiada.` });
    } catch (err) {
      setDnsFeedback({ ok: false, message: err instanceof Error ? err.message : String(err) });
    } finally {
      setDnsApplyingId(null);
    }
  }

  async function handleResetDNSDHCP() {
    setDnsApplyingId('dhcp');
    setDnsFeedback(null);
    try {
      const updatedNet = await ResetDNSToDHCP();
      setNetworkSummary(updatedNet);
      setDnsFeedback({ ok: true, message: 'DNS restaurado a automático (DHCP) correctamente.' });
    } catch (err) {
      setDnsFeedback({ ok: false, message: err instanceof Error ? err.message : String(err) });
    } finally {
      setDnsApplyingId(null);
    }
  }

  async function handleFlushDNSCache() {
    setDnsFeedback(null);
    try {
      await FlushDNSCache();
      setDnsFeedback({ ok: true, message: 'Caché DNS de Windows limpiada sin errores.' });
    } catch (err) {
      setDnsFeedback({ ok: false, message: err instanceof Error ? err.message : String(err) });
    }
  }

  async function handleScanUPnP() {
    if (upnpScanning) return;
    setUpnpScanning(true);
    setUpnpError(null);
    try {
      setUpnpResult(await ScanUPnPPorts());
    } catch (err) {
      setUpnpError(err instanceof Error ? err.message : String(err));
    } finally {
      setUpnpScanning(false);
    }
  }

  async function handleDeleteUPnP(protocol: string, externalPort: number) {
    const key = `${protocol}:${externalPort}`;
    setUpnpDeletingKey(key);
    setUpnpError(null);
    try {
      setUpnpResult(await DeleteUPnPMapping(protocol, externalPort));
    } catch (err) {
      setUpnpError(err instanceof Error ? err.message : String(err));
    } finally {
      setUpnpDeletingKey(null);
    }
  }

  async function handleDetectRouterProfile() {
    if (detectingRouter) return;
    setDetectingRouter(true);
    try {
      setRouterProfile(await DetectRouterProfile());
    } catch (err) {
      console.error('Error detecting router profile:', err);
    } finally {
      setDetectingRouter(false);
    }
  }

  async function handleOpenRouterAdmin() {
    try {
      await OpenRouterAdminPanel();
    } catch (err) {
      console.error('Error opening router admin panel:', err);
    }
  }

  async function handleCaptureBaseline() {
    setBeforeAfter(await CaptureRouterBaseline());
  }

  async function handleCompareAfter() {
    setBeforeAfter(await CompareRouterAfter());
  }

  async function handleApplyRouterChannel(req: router.ChannelChangeRequest) {
    const res = await ApplyRouterChannel(req);
    setLastBackup(await GetLastRouterBackup());
    return res;
  }

  async function handleRollbackRouterChannel() {
    const res = await RollbackRouterChannel();
    setLastBackup(await GetLastRouterBackup());
    return res;
  }

  async function handleGenerateDiscordReport(forceRedactArg: boolean) {
    setDiscordLoading(true);
    try {
      const rep = await GenerateDiscordReport(forceRedactArg);
      setDiscordReport(rep);
    } catch (err) {
      console.error('Error generating Discord report:', err);
    } finally {
      setDiscordLoading(false);
    }
  }

  async function handleToggleDiscordForceRedact(val: boolean) {
    setDiscordForceRedact(val);
    await handleGenerateDiscordReport(val);
  }

  return (
    <div className="h-screen w-screen bg-[#090D16] text-slate-100 flex overflow-hidden">
      <Sidebar
        activeSection={activeSection}
        onSelectSection={setActiveSection}
        version={appInfo?.version ?? '1.2.0'}
        onOpenPatchNotes={() => setPatchNotesOpen(true)}
        lang={lang}
        radarTotalCount={radarResult?.totalDevices ?? 0}
        radarUnknownCount={radarResult?.unknownCount ?? 0}
        wifiAPCount={wifiResult?.accessPoints?.length ?? 0}
        wifiStatus={wifiResult?.status ?? ''}
        processCount={procSnapshot?.totalProcesses ?? 0}
        upnpHighRiskCount={upnpResult?.highRiskCount ?? 0}
      />

      <div className="flex-1 min-w-0 flex flex-col">
        <TopBar
          section={SECTIONS[activeSection]}
          privacyMode={privacyMode}
          savingPrivacy={savingPrivacy}
          onTogglePrivacy={handleTogglePrivacy}
          networkSummary={networkSummary}
          lang={lang}
          theme={theme}
          onToggleLang={handleToggleLang}
          onToggleTheme={handleToggleTheme}
        />

        <main className="flex-1 overflow-y-auto np-scroll p-6">
          {activeSection === 'dashboard' ? (
            <DashboardView
              appInfo={appInfo}
              privacyMode={privacyMode}
              networkSummary={networkSummary}
              dualHop={dualHop}
              selectedTarget={selectedTarget}
              onChangeTarget={handleChangeTarget}
              onToggleMonitor={handleToggleMonitor}
              onResetMonitor={handleResetMonitor}
              bloatRunning={bloatRunning}
              bloatProgress={bloatProgress}
              bloatResult={bloatResult}
              bloatError={bloatError}
              onRunBufferbloat={handleRunBufferbloat}
              onCancelBufferbloat={handleCancelBufferbloat}
              regionResult={regionResult}
              regionRunning={regionRunning}
              onRunRegions={handleRunRegions}
              traceResult={traceResult}
              traceRunning={traceRunning}
              onRunTraceroute={handleRunTraceroute}
              mtrSnapshot={mtrSnapshot}
              onToggleMTR={handleToggleMTR}
              networkAudit={networkAudit}
              auditRunning={auditRunning}
              onRunNetworkAudit={handleRunNetworkAudit}
              incidentHistory={incidentHistory}
              onClearHistory={handleClearHistory}
              onExportISPReport={handleExportISPReport}
              minimizeToTray={minimizeToTray}
              backgroundRadarAlert={backgroundRadarAlert}
              onChangeTrayPrefs={handleChangeTrayPrefs}
              startWithWindows={startWithWindows}
              onToggleAutostart={handleToggleAutostart}
              updateResult={updateResult}
              checkingUpdates={checkingUpdates}
              onCheckUpdates={handleCheckUpdates}
              onOpenExternalURL={BrowserOpenURL}
              onOpenPatchNotes={() => setPatchNotesOpen(true)}
              lang={lang}
            />
          ) : activeSection === 'radar' ? (
            <RadarView
              scanning={radarScanning}
              progress={radarProgress}
              scanResult={radarResult}
              error={radarError}
              connected={networkSummary?.connected ?? false}
              privacyMode={privacyMode}
              onScan={handleScanLAN}
              onUpdateDevice={handleUpdateRadarDevice}
              onTrustAll={handleTrustAllDevices}
              onInspectServices={InspectRadarDeviceServices}
              onWakeDevice={WakeRadarDevice}
              onConfirmGatewayMAC={handleConfirmGatewayMAC}
              onRenameProfile={handleRenameRadarProfile}
            />
          ) : activeSection === 'wifi' ? (
            <WifiView
              scanning={wifiScanning}
              scanResult={wifiResult}
              privacyMode={privacyMode}
              onScan={handleScanWiFi}
              onOpenLocationSettings={handleOpenLocationSettings}
            />
          ) : activeSection === 'processes' ? (
            <ProcessesView
              snapshot={procSnapshot}
              loading={procLoading}
              privacyMode={privacyMode}
              lang={lang}
              onRefresh={handleRefreshProcesses}
            />
          ) : activeSection === 'optimizer' ? (
            <OptimizerView
              networkSummary={networkSummary}
              dnsBenchmark={dnsBenchmark}
              dnsRunning={dnsRunning}
              dnsApplyingId={dnsApplyingId}
              dnsFeedback={dnsFeedback}
              upnpResult={upnpResult}
              upnpScanning={upnpScanning}
              upnpDeletingKey={upnpDeletingKey}
              upnpError={upnpError}
              onRunDNSBenchmark={handleRunDNSBenchmark}
              onApplyDNS={handleApplyDNS}
              onResetDNSDHCP={handleResetDNSDHCP}
              onFlushDNSCache={handleFlushDNSCache}
              onScanUPnP={handleScanUPnP}
              onDeleteUPnP={handleDeleteUPnP}
            />
          ) : activeSection === 'router' ? (
            <RouterView
              profile={routerProfile}
              detectingProfile={detectingRouter}
              beforeAfter={beforeAfter}
              privacyMode={privacyMode}
              onDetectProfile={handleDetectRouterProfile}
              onOpenAdminPanel={handleOpenRouterAdmin}
              onCaptureBaseline={handleCaptureBaseline}
              onCompareAfter={handleCompareAfter}
              onLoadSavedConfig={GetSavedRouterConfig}
              onClearSavedCreds={ClearSavedRouterCredentials}
              onClearSSHFingerprint={ClearRouterSSHFingerprint}
              onTestAutoPilot={TestRouterAutoPilot}
              onApplyChannel={handleApplyRouterChannel}
              onRollbackChannel={handleRollbackRouterChannel}
              lastBackup={lastBackup}
            />
          ) : (
            <DiscordReportView
              discordReport={discordReport}
              loading={discordLoading}
              privacyMode={privacyMode}
              forceRedact={discordForceRedact}
              onToggleForceRedact={handleToggleDiscordForceRedact}
              onRefresh={() => handleGenerateDiscordReport(discordForceRedact)}
              onSaveMarkdown={SaveDiscordReportFile}
            />
          )}
        </main>
      </div>

      <PatchNotesModal
        open={patchNotesOpen}
        onClose={() => setPatchNotesOpen(false)}
        lang={lang}
      />
    </div>
  );
}

export default App;
