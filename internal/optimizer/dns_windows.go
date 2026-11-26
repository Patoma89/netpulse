//go:build windows

package optimizer

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modDnsapi                = windows.NewLazySystemDLL("dnsapi.dll")
	procDnsFlushResolver     = modDnsapi.NewProc("DnsFlushResolverCache")
	modShell32               = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW      = modShell32.NewProc("ShellExecuteExW")
)

const (
	seeMaskNoCloseProcess uint32 = 0x00000040
	errWin32Cancelled     uint32 = 1223
)

// shellExecuteInfoW matches Win32 SHELLEXECUTEINFOW (112 bytes on windows/amd64).
type shellExecuteInfoW struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         uintptr
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	HInstApp     uintptr
	LpIDList     uintptr
	LpClass      *uint16
	HkeyClass    uintptr
	DwHotKey     uint32
	HIcon        uintptr
	HProcess     uintptr
}

// ValidateIPv4List strictly verifies each non-empty address is a canonical IPv4 literal.
func ValidateIPv4List(ips ...string) ([]string, error) {
	out := make([]string, 0, len(ips))
	for _, raw := range ips {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parsed := net.ParseIP(trimmed)
		v4 := parsed.To4()
		if v4 == nil {
			return nil, fmt.Errorf("dirección IPv4 inválida: %q", raw)
		}
		out = append(out, v4.String())
	}
	if len(out) == 0 {
		return nil, errors.New("debe indicar al menos una dirección DNS IPv4 válida")
	}
	return out, nil
}

// FlushDNSCache flushes the Windows DNS client resolver cache without Admin privileges.
func FlushDNSCache() error {
	ret, _, callErr := procDnsFlushResolver.Call()
	if ret == 0 {
		return fmt.Errorf("DnsFlushResolverCache falló: %v", callErr)
	}
	return nil
}

// ApplyStaticDNS updates the Windows adapter DNS servers using on-demand UAC elevation.
func ApplyStaticDNS(ifIndex uint32, primary, secondary string) error {
	if ifIndex == 0 {
		return errors.New("índice de adaptador inválido")
	}
	cleanIPs, err := ValidateIPv4List(primary, secondary)
	if err != nil {
		return err
	}

	quoted := make([]string, len(cleanIPs))
	for i, ip := range cleanIPs {
		quoted[i] = "'" + ip + "'"
	}
	psCmd := fmt.Sprintf(
		"Set-DnsClientServerAddress -InterfaceIndex %d -ServerAddresses (%s); Clear-DnsClientCache",
		ifIndex,
		strings.Join(quoted, ","),
	)
	return runElevatedPowerShell(psCmd)
}

// ResetDNSToDHCP restores automatic DHCP DNS configuration on the adapter via on-demand UAC.
func ResetDNSToDHCP(ifIndex uint32) error {
	if ifIndex == 0 {
		return errors.New("índice de adaptador inválido")
	}
	psCmd := fmt.Sprintf(
		"Set-DnsClientServerAddress -InterfaceIndex %d -ResetServerAddresses; Clear-DnsClientCache",
		ifIndex,
	)
	return runElevatedPowerShell(psCmd)
}

func runElevatedPowerShell(script string) error {
	args := "-NoProfile -NonInteractive -WindowStyle Hidden -Command " + script

	sei := shellExecuteInfoW{
		CbSize:       uint32(unsafe.Sizeof(shellExecuteInfoW{})),
		FMask:        seeMaskNoCloseProcess,
		LpVerb:       windows.StringToUTF16Ptr("runas"),
		LpFile:       windows.StringToUTF16Ptr("powershell.exe"),
		LpParameters: windows.StringToUTF16Ptr(args),
		NShow:        windows.SW_HIDE,
	}

	ret, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if ret == 0 {
		if errno, ok := callErr.(windows.Errno); ok && uint32(errno) == errWin32Cancelled {
			return errors.New("operación cancelada: no se otorgaron permisos de Administrador (UAC)")
		}
		return fmt.Errorf("no se pudo elevar privilegios: %v", callErr)
	}

	if sei.HProcess == 0 {
		return nil
	}
	handle := windows.Handle(sei.HProcess)
	defer windows.CloseHandle(handle)

	event, waitErr := windows.WaitForSingleObject(handle, 15000)
	if waitErr != nil {
		return fmt.Errorf("esperando comando elevado: %w", waitErr)
	}
	if event == uint32(windows.WAIT_TIMEOUT) {
		return errors.New("el cambio de DNS excedió el tiempo de espera")
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("PowerShell devolvió código de error %d al configurar DNS", exitCode)
	}
	return nil
}
