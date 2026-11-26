import { Lock, Unlock } from 'lucide-react';

interface PrivacyValueProps {
  label: string;
  value: string;
  privacyMode: boolean;
}

export function PrivacyValue({ label, value, privacyMode }: PrivacyValueProps) {
  return (
    <div className="rounded-lg bg-[#090D16] border border-white/[0.07] px-3.5 py-2.5 flex items-center justify-between gap-3">
      <div className="min-w-0">
        <div className="text-[11px] uppercase tracking-wider text-slate-400">
          {label}
        </div>
        <div
          className={`font-mono-telemetry text-sm mt-0.5 truncate ${
            privacyMode ? 'text-emerald-300' : 'text-slate-100'
          }`}
        >
          {value || '—'}
        </div>
      </div>
      <span
        className={`inline-flex items-center gap-1 text-[10px] font-mono-telemetry px-2 py-0.5 rounded border shrink-0 ${
          privacyMode
            ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/25'
            : 'bg-amber-500/10 text-amber-300 border-amber-500/25'
        }`}
      >
        {privacyMode ? (
          <>
            <Lock className="h-3 w-3" />
            OCULTO
          </>
        ) : (
          <>
            <Unlock className="h-3 w-3" />
            VISIBLE
          </>
        )}
      </span>
    </div>
  );
}
