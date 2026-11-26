import {
  Cable,
  Eye,
  EyeOff,
  Languages,
  Moon,
  Sun,
  Wifi,
  WifiOff,
} from 'lucide-react';
import type { SectionMeta } from '../types/navigation';
import type { netinfo } from '../../wailsjs/go/models';
import { tr, type UILang, type UITheme } from '../i18n';

interface TopBarProps {
  section: SectionMeta;
  privacyMode: boolean;
  savingPrivacy: boolean;
  onTogglePrivacy: () => void;
  networkSummary: netinfo.NetworkSummary | null;
  lang: UILang;
  theme: UITheme;
  onToggleLang: () => void;
  onToggleTheme: () => void;
}

export function TopBar({
  section,
  privacyMode,
  savingPrivacy,
  onTogglePrivacy,
  networkSummary,
  lang,
  theme,
  onToggleLang,
  onToggleTheme,
}: TopBarProps) {
  const connected = networkSummary?.connected ?? false;
  const linkType = networkSummary?.linkType ?? 'other';
  const speed = networkSummary?.linkSpeedMbps ?? 0;

  let statusLabel = tr(lang, 'topbar.detecting');
  if (networkSummary) {
    if (!connected) {
      statusLabel = tr(lang, 'topbar.noNet');
    } else {
      const kind =
        linkType === 'wifi'
          ? 'Wi-Fi'
          : linkType === 'ethernet'
          ? 'Ethernet'
          : lang === 'en'
          ? 'Net'
          : 'Red';
      const prefix = lang === 'en' ? 'Link' : 'Red';
      statusLabel =
        speed > 0 ? `${prefix}: ${kind} · ${speed} Mbps` : `${prefix}: ${kind}`;
    }
  }

  return (
    <header className="h-16 shrink-0 px-6 border-b border-white/[0.07] bg-[#090D16]/90 flex items-center justify-between gap-4">
      <div className="min-w-0">
        <div className="flex items-center gap-2.5">
          <h1 className="text-base font-semibold text-slate-100 truncate">
            {tr(lang, `sec.${section.id}.title`, section.title)}
          </h1>
          <span className="font-mono-telemetry text-[11px] px-2 py-0.5 rounded-full bg-white/[0.05] border border-white/[0.08] text-slate-300 shrink-0">
            {section.phase}
          </span>
        </div>
        <p className="text-xs text-slate-400 truncate mt-0.5">
          {tr(lang, `sec.${section.id}.subtitle`, section.subtitle)}
        </p>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <div
          className={`hidden md:flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-mono-telemetry border ${
            connected
              ? 'bg-white/[0.03] border-white/[0.08] text-slate-200'
              : 'bg-rose-500/10 border-rose-500/25 text-rose-300'
          }`}
          title={networkSummary?.adapterName || 'Estado de enlace principal'}
        >
          <span
            className={`h-2 w-2 rounded-full ${
              connected
                ? 'bg-emerald-400 shadow-[0_0_8px_#10B981]'
                : 'bg-rose-400 shadow-[0_0_8px_#F43F5E]'
            }`}
          />
          {!connected ? (
            <WifiOff className="h-3.5 w-3.5 text-rose-400" />
          ) : linkType === 'wifi' ? (
            <Wifi className="h-3.5 w-3.5 text-cyan-400" />
          ) : (
            <Cable className="h-3.5 w-3.5 text-emerald-400" />
          )}
          <span>{statusLabel}</span>
        </div>

        <button
          type="button"
          onClick={onToggleLang}
          title={tr(lang, 'topbar.langTitle')}
          className="flex items-center gap-1 px-2.5 py-2 rounded-lg text-xs font-mono-telemetry bg-white/[0.04] hover:bg-white/[0.09] text-slate-200 border border-white/[0.08] cursor-pointer"
        >
          <Languages className="h-3.5 w-3.5 text-cyan-400" />
          <span>{lang.toUpperCase()}</span>
        </button>

        <button
          type="button"
          onClick={onToggleTheme}
          title={
            theme === 'dark'
              ? tr(lang, 'topbar.themeLight')
              : tr(lang, 'topbar.themeDark')
          }
          className="p-2 rounded-lg bg-white/[0.04] hover:bg-white/[0.09] text-slate-200 border border-white/[0.08] cursor-pointer"
        >
          {theme === 'dark' ? (
            <Sun className="h-3.5 w-3.5 text-amber-300" />
          ) : (
            <Moon className="h-3.5 w-3.5 text-cyan-400" />
          )}
        </button>

        <button
          type="button"
          onClick={onTogglePrivacy}
          disabled={savingPrivacy}
          title={tr(lang, 'topbar.privacyTitle')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-medium border transition-all cursor-pointer shrink-0 ${
            privacyMode
              ? 'bg-emerald-500/12 border-emerald-500/35 text-emerald-300 hover:bg-emerald-500/20'
              : 'bg-amber-500/12 border-amber-500/35 text-amber-300 hover:bg-amber-500/20'
          } ${savingPrivacy ? 'opacity-60 cursor-wait' : ''}`}
        >
          {privacyMode ? (
            <EyeOff className="h-3.5 w-3.5 text-emerald-400" />
          ) : (
            <Eye className="h-3.5 w-3.5 text-amber-400" />
          )}
          <span>
            {privacyMode
              ? tr(lang, 'topbar.privacyOn')
              : tr(lang, 'topbar.privacyOff')}
          </span>
        </button>
      </div>
    </header>
  );
}
