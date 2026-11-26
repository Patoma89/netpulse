import { useMemo, useState } from 'react';
import type { diagnostics } from '../../wailsjs/go/models';

interface DualHopChartProps {
  samples: diagnostics.DualHopSample[];
  routerTarget: string;
  internetTarget: string;
}

const CHART_WIDTH = 1000;
const CHART_HEIGHT = 200;
const PAD_TOP = 10;
const PAD_RIGHT = 4;
const PAD_BOTTOM = 10;
const PLOT_W = CHART_WIDTH - PAD_RIGHT;
const PLOT_H = CHART_HEIGHT - PAD_TOP - PAD_BOTTOM;
const MAX_WINDOW = 120;

// Pick clean multiples of 4 so the 5 grid ticks (0%, 25%, 50%, 75%, 100%) are always crisp round integers.
function computeNiceMaxY(samples: diagnostics.DualHopSample[]): number {
  let maxVal = 0;
  for (const s of samples) {
    if (s.routerOk && s.routerMs > maxVal) maxVal = s.routerMs;
    if (s.internetOk && s.internetMs > maxVal) maxVal = s.internetMs;
  }
  const raw = maxVal * 1.25;
  if (raw <= 12) return 12; // 0, 3, 6, 9, 12
  if (raw <= 20) return 20; // 0, 5, 10, 15, 20
  if (raw <= 40) return 40; // 0, 10, 20, 30, 40
  if (raw <= 100) return Math.ceil(raw / 20) * 20; // 60, 80, 100
  if (raw <= 200) return Math.ceil(raw / 40) * 40; // 120, 160, 200
  return Math.ceil(raw / 100) * 100;
}

function buildSegmentedPaths(
  samples: diagnostics.DualHopSample[],
  getX: (idx: number) => number,
  getY: (ms: number) => number,
  pick: (s: diagnostics.DualHopSample) => { ok: boolean; ms: number },
): { linePath: string; areaPath: string } {
  const lineParts: string[] = [];
  const areaParts: string[] = [];
  const baselineY = PAD_TOP + PLOT_H;

  let segStartIdx = -1;
  let prevX = 0;
  let lastPointX = 0;

  for (let i = 0; i < samples.length; i++) {
    const { ok, ms } = pick(samples[i]);
    if (ok && ms >= 0) {
      const x = getX(i);
      const y = getY(ms);
      if (segStartIdx === -1) {
        segStartIdx = i;
        lineParts.push(`M ${x.toFixed(1)} ${y.toFixed(1)}`);
        areaParts.push(
          `M ${x.toFixed(1)} ${baselineY.toFixed(1)} L ${x.toFixed(1)} ${y.toFixed(1)}`,
        );
        prevX = x;
      } else {
        lineParts.push(`L ${x.toFixed(1)} ${y.toFixed(1)}`);
        areaParts.push(`L ${x.toFixed(1)} ${y.toFixed(1)}`);
      }
      lastPointX = x;
    } else if (segStartIdx !== -1) {
      areaParts.push(`L ${lastPointX.toFixed(1)} ${baselineY.toFixed(1)} Z`);
      if (lastPointX === prevX) {
        lineParts.push(
          `L ${(lastPointX + 0.5).toFixed(1)} ${lineParts[lineParts.length - 1].split(' ')[2]}`,
        );
      }
      segStartIdx = -1;
    }
  }

  if (segStartIdx !== -1) {
    areaParts.push(`L ${lastPointX.toFixed(1)} ${baselineY.toFixed(1)} Z`);
  }

  return {
    linePath: lineParts.join(' '),
    areaPath: areaParts.join(' '),
  };
}

export function DualHopChart({
  samples,
  routerTarget,
  internetTarget,
}: DualHopChartProps) {
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);

  const maxY = useMemo(() => computeNiceMaxY(samples), [samples]);
  const slotCount = Math.max(samples.length, 2);
  const startOffset = Math.max(0, MAX_WINDOW - samples.length);

  const getX = (idx: number) => {
    const denom = Math.max(MAX_WINDOW - 1, slotCount - 1);
    return ((startOffset + idx) / denom) * PLOT_W;
  };

  const getY = (ms: number) => {
    const clamped = Math.max(0, Math.min(ms, maxY));
    return PAD_TOP + PLOT_H - (clamped / maxY) * PLOT_H;
  };

  const routerPaths = useMemo(
    () =>
      buildSegmentedPaths(samples, getX, getY, (s) => ({
        ok: s.routerOk,
        ms: s.routerMs,
      })),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [samples, maxY],
  );

  const internetPaths = useMemo(
    () =>
      buildSegmentedPaths(samples, getX, getY, (s) => ({
        ok: s.internetOk,
        ms: s.internetMs,
      })),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [samples, maxY],
  );

  const yTicks = [0, 0.25, 0.5, 0.75, 1];
  const hovered =
    hoverIdx !== null && hoverIdx >= 0 && hoverIdx < samples.length
      ? samples[hoverIdx]
      : null;

  function handleMouseMove(e: React.MouseEvent<SVGSVGElement>) {
    if (samples.length === 0) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const relX = ((e.clientX - rect.left) / rect.width) * CHART_WIDTH;
    if (relX < 0 || relX > CHART_WIDTH) {
      setHoverIdx(null);
      return;
    }

    let bestIdx = 0;
    let bestDist = Infinity;
    for (let i = 0; i < samples.length; i++) {
      const dist = Math.abs(getX(i) - relX);
      if (dist < bestDist) {
        bestDist = dist;
        bestIdx = i;
      }
    }
    setHoverIdx(bestIdx);
  }

  return (
    <div className="relative rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
      <div className="flex flex-wrap items-center justify-between gap-2 mb-3 px-1">
        <div className="flex flex-wrap items-center gap-4 text-[11px] font-mono-telemetry">
          <span className="inline-flex items-center gap-1.5 text-cyan-300">
            <span className="h-2 w-2 rounded-full bg-cyan-400" />
            Salto 1 · Router ({routerTarget || '—'})
          </span>
          <span className="inline-flex items-center gap-1.5 text-emerald-300">
            <span className="h-2 w-2 rounded-full bg-emerald-400" />
            Salto 2 · Internet ({internetTarget || '1.1.1.1'})
          </span>
          <span className="inline-flex items-center gap-1.5 text-rose-300">
            <span className="h-2.5 w-0.5 bg-rose-400" />
            Pérdida
          </span>
        </div>

        <span className="text-[11px] font-mono-telemetry text-slate-400">
          Ventana: 60s ({samples.length}/{MAX_WINDOW} muestras)
        </span>
      </div>

      <div className="relative w-full h-56 flex items-stretch">
        {samples.length === 0 ? (
          <div className="w-full h-full flex items-center justify-center text-xs text-slate-400 font-mono-telemetry">
            Esperando primeras muestras ICMP Dual-Hop...
          </div>
        ) : (
          <>
            {/* Crisp unstretched HTML Y-axis labels */}
            <div className="w-12 shrink-0 relative select-none pr-2">
              {yTicks.map((frac) => {
                const val = Math.round(maxY * frac);
                const topPct =
                  ((PAD_TOP + PLOT_H - frac * PLOT_H) / CHART_HEIGHT) * 100;
                return (
                  <div
                    key={frac}
                    style={{ top: `${topPct}%` }}
                    className="absolute right-2 -translate-y-1/2 text-right leading-none whitespace-nowrap"
                  >
                    <span className="font-mono-telemetry text-[11px] font-medium text-slate-300">
                      {val}
                    </span>
                    <span className="font-mono-telemetry text-[9px] text-slate-500 ml-0.5">
                      ms
                    </span>
                  </div>
                );
              })}
            </div>

            {/* Plot Canvas */}
            <div className="relative flex-1 h-full">
              <svg
                viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`}
                preserveAspectRatio="none"
                className="w-full h-full overflow-visible select-none"
                onMouseMove={handleMouseMove}
                onMouseLeave={() => setHoverIdx(null)}
              >
                <defs>
                  <linearGradient id="npCyanGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#06B6D4" stopOpacity="0.24" />
                    <stop offset="100%" stopColor="#06B6D4" stopOpacity="0.0" />
                  </linearGradient>
                  <linearGradient id="npEmeraldGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#10B981" stopOpacity="0.22" />
                    <stop offset="100%" stopColor="#10B981" stopOpacity="0.0" />
                  </linearGradient>
                </defs>

                {yTicks.map((frac) => {
                  const y = PAD_TOP + PLOT_H - frac * PLOT_H;
                  return (
                    <line
                      key={frac}
                      x1={0}
                      y1={y}
                      x2={PLOT_W}
                      y2={y}
                      stroke="rgba(148,163,184,0.14)"
                      strokeDasharray={frac === 0 ? undefined : '4 4'}
                      strokeWidth="1"
                      vectorEffect="non-scaling-stroke"
                    />
                  );
                })}

                {internetPaths.areaPath && (
                  <path d={internetPaths.areaPath} fill="url(#npEmeraldGrad)" />
                )}
                {routerPaths.areaPath && (
                  <path d={routerPaths.areaPath} fill="url(#npCyanGrad)" />
                )}

                {samples.map((s, idx) => {
                  if (s.routerOk && s.internetOk) return null;
                  const x = getX(idx);
                  return (
                    <line
                      key={`loss-${s.seq}-${idx}`}
                      x1={x}
                      y1={PAD_TOP}
                      x2={x}
                      y2={PAD_TOP + PLOT_H}
                      stroke="#F43F5E"
                      strokeOpacity="0.55"
                      strokeWidth="1.5"
                      vectorEffect="non-scaling-stroke"
                    />
                  );
                })}

                {internetPaths.linePath && (
                  <path
                    d={internetPaths.linePath}
                    fill="none"
                    stroke="#10B981"
                    strokeWidth="2"
                    strokeLinejoin="round"
                    strokeLinecap="round"
                    vectorEffect="non-scaling-stroke"
                  />
                )}
                {routerPaths.linePath && (
                  <path
                    d={routerPaths.linePath}
                    fill="none"
                    stroke="#06B6D4"
                    strokeWidth="2"
                    strokeLinejoin="round"
                    strokeLinecap="round"
                    vectorEffect="non-scaling-stroke"
                  />
                )}

                {hovered && hoverIdx !== null && (
                  <line
                    x1={getX(hoverIdx)}
                    y1={PAD_TOP}
                    x2={getX(hoverIdx)}
                    y2={PAD_TOP + PLOT_H}
                    stroke="rgba(148,163,184,0.45)"
                    strokeWidth="1"
                    strokeDasharray="3 3"
                    vectorEffect="non-scaling-stroke"
                  />
                )}
              </svg>

              {/* Round HTML Hover Dots (never warped by preserveAspectRatio) */}
              {hovered && hoverIdx !== null && (
                <>
                  {hovered.routerOk && (
                    <div
                      style={{
                        left: `${(getX(hoverIdx) / CHART_WIDTH) * 100}%`,
                        top: `${(getY(hovered.routerMs) / CHART_HEIGHT) * 100}%`,
                      }}
                      className="pointer-events-none absolute h-2.5 w-2.5 rounded-full bg-cyan-400 border-2 border-[#090D16] shadow -translate-x-1/2 -translate-y-1/2"
                    />
                  )}
                  {hovered.internetOk && (
                    <div
                      style={{
                        left: `${(getX(hoverIdx) / CHART_WIDTH) * 100}%`,
                        top: `${(getY(hovered.internetMs) / CHART_HEIGHT) * 100}%`,
                      }}
                      className="pointer-events-none absolute h-2.5 w-2.5 rounded-full bg-emerald-400 border-2 border-[#090D16] shadow -translate-x-1/2 -translate-y-1/2"
                    />
                  )}

                  <div
                    className="pointer-events-none absolute top-2 z-10 rounded-lg bg-[#111827]/95 border border-white/[0.12] px-3 py-2 shadow-xl text-[11px] font-mono-telemetry"
                    style={{
                      left:
                        getX(hoverIdx) > CHART_WIDTH * 0.65
                          ? undefined
                          : `${Math.min(72, (getX(hoverIdx) / CHART_WIDTH) * 100 + 2)}%`,
                      right:
                        getX(hoverIdx) > CHART_WIDTH * 0.65
                          ? `${Math.max(4, 100 - (getX(hoverIdx) / CHART_WIDTH) * 100 + 2)}%`
                          : undefined,
                    }}
                  >
                    <div className="text-slate-400 mb-1">
                      Muestra #{hovered.seq}
                    </div>
                    <div className="text-cyan-300">
                      Router:{' '}
                      {hovered.routerOk
                        ? `${hovered.routerMs.toFixed(1)} ms`
                        : 'PÉRDIDA'}
                    </div>
                    <div className="text-emerald-300">
                      Internet:{' '}
                      {hovered.internetOk
                        ? `${hovered.internetMs.toFixed(1)} ms`
                        : 'PÉRDIDA'}
                    </div>
                    {hovered.routerOk && hovered.internetOk && (
                      <div className="text-slate-300 mt-0.5 pt-0.5 border-t border-white/[0.08]">
                        Delta ISP: +
                        {Math.max(
                          0,
                          hovered.internetMs - hovered.routerMs,
                        ).toFixed(1)}{' '}
                        ms
                      </div>
                    )}
                  </div>
                </>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
