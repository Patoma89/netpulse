package radar

import "strings"

// InferCategory determines the best device category icon/type from gateway status,
// discovery protocols, hostname, and OUI vendor.
func InferCategory(d Device) string {
	if d.IsGateway {
		return "router"
	}
	if d.IsLocal {
		return "pc"
	}

	text := strings.ToLower(strings.Join([]string{
		d.Alias,
		d.SSDPName,
		d.MDNSName,
		d.NetBIOSName,
		d.Hostname,
		d.Vendor,
		strings.Join(d.Services, " "),
	}, " "))

	switch {
	case containsAny(text, "playstation", "ps4", "ps5", "xbox", "nintendo", "switch", "steamdeck", "valve"):
		return "console"
	case containsAny(text, "chromecast", "googlecast", "google tv", "androidtv", "smarttv", "smart-tv", "webos", "tizen", "bravia", "roku", "firetv", "fire tv", "appletv", "apple tv", "airplay", "lg electronics", "samsung tv", "dial"):
		return "tv"
	case containsAny(text, "_printer", "_ipp", "laserjet", "deskjet", "officejet", "canon", "epson", "brother", "printer", "impresora"):
		return "printer"
	case containsAny(text, "router", "gateway", "openwrt", "mikrotik", "ubiquiti", "unifi", "fritz", "sagemcom", "arris", "technicolor", "mitrastar", "sercomm", "hitron", "arcadyan", "fiberhome", "tp-link", "tenda", "internetgateway"):
		return "router"
	case containsAny(text, "espressif", "tuya", "shelly", "hue", "sonos", "alexa", "echo", "camera", "hikvision", "dahua", "wyze", "ring", "esp8266", "esp32", "raspberry", "_hap"):
		return "iot"
	case containsAny(text, "iphone", "ipad", "android", "galaxy", "redmi", "poco", "pixel", "motorola", "xiaomi", "_companion-link"):
		return "phone"
	case d.IsRandomizedMAC:
		return "phone"
	case containsAny(text, "desktop", "laptop", "notebook", "macbook", "imac", "intel", "realtek", "gigabyte", "asrock", "micro-star", "msi", "asustek", "lenovo", "dell", "synology", "qnap", "vmware", "hyper-v", "_smb", "workstation"):
		return "pc"
	case containsAny(text, "apple", "samsung"):
		return "phone"
	default:
		return "unknown"
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}
