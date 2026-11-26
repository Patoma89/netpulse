//go:build windows

package ping

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modIphlpapi         = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreateFile  = modIphlpapi.NewProc("IcmpCreateFile")
	procIcmpSendEcho    = modIphlpapi.NewProc("IcmpSendEcho")
	procIcmpCloseHandle = modIphlpapi.NewProc("IcmpCloseHandle")
)

const (
	ipSuccess         uint32 = 0
	ipDestNetUnreach  uint32 = 11002
	ipDestHostUnreach uint32 = 11003
	ipReqTimedOut     uint32 = 11010
	ipTTLExpired      uint32 = 11013
)

type ipOptionInformation struct {
	TTL         uint8
	Tos         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

var icmpPayload = []byte("NetPulseICMPPingPayload012345678")

// WinPinger executes non-privileged ICMP echo requests via Win32 iphlpapi.dll.
type WinPinger struct{}

// NewWinPinger constructs a Win32 IcmpSendEcho-based Pinger.
func NewWinPinger() *WinPinger {
	return &WinPinger{}
}

// Ping sends a single ICMP echo request to an IPv4 address or hostname.
func (p *WinPinger) Ping(ctx context.Context, target string, timeout time.Duration) Result {
	nowMs := time.Now().UnixMilli()
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return Result{Target: target, Success: false, Error: "empty target", Timestamp: nowMs}
	}
	if err := ctx.Err(); err != nil {
		return Result{Target: trimmed, Success: false, Error: err.Error(), Timestamp: nowMs}
	}

	destAddr, err := resolveIPv4Addr(ctx, trimmed)
	if err != nil {
		return Result{Target: trimmed, Success: false, Error: err.Error(), Timestamp: nowMs}
	}

	handle, _, callErr := procIcmpCreateFile.Call()
	if handle == uintptr(windows.InvalidHandle) || handle == 0 {
		return Result{
			Target:    trimmed,
			Success:   false,
			Error:     fmt.Sprintf("IcmpCreateFile: %v", callErr),
			Timestamp: nowMs,
		}
	}
	defer procIcmpCloseHandle.Call(handle)

	timeoutMs := uint32(timeout.Milliseconds())
	if timeoutMs == 0 {
		timeoutMs = 1000
	}

	replySize := int(unsafe.Sizeof(icmpEchoReply{})) + len(icmpPayload) + 64
	replyBuf := make([]byte, replySize)

	start := time.Now()
	ret, _, sendErr := procIcmpSendEcho.Call(
		handle,
		uintptr(destAddr),
		uintptr(unsafe.Pointer(&icmpPayload[0])),
		uintptr(len(icmpPayload)),
		0,
		uintptr(unsafe.Pointer(&replyBuf[0])),
		uintptr(replySize),
		uintptr(timeoutMs),
	)
	elapsed := time.Since(start)

	reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
	if ret > 0 && reply.Status == ipSuccess {
		rttMs := math.Round(float64(elapsed.Microseconds())/100.0) / 10.0
		if rttMs < 0.1 {
			rttMs = 0.1
		}
		if reply.RoundTripTime > 0 && float64(reply.RoundTripTime) > rttMs+2 {
			rttMs = float64(reply.RoundTripTime)
		}
		return Result{
			Target:    trimmed,
			RTTMs:     rttMs,
			TTL:       reply.Options.TTL,
			Success:   true,
			Status:    reply.Status,
			Timestamp: time.Now().UnixMilli(),
		}
	}

	errMsg := describeICMPStatus(reply.Status, sendErr)
	return Result{
		Target:    trimmed,
		Success:   false,
		Status:    reply.Status,
		Error:     errMsg,
		Timestamp: time.Now().UnixMilli(),
	}
}

func resolveIPv4Addr(ctx context.Context, target string) (uint32, error) {
	if ip := net.ParseIP(target); ip != nil {
		addr, ok := ParseIPv4ToUint32(ip)
		if !ok {
			return 0, fmt.Errorf("target %q is not an IPv4 address", target)
		}
		return addr, nil
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", target)
	if err != nil {
		return 0, fmt.Errorf("lookup %q: %w", target, err)
	}
	for _, ip := range ips {
		if addr, ok := ParseIPv4ToUint32(ip); ok {
			return addr, nil
		}
	}
	return 0, fmt.Errorf("no IPv4 address found for %q", target)
}

func describeICMPStatus(status uint32, callErr error) string {
	switch status {
	case ipReqTimedOut:
		return "timeout"
	case ipDestNetUnreach, ipDestHostUnreach:
		return "host unreachable"
	case ipTTLExpired:
		return "ttl expired"
	case 0:
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return callErr.Error()
		}
		return "no reply"
	default:
		return fmt.Sprintf("icmp status %d", status)
	}
}
