import { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  ChevronDown,
  ChevronRight,
  Cpu,
  Lock,
  RefreshCw,
  Search,
  ShieldCheck,
} from 'lucide-react';
import type { processes } from '../../wailsjs/go/models';
import type { UILang } from '../i18n';

interface ProcessesViewProps {
  snapshot: processes.ProcessSnapshot | null;
  loading: boolean;
  privacyMode: boolean;
  lang: UILang;
  onRefresh: () => void;
}

export function ProcessesView({
  snapshot,
  loading,
  privacyMode,
  lang,
  onRefresh,
}: ProcessesViewProps) {
  const [search, setSearch] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [expandedPID, setExpandedPID] = useState<number | null>(null);
  const [onlyEstablished, setOnlyEstablished] = useState(false);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => {
      onRefresh();
    }, 3000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoRefresh]);

  const list = useMemo(() => {
    const q = search.trim().toLowerCase();
    const procs = snapshot?.processes ?? [];
    return procs.filter((p) => {
      if (onlyEstablished && p.establishedCount === 0) return false;
      if (!q) return true;
      return (
        p.name.toLowerCase().includes(q) ||
        String(p.pid).includes(q) ||
        p.exePath.toLowerCase().includes(q)
      );
    });
  }, [snapshot, search, onlyEstablished]);

  const en = lang === 'en';

  return (
    <div className="space-y-5">
      {/* Header Card */}
      <section className="np-card rounded-xl p-5 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <Cpu className="h-4 w-4 text-cyan-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                {en
                  ? 'Per-Process Network Monitor (Driverless Win32)'
                  : 'Monitor de Conexiones por Proceso (Sin Drivers)'}
              </h2>
              {snapshot?.capturedAt && (
                <span className="font-mono-telemetry text-[11px] px-2 py-0.5 rounded bg-white/[0.05] border border-white/[0.08] text-slate-300">
                  {snapshot.capturedAt}
                </span>
              )}
            </div>
            <p className="text-xs text-slate-400 mt-1">
              {en
                ? 'Inspects active TCP/UDP sockets and process I/O via Win32 IP Helper without kernel drivers or Admin rights.'
                : 'Lista qué programas tienen conexiones TCP/UDP activas y actividad I/O en tiempo real mediante Win32 sin instalar Npcap ni drivers.'}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <label className="inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs bg-[#090D16] border border-white/[0.08] text-slate-300 cursor-pointer">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
              />
              <span>{en ? 'Auto 3s' : 'Auto 3s'}</span>
            </label>

            <button
              type="button"
              onClick={onRefresh}
              disabled={loading}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-50 text-cyan-300 border border-cyan-500/35 cursor-pointer"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              <span>{en ? 'Refresh (F5)' : 'Actualizar (F5)'}</span>
            </button>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3">
            <div className="text-[11px] uppercase tracking-wider text-slate-400">
              {en ? 'Processes with Sockets' : 'Procesos con Red'}
            </div>
            <div className="font-mono-telemetry text-xl font-bold text-slate-100 mt-0.5">
              {snapshot?.totalProcesses ?? 0}
            </div>
          </div>
          <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3">
            <div className="text-[11px] uppercase tracking-wider text-slate-400">
              {en ? 'Established TCP' : 'TCP Activas (Established)'}
            </div>
            <div className="font-mono-telemetry text-xl font-bold text-emerald-300 mt-0.5">
              {snapshot?.establishedTotal ?? 0}
            </div>
          </div>
          <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3">
            <div className="text-[11px] uppercase tracking-wider text-slate-400">
              {en ? 'Total TCP + UDP' : 'Sockets Totales (TCP + UDP)'}
            </div>
            <div className="font-mono-telemetry text-xl font-bold text-cyan-300 mt-0.5">
              {snapshot?.totalConnections ?? 0}
            </div>
          </div>
        </div>

        {privacyMode && (
          <div className="rounded-lg bg-emerald-500/[0.06] border border-emerald-500/25 px-3.5 py-2 flex items-center gap-2 text-xs text-emerald-300">
            <ShieldCheck className="h-4 w-4 shrink-0" />
            <span>
              {en
                ? 'Discord Mode active: remote IPs and local Windows user paths are masked.'
                : 'Modo Discord activo: las IPs remotas y rutas de usuario de Windows se censuran automáticamente.'}
            </span>
          </div>
        )}
      </section>

      {/* Filter + Table */}
      <section className="np-card rounded-xl p-5 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="relative flex-1 min-w-[220px]">
            <Search className="h-3.5 w-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={
                en
                  ? 'Filter by app name, PID, or path...'
                  : 'Buscar proceso (.exe), PID o ruta...'
              }
              className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] pl-9 pr-3 py-2 text-xs text-slate-100 placeholder:text-slate-500 focus:outline-none focus:border-cyan-500/50"
            />
          </div>

          <label className="inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs bg-[#090D16] border border-white/[0.08] text-slate-300 cursor-pointer">
            <input
              type="checkbox"
              checked={onlyEstablished}
              onChange={(e) => setOnlyEstablished(e.target.checked)}
            />
            <span>
              {en ? 'Only Established TCP' : 'Solo conexiones TCP activas'}
            </span>
          </label>
        </div>

        {list.length === 0 ? (
          <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-10 text-center text-xs text-slate-400">
            {en
              ? 'No processes match the current filter.'
              : 'No hay procesos que coincidan con el filtro actual.'}
          </div>
        ) : (
          <div className="space-y-2">
            {list.map((p) => {
              const isExpanded = expandedPID === p.pid;
              const highActivity = p.ioKbPerSec >= 50 || p.establishedCount >= 10;
              return (
                <div
                  key={`${p.pid}-${p.name}`}
                  className="rounded-lg bg-[#090D16] border border-white/[0.07] overflow-hidden"
                >
                  <button
                    type="button"
                    onClick={() => setExpandedPID(isExpanded ? null : p.pid)}
                    className="w-full p-3 flex flex-wrap items-center justify-between gap-3 text-left hover:bg-white/[0.02] cursor-pointer"
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      {isExpanded ? (
                        <ChevronDown className="h-4 w-4 text-cyan-400 shrink-0" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-slate-400 shrink-0" />
                      )}
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-semibold text-slate-100 truncate">
                            {p.name}
                          </span>
                          <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                            PID {p.pid}
                          </span>
                          {highActivity && (
                            <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-300 border border-amber-500/30 inline-flex items-center gap-1">
                              <Activity className="h-2.5 w-2.5" />
                              {en ? 'Active' : 'Alto'}
                            </span>
                          )}
                        </div>
                        {p.exePath && (
                          <div className="font-mono-telemetry text-[11px] text-slate-500 truncate mt-0.5">
                            {p.exePath}
                          </div>
                        )}
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 font-mono-telemetry text-xs ml-auto">
                      <span className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-300 border border-emerald-500/25">
                        {p.establishedCount} {en ? 'active' : 'activas'}
                      </span>
                      <span className="px-2 py-0.5 rounded bg-white/[0.04] text-slate-300 border border-white/[0.08]">
                        {p.tcpCount} TCP · {p.udpCount} UDP
                      </span>
                      <span className="px-2 py-0.5 rounded bg-cyan-500/10 text-cyan-300 border border-cyan-500/25 min-w-[82px] text-right">
                        {p.ioKbPerSec.toFixed(1)} KB/s
                      </span>
                    </div>
                  </button>

                  {isExpanded && (
                    <div className="px-4 pb-3 pt-2 border-t border-white/[0.06] bg-black/20">
                      {p.remoteSample?.length > 0 ? (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-1.5 font-mono-telemetry text-[11px]">
                          {p.remoteSample.map((ep, idx) => (
                            <div
                              key={`${ep.protocol}-${ep.localPort}-${ep.remoteIp}-${ep.remotePort}-${idx}`}
                              className="rounded px-2.5 py-1.5 bg-white/[0.03] border border-white/[0.06] flex items-center justify-between gap-2 text-slate-300"
                            >
                              <span className="flex items-center gap-1.5 truncate">
                                <span className="text-cyan-300">{ep.protocol}</span>
                                <span>:{ep.localPort}</span>
                                {ep.remoteIp && (
                                  <>
                                    <span>→</span>
                                    {privacyMode && (
                                      <Lock className="h-3 w-3 text-emerald-400 shrink-0" />
                                    )}
                                    <span className="truncate">
                                      {ep.remoteIp}
                                      {ep.remotePort > 0 ? `:${ep.remotePort}` : ''}
                                    </span>
                                  </>
                                )}
                              </span>
                              <span className="text-[10px] text-slate-400 shrink-0">
                                {ep.state}
                              </span>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div className="text-[11px] text-slate-400">
                          {en
                            ? 'Only local listening ports.'
                            : 'Solo puertos locales en escucha.'}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
