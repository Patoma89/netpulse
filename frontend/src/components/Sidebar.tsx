import {
  Activity,
  Cpu,
  Radar,
  Wifi,
  Zap,
  Router,
  Share2,
  ShieldCheck,
} from 'lucide-react';
import type { NavSection } from '../types/navigation';
import { tr, type UILang } from '../i18n';

interface SidebarProps {
  activeSection: NavSection;
  onSelectSection: (section: NavSection) => void;
  version: string;
  onOpenPatchNotes?: () => void;
  lang?: UILang;
  radarTotalCount?: number;
  radarUnknownCount?: number;
  wifiAPCount?: number;
  wifiStatus?: string;
  processCount?: number;
  upnpHighRiskCount?: number;
}

const NAV_ITEMS: Array<{
  id: NavSection;
  label: string;
  phaseBadge?: string;
  icon: typeof Activity;
}> = [
  { id: 'dashboard', label: 'Centro de Mando', icon: Activity },
  { id: 'radar', label: 'Radar LAN', icon: Radar },
  { id: 'wifi', label: 'Espectro Wi-Fi', icon: Wifi },
  { id: 'processes', label: 'Conexiones', icon: Cpu },
  { id: 'optimizer', label: 'DNS & UPnP', icon: Zap },
  { id: 'router', label: 'Piloto Router', icon: Router },
  { id: 'discord', label: 'Reporte Discord', icon: Share2 },
];

export function Sidebar({
  activeSection,
  onSelectSection,
  version,
  onOpenPatchNotes,
  lang = 'es',
  radarTotalCount = 0,
  radarUnknownCount = 0,
  wifiAPCount = 0,
  wifiStatus = '',
  processCount = 0,
  upnpHighRiskCount = 0,
}: SidebarProps) {
  return (
    <aside className="w-60 shrink-0 bg-[#0B101B] border-r border-white/[0.07] flex flex-col justify-between select-none">
      <div>
        {/* Brand Header */}
        <div className="h-16 px-5 flex items-center justify-between border-b border-white/[0.07]">
          <div className="flex items-center gap-2.5">
            <div className="relative flex h-8 w-8 items-center justify-center rounded-lg bg-cyan-500/10 border border-cyan-500/30">
              <span className="h-2.5 w-2.5 rounded-full bg-cyan-400 shadow-[0_0_10px_#06B6D4]" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold tracking-tight text-slate-100">
                  NetPulse
                </span>
                <button
                  type="button"
                  onClick={onOpenPatchNotes}
                  title={
                    lang === 'en'
                      ? 'View Patch Notes / Changelog'
                      : 'Ver Patch Notes / Novedades'
                  }
                  className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.06] hover:bg-cyan-500/20 text-slate-300 hover:text-cyan-200 border border-white/[0.08] hover:border-cyan-500/35 transition-colors cursor-pointer"
                >
                  v{version}
                </button>
              </div>
              <p className="text-[11px] text-slate-400 leading-none mt-0.5">
                Tactical Telemetry
              </p>
            </div>
          </div>
        </div>

        {/* Navigation Links */}
        <nav className="p-3 space-y-1">
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            const active = activeSection === item.id;

            let rightBadge: React.ReactNode = null;
            if (item.id === 'radar') {
              rightBadge =
                radarTotalCount > 0 ? (
                  <span
                    className={`font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded border ${
                      radarUnknownCount > 0
                        ? 'bg-amber-500/15 text-amber-300 border-amber-500/35'
                        : 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30'
                    }`}
                  >
                    {radarTotalCount}
                  </span>
                ) : (
                  <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                    LAN
                  </span>
                );
            } else if (item.id === 'wifi') {
              if (wifiStatus === 'no_wifi_adapter') {
                rightBadge = (
                  <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                    ETH
                  </span>
                );
              } else if (wifiStatus === 'location_denied') {
                rightBadge = (
                  <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-300 border border-amber-500/35">
                    !
                  </span>
                );
              } else if (wifiAPCount > 0) {
                rightBadge = (
                  <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-cyan-500/15 text-cyan-300 border border-cyan-500/30">
                    {wifiAPCount}
                  </span>
                );
              } else {
                rightBadge = (
                  <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                    RF
                  </span>
                );
              }
            } else if (item.id === 'processes') {
              rightBadge = (
                <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                  {processCount > 0 ? processCount : 'APP'}
                </span>
              );
            } else if (item.id === 'optimizer') {
              rightBadge =
                upnpHighRiskCount > 0 ? (
                  <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-rose-500/15 text-rose-300 border border-rose-500/35">
                    {upnpHighRiskCount}
                  </span>
                ) : (
                  <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                    DNS
                  </span>
                );
            } else if (item.id === 'router') {
              rightBadge = (
                <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                  PILOT
                </span>
              );
            } else if (item.id === 'discord') {
              rightBadge = (
                <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/12 text-emerald-300 border border-emerald-500/25">
                  SHARE
                </span>
              );
            } else if (item.phaseBadge) {
              rightBadge = (
                <span className="font-mono-telemetry text-[10px] px-1.5 py-0.5 rounded bg-white/[0.05] text-slate-400">
                  {item.phaseBadge}
                </span>
              );
            }

            return (
              <button
                key={item.id}
                type="button"
                onClick={() => onSelectSection(item.id)}
                className={`w-full flex items-center justify-between px-3 py-2.5 rounded-lg text-sm transition-colors cursor-pointer ${
                  active
                    ? 'bg-cyan-500/12 text-cyan-300 border border-cyan-500/25'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.04] border border-transparent'
                }`}
              >
                <span className="flex items-center gap-2.5">
                  <Icon
                    className={`h-4 w-4 ${
                      active ? 'text-cyan-400' : 'text-slate-400'
                    }`}
                  />
                  <span className="font-medium">
                    {tr(lang, `nav.${item.id}`, item.label)}
                  </span>
                </span>
                {rightBadge}
              </button>
            );
          })}
        </nav>
      </div>

      {/* Zero-Telemetry Local Guarantee Footer */}
      <div className="p-3 border-t border-white/[0.07]">
        <div className="rounded-lg bg-emerald-500/[0.07] border border-emerald-500/20 px-3 py-2.5">
          <div className="flex items-center gap-2 text-emerald-400 text-xs font-medium">
            <ShieldCheck className="h-4 w-4 shrink-0" />
            <span>
              {lang === 'en'
                ? '100% Local · 0 Telemetry'
                : '100% Local · 0 Telemetría'}
            </span>
          </div>
          <p className="text-[11px] text-slate-400 mt-1 leading-snug">
            {lang === 'en'
              ? 'No accounts, no cloud, zero background uploads.'
              : 'Sin cuentas, sin nube y sin datos enviados fuera de tu PC.'}
          </p>
        </div>
      </div>
    </aside>
  );
}
