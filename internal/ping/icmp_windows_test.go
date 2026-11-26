//go:build windows

package ping

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestParseIPv4ToUint32(t *testing.T) {
	cases := []struct {
		input string
		want  uint32
		ok    bool
	}{
		{"1.1.1.1", 0x01010101, true},
		{"192.168.1.1", 0x0101A8C0, true},
		{"127.0.0.1", 0x0100007F, true},
		{"::1", 0, false},
		{"", 0, false},
	}

	for _, tc := range cases {
		ip := net.ParseIP(tc.input)
		got, ok := ParseIPv4ToUint32(ip)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParseIPv4ToUint32(%q) = (0x%08x, %v), want (0x%08x, %v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestICMPEchoReplyLayout(t *testing.T) {
	// On windows/amd64, ICMP_ECHO_REPLY is 40 bytes (including 64-bit Data pointer and alignment).
	if size := unsafe.Sizeof(icmpEchoReply{}); size != 40 {
		t.Fatalf("unexpected icmpEchoReply size on amd64: got %d, want 40", size)
	}
}

func TestWinPingerLoopback(t *testing.T) {
	p := NewWinPinger()
	res := p.Ping(context.Background(), "127.0.0.1", 1000*time.Millisecond)
	if !res.Success {
		lower := strings.ToLower(res.Error)
		if strings.Contains(lower, "access") || strings.Contains(lower, "denied") || strings.Contains(lower, "perm") {
			t.Skipf("ICMP blocked by sandbox policy: %s", res.Error)
		}
		t.Fatalf("loopback ping failed: status=%d err=%q", res.Status, res.Error)
	}
	if res.RTTMs <= 0 {
		t.Fatalf("expected positive RTTMs, got %v", res.RTTMs)
	}
}
