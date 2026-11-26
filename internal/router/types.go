package router

// RouterProfile describes the detected local gateway hardware, admin URL, and Co-Pilot guidance.
type RouterProfile struct {
	Detected           bool   `json:"detected"`
	GatewayIP          string `json:"gatewayIp"`
	GatewayMAC         string `json:"gatewayMac"`
	Vendor             string `json:"vendor"`
	AdminURL           string `json:"adminUrl"`
	HTTPTitle          string `json:"httpTitle"`
	ServerHeader       string `json:"serverHeader"`
	MenuGuide          string `json:"menuGuide"`
	SuggestedDriver    string `json:"suggestedDriver"` // "openwrt" | "mikrotik" | "tr064" | ""
	AutoPilotSupported bool   `json:"autoPilotSupported"`
	Rec24Channel       int    `json:"rec24Channel"`
	Rec5Channel        int    `json:"rec5Channel"`
	RecDNSName         string `json:"recDnsName"`
	RecDNSPrimary      string `json:"recDnsPrimary"`
}

// BeforeAfterSnapshot captures network telemetry at a point in time for Co-Pilot verification.
type BeforeAfterSnapshot struct {
	CapturedAt     string  `json:"capturedAt"`
	RouterAvgMs    float64 `json:"routerAvgMs"`
	RouterJitterMs float64 `json:"routerJitterMs"`
	RouterLossPct  float64 `json:"routerLossPct"`
	InternetAvgMs  float64 `json:"internetAvgMs"`
	WiFiBand       string  `json:"wifiBand"`
	WiFiChannel    int     `json:"wifiChannel"`
	WiFiCongestion int     `json:"wifiCongestion"`
	Score          int     `json:"score"`
}

// BeforeAfterReport compares baseline vs post-change telemetry for Co-Pilot verification.
type BeforeAfterReport struct {
	HasBaseline     bool                 `json:"hasBaseline"`
	Baseline        BeforeAfterSnapshot  `json:"baseline"`
	After           *BeforeAfterSnapshot `json:"after,omitempty"`
	DeltaRouterMs   float64              `json:"deltaRouterMs"`
	DeltaJitterMs   float64              `json:"deltaJitterMs"`
	DeltaCongestion int                  `json:"deltaCongestion"`
	DeltaScore      int                  `json:"deltaScore"`
	Verdict         string               `json:"verdict"`
}

// AutoPilotConfig configures connection settings for a supported router driver.
type AutoPilotConfig struct {
	Driver          string `json:"driver"` // "openwrt" | "mikrotik" | "tr064"
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password,omitempty"`
	UseTLS          bool   `json:"useTls"`
	SaveCredentials bool   `json:"saveCredentials"`
	HasSavedCreds   bool   `json:"hasSavedCreds"`
}

// RadioChannelInfo describes one wireless radio detected on a router via Auto-Pilot.
type RadioChannelInfo struct {
	InterfaceName  string `json:"interfaceName"`
	Band           string `json:"band"` // "2.4GHz" | "5GHz"
	CurrentChannel int    `json:"currentChannel"`
	WidthMHz       int    `json:"widthMHz"`
}

// AutoPilotStatus reports the outcome of testing a router driver connection.
type AutoPilotStatus struct {
	Connected       bool               `json:"connected"`
	Driver          string             `json:"driver"`
	Model           string             `json:"model"`
	Message         string             `json:"message"`
	HostFingerprint string             `json:"hostFingerprint,omitempty"`
	Radios          []RadioChannelInfo `json:"radios"`
}

// ChannelChangeRequest holds parameters and explicit safety confirmations for applying a Wi-Fi channel.
type ChannelChangeRequest struct {
	Driver              string `json:"driver"`
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	UseTLS              bool   `json:"useTls"`
	InterfaceName       string `json:"interfaceName"`
	Band                string `json:"band"`
	TargetChannel       int    `json:"targetChannel"`
	AllowDFS            bool   `json:"allowDfs"`
	ConfirmedDisconnect bool   `json:"confirmedDisconnect"`
}

// ChannelBackup stores the previous channel state for 1-click rollback.
type ChannelBackup struct {
	Driver          string `json:"driver"`
	Host            string `json:"host"`
	InterfaceName   string `json:"interfaceName"`
	Band            string `json:"band"`
	PreviousChannel int    `json:"previousChannel"`
	AppliedChannel  int    `json:"appliedChannel"`
	SavedAt         string `json:"savedAt"`
}

// ChannelChangeResult summarizes the applied or rolled-back router change and connectivity check.
type ChannelChangeResult struct {
	Success         bool           `json:"success"`
	Message         string         `json:"message"`
	PreviousChannel int            `json:"previousChannel"`
	AppliedChannel  int            `json:"appliedChannel"`
	ConnectivityOK  bool           `json:"connectivityOk"`
	Backup          *ChannelBackup `json:"backup,omitempty"`
}
