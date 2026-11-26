//go:build windows

package tray

import (
	"testing"
	"unsafe"
)

func TestTrayWin64StructLayouts(t *testing.T) {
	if sz := unsafe.Sizeof(wndClassExW{}); sz != 80 {
		t.Fatalf("unexpected wndClassExW size on amd64: got %d, want 80", sz)
	}
	if sz := unsafe.Sizeof(notifyIconDataW{}); sz != 976 {
		t.Fatalf("unexpected notifyIconDataW size on amd64: got %d, want 976", sz)
	}
	if sz := unsafe.Sizeof(winMsg{}); sz != 48 {
		t.Fatalf("unexpected winMsg size on amd64: got %d, want 48", sz)
	}
}
