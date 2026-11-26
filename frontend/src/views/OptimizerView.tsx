import { useState } from 'react';
import {
  AlertCircle,
  CheckCircle2,
  Eraser,
  GlobeLock,
  Play,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
  Trash2,
  Zap,
} from 'lucide-react';
import type { netinfo, optimizer } from '../../wailsjs/go/models';

interface OptimizerViewProps {
  networkSummary: netinfo.NetworkSummary | null;
  dnsBenchmark: optimizer.DNSBenchmarkResult | null;
  dnsRunning: boolean;
  dnsApplyingId: string | null;
  dnsFeedback: { ok: boolean; message: string } | null;
  upnpResult: optimizer.UPnPScanResult | null;
  upnpScanning: boolean;
  upnpDeletingKey: string | null;
  upnpError: string | null;
  onRunDNSBenchmark: () => void;
  onApplyDNS: (id: string, primary: string, secondary: string) => void;
  onResetDNSDHCP: () => void;
  onFlushDNSCache: () => void;
  onScanUPnP: () => void;
  onDeleteUPnP: (protocol: string, externalPort: number) => void;
}

function riskBadgeStyle(risk: string): string {
  switch (risk) {
    case 'high':
      return 'bg-rose-500/15 text-rose-300 border-rose-500/35';
    case 'medium':
      return 'bg-amber-500/15 text-amber-300 border-amber-500/35';
    default:
      return 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30';
  }
}

function riskLabel(risk: string): string {
  switch (risk) {
    case 'high':
      return 'Riesgo Alto';
    case 'medium':
      return 'Riesgo Medio';
    default:
      return 'Temporal';
  }
}

export function OptimizerView({
  networkSummary,
  dnsBenchmark,
  dnsRunning,
  dnsApplyingId,
  dnsFeedback,
  upnpResult,
  upnpScanning,
  upnpDeletingKey,
  upnpError,
  onRunDNSBenchmark,
  onApplyDNS,
  onResetDNSDHCP,
  onFlushDNSCache,
  onScanUPnP,
  onDeleteUPnP,
}: OptimizerViewProps) {
  const [confirmReset, setConfirmReset] = useState(false);

  const connected = networkSummary?.connected ?? false;
  const activeDNSList = networkSummary?.primary?.dnsServers ?? [];
  const results = dnsBenchmark?.results ?? [];
  const maxMedian = results.reduce(
    (acc, r) => (r.reachable && r.medianMs > acc ? r.medianMs : acc),
    25,
  );

  const mappings = upnpResult?.mappings ?? [];

  return (
    <div className="space-y-6">
      {/* 1. DNS Turbo Section */}
      <section className="np-card rounded-xl p-5 space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <Zap className="h-4 w-4 text-cyan-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                DNS Turbo · Benchmark Paralelo y Cambio en 1 Clic
              </h2>
            </div>
            <p className="text-xs text-slate-400 mt-1">
              Mide latencia UDP real contra los mejores resolvers y aplica el más rápido con elevación UAC puntual y rollback a DHCP.
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs font-mono-telemetry">
              <span className="text-slate-400">DNS Actual ({networkSummary?.adapterName || 'Adaptador'}):</span>
              {activeDNSList.length > 0 ? (
                activeDNSList.map((ip) => (
                  <span
                    key={ip}
                    className="px-2 py-0.5 rounded bg-white/[0.05] border border-white/[0.08] text-cyan-300"
                  >
                    {ip}
                  </span>
                ))
              ) : (
                <span className="text-slate-400">Automático / DHCP</span>
              )}
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={onFlushDNSCache}
              className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-slate-200 border border-white/[0.09] transition-colors cursor-pointer"
            >
              <Eraser className="h-3.5 w-3.5 text-slate-300" />
              Limpiar Caché DNS
            </button>

            {confirmReset ? (
              <div className="inline-flex items-center gap-1.5">
                <button
                  type="button"
                  onClick={() => {
                    setConfirmReset(false);
                    onResetDNSDHCP();
                  }}
                  disabled={dnsApplyingId !== null}
                  className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 border border-amber-500/40 cursor-pointer"
                >
                  Confirmar Rollback DHCP (UAC)
                </button>
                <button
                  type="button"
                  onClick={() => setConfirmReset(false)}
                  className="px-2.5 py-2 rounded-lg text-xs text-slate-400 hover:text-slate-200 bg-white/[0.04] border border-white/[0.08] cursor-pointer"
                >
                  Cancelar
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setConfirmReset(true)}
                disabled={!connected || dnsApplyingId !== null}
                className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] disabled:opacity-45 text-slate-200 border border-white/[0.09] transition-colors cursor-pointer"
              >
                <RotateCcw className="h-3.5 w-3.5 text-amber-300" />
                Restaurar DHCP
              </button>
            )}

            <button
              type="button"
              onClick={onRunDNSBenchmark}
              disabled={!connected || dnsRunning}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 text-cyan-300 border border-cyan-500/35 transition-colors cursor-pointer"
            >
              <Play className={`h-3.5 w-3.5 ${dnsRunning ? 'animate-pulse' : ''}`} />
              {dnsRunning ? 'Midiendo DNS...' : 'Ejecutar Benchmark DNS'}
            </button>
          </div>
        </div>

        {dnsFeedback && (
          <div
            className={`rounded-lg px-3.5 py-2.5 border flex items-center gap-2.5 text-xs ${
              dnsFeedback.ok
                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
            }`}
          >
            {dnsFeedback.ok ? (
              <CheckCircle2 className="h-4 w-4 shrink-0" />
            ) : (
              <AlertCircle className="h-4 w-4 shrink-0" />
            )}
            <span>{dnsFeedback.message}</span>
          </div>
        )}

        {results.length === 0 ? (
          <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-8 text-center space-y-2">
            <p className="text-xs text-slate-300">
              Tocá <strong>Ejecutar Benchmark DNS</strong> para comparar tu DNS actual contra Cloudflare, Google, Quad9, AdGuard y OpenDNS.
            </p>
            <p className="text-[11px] text-slate-500">
              Aplicar un DNS solicita autorización de Windows (UAC) solo en ese instante.
            </p>
          </div>
        ) : (
          <div className="space-y-2.5">
            {results.map((r) => {
              const p = r.provider;
              const barPct = r.reachable
                ? Math.min(100, Math.max(8, (r.medianMs / maxMedian) * 100))
                : 100;
              const isApplying = dnsApplyingId === p.id;
              const isCustomCurrent = p.id === 'current-isp';

              return (
                <div
                  key={p.id}
                  className={`rounded-lg bg-[#090D16] border p-3.5 flex flex-wrap items-center justify-between gap-4 ${
                    r.recommended
                      ? 'border-emerald-500/35 bg-emerald-500/[0.03]'
                      : 'border-white/[0.07]'
                  }`}
                >
                  <div className="min-w-[220px] flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-semibold text-slate-100">
                        {p.name}
                      </span>
                      {r.recommended && (
                        <span className="inline-flex items-center gap-1 font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                          <Sparkles className="h-3 w-3" />
                          MÁS RÁPIDO
                        </span>
                      )}
                      {p.isCurrent && (
                        <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-cyan-500/15 text-cyan-300 border border-cyan-500/30">
                          ACTIVO
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5">{p.tagline}</div>
                    <div className="font-mono-telemetry text-[11px] text-slate-400 mt-1">
                      {p.primary}
                      {p.secondary ? ` · ${p.secondary}` : ''}
                    </div>
                  </div>

                  <div className="flex items-center gap-4 ml-auto">
                    <div className="w-36 hidden sm:block">
                      <div className="h-1.5 w-full rounded-full bg-white/[0.07] overflow-hidden">
                        <div
                          className={`h-full rounded-full ${
                            !r.reachable
                              ? 'bg-rose-500/60'
                              : r.recommended
                              ? 'bg-emerald-400'
                              : 'bg-cyan-400'
                          }`}
                          style={{ width: `${barPct}%` }}
                        />
                      </div>
                      <div className="flex justify-between font-mono-telemetry text-[10px] text-slate-400 mt-1">
                        <span>{r.successCount}/{r.totalQueries} OK</span>
                        <span>Jit {r.jitterMs.toFixed(1)}ms</span>
                      </div>
                    </div>

                    <div className="w-20 text-right font-mono-telemetry">
                      <div className="text-sm font-semibold text-slate-100">
                        {r.reachable ? `${r.medianMs.toFixed(1)} ms` : 'Timeout'}
                      </div>
                      <div className="text-[10px] text-slate-400">
                        {r.reachable ? `Mín ${r.minMs.toFixed(1)}ms` : 'Sin respuesta'}
                      </div>
                    </div>

                    {!isCustomCurrent && (
                      <button
                        type="button"
                        onClick={() => onApplyDNS(p.id, p.primary, p.secondary)}
                        disabled={!connected || !r.reachable || dnsApplyingId !== null}
                        className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors cursor-pointer disabled:opacity-45 ${
                          r.recommended
                            ? 'bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-300 border-emerald-500/35'
                            : 'bg-white/[0.05] hover:bg-white/[0.1] text-slate-200 border-white/[0.1]'
                        }`}
                      >
                        {isApplying ? 'Aplicando...' : p.isCurrent ? 'Reaplicar' : 'Aplicar (UAC)'}
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      {/* 2. UPnP Security Audit Section */}
      <section className="np-card rounded-xl p-5 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <GlobeLock className="h-4 w-4 text-cyan-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                Auditoría de Seguridad UPnP (Puertos Abiertos en tu Router)
              </h2>
            </div>
            <p className="text-xs text-slate-400 mt-1">
              Detecta qué puertos abrieron juegos, torrents o aplicaciones en tu módem sin avisarte, y cerrá los riesgosos o huérfanos en 1 clic.
            </p>
          </div>

          <button
            type="button"
            onClick={onScanUPnP}
            disabled={!connected || upnpScanning}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 text-cyan-300 border border-cyan-500/35 transition-colors cursor-pointer"
          >
            <ShieldAlert className={`h-3.5 w-3.5 ${upnpScanning ? 'animate-pulse' : ''}`} />
            {upnpScanning ? 'Auditando UPnP...' : 'Escanear Puertos UPnP'}
          </button>
        </div>

        {upnpError && (
          <div className="rounded-lg bg-rose-500/10 border border-rose-500/30 px-3.5 py-2.5 flex items-center gap-2.5 text-xs text-rose-300">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{upnpError}</span>
          </div>
        )}

        {!upnpResult ? (
          <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-7 text-center text-xs text-slate-400">
            Presioná <strong>Escanear Puertos UPnP</strong> para auditar la tabla de redirección IGD de tu router.
          </div>
        ) : upnpResult.status === 'upnp_disabled' ? (
          <div className="rounded-lg bg-emerald-500/[0.06] border border-emerald-500/30 p-4 flex items-center gap-3">
            <ShieldCheck className="h-5 w-5 text-emerald-400 shrink-0" />
            <div>
              <div className="text-sm font-medium text-emerald-300">
                Sin exposición UPnP detectada
              </div>
              <div className="text-xs text-slate-300 mt-0.5">{upnpResult.message}</div>
            </div>
          </div>
        ) : upnpResult.status === 'error' ? (
          <div className="rounded-lg bg-amber-500/[0.06] border border-amber-500/30 p-4 text-xs text-amber-200">
            {upnpResult.message}
          </div>
        ) : mappings.length === 0 ? (
          <div className="rounded-lg bg-emerald-500/[0.06] border border-emerald-500/30 p-4 flex items-center gap-3">
            <ShieldCheck className="h-5 w-5 text-emerald-400 shrink-0" />
            <div>
              <div className="text-sm font-medium text-emerald-300">
                {upnpResult.deviceName || 'Gateway'} · 0 puertos abiertos por UPnP
              </div>
              <div className="text-xs text-slate-300 mt-0.5">{upnpResult.message}</div>
            </div>
          </div>
        ) : (
          <div className="space-y-2.5">
            <div className="flex items-center justify-between text-xs text-slate-400 font-mono-telemetry px-1">
              <span>
                Gateway: {upnpResult.deviceName || upnpResult.gatewayIp} · {mappings.length} reglas
              </span>
              {upnpResult.highRiskCount > 0 && (
                <span className="text-rose-300 font-semibold">
                  {upnpResult.highRiskCount} de riesgo alto
                </span>
              )}
            </div>

            {mappings.map((m) => {
              const delKey = `${m.protocol}:${m.externalPort}`;
              const isDeleting = upnpDeletingKey === delKey;
              return (
                <div
                  key={`${delKey}-${m.index}`}
                  className="rounded-lg bg-[#090D16] border border-white/[0.07] px-4 py-3 flex flex-wrap items-center justify-between gap-3"
                >
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono-telemetry text-sm font-semibold text-slate-100">
                        {m.protocol} :{m.externalPort} → {m.internalClient}:{m.internalPort}
                      </span>
                      <span
                        className={`font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded border ${riskBadgeStyle(
                          m.risk,
                        )}`}
                      >
                        {riskLabel(m.risk)}
                      </span>
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5">
                      {m.description || 'Sin descripción'} · {m.riskReason}
                    </div>
                  </div>

                  <button
                    type="button"
                    onClick={() => onDeleteUPnP(m.protocol, m.externalPort)}
                    disabled={isDeleting}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-rose-500/12 hover:bg-rose-500/22 text-rose-300 border border-rose-500/30 transition-colors cursor-pointer"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    {isDeleting ? 'Cerrando...' : 'Cerrar Puerto'}
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
