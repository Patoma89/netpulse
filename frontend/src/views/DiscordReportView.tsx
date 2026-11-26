import { useState } from 'react';
import {
  Check,
  CheckCircle2,
  Copy,
  Download,
  FileText,
  Image as ImageIcon,
  Lock,
  RefreshCw,
  Share2,
  ShieldAlert,
  ShieldCheck,
} from 'lucide-react';
import type { report } from '../../wailsjs/go/models';

interface DiscordReportViewProps {
  discordReport: report.DiscordReport | null;
  loading: boolean;
  privacyMode: boolean;
  forceRedact: boolean;
  onToggleForceRedact: (val: boolean) => void;
  onRefresh: () => void;
  onSaveMarkdown: (content: string) => Promise<string>;
}

function severityAccent(severity?: string): string {
  switch (severity) {
    case 'emerald':
      return '#10B981';
    case 'amber':
      return '#F59E0B';
    case 'rose':
      return '#F43F5E';
    default:
      return '#06B6D4';
  }
}

function drawRoundedRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + w - r, y);
  ctx.quadraticCurveTo(x + w, y, x + w, y + r);
  ctx.lineTo(x + w, y + h - r);
  ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
  ctx.lineTo(x + r, y + h);
  ctx.quadraticCurveTo(x, y + h, x, y + h - r);
  ctx.lineTo(x, y + r);
  ctx.quadraticCurveTo(x, y, x + r, y);
  ctx.closePath();
}

function truncateCanvasText(
  ctx: CanvasRenderingContext2D,
  text: string,
  maxWidth: number,
): string {
  if (ctx.measureText(text).width <= maxWidth) return text;
  let cut = text;
  while (cut.length > 3 && ctx.measureText(cut + '…').width > maxWidth) {
    cut = cut.slice(0, -1);
  }
  return cut + '…';
}

function buildShareCardCanvas(rep: report.DiscordReport): HTMLCanvasElement | null {
  const canvas = document.createElement('canvas');
  canvas.width = 1200;
  canvas.height = 660;
  const ctx = canvas.getContext('2d');
  if (!ctx) return null;

  // Base Dark Background
  ctx.fillStyle = '#090D16';
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  // Subtle top glow
  const grad = ctx.createLinearGradient(0, 0, 1200, 0);
  grad.addColorStop(0, 'rgba(6, 182, 212, 0.14)');
  grad.addColorStop(1, 'rgba(16, 185, 129, 0.10)');
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, 1200, 110);

  // Header
  ctx.fillStyle = '#06B6D4';
  ctx.beginPath();
  ctx.arc(58, 56, 8, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = '#F1F5F9';
  ctx.font = 'bold 28px Inter, Segoe UI, sans-serif';
  ctx.fillText(`NetPulse v${rep.version}`, 82, 65);

  ctx.fillStyle = '#94A3B8';
  ctx.font = '15px Consolas, monospace';
  ctx.fillText('Tactical Network Telemetry · Reporte Discord', 290, 64);

  // Privacy pill top-right
  const pillColor = rep.redacted ? '#10B981' : '#F59E0B';
  drawRoundedRect(ctx, 845, 34, 315, 42, 10);
  ctx.fillStyle = rep.redacted
    ? 'rgba(16, 185, 129, 0.14)'
    : 'rgba(245, 158, 11, 0.14)';
  ctx.fill();
  ctx.strokeStyle = pillColor;
  ctx.lineWidth = 1.5;
  ctx.stroke();

  ctx.fillStyle = pillColor;
  ctx.font = 'bold 14px Consolas, monospace';
  ctx.fillText(
    rep.redacted ? '100% ANONIMIZADO · DISCORD SAFE' : 'DATOS SIN CENSURA',
    866,
    60,
  );

  // Hero Score & Verdict Card
  const accent = severityAccent(rep.verdictSeverity);
  drawRoundedRect(ctx, 40, 126, 1120, 122, 14);
  ctx.fillStyle = '#111827';
  ctx.fill();
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.09)';
  ctx.lineWidth = 1.5;
  ctx.stroke();

  ctx.fillStyle = '#94A3B8';
  ctx.font = '13px Consolas, monospace';
  ctx.fillText('SALUD DE RED Y VEREDICTO EN VIVO', 68, 160);

  ctx.fillStyle = '#F8FAFC';
  ctx.font = 'bold 28px Inter, Segoe UI, sans-serif';
  ctx.fillText(truncateCanvasText(ctx, rep.verdictTitle, 800), 68, 198);

  ctx.fillStyle = '#94A3B8';
  ctx.font = '16px Consolas, monospace';
  ctx.fillText(`Enlace: ${rep.linkLabel}`, 68, 228);

  // Score right
  ctx.fillStyle = accent;
  ctx.font = 'bold 54px Consolas, monospace';
  const scoreStr = `${rep.healthScore}`;
  const scoreW = ctx.measureText(scoreStr).width;
  ctx.fillText(scoreStr, 1060 - scoreW, 204);
  ctx.fillStyle = '#64748B';
  ctx.font = '20px Consolas, monospace';
  ctx.fillText('/100', 1065, 204);

  // 2x3 Metric Tiles
  const tiles = [
    { label: 'SALTO 1 · ROUTER', value: rep.routerLatency, color: '#06B6D4' },
    { label: 'SALTO 2 · INTERNET', value: rep.internetLatency, color: '#10B981' },
    {
      label: 'BUFFERBLOAT',
      value: `Nota ${rep.bufferbloatGrade}`,
      color: '#F59E0B',
    },
    { label: 'ESPECTRO WI-FI', value: rep.wifiSummary, color: '#38BDF8' },
    { label: 'RADAR LAN', value: rep.radarSummary, color: '#A78BFA' },
    {
      label: 'DNS & UPnP',
      value: `${rep.dnsSummary} | ${rep.upnpSummary}`,
      color: '#34D399',
    },
  ];

  const colW = 548;
  const rowH = 104;
  tiles.forEach((tile, idx) => {
    const col = idx % 2;
    const row = Math.floor(idx / 2);
    const x = 40 + col * (colW + 24);
    const y = 268 + row * (rowH + 16);

    drawRoundedRect(ctx, x, y, colW, rowH, 12);
    ctx.fillStyle = '#111827';
    ctx.fill();
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
    ctx.lineWidth = 1;
    ctx.stroke();

    ctx.fillStyle = tile.color;
    ctx.font = 'bold 13px Consolas, monospace';
    ctx.fillText(tile.label, x + 20, y + 32);

    ctx.fillStyle = '#E2E8F0';
    ctx.font = '16px Inter, Segoe UI, sans-serif';
    ctx.fillText(truncateCanvasText(ctx, tile.value, colW - 40), x + 20, y + 68);
  });

  // Footer
  ctx.fillStyle = '#64748B';
  ctx.font = '13px Consolas, monospace';
  ctx.fillText(
    `Generado 100% offline con NetPulse v${rep.version} · 0% telemetria · Sin IPs, MACs ni SSIDs expuestos`,
    40,
    636,
  );

  return canvas;
}

function exportShareCardPNG(rep: report.DiscordReport) {
  const canvas = buildShareCardCanvas(rep);
  if (!canvas) return;
  const link = document.createElement('a');
  link.download = 'netpulse-tarjeta-discord.png';
  link.href = canvas.toDataURL('image/png');
  link.click();
}

async function copyShareCardImage(rep: report.DiscordReport): Promise<void> {
  const canvas = buildShareCardCanvas(rep);
  if (!canvas) throw new Error('No se pudo renderizar el lienzo');
  const blob = await new Promise<Blob | null>((resolve) =>
    canvas.toBlob((b) => resolve(b), 'image/png'),
  );
  if (!blob) throw new Error('No se pudo generar la imagen PNG');
  await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
}

export function DiscordReportView({
  discordReport,
  loading,
  privacyMode,
  forceRedact,
  onToggleForceRedact,
  onRefresh,
  onSaveMarkdown,
}: DiscordReportViewProps) {
  const [copied, setCopied] = useState(false);
  const [copiedImg, setCopiedImg] = useState(false);
  const [savedPath, setSavedPath] = useState<string | null>(null);
  const [saveErr, setSaveErr] = useState<string | null>(null);

  const effectiveRedact = forceRedact || privacyMode;

  async function handleCopy() {
    if (!discordReport?.markdown) return;
    await navigator.clipboard.writeText(discordReport.markdown);
    setCopied(true);
    setTimeout(() => setCopied(false), 2500);
  }

  async function handleCopyImage() {
    if (!discordReport) return;
    setSaveErr(null);
    try {
      await copyShareCardImage(discordReport);
      setCopiedImg(true);
      setTimeout(() => setCopiedImg(false), 2500);
    } catch (err) {
      setSaveErr(
        err instanceof Error
          ? `No se pudo copiar imagen al portapapeles: ${err.message}`
          : 'No se pudo copiar la imagen al portapapeles',
      );
    }
  }

  async function handleSave() {
    if (!discordReport?.markdown) return;
    setSaveErr(null);
    setSavedPath(null);
    try {
      const p = await onSaveMarkdown(discordReport.markdown);
      if (p) setSavedPath(p);
    } catch (err) {
      setSaveErr(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="space-y-5">
      {/* Top Action Bar */}
      <section className="np-card rounded-xl p-5 flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <Share2 className="h-4 w-4 text-cyan-400" />
            <h2 className="text-sm font-semibold text-slate-100">
              Exportador Seguro para Discord (Markdown + Tarjeta PNG)
            </h2>
          </div>
          <p className="text-xs text-slate-400 mt-1">
            Genera un resumen técnico de todos los módulos listo para compartir en canales sin filtrar IPs, MACs ni ubicación.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <label
            className={`inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs border cursor-pointer ${
              effectiveRedact
                ? 'bg-emerald-500/12 border-emerald-500/30 text-emerald-300'
                : 'bg-amber-500/12 border-amber-500/30 text-amber-300'
            }`}
            title={
              privacyMode
                ? 'El Modo Discord global está activo en la barra superior'
                : 'Fuerza censura de datos sensibles en el reporte exportado'
            }
          >
            <input
              type="checkbox"
              checked={effectiveRedact}
              disabled={privacyMode}
              onChange={(e) => onToggleForceRedact(e.target.checked)}
            />
            <Lock className="h-3.5 w-3.5" />
            <span>Censura Máxima</span>
          </label>

          <button
            type="button"
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-slate-200 border border-white/[0.09] cursor-pointer"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            Actualizar
          </button>

          <button
            type="button"
            onClick={handleCopy}
            disabled={!discordReport}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-medium bg-cyan-500/15 hover:bg-cyan-500/25 disabled:opacity-45 text-cyan-200 border border-cyan-500/35 cursor-pointer"
          >
            {copied ? (
              <>
                <Check className="h-3.5 w-3.5 text-emerald-300" />
                ¡Copiado para Discord!
              </>
            ) : (
              <>
                <Copy className="h-3.5 w-3.5" />
                Copiar Markdown
              </>
            )}
          </button>

          <button
            type="button"
            onClick={handleCopyImage}
            disabled={!discordReport}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-medium bg-emerald-500/15 hover:bg-emerald-500/25 disabled:opacity-45 text-emerald-200 border border-emerald-500/35 cursor-pointer"
          >
            {copiedImg ? (
              <>
                <Check className="h-3.5 w-3.5 text-emerald-300" />
                ¡Imagen copiada! (Ctrl+V)
              </>
            ) : (
              <>
                <ImageIcon className="h-3.5 w-3.5" />
                Copiar Imagen
              </>
            )}
          </button>

          <button
            type="button"
            onClick={() => discordReport && exportShareCardPNG(discordReport)}
            disabled={!discordReport}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] disabled:opacity-45 text-slate-200 border border-white/[0.09] cursor-pointer"
          >
            <Download className="h-3.5 w-3.5" />
            Descargar PNG
          </button>

          <button
            type="button"
            onClick={handleSave}
            disabled={!discordReport}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] disabled:opacity-45 text-slate-200 border border-white/[0.09] cursor-pointer"
          >
            <Download className="h-3.5 w-3.5" />
            Guardar .md
          </button>
        </div>
      </section>

      {savedPath && (
        <div className="rounded-lg bg-emerald-500/10 border border-emerald-500/30 px-4 py-2.5 flex items-center gap-2 text-xs text-emerald-300">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <span>
            Reporte guardado en <code className="font-mono-telemetry">{savedPath}</code>
          </span>
        </div>
      )}

      {saveErr && (
        <div className="rounded-lg bg-rose-500/10 border border-rose-500/30 px-4 py-2.5 text-xs text-rose-300">
          {saveErr}
        </div>
      )}

      {discordReport && (
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-5">
          {/* Left: Visual Card Preview + Privacy Audit */}
          <div className="lg:col-span-7 space-y-4">
            <section className="np-card rounded-xl p-5 border border-cyan-500/20 space-y-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <span className="font-mono-telemetry text-[11px] text-cyan-300">
                    VISTA PREVIA TARJETA · NetPulse v{discordReport.version}
                  </span>
                  <h3 className="text-lg font-semibold text-slate-100 mt-1">
                    {discordReport.verdictTitle}
                  </h3>
                  <div className="font-mono-telemetry text-xs text-slate-400 mt-0.5">
                    Enlace: {discordReport.linkLabel}
                  </div>
                </div>
                <div className="text-right font-mono-telemetry shrink-0">
                  <div className="text-[10px] text-slate-400">PUNTAJE</div>
                  <div className="text-3xl font-bold text-emerald-300">
                    {discordReport.healthScore}
                    <span className="text-xs text-slate-400 font-normal">/100</span>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5 text-xs">
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
                  <div className="font-mono-telemetry text-[10px] text-cyan-300">
                    SALTO 1 · ROUTER
                  </div>
                  <div className="text-slate-200 mt-1">{discordReport.routerLatency}</div>
                </div>
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
                  <div className="font-mono-telemetry text-[10px] text-emerald-300">
                    SALTO 2 · INTERNET
                  </div>
                  <div className="text-slate-200 mt-1">{discordReport.internetLatency}</div>
                </div>
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
                  <div className="font-mono-telemetry text-[10px] text-amber-300">
                    BUFFERBLOAT
                  </div>
                  <div className="text-slate-200 mt-1">
                    Nota: {discordReport.bufferbloatGrade}
                  </div>
                </div>
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
                  <div className="font-mono-telemetry text-[10px] text-sky-300">
                    ESPECTRO WI-FI
                  </div>
                  <div className="text-slate-200 mt-1">{discordReport.wifiSummary}</div>
                </div>
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
                  <div className="font-mono-telemetry text-[10px] text-violet-300">
                    RADAR LAN
                  </div>
                  <div className="text-slate-200 mt-1">{discordReport.radarSummary}</div>
                </div>
                <div className="rounded-lg bg-[#090D16] border border-white/[0.06] p-3">
                  <div className="font-mono-telemetry text-[10px] text-emerald-300">
                    DNS & UPnP
                  </div>
                  <div className="text-slate-200 mt-1">
                    {discordReport.dnsSummary} · {discordReport.upnpSummary}
                  </div>
                </div>
              </div>
            </section>

            {/* Privacy Verification Checklist */}
            <section
              className={`np-card rounded-xl p-4 border ${
                discordReport.redacted
                  ? 'border-emerald-500/30 bg-emerald-500/[0.05]'
                  : 'border-amber-500/30 bg-amber-500/[0.05]'
              }`}
            >
              {discordReport.redacted ? (
                <div className="space-y-2">
                  <div className="flex items-center gap-2 text-xs font-semibold text-emerald-300">
                    <ShieldCheck className="h-4 w-4" />
                    <span>Auditoría de Privacidad: 100% Seguro para Discord</span>
                  </div>
                  <ul className="grid grid-cols-1 sm:grid-cols-2 gap-1.5 text-xs text-slate-300">
                    {discordReport.redactedFields.map((field) => (
                      <li key={field} className="flex items-center gap-1.5">
                        <CheckCircle2 className="h-3.5 w-3.5 text-emerald-400 shrink-0" />
                        <span>{field}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <div className="flex items-center gap-2.5 text-xs text-amber-200">
                  <ShieldAlert className="h-4 w-4 text-amber-400 shrink-0" />
                  <span>
                    Advertencia: desactivaste la censura. Revisá que no haya datos personales antes de pegar en un servidor público.
                  </span>
                </div>
              )}
            </section>
          </div>

          {/* Right: Markdown Code Preview */}
          <section className="lg:col-span-5 np-card rounded-xl p-5 flex flex-col">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2 text-xs font-semibold text-slate-200">
                <FileText className="h-4 w-4 text-cyan-400" />
                <span>Markdown Listo para Pegar en Discord</span>
              </div>
              <span className="font-mono-telemetry text-[10px] text-slate-400">
                UTF-8
              </span>
            </div>
            <pre className="flex-1 rounded-lg bg-[#090D16] border border-white/[0.07] p-3.5 font-mono-telemetry text-[11px] text-slate-200 whitespace-pre-wrap leading-relaxed overflow-y-auto np-scroll">
              {discordReport.markdown}
            </pre>
          </section>
        </div>
      )}
    </div>
  );
}
