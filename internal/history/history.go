package history

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"netpulse/internal/privacy"
)

const (
	MaxRetentionDays = 30
	MaxStoredRecords = 1500
)

var ipv4Regex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// IncidentRecord stores a persisted network microcut or latency spike event.
type IncidentRecord struct {
	ID          int64   `json:"id"`
	Timestamp   int64   `json:"timestamp"`
	TimeISO     string  `json:"timeIso"`
	NetworkName string  `json:"networkName"`
	Hop         string  `json:"hop"`      // "router" | "internet"
	Kind        string  `json:"kind"`     // "loss" | "spike"
	Severity    string  `json:"severity"` // "amber" | "rose"
	DurationMs  int64   `json:"durationMs"`
	PeakMs      float64 `json:"peakMs"`
	Message     string  `json:"message"`
}

// DailySummary aggregates incidents per calendar day for 30-day ISP evidence reports.
type DailySummary struct {
	Date           string  `json:"date"`
	TotalIncidents int     `json:"totalIncidents"`
	LossIncidents  int     `json:"lossIncidents"`
	SpikeIncidents int     `json:"spikeIncidents"`
	WorstPeakMs    float64 `json:"worstPeakMs"`
}

// HistorySnapshot holds filtered incident records and daily counts.
type HistorySnapshot struct {
	TotalStored   int              `json:"totalStored"`
	RetentionDays int              `json:"retentionDays"`
	Incidents     []IncidentRecord `json:"incidents"`
	Daily         []DailySummary   `json:"daily"`
}

type historyFile struct {
	NextID    int64            `json:"nextId"`
	Incidents []IncidentRecord `json:"incidents"`
}

// Store persists 30-day rolling stability incidents locally in %APPDATA%\NetPulse\history.json.
type Store struct {
	mu     sync.RWMutex
	path   string
	nextID int64
	items  []IncidentRecord
}

// DefaultPath resolves %APPDATA%\NetPulse\history.json.
func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		return "", errors.New("could not determine user config directory")
	}
	return filepath.Join(base, "NetPulse", "history.json"), nil
}

// NewStore opens or creates the incident history store at the default %APPDATA% path.
func NewStore() (*Store, error) {
	p, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return NewStoreAt(p)
}

// NewStoreAt opens or creates the incident history store at an explicit path.
func NewStoreAt(path string) (*Store, error) {
	s := &Store{
		path:  path,
		items: make([]IncidentRecord, 0, 64),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.items = make([]IncidentRecord, 0, 64)
			return nil
		}
		return err
	}

	if len(data) == 0 {
		s.items = make([]IncidentRecord, 0, 64)
		return nil
	}

	var parsed historyFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	s.nextID = parsed.NextID
	s.items = parsed.Incidents
	s.pruneLocked(time.Now())
	return nil
}

// AppendIncident records a new stability incident, prunes entries older than 30 days, and persists atomically.
func (s *Store) AppendIncident(rec IncidentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if rec.Timestamp == 0 {
		rec.Timestamp = now.UnixMilli()
	}
	if rec.TimeISO == "" {
		rec.TimeISO = time.UnixMilli(rec.Timestamp).Format(time.RFC3339)
	}
	s.nextID++
	rec.ID = s.nextID

	// Prepend newest first
	s.items = append([]IncidentRecord{rec}, s.items...)
	s.pruneLocked(now)
	return s.saveLocked()
}

func (s *Store) pruneLocked(now time.Time) {
	cutoffMs := now.Add(-MaxRetentionDays * 24 * time.Hour).UnixMilli()
	filtered := make([]IncidentRecord, 0, len(s.items))
	for _, it := range s.items {
		if it.Timestamp >= cutoffMs {
			filtered = append(filtered, it)
		}
		if len(filtered) >= MaxStoredRecords {
			break
		}
	}
	s.items = filtered
}

func (s *Store) saveLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(historyFile{
		NextID:    s.nextID,
		Incidents: s.items,
	}, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "history-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// Clear removes all persisted incident history entries.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make([]IncidentRecord, 0)
	return s.saveLocked()
}

// Snapshot returns incidents from the last N days (clamped 1..MaxRetentionDays) and daily summaries.
func (s *Store) Snapshot(days int) HistorySnapshot {
	if days <= 0 || days > MaxRetentionDays {
		days = MaxRetentionDays
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoffMs := time.Now().Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()
	filtered := make([]IncidentRecord, 0, len(s.items))
	dailyMap := make(map[string]*DailySummary)

	for _, it := range s.items {
		if it.Timestamp < cutoffMs {
			continue
		}
		filtered = append(filtered, it)

		dateKey := time.UnixMilli(it.Timestamp).Format("2006-01-02")
		d, ok := dailyMap[dateKey]
		if !ok {
			d = &DailySummary{Date: dateKey}
			dailyMap[dateKey] = d
		}
		d.TotalIncidents++
		if it.Kind == "loss" {
			d.LossIncidents++
		} else {
			d.SpikeIncidents++
		}
		if it.PeakMs > d.WorstPeakMs {
			d.WorstPeakMs = it.PeakMs
		}
	}

	daily := make([]DailySummary, 0, len(dailyMap))
	for _, d := range dailyMap {
		daily = append(daily, *d)
	}
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].Date > daily[j].Date
	})

	return HistorySnapshot{
		TotalStored:   len(filtered),
		RetentionDays: MaxRetentionDays,
		Incidents:     filtered,
		Daily:         daily,
	}
}

func redactText(s string) string {
	return ipv4Regex.ReplaceAllStringFunc(s, func(ip string) string {
		return privacy.MaskIP(ip)
	})
}

// BuildISPReportCSV exports incidents as an Excel-compatible UTF-8 CSV table for ISP technical support tickets.
func BuildISPReportCSV(snap HistorySnapshot, netLabel string, redact bool) string {
	var sb strings.Builder
	// UTF-8 BOM for seamless Excel opening in Windows
	sb.WriteString("\uFEFF")
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{"FechaHoraISO", "Red", "Salto", "Tipo", "Severidad", "DuracionMs", "PicoMs", "Detalle"})

	cleanNet := strings.TrimSpace(netLabel)
	if cleanNet == "" {
		cleanNet = "Red Principal"
	}
	if redact {
		cleanNet = redactText(privacy.MaskSSID(cleanNet))
	}

	for _, it := range snap.Incidents {
		msg := it.Message
		netName := it.NetworkName
		if netName == "" {
			netName = cleanNet
		}
		if redact {
			msg = redactText(msg)
			netName = redactText(netName)
		}
		_ = w.Write([]string{
			it.TimeISO,
			netName,
			it.Hop,
			it.Kind,
			it.Severity,
			fmt.Sprintf("%d", it.DurationMs),
			fmt.Sprintf("%.1f", it.PeakMs),
			msg,
		})
	}
	w.Flush()
	return sb.String()
}

// BuildISPReportHTML generates a clean standalone HTML evidence report ready to attach or print to PDF for an ISP claim.
func BuildISPReportHTML(snap HistorySnapshot, netLabel, appVersion string, redact bool) string {
	cleanNet := strings.TrimSpace(netLabel)
	if cleanNet == "" {
		cleanNet = "Red Local"
	}
	if redact {
		cleanNet = redactText(cleanNet)
	}

	var rows strings.Builder
	for _, it := range snap.Incidents {
		msg := it.Message
		if redact {
			msg = redactText(msg)
		}
		badgeColor := "#F59E0B"
		if it.Severity == "rose" {
			badgeColor = "#F43F5E"
		}
		rows.WriteString(fmt.Sprintf(
			"<tr><td>%s</td><td>%s</td><td><span style=\"color:%s;font-weight:600\">%s</span></td><td>%d ms</td><td>%.1f ms</td><td>%s</td></tr>\n",
			html.EscapeString(it.TimeISO),
			html.EscapeString(strings.ToUpper(it.Hop)),
			badgeColor,
			html.EscapeString(strings.ToUpper(it.Kind)),
			it.DurationMs,
			it.PeakMs,
			html.EscapeString(msg),
		))
	}

	var dailyRows strings.Builder
	for _, d := range snap.Daily {
		dailyRows.WriteString(fmt.Sprintf(
			"<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%.1f ms</td></tr>\n",
			html.EscapeString(d.Date),
			d.TotalIncidents,
			d.LossIncidents,
			d.SpikeIncidents,
			d.WorstPeakMs,
		))
	}

	privacyLabel := "Desactivado (datos reales)"
	if redact {
		privacyLabel = "Activado (IPs censuradas)"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8" />
<title>NetPulse — Evidencia de Estabilidad ISP</title>
<style>
  body { font-family: "Segoe UI", system-ui, sans-serif; background: #0B101B; color: #E2E8F0; margin: 32px; }
  h1, h2 { color: #38BDF8; margin-bottom: 8px; }
  .meta { background: #111827; border: 1px solid #1E293B; border-radius: 10px; padding: 14px 18px; margin-bottom: 24px; font-size: 13px; line-height: 1.6; }
  table { width: 100%%; border-collapse: collapse; background: #111827; border: 1px solid #1E293B; margin-bottom: 24px; font-size: 13px; }
  th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #1E293B; }
  th { color: #94A3B8; font-size: 11px; text-transform: uppercase; letter-spacing: .05em; }
  @media print { body { background: #FFFFFF; color: #0F172A; } table, .meta { background: #F8FAFC; border-color: #CBD5E1; } }
</style>
</head>
<body>
  <h1>NetPulse v%s — Informe Técnico de Incidentes para Reclamo ISP</h1>
  <div class="meta">
    <strong>Generado:</strong> %s<br/>
    <strong>Red evaluada:</strong> %s<br/>
    <strong>Total de incidentes registrados:</strong> %d (retención %d días)<br/>
    <strong>Modo Privacidad Discord:</strong> %s
  </div>
  <h2>Resumen Diario</h2>
  <table>
    <thead><tr><th>Fecha</th><th>Incidentes</th><th>Microcortes</th><th>Picos Latencia</th><th>Peor Pico</th></tr></thead>
    <tbody>%s</tbody>
  </table>
  <h2>Registro Cronológico Detallado</h2>
  <table>
    <thead><tr><th>Fecha / Hora (ISO)</th><th>Salto</th><th>Tipo</th><th>Duración</th><th>Pico RTT</th><th>Detalle</th></tr></thead>
    <tbody>%s</tbody>
  </table>
</body>
</html>`,
		html.EscapeString(appVersion),
		html.EscapeString(time.Now().Format(time.RFC3339)),
		html.EscapeString(cleanNet),
		snap.TotalStored,
		snap.RetentionDays,
		privacyLabel,
		dailyRows.String(),
		rows.String(),
	)
}
