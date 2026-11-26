package netinfo

// AdapterInfo describes a Windows network interface retrieved via GetAdaptersAddresses.
type AdapterInfo struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	MAC           string   `json:"mac"`
	IPv4          string   `json:"ipv4"`
	PrefixLen     uint8    `json:"prefixLen"`
	Gateway       string   `json:"gateway"`
	DNSServers    []string `json:"dnsServers"`
	Type          string   `json:"type"` // "wifi" | "ethernet" | "other"
	LinkSpeedMbps uint64   `json:"linkSpeedMbps"`
	IfIndex       uint32   `json:"ifIndex"`
	IsUp          bool     `json:"isUp"`
	IsPrimary     bool     `json:"isPrimary"`
	IsVirtual     bool     `json:"isVirtual"`
}

// NetworkSummary summarizes active local network connectivity and primary gateway details.
type NetworkSummary struct {
	Connected      bool          `json:"connected"`
	Primary        *AdapterInfo  `json:"primary"`
	Adapters       []AdapterInfo `json:"adapters"`
	HasWiFiAdapter bool          `json:"hasWiFiAdapter"`
	GatewayIP      string        `json:"gatewayIp"`
	LocalIP        string        `json:"localIp"`
	AdapterMAC     string        `json:"adapterMac"`
	AdapterName    string        `json:"adapterName"`
	LinkType       string        `json:"linkType"`
	LinkSpeedMbps  uint64        `json:"linkSpeedMbps"`
}
