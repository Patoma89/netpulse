package wifi

import "bytes"

// IEInfo captures parsed 802.11 Information Elements for width, center channel, generation, and security.
type IEInfo struct {
	WidthMHz      int
	CenterChannel int
	Generation    string
	Security      string
	HasHT         bool
	HasVHT        bool
	HasHE         bool
	HasEHT        bool
}

const (
	ieHTCaps     byte = 45
	ieRSN        byte = 48
	ieHTOp       byte = 61
	ieVHTCaps    byte = 191
	ieVHTOp      byte = 192
	ieVendorSpec byte = 221
	ieExtension  byte = 255

	extHECaps byte = 35
	extHEOp   byte = 36
	extEHTCap byte = 108
	extEHTOp  byte = 106
)

var (
	akmSuiteRSNOUI = []byte{0x00, 0x0F, 0xAC}
	wpaVendorOUI   = []byte{0x00, 0x50, 0xF2, 0x01}
)

// ParseIEs safely decodes raw 802.11 Information Elements from a beacon/probe response blob.
func ParseIEs(ieBlob []byte, primaryChannel int, band string, phyType uint32, capInfo uint16) IEInfo {
	info := IEInfo{
		WidthMHz:      20,
		CenterChannel: primaryChannel,
		Generation:    generationFromPhyType(phyType),
		Security:      "Abierta",
	}

	// Capability bit 4 (0x0010) indicates Privacy (WEP or higher)
	privacyBit := capInfo&0x0010 != 0
	var hasRSN, hasWPA, hasWPA2PSK, hasWPA3SAE bool
	var htSecOffset byte
	var ht40Supported bool
	var vhtWidth byte
	var vhtCenter0 int

	for i := 0; i+2 <= len(ieBlob); {
		id := ieBlob[i]
		length := int(ieBlob[i+1])
		dataStart := i + 2
		dataEnd := dataStart + length
		if dataEnd > len(ieBlob) {
			break
		}
		data := ieBlob[dataStart:dataEnd]

		switch id {
		case ieHTCaps:
			info.HasHT = true
			if len(data) >= 2 {
				// HT Capability Info bit 1 (0x0002) = Supported Channel Width Set (20/40 MHz)
				ht40Supported = data[0]&0x02 != 0
			}
		case ieHTOp:
			info.HasHT = true
			if len(data) >= 2 {
				// Secondary channel offset bits 0..1: 1 = above, 3 = below, 0 = none
				htSecOffset = data[1] & 0x03
			}
		case ieVHTCaps:
			info.HasVHT = true
		case ieVHTOp:
			info.HasVHT = true
			if len(data) >= 2 {
				vhtWidth = data[0]
				vhtCenter0 = int(data[1])
			}
		case ieRSN:
			hasRSN = true
			wpa2, wpa3 := parseRSNAKM(data)
			if wpa2 {
				hasWPA2PSK = true
			}
			if wpa3 {
				hasWPA3SAE = true
			}
		case ieVendorSpec:
			if len(data) >= 4 && bytes.Equal(data[:4], wpaVendorOUI) {
				hasWPA = true
			}
		case ieExtension:
			if len(data) >= 1 {
				switch data[0] {
				case extHECaps, extHEOp:
					info.HasHE = true
				case extEHTCap, extEHTOp:
					info.HasEHT = true
				}
			}
		}

		i = dataEnd
	}

	// Determine channel width & center channel
	if vhtWidth >= 2 {
		info.WidthMHz = 160
		if vhtCenter0 > 0 {
			info.CenterChannel = vhtCenter0
		}
	} else if vhtWidth == 1 {
		info.WidthMHz = 80
		if vhtCenter0 > 0 {
			info.CenterChannel = vhtCenter0
		}
	} else if htSecOffset == 1 {
		info.WidthMHz = 40
		if primaryChannel > 0 {
			info.CenterChannel = primaryChannel + 2
		}
	} else if htSecOffset == 3 {
		info.WidthMHz = 40
		if primaryChannel > 2 {
			info.CenterChannel = primaryChannel - 2
		}
	} else if ht40Supported && band == "5GHz" && info.HasVHT {
		info.WidthMHz = 40
	}

	// Determine Wi-Fi generation (highest supported standard wins)
	switch {
	case info.HasEHT || phyType == 10:
		info.Generation = "Wi-Fi 7"
	case info.HasHE || phyType == 9:
		if band == "6GHz" {
			info.Generation = "Wi-Fi 6E"
		} else {
			info.Generation = "Wi-Fi 6"
		}
	case info.HasVHT || phyType == 8:
		info.Generation = "Wi-Fi 5"
	case info.HasHT || phyType == 7:
		info.Generation = "Wi-Fi 4"
	}

	// Determine security mode
	switch {
	case hasRSN && hasWPA3SAE && hasWPA2PSK:
		info.Security = "WPA2/WPA3"
	case hasRSN && hasWPA3SAE:
		info.Security = "WPA3"
	case hasRSN:
		info.Security = "WPA2"
	case hasWPA:
		info.Security = "WPA"
	case privacyBit:
		info.Security = "WEP"
	default:
		info.Security = "Abierta"
	}

	return info
}

func parseRSNAKM(data []byte) (hasWPA2PSK, hasWPA3SAE bool) {
	// RSN IE layout: Version (2) + Group Cipher (4) + Pairwise Count (2) + Pairwise Suites (4*N) + AKM Count (2) + AKM Suites (4*M)
	if len(data) < 8 {
		return false, false
	}
	pairwiseCount := int(data[6]) | int(data[7])<<8
	pos := 8 + pairwiseCount*4
	if pos+2 > len(data) {
		return false, false
	}
	akmCount := int(data[pos]) | int(data[pos+1])<<8
	pos += 2
	for i := 0; i < akmCount && pos+4 <= len(data); i++ {
		suite := data[pos : pos+4]
		if bytes.Equal(suite[:3], akmSuiteRSNOUI) {
			switch suite[3] {
			case 2, 4, 6: // PSK / FT-PSK / PSK-SHA256
				hasWPA2PSK = true
			case 8, 9, 18, 24: // SAE / FT-SAE / OWE / SAE-EXT
				hasWPA3SAE = true
			}
		}
		pos += 4
	}
	return hasWPA2PSK, hasWPA3SAE
}

func generationFromPhyType(phyType uint32) string {
	switch phyType {
	case 7:
		return "Wi-Fi 4"
	case 8:
		return "Wi-Fi 5"
	case 9:
		return "Wi-Fi 6"
	case 10:
		return "Wi-Fi 7"
	case 4, 5, 6:
		return "Legacy"
	default:
		return "Wi-Fi"
	}
}
