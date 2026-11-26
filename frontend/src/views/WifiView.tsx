import { useMemo, useState } from 'react';
import {
  AlertCircle,
  Cable,
  CheckCircle2,
  ExternalLink,
  Lock,
  MapPinOff,
  Radio,
  RefreshCw,
  Search,
  Sparkles,
  Wifi,
} from 'lucide-react';
import type { wifi } from '../../wailsjs/go/models';
import { SpectrumChart } from '../components/SpectrumChart';

interface WifiViewProps {
  scanning: boolean;
  scanResult: wifi.ScanResult | null;
  privacyMode: boolean;
  onScan: (active: boolean) => void;
  onOpenLocationSettings: () => void;
}

function signalBarColor(rssi: number): string {
  if (rssi >= -60) return 'bg-emerald-400';
  if (rssi >= -72) return 'bg-cyan-400';
  if (rssi >= -82) return 'bg-amber-400';
  return 'bg-rose-400';
}

function congestionBadge(pct: number): string {
  if (pct <= 20) return 'text-emerald-300 border-emerald-500/30 bg-emerald-500/10';
  if (pct <= 50) return 'text-cyan-300 border-cyan-500/30 bg-cyan-500/10';
  if (pct <= 75) return 'text-amber-300 border-amber-500/30 bg-amber-500/10';
  return 'text-rose-300 border-rose-500/30 bg-rose-500/10';
}

export function WifiView({
  scanning,
  scanResult,
  privacyMode,
  onScan,
  onOpenLocationSettings,
}: WifiViewProps) {
  const [selectedBand, setSelectedBand] = useState<'2.4GHz' | '5GHz' | '6GHz'>(() =>
    scanResult?.currentBand === '2.4GHz'
      ? '2.4GHz'
      : scanResult?.currentBand === '6GHz'
      ? '6GHz'
      : '5GHz',
  );
  const [search, setSearch] = useState('');

  const aps = scanResult?.accessPoints ?? [];
  const count24 = useMemo(
    () => aps.filter((a) => a.band === '2.4GHz').length,
    [aps],
  );
  const count5 = useMemo(
    () => aps.filter((a) => a.band === '5GHz').length,
    [aps],
  );
  const count6 = useMemo(
    () => aps.filter((a) => a.band === '6GHz').length,
    [aps],
  );

  // If user didn't get 5 GHz results but has 2.4 GHz results, auto-display active band cleanly
  const activeBand: '2.4GHz' | '5GHz' | '6GHz' =
    selectedBand === '5GHz' && count5 === 0 && count24 > 0
      ? '2.4GHz'
      : selectedBand === '6GHz' && count6 === 0
      ? count5 > 0
        ? '5GHz'
        : '2.4GHz'
      : selectedBand;

  const bandAPs = useMemo(() => {
    const q = search.trim().toLowerCase();
    return aps.filter((ap) => {
      if (ap.band !== activeBand) return false;
      if (!q) return true;
      return (
        ap.ssid.toLowerCase().includes(q) ||
        ap.bssid.toLowerCase().includes(q) ||
        ap.vendor.toLowerCase().includes(q) ||
        String(ap.channel).includes(q)
      );
    });
  }, [aps, activeBand, search]);

  const activeScores =
    activeBand === '2.4GHz'
      ? scanResult?.scores24Ghz ?? []
      : activeBand === '6GHz'
      ? scanResult?.scores6Ghz ?? []
      : scanResult?.scores5Ghz ?? [];
  const activeRec =
    activeBand === '2.4GHz'
      ? scanResult?.rec24Ghz
      : activeBand === '6GHz'
      ? scanResult?.rec6Ghz
      : scanResult?.rec5Ghz;

  const status = scanResult?.status ?? '';

  return (
    <div className="space-y-5">
      {/* Header Toolbar */}
      <section className="np-card rounded-xl p-5 flex flex-wrap items-center justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Radio className="h-4 w-4 text-cyan-400 shrink-0" />
            <h2 className="text-sm font-semibold text-slate-100">
              Espectro Wi-Fi & Recomendador Matemático (Win32 WLAN)
            </h2>
            {scanResult?.adapterName ? (
              <span className="font-mono-telemetry text-[11px] px-2 py-0.5 rounded bg-white/[0.05] border border-white/[0.08] text-slate-300">
                {scanResult.adapterName}
              </span>
            ) : null}
          </div>

          {scanResult?.connected && scanResult.currentSsid ? (
            <div className="flex flex-wrap items-center gap-3 text-xs font-mono-telemetry text-slate-300 mt-2">
              <span className="inline-flex items-center gap-1.5 text-cyan-300">
                {privacyMode && <Lock className="h-3 w-3 text-emerald-400" />}
                {scanResult.currentSsid}
              </span>
              <span className="text-slate-500">·</span>
              <span>
                {scanResult.currentBand || '—'} · Canal{' '}
                {scanResult.currentChannel || '—'} ({scanResult.currentWidthMHz || 20}{' '}
                MHz)
              </span>
              <span className="text-slate-500">·</span>
              <span>{scanResult.currentRssi} dBm</span>
              {(scanResult.rxRateMbps > 0 || scanResult.txRateMbps > 0) && (
                <>
                  <span className="text-slate-500">·</span>
                  <span>
                    Rx/Tx {scanResult.rxRateMbps}/{scanResult.txRateMbps} Mbps
                  </span>
                </>
              )}
            </div>
          ) : (
            <p className="text-xs text-slate-400 mt-1">
              Lectura nativa vía wlanapi.dll con cálculo físico en mW y filtro anti-DFS en 5 GHz.
            </p>
          )}
        </div>

        <button
          type="button"
          onClick={() => onScan(true)}
          disabled={scanning}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-50 text-cyan-300 border border-cyan-500/35 transition-colors cursor-pointer shrink-0"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${scanning ? 'animate-spin' : ''}`} />
          {scanning ? 'Escaneando espectro...' : 'Escanear Espectro'}
        </button>
      </section>

      {/* Windows 11 24H2+ Location Permission Guard */}
      {status === 'location_denied' && (
        <section className="np-card rounded-xl p-6 border border-amber-500/35 bg-amber-500/[0.05] space-y-4">
          <div className="flex items-start gap-3">
            <MapPinOff className="h-6 w-6 text-amber-400 shrink-0 mt-0.5" />
            <div className="space-y-1.5">
              <h3 className="text-sm font-semibold text-amber-200">
                Windows 11 bloqueó el escaneo Wi-Fi por permiso de Ubicación
              </h3>
              <p className="text-xs text-slate-300 leading-relaxed">
                {scanResult?.errorMessage ||
                  'Desde Windows 11 24H2, Microsoft exige que el permiso de Ubicación para aplicaciones de escritorio esté activado para leer redes Wi-Fi cercanas (BSSID/canales).'}
              </p>
              <p className="text-xs text-emerald-300">
                Garantía NetPulse: la app tiene 0% telemetría y no envía ni guarda tu ubicación ni tus redes fuera de tu PC.
              </p>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2.5 pl-9">
            <button
              type="button"
              onClick={onOpenLocationSettings}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 border border-amber-500/40 transition-colors cursor-pointer"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Abrir Configuración de Ubicación
            </button>
            <button
              type="button"
              onClick={() => onScan(true)}
              disabled={scanning}
              className="inline-flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-medium bg-white/[0.06] hover:bg-white/[0.12] text-slate-200 border border-white/[0.1] transition-colors cursor-pointer"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${scanning ? 'animate-spin' : ''}`} />
              Reintentar Escaneo
            </button>
          </div>
        </section>
      )}

      {/* Ethernet-Only Graceful State */}
      {status === 'no_wifi_adapter' && (
        <section className="np-card rounded-xl p-8 text-center space-y-3">
          <div className="mx-auto h-12 w-12 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center">
            <Cable className="h-6 w-6 text-emerald-400" />
          </div>
          <h3 className="text-base font-semibold text-slate-100">
            PC conectada sin radio Wi-Fi activa (Solo Cable / Ethernet)
          </h3>
          <p className="text-xs text-slate-400 max-w-lg mx-auto leading-relaxed">
            {scanResult?.errorMessage ||
              'No se detectó una placa Wi-Fi activa en esta computadora. Por cable Ethernet tenés cero interferencia RF; podés seguir usando normalmente el Centro de Mando, Radar LAN, DNS y Piloto de Router.'}
          </p>
        </section>
      )}

      {/* Generic Error */}
      {status === 'error' && (
        <section className="np-card rounded-xl p-5 border border-rose-500/30 bg-rose-500/[0.06] flex items-center gap-3 text-xs text-rose-300">
          <AlertCircle className="h-5 w-5 shrink-0" />
          <span>{scanResult?.errorMessage || 'Error leyendo interfaz WLAN.'}</span>
        </section>
      )}

      {/* Main RF View */}
      {status === 'ok' && (
        <>
          {/* Band Tabs + Recommendation Hero */}
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
            <div className="lg:col-span-4 np-card rounded-xl p-4 flex flex-col justify-between gap-3">
              <div>
                <div className="text-[11px] uppercase tracking-wider text-slate-400 mb-2">
                  Banda de Frecuencia
                </div>
                <div className={`grid gap-2 ${count6 > 0 ? 'grid-cols-3' : 'grid-cols-2'}`}>
                  {(
                    [
                      { id: '2.4GHz', label: '2.4 GHz', count: count24 },
                      { id: '5GHz', label: '5 GHz', count: count5 },
                      ...(count6 > 0 ? [{ id: '6GHz' as const, label: '6 GHz', count: count6 }] : []),
                    ] as const
                  ).map((tab) => {
                    const active = activeBand === tab.id;
                    return (
                      <button
                        key={tab.id}
                        type="button"
                        onClick={() => setSelectedBand(tab.id)}
                        className={`px-3 py-2.5 rounded-lg border text-left transition-colors cursor-pointer ${
                          active
                            ? 'bg-cyan-500/15 border-cyan-500/35 text-cyan-200'
                            : 'bg-[#090D16] border-white/[0.07] text-slate-400 hover:text-slate-200'
                        }`}
                      >
                        <div className="font-mono-telemetry text-sm font-semibold">
                          {tab.label}
                        </div>
                        <div className="text-[11px] opacity-80 mt-0.5">
                          {tab.count} {tab.count === 1 ? 'red' : 'redes'}
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>

              <div className="text-[11px] text-slate-400 leading-relaxed">
                {activeBand === '2.4GHz'
                  ? 'Solo recomienda canales 1, 6 u 11 para evitar solapamiento destructivo.'
                  : 'Excluye canales DFS (52–144) para evitar desconexiones por radares.'}
              </div>
            </div>

            {/* Recommendation Card */}
            <div
              className={`lg:col-span-8 np-card rounded-xl p-5 border ${
                activeRec?.shouldSwitch
                  ? 'border-amber-500/30 bg-amber-500/[0.05]'
                  : 'border-emerald-500/30 bg-emerald-500/[0.05]'
              }`}
            >
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div className="space-y-1.5">
                  <div className="flex items-center gap-2 text-xs font-medium">
                    {activeRec?.shouldSwitch ? (
                      <Sparkles className="h-4 w-4 text-amber-400" />
                    ) : (
                      <CheckCircle2 className="h-4 w-4 text-emerald-400" />
                    )}
                    <span
                      className={
                        activeRec?.shouldSwitch ? 'text-amber-300' : 'text-emerald-300'
                      }
                    >
                      Recomendación Matemática RF · {activeBand}
                    </span>
                  </div>
                  <h3 className="text-lg font-semibold text-slate-100">
                    {activeRec?.headline || 'Analizando canales...'}
                  </h3>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    {activeRec?.detail}
                  </p>
                </div>

                {activeRec && activeRec.recommendedChannel > 0 && (
                  <div className="shrink-0 rounded-xl bg-[#090D16] border border-white/[0.1] px-4 py-2.5 text-center font-mono-telemetry">
                    <div className="text-[10px] text-slate-400 uppercase">
                      Canal Óptimo
                    </div>
                    <div className="text-2xl font-bold text-emerald-300 mt-0.5">
                      Ch {activeRec.recommendedChannel}
                    </div>
                    <div className="text-[10px] text-slate-400">
                      {activeRec.bestCongestion}% congestión
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Spectrum Curves Chart */}
          <section className="np-card rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-slate-100">
                Mapa Espectral en {activeBand}
              </h3>
              <span className="text-[11px] text-slate-400">
                Pasá el mouse sobre una curva para inspeccionar el punto de acceso
              </span>
            </div>
            <SpectrumChart
              band={activeBand}
              accessPoints={aps}
              recommendedChannel={activeRec?.recommendedChannel ?? 0}
              currentChannel={
                scanResult?.currentBand === activeBand
                  ? scanResult.currentChannel
                  : 0
              }
            />
          </section>

          {/* Channel Congestion Grid */}
          <section className="np-card rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-slate-100">
                Congestión por Canal ({activeBand})
              </h3>
              <span className="text-[11px] font-mono-telemetry text-slate-400">
                Calculado sumando potencia vecina en milivatios (mW)
              </span>
            </div>

            <div className="grid grid-cols-3 sm:grid-cols-6 lg:grid-cols-11 gap-2">
              {activeScores.map((cs) => (
                <div
                  key={cs.channel}
                  className={`rounded-lg p-2.5 border text-center font-mono-telemetry ${
                    cs.isRecommended
                      ? 'border-emerald-500/45 bg-emerald-500/10'
                      : cs.isCurrent
                      ? 'border-cyan-500/45 bg-cyan-500/10'
                      : 'border-white/[0.07] bg-[#090D16]'
                  }`}
                >
                  <div className="flex items-center justify-center gap-1 text-xs font-semibold text-slate-100">
                    <span>Ch {cs.channel}</span>
                  </div>
                  <div
                    className={`mt-1 text-[11px] px-1.5 py-0.5 rounded border inline-block ${congestionBadge(
                      cs.congestionPct,
                    )}`}
                  >
                    {cs.congestionPct}%
                  </div>
                  <div className="text-[10px] text-slate-400 mt-1">
                    {cs.coChannelCount + cs.overlapCount} vec.
                  </div>
                  {cs.isRecommended && (
                    <div className="text-[9px] text-emerald-300 mt-0.5 font-semibold">
                      ÓPTIMO
                    </div>
                  )}
                  {!cs.isRecommended && cs.isCurrent && (
                    <div className="text-[9px] text-cyan-300 mt-0.5 font-semibold">
                      ACTUAL
                    </div>
                  )}
                  {!cs.isRecommended && !cs.isCurrent && cs.isDfs && (
                    <div className="text-[9px] text-amber-400 mt-0.5">DFS</div>
                  )}
                </div>
              ))}
            </div>
          </section>

          {/* Nearby Networks Table */}
          <section className="np-card rounded-xl p-5 space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Wifi className="h-4 w-4 text-cyan-400" />
                <h3 className="text-sm font-semibold text-slate-100">
                  Redes Detectadas en {activeBand} ({bandAPs.length})
                </h3>
              </div>
              <div className="relative w-64">
                <Search className="h-3.5 w-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  type="text"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Filtrar SSID, marca o canal..."
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] pl-9 pr-3 py-1.5 text-xs text-slate-100 placeholder:text-slate-500 focus:outline-none focus:border-cyan-500/50"
                />
              </div>
            </div>

            {bandAPs.length === 0 ? (
              <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-6 text-center text-xs text-slate-400">
                No hay redes en {activeBand} que coincidan con el filtro.
              </div>
            ) : (
              <div className="space-y-2">
                {bandAPs.map((ap) => {
                  const pct = Math.min(100, Math.max(8, ((ap.rssi + 100) / 65) * 100));
                  return (
                    <div
                      key={ap.bssid}
                      className={`rounded-lg bg-[#090D16] border px-4 py-2.5 flex flex-wrap items-center justify-between gap-3 ${
                        ap.isCurrent
                          ? 'border-cyan-500/40 bg-cyan-500/[0.04]'
                          : 'border-white/[0.07]'
                      }`}
                    >
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-sm font-semibold text-slate-100 truncate">
                            {ap.ssid}
                          </span>
                          {ap.isCurrent && (
                            <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-cyan-500/15 text-cyan-300 border border-cyan-500/30">
                              Tu Red
                            </span>
                          )}
                          {ap.isDfs && (
                            <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-300 border border-amber-500/30">
                              DFS
                            </span>
                          )}
                          <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-300 border border-white/[0.08]">
                            {ap.generation}
                          </span>
                          <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400 border border-white/[0.08]">
                            {ap.security}
                          </span>
                        </div>
                        <div className="font-mono-telemetry text-[11px] text-slate-400 mt-0.5 flex flex-wrap items-center gap-2">
                          <span className="inline-flex items-center gap-1">
                            {privacyMode && (
                              <Lock className="h-3 w-3 text-emerald-400" />
                            )}
                            {ap.bssid}
                          </span>
                          <span>·</span>
                          <span>{ap.vendor}</span>
                        </div>
                      </div>

                      <div className="flex items-center gap-4 ml-auto font-mono-telemetry">
                        <div className="text-right">
                          <div className="text-xs text-slate-100">
                            Canal {ap.channel}
                          </div>
                          <div className="text-[11px] text-slate-400">
                            {ap.widthMHz} MHz
                          </div>
                        </div>

                        <div className="w-24 text-right">
                          <div className="text-xs font-semibold text-slate-100">
                            {ap.rssi} dBm
                          </div>
                          <div className="h-1.5 w-full rounded-full bg-white/[0.07] overflow-hidden mt-1">
                            <div
                              className={`h-full rounded-full ${signalBarColor(ap.rssi)}`}
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}
