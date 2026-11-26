package history

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryStoreRetentionAndExports(t *testing.T) {
	histPath := filepath.Join(t.TempDir(), "NetPulse", "history.json")
	store, err := NewStoreAt(histPath)
	if err != nil {
		t.Fatalf("NewStoreAt failed: %v", err)
	}

	now := time.Now()
	if err := store.AppendIncident(IncidentRecord{
		Timestamp:   now.UnixMilli(),
		NetworkName: "Casa 192.168.1.1",
		Hop:         "internet",
		Kind:        "loss",
		Severity:    "rose",
		DurationMs:  2000,
		Message:     "Microcorte ISP hacia 192.168.1.1",
	}); err != nil {
		t.Fatalf("AppendIncident failed: %v", err)
	}

	// Simulate an entry older than 30 days -> should be pruned immediately
	oldTs := now.Add(-35 * 24 * time.Hour).UnixMilli()
	if err := store.AppendIncident(IncidentRecord{
		Timestamp: oldTs,
		Hop:       "router",
		Kind:      "spike",
		Severity:  "amber",
		PeakMs:    95.5,
		Message:   "Viejo",
	}); err != nil {
		t.Fatalf("AppendIncident old failed: %v", err)
	}

	reloaded, err := NewStoreAt(histPath)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	snap := reloaded.Snapshot(30)
	if snap.TotalStored != 1 || len(snap.Daily) != 1 {
		t.Fatalf("expected 1 retained incident and 1 daily bucket, got total=%d daily=%d", snap.TotalStored, len(snap.Daily))
	}

	csvText := BuildISPReportCSV(snap, "Casa", true)
	if !strings.Contains(csvText, "Microcorte ISP") || strings.Contains(csvText, "192.168.1.1") {
		t.Fatalf("expected redacted CSV output, got:\n%s", csvText)
	}

	htmlText := BuildISPReportHTML(snap, "Casa", "1.2.0", true)
	if !strings.Contains(htmlText, "<html") || !strings.Contains(htmlText, "NetPulse v1.2.0") || strings.Contains(htmlText, "192.168.1.1") {
		t.Fatalf("unexpected redacted HTML report")
	}

	if err := reloaded.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if after := reloaded.Snapshot(30); after.TotalStored != 0 {
		t.Fatalf("expected 0 after Clear, got %d", after.TotalStored)
	}
}
