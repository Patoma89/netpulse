//go:build windows

package vault

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestVaultHealthCheck(t *testing.T) {
	v := New()
	if err := v.HealthCheck(); err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

func TestTargetName(t *testing.T) {
	if _, err := targetName("   "); err == nil {
		t.Fatalf("expected error for empty vault key")
	}

	got, err := targetName("router-primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "NetPulse:router-primary" {
		t.Fatalf("got %q, want %q", got, "NetPulse:router-primary")
	}

	gotPrefixed, err := targetName("NetPulse:already")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPrefixed != "NetPulse:already" {
		t.Fatalf("got %q, want %q", gotPrefixed, "NetPulse:already")
	}
}

func TestUTF16RoundTrip(t *testing.T) {
	cases := []string{
		"",
		"simple123",
		"NetPulse!ClaveSegura#2026",
		"ñandú-wifi-€-🔑",
	}
	for _, want := range cases {
		encoded := encodeUTF16LE(want)
		got := decodeUTF16LE(encoded)
		if got != want {
			t.Errorf("UTF16 roundtrip mismatch: got %q, want %q", got, want)
		}
	}
}

func TestVaultRoundTrip(t *testing.T) {
	v := New()
	if err := v.HealthCheck(); err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	key := fmt.Sprintf("test-roundtrip-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = v.DeleteSecret(key)
	})

	if v.HasSecret(key) {
		t.Fatalf("expected key %q not to exist before SetSecret", key)
	}

	wantUser := "admin"
	wantPass := "NetPulse!ClaveSegura#2026"
	if err := v.SetSecret(key, wantUser, wantPass); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Skip("skipping live Credential Manager write in restricted sandbox")
		}
		t.Fatalf("SetSecret failed: %v", err)
	}

	if !v.HasSecret(key) {
		t.Fatalf("expected HasSecret(%q) to be true", key)
	}

	gotUser, gotPass, err := v.GetSecret(key)
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if gotUser != wantUser || gotPass != wantPass {
		t.Fatalf("got (%q, %q), want (%q, %q)", gotUser, gotPass, wantUser, wantPass)
	}

	if err := v.DeleteSecret(key); err != nil {
		t.Fatalf("DeleteSecret failed: %v", err)
	}

	if v.HasSecret(key) {
		t.Fatalf("expected HasSecret(%q) to be false after DeleteSecret", key)
	}

	if _, _, err := v.GetSecret(key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after DeleteSecret, got %v", err)
	}
}
