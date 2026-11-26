//go:build windows

package tray

import (
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32            = windows.NewLazySystemDLL("user32.dll")
	modShell32           = windows.NewLazySystemDLL("shell32.dll")
	modKernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandleW = modKernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW  = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW   = modUser32.NewProc("DefWindowProcW")
	procDestroyWindow    = modUser32.NewProc("DestroyWindow")
	procPostQuitMessage  = modUser32.NewProc("PostQuitMessage")
	procPostMessageW     = modUser32.NewProc("PostMessageW")
	procGetMessageW      = modUser32.NewProc("GetMessageW")
	procTranslateMessage = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW = modUser32.NewProc("DispatchMessageW")
	procLoadIconW        = modUser32.NewProc("LoadIconW")
	procCreatePopupMenu  = modUser32.NewProc("CreatePopupMenu")
	procAppendMenuW      = modUser32.NewProc("AppendMenuW")
	procDestroyMenu      = modUser32.NewProc("DestroyMenu")
	procTrackPopupMenu   = modUser32.NewProc("TrackPopupMenu")
	procSetForegroundWnd = modUser32.NewProc("SetForegroundWindow")
	procGetCursorPos     = modUser32.NewProc("GetCursorPos")
	procShellNotifyIconW = modShell32.NewProc("Shell_NotifyIconW")
	procExtractIconExW   = modShell32.NewProc("ExtractIconExW")
)

const (
	wmDestroy     uint32 = 0x0002
	wmCommand     uint32 = 0x0111
	wmUser        uint32 = 0x0400
	wmTray        uint32 = wmUser + 42
	wmLButtonDbl  uintptr = 0x0203
	wmRButtonUp   uintptr = 0x0205
	wmContextMenu uintptr = 0x007B

	nimAdd        uint32 = 0x00000000
	nimModify     uint32 = 0x00000001
	nimDelete     uint32 = 0x00000002
	nifMessage    uint32 = 0x00000001
	nifIcon       uint32 = 0x00000002
	nifTip        uint32 = 0x00000004
	nifInfo       uint32 = 0x00000010
	niifInfo      uint32 = 0x00000001
	idiApp        uintptr = 32512

	mfString    uintptr = 0x0000
	mfSeparator uintptr = 0x0800
	tpmRightBtn uintptr = 0x0002
	tpmBottom   uintptr = 0x0020

	cmdOpenWindow = 1001
	cmdScanRadar  = 1002
	cmdExitApp    = 1003
)

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type winPoint struct {
	X int32
	Y int32
}

type winMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
}

// Callbacks connects native Windows tray menu actions to the Wails app.
type Callbacks struct {
	OnOpen func()
	OnScan func()
	OnQuit func()
}

// Manager owns the Win32 system tray icon and balloon notifications (pure Go, no CGO).
type Manager struct {
	mu      sync.Mutex
	hwnd    uintptr
	hIcon   uintptr
	started bool
	cb      Callbacks
}

var (
	activeMgrMu sync.RWMutex
	activeMgr   *Manager
)

// New creates a Win32 tray Manager.
func New(cb Callbacks) *Manager {
	return &Manager{cb: cb}
}

// Start launches the tray icon message loop on a dedicated locked OS thread.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	activeMgrMu.Lock()
	activeMgr = m
	activeMgrMu.Unlock()

	go m.runLoop()
}

func (m *Manager) runLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className := windows.StringToUTF16Ptr("NetPulseTrayMsgWindow")

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		HInstance:     hInst,
		LpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwndMessage := ^uintptr(2) // HWND_MESSAGE = (HWND)-3
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("NetPulse Tray"))),
		0,
		0, 0, 0, 0,
		hwndMessage,
		0,
		hInst,
		0,
	)
	if hwnd == 0 {
		return
	}

	hIcon := loadAppTrayIcon()

	m.mu.Lock()
	m.hwnd = hwnd
	m.hIcon = hIcon
	m.mu.Unlock()

	nid := notifyIconDataW{
		CbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTray,
		HIcon:            hIcon,
	}
	copyUTF16Fixed(nid.SzTip[:], "NetPulse — Monitor y Radar de Red")
	procShellNotifyIconW.Call(uintptr(nimAdd), uintptr(unsafe.Pointer(&nid)))

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	delNid := notifyIconDataW{
		CbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		HWnd:   hwnd,
		UID:    1,
	}
	procShellNotifyIconW.Call(uintptr(nimDelete), uintptr(unsafe.Pointer(&delNid)))
}

// Notify displays a Windows system tray balloon notification.
func (m *Manager) Notify(title, message string) {
	m.mu.Lock()
	hwnd := m.hwnd
	hIcon := m.hIcon
	m.mu.Unlock()

	if hwnd == 0 {
		return
	}

	nid := notifyIconDataW{
		CbSize:      uint32(unsafe.Sizeof(notifyIconDataW{})),
		HWnd:        hwnd,
		UID:         1,
		UFlags:      nifInfo | nifIcon,
		HIcon:       hIcon,
		DwInfoFlags: niifInfo,
	}
	copyUTF16Fixed(nid.SzInfoTitle[:], title)
	copyUTF16Fixed(nid.SzInfo[:], message)
	procShellNotifyIconW.Call(uintptr(nimModify), uintptr(unsafe.Pointer(&nid)))
}

// Stop removes the tray icon and closes the message window.
func (m *Manager) Stop() {
	m.mu.Lock()
	hwnd := m.hwnd
	m.hwnd = 0
	m.started = false
	m.mu.Unlock()

	if hwnd != 0 {
		procPostMessageW.Call(hwnd, uintptr(wmDestroy), 0, 0)
	}
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmTray:
		if lParam == wmLButtonDbl {
			activeMgrMu.RLock()
			mgr := activeMgr
			activeMgrMu.RUnlock()
			if mgr != nil && mgr.cb.OnOpen != nil {
				go mgr.cb.OnOpen()
			}
			return 0
		}
		if lParam == wmRButtonUp || lParam == wmContextMenu {
			showTrayMenu(hwnd)
			return 0
		}
	case wmCommand:
		cmd := int(wParam & 0xFFFF)
		activeMgrMu.RLock()
		mgr := activeMgr
		activeMgrMu.RUnlock()
		if mgr != nil {
			switch cmd {
			case cmdOpenWindow:
				if mgr.cb.OnOpen != nil {
					go mgr.cb.OnOpen()
				}
			case cmdScanRadar:
				if mgr.cb.OnScan != nil {
					go mgr.cb.OnScan()
				}
			case cmdExitApp:
				if mgr.cb.OnQuit != nil {
					go mgr.cb.OnQuit()
				}
			}
		}
		return 0
	case wmDestroy:
		procDestroyWindow.Call(hwnd)
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func showTrayMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	procAppendMenuW.Call(menu, mfString, cmdOpenWindow, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Abrir NetPulse"))))
	procAppendMenuW.Call(menu, mfString, cmdScanRadar, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Escanear Radar LAN"))))
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	procAppendMenuW.Call(menu, mfString, cmdExitApp, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Salir"))))

	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWnd.Call(hwnd)
	procTrackPopupMenu.Call(menu, tpmRightBtn|tpmBottom, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
}

func loadAppTrayIcon() uintptr {
	if exePath, err := os.Executable(); err == nil && exePath != "" {
		var smallIcon uintptr
		ret, _, _ := procExtractIconExW.Call(
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(exePath))),
			0,
			0,
			uintptr(unsafe.Pointer(&smallIcon)),
			1,
		)
		if ret > 0 && smallIcon != 0 {
			return smallIcon
		}
	}
	icon, _, _ := procLoadIconW.Call(0, idiApp)
	return icon
}

func copyUTF16Fixed(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	u16, err := windows.UTF16FromString(s)
	if err != nil {
		return
	}
	n := len(u16)
	if n > len(dst) {
		n = len(dst)
		u16[n-1] = 0
	}
	copy(dst[:n], u16[:n])
}
