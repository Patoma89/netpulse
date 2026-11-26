//go:build windows

package wifi

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"netpulse/internal/radar"
)

var (
	modWlanapi                = windows.NewLazySystemDLL("wlanapi.dll")
	procWlanOpenHandle        = modWlanapi.NewProc("WlanOpenHandle")
	procWlanCloseHandle       = modWlanapi.NewProc("WlanCloseHandle")
	procWlanFreeMemory        = modWlanapi.NewProc("WlanFreeMemory")
	procWlanEnumInterfaces    = modWlanapi.NewProc("WlanEnumInterfaces")
	procWlanScan              = modWlanapi.NewProc("WlanScan")
	procWlanGetNetworkBssList = modWlanapi.NewProc("WlanGetNetworkBssList")
	procWlanQueryInterface    = modWlanapi.NewProc("WlanQueryInterface")
)

const (
	wlanIntfStateConnected         uint32 = 1
	dot11BSSTypeAny                uint32 = 3
	wlanIntfOpcodeCurrentConn      uint32 = 7
	win32ErrAccessDenied           uintptr = 5
	win32ErrServiceNotActive       uintptr = 1062
)

type wlanInterfaceInfo struct {
	InterfaceGuid windows.GUID
	Description   [256]uint16
	State         uint32
}

type dot11SSID struct {
	Length uint32
	SSID   [32]byte
}

type wlanRateSet struct {
	Length  uint32
	RateSet [126]uint16
}

// wlanBSSEntry matches Win32 WLAN_BSS_ENTRY (360 bytes on windows/amd64).
type wlanBSSEntry struct {
	Dot11Ssid             dot11SSID
	PhyID                 uint32
	Dot11Bssid            [6]byte
	BssType               uint32
	PhyType               uint32
	Rssi                  int32
	LinkQuality           uint32
	InRegDomain           uint8
	BeaconPeriod          uint16
	Timestamp             uint64
	HostTimestamp         uint64
	CapabilityInformation uint16
	UlChCenterFrequency   uint32
	WlanRateSet           wlanRateSet
	UlIeOffset            uint32
	UlIeSize              uint32
}

type wlanAssociationAttributes struct {
	Dot11Ssid         dot11SSID
	Dot11BssType      uint32
	Dot11Bssid        [6]byte
	Dot11PhyType      uint32
	Dot11PhyIndex     uint32
	WlanSignalQuality uint32
	RxRate            uint32
	TxRate            uint32
}

type wlanConnectionAttributes struct {
	IsState                   uint32
	WlanConnectionMode        uint32
	ProfileName               [256]uint16
	WlanAssociationAttributes wlanAssociationAttributes
	// Security attributes follow in Win32 struct, not needed here.
}

// ScanWiFi reads nearby Wi-Fi BSSIDs and current link telemetry directly via Win32 wlanapi.dll.
func ScanWiFi(activeScan bool) ScanResult {
	res := ScanResult{
		Status:       "ok",
		CompletedAt:  time.Now().Format(time.RFC3339),
		AccessPoints: make([]AccessPoint, 0),
		Scores24GHz:  make([]ChannelScore, 0),
		Scores5GHz:   make([]ChannelScore, 0),
		Scores6GHz:   make([]ChannelScore, 0),
	}

	if err := modWlanapi.Load(); err != nil {
		res.Status = "no_wifi_adapter"
		res.ErrorMessage = "No se encontró el servicio WLAN de Windows en este equipo."
		return res
	}

	var negVersion uint32
	var handle uintptr
	ret, _, _ := procWlanOpenHandle.Call(
		2,
		0,
		uintptr(unsafe.Pointer(&negVersion)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if ret != 0 {
		if ret == win32ErrServiceNotActive {
			res.Status = "no_wifi_adapter"
			res.ErrorMessage = "El servicio WLAN AutoConfig (WlanSvc) está inactivo; parece una PC conectada solo por cable Ethernet."
			return res
		}
		if ret == win32ErrAccessDenied {
			res.Status = "location_denied"
			res.ErrorMessage = "Windows bloqueó el acceso a WLAN por permisos de ubicación."
			return res
		}
		res.Status = "error"
		res.ErrorMessage = fmt.Sprintf("WlanOpenHandle error %d", ret)
		return res
	}
	defer procWlanCloseHandle.Call(handle, 0)

	var ifListPtr uintptr
	ret, _, _ = procWlanEnumInterfaces.Call(handle, 0, uintptr(unsafe.Pointer(&ifListPtr)))
	if ret != 0 {
		res.Status = "error"
		res.ErrorMessage = fmt.Sprintf("WlanEnumInterfaces error %d", ret)
		return res
	}
	if ifListPtr == 0 {
		res.Status = "no_wifi_adapter"
		res.ErrorMessage = "No se detectaron placas Wi-Fi físicas en esta PC."
		return res
	}
	defer procWlanFreeMemory.Call(ifListPtr)

	numIfaces := *(*uint32)(unsafe.Pointer(ifListPtr))
	if numIfaces == 0 {
		res.Status = "no_wifi_adapter"
		res.ErrorMessage = "No se detectaron placas Wi-Fi físicas en esta PC."
		return res
	}

	ifaceBase := ifListPtr + 8
	ifaceStride := unsafe.Sizeof(wlanInterfaceInfo{})
	selected := (*wlanInterfaceInfo)(unsafe.Pointer(ifaceBase))
	for i := uint32(0); i < numIfaces; i++ {
		cand := (*wlanInterfaceInfo)(unsafe.Pointer(ifaceBase + uintptr(i)*ifaceStride))
		if cand.State == wlanIntfStateConnected {
			selected = cand
			break
		}
	}

	res.AdapterName = windows.UTF16ToString(selected.Description[:])
	res.Connected = selected.State == wlanIntfStateConnected

	if res.Connected {
		var dataSize uint32
		var dataPtr uintptr
		var opcodeValType uint32
		qRet, _, _ := procWlanQueryInterface.Call(
			handle,
			uintptr(unsafe.Pointer(&selected.InterfaceGuid)),
			uintptr(wlanIntfOpcodeCurrentConn),
			0,
			uintptr(unsafe.Pointer(&dataSize)),
			uintptr(unsafe.Pointer(&dataPtr)),
			uintptr(unsafe.Pointer(&opcodeValType)),
		)
		if qRet == 0 && dataPtr != 0 {
			if uintptr(dataSize) >= unsafe.Sizeof(wlanConnectionAttributes{}) {
				attrs := (*wlanConnectionAttributes)(unsafe.Pointer(dataPtr))
				assoc := attrs.WlanAssociationAttributes
				res.CurrentSSID = decodeSSID(assoc.Dot11Ssid)
				res.CurrentBSSID = formatBSSID(assoc.Dot11Bssid[:])
				res.RxRateMbps = int(assoc.RxRate / 1000)
				res.TxRateMbps = int(assoc.TxRate / 1000)
			}
			procWlanFreeMemory.Call(dataPtr)
		}
	}

	if activeScan {
		sRet, _, _ := procWlanScan.Call(
			handle,
			uintptr(unsafe.Pointer(&selected.InterfaceGuid)),
			0,
			0,
			0,
		)
		if sRet == win32ErrAccessDenied {
			res.Status = "location_denied"
			res.ErrorMessage = "Windows 11 requiere activar los Servicios de Ubicación para aplicaciones de escritorio para escanear redes Wi-Fi cercanas."
			return res
		}
		if sRet == 0 {
			time.Sleep(900 * time.Millisecond)
		}
	}

	var bssListPtr uintptr
	bRet, _, _ := procWlanGetNetworkBssList.Call(
		handle,
		uintptr(unsafe.Pointer(&selected.InterfaceGuid)),
		0,
		uintptr(dot11BSSTypeAny),
		0,
		0,
		uintptr(unsafe.Pointer(&bssListPtr)),
	)
	if bRet == win32ErrAccessDenied {
		res.Status = "location_denied"
		res.ErrorMessage = "Windows 11 bloqueó la lectura de canales Wi-Fi cercanos (ERROR_ACCESS_DENIED) porque el permiso de Ubicación está desactivado."
		return res
	}
	if bRet != 0 {
		res.Status = "error"
		res.ErrorMessage = fmt.Sprintf("WlanGetNetworkBssList error %d", bRet)
		return res
	}
	if bssListPtr == 0 {
		if res.Connected {
			res.Status = "location_denied"
			res.ErrorMessage = "Windows devolvió una lista vacía de redes Wi-Fi. Verificá que Ubicación para apps de escritorio esté activada en Windows 11."
		}
		return res
	}
	defer procWlanFreeMemory.Call(bssListPtr)

	totalSize := *(*uint32)(unsafe.Pointer(bssListPtr))
	numItems := *(*uint32)(unsafe.Pointer(bssListPtr + 4))
	entrySize := unsafe.Sizeof(wlanBSSEntry{})

	if totalSize < 8 {
		return res
	}
	maxItems := uint32((uintptr(totalSize) - 8) / entrySize)
	if numItems > maxItems {
		numItems = maxItems
	}

	rawBytes := unsafe.Slice((*byte)(unsafe.Pointer(bssListPtr)), totalSize)
	byBSSID := make(map[string]AccessPoint, numItems)

	for i := uint32(0); i < numItems; i++ {
		entryOffset := uintptr(8) + uintptr(i)*entrySize
		entry := (*wlanBSSEntry)(unsafe.Pointer(bssListPtr + entryOffset))

		band, ch, freqMHz := FrequencyKHzToBandChannel(entry.UlChCenterFrequency)
		if band == "unknown" || ch == 0 {
			continue
		}

		bssid := formatBSSID(entry.Dot11Bssid[:])
		if bssid == "" {
			continue
		}

		ssid := decodeSSID(entry.Dot11Ssid)
		hidden := ssid == ""
		if hidden {
			ssid = "(Red oculta)"
		}

		var ieSlice []byte
		if entry.UlIeSize > 0 {
			// Win32 WLAN_BSS_ENTRY.ulIeOffset is relative to the start of that WLAN_BSS_ENTRY.
			ieStart := uint64(entryOffset) + uint64(entry.UlIeOffset)
			ieEnd := ieStart + uint64(entry.UlIeSize)
			if ieStart >= uint64(entryOffset) && ieEnd <= uint64(totalSize) {
				ieSlice = rawBytes[ieStart:ieEnd]
			}
		}

		ie := ParseIEs(ieSlice, ch, band, entry.PhyType, entry.CapabilityInformation)
		vendor, _ := radar.LookupVendor(bssid)
		isCurrent := res.CurrentBSSID != "" && strings.EqualFold(bssid, res.CurrentBSSID)

		ap := AccessPoint{
			SSID:          ssid,
			Hidden:        hidden,
			BSSID:         bssid,
			Vendor:        vendor,
			Band:          band,
			Channel:       ch,
			CenterChannel: ie.CenterChannel,
			FrequencyMHz:  freqMHz,
			WidthMHz:      ie.WidthMHz,
			RSSI:          int(entry.Rssi),
			LinkQuality:   int(entry.LinkQuality),
			Generation:    ie.Generation,
			Security:      ie.Security,
			IsDFS:         band == "5GHz" && IsDFSChannel(ch),
			IsCurrent:     isCurrent,
		}

		if isCurrent {
			res.CurrentBand = ap.Band
			res.CurrentChannel = ap.Channel
			res.CurrentWidthMHz = ap.WidthMHz
			res.CurrentRSSI = ap.RSSI
		}

		if prev, exists := byBSSID[bssid]; !exists || ap.RSSI > prev.RSSI {
			byBSSID[bssid] = ap
		}
	}

	if res.Connected && len(byBSSID) == 0 {
		res.Status = "location_denied"
		res.ErrorMessage = "Windows ocultó las redes Wi-Fi cercanas por permisos de Ubicación (Windows 11 24H2+)."
		return res
	}

	aps := make([]AccessPoint, 0, len(byBSSID))
	for _, ap := range byBSSID {
		aps = append(aps, ap)
	}
	sort.Slice(aps, func(i, j int) bool {
		if aps[i].IsCurrent != aps[j].IsCurrent {
			return aps[i].IsCurrent
		}
		if aps[i].RSSI != aps[j].RSSI {
			return aps[i].RSSI > aps[j].RSSI
		}
		return aps[i].SSID < aps[j].SSID
	})

	scores24, scores5, rec24, rec5 := AnalyzeSpectrum(aps, res.CurrentBand, res.CurrentChannel)
	scores6, rec6 := AnalyzeSpectrum6GHz(aps, res.CurrentBand, res.CurrentChannel)
	res.AccessPoints = aps
	res.Scores24GHz = scores24
	res.Scores5GHz = scores5
	res.Scores6GHz = scores6
	res.Rec24GHz = rec24
	res.Rec5GHz = rec5
	res.Rec6GHz = rec6

	return res
}

// OpenLocationSettings opens Windows Privacy -> Location settings natively via ShellExecute.
func OpenLocationSettings() error {
	verb := windows.StringToUTF16Ptr("open")
	target := windows.StringToUTF16Ptr("ms-settings:privacy-location")
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}

func decodeSSID(s dot11SSID) string {
	n := int(s.Length)
	if n <= 0 {
		return ""
	}
	if n > len(s.SSID) {
		n = len(s.SSID)
	}
	raw := string(s.SSID[:n])
	return strings.TrimRight(raw, "\x00")
}

func formatBSSID(b []byte) string {
	if len(b) < 6 {
		return ""
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[0], b[1], b[2], b[3], b[4], b[5])
}
