import { useMemo, useState } from 'react';
import type { wifi } from '../../wailsjs/go/models';

interface SpectrumChartProps {
  band: '2.4GHz' | '5GHz' | '6GHz';
  accessPoints: wifi.AccessPoint[];
  recommendedChannel: number;
  currentChannel: number;
}

const CHART_W = 1000;
const CHART_H = 200;
const PAD_TOP = 10;
const PAD_BOTTOM = 4;
const PLOT_W = CHART_W;
const PLOT_H = CHART_H - PAD_TOP - PAD_BOTTOM;

const RSSI_MIN = -100;
const RSSI_MAX = -30;

const CHANNELS_24 = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13];
const CHANNELS_5_TICKS = [36, 48, 52, 64, 100, 116, 132, 144, 149, 165];
const CHANNELS_6_TICKS = [5, 21, 37, 53, 69, 85, 101, 117, 133, 149, 165, 181, 197, 213];

function freqMHzFromChannel(band: '2.4GHz' | '5GHz' | '6GHz', ch: number): number {
  if (band === '2.4GHz') {
    return 2412 + (ch - 1) * 5;
  }
  if (band === '6GHz') {
    return 5950 + ch * 5;
  }
  return 5000 + ch * 5;
}

function apColor(ap: wifi.AccessPoint): { stroke: string; fill: string } {
  if (ap.isCurrent) {
    return { stroke: '#06B6D4', fill: 'rgba(6, 182, 212, 0.22)' };
  }
  if (ap.rssi >= -62) {
    return { stroke: '#F43F5E', fill: 'rgba(244, 63, 94, 0.14)' };
  }
  if (ap.rssi >= -76) {
    return { stroke: '#F59E0B', fill: 'rgba(245, 158, 11, 0.12)' };
  }
  return { stroke: '#818CF8', fill: 'rgba(129, 140, 248, 0.10)' };
}

export function SpectrumChart({
  band,
  accessPoints,
  recommendedChannel,
  currentChannel,
}: SpectrumChartProps) {
  const [hoveredBSSID, setHoveredBSSID] = useState<string | null>(null);

  const bandAPs = useMemo(
    () => accessPoints.filter((ap) => ap.band === band),
    [accessPoints, band],
  );

  const minFreq = band === '2.4GHz' ? 2400 : band === '6GHz' ? 5935 : 5160;
  const maxFreq = band === '2.4GHz' ? 2485 : band === '6GHz' ? 7125 : 5845;

  const toX = (freqMHz: number) => {
    const clamped = Math.max(minFreq, Math.min(maxFreq, freqMHz));
    return ((clamped - minFreq) / (maxFreq - minFreq)) * PLOT_W;
  };

  const toY = (rssi: number) => {
    const clamped = Math.max(RSSI_MIN, Math.min(RSSI_MAX, rssi));
    const frac = (clamped - RSSI_MIN) / (RSSI_MAX - RSSI_MIN);
    return PAD_TOP + PLOT_H - frac * PLOT_H;
  };

  const baselineY = PAD_TOP + PLOT_H;
  const rssiTicks = [-95, -80, -65, -50, -35];
  const channelTicks =
    band === '2.4GHz'
      ? CHANNELS_24
      : band === '6GHz'
      ? CHANNELS_6_TICKS
      : CHANNELS_5_TICKS;

  const hoveredAP = useMemo(
    () => bandAPs.find((ap) => ap.bssid === hoveredBSSID) ?? null,
    [bandAPs, hoveredBSSID],
  );

  const curves = useMemo(() => {
    return bandAPs.map((ap) => {
      const centerCh = ap.centerChannel > 0 ? ap.centerChannel : ap.channel;
      const centerFreq =
        freqMHzFromChannel(band, centerCh) || ap.frequencyMHz || 2412;
      const width = Math.max(20, ap.widthMHz || 20);
      const halfSpan = width / 2 + (band === '2.4GHz' ? 1 : 2);
      const startF = centerFreq - halfSpan;
      const endF = centerFreq + halfSpan;
      const peakY = toY(ap.rssi);

      const steps = 28;
      const pts: string[] = [];
      for (let i = 0; i <= steps; i++) {
        const t = i / steps;
        const f = startF + t * (endF - startF);
        const bell = 0.5 * (1 - Math.cos(2 * Math.PI * t));
        const y = baselineY - bell * (baselineY - peakY);
        pts.push(`${i === 0 ? 'M' : 'L'} ${toX(f).toFixed(1)} ${y.toFixed(1)}`);
      }
      const linePath = pts.join(' ');
      const areaPath = `${linePath} L ${toX(endF).toFixed(1)} ${baselineY.toFixed(
        1,
      )} L ${toX(startF).toFixed(1)} ${baselineY.toFixed(1)} Z`;

      return {
        ap,
        linePath,
        areaPath,
        colors: apColor(ap),
      };
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bandAPs, band]);

  const recX =
    recommendedChannel > 0
      ? toX(freqMHzFromChannel(band, recommendedChannel))
      : null;

  const dfsStartX = band === '5GHz' ? toX(5250) : 0;
  const dfsEndX = band === '5GHz' ? toX(5730) : 0;

  return (
    <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
      <div className="flex flex-wrap items-center justify-between gap-2 mb-3 px-1">
        <div className="flex flex-wrap items-center gap-4 text-[11px] font-mono-telemetry">
          <span className="inline-flex items-center gap-1.5 text-cyan-300">
            <span className="h-2 w-2 rounded-full bg-cyan-400" />
            Tu Red ({currentChannel > 0 ? `Ch ${currentChannel}` : '—'})
          </span>
          <span className="inline-flex items-center gap-1.5 text-rose-300">
            <span className="h-2 w-2 rounded-full bg-rose-400" />
            Vecino Fuerte (&ge; -62 dBm)
          </span>
          <span className="inline-flex items-center gap-1.5 text-amber-300">
            <span className="h-2 w-2 rounded-full bg-amber-400" />
            Vecino Medio
          </span>
          {recommendedChannel > 0 && (
            <span className="inline-flex items-center gap-1.5 text-emerald-300">
              <span className="h-2.5 w-0.5 bg-emerald-400" />
              Recomendado (Ch {recommendedChannel})
            </span>
          )}
        </div>

        <span className="text-[11px] font-mono-telemetry text-slate-400">
          {bandAPs.length} {bandAPs.length === 1 ? 'radio' : 'radios'} en {band}
        </span>
      </div>

      {/* Main Plot Area */}
      <div className="relative w-full h-52 flex items-stretch">
        {/* Crisp unstretched HTML Y-axis labels */}
        <div className="w-12 shrink-0 relative select-none pr-2">
          <div className="absolute -top-2.5 right-2 font-mono-telemetry text-[9px] text-slate-500">
            dBm
          </div>
          {rssiTicks.map((dbm) => {
            const topPct = (toY(dbm) / CHART_H) * 100;
            return (
              <div
                key={dbm}
                style={{ top: `${topPct}%` }}
                className="absolute right-2 -translate-y-1/2 font-mono-telemetry text-[11px] text-slate-400 leading-none"
              >
                {dbm}
              </div>
            );
          })}
        </div>

        <div className="relative flex-1 h-full">
          <svg
            viewBox={`0 0 ${CHART_W} ${CHART_H}`}
            preserveAspectRatio="none"
            className="w-full h-full overflow-visible select-none"
          >
            {band === '5GHz' && (
              <rect
                x={dfsStartX}
                y={PAD_TOP}
                width={Math.max(0, dfsEndX - dfsStartX)}
                height={PLOT_H}
                fill="rgba(245, 158, 11, 0.06)"
              />
            )}

            {rssiTicks.map((dbm) => {
              const y = toY(dbm);
              return (
                <line
                  key={dbm}
                  x1={0}
                  y1={y}
                  x2={PLOT_W}
                  y2={y}
                  stroke="rgba(148,163,184,0.12)"
                  strokeDasharray="4 4"
                  strokeWidth="1"
                  vectorEffect="non-scaling-stroke"
                />
              );
            })}

            <line
              x1={0}
              y1={baselineY}
              x2={PLOT_W}
              y2={baselineY}
              stroke="rgba(148,163,184,0.22)"
              strokeWidth="1"
              vectorEffect="non-scaling-stroke"
            />

            {recX !== null && (
              <line
                x1={recX}
                y1={PAD_TOP}
                x2={recX}
                y2={baselineY}
                stroke="#10B981"
                strokeWidth="1.5"
                strokeDasharray="4 3"
                strokeOpacity="0.85"
                vectorEffect="non-scaling-stroke"
              />
            )}

            {curves.map(({ ap, linePath, areaPath, colors }) => {
              const isHovered = hoveredBSSID === ap.bssid;
              return (
                <g
                  key={ap.bssid}
                  onMouseEnter={() => setHoveredBSSID(ap.bssid)}
                  onMouseLeave={() => setHoveredBSSID(null)}
                  className="cursor-pointer"
                >
                  <path d={areaPath} fill={colors.fill} />
                  <path
                    d={linePath}
                    fill="none"
                    stroke={colors.stroke}
                    strokeWidth={ap.isCurrent || isHovered ? 2.4 : 1.5}
                    strokeOpacity={isHovered || ap.isCurrent ? 1 : 0.85}
                    vectorEffect="non-scaling-stroke"
                  />
                </g>
              );
            })}
          </svg>

          {band === '5GHz' && dfsEndX > dfsStartX && (
            <div
              style={{
                left: `${((dfsStartX + dfsEndX) / 2 / CHART_W) * 100}%`,
              }}
              className="pointer-events-none absolute top-1.5 -translate-x-1/2 font-mono-telemetry text-[10px] text-amber-400/80 whitespace-nowrap"
            >
              Zona DFS (Riesgo cortes por radar)
            </div>
          )}

          {bandAPs.length === 0 && (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center text-xs font-mono-telemetry text-slate-400">
              No se detectaron puntos de acceso en {band}
            </div>
          )}

          {hoveredAP && (
            <div className="pointer-events-none absolute top-2 right-2 z-10 rounded-lg bg-[#111827]/95 border border-white/[0.14] px-3 py-2 shadow-xl text-[11px] font-mono-telemetry space-y-0.5 max-w-xs">
              <div className="text-slate-100 font-semibold truncate">
                {hoveredAP.ssid} {hoveredAP.isCurrent ? '(Tu Red)' : ''}
              </div>
              <div className="text-slate-400 truncate">
                {hoveredAP.bssid} · {hoveredAP.vendor}
              </div>
              <div className="text-cyan-300">
                Canal {hoveredAP.channel} ({hoveredAP.widthMHz} MHz) · {hoveredAP.rssi} dBm
              </div>
              <div className="text-slate-300">
                {hoveredAP.generation} · {hoveredAP.security}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Crisp unstretched HTML X-axis channel labels */}
      <div className="flex mt-1 select-none">
        <div className="w-12 shrink-0" />
        <div className="relative flex-1 h-5">
          {channelTicks.map((ch) => {
            const leftPct = (toX(freqMHzFromChannel(band, ch)) / CHART_W) * 100;
            const isCleanTriad =
              band === '2.4GHz' && (ch === 1 || ch === 6 || ch === 11);
            const isRec = ch === recommendedChannel;
            return (
              <div
                key={ch}
                style={{ left: `${leftPct}%` }}
                className="absolute top-0 -translate-x-1/2 flex flex-col items-center"
              >
                <span className="h-1 w-px bg-white/20 mb-0.5" />
                <span
                  className={`font-mono-telemetry text-[10.5px] leading-none ${
                    isRec
                      ? 'text-emerald-400 font-semibold'
                      : isCleanTriad
                      ? 'text-slate-200 font-semibold'
                      : 'text-slate-400'
                  }`}
                >
                  {ch}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
