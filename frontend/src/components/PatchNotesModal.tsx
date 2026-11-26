import { useEffect } from 'react';
import { Sparkles, X } from 'lucide-react';
import type { UILang } from '../i18n';

interface PatchNotesModalProps {
  open: boolean;
  onClose: () => void;
  lang: UILang;
}

interface ReleaseEntry {
  version: string;
  tag: string;
  badge: string;
  highlights: string[];
}

const RELEASES_ES: ReleaseEntry[] = [
  {
    version: 'v1.2.0',
    tag: 'Pro Edition',
    badge: 'ACTUAL',
    highlights: [
      'MTR continuo estilo WinMTR salto por salto con pérdida real y jitter sin permisos de Administrador.',
      'Auditoría ISP & Gaming en 1 clic: detección de CGNAT (100.64.0.0/10), Doble NAT, Path MTU (flag DF) e IPv6 real.',
      'Histórico persistente de microcortes de 30 días exportable en CSV (Excel) y HTML técnico listo para reclamos al ISP.',
      'Monitor de Conexiones por Proceso (sin drivers de kernel ni Npcap): identifica qué .exe usa sockets TCP/UDP e I/O.',
      'Perfiles de red en Radar LAN (Casa/Oficina/Hotspot), contador de presencia y alerta anti cambio de MAC del Gateway.',
      'Instancia única Win32, inicio minimizado con Windows opcional, idioma ES/EN, tema Oscuro/Claro y atajos Ctrl+Shift+D / F5.',
      'Gráficos Dual-Hop y Espectro Wi-Fi con tipografía nítida sin deformación SVG, escala Y en múltiplos limpios y manifiestos WinGet/Scoop.',
    ],
  },
  {
    version: 'v1.1.0',
    tag: 'Hardening & Tray',
    badge: 'ANTERIOR',
    highlights: [
      'Registro en vivo de microcortes y picos en el Centro de Mando, Benchmark de Regiones Gaming y Traceroute Win32 sin Admin.',
      'Base IEEE OUI ampliada offline, inspección suave de 9 puertos por equipo y Wake-on-LAN (Magic Packet UDP).',
      'Soporte de banda Wi-Fi 6 GHz (Wi-Fi 6E/7) y verificación SSH TOFU (Trust On First Use) para OpenWrt.',
      'Bandeja del sistema Windows con minimizado opcional, vigilancia de intrusos y copia directa de tarjeta PNG al portapapeles.',
    ],
  },
  {
    version: 'v1.0.0',
    tag: 'Lanzamiento Inicial',
    badge: 'BASE',
    highlights: [
      'Monitor Dual-Hop simultáneo cada 500 ms (Router vs Internet) con veredicto automático Wi-Fi vs ISP y test de Bufferbloat (A+ a F).',
      'Radar LAN multi-protocolo (ARP + mDNS + SSDP/UPnP + NetBIOS) y Espectro Wi-Fi nativo Win32 (wlanapi.dll).',
      'Optimizador DNS paralelo con rollback DHCP, Auditor UPnP, Co-Piloto / Auto-Piloto Router y Modo Discord activado por defecto.',
    ],
  },
];

const RELEASES_EN: ReleaseEntry[] = [
  {
    version: 'v1.2.0',
    tag: 'Pro Edition',
    badge: 'CURRENT',
    highlights: [
      'Continuous WinMTR-style hop probing showing real packet loss & jitter without Admin rights.',
      '1-click ISP & Gaming Audit: detects CGNAT (100.64.0.0/10), Double NAT, Path MTU (DF bit), and end-to-end IPv6.',
      '30-day persistent stability incident history with CSV & printable HTML ISP claim exports.',
      'Driverless Per-Process Connections monitor (TCP/UDP sockets & I/O via Win32 IP Helper, zero kernel drivers).',
      'Per-network Radar LAN profiles, device presence tracking, and Gateway MAC change / ARP spoofing alert.',
      'Win32 single-instance lock, optional start with Windows (--minimized), ES/EN switch, Dark/Light theme & Ctrl+Shift+D / F5 shortcuts.',
      'Crisp unstretched Dual-Hop & Wi-Fi spectrum charts, round Y-axis steps, and WinGet/Scoop manifests.',
    ],
  },
  {
    version: 'v1.1.0',
    tag: 'Hardening & Tray',
    badge: 'PREVIOUS',
    highlights: [
      'Live micro-outage & spike log, Gaming Regions ping benchmark, and Admin-free Win32 ICMP Traceroute.',
      'Expanded offline OUI vendor database, gentle 9-port service inspection, and Wake-on-LAN magic packets.',
      'Wi-Fi 6 GHz spectrum analysis and OpenWrt SSH TOFU host-key verification.',
      'Native Windows system tray icon with background intruder alerts and 1-click PNG clipboard copy for Discord.',
    ],
  },
  {
    version: 'v1.0.0',
    tag: 'Initial Release',
    badge: 'BASE',
    highlights: [
      '500ms Dual-Hop latency monitor (Router vs Internet), Wi-Fi vs ISP live verdict, and Bufferbloat A+–F test.',
      'Multi-protocol LAN Radar and native Win32 wlanapi.dll Wi-Fi spectrum analyzer (2.4 / 5 GHz).',
      'Parallel DNS benchmark with DHCP rollback, UPnP port audit, Router Pilot, and default-on Discord Privacy Mode.',
    ],
  },
];

export function PatchNotesModal({ open, onClose, lang }: PatchNotesModalProps) {
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open) return null;

  const en = lang === 'en';
  const releases = en ? RELEASES_EN : RELEASES_ES;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/65 backdrop-blur-xs p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-2xl max-h-[82vh] rounded-2xl bg-[#0B101B] border border-white/[0.12] shadow-2xl flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-5 py-4 border-b border-white/[0.08] flex items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <div className="h-8 w-8 rounded-lg bg-cyan-500/15 border border-cyan-500/35 flex items-center justify-center text-cyan-300">
              <Sparkles className="h-4 w-4" />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-slate-100">
                {en ? 'NetPulse Patch Notes & Changelog' : 'Patch Notes & Novedades de NetPulse'}
              </h2>
              <p className="text-[11px] text-slate-400">
                {en
                  ? '100% Local-First · Zero Telemetry · Zero Kernel Drivers'
                  : '100% Local-First · Cero Telemetría · Sin Drivers de Kernel'}
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.1] text-slate-300 border border-white/[0.08] cursor-pointer"
            aria-label="Cerrar"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto np-scroll p-5 space-y-4">
          {releases.map((rel, idx) => (
            <div
              key={rel.version}
              className={`rounded-xl p-4 border space-y-2.5 ${
                idx === 0
                  ? 'bg-cyan-500/[0.06] border-cyan-500/30'
                  : 'bg-[#090D16] border-white/[0.07]'
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <span className="font-mono-telemetry text-sm font-bold text-slate-100">
                    {rel.version}
                  </span>
                  <span className="text-xs text-slate-400">· {rel.tag}</span>
                </div>
                <span
                  className={`font-mono-telemetry text-[10px] px-2 py-0.5 rounded border ${
                    idx === 0
                      ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30'
                      : 'bg-white/[0.05] text-slate-400 border-white/[0.08]'
                  }`}
                >
                  {rel.badge}
                </span>
              </div>

              <ul className="space-y-1.5 text-xs text-slate-300 leading-relaxed list-disc pl-4">
                {rel.highlights.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="px-5 py-3 border-t border-white/[0.08] flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-1.5 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 text-cyan-200 border border-cyan-500/35 cursor-pointer"
          >
            {en ? 'Close' : 'Cerrar'}
          </button>
        </div>
      </div>
    </div>
  );
}
