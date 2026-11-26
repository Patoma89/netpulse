//go:build windows

package radar

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"netpulse/internal/ping"
)

var (
	modIphlpapi       = windows.NewLazySystemDLL("iphlpapi.dll")
	procSendARP       = modIphlpapi.NewProc("SendARP")
	procGetIpNetTable = modIphlpapi.NewProc("GetIpNetTable")
)

const (
	mibIpNetTypeOther   uint32 = 1
	mibIpNetTypeInvalid uint32 = 2
	mibIpNetTypeDynamic uint32 = 3
	mibIpNetTypeStatic  uint32 = 4
)

// mibIpNetRow matches Win32 MIB_IPNETROW (24 bytes on both 32-bit and 64-bit Windows).
type mibIpNetRow struct {
	DwIndex       uint32
	DwPhysAddrLen uint32
	BPhysAddr     [8]byte
	DwAddr        uint32
	DwType        uint32
}

// ARPScanner abstracts Win32 ARP probing and ARP table reading for testability.
type ARPScanner interface {
	ProbeARP(ctx context.Context, destIP, srcIP string) (mac string, rttMs float64, ok bool)
	ReadARPTable(ifIndex uint32) ([]ARPEntry, error)
}

// WinARPScanner implements ARPScanner using Win32 iphlpapi.dll without Admin privileges.
type WinARPScanner struct{}

// NewWinARPScanner constructs a Win32 ARPScanner.
func NewWinARPScanner() *WinARPScanner {
	return &WinARPScanner{}
}

// ProbeARP sends a single Win32 SendARP request to destIP and returns its normalized MAC and RTT.
func (w *WinARPScanner) ProbeARP(ctx context.Context, destIP, srcIP string) (string, float64, bool) {
	if ctx.Err() != nil {
		return "", 0, false
	}

	dstParsed := net.ParseIP(destIP)
	dstAddr, ok := ping.ParseIPv4ToUint32(dstParsed)
	if !ok {
		return "", 0, false
	}

	var srcAddr uint32
	if srcIP != "" {
		if s, ok := ping.ParseIPv4ToUint32(net.ParseIP(srcIP)); ok {
			srcAddr = s
		}
	}

	var macBuf [8]byte
	macLen := uint32(6)

	start := time.Now()
	ret, _, _ := procSendARP.Call(
		uintptr(dstAddr),
		uintptr(srcAddr),
		uintptr(unsafe.Pointer(&macBuf[0])),
		uintptr(unsafe.Pointer(&macLen)),
	)
	elapsed := time.Since(start)

	if ret != 0 || macLen != 6 {
		return "", 0, false
	}
	if !isValidUnicastMAC(macBuf[:6]) {
		return "", 0, false
	}

	mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		macBuf[0], macBuf[1], macBuf[2], macBuf[3], macBuf[4], macBuf[5])
	rttMs := math.Round(float64(elapsed.Microseconds())/100.0) / 10.0
	if rttMs < 0.1 {
		rttMs = 0.1
	}
	return mac, rttMs, true
}

// ReadARPTable reads cached ARP entries from Win32 GetIpNetTable for the given interface index.
func (w *WinARPScanner) ReadARPTable(ifIndex uint32) ([]ARPEntry, error) {
	bufSize := uint32(8192)
	var buf []byte

	for attempt := 0; attempt < 3; attempt++ {
		buf = make([]byte, bufSize)
		ret, _, _ := procGetIpNetTable.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&bufSize)),
			0,
		)
		if ret == 0 {
			return parseIpNetTable(buf, ifIndex), nil
		}
		if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
			return nil, fmt.Errorf("GetIpNetTable failed with code %d", ret)
		}
	}

	return nil, fmt.Errorf("GetIpNetTable buffer overflow after retries")
}

func parseIpNetTable(buf []byte, ifIndex uint32) []ARPEntry {
	if len(buf) < 4 {
		return nil
	}
	numEntries := int(binary.LittleEndian.Uint32(buf[:4]))
	rowSize := int(unsafe.Sizeof(mibIpNetRow{}))
	maxPossible := (len(buf) - 4) / rowSize
	if numEntries > maxPossible {
		numEntries = maxPossible
	}

	entries := make([]ARPEntry, 0, numEntries)
	for i := 0; i < numEntries; i++ {
		offset := 4 + i*rowSize
		row := (*mibIpNetRow)(unsafe.Pointer(&buf[offset]))

		if ifIndex > 0 && row.DwIndex != ifIndex {
			continue
		}
		if row.DwType != mibIpNetTypeDynamic && row.DwType != mibIpNetTypeStatic {
			continue
		}
		if row.DwPhysAddrLen != 6 || !isValidUnicastMAC(row.BPhysAddr[:6]) {
			continue
		}

		ip := net.IPv4(
			byte(row.DwAddr),
			byte(row.DwAddr>>8),
			byte(row.DwAddr>>16),
			byte(row.DwAddr>>24),
		).String()
		mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			row.BPhysAddr[0], row.BPhysAddr[1], row.BPhysAddr[2],
			row.BPhysAddr[3], row.BPhysAddr[4], row.BPhysAddr[5])

		entries = append(entries, ARPEntry{
			IP:      ip,
			MAC:     mac,
			IfIndex: row.DwIndex,
			Type:    row.DwType,
		})
	}
	return entries
}

func isValidUnicastMAC(b []byte) bool {
	if len(b) < 6 {
		return false
	}
	allZero := true
	allFF := true
	for i := 0; i < 6; i++ {
		if b[i] != 0x00 {
			allZero = false
		}
		if b[i] != 0xFF {
			allFF = false
		}
	}
	if allZero || allFF {
		return false
	}
	// Bit 0 of first byte is multicast group bit (excludes 01:00:5E:* IPv4 multicast MACs).
	return b[0]&0x01 == 0
}
