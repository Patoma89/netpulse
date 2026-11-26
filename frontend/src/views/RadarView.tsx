import { useMemo, useState } from 'react';
import {
  AlertCircle,
  Check,
  Cpu,
  Gamepad2,
  HelpCircle,
  Laptop,
  Lock,
  Pencil,
  Power,
  Printer,
  Radar,
  RefreshCw,
  Router,
  Search,
  Server,
  ShieldAlert,
  ShieldCheck,
  Smartphone,
  Tv,
  X,
} from 'lucide-react';
import type { radar } from '../../wailsjs/go/models';

export interface RadarScanProgress {
  stage: 'arp' | 'discovery' | 'classify' | 'done' | 'error';
  progressPct: number;
  foundCount: number;
  message: string;
}

interface RadarViewProps {
  scanning: boolean;
  progress: RadarScanProgress | null;
  scanResult: radar.ScanResult | null;
  error: string | null;
  connected: boolean;
  privacyMode: boolean;
  onScan: () => void;
  onUpdateDevice: (
    deviceKey: string,
    alias: string,
    category: string,
    trusted: boolean,
  ) => Promise<void>;
  onTrustAll: () => Promise<void>;
  onInspectServices: (deviceKey: string) => Promise<radar.DeviceServicesResult>;
  onWakeDevice: (deviceKey: string) => Promise<void>;
  onConfirmGatewayMAC: () => Promise<void>;
  onRenameProfile: (name: string) => Promise<void>;
}

type FilterKey =
  | 'all'
  | 'untrusted'
  | 'trusted'
  | 'router'
  | 'pc'
  | 'phone'
  | 'media'
  | 'iot';

const CATEGORY_OPTIONS = [
  { value: 'router', label: 'Router / Gateway' },
  { value: 'pc', label: 'PC / Notebook / NAS' },
  { value: 'phone', label: 'Celular / Tablet' },
  { value: 'tv', label: 'Smart TV / Streaming' },
  { value: 'console', label: 'Consola Gaming' },
  { value: 'printer', label: 'Impresora' },
  { value: 'iot', label: 'Domótica / IoT' },
  { value: 'unknown', label: 'Otro / Desconocido' },
];

function categoryIcon(category: string) {
  switch (category) {
    case 'router':
      return Router;
    case 'pc':
      return Laptop;
    case 'phone':
      return Smartphone;
    case 'tv':
      return Tv;
    case 'console':
      return Gamepad2;
    case 'printer':
      return Printer;
    case 'iot':
      return Cpu;
    default:
      return HelpCircle;
  }
}

function categoryLabel(category: string): string {
  return CATEGORY_OPTIONS.find((c) => c.value === category)?.label ?? 'Desconocido';
}

function resolveDisplayName(d: radar.Device): string {
  if (d.alias) return d.alias;
  if (d.ssdpName) return d.ssdpName;
  if (d.mdnsName) return d.mdnsName;
  if (d.netbiosName) return d.netbiosName;
  if (d.hostname) return d.hostname;
  if (d.isGateway) return 'Router / Puerta de Enlace';
  if (d.isLocal) return 'Esta PC (Local)';
  return d.vendor || 'Dispositivo LAN';
}

export function RadarView({
  scanning,
  progress,
  scanResult,
  error,
  connected,
  privacyMode,
  onScan,
  onUpdateDevice,
  onTrustAll,
  onInspectServices,
  onWakeDevice,
  onConfirmGatewayMAC,
  onRenameProfile,
}: RadarViewProps) {
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<FilterKey>('all');
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editAlias, setEditAlias] = useState('');
  const [editCategory, setEditCategory] = useState('unknown');
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [trustingAll, setTrustingAll] = useState(false);
  const [editingProfile, setEditingProfile] = useState(false);
  const [profileNameInput, setProfileNameInput] = useState('');
  const [confirmingGW, setConfirmingGW] = useState(false);
  const [servicesByKey, setServicesByKey] = useState<
    Record<string, radar.DeviceServicesResult>
  >({});
  const [inspectingKey, setInspectingKey] = useState<string | null>(null);
  const [wakingKey, setWakingKey] = useState<string | null>(null);
  const [actionFeedback, setActionFeedback] = useState<{
    type: 'ok' | 'err';
    text: string;
  } | null>(null);

  const devices = scanResult?.devices ?? [];
  const hasScanned = (scanResult?.completedAt ?? '') !== '' || devices.length > 0;

  const filteredDevices = useMemo(() => {
    const q = search.trim().toLowerCase();
    return devices.filter((d) => {
      if (filter === 'untrusted' && d.trusted) return false;
      if (filter === 'trusted' && !d.trusted) return false;
      if (filter === 'router' && d.category !== 'router') return false;
      if (filter === 'pc' && d.category !== 'pc') return false;
      if (filter === 'phone' && d.category !== 'phone') return false;
      if (filter === 'media' && d.category !== 'tv' && d.category !== 'console') {
        return false;
      }
      if (filter === 'iot' && d.category !== 'iot' && d.category !== 'printer') {
        return false;
      }

      if (!q) return true;
      const hay = [
        d.alias,
        d.ssdpName,
        d.mdnsName,
        d.netbiosName,
        d.hostname,
        d.vendor,
        d.ip,
        d.mac,
        d.category,
      ]
        .join(' ')
        .toLowerCase();
      return hay.includes(q);
    });
  }, [devices, filter, search]);

  function startEdit(d: radar.Device) {
    setEditingKey(d.key);
    setEditAlias(d.alias || '');
    setEditCategory(d.category || 'unknown');
  }

  async function saveEdit(d: radar.Device) {
    setSavingKey(d.key);
    try {
      await onUpdateDevice(d.key, editAlias, editCategory, d.trusted);
      setEditingKey(null);
    } finally {
      setSavingKey(null);
    }
  }

  async function handleToggleTrust(d: radar.Device) {
    setSavingKey(d.key);
    try {
      await onUpdateDevice(d.key, d.alias || '', d.category || 'unknown', !d.trusted);
    } finally {
      setSavingKey(null);
    }
  }

  async function handleTrustAllClick() {
    setTrustingAll(true);
    try {
      await onTrustAll();
    } finally {
      setTrustingAll(false);
    }
  }

  async function handleInspectServices(d: radar.Device) {
    setInspectingKey(d.key);
    setActionFeedback(null);
    try {
      const res = await onInspectServices(d.key);
      setServicesByKey((prev) => ({ ...prev, [d.key]: res }));
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setActionFeedback({
        type: 'err',
        text: `No se pudieron inspeccionar servicios de ${resolveDisplayName(d)}: ${msg}`,
      });
    } finally {
      setInspectingKey(null);
    }
  }

  async function handleWakeDevice(d: radar.Device) {
    setWakingKey(d.key);
    setActionFeedback(null);
    try {
      await onWakeDevice(d.key);
      setActionFeedback({
        type: 'ok',
        text: `Magic Packet (Wake-on-LAN) enviado a ${resolveDisplayName(d)} (${d.mac}).`,
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setActionFeedback({
        type: 'err',
        text: `Error al enviar Wake-on-LAN: ${msg}`,
      });
    } finally {
      setWakingKey(null);
    }
  }

  async function handleSaveProfileName() {
    const trimmed = profileNameInput.trim();
    if (!trimmed) return;
    try {
      await onRenameProfile(trimmed);
      setEditingProfile(false);
    } catch (err) {
      setActionFeedback({
        type: 'err',
        text: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async function handleConfirmGatewayClick() {
    setConfirmingGW(true);
    try {
      await onConfirmGatewayMAC();
      setActionFeedback({
        type: 'ok',
        text: 'Nueva MAC del router confirmada y guardada para este perfil de red.',
      });
    } finally {
      setConfirmingGW(false);
    }
  }

  return (
    <div className="space-y-5">
      {/* Toolbar & Progress Card */}
      <section className="np-card rounded-xl p-5 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <Radar className="h-4 w-4 text-cyan-400" />
              <h2 className="text-sm font-semibold text-slate-100">
                Radar LAN Multi-Protocolo (Sin Admin)
              </h2>
              {scanResult?.subnet ? (
                <span className="font-mono-telemetry text-[11px] px-2 py-0.5 rounded bg-white/[0.05] border border-white/[0.08] text-cyan-300">
                  {scanResult.subnet}
                </span>
              ) : null}
              {scanResult?.profileName && (
                editingProfile ? (
                  <span className="inline-flex items-center gap-1">
                    <input
                      type="text"
                      value={profileNameInput}
                      onChange={(e) => setProfileNameInput(e.target.value)}
                      placeholder="Ej: Casa, Oficina..."
                      className="rounded bg-[#090D16] border border-cyan-500/40 px-2 py-0.5 text-[11px] text-slate-100 w-32"
                    />
                    <button
                      type="button"
                      onClick={handleSaveProfileName}
                      className="p-1 rounded bg-cyan-500/20 text-cyan-200 cursor-pointer"
                    >
                      <Check className="h-3 w-3" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setEditingProfile(false)}
                      className="p-1 rounded bg-white/[0.05] text-slate-300 cursor-pointer"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </span>
                ) : (
                  <button
                    type="button"
                    onClick={() => {
                      setProfileNameInput(scanResult.profileName || 'Red Local');
                      setEditingProfile(true);
                    }}
                    title="Perfil de red actual (clic para renombrar)"
                    className="inline-flex items-center gap-1 font-mono-telemetry text-[11px] px-2 py-0.5 rounded bg-emerald-500/12 border border-emerald-500/30 text-emerald-300 hover:bg-emerald-500/20 cursor-pointer"
                  >
                    <span>Perfil: {scanResult.profileName}</span>
                    <Pencil className="h-2.5 w-2.5" />
                  </button>
                )
              )}
            </div>
            <p className="text-xs text-slate-400 mt-1">
              Combina barrido ARP concurrente + mDNS + SSDP/UPnP + NetBIOS e identifica fabricantes IEEE OUI 100% offline.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {hasScanned && (scanResult?.unknownCount ?? 0) > 0 && (
              <button
                type="button"
                onClick={handleTrustAllClick}
                disabled={scanning || trustingAll}
                className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-emerald-500/12 hover:bg-emerald-500/20 disabled:opacity-50 text-emerald-300 border border-emerald-500/30 transition-colors cursor-pointer"
              >
                <ShieldCheck className="h-3.5 w-3.5" />
                {trustingAll ? 'Guardando...' : 'Marcar todos confiables'}
              </button>
            )}

            <button
              type="button"
              onClick={onScan}
              disabled={!connected || scanning}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-50 disabled:cursor-not-allowed text-cyan-300 border border-cyan-500/35 transition-colors cursor-pointer"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${scanning ? 'animate-spin' : ''}`} />
              {scanning ? 'Escaneando red...' : hasScanned ? 'Re-escanear Red' : 'Escanear Red'}
            </button>
          </div>
        </div>

        {scanning && progress && (
          <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3.5 space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="text-cyan-300 font-medium">{progress.message}</span>
              <span className="font-mono-telemetry text-slate-300">
                {progress.foundCount} equipos · {progress.progressPct}%
              </span>
            </div>
            <div className="h-2 w-full rounded-full bg-white/[0.06] overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-cyan-500 to-emerald-400 transition-all duration-200"
                style={{ width: `${Math.min(100, Math.max(5, progress.progressPct))}%` }}
              />
            </div>
          </div>
        )}

        {error && !scanning && (
          <div className="rounded-lg bg-rose-500/10 border border-rose-500/30 px-3.5 py-2.5 flex items-center gap-2.5 text-xs text-rose-300">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {scanResult?.gatewayMacAlert?.active && (
          <div className="rounded-lg bg-rose-500/12 border border-rose-500/40 px-4 py-3 flex flex-wrap items-center justify-between gap-3 text-xs text-rose-200">
            <div className="flex items-start gap-2.5">
              <ShieldAlert className="h-4 w-4 text-rose-400 shrink-0 mt-0.5" />
              <div>
                <div className="font-semibold text-rose-200">
                  Cambio de MAC detectado en el Gateway ({scanResult.gatewayMacAlert.detectedAt})
                </div>
                <div className="text-[11px] text-rose-300/90 mt-0.5 font-mono-telemetry">
                  Anterior: {scanResult.gatewayMacAlert.previousMac} → Actual: {scanResult.gatewayMacAlert.currentMac}.
                  Puede deberse a un reemplazo de router o a un posible conflicto IP / ARP spoofing.
                </div>
              </div>
            </div>
            <button
              type="button"
              onClick={handleConfirmGatewayClick}
              disabled={confirmingGW}
              className="px-3 py-1.5 rounded-lg text-[11px] font-medium bg-white/[0.1] hover:bg-white/[0.2] text-white border border-white/[0.2] cursor-pointer shrink-0"
            >
              {confirmingGW ? 'Confirmando...' : 'Confirmar nuevo router'}
            </button>
          </div>
        )}
      </section>

      {/* KPI Strip */}
      {hasScanned && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          <div className="np-card rounded-xl p-4">
            <div className="text-[11px] uppercase tracking-wider text-slate-400">
              Equipos Activos
            </div>
            <div className="font-mono-telemetry text-2xl font-bold text-slate-100 mt-1">
              {scanResult?.totalDevices ?? 0}
            </div>
            <div className="text-[11px] text-slate-400 mt-0.5">
              De {scanResult?.scannedHosts ?? 0} IPs sondeadas
            </div>
          </div>

          <div className="np-card rounded-xl p-4">
            <div className="text-[11px] uppercase tracking-wider text-slate-400">
              Confiables
            </div>
            <div className="font-mono-telemetry text-2xl font-bold text-emerald-300 mt-1">
              {scanResult?.trustedCount ?? 0}
            </div>
            <div className="text-[11px] text-slate-400 mt-0.5">
              Guardados localmente
            </div>
          </div>

          <div className="np-card rounded-xl p-4">
            <div className="text-[11px] uppercase tracking-wider text-slate-400">
              Sin Confirmar / Nuevos
            </div>
            <div
              className={`font-mono-telemetry text-2xl font-bold mt-1 ${
                (scanResult?.unknownCount ?? 0) > 0 ? 'text-amber-300' : 'text-slate-300'
              }`}
            >
              {scanResult?.unknownCount ?? 0}
            </div>
            <div className="text-[11px] text-slate-400 mt-0.5">
              {scanResult?.newCount ?? 0} detectados por 1ra vez
            </div>
          </div>

          <div className="np-card rounded-xl p-4">
            <div className="text-[11px] uppercase tracking-wider text-slate-400">
              Tiempo de Barrido
            </div>
            <div className="font-mono-telemetry text-2xl font-bold text-cyan-300 mt-1">
              {scanResult ? `${(scanResult.durationMs / 1000).toFixed(2)}s` : '—'}
            </div>
            <div className="text-[11px] text-slate-400 mt-0.5">
              ARP · mDNS · SSDP · NetBIOS
            </div>
          </div>
        </div>
      )}

      {/* Empty State before first scan */}
      {!hasScanned && !scanning ? (
        <section className="np-card rounded-xl p-10 text-center space-y-4">
          <div className="mx-auto h-14 w-14 rounded-2xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center">
            <Radar className="h-7 w-7 text-cyan-400" />
          </div>
          <div className="max-w-lg mx-auto space-y-1.5">
            <h3 className="text-base font-semibold text-slate-100">
              Descubrí quién está conectado a tu Wi-Fi o LAN
            </h3>
            <p className="text-xs text-slate-400 leading-relaxed">
              NetPulse escanea tu subred en ~1–2 segundos sin permisos de Administrador y sin inundar dispositivos IoT. Podés poner alias, marcar equipos confiables y detectar intrusos nuevos.
            </p>
          </div>
          <div>
            <button
              type="button"
              onClick={onScan}
              disabled={!connected}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg text-xs font-medium bg-cyan-500/20 hover:bg-cyan-500/30 disabled:opacity-50 text-cyan-200 border border-cyan-500/40 transition-colors cursor-pointer"
            >
              <Radar className="h-4 w-4" />
              Iniciar Escaneo de Red
            </button>
          </div>
        </section>
      ) : null}

      {/* Search & Filter + Device List */}
      {hasScanned && (
        <section className="np-card rounded-xl p-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="relative flex-1 min-w-[220px] max-w-sm">
              <Search className="h-3.5 w-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Buscar por alias, marca, IP o MAC..."
                className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] pl-9 pr-3 py-1.5 text-xs text-slate-100 placeholder:text-slate-500 focus:outline-none focus:border-cyan-500/50"
              />
            </div>

            <div className="flex flex-wrap items-center gap-1.5">
              {(
                [
                  { id: 'all', label: 'Todos' },
                  { id: 'untrusted', label: 'Sin confirmar' },
                  { id: 'trusted', label: 'Confiables' },
                  { id: 'router', label: 'Routers' },
                  { id: 'pc', label: 'PCs' },
                  { id: 'phone', label: 'Celulares' },
                  { id: 'media', label: 'TV / Consolas' },
                  { id: 'iot', label: 'IoT / Imp.' },
                ] as const
              ).map((pill) => (
                <button
                  key={pill.id}
                  type="button"
                  onClick={() => setFilter(pill.id)}
                  className={`px-2.5 py-1 rounded-md text-[11px] font-medium border transition-colors cursor-pointer ${
                    filter === pill.id
                      ? 'bg-cyan-500/15 text-cyan-300 border-cyan-500/35'
                      : 'bg-white/[0.03] text-slate-400 border-white/[0.07] hover:text-slate-200'
                  }`}
                >
                  {pill.label}
                </button>
              ))}
            </div>
          </div>

          {actionFeedback && (
            <div
              className={`rounded-lg border px-3.5 py-2.5 flex items-center justify-between gap-2 text-xs ${
                actionFeedback.type === 'ok'
                  ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                  : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
              }`}
            >
              <span>{actionFeedback.text}</span>
              <button
                type="button"
                onClick={() => setActionFeedback(null)}
                className="p-0.5 rounded hover:bg-white/[0.08] cursor-pointer"
                aria-label="Cerrar aviso"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          )}

          {filteredDevices.length === 0 ? (
            <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-8 text-center text-xs text-slate-400">
              No hay dispositivos que coincidan con el filtro actual.
            </div>
          ) : (
            <div className="space-y-2.5">
              {filteredDevices.map((d) => {
                const Icon = categoryIcon(d.category);
                const displayName = resolveDisplayName(d);
                const isEditing = editingKey === d.key;
                const isSaving = savingKey === d.key;
                const isInspecting = inspectingKey === d.key;
                const isWaking = wakingKey === d.key;
                const serviceResult = servicesByKey[d.key];
                const openServices =
                  serviceResult?.services?.filter((s) => s.open) ?? [];

                return (
                  <div
                    key={d.key}
                    className={`rounded-lg bg-[#090D16] border p-3.5 transition-colors ${
                      d.isNew && !d.trusted
                        ? 'border-rose-500/35 bg-rose-500/[0.03]'
                        : !d.trusted
                        ? 'border-amber-500/25'
                        : 'border-white/[0.07]'
                    }`}
                  >
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="flex items-center gap-3 min-w-0">
                        <div
                          className={`h-10 w-10 rounded-lg border flex items-center justify-center shrink-0 ${
                            d.isGateway
                              ? 'bg-cyan-500/12 border-cyan-500/30 text-cyan-300'
                              : d.isLocal
                              ? 'bg-emerald-500/12 border-emerald-500/30 text-emerald-300'
                              : !d.trusted
                              ? 'bg-amber-500/12 border-amber-500/30 text-amber-300'
                              : 'bg-white/[0.05] border-white/[0.1] text-slate-300'
                          }`}
                        >
                          <Icon className="h-5 w-5" />
                        </div>

                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="text-sm font-semibold text-slate-100 truncate">
                              {displayName}
                            </span>

                            {d.isLocal && (
                              <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                                Esta PC
                              </span>
                            )}
                            {d.isGateway && (
                              <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-cyan-500/15 text-cyan-300 border border-cyan-500/30">
                                Gateway
                              </span>
                            )}
                            {d.isNew && !d.trusted && (
                              <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-rose-500/15 text-rose-300 border border-rose-500/30">
                                Nuevo
                              </span>
                            )}
                            {d.trusted ? (
                              <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-300 border border-emerald-500/25">
                                Confiable
                              </span>
                            ) : (
                              <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-300 border border-amber-500/25">
                                Sin confirmar
                              </span>
                            )}
                            {d.isRandomizedMac && (
                              <span
                                className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-300 border border-white/[0.08]"
                                title="El dispositivo usa MAC aleatoria de privacidad (común en iOS/Android/Win11)"
                              >
                                MAC Privada
                              </span>
                            )}
                          </div>

                          <div className="flex flex-wrap items-center gap-2 text-[11px] text-slate-400 mt-1">
                            <span>{d.vendor}</span>
                            <span>·</span>
                            <span>{categoryLabel(d.category)}</span>
                            {d.protocols?.length ? (
                              <>
                                <span>·</span>
                                <span className="font-mono-telemetry text-slate-400">
                                  {d.protocols.join(' · ')}
                                </span>
                              </>
                            ) : null}
                            {d.seenCount > 0 && (
                              <>
                                <span>·</span>
                                <span className="font-mono-telemetry text-slate-400">
                                  Visto {d.seenCount}x
                                  {d.lastTransition ? ` (${d.lastTransition})` : ''}
                                </span>
                              </>
                            )}
                          </div>
                        </div>
                      </div>

                      <div className="flex flex-wrap items-center gap-3 ml-auto">
                        <div className="text-right font-mono-telemetry">
                          <div className="text-xs text-slate-100 flex items-center justify-end gap-1">
                            {privacyMode && (
                              <Lock className="h-3 w-3 text-emerald-400" />
                            )}
                            <span>{d.ip}</span>
                          </div>
                          <div className="text-[11px] text-slate-400 mt-0.5">
                            {d.mac}
                            {d.rttMs > 0 ? ` · ${d.rttMs.toFixed(1)} ms` : ''}
                          </div>
                        </div>

                        <div className="flex items-center gap-1.5">
                          <button
                            type="button"
                            onClick={() => handleToggleTrust(d)}
                            disabled={isSaving}
                            title={
                              d.trusted
                                ? 'Quitar de confiables'
                                : 'Marcar dispositivo como confiable'
                            }
                            className={`inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-[11px] font-medium border transition-colors cursor-pointer ${
                              d.trusted
                                ? 'bg-emerald-500/12 border-emerald-500/30 text-emerald-300 hover:bg-emerald-500/20'
                                : 'bg-amber-500/12 border-amber-500/30 text-amber-300 hover:bg-amber-500/20'
                            }`}
                          >
                            {d.trusted ? (
                              <ShieldCheck className="h-3.5 w-3.5" />
                            ) : (
                              <ShieldAlert className="h-3.5 w-3.5" />
                            )}
                            <span>{d.trusted ? 'Confiable' : 'Confiar'}</span>
                          </button>

                          {!d.isLocal && (
                            <>
                              <button
                                type="button"
                                onClick={() => handleInspectServices(d)}
                                disabled={isInspecting}
                                title="Inspeccionar puertos locales comunes (TCP)"
                                className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-[11px] font-medium bg-cyan-500/10 hover:bg-cyan-500/20 disabled:opacity-45 text-cyan-300 border border-cyan-500/25 transition-colors cursor-pointer"
                              >
                                <Server className="h-3.5 w-3.5" />
                                <span>
                                  {isInspecting ? 'Escaneando...' : 'Servicios'}
                                </span>
                              </button>

                              <button
                                type="button"
                                onClick={() => handleWakeDevice(d)}
                                disabled={isWaking || !d.mac}
                                title="Enviar Magic Packet Wake-on-LAN (UDP puerto 9)"
                                className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-[11px] font-medium bg-violet-500/10 hover:bg-violet-500/20 disabled:opacity-45 text-violet-300 border border-violet-500/25 transition-colors cursor-pointer"
                              >
                                <Power className="h-3.5 w-3.5" />
                                <span>
                                  {isWaking ? 'Enviando...' : 'Encender'}
                                </span>
                              </button>
                            </>
                          )}

                          <button
                            type="button"
                            onClick={() =>
                              isEditing ? setEditingKey(null) : startEdit(d)
                            }
                            title="Editar alias y categoría"
                            className="p-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.1] text-slate-300 border border-white/[0.08] transition-colors cursor-pointer"
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </div>
                    </div>

                    {/* Inline Edit Row */}
                    {isEditing && (
                      <div className="mt-3 pt-3 border-t border-white/[0.07] flex flex-wrap items-center gap-2">
                        <input
                          type="text"
                          value={editAlias}
                          onChange={(e) => setEditAlias(e.target.value)}
                          placeholder="Ej: PC de Nico, PS5 Living..."
                          className="flex-1 min-w-[180px] rounded-lg bg-[#111827] border border-white/[0.12] px-3 py-1.5 text-xs text-slate-100 placeholder:text-slate-500 focus:outline-none focus:border-cyan-500/50"
                        />
                        <select
                          value={editCategory}
                          onChange={(e) => setEditCategory(e.target.value)}
                          className="rounded-lg bg-[#111827] border border-white/[0.12] px-3 py-1.5 text-xs text-slate-100 focus:outline-none focus:border-cyan-500/50 cursor-pointer"
                        >
                          {CATEGORY_OPTIONS.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.label}
                            </option>
                          ))}
                        </select>
                        <button
                          type="button"
                          onClick={() => saveEdit(d)}
                          disabled={isSaving}
                          className="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-200 border border-cyan-500/35 cursor-pointer"
                        >
                          <Check className="h-3.5 w-3.5" />
                          Guardar
                        </button>
                        <button
                          type="button"
                          onClick={() => setEditingKey(null)}
                          className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 border border-white/[0.08] cursor-pointer"
                        >
                          <X className="h-3.5 w-3.5" />
                          Cancelar
                        </button>
                      </div>
                    )}

                    {/* Detected Services Row */}
                    {serviceResult && (
                      <div className="mt-3 pt-3 border-t border-white/[0.07] flex flex-wrap items-center justify-between gap-2">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <span className="text-[11px] text-slate-400 mr-1">
                            Servicios locales ({serviceResult.checkedAt}):
                          </span>
                          {openServices.length > 0 ? (
                            openServices.map((svc) => (
                              <span
                                key={svc.port}
                                className="font-mono-telemetry text-[11px] px-2 py-0.5 rounded bg-emerald-500/12 text-emerald-300 border border-emerald-500/30"
                              >
                                {svc.port}/TCP · {svc.name} (
                                {svc.rttMs.toFixed(0)} ms)
                              </span>
                            ))
                          ) : (
                            <span className="text-[11px] text-slate-400">
                              Sin puertos comunes abiertos (
                              {serviceResult.services?.length ?? 0} revisados)
                            </span>
                          )}
                        </div>
                        <button
                          type="button"
                          onClick={() =>
                            setServicesByKey((prev) => {
                              const next = { ...prev };
                              delete next[d.key];
                              return next;
                            })
                          }
                          className="p-1 rounded hover:bg-white/[0.08] text-slate-400 hover:text-slate-200 cursor-pointer"
                          title="Ocultar servicios"
                        >
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </section>
      )}
    </div>
  );
}
