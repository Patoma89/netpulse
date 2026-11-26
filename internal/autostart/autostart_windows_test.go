//go:build windows

package autostart

import "testing"

func TestBuildRunCommand(t *testing.T) {
	gotMin := BuildRunCommand(`C:\Program Files\NetPulse\NetPulse.exe`, true)
	wantMin := `"C:\Program Files\NetPulse\NetPulse.exe" --minimized`
	if gotMin != wantMin {
		t.Fatalf("BuildRunCommand(true) = %q, want %q", gotMin, wantMin)
	}

	gotNormal := BuildRunCommand(`"D:\Tools\NetPulse.exe"`, false)
	wantNormal := `"D:\Tools\NetPulse.exe"`
	if gotNormal != wantNormal {
		t.Fatalf("BuildRunCommand(false) = %q, want %q", gotNormal, wantNormal)
	}

	if empty := BuildRunCommand("   ", true); empty != "" {
		t.Fatalf("expected empty output for blank path, got %q", empty)
	}
}
