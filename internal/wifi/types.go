package wifi

// AccessPoint describes a nearby or currently connected Wi-Fi BSSID radio.
type AccessPoint struct {
	SSID          string `json:"ssid"`
	Hidden        bool   `json:"hidden"`
	BSSID         string `json:"bssid"`
	Vendor        string `json:"vendor"`
	Band          string `json:"band"` // "2.4GHz" | "5GHz" | "6GHz"
	Channel       int    `json:"channel"`
	CenterChannel int    `json:"centerChannel"`
	FrequencyMHz  int    `json:"frequencyMHz"`
	WidthMHz      int    `json:"widthMHz"` // 20 | 40 | 80 | 160
	RSSI          int    `json:"rssi"`
	LinkQuality   int    `json:"linkQuality"`
	Generation    string `json:"generation"`
	Security      string `json:"security"`
	IsDFS         bool   `json:"isDfs"`
	IsCurrent     bool   `json:"isCurrent"`
}

// ChannelScore holds physical RF congestion and overlap metrics for a single channel.
type ChannelScore struct {
	Channel         int     `json:"channel"`
	Band            string  `json:"band"`
	CongestionPct   int     `json:"congestionPct"`
	QualityScore    int     `json:"qualityScore"`
	InterferenceDBm float64 `json:"interferenceDbm"`
	CoChannelCount  int     `json:"coChannelCount"`
	OverlapCount    int     `json:"overlapCount"`
	IsDFS           bool    `json:"isDfs"`
	IsRecommended   bool    `json:"isRecommended"`
	IsCurrent       bool    `json:"isCurrent"`
}

// BandRecommendation summarizes the optimal channel choice for 2.4 GHz or 5 GHz.
type BandRecommendation struct {
	Band               string `json:"band"`
	CurrentChannel     int    `json:"currentChannel"`
	RecommendedChannel int    `json:"recommendedChannel"`
	CurrentCongestion  int    `json:"currentCongestion"`
	BestCongestion     int    `json:"bestCongestion"`
	ImprovementPct     int    `json:"improvementPct"`
	ShouldSwitch       bool   `json:"shouldSwitch"`
	Status             string `json:"status"` // "optimal" | "switch_recommended" | "no_data"
	Headline           string `json:"headline"`
	Detail             string `json:"detail"`
}

// ScanResult holds the full Win32 WLAN scan output, permissions state, and RF recommendations.
type ScanResult struct {
	Status          string             `json:"status"` // "ok" | "location_denied" | "no_wifi_adapter" | "error"
	ErrorMessage    string             `json:"errorMessage"`
	AdapterName     string             `json:"adapterName"`
	Connected       bool               `json:"connected"`
	CurrentSSID     string             `json:"currentSsid"`
	CurrentBSSID    string             `json:"currentBssid"`
	CurrentBand     string             `json:"currentBand"`
	CurrentChannel  int                `json:"currentChannel"`
	CurrentWidthMHz int                `json:"currentWidthMHz"`
	CurrentRSSI     int                `json:"currentRssi"`
	RxRateMbps      int                `json:"rxRateMbps"`
	TxRateMbps      int                `json:"txRateMbps"`
	CompletedAt     string             `json:"completedAt"`
	AccessPoints    []AccessPoint      `json:"accessPoints"`
	Scores24GHz     []ChannelScore     `json:"scores24Ghz"`
	Scores5GHz      []ChannelScore     `json:"scores5Ghz"`
	Scores6GHz      []ChannelScore     `json:"scores6Ghz"`
	Rec24GHz        BandRecommendation `json:"rec24Ghz"`
	Rec5GHz         BandRecommendation `json:"rec5Ghz"`
	Rec6GHz         BandRecommendation `json:"rec6Ghz"`
}

// FrequencyKHzToBandChannel converts a center frequency in kHz (from Win32 WLAN_BSS_ENTRY)
// into Wi-Fi band, primary/center channel number, and frequency in MHz.
func FrequencyKHzToBandChannel(freqKHz uint32) (band string, channel int, freqMHz int) {
	freqMHz = int(freqKHz / 1000)

	switch {
	case freqMHz >= 2412 && freqMHz <= 2472:
		return "2.4GHz", (freqMHz-2412)/5 + 1, freqMHz
	case freqMHz == 2484:
		return "2.4GHz", 14, freqMHz
	case freqMHz >= 5180 && freqMHz <= 5885:
		return "5GHz", (freqMHz - 5000) / 5, freqMHz
	case freqMHz >= 5955 && freqMHz <= 7115:
		return "6GHz", (freqMHz - 5950) / 5, freqMHz
	default:
		return "unknown", 0, freqMHz
	}
}

// ChannelToCenterFreqMHz converts a 2.4 GHz or 5 GHz channel number to its center frequency in MHz.
func ChannelToCenterFreqMHz(band string, channel int) int {
	switch band {
	case "2.4GHz":
		if channel >= 1 && channel <= 13 {
			return 2412 + (channel-1)*5
		}
		if channel == 14 {
			return 2484
		}
	case "5GHz":
		if channel >= 36 && channel <= 177 {
			return 5000 + channel*5
		}
	case "6GHz":
		if channel >= 1 && channel <= 233 {
			return 5950 + channel*5
		}
	}
	return 0
}

// IsDFSChannel reports whether a 5 GHz channel falls inside UNII-2/UNII-2e DFS radar bands (52..144).
func IsDFSChannel(channel int) bool {
	return channel >= 52 && channel <= 144
}
