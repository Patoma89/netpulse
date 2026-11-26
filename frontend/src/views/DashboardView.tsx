import { useState } from 'react';
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  BellRing,
  Cable,
  Check,
  CheckCircle2,
  Copy,
  Download,
  ExternalLink,
  FolderLock,
  Gamepad2,
  Globe,
  KeyRound,
  Laptop,
  MonitorDown,
  Pause,
  Play,
  RefreshCw,
  RotateCcw,
  Route,
  Router,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
  Trash2,
  Wifi,
} from 'lucide-react';
import { diagnostics, history, main, netinfo, updater } from '../../wailsjs/go/models';
import { DualHopChart } from '../components/DualHopChart';
import {
  BufferbloatCard,
  type BufferbloatProgress,
} from '../components/BufferbloatCard';
import { PrivacyValue } from '../components/PrivacyValue';
import type { UILang } from '../i18n';

interface DashboardViewProps {
  appInfo: main.AppInfo | null;
  privacyMode: boolean;
  networkSummary: netinfo.NetworkSummary | null;
  dualHop: diagnostics.DualHopSnapshot | null;
  selectedTarget: string;
  onChangeTarget: (target: string) => void;
  onToggleMonitor: () => void;
  onResetMonitor: () => void;
  bloatRunning: boolean;
  bloatProgress: BufferbloatProgress | null;
  bloatResult: diagnostics.BufferbloatResult | null;
  bloatError: string | null;
  onRunBufferbloat: () => void;
  onCancelBufferbloat: () => void;
  regionResult: diagnostics.RegionBenchmarkResult | null;
  regionRunning: boolean;
  onRunRegions: () => void;
  traceResult: diagnostics.TracerouteResult | null;
  traceRunning: boolean;
  onRunTraceroute: () => void;
  mtrSnapshot: diagnostics.MTRSnapshot | null;
  onToggleMTR: () => void;
  networkAudit: diagnostics.NetworkAuditResult | null;
  auditRunning: boolean;
  onRunNetworkAudit: () => void;
  incidentHistory: history.HistorySnapshot | null;
  onClearHistory: () => Promise<void>;
  onExportISPReport: (format: 'csv' | 'html') => Promise<void>;
  minimizeToTray: boolean;
  backgroundRadarAlert: boolean;
  onChangeTrayPrefs: (minimize: boolean, bgAlert: boolean) => void;
  startWithWindows: boolean;
  onToggleAutostart: (enabled: boolean) => void;
  updateResult: updater.UpdateCheckResult | null;
  checkingUpdates: boolean;
  onCheckUpdates: () => void;
  onOpenExternalURL: (url: string) => void;
  onOpenPatchNotes: () => void;
  lang: UILang;
}

const TARGET_PRESETS = [
  { value: '1.1.1.1', label: 'Cloudflare (1.1.1.1)' },
  { value: '8.8.8.8', label: 'Google (8.8.8.8)' },
  { value: '9.9.9.9', label: 'Quad9 (9.9.9.9)' },
];

function severityTheme(severity?: string) {
  switch (severity) {
    case 'emerald':
      return {
        border: 'border-emerald-500/30',
        bg: 'bg-emerald-500/[0.06]',
        text: 'text-emerald-300',
      };
    case 'amber':
      return {
        border: 'border-amber-500/30',
        bg: 'bg-amber-500/[0.06]',
        text: 'text-amber-300',
      };
    case 'rose':
      return {
        border: 'border-rose-500/30',
        bg: 'bg-rose-500/[0.06]',
        text: 'text-rose-300',
      };
    default:
      return {
        border: 'border-cyan-500/30',
        bg: 'bg-cyan-500/[0.06]',
        text: 'text-cyan-300',
      };
  }
}

function regionGradeStyle(grade: string): string {
  switch (grade) {
    case 'great':
      return 'text-emerald-300 bg-emerald-500/15 border-emerald-500/30';
    case 'good':
      return 'text-cyan-300 bg-cyan-500/15 border-cyan-500/30';
    case 'fair':
      return 'text-amber-300 bg-amber-500/15 border-amber-500/30';
    default:
      return 'text-rose-300 bg-rose-500/15 border-rose-500/30';
  }
}

export function DashboardView({
  appInfo,
  privacyMode,
  networkSummary,
  dualHop,
  selectedTarget,
  onChangeTarget,
  onToggleMonitor,
  onResetMonitor,
  bloatRunning,
  bloatProgress,
  bloatResult,
  bloatError,
  onRunBufferbloat,
  onCancelBufferbloat,
  regionResult,
  regionRunning,
  onRunRegions,
  traceResult,
  traceRunning,
  onRunTraceroute,
  mtrSnapshot,
  onToggleMTR,
  networkAudit,
  auditRunning,
  onRunNetworkAudit,
  incidentHistory,
  onClearHistory,
  onExportISPReport,
  minimizeToTray,
  backgroundRadarAlert,
  onChangeTrayPrefs,
  startWithWindows,
  onToggleAutostart,
  updateResult,
  checkingUpdates,
  onCheckUpdates,
  onOpenExternalURL,
  onOpenPatchNotes,
  lang,
}: DashboardViewProps) {
  const [copiedLog, setCopiedLog] = useState(false);
  const [historyTab, setHistoryTab] = useState<'session' | 'history30'>('session');
  const [exportNote, setExportNote] = useState<string | null>(null);

  const verdict = dualHop?.verdict;
  const theme = severityTheme(verdict?.severity);
  const routerStats = dualHop?.router;
  const internetStats = dualHop?.internet;
  const incidents = dualHop?.incidents ?? [];
  const connected = networkSummary?.connected ?? false;

  const ispDeltaAvg =
    routerStats && internetStats && internetStats.sent > 0 && routerStats.sent > 0
      ? Math.max(0, internetStats.avgMs - routerStats.avgMs)
      : 0;

  async function handleCopyIncidents() {
    if (incidents.length === 0) return;
    const text = incidents
      .map((i) => `[${i.timeLabel}] (${i.hop.toUpperCase()}) ${i.message}`)
      .join('\n');
    await navigator.clipboard.writeText(text);
    setCopiedLog(true);
    setTimeout(() => setCopiedLog(false), 2000);
  }

  return (
    <div className="space-y-5">
      {/* 1. Hero Telemetry Row */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        <div
          className={`lg:col-span-7 np-card rounded-xl p-5 border ${theme.border} ${theme.bg} flex flex-col justify-between`}
        >
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <Activity className={`h-4 w-4 ${theme.text}`} />
                <span className="text-xs font-medium uppercase tracking-wider text-slate-300">
                  Veredicto en Vivo · ¿Es mi Wi-Fi o mi Proveedor?
                </span>
              </div>
              <h2 className="text-lg font-semibold text-slate-100 mt-2">
                {verdict?.title ??
                  (connected
                    ? 'Iniciando diagnóstico Dual-Hop...'
                    : 'Sin conexión a red local')}
              </h2>
              <p className="text-xs text-slate-300 mt-1.5 leading-relaxed">
                {verdict?.detail ??
                  'Verificando adaptador principal y puerta de enlace mediante Win32 API.'}
              </p>
            </div>

            <div className="shrink-0 text-right">
              <div className="text-[10px] font-mono-telemetry uppercase tracking-wider text-slate-400">
                Salud de Red
              </div>
              <div
                className={`font-mono-telemetry text-3xl font-bold mt-0.5 ${theme.text}`}
              >
                {verdict ? verdict.score : '—'}
                <span className="text-xs font-normal text-slate-400">/100</span>
              </div>
            </div>
          </div>

          <div className="mt-4 pt-3 border-t border-white/[0.08] flex flex-wrap items-center justify-between gap-2 text-[11px] font-mono-telemetry text-slate-400">
            <span>
              Salto 1 (Router):{' '}
              <strong className="text-cyan-300">
                {routerStats && routerStats.sent > 0
                  ? `${routerStats.avgMs.toFixed(1)} ms`
                  : '—'}
              </strong>
            </span>
            <span>
              Salto 2 (Internet):{' '}
              <strong className="text-emerald-300">
                {internetStats && internetStats.sent > 0
                  ? `${internetStats.avgMs.toFixed(1)} ms`
                  : '—'}
              </strong>
            </span>
            <span>
              Delta ISP:{' '}
              <strong className="text-slate-200">+{ispDeltaAvg.toFixed(1)} ms</strong>
            </span>
          </div>
        </div>

        {/* Active Interface Card */}
        <div className="lg:col-span-5 np-card rounded-xl p-5 flex flex-col justify-between gap-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2 text-xs text-slate-400">
                {networkSummary?.linkType === 'wifi' ? (
                  <Wifi className="h-4 w-4 text-cyan-400 shrink-0" />
                ) : (
                  <Cable className="h-4 w-4 text-emerald-400 shrink-0" />
                )}
                <span className="truncate font-medium text-slate-200">
                  {networkSummary?.adapterName || 'Adaptador Principal'}
                </span>
              </div>
              <p className="text-[11px] text-slate-400 mt-1 truncate">
                {networkSummary?.primary?.description || 'Sin adaptador activo'}
              </p>
            </div>
            {networkSummary?.linkSpeedMbps ? (
              <span className="font-mono-telemetry text-[11px] px-2 py-0.5 rounded bg-white/[0.05] border border-white/[0.08] text-slate-200 shrink-0">
                {networkSummary.linkSpeedMbps} Mbps
              </span>
            ) : null}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <PrivacyValue
              label="IP Local"
              value={networkSummary?.localIp || '—'}
              privacyMode={privacyMode}
            />
            <PrivacyValue
              label="MAC Adaptador"
              value={networkSummary?.adapterMac || '—'}
              privacyMode={privacyMode}
            />
          </div>
        </div>
      </div>

      {/* 2. Dual-Hop Visual Topology Bar */}
      <section className="np-card rounded-xl p-4">
        <div className="grid grid-cols-1 md:grid-cols-5 items-center gap-3">
          <div className="rounded-lg bg-[#090D16] border border-white/[0.07] px-3.5 py-2.5 flex items-center gap-3">
            <Laptop className="h-5 w-5 text-slate-300 shrink-0" />
            <div className="min-w-0">
              <div className="text-xs font-semibold text-slate-100">Tu PC</div>
              <div className="font-mono-telemetry text-[11px] text-slate-400 truncate">
                {networkSummary?.localIp || '—'}
              </div>
            </div>
          </div>

          <div className="flex flex-col items-center justify-center text-center px-2">
            <div className="flex items-center gap-1.5 text-[11px] font-mono-telemetry text-cyan-300">
              <span>
                {routerStats && routerStats.sent > 0
                  ? `${routerStats.currentMs.toFixed(1)} ms`
                  : '—'}
              </span>
              <ArrowRight className="h-3.5 w-3.5 text-cyan-400" />
            </div>
            <div className="text-[10px] font-mono-telemetry text-slate-400 mt-0.5">
              Jit {routerStats ? routerStats.jitterMs.toFixed(1) : '0.0'} ms · Loss{' '}
              {routerStats ? routerStats.lossPct.toFixed(1) : '0.0'}%
            </div>
          </div>

          <div className="rounded-lg bg-[#090D16] border border-cyan-500/25 px-3.5 py-2.5 flex items-center gap-3">
            <Router className="h-5 w-5 text-cyan-400 shrink-0" />
            <div className="min-w-0">
              <div className="text-xs font-semibold text-slate-100">
                Router / Gateway
              </div>
              <div className="font-mono-telemetry text-[11px] text-cyan-300 truncate">
                {networkSummary?.gatewayIp || '—'}
              </div>
            </div>
          </div>

          <div className="flex flex-col items-center justify-center text-center px-2">
            <div className="flex items-center gap-1.5 text-[11px] font-mono-telemetry text-emerald-300">
              <span>
                {internetStats && internetStats.sent > 0
                  ? `${internetStats.currentMs.toFixed(1)} ms`
                  : '—'}
              </span>
              <ArrowRight className="h-3.5 w-3.5 text-emerald-400" />
            </div>
            <div className="text-[10px] font-mono-telemetry text-slate-400 mt-0.5">
              Delta +{ispDeltaAvg.toFixed(1)} ms · Loss{' '}
              {internetStats ? internetStats.lossPct.toFixed(1) : '0.0'}%
            </div>
          </div>

          <div className="rounded-lg bg-[#090D16] border border-emerald-500/25 px-3.5 py-2.5 flex items-center gap-3">
            <Globe className="h-5 w-5 text-emerald-400 shrink-0" />
            <div className="min-w-0">
              <div className="text-xs font-semibold text-slate-100">Internet</div>
              <div className="font-mono-telemetry text-[11px] text-emerald-300 truncate">
                {dualHop?.internetTarget || selectedTarget}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 3. Live Dual-Hop Chart Card */}
      <section className="np-card rounded-xl p-5 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold text-slate-100">
              Monitor Dual-Hop en Tiempo Real (500 ms)
            </h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Ping ICMP nativo sin permisos de Administrador. Separá al instante lag interno de problemas del proveedor.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <select
              value={selectedTarget}
              onChange={(e) => onChangeTarget(e.target.value)}
              aria-label="Destino externo de ping"
              className="rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-1.5 text-xs font-mono-telemetry text-slate-200 focus:outline-none focus:border-cyan-500/50 cursor-pointer"
            >
              {TARGET_PRESETS.map((preset) => (
                <option key={preset.value} value={preset.value}>
                  {preset.label}
                </option>
              ))}
            </select>

            <button
              type="button"
              onClick={onToggleMonitor}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors cursor-pointer ${
                dualHop?.running
                  ? 'bg-amber-500/12 border-amber-500/35 text-amber-300 hover:bg-amber-500/20'
                  : 'bg-emerald-500/12 border-emerald-500/35 text-emerald-300 hover:bg-emerald-500/20'
              }`}
            >
              {dualHop?.running ? (
                <>
                  <Pause className="h-3.5 w-3.5" />
                  Pausar
                </>
              ) : (
                <>
                  <Play className="h-3.5 w-3.5" />
                  Reanudar
                </>
              )}
            </button>

            <button
              type="button"
              onClick={onResetMonitor}
              title="Reiniciar muestras e incidentes"
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 border border-white/[0.08] transition-colors cursor-pointer"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Limpiar
            </button>
          </div>
        </div>

        <DualHopChart
          samples={dualHop?.samples ?? []}
          routerTarget={dualHop?.routerTarget || networkSummary?.gatewayIp || ''}
          internetTarget={dualHop?.internetTarget || selectedTarget}
        />

        {/* Compact 2-column Hop Telemetry Summary */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {[
            {
              title: 'Salto 1 · Router Local',
              accent: 'text-cyan-300',
              stats: routerStats,
            },
            {
              title: `Salto 2 · Internet (${dualHop?.internetTarget || selectedTarget})`,
              accent: 'text-emerald-300',
              stats: internetStats,
            },
          ].map(({ title, accent, stats }) => (
            <div
              key={title}
              className="rounded-lg bg-[#090D16] border border-white/[0.06] px-4 py-3"
            >
              <div className={`text-xs font-semibold ${accent} mb-2`}>{title}</div>
              <div className="grid grid-cols-5 gap-2 text-center font-mono-telemetry">
                <div>
                  <div className="text-[10px] text-slate-400">ACTUAL</div>
                  <div className="text-xs font-semibold text-slate-100 mt-0.5">
                    {stats && stats.lastOk ? `${stats.currentMs.toFixed(1)}ms` : '—'}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] text-slate-400">PROM</div>
                  <div className="text-xs font-semibold text-slate-100 mt-0.5">
                    {stats && stats.sent > 0 ? `${stats.avgMs.toFixed(1)}ms` : '—'}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] text-slate-400">P95</div>
                  <div className="text-xs font-semibold text-slate-100 mt-0.5">
                    {stats && stats.sent > 0 ? `${stats.p95Ms.toFixed(1)}ms` : '—'}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] text-slate-400">JITTER</div>
                  <div className="text-xs font-semibold text-slate-100 mt-0.5">
                    {stats && stats.sent > 0 ? `${stats.jitterMs.toFixed(1)}ms` : '—'}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] text-slate-400">LOSS</div>
                  <div
                    className={`text-xs font-semibold mt-0.5 ${
                      stats && stats.lossPct > 0 ? 'text-rose-300' : 'text-slate-100'
                    }`}
                  >
                    {stats ? `${stats.lossPct.toFixed(1)}%` : '0.0%'}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* 4. Bufferbloat Test Card */}
      <BufferbloatCard
        running={bloatRunning}
        progress={bloatProgress}
        result={bloatResult}
        error={bloatError}
        connected={connected}
        onRun={onRunBufferbloat}
        onCancel={onCancelBufferbloat}
      />

      {/* 4.5 CGNAT / Double NAT / Path MTU / IPv6 Audit Card */}
      <section className="np-card rounded-xl p-5 space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-cyan-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                {lang === 'en'
                  ? 'ISP & Gaming Topology Audit (CGNAT · Path MTU · IPv6)'
                  : 'Auditoría ISP & Gaming (CGNAT · Path MTU · IPv6)'}
              </h2>
            </div>
            <p className="text-xs text-slate-400 mt-0.5">
              {networkAudit?.overallNote ||
                'Detecta en segundos si estás detrás de CGNAT, Doble NAT, MTU fragmentado (PPPoE) o sin IPv6 real.'}
            </p>
          </div>
          <button
            type="button"
            onClick={onRunNetworkAudit}
            disabled={!connected || auditRunning}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 text-cyan-200 border border-cyan-500/35 cursor-pointer"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${auditRunning ? 'animate-spin' : ''}`} />
            {auditRunning ? 'Auditando...' : 'Auditar NAT / MTU / IPv6'}
          </button>
        </div>

        {networkAudit?.completedAt ? (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3.5 space-y-1">
              <div className="text-[11px] uppercase tracking-wider text-slate-400">
                Tipo de NAT / Salida
              </div>
              <div
                className={`text-sm font-semibold ${
                  networkAudit.nat.kind === 'public'
                    ? 'text-emerald-300'
                    : networkAudit.nat.kind === 'unknown'
                    ? 'text-slate-200'
                    : 'text-amber-300'
                }`}
              >
                {networkAudit.nat.label}
              </div>
              <p className="text-[11px] text-slate-400 leading-snug">
                {networkAudit.nat.detail}
              </p>
            </div>

            <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3.5 space-y-1">
              <div className="text-[11px] uppercase tracking-wider text-slate-400">
                Path MTU (DF Bit)
              </div>
              <div
                className={`font-mono-telemetry text-sm font-semibold ${
                  networkAudit.mtu.pathMtu === 1500
                    ? 'text-emerald-300'
                    : networkAudit.mtu.pathMtu > 0
                    ? 'text-amber-300'
                    : 'text-slate-300'
                }`}
              >
                {networkAudit.mtu.pathMtu > 0
                  ? `${networkAudit.mtu.pathMtu} / 1500 bytes`
                  : 'No medido'}
              </div>
              <p className="text-[11px] text-slate-400 leading-snug">
                {networkAudit.mtu.detail}
              </p>
            </div>

            <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3.5 space-y-1">
              <div className="text-[11px] uppercase tracking-wider text-slate-400">
                Conectividad IPv6
              </div>
              <div
                className={`text-sm font-semibold ${
                  networkAudit.ipv6.reachable
                    ? 'text-emerald-300'
                    : networkAudit.ipv6.hasGlobalIpv6
                    ? 'text-amber-300'
                    : 'text-slate-300'
                }`}
              >
                {networkAudit.ipv6.reachable
                  ? `Activo (${networkAudit.ipv6.rttMs.toFixed(1)} ms)`
                  : networkAudit.ipv6.hasGlobalIpv6
                  ? 'IPv6 sin salida'
                  : 'Solo IPv4'}
              </div>
              <p className="text-[11px] text-slate-400 leading-snug">
                {networkAudit.ipv6.detail}
              </p>
            </div>
          </div>
        ) : (
          <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-3.5 px-4 text-xs text-slate-400">
            Ejecutá <strong>Auditar NAT / MTU / IPv6</strong> para diagnosticar problemas de NAT estricta en juegos o fragmentación de paquetes.
          </div>
        )}
      </section>

      {/* 5. Incident Log (Session + 30d History) + Gaming Regions */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        {/* Incident Log */}
        <section className="lg:col-span-5 np-card rounded-xl p-5 flex flex-col space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-amber-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                Registro de Microcortes
              </h2>
            </div>

            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setHistoryTab('session')}
                className={`px-2 py-1 rounded text-[11px] cursor-pointer ${
                  historyTab === 'session'
                    ? 'bg-cyan-500/20 text-cyan-200 border border-cyan-500/35'
                    : 'bg-white/[0.04] text-slate-400 border border-white/[0.07]'
                }`}
              >
                Sesión ({incidents.length})
              </button>
              <button
                type="button"
                onClick={() => setHistoryTab('history30')}
                className={`px-2 py-1 rounded text-[11px] cursor-pointer ${
                  historyTab === 'history30'
                    ? 'bg-cyan-500/20 text-cyan-200 border border-cyan-500/35'
                    : 'bg-white/[0.04] text-slate-400 border border-white/[0.07]'
                }`}
              >
                30 días ({incidentHistory?.totalStored ?? 0})
              </button>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            {incidents.length > 0 && historyTab === 'session' && (
              <button
                type="button"
                onClick={handleCopyIncidents}
                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] bg-white/[0.05] hover:bg-white/[0.1] text-slate-200 border border-white/[0.08] cursor-pointer"
              >
                {copiedLog ? (
                  <>
                    <Check className="h-3 w-3 text-emerald-400" />
                    Copiado
                  </>
                ) : (
                  <>
                    <Copy className="h-3 w-3" />
                    Copiar
                  </>
                )}
              </button>
            )}
            <button
              type="button"
              onClick={async () => {
                await onExportISPReport('csv');
                setExportNote('CSV listo para reclamo ISP');
              }}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] bg-emerald-500/12 hover:bg-emerald-500/20 text-emerald-300 border border-emerald-500/25 cursor-pointer"
            >
              <Download className="h-3 w-3" />
              CSV ISP
            </button>
            <button
              type="button"
              onClick={async () => {
                await onExportISPReport('html');
                setExportNote('HTML técnico listo para soporte ISP');
              }}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] bg-cyan-500/12 hover:bg-cyan-500/20 text-cyan-300 border border-cyan-500/25 cursor-pointer"
            >
              <Download className="h-3 w-3" />
              HTML ISP
            </button>
            {historyTab === 'history30' && (incidentHistory?.totalStored ?? 0) > 0 && (
              <button
                type="button"
                onClick={onClearHistory}
                className="inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/25 cursor-pointer ml-auto"
                title="Limpiar histórico local"
              >
                <Trash2 className="h-3 w-3" />
                Limpiar
              </button>
            )}
          </div>

          {exportNote && (
            <div className="text-[11px] text-emerald-300">{exportNote}</div>
          )}

          {historyTab === 'session' ? (
            incidents.length === 0 ? (
              <div className="flex-1 rounded-lg bg-[#090D16] border border-white/[0.06] p-5 flex items-center justify-center text-center text-xs text-slate-400">
                Sin microcortes ni picos fuertes registrados en esta sesión.
              </div>
            ) : (
              <div className="space-y-2 max-h-56 overflow-y-auto np-scroll pr-1">
                {incidents.map((inc) => (
                  <div
                    key={inc.id}
                    className={`rounded-lg px-3 py-2 border text-xs ${
                      inc.severity === 'rose'
                        ? 'bg-rose-500/[0.07] border-rose-500/25 text-rose-200'
                        : 'bg-amber-500/[0.07] border-amber-500/25 text-amber-200'
                    }`}
                  >
                    <div className="flex items-center justify-between font-mono-telemetry text-[10px] opacity-85">
                      <span>{inc.timeLabel}</span>
                      <span>{inc.hop === 'router' ? 'WI-FI / LAN' : 'ISP / INTERNET'}</span>
                    </div>
                    <div className="mt-0.5">{inc.message}</div>
                  </div>
                ))}
              </div>
            )
          ) : (incidentHistory?.incidents?.length ?? 0) === 0 ? (
            <div className="flex-1 rounded-lg bg-[#090D16] border border-white/[0.06] p-5 flex items-center justify-center text-center text-xs text-slate-400">
              Sin incidentes guardados aún en los últimos 30 días. Cada microcorte o pico detectado se guarda automáticamente en local.
            </div>
          ) : (
            <div className="space-y-2 max-h-56 overflow-y-auto np-scroll pr-1">
              {incidentHistory!.incidents.slice(0, 40).map((inc) => (
                <div
                  key={inc.id}
                  className={`rounded-lg px-3 py-2 border text-xs ${
                    inc.severity === 'rose'
                      ? 'bg-rose-500/[0.07] border-rose-500/25 text-rose-200'
                      : 'bg-amber-500/[0.07] border-amber-500/25 text-amber-200'
                  }`}
                >
                  <div className="flex items-center justify-between font-mono-telemetry text-[10px] opacity-85">
                    <span>{inc.timeIso.slice(0, 19).replace('T', ' ')}</span>
                    <span>{inc.hop === 'router' ? 'LAN' : 'ISP'}</span>
                  </div>
                  <div className="mt-0.5">{inc.message}</div>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Gaming Regions Benchmark */}
        <section className="lg:col-span-7 np-card rounded-xl p-5 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <div className="flex items-center gap-2">
                <Gamepad2 className="h-4 w-4 text-cyan-400" />
                <h2 className="text-sm font-semibold text-slate-100">
                  Servidores de Juegos & Regiones (LATAM / NA / EU)
                </h2>
              </div>
              <p className="text-xs text-slate-400 mt-0.5">
                Mide latencia real hacia Discord, Steam, Brasil, USA y Europa antes de jugar.
              </p>
            </div>
            <button
              type="button"
              onClick={onRunRegions}
              disabled={!connected || regionRunning}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 text-cyan-200 border border-cyan-500/35 cursor-pointer"
            >
              <Play className={`h-3.5 w-3.5 ${regionRunning ? 'animate-pulse' : ''}`} />
              {regionRunning ? 'Midiendo...' : 'Medir Regiones'}
            </button>
          </div>

          {!regionResult ? (
            <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-5 text-center text-xs text-slate-400">
              Tocá <strong>Medir Regiones</strong> para detectar el mejor servidor regional y verificar el ruteo de tu ISP.
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {regionResult.results.map((r) => (
                <div
                  key={r.id}
                  className="rounded-lg bg-[#090D16] border border-white/[0.06] px-3 py-2 flex items-center justify-between gap-2"
                >
                  <div className="min-w-0">
                    <div className="text-xs font-medium text-slate-100 truncate">
                      {r.name}
                    </div>
                    <div className="text-[10px] text-slate-400 truncate">
                      {r.region}
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0 font-mono-telemetry">
                    <span className="text-[10px] text-slate-400">
                      {r.reachable ? `jit ${r.jitterMs.toFixed(1)}ms` : ''}
                    </span>
                    <span
                      className={`text-xs px-2 py-0.5 rounded border ${regionGradeStyle(
                        r.grade,
                      )}`}
                    >
                      {r.reachable ? `${r.avgMs.toFixed(0)} ms` : 'Timeout'}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>

      {/* 6. Traceroute & Continuous MTR + Pro Desktop Preferences */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        <section className="lg:col-span-7 np-card rounded-xl p-5 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <div className="flex items-center gap-2">
                <Route className="h-4 w-4 text-emerald-400" />
                <h2 className="text-sm font-semibold text-slate-100">
                  MTR Continuo & Traceroute hacia {selectedTarget}
                </h2>
              </div>
              <p className="text-xs text-slate-400 mt-0.5">
                Mide pérdida y jitter por salto en tiempo real estilo WinMTR (sin Admin; los 2 primeros saltos se censuran en Modo Discord).
              </p>
            </div>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={onToggleMTR}
                disabled={!connected}
                className={`inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium border cursor-pointer disabled:opacity-45 ${
                  mtrSnapshot?.running
                    ? 'bg-rose-500/15 hover:bg-rose-500/25 text-rose-200 border-rose-500/35'
                    : 'bg-cyan-500/15 hover:bg-cyan-500/25 text-cyan-200 border-cyan-500/35'
                }`}
              >
                {mtrSnapshot?.running ? (
                  <>
                    <Pause className="h-3.5 w-3.5" />
                    Detener MTR
                  </>
                ) : (
                  <>
                    <Play className="h-3.5 w-3.5" />
                    MTR Continuo
                  </>
                )}
              </button>

              <button
                type="button"
                onClick={onRunTraceroute}
                disabled={!connected || traceRunning}
                className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-emerald-500/15 hover:bg-emerald-500/25 disabled:opacity-45 text-emerald-200 border border-emerald-500/35 cursor-pointer"
              >
                <Route className={`h-3.5 w-3.5 ${traceRunning ? 'animate-pulse' : ''}`} />
                {traceRunning ? 'Trazando...' : 'Pasada Única'}
              </button>
            </div>
          </div>

          {mtrSnapshot && mtrSnapshot.hops?.length > 0 ? (
            <div className="space-y-2">
              <div className="rounded-lg bg-cyan-500/[0.06] border border-cyan-500/25 px-3 py-2 text-xs text-cyan-200 flex items-center justify-between gap-2">
                <span>{mtrSnapshot.summary}</span>
                <span className="font-mono-telemetry text-[11px] shrink-0">
                  {mtrSnapshot.cycles} ciclos
                </span>
              </div>
              <div className="overflow-x-auto max-h-56 overflow-y-auto np-scroll">
                <table className="w-full text-left font-mono-telemetry text-[11px]">
                  <thead className="text-slate-400 border-b border-white/[0.08]">
                    <tr>
                      <th className="py-1.5 pr-2">#</th>
                      <th className="py-1.5 pr-2">Salto IP</th>
                      <th className="py-1.5 pr-2 text-right">Env</th>
                      <th className="py-1.5 pr-2 text-right">Pérd.</th>
                      <th className="py-1.5 pr-2 text-right">Últ</th>
                      <th className="py-1.5 pr-2 text-right">Prom</th>
                      <th className="py-1.5 pr-2 text-right">Peor</th>
                      <th className="py-1.5 text-right">Jit</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-white/[0.05]">
                    {mtrSnapshot.hops.map((h) => {
                      const isBad =
                        mtrSnapshot.worstLossTtl > 0 && h.ttl === mtrSnapshot.worstLossTtl && h.lossPct >= 5;
                      return (
                        <tr
                          key={h.ttl}
                          className={
                            isBad
                              ? 'bg-rose-500/10 text-rose-200'
                              : h.isTarget
                              ? 'bg-emerald-500/10 text-emerald-200'
                              : 'text-slate-300'
                          }
                        >
                          <td className="py-1.5 pr-2 text-slate-400">{h.ttl}</td>
                          <td className="py-1.5 pr-2">{h.ip}</td>
                          <td className="py-1.5 pr-2 text-right">{h.sent}</td>
                          <td className="py-1.5 pr-2 text-right">{h.lossPct.toFixed(1)}%</td>
                          <td className="py-1.5 pr-2 text-right">{h.lastMs > 0 ? `${h.lastMs.toFixed(1)}` : '—'}</td>
                          <td className="py-1.5 pr-2 text-right">{h.avgMs > 0 ? `${h.avgMs.toFixed(1)}` : '—'}</td>
                          <td className="py-1.5 pr-2 text-right">{h.worstMs > 0 ? `${h.worstMs.toFixed(1)}` : '—'}</td>
                          <td className="py-1.5 text-right">{h.jitterMs > 0 ? `${h.jitterMs.toFixed(1)}` : '—'}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          ) : traceResult ? (
            <div className="space-y-2.5">
              <div className="rounded-lg bg-white/[0.03] border border-white/[0.07] px-3 py-2 text-xs text-slate-200">
                {traceResult.summary}
              </div>
              <div className="space-y-1.5 max-h-52 overflow-y-auto np-scroll pr-1 font-mono-telemetry text-xs">
                {traceResult.hops.map((h) => {
                  const isWorst =
                    traceResult.worstHopTtl > 0 && h.ttl === traceResult.worstHopTtl;
                  return (
                    <div
                      key={h.ttl}
                      className={`rounded-md px-3 py-1.5 border flex items-center justify-between gap-2 ${
                        isWorst
                          ? 'bg-amber-500/10 border-amber-500/35 text-amber-200'
                          : h.isTarget
                          ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-200'
                          : 'bg-[#090D16] border-white/[0.06] text-slate-300'
                      }`}
                    >
                      <span className="w-12 shrink-0 text-slate-400">#{h.ttl}</span>
                      <span className="flex-1 truncate">{h.ip}</span>
                      <span className="shrink-0">
                        {h.timedOut
                          ? '* (sin respuesta)'
                          : `${h.rttMs.toFixed(1)} ms${
                              isWorst ? ` (+${h.deltaMs.toFixed(1)} ms salto)` : ''
                            }`}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          ) : (
            <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-5 text-center text-xs text-slate-400">
              Usá <strong>MTR Continuo</strong> para monitorear pérdida de paquetes salto por salto o <strong>Pasada Única</strong> para un trazado rápido.
            </div>
          )}
        </section>

        {/* System Tray, Autostart & Opt-in Updates */}
        <section className="lg:col-span-5 np-card rounded-xl p-5 flex flex-col justify-between space-y-3">
          <div>
            <div className="flex items-center gap-2">
              <MonitorDown className="h-4 w-4 text-cyan-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                Preferencias Pro & Actualizaciones
              </h2>
            </div>
            <p className="text-xs text-slate-400 mt-1">
              Bandeja Windows, inicio automático sin Admin y búsqueda manual de nuevas versiones (0 consultas automáticas).
            </p>
          </div>

          <div className="space-y-2">
            <label className="rounded-lg bg-[#090D16] border border-white/[0.07] p-2.5 flex items-start gap-2.5 cursor-pointer">
              <input
                type="checkbox"
                checked={minimizeToTray}
                onChange={(e) =>
                  onChangeTrayPrefs(e.target.checked, backgroundRadarAlert)
                }
                className="mt-0.5"
              />
              <div>
                <div className="text-xs font-medium text-slate-100">
                  Minimizar a la bandeja al cerrar la ventana
                </div>
                <div className="text-[11px] text-slate-400">
                  Clic derecho en el ícono del reloj → Salir para cerrar por completo.
                </div>
              </div>
            </label>

            <label className="rounded-lg bg-[#090D16] border border-white/[0.07] p-2.5 flex items-start gap-2.5 cursor-pointer">
              <input
                type="checkbox"
                checked={backgroundRadarAlert}
                onChange={(e) =>
                  onChangeTrayPrefs(minimizeToTray, e.target.checked)
                }
                className="mt-0.5"
              />
              <div>
                <div className="text-xs font-medium text-slate-100 flex items-center gap-1.5">
                  <BellRing className="h-3.5 w-3.5 text-amber-400" />
                  <span>Alerta de intrusos nuevos (cada 5 min)</span>
                </div>
                <div className="text-[11px] text-slate-400">
                  Notifica en Windows si se conecta un equipo desconocido nuevo.
                </div>
              </div>
            </label>

            <label className="rounded-lg bg-[#090D16] border border-white/[0.07] p-2.5 flex items-start gap-2.5 cursor-pointer">
              <input
                type="checkbox"
                checked={startWithWindows}
                onChange={(e) => onToggleAutostart(e.target.checked)}
                className="mt-0.5"
              />
              <div>
                <div className="text-xs font-medium text-slate-100">
                  Iniciar con Windows (minimizado en bandeja)
                </div>
                <div className="text-[11px] text-slate-400">
                  Usa HKCU Run sin permisos de Administrador.
                </div>
              </div>
            </label>

            <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-2.5 space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-xs text-slate-300">
                  Versión actual: <strong>v{appInfo?.version ?? '1.2.0'}</strong>
                </span>
                <div className="flex items-center gap-1.5">
                  <button
                    type="button"
                    onClick={onOpenPatchNotes}
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-medium bg-white/[0.05] hover:bg-white/[0.1] text-slate-200 border border-white/[0.1] cursor-pointer"
                  >
                    <Sparkles className="h-3 w-3 text-cyan-300" />
                    Patch Notes
                  </button>
                  <button
                    type="button"
                    onClick={onCheckUpdates}
                    disabled={checkingUpdates}
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-medium bg-white/[0.05] hover:bg-white/[0.1] text-cyan-300 border border-white/[0.1] cursor-pointer disabled:opacity-50"
                  >
                    <RefreshCw className={`h-3 w-3 ${checkingUpdates ? 'animate-spin' : ''}`} />
                    {checkingUpdates ? 'Consultando...' : 'Buscar actualizaciones'}
                  </button>
                </div>
              </div>
              {updateResult && (
                <div className="text-[11px] text-slate-300 flex flex-wrap items-center justify-between gap-2">
                  <span>
                    {updateResult.status === 'update_available'
                      ? `Disponible v${updateResult.latestVersion}`
                      : updateResult.status === 'up_to_date'
                      ? `Al día (v${updateResult.latestVersion})`
                      : updateResult.error || 'Sin releases publicados aún.'}
                  </span>
                  {updateResult.releaseUrl && (
                    <button
                      type="button"
                      onClick={() => onOpenExternalURL(updateResult.releaseUrl)}
                      className="inline-flex items-center gap-1 text-cyan-300 hover:underline cursor-pointer"
                    >
                      <span>Ver release</span>
                      <ExternalLink className="h-3 w-3" />
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>
        </section>
      </div>

      {/* 7. Compact Local Security & 6-Phase Roadmap Footer */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <div className="np-card rounded-xl p-3.5 flex items-center gap-3">
          <KeyRound
            className={`h-4 w-4 shrink-0 ${
              appInfo?.vaultReady ? 'text-emerald-400' : 'text-amber-400'
            }`}
          />
          <div className="min-w-0">
            <div className="text-xs font-medium text-slate-200 flex items-center gap-1.5">
              {appInfo?.vaultReady ? (
                <CheckCircle2 className="h-3.5 w-3.5 text-emerald-400 shrink-0" />
              ) : (
                <ShieldAlert className="h-3.5 w-3.5 text-amber-400 shrink-0" />
              )}
              <span className="truncate">Windows Credential Manager</span>
            </div>
            <div className="text-[11px] text-slate-400 truncate">
              {appInfo?.vaultReady
                ? 'Bóveda DPAPI lista'
                : appInfo?.vaultError || 'Verificando'}
            </div>
          </div>
        </div>

        <div className="np-card rounded-xl p-3.5 flex items-center gap-3">
          <ShieldCheck className="h-4 w-4 text-emerald-400 shrink-0" />
          <div className="min-w-0">
            <div className="text-xs font-medium text-slate-200">
              {appInfo?.zeroTelemetry ? '0% Telemetría Activa' : 'Local'}
            </div>
            <div className="font-mono-telemetry text-[11px] text-slate-400 truncate">
              {appInfo ? `NetPulse v${appInfo.version} · CGO=0` : 'Inicializando...'}
            </div>
          </div>
        </div>

        <div className="np-card rounded-xl p-3.5 flex items-center gap-3">
          <FolderLock className="h-4 w-4 text-cyan-400 shrink-0" />
          <div className="min-w-0">
            <div className="text-xs font-medium text-slate-200">Almacén Local</div>
            <div
              className="font-mono-telemetry text-[11px] text-slate-400 truncate"
              title={appInfo?.configPath || ''}
            >
              {appInfo?.configPath || '%APPDATA%\\NetPulse\\config.json'}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
