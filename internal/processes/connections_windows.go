//go:build windows

package processes

import (
	"fmt"
	"math"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	afINET                         uint32 = 2
	tcpTableOwnerPIDAll            uint32 = 5
	udpTableOwnerPID               uint32 = 1
	errInsufficientBuffer          uintptr = 122
	processQueryLimitedInformation uint32 = 0x1000
	tcpStateEstablished            uint32 = 5
)

var (
	modIphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable    = modIphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable    = modIphlpapi.NewProc("GetExtendedUdpTable")
	modKernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procQueryFullProcessImageW = modKernel32.NewProc("QueryFullProcessImageNameW")
	procGetProcessIoCounters   = modKernel32.NewProc("GetProcessIoCounters")
)

// Exact Win32 MIB_TCPROW_OWNER_PID layout (24 bytes on Windows x64).
type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPid  uint32
}

// Exact Win32 MIB_UDPROW_OWNER_PID layout (12 bytes on Windows x64).
type mibUDPRowOwnerPID struct {
	LocalAddr uint32
	LocalPort uint32
	OwningPid uint32
}

// Exact Win32 IO_COUNTERS layout (48 bytes on Windows x64).
type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

// ConnectionEndpoint represents one TCP or UDP socket owned by a local process.
type ConnectionEndpoint struct {
	Protocol   string `json:"protocol"` // "TCP" | "UDP"
	State      string `json:"state"`
	LocalPort  uint16 `json:"localPort"`
	RemoteIP   string `json:"remoteIp"`
	RemotePort uint16 `json:"remotePort"`
}

// ProcessNetUsage summarizes active sockets and disk/network I/O rate for a Windows process.
type ProcessNetUsage struct {
	PID              uint32               `json:"pid"`
	Name             string               `json:"name"`
	ExePath          string               `json:"exePath"`
	TCPCount         int                  `json:"tcpCount"`
	EstablishedCount int                  `json:"establishedCount"`
	UDPCount         int                  `json:"udpCount"`
	TotalConnections int                  `json:"totalConnections"`
	IOKBPerSec       float64              `json:"ioKbPerSec"`
	RemoteSample     []ConnectionEndpoint `json:"remoteSample"`
}

// ProcessSnapshot contains active TCP/UDP usage grouped by local Windows process without kernel drivers.
type ProcessSnapshot struct {
	CapturedAt       string            `json:"capturedAt"`
	TotalProcesses   int               `json:"totalProcesses"`
	TotalConnections int               `json:"totalConnections"`
	EstablishedTotal int               `json:"establishedTotal"`
	Processes        []ProcessNetUsage `json:"processes"`
}

type ioSample struct {
	bytes uint64
	at    time.Time
}

// Monitor samples Win32 IP Helper TCP/UDP owner-PID tables and per-process I/O counters without Admin rights.
type Monitor struct {
	mu     sync.Mutex
	prevIO map[uint32]ioSample
}

// NewMonitor initializes a driverless process connection monitor.
func NewMonitor() *Monitor {
	return &Monitor{
		prevIO: make(map[uint32]ioSample),
	}
}

// Snapshot reads live IPv4 TCP & UDP sockets by PID, resolves process names, and calculates per-PID I/O rates.
func (m *Monitor) Snapshot() (ProcessSnapshot, error) {
	now := time.Now()
	tcpRows, err := readExtendedTCPTable()
	if err != nil {
		return ProcessSnapshot{}, err
	}
	udpRows, udpErr := readExtendedUDPTable()
	if udpErr != nil {
		udpRows = nil
	}

	type procAcc struct {
		usage   ProcessNetUsage
		remotes []ConnectionEndpoint
	}
	byPID := make(map[uint32]*procAcc)
	getAcc := func(pid uint32) *procAcc {
		acc, ok := byPID[pid]
		if !ok {
			acc = &procAcc{
				usage: ProcessNetUsage{
					PID:          pid,
					RemoteSample: []ConnectionEndpoint{},
				},
			}
			byPID[pid] = acc
		}
		return acc
	}

	totalConns := 0
	establishedTotal := 0

	for _, r := range tcpRows {
		acc := getAcc(r.OwningPid)
		acc.usage.TCPCount++
		acc.usage.TotalConnections++
		totalConns++
		if r.State == tcpStateEstablished {
			acc.usage.EstablishedCount++
			establishedTotal++
		}
		remoteIP := uint32ToIPv4(r.RemoteAddr)
		remotePort := ntohs(r.RemotePort)
		if remoteIP != "0.0.0.0" || r.State == tcpStateEstablished {
			acc.remotes = append(acc.remotes, ConnectionEndpoint{
				Protocol:   "TCP",
				State:      tcpStateName(r.State),
				LocalPort:  ntohs(r.LocalPort),
				RemoteIP:   remoteIP,
				RemotePort: remotePort,
			})
		}
	}

	for _, r := range udpRows {
		acc := getAcc(r.OwningPid)
		acc.usage.UDPCount++
		acc.usage.TotalConnections++
		totalConns++
		if len(acc.remotes) < 8 {
			acc.remotes = append(acc.remotes, ConnectionEndpoint{
				Protocol:  "UDP",
				State:     "OPEN",
				LocalPort: ntohs(r.LocalPort),
				RemoteIP:  uint32ToIPv4(r.LocalAddr),
			})
		}
	}

	m.mu.Lock()
	nextIO := make(map[uint32]ioSample, len(byPID))
	outProcs := make([]ProcessNetUsage, 0, len(byPID))

	for pid, acc := range byPID {
		name, exePath, totalIO, hasIO := queryProcessDetails(pid)
		acc.usage.Name = name
		acc.usage.ExePath = exePath

		if hasIO {
			nextIO[pid] = ioSample{bytes: totalIO, at: now}
			if prev, ok := m.prevIO[pid]; ok && totalIO >= prev.bytes {
				dt := now.Sub(prev.at).Seconds()
				if dt >= 0.05 {
					kbPerSec := float64(totalIO-prev.bytes) / 1024.0 / dt
					acc.usage.IOKBPerSec = math.Round(kbPerSec*10.0) / 10.0
				}
			}
		}

		// Prioritize ESTABLISHED TCP sockets in RemoteSample (up to 8 entries)
		sort.SliceStable(acc.remotes, func(i, j int) bool {
			ei, ej := acc.remotes[i], acc.remotes[j]
			if (ei.State == "ESTABLISHED") != (ej.State == "ESTABLISHED") {
				return ei.State == "ESTABLISHED"
			}
			return ei.Protocol < ej.Protocol
		})
		if len(acc.remotes) > 8 {
			acc.remotes = acc.remotes[:8]
		}
		acc.usage.RemoteSample = acc.remotes
		outProcs = append(outProcs, acc.usage)
	}
	m.prevIO = nextIO
	m.mu.Unlock()

	sort.Slice(outProcs, func(i, j int) bool {
		a, b := outProcs[i], outProcs[j]
		if a.EstablishedCount != b.EstablishedCount {
			return a.EstablishedCount > b.EstablishedCount
		}
		if a.IOKBPerSec != b.IOKBPerSec {
			return a.IOKBPerSec > b.IOKBPerSec
		}
		if a.TotalConnections != b.TotalConnections {
			return a.TotalConnections > b.TotalConnections
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	return ProcessSnapshot{
		CapturedAt:       now.Format("15:04:05"),
		TotalProcesses:   len(outProcs),
		TotalConnections: totalConns,
		EstablishedTotal: establishedTotal,
		Processes:        outProcs,
	}, nil
}

func readExtendedTCPTable() ([]mibTCPRowOwnerPID, error) {
	var size uint32
	for attempt := 0; attempt < 4; attempt++ {
		var buf []byte
		var ptr uintptr
		if size > 0 {
			buf = make([]byte, size)
			ptr = uintptr(unsafe.Pointer(&buf[0]))
		}
		ret, _, _ := procGetExtendedTcpTable.Call(
			ptr,
			uintptr(unsafe.Pointer(&size)),
			0,
			uintptr(afINET),
			uintptr(tcpTableOwnerPIDAll),
			0,
		)
		if ret == 0 && len(buf) >= 4 {
			count := int(*(*uint32)(unsafe.Pointer(&buf[0])))
			rowSize := int(unsafe.Sizeof(mibTCPRowOwnerPID{}))
			maxRows := (len(buf) - 4) / rowSize
			if count > maxRows {
				count = maxRows
			}
			rows := make([]mibTCPRowOwnerPID, count)
			for i := 0; i < count; i++ {
				offset := 4 + i*rowSize
				rows[i] = *(*mibTCPRowOwnerPID)(unsafe.Pointer(&buf[offset]))
			}
			return rows, nil
		}
		if ret != errInsufficientBuffer {
			return nil, fmt.Errorf("GetExtendedTcpTable error %d", ret)
		}
	}
	return nil, fmt.Errorf("GetExtendedTcpTable exceeded buffer retries")
}

func readExtendedUDPTable() ([]mibUDPRowOwnerPID, error) {
	var size uint32
	for attempt := 0; attempt < 4; attempt++ {
		var buf []byte
		var ptr uintptr
		if size > 0 {
			buf = make([]byte, size)
			ptr = uintptr(unsafe.Pointer(&buf[0]))
		}
		ret, _, _ := procGetExtendedUdpTable.Call(
			ptr,
			uintptr(unsafe.Pointer(&size)),
			0,
			uintptr(afINET),
			uintptr(udpTableOwnerPID),
			0,
		)
		if ret == 0 && len(buf) >= 4 {
			count := int(*(*uint32)(unsafe.Pointer(&buf[0])))
			rowSize := int(unsafe.Sizeof(mibUDPRowOwnerPID{}))
			maxRows := (len(buf) - 4) / rowSize
			if count > maxRows {
				count = maxRows
			}
			rows := make([]mibUDPRowOwnerPID, count)
			for i := 0; i < count; i++ {
				offset := 4 + i*rowSize
				rows[i] = *(*mibUDPRowOwnerPID)(unsafe.Pointer(&buf[offset]))
			}
			return rows, nil
		}
		if ret != errInsufficientBuffer {
			return nil, fmt.Errorf("GetExtendedUdpTable error %d", ret)
		}
	}
	return nil, fmt.Errorf("GetExtendedUdpTable exceeded buffer retries")
}

func queryProcessDetails(pid uint32) (name string, exePath string, totalIO uint64, hasIO bool) {
	if pid == 0 {
		return "System Idle", "", 0, false
	}
	if pid == 4 {
		return "System", "ntoskrnl.exe", 0, false
	}

	h, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil || h == 0 {
		return fmt.Sprintf("PID %d (protegido)", pid), "", 0, false
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, 512)
	bufLen := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageW.Call(
		uintptr(h),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufLen)),
	)
	if ret != 0 && bufLen > 0 {
		exePath = windows.UTF16ToString(buf[:bufLen])
		name = filepath.Base(exePath)
	}
	if name == "" {
		name = fmt.Sprintf("PID %d", pid)
	}

	var io ioCounters
	ioRet, _, _ := procGetProcessIoCounters.Call(uintptr(h), uintptr(unsafe.Pointer(&io)))
	if ioRet != 0 {
		totalIO = io.ReadTransferCount + io.WriteTransferCount
		hasIO = true
	}
	return name, exePath, totalIO, hasIO
}

func ntohs(raw uint32) uint16 {
	return uint16(((raw & 0xFF) << 8) | ((raw >> 8) & 0xFF))
}

func uint32ToIPv4(addr uint32) string {
	return net.IPv4(
		byte(addr),
		byte(addr>>8),
		byte(addr>>16),
		byte(addr>>24),
	).String()
}

func tcpStateName(state uint32) string {
	switch state {
	case 1:
		return "CLOSED"
	case 2:
		return "LISTEN"
	case 3:
		return "SYN_SENT"
	case 4:
		return "SYN_RCVD"
	case 5:
		return "ESTABLISHED"
	case 6:
		return "FIN_WAIT1"
	case 7:
		return "FIN_WAIT2"
	case 8:
		return "CLOSE_WAIT"
	case 9:
		return "CLOSING"
	case 10:
		return "LAST_ACK"
	case 11:
		return "TIME_WAIT"
	case 12:
		return "DELETE_TCB"
	default:
		return "UNKNOWN"
	}
}
