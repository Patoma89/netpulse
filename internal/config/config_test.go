package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreDefaultsAndPersistence(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "NetPulse", "config.json")

	store, err := NewStoreAt(cfgPath)
	if err != nil {
		t.Fatalf("NewStoreAt failed: %v", err)
	}

	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config file to be created at %s: %v", cfgPath, err)
	}

	cfg := store.Get()
	if !cfg.PrivacyMode {
		t.Fatalf("expected PrivacyMode default true, got false")
	}
	if cfg.Language != "es" {
		t.Fatalf("expected Language default 'es', got %q", cfg.Language)
	}
	if cfg.PingTarget != "1.1.1.1" {
		t.Fatalf("expected PingTarget default '1.1.1.1', got %q", cfg.PingTarget)
	}
	if cfg.KnownDevices == nil {
		t.Fatalf("expected KnownDevices map to be initialized")
	}

	updated, err := store.SetPrivacyMode(false)
	if err != nil {
		t.Fatalf("SetPrivacyMode failed: %v", err)
	}
	if updated.PrivacyMode {
		t.Fatalf("expected PrivacyMode false after toggle")
	}

	reopened, err := NewStoreAt(cfgPath)
	if err != nil {
		t.Fatalf("reopening store failed: %v", err)
	}
	if reopened.Get().PrivacyMode {
		t.Fatalf("expected persisted PrivacyMode false on reload")
	}

	if _, err := reopened.SetPingTarget("   "); err == nil {
		t.Fatalf("expected error on empty ping target")
	}

	updatedTarget, err := reopened.SetPingTarget(" 8.8.8.8 ")
	if err != nil {
		t.Fatalf("SetPingTarget failed: %v", err)
	}
	if updatedTarget.PingTarget != "8.8.8.8" {
		t.Fatalf("expected PingTarget '8.8.8.8', got %q", updatedTarget.PingTarget)
	}

	reloadedTarget, err := NewStoreAt(cfgPath)
	if err != nil {
		t.Fatalf("reloading store after SetPingTarget failed: %v", err)
	}
	if reloadedTarget.Get().PingTarget != "8.8.8.8" {
		t.Fatalf("expected persisted PingTarget '8.8.8.8', got %q", reloadedTarget.Get().PingTarget)
	}
}

func TestKnownDevicesPersistenceAndTrustBatch(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "NetPulse", "config.json")
	store, err := NewStoreAt(cfgPath)
	if err != nil {
		t.Fatalf("NewStoreAt failed: %v", err)
	}

	if _, err := store.UpsertKnownDevice("  ", KnownDevice{Alias: "Empty"}); err == nil {
		t.Fatalf("expected error for empty MAC")
	}

	_, err = store.UpsertKnownDevice("a4-b1-c2-d3-e4-f5", KnownDevice{
		Alias:     "PS5 Living",
		Category:  "console",
		Trusted:   true,
		FirstSeen: "2026-01-01T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("UpsertKnownDevice failed: %v", err)
	}

	// Updating again with a different FirstSeen should preserve the original FirstSeen.
	updated, err := store.UpsertKnownDevice("A4:B1:C2:D3:E4:F5", KnownDevice{
		Alias:     "PS5 Pro",
		Category:  "console",
		Trusted:   true,
		FirstSeen: "2026-12-12T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("second UpsertKnownDevice failed: %v", err)
	}
	dev := updated.KnownDevices["A4:B1:C2:D3:E4:F5"]
	if dev.Alias != "PS5 Pro" || dev.FirstSeen != "2026-01-01T10:00:00Z" {
		t.Fatalf("unexpected device after upsert: %+v", dev)
	}

	// Batch trust should preserve existing alias while adding new trusted entries.
	batched, err := store.TrustDevices(map[string]KnownDevice{
		"a4:b1:c2:d3:e4:f5": {Alias: "IgnoredOverride", Category: "unknown"},
		"11:22:33:44:55:66": {Alias: "Router", Category: "router"},
	})
	if err != nil {
		t.Fatalf("TrustDevices failed: %v", err)
	}
	if batched.KnownDevices["A4:B1:C2:D3:E4:F5"].Alias != "PS5 Pro" {
		t.Fatalf("expected existing alias to be preserved, got %+v", batched.KnownDevices["A4:B1:C2:D3:E4:F5"])
	}
	if !batched.KnownDevices["11:22:33:44:55:66"].Trusted || batched.KnownDevices["11:22:33:44:55:66"].FirstSeen == "" {
		t.Fatalf("expected new device to be trusted with FirstSeen, got %+v", batched.KnownDevices["11:22:33:44:55:66"])
	}

	reloaded, err := NewStoreAt(cfgPath)
	if err != nil {
		t.Fatalf("reloading store failed: %v", err)
	}
	if len(reloaded.Get().KnownDevices) != 2 {
		t.Fatalf("expected 2 persisted known devices, got %d", len(reloaded.Get().KnownDevices))
	}

	trayCfg, err := reloaded.SetTrayPreferences(true, true)
	if err != nil {
		t.Fatalf("SetTrayPreferences failed: %v", err)
	}
	if !trayCfg.MinimizeToTray || !trayCfg.BackgroundRadarAlert {
		t.Fatalf("expected tray prefs true, got %+v", trayCfg)
	}

	if err := reloaded.SaveSSHHostFingerprint("192.168.1.1:22", "SHA256:abc123"); err != nil {
		t.Fatalf("SaveSSHHostFingerprint failed: %v", err)
	}
	if fp, ok := reloaded.GetSSHHostFingerprint("192.168.1.1:22"); !ok || fp != "SHA256:abc123" {
		t.Fatalf("unexpected fingerprint: %q ok=%v", fp, ok)
	}
	if err := reloaded.ClearSSHHostFingerprint("192.168.1.1:22"); err != nil {
		t.Fatalf("ClearSSHHostFingerprint failed: %v", err)
	}
	if _, ok := reloaded.GetSSHHostFingerprint("192.168.1.1:22"); ok {
		t.Fatalf("expected fingerprint to be cleared")
	}

	uiCfg, err := reloaded.SetUIPreferences("en", "light")
	if err != nil {
		t.Fatalf("SetUIPreferences failed: %v", err)
	}
	if uiCfg.Language != "en" || uiCfg.Theme != "light" {
		t.Fatalf("unexpected UI prefs: %+v", uiCfg)
	}

	autoCfg, err := reloaded.SetAutostartPreference(true)
	if err != nil {
		t.Fatalf("SetAutostartPreference failed: %v", err)
	}
	if !autoCfg.StartWithWindows {
		t.Fatalf("expected StartWithWindows true")
	}
}

func TestNetworkProfilesGatewaySpoofAndPresence(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "NetPulse", "config.json")
	store, err := NewStoreAt(cfgPath)
	if err != nil {
		t.Fatalf("NewStoreAt failed: %v", err)
	}

	prof, changed, prevMAC, err := store.RecordNetworkPresence(
		"gw:192.168.1.1/192.168.1.0/24",
		"Red Casa",
		"192.168.1.1",
		"00:11:22:33:44:55",
		"192.168.1.0/24",
		[]string{"00:11:22:33:44:55", "aa:bb:cc:dd:ee:01"},
	)
	if err != nil {
		t.Fatalf("RecordNetworkPresence failed: %v", err)
	}
	if changed || prevMAC != "" || prof.GatewayMAC != "00:11:22:33:44:55" {
		t.Fatalf("unexpected initial profile state: prof=%+v changed=%v prev=%s", prof, changed, prevMAC)
	}

	cfg := store.Get()
	if !cfg.KnownDevices["AA:BB:CC:DD:EE:01"].Online || cfg.KnownDevices["AA:BB:CC:DD:EE:01"].SeenCount != 1 {
		t.Fatalf("expected device presence to be recorded, got %+v", cfg.KnownDevices["AA:BB:CC:DD:EE:01"])
	}

	// Simulate gateway MAC change and device going offline
	prof2, changed, prevMAC, err := store.RecordNetworkPresence(
		"gw:192.168.1.1/192.168.1.0/24",
		"Red Casa",
		"192.168.1.1",
		"66:77:88:99:AA:BB",
		"192.168.1.0/24",
		[]string{"66:77:88:99:AA:BB"},
	)
	if err != nil {
		t.Fatalf("second RecordNetworkPresence failed: %v", err)
	}
	if !changed || prevMAC != "00:11:22:33:44:55" {
		t.Fatalf("expected gateway MAC change alert, got changed=%v prev=%s", changed, prevMAC)
	}
	if prof2.GatewayMAC != "00:11:22:33:44:55" {
		t.Fatalf("pinned GatewayMAC should remain unchanged until confirmed, got %s", prof2.GatewayMAC)
	}
	if store.Get().KnownDevices["AA:BB:CC:DD:EE:01"].Online {
		t.Fatalf("expected absent device to be marked offline")
	}

	if err := store.ConfirmGatewayMAC(prof.ID, "66:77:88:99:AA:BB"); err != nil {
		t.Fatalf("ConfirmGatewayMAC failed: %v", err)
	}
	renamed, err := store.RenameNetworkProfile(prof.ID, "Casa Fibra")
	if err != nil {
		t.Fatalf("RenameNetworkProfile failed: %v", err)
	}
	if renamed.GatewayMAC != "66:77:88:99:AA:BB" || renamed.Name != "Casa Fibra" {
		t.Fatalf("unexpected confirmed/renamed profile: %+v", renamed)
	}

	if err := store.SetDeviceTrustedForProfile(prof.ID, "aa:bb:cc:dd:ee:01", true); err != nil {
		t.Fatalf("SetDeviceTrustedForProfile failed: %v", err)
	}
	if err := store.TrustAllForProfile(prof.ID, []string{"11:22:33:44:55:66"}); err != nil {
		t.Fatalf("TrustAllForProfile failed: %v", err)
	}
	finalProf := store.Get().NetworkProfiles[prof.ID]
	if !finalProf.TrustedMACs["AA:BB:CC:DD:EE:01"] || !finalProf.TrustedMACs["11:22:33:44:55:66"] {
		t.Fatalf("expected trusted MACs in profile, got %+v", finalProf.TrustedMACs)
	}
}
