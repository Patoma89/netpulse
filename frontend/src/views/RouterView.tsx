import { useEffect, useState } from 'react';
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ExternalLink,
  KeyRound,
  Lock,
  RefreshCw,
  RotateCcw,
  Router as RouterIcon,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
  Trash2,
} from 'lucide-react';
import { router } from '../../wailsjs/go/models';

interface RouterViewProps {
  profile: router.RouterProfile | null;
  detectingProfile: boolean;
  beforeAfter: router.BeforeAfterReport | null;
  privacyMode: boolean;
  onDetectProfile: () => void;
  onOpenAdminPanel: () => void;
  onCaptureBaseline: () => void;
  onCompareAfter: () => void;
  onLoadSavedConfig: (driver: string) => Promise<router.AutoPilotConfig>;
  onClearSavedCreds: (driver: string) => Promise<void>;
  onTestAutoPilot: (cfg: router.AutoPilotConfig) => Promise<router.AutoPilotStatus>;
  onApplyChannel: (req: router.ChannelChangeRequest) => Promise<router.ChannelChangeResult>;
  onRollbackChannel: () => Promise<router.ChannelChangeResult>;
  onClearSSHFingerprint: (host: string, port: number) => Promise<void>;
  lastBackup: router.ChannelBackup | null;
}

const DRIVER_OPTIONS = [
  { id: 'openwrt', label: 'OpenWrt (SSH / UCI)', defaultPort: 22, defaultUser: 'root' },
  { id: 'mikrotik', label: 'MikroTik (RouterOS REST API)', defaultPort: 80, defaultUser: 'admin' },
  { id: 'tr064', label: 'TR-064 / FRITZ!Box (SOAP)', defaultPort: 49000, defaultUser: 'admin' },
];

const CHANNELS_24 = [1, 6, 11];
const CHANNELS_5_SAFE = [36, 40, 44, 48, 149, 153, 157, 161, 165];
const CHANNELS_5_DFS = [52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140, 144];

export function RouterView({
  profile,
  detectingProfile,
  beforeAfter,
  privacyMode,
  onDetectProfile,
  onOpenAdminPanel,
  onCaptureBaseline,
  onCompareAfter,
  onLoadSavedConfig,
  onClearSavedCreds,
  onTestAutoPilot,
  onApplyChannel,
  onRollbackChannel,
  onClearSSHFingerprint,
  lastBackup,
}: RouterViewProps) {
  const [tab, setTab] = useState<'copilot' | 'autopilot'>('copilot');

  // Auto-Pilot state
  const [driver, setDriver] = useState<'openwrt' | 'mikrotik' | 'tr064'>('openwrt');
  const [host, setHost] = useState('');
  const [port, setPort] = useState(22);
  const [username, setUsername] = useState('root');
  const [password, setPassword] = useState('');
  const [useTLS, setUseTLS] = useState(false);
  const [saveCreds, setSaveCreds] = useState(true);
  const [hasSavedCreds, setHasSavedCreds] = useState(false);

  const [testingConn, setTestingConn] = useState(false);
  const [apStatus, setApStatus] = useState<router.AutoPilotStatus | null>(null);
  const [apError, setApError] = useState<string | null>(null);

  const [selectedIface, setSelectedIface] = useState('');
  const [selectedBand, setSelectedBand] = useState<'2.4GHz' | '5GHz'>('5GHz');
  const [targetChannel, setTargetChannel] = useState<number>(36);
  const [allowDFS, setAllowDFS] = useState(false);
  const [confirmedDisconnect, setConfirmedDisconnect] = useState(false);

  const [applyingChannel, setApplyingChannel] = useState(false);
  const [rollingBack, setRollingBack] = useState(false);
  const [changeResult, setChangeResult] = useState<router.ChannelChangeResult | null>(null);
  const [clearingFP, setClearingFP] = useState(false);
  const [fpNotice, setFpNotice] = useState<string | null>(null);

  useEffect(() => {
    if (profile?.suggestedDriver === 'openwrt' || profile?.suggestedDriver === 'mikrotik' || profile?.suggestedDriver === 'tr064') {
      setDriver(profile.suggestedDriver);
    }
  }, [profile?.suggestedDriver]);

  useEffect(() => {
    let active = true;
    async function load() {
      try {
        const saved = await onLoadSavedConfig(driver);
        if (!active) return;
        const meta = DRIVER_OPTIONS.find((d) => d.id === driver)!;
        setHost(saved.host || profile?.gatewayIp || '');
        setPort(saved.port > 0 ? saved.port : meta.defaultPort);
        setUsername(saved.username || meta.defaultUser);
        setUseTLS(saved.useTls);
        setHasSavedCreds(saved.hasSavedCreds);
        setPassword('');
        setApStatus(null);
        setApError(null);
      } catch (err) {
        console.error('Error loading router config:', err);
      }
    }
    load();
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [driver]);

  async function handleClearSSHFingerprint() {
    const targetHost = (host || profile?.gatewayIp || '').trim();
    if (!targetHost) return;
    setClearingFP(true);
    setFpNotice(null);
    try {
      await onClearSSHFingerprint(targetHost, port || 22);
      setFpNotice(
        'Huella SSH eliminada. La próxima conexión volverá a verificar y guardar la clave del router.',
      );
      setApStatus((prev) =>
        prev
          ? router.AutoPilotStatus.createFrom({
              ...prev,
              hostFingerprint: '',
            })
          : null,
      );
    } catch (err) {
      setApError(err instanceof Error ? err.message : String(err));
    } finally {
      setClearingFP(false);
    }
  }

  async function handleTestConn() {
    setTestingConn(true);
    setApError(null);
    setFpNotice(null);
    setChangeResult(null);
    try {
      const cfg = router.AutoPilotConfig.createFrom({
        driver,
        host,
        port,
        username,
        password,
        useTls: useTLS,
        saveCredentials: saveCreds,
        hasSavedCreds,
      });
      const st = await onTestAutoPilot(cfg);
      setApStatus(st);
      if (saveCreds && password) {
        setHasSavedCreds(true);
      }
      if (st.radios?.length > 0) {
        const first = st.radios[0];
        setSelectedIface(first.interfaceName);
        const b = first.band === '2.4GHz' ? '2.4GHz' : '5GHz';
        setSelectedBand(b);
        setTargetChannel(
          b === '2.4GHz'
            ? profile?.rec24Channel || 11
            : profile?.rec5Channel || 36,
        );
      }
    } catch (err) {
      setApError(err instanceof Error ? err.message : String(err));
      setApStatus(null);
    } finally {
      setTestingConn(false);
    }
  }

  async function handleApply() {
    setApplyingChannel(true);
    setApError(null);
    try {
      const req = router.ChannelChangeRequest.createFrom({
        driver,
        host,
        port,
        username,
        password,
        useTls: useTLS,
        interfaceName: selectedIface,
        band: selectedBand,
        targetChannel,
        allowDfs: allowDFS,
        confirmedDisconnect,
      });
      const res = await onApplyChannel(req);
      setChangeResult(res);
      setConfirmedDisconnect(false);
    } catch (err) {
      setApError(err instanceof Error ? err.message : String(err));
    } finally {
      setApplyingChannel(false);
    }
  }

  async function handleRollback() {
    setRollingBack(true);
    setApError(null);
    try {
      const res = await onRollbackChannel();
      setChangeResult(res);
    } catch (err) {
      setApError(err instanceof Error ? err.message : String(err));
    } finally {
      setRollingBack(false);
    }
  }

  const availableChannels =
    selectedBand === '2.4GHz'
      ? CHANNELS_24
      : allowDFS
      ? [...CHANNELS_5_SAFE, ...CHANNELS_5_DFS].sort((a, b) => a - b)
      : CHANNELS_5_SAFE;

  return (
    <div className="space-y-5">
      {/* Header & Mode Switcher */}
      <section className="np-card rounded-xl p-5 flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <RouterIcon className="h-4 w-4 text-cyan-400" />
            <h2 className="text-sm font-semibold text-slate-100">
              Piloto de Router · Co-Piloto Universal & Auto-Piloto Seguro
            </h2>
          </div>
          <p className="text-xs text-slate-400 mt-1">
            Cero scrapers frágiles en módems ISP: guía exacta + verificación Antes/Después para cualquier router, y automatización segura en routers con API oficial.
          </p>
        </div>

        <div className="flex items-center gap-1.5 p-1 rounded-lg bg-[#090D16] border border-white/[0.08]">
          <button
            type="button"
            onClick={() => setTab('copilot')}
            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer ${
              tab === 'copilot'
                ? 'bg-cyan-500/20 text-cyan-200 border border-cyan-500/35'
                : 'text-slate-400 hover:text-slate-200 border border-transparent'
            }`}
          >
            Co-Piloto Universal
          </button>
          <button
            type="button"
            onClick={() => setTab('autopilot')}
            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer ${
              tab === 'autopilot'
                ? 'bg-cyan-500/20 text-cyan-200 border border-cyan-500/35'
                : 'text-slate-400 hover:text-slate-200 border border-transparent'
            }`}
          >
            Auto-Piloto Seguro
          </button>
        </div>
      </section>

      {tab === 'copilot' ? (
        <>
          {/* Detected Gateway + Recommended Values */}
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
            <section className="lg:col-span-6 np-card rounded-xl p-5 space-y-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-[11px] uppercase tracking-wider text-slate-400">
                    Gateway / Módem Detectado
                  </div>
                  <h3 className="text-base font-semibold text-slate-100 mt-1">
                    {profile?.vendor || 'Detectando módem...'}
                  </h3>
                  {profile?.httpTitle && (
                    <div className="text-xs text-cyan-300 mt-0.5 truncate">
                      Panel: {profile.httpTitle}
                    </div>
                  )}
                </div>
                <button
                  type="button"
                  onClick={onDetectProfile}
                  disabled={detectingProfile}
                  className="p-2 rounded-lg bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 border border-white/[0.08] cursor-pointer"
                  title="Volver a detectar gateway"
                >
                  <RefreshCw className={`h-3.5 w-3.5 ${detectingProfile ? 'animate-spin' : ''}`} />
                </button>
              </div>

              <div className="grid grid-cols-2 gap-2.5 font-mono-telemetry text-xs">
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] px-3 py-2">
                  <div className="text-[10px] text-slate-400">IP PUERTA DE ENLACE</div>
                  <div className="text-slate-100 mt-0.5 flex items-center gap-1.5">
                    {privacyMode && <Lock className="h-3 w-3 text-emerald-400" />}
                    <span>{profile?.gatewayIp || '—'}</span>
                  </div>
                </div>
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] px-3 py-2">
                  <div className="text-[10px] text-slate-400">MAC ROUTER</div>
                  <div className="text-slate-100 mt-0.5">
                    {profile?.gatewayMac || '—'}
                  </div>
                </div>
              </div>

              <div className="rounded-lg bg-white/[0.03] border border-white/[0.07] p-3 text-xs text-slate-300 leading-relaxed">
                {profile?.menuGuide || 'Analizando fabricante del router...'}
              </div>

              <button
                type="button"
                onClick={onOpenAdminPanel}
                disabled={!profile?.detected}
                className="w-full inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 text-cyan-200 border border-cyan-500/35 transition-colors cursor-pointer"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                Abrir Panel del Router ({profile?.adminUrl || '—'})
              </button>
            </section>

            {/* Recommended Values Card */}
            <section className="lg:col-span-6 np-card rounded-xl p-5 flex flex-col justify-between space-y-4">
              <div>
                <div className="flex items-center gap-2 text-emerald-300 text-xs font-medium">
                  <Sparkles className="h-4 w-4" />
                  <span>Valores Recomendados para Copiar</span>
                </div>
                <p className="text-xs text-slate-400 mt-1">
                  Basado en tu último escaneo de Espectro Wi-Fi y Benchmark DNS.
                </p>

                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-4 font-mono-telemetry">
                  <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3">
                    <div className="text-[10px] text-slate-400">CANAL 2.4 GHZ</div>
                    <div className="text-lg font-bold text-cyan-300 mt-1">
                      {profile?.rec24Channel ? `Ch ${profile.rec24Channel}` : 'Escanear Wi-Fi'}
                    </div>
                    <div className="text-[10px] text-slate-500 mt-0.5">Tríada limpia 1/6/11</div>
                  </div>

                  <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3">
                    <div className="text-[10px] text-slate-400">CANAL 5 GHZ</div>
                    <div className="text-lg font-bold text-emerald-300 mt-1">
                      {profile?.rec5Channel ? `Ch ${profile.rec5Channel}` : 'Escanear Wi-Fi'}
                    </div>
                    <div className="text-[10px] text-slate-500 mt-0.5">Seguro sin DFS</div>
                  </div>

                  <div className="rounded-lg bg-[#090D16] border border-white/[0.07] p-3">
                    <div className="text-[10px] text-slate-400">MEJOR DNS</div>
                    <div className="text-sm font-bold text-slate-100 mt-1 truncate">
                      {profile?.recDnsPrimary || 'Correr Test'}
                    </div>
                    <div className="text-[10px] text-slate-500 mt-0.5 truncate">
                      {profile?.recDnsName || 'DNS Turbo'}
                    </div>
                  </div>
                </div>
              </div>

              <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3 text-[11px] text-slate-400 leading-relaxed">
                Tip: Si cambiás el canal desde el panel del módem, el Wi-Fi se reinicia ~5 segundos. Usá el comparador <strong>Antes vs. Después</strong> abajo para comprobar la mejora real.
              </div>
            </section>
          </div>

          {/* Before vs After Comparator */}
          <section className="np-card rounded-xl p-5 space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h3 className="text-sm font-semibold text-slate-100">
                  Verificador Antes vs. Después
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Guardá una foto de tu latencia y congestión antes de tocar el módem, y medí de nuevo después del cambio.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={onCaptureBaseline}
                  className="px-3.5 py-2 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-slate-200 border border-white/[0.1] cursor-pointer"
                >
                  1. Guardar Base (Antes)
                </button>
                <button
                  type="button"
                  onClick={onCompareAfter}
                  disabled={!beforeAfter?.hasBaseline}
                  className="px-3.5 py-2 rounded-lg text-xs font-medium bg-emerald-500/15 hover:bg-emerald-500/25 disabled:opacity-45 text-emerald-300 border border-emerald-500/35 cursor-pointer"
                >
                  2. Medir Después
                </button>
              </div>
            </div>

            {beforeAfter?.hasBaseline ? (
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 font-mono-telemetry">
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3.5">
                  <div className="text-[10px] text-slate-400">
                    ANTES ({beforeAfter.baseline.capturedAt})
                  </div>
                  <div className="text-xs text-slate-200 mt-1.5 space-y-1">
                    <div>Router: {beforeAfter.baseline.routerAvgMs.toFixed(1)} ms (jit {beforeAfter.baseline.routerJitterMs.toFixed(1)} ms)</div>
                    <div>Puntaje: {beforeAfter.baseline.score}/100</div>
                    {beforeAfter.baseline.wifiChannel > 0 && (
                      <div>Wi-Fi: Ch {beforeAfter.baseline.wifiChannel} ({beforeAfter.baseline.wifiCongestion}% cong.)</div>
                    )}
                  </div>
                </div>

                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3.5">
                  <div className="text-[10px] text-slate-400">
                    DESPUÉS {beforeAfter.after ? `(${beforeAfter.after.capturedAt})` : '(Pendiente)'}
                  </div>
                  {beforeAfter.after ? (
                    <div className="text-xs text-slate-200 mt-1.5 space-y-1">
                      <div>Router: {beforeAfter.after.routerAvgMs.toFixed(1)} ms (jit {beforeAfter.after.routerJitterMs.toFixed(1)} ms)</div>
                      <div>Puntaje: {beforeAfter.after.score}/100</div>
                      {beforeAfter.after.wifiChannel > 0 && (
                        <div>Wi-Fi: Ch {beforeAfter.after.wifiChannel} ({beforeAfter.after.wifiCongestion}% cong.)</div>
                      )}
                    </div>
                  ) : (
                    <div className="text-xs text-slate-500 mt-2">
                      Hacé el cambio en el router y tocá &quot;Medir Después&quot;.
                    </div>
                  )}
                </div>

                <div className="rounded-lg bg-cyan-500/[0.05] border border-cyan-500/25 p-3.5 flex flex-col justify-center">
                  <div className="text-[10px] text-cyan-300 uppercase">Resultado</div>
                  <div className="text-xs font-sans text-slate-200 mt-1 leading-relaxed">
                    {beforeAfter.verdict || 'Base guardada correctamente.'}
                  </div>
                </div>
              </div>
            ) : (
              <div className="rounded-lg bg-[#090D16] border border-white/[0.06] py-5 text-center text-xs text-slate-400">
                Tocá <strong>1. Guardar Base (Antes)</strong> antes de modificar la configuración de tu módem.
              </div>
            )}
          </section>
        </>
      ) : (
        /* Auto-Pilot Tab */
        <div className="space-y-4">
          <section className="np-card rounded-xl p-5 space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <KeyRound className="h-4 w-4 text-emerald-400" />
                <h3 className="text-sm font-semibold text-slate-100">
                  Conexión Segura por API / SSH (Windows Credential Manager)
                </h3>
              </div>

              {hasSavedCreds && (
                <div className="flex items-center gap-2">
                  <span className="font-mono-telemetry text-[11px] px-2 py-0.5 rounded bg-emerald-500/12 text-emerald-300 border border-emerald-500/30">
                    Credencial DPAPI guardada
                  </span>
                  <button
                    type="button"
                    onClick={async () => {
                      await onClearSavedCreds(driver);
                      setHasSavedCreds(false);
                      setPassword('');
                    }}
                    className="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/25 cursor-pointer"
                    title="Borrar credenciales guardadas"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              )}
            </div>

            <div className="grid grid-cols-1 md:grid-cols-6 gap-3">
              <div className="md:col-span-2">
                <label className="block text-[11px] text-slate-400 mb-1">
                  Conector / Firmware
                </label>
                <select
                  value={driver}
                  onChange={(e) =>
                    setDriver(e.target.value as 'openwrt' | 'mikrotik' | 'tr064')
                  }
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs text-slate-100 focus:outline-none focus:border-cyan-500/50 cursor-pointer"
                >
                  {DRIVER_OPTIONS.map((opt) => (
                    <option key={opt.id} value={opt.id}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="md:col-span-2">
                <label className="block text-[11px] text-slate-400 mb-1">
                  IP del Router (Solo LAN privada)
                </label>
                <input
                  type="text"
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                  placeholder="192.168.1.1"
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs font-mono-telemetry text-slate-100 focus:outline-none focus:border-cyan-500/50"
                />
              </div>

              <div>
                <label className="block text-[11px] text-slate-400 mb-1">
                  Puerto
                </label>
                <input
                  type="number"
                  value={port}
                  onChange={(e) => setPort(Number(e.target.value))}
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs font-mono-telemetry text-slate-100 focus:outline-none focus:border-cyan-500/50"
                />
              </div>

              <div className="flex items-end">
                {driver !== 'openwrt' && (
                  <label className="inline-flex items-center gap-2 text-xs text-slate-300 pb-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={useTLS}
                      onChange={(e) => setUseTLS(e.target.checked)}
                      className="rounded border-white/20 bg-[#090D16]"
                    />
                    Usar HTTPS
                  </label>
                )}
              </div>

              <div className="md:col-span-2">
                <label className="block text-[11px] text-slate-400 mb-1">
                  Usuario
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs font-mono-telemetry text-slate-100 focus:outline-none focus:border-cyan-500/50"
                />
              </div>

              <div className="md:col-span-2">
                <label className="block text-[11px] text-slate-400 mb-1">
                  Contraseña {hasSavedCreds ? '(dejá vacío para usar guardada)' : ''}
                </label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={hasSavedCreds ? '•••••••• (en bóveda Windows)' : 'Contraseña del router'}
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs font-mono-telemetry text-slate-100 focus:outline-none focus:border-cyan-500/50"
                />
              </div>

              <div className="md:col-span-2 flex items-end justify-between gap-2">
                <label className="inline-flex items-center gap-2 text-xs text-slate-300 pb-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={saveCreds}
                    onChange={(e) => setSaveCreds(e.target.checked)}
                  />
                  Guardar en Windows Vault
                </label>
                <button
                  type="button"
                  onClick={handleTestConn}
                  disabled={testingConn}
                  className="px-4 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 text-cyan-300 border border-cyan-500/35 cursor-pointer shrink-0"
                >
                  {testingConn ? 'Conectando...' : 'Probar Conexión'}
                </button>
              </div>
            </div>

            {apError && (
              <div className="rounded-lg bg-rose-500/10 border border-rose-500/30 px-3.5 py-2.5 flex flex-wrap items-center justify-between gap-2 text-xs text-rose-300">
                <div className="flex items-center gap-2">
                  <ShieldAlert className="h-4 w-4 shrink-0" />
                  <span>{apError}</span>
                </div>
                {driver === 'openwrt' && (
                  <button
                    type="button"
                    onClick={handleClearSSHFingerprint}
                    disabled={clearingFP}
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-medium bg-white/[0.06] hover:bg-white/[0.12] text-slate-200 border border-white/[0.12] cursor-pointer"
                  >
                    <Trash2 className="h-3 w-3" />
                    {clearingFP ? 'Restableciendo...' : 'Restablecer huella SSH'}
                  </button>
                )}
              </div>
            )}

            {fpNotice && (
              <div className="rounded-lg bg-cyan-500/10 border border-cyan-500/30 px-3.5 py-2.5 text-xs text-cyan-200">
                {fpNotice}
              </div>
            )}

            {apStatus?.connected && (
              <div className="rounded-lg bg-emerald-500/[0.06] border border-emerald-500/30 p-3.5 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2 text-xs font-medium text-emerald-300">
                    <ShieldCheck className="h-4 w-4" />
                    <span>{apStatus.message}</span>
                  </div>
                  {apStatus.hostFingerprint && (
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono-telemetry text-[11px] text-slate-300 bg-[#090D16] border border-white/[0.08] px-2 py-1 rounded">
                        <KeyRound className="h-3 w-3 inline mr-1 text-cyan-300" />
                        {apStatus.hostFingerprint}
                      </span>
                      <button
                        type="button"
                        onClick={handleClearSSHFingerprint}
                        disabled={clearingFP}
                        className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-medium bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 border border-white/[0.1] cursor-pointer"
                      >
                        <Trash2 className="h-3 w-3" />
                        {clearingFP ? 'Borrando...' : 'Restablecer huella SSH'}
                      </button>
                    </div>
                  )}
                </div>

                {apStatus.radios?.length > 0 && (
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                    {apStatus.radios.map((r) => (
                      <button
                        key={r.interfaceName}
                        type="button"
                        onClick={() => {
                          setSelectedIface(r.interfaceName);
                          const b = r.band === '2.4GHz' ? '2.4GHz' : '5GHz';
                          setSelectedBand(b);
                          setTargetChannel(
                            b === '2.4GHz'
                              ? profile?.rec24Channel || 11
                              : profile?.rec5Channel || 36,
                          );
                        }}
                        className={`rounded-lg p-2.5 border text-left font-mono-telemetry text-xs cursor-pointer ${
                          selectedIface === r.interfaceName
                            ? 'bg-cyan-500/15 border-cyan-500/40 text-cyan-200'
                            : 'bg-[#090D16] border-white/[0.08] text-slate-300'
                        }`}
                      >
                        <div className="font-semibold">{r.interfaceName}</div>
                        <div className="text-[11px] opacity-80 mt-0.5">
                          {r.band} · Canal {r.currentChannel} ({r.widthMHz} MHz)
                        </div>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
          </section>

          {/* Safe Channel Application & Rollback */}
          <section className="np-card rounded-xl p-5 space-y-4">
            <h3 className="text-sm font-semibold text-slate-100">
              Aplicar Canal Wi-Fi con Guardarraíles & Rollback
            </h3>

            <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
              <div>
                <label className="block text-[11px] text-slate-400 mb-1">
                  Interfaz / Radio
                </label>
                <input
                  type="text"
                  value={selectedIface}
                  onChange={(e) => setSelectedIface(e.target.value)}
                  placeholder="radio0 / wlan1 / wlanconfig1"
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs font-mono-telemetry text-slate-100 focus:outline-none focus:border-cyan-500/50"
                />
              </div>

              <div>
                <label className="block text-[11px] text-slate-400 mb-1">
                  Banda
                </label>
                <select
                  value={selectedBand}
                  onChange={(e) => {
                    const b = e.target.value as '2.4GHz' | '5GHz';
                    setSelectedBand(b);
                    setTargetChannel(b === '2.4GHz' ? 11 : 36);
                  }}
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs text-slate-100 focus:outline-none focus:border-cyan-500/50 cursor-pointer"
                >
                  <option value="2.4GHz">2.4 GHz (Solo 1, 6, 11)</option>
                  <option value="5GHz">5 GHz</option>
                </select>
              </div>

              <div>
                <label className="block text-[11px] text-slate-400 mb-1">
                  Canal Destino
                </label>
                <select
                  value={targetChannel}
                  onChange={(e) => setTargetChannel(Number(e.target.value))}
                  className="w-full rounded-lg bg-[#090D16] border border-white/[0.1] px-3 py-2 text-xs font-mono-telemetry text-slate-100 focus:outline-none focus:border-cyan-500/50 cursor-pointer"
                >
                  {availableChannels.map((ch) => (
                    <option key={ch} value={ch}>
                      Canal {ch}
                      {selectedBand === '5GHz' && CHANNELS_5_DFS.includes(ch) ? ' (DFS)' : ''}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex items-end pb-2">
                {selectedBand === '5GHz' && (
                  <label className="inline-flex items-center gap-2 text-xs text-amber-300 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allowDFS}
                      onChange={(e) => setAllowDFS(e.target.checked)}
                    />
                    Desbloquear canales DFS
                  </label>
                )}
              </div>
            </div>

            <div className="rounded-lg bg-amber-500/[0.06] border border-amber-500/25 p-3 flex items-start gap-2.5">
              <AlertTriangle className="h-4 w-4 text-amber-400 shrink-0 mt-0.5" />
              <label className="text-xs text-slate-200 cursor-pointer select-none leading-relaxed">
                <input
                  type="checkbox"
                  checked={confirmedDisconnect}
                  onChange={(e) => setConfirmedDisconnect(e.target.checked)}
                  className="mr-2 align-middle"
                />
                Entiendo que cambiar el canal Wi-Fi reinicia la radio del router y desconectará brevemente a los dispositivos conectados (~5–10 segundos).
              </label>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={handleApply}
                disabled={applyingChannel || !confirmedDisconnect || !selectedIface}
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium bg-emerald-500/18 hover:bg-emerald-500/28 disabled:opacity-45 text-emerald-200 border border-emerald-500/40 cursor-pointer"
              >
                <ArrowRight className="h-3.5 w-3.5" />
                {applyingChannel ? 'Aplicando y verificando...' : 'Aplicar Canal Seguro'}
              </button>

              {lastBackup && (
                <button
                  type="button"
                  onClick={handleRollback}
                  disabled={rollingBack}
                  className="inline-flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-medium bg-amber-500/15 hover:bg-amber-500/25 text-amber-200 border border-amber-500/35 cursor-pointer"
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                  {rollingBack
                    ? 'Restaurando...'
                    : `Rollback al Canal ${lastBackup.previousChannel}`}
                </button>
              )}
            </div>

            {changeResult && (
              <div
                className={`rounded-lg px-3.5 py-2.5 border flex items-center gap-2 text-xs ${
                  changeResult.connectivityOk
                    ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                    : 'bg-amber-500/10 border-amber-500/30 text-amber-200'
                }`}
              >
                <CheckCircle2 className="h-4 w-4 shrink-0" />
                <span>{changeResult.message}</span>
              </div>
            )}
          </section>
        </div>
      )}
    </div>
  );
}
