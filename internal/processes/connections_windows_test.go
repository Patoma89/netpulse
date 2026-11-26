//go:build windows

package processes

import (
	"testing"
	"unsafe"
)

func TestWin32ProcessStructLayoutsAndHelpers(t *testing.T) {
	if got := unsafe.Sizeof(mibTCPRowOwnerPID{}); got != 24 {
		t.Fatalf("sizeof(mibTCPRowOwnerPID) = %d, want 24", got)
	}
	if got := unsafe.Sizeof(mibUDPRowOwnerPID{}); got != 12 {
		t.Fatalf("sizeof(mibUDPRowOwnerPID) = %d, want 12", got)
	}
	if got := unsafe.Sizeof(ioCounters{}); got != 48 {
		t.Fatalf("sizeof(ioCounters) = %d, want 48", got)
	}

	if got := ntohs(0xBB01); got != 443 {
		t.Fatalf("ntohs(0xBB01) = %d, want 443", got)
	}
	if got := uint32ToIPv4(0x01010101); got != "1.1.1.1" {
		t.Fatalf("uint32ToIPv4 = %q, want 1.1.1.1", got)
	}
}

func TestMonitorSnapshotRunsWithoutAdmin(t *testing.T) {
	mon := NewMonitor()
	snap, err := mon.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if snap.CapturedAt == "" || snap.TotalProcesses == 0 {
		t.Fatalf("expected non-empty live socket snapshot on Windows, got %+v", snap)
	}
}
