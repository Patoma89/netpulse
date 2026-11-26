import { Activity, Gauge, Play, Square, AlertCircle } from 'lucide-react';
import type { diagnostics } from '../../wailsjs/go/models';

export interface BufferbloatProgress {
  phase: 'idle' | 'download' | 'upload' | 'done' | 'cancelled' | 'error';
  progressPct: number;
  currentPingMs: number;
  currentMbps: number;
  message: string;
}

interface BufferbloatCardProps {
  running: boolean;
  progress: BufferbloatProgress | null;
  result: diagnostics.BufferbloatResult | null;
  error: string | null;
  connected: boolean;
  onRun: () => void;
  onCancel: () => void;
}

function gradeBadgeStyles(severity?: string): string {
  switch (severity) {
    case 'emerald':
      return 'bg-emerald-500/15 text-emerald-300 border-emerald-500/35';
    case 'cyan':
      return 'bg-cyan-500/15 text-cyan-300 border-cyan-500/35';
    case 'amber':
      return 'bg-amber-500/15 text-amber-300 border-amber-500/35';
    case 'rose':
      return 'bg-rose-500/15 text-rose-300 border-rose-500/35';
    default:
      return 'bg-white/[0.06] text-slate-300 border-white/[0.1]';
  }
}

export function BufferbloatCard({
  running,
  progress,
  result,
  error,
  connected,
  onRun,
  onCancel,
}: BufferbloatCardProps) {
  const phaseLabels: Record<string, string> = {
    idle: 'Fase 1/3 · Reposo',
    download: 'Fase 2/3 · Carga Descarga',
    upload: 'Fase 3/3 · Carga Subida',
    done: 'Completado',
    cancelled: 'Cancelado',
    error: 'Error',
  };

  return (
    <section className="np-card rounded-xl p-5">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div
            className={`h-12 w-12 rounded-xl border flex items-center justify-center font-mono-telemetry text-xl font-bold shrink-0 ${gradeBadgeStyles(
              result?.severity,
            )}`}
          >
            {result?.grade ?? '—'}
          </div>
          <div>
            <div className="flex items-center gap-2">
              <Gauge className="h-4 w-4 text-cyan-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                Test de Bufferbloat (Latencia bajo Carga)
              </h2>
            </div>
            <p className="text-xs text-slate-400 mt-0.5">
              {result
                ? result.summary
                : 'Mide cuánto se dispara tu ping cuando alguien descarga o sube archivos pesados.'}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {running ? (
            <button
              type="button"
              onClick={onCancel}
              className="inline-flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-medium bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/35 transition-colors cursor-pointer"
            >
              <Square className="h-3.5 w-3.5" />
              Cancelar
            </button>
          ) : (
            <button
              type="button"
              onClick={onRun}
              disabled={!connected}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 disabled:cursor-not-allowed text-cyan-300 border border-cyan-500/35 transition-colors cursor-pointer"
            >
              <Play className="h-3.5 w-3.5" />
              Ejecutar Test (~14s)
            </button>
          )}
        </div>
      </div>

      {/* Live progress bar while running */}
      {running && progress && (
        <div className="mt-4 rounded-lg bg-[#090D16] border border-white/[0.06] p-3.5 space-y-2.5">
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
            <span className="inline-flex items-center gap-2 text-cyan-300 font-medium">
              <Activity className="h-3.5 w-3.5 animate-pulse" />
              {phaseLabels[progress.phase] || progress.phase}: {progress.message}
            </span>
            <div className="flex items-center gap-4 font-mono-telemetry text-slate-300">
              <span>
                Ping:{' '}
                <strong className="text-slate-100">
                  {progress.currentPingMs > 0
                    ? `${progress.currentPingMs.toFixed(1)} ms`
                    : '—'}
                </strong>
              </span>
              {progress.phase !== 'idle' && (
                <span>
                  Tráfico:{' '}
                  <strong className="text-emerald-300">
                    {progress.currentMbps.toFixed(1)} Mbps
                  </strong>
                </span>
              )}
              <span>{progress.progressPct}%</span>
            </div>
          </div>

          <div className="h-2 w-full rounded-full bg-white/[0.06] overflow-hidden">
            <div
              className="h-full bg-gradient-to-r from-cyan-500 to-emerald-400 transition-all duration-200"
              style={{ width: `${Math.min(100, Math.max(4, progress.progressPct))}%` }}
            />
          </div>
        </div>
      )}

      {/* Error banner */}
      {error && !running && (
        <div className="mt-4 rounded-lg bg-rose-500/10 border border-rose-500/30 px-3.5 py-2.5 flex items-center gap-2.5 text-xs text-rose-300">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Results grid */}
      {result && !running && (
        <div className="mt-4 space-y-3">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3.5">
              <div className="text-[11px] uppercase tracking-wider text-slate-400">
                Reposo (Base)
              </div>
              <div className="font-mono-telemetry text-lg font-semibold text-slate-100 mt-1">
                {result.idleMedianMs.toFixed(1)} ms
              </div>
              <div className="font-mono-telemetry text-[11px] text-slate-400 mt-0.5">
                Promedio: {result.idleAvgMs.toFixed(1)} ms · Destino {result.target}
              </div>
            </div>

            <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3.5">
              <div className="flex items-center justify-between text-[11px] uppercase tracking-wider text-slate-400">
                <span>Carga Descarga</span>
                <span className="font-mono-telemetry text-cyan-300">
                  +{result.downloadDeltaMs.toFixed(1)} ms
                </span>
              </div>
              <div className="font-mono-telemetry text-lg font-semibold text-slate-100 mt-1">
                {result.downloadMedianMs.toFixed(1)} ms
              </div>
              <div className="font-mono-telemetry text-[11px] text-slate-400 mt-0.5">
                {result.downloadMbps.toFixed(1)} Mbps · {result.downloadLossPct.toFixed(1)}% pérdida
              </div>
            </div>

            <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3.5">
              <div className="flex items-center justify-between text-[11px] uppercase tracking-wider text-slate-400">
                <span>Carga Subida</span>
                {!result.uploadSkipped && (
                  <span className="font-mono-telemetry text-emerald-300">
                    +{result.uploadDeltaMs.toFixed(1)} ms
                  </span>
                )}
              </div>
              {result.uploadSkipped ? (
                <>
                  <div className="font-mono-telemetry text-sm font-medium text-amber-300 mt-1.5">
                    Omitida (bloqueo firewall/ISP)
                  </div>
                  <div className="text-[11px] text-slate-400 mt-0.5">
                    Calificación basada en descarga
                  </div>
                </>
              ) : (
                <>
                  <div className="font-mono-telemetry text-lg font-semibold text-slate-100 mt-1">
                    {result.uploadMedianMs.toFixed(1)} ms
                  </div>
                  <div className="font-mono-telemetry text-[11px] text-slate-400 mt-0.5">
                    {result.uploadMbps.toFixed(1)} Mbps · {result.uploadLossPct.toFixed(1)}% pérdida
                  </div>
                </>
              )}
            </div>
          </div>

          <div className="rounded-lg bg-white/[0.03] border border-white/[0.06] px-3.5 py-2.5 text-xs text-slate-300 leading-relaxed">
            <span className="font-semibold text-slate-100">Recomendación: </span>
            {result.recommendation}
          </div>
        </div>
      )}
    </section>
  );
}
