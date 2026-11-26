package optimizer

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// UPnPMapping represents an active UPnP IGD port forwarding entry on the router.
type UPnPMapping struct {
	Index          int    `json:"index"`
	ExternalPort   int    `json:"externalPort"`
	InternalPort   int    `json:"internalPort"`
	Protocol       string `json:"protocol"`
	InternalClient string `json:"internalClient"`
	Enabled        bool   `json:"enabled"`
	Description    string `json:"description"`
	LeaseDuration  int    `json:"leaseDuration"`
	Risk           string `json:"risk"` // "high" | "medium" | "low"
	RiskReason     string `json:"riskReason"`
}

// UPnPScanResult holds the router IGD metadata and discovered UPnP port mappings.
type UPnPScanResult struct {
	Status        string        `json:"status"` // "ok" | "upnp_disabled" | "error"
	Message       string        `json:"message"`
	GatewayIP     string        `json:"gatewayIp"`
	DeviceName    string        `json:"deviceName"`
	ServiceType   string        `json:"serviceType"`
	ControlURL    string        `json:"controlUrl"`
	CompletedAt   string        `json:"completedAt"`
	Mappings      []UPnPMapping `json:"mappings"`
	HighRiskCount int           `json:"highRiskCount"`
}

type igdRootXML struct {
	Device igdDeviceXML `xml:"device"`
}

type igdDeviceXML struct {
	FriendlyName string           `xml:"friendlyName"`
	ServiceList  []igdServiceXML  `xml:"serviceList>service"`
	DeviceList   []igdDeviceXML   `xml:"deviceList>device"`
}

type igdServiceXML struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

var highRiskPorts = map[int]string{
	22:   "SSH expuesto",
	23:   "Telnet inseguro expuesto",
	80:   "Panel HTTP expuesto",
	135:  "RPC Windows expuesto",
	137:  "NetBIOS expuesto",
	138:  "NetBIOS expuesto",
	139:  "SMB/NetBIOS expuesto",
	443:  "Panel HTTPS expuesto",
	445:  "SMB Windows expuesto (alto riesgo ransomware)",
	1433: "SQL Server expuesto",
	3306: "MySQL expuesto",
	3389: "Escritorio Remoto (RDP) expuesto a Internet",
	5900: "VNC remoto expuesto",
	8080: "Servicio web alternativo expuesto",
}

// ScanUPnP discovers the local Internet Gateway Device via SSDP and enumerates open UPnP mappings.
func ScanUPnP(ctx context.Context, localIP, gatewayIP string) UPnPScanResult {
	res := UPnPScanResult{
		Status:      "upnp_disabled",
		GatewayIP:   gatewayIP,
		CompletedAt: time.Now().Format(time.RFC3339),
		Mappings:    make([]UPnPMapping, 0),
	}

	locURL, responderIP := discoverIGDLocation(ctx, localIP)
	if locURL == "" {
		res.Message = "Tu router no respondió a UPnP IGD (UPnP desactivado o bloqueado: buena señal de seguridad)."
		return res
	}
	if !isPrivateHTTPURL(locURL, responderIP) {
		res.Status = "error"
		res.Message = "Se ignoró una respuesta SSDP fuera de la red local por seguridad."
		return res
	}

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, locURL, nil)
	if err != nil {
		res.Status = "error"
		res.Message = err.Error()
		return res
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Status = "error"
		res.Message = fmt.Sprintf("No se pudo leer descripción IGD del router: %v", err)
		return res
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	_ = resp.Body.Close()
	if err != nil {
		res.Status = "error"
		res.Message = err.Error()
		return res
	}

	deviceName, serviceType, controlURL, err := ParseIGDDescription(body, locURL)
	if err != nil {
		res.Status = "upnp_disabled"
		res.Message = "El gateway anunció UPnP pero no expone servicio WANIPConnection/WANPPPConnection."
		return res
	}
	if !isPrivateHTTPURL(controlURL, "") {
		res.Status = "error"
		res.Message = "ControlURL UPnP apunta fuera de la red local; bloqueado por seguridad."
		return res
	}

	res.Status = "ok"
	res.DeviceName = deviceName
	res.ServiceType = serviceType
	res.ControlURL = controlURL

	for idx := 0; idx < 64; idx++ {
		if ctx.Err() != nil {
			break
		}
		mapping, ok := fetchPortMappingEntry(ctx, client, controlURL, serviceType, idx)
		if !ok {
			break
		}
		res.Mappings = append(res.Mappings, mapping)
		if mapping.Risk == "high" {
			res.HighRiskCount++
		}
	}

	if len(res.Mappings) == 0 {
		res.Message = "UPnP activo en el router, pero no hay puertos abiertos actualmente."
	} else {
		res.Message = fmt.Sprintf("Se detectaron %d reglas UPnP activas en %s.", len(res.Mappings), deviceName)
	}
	return res
}

// DeleteUPnPMapping closes an open UPnP port forwarding entry on the router via SOAP DeletePortMapping.
func DeleteUPnPMapping(ctx context.Context, controlURL, serviceType, protocol string, externalPort int) error {
	if !isPrivateHTTPURL(controlURL, "") {
		return errors.New("ControlURL inválido o fuera de la red local")
	}
	if serviceType == "" {
		return errors.New("ServiceType UPnP faltante")
	}
	proto := strings.ToUpper(strings.TrimSpace(protocol))
	if proto != "TCP" && proto != "UDP" {
		return fmt.Errorf("protocolo UPnP inválido: %q", protocol)
	}
	if externalPort < 1 || externalPort > 65535 {
		return fmt.Errorf("puerto externo inválido: %d", externalPort)
	}

	soapBody := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:DeletePortMapping xmlns:u="%s">
      <NewRemoteHost></NewRemoteHost>
      <NewExternalPort>%d</NewExternalPort>
      <NewProtocol>%s</NewProtocol>
    </u:DeletePortMapping>
  </s:Body>
</s:Envelope>`, serviceType, externalPort, proto)

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL, strings.NewReader(soapBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#DeletePortMapping"`, serviceType))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error enviando DeletePortMapping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("el router rechazó cerrar el puerto (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// ParseIGDDescription parses an UPnP IGD root XML document to extract friendlyName, WAN serviceType, and resolved controlURL.
func ParseIGDDescription(xmlData []byte, baseURL string) (deviceName, serviceType, controlURL string, err error) {
	var root igdRootXML
	if err := xml.Unmarshal(xmlData, &root); err != nil {
		return "", "", "", err
	}

	deviceName = strings.TrimSpace(root.Device.FriendlyName)
	svc, devName := findWANService(root.Device)
	if svc == nil {
		return deviceName, "", "", errors.New("WANIPConnection/WANPPPConnection service not found")
	}
	if devName != "" && deviceName == "" {
		deviceName = devName
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", "", "", err
	}
	rel, err := url.Parse(strings.TrimSpace(svc.ControlURL))
	if err != nil {
		return "", "", "", err
	}
	resolved := base.ResolveReference(rel).String()
	return deviceName, strings.TrimSpace(svc.ServiceType), resolved, nil
}

func findWANService(dev igdDeviceXML) (*igdServiceXML, string) {
	for i := range dev.ServiceList {
		st := dev.ServiceList[i].ServiceType
		if strings.Contains(st, "WANIPConnection") || strings.Contains(st, "WANPPPConnection") {
			return &dev.ServiceList[i], strings.TrimSpace(dev.FriendlyName)
		}
	}
	for _, child := range dev.DeviceList {
		if svc, name := findWANService(child); svc != nil {
			if name == "" {
				name = strings.TrimSpace(dev.FriendlyName)
			}
			return svc, name
		}
	}
	return nil, ""
}

// ParsePortMappingResponse extracts a UPnPMapping from a SOAP GetGenericPortMappingEntryResponse XML payload.
func ParsePortMappingResponse(xmlData []byte, index int) (UPnPMapping, error) {
	extPortStr := extractXMLTag(xmlData, "NewExternalPort")
	intPortStr := extractXMLTag(xmlData, "NewInternalPort")
	proto := strings.ToUpper(strings.TrimSpace(extractXMLTag(xmlData, "NewProtocol")))
	clientIP := strings.TrimSpace(extractXMLTag(xmlData, "NewInternalClient"))
	enabledStr := strings.TrimSpace(extractXMLTag(xmlData, "NewEnabled"))
	desc := strings.TrimSpace(extractXMLTag(xmlData, "NewPortMappingDescription"))
	leaseStr := strings.TrimSpace(extractXMLTag(xmlData, "NewLeaseDuration"))

	extPort, err := strconv.Atoi(extPortStr)
	if err != nil || extPort <= 0 {
		return UPnPMapping{}, errors.New("missing external port")
	}
	intPort, _ := strconv.Atoi(intPortStr)
	lease, _ := strconv.Atoi(leaseStr)
	enabled := enabledStr == "1" || strings.EqualFold(enabledStr, "true")

	risk, reason := ClassifyPortRisk(extPort, proto, desc, lease)

	return UPnPMapping{
		Index:          index,
		ExternalPort:   extPort,
		InternalPort:   intPort,
		Protocol:       proto,
		InternalClient: clientIP,
		Enabled:        enabled,
		Description:    desc,
		LeaseDuration:  lease,
		Risk:           risk,
		RiskReason:     reason,
	}, nil
}

// ClassifyPortRisk evaluates security risk for an UPnP port forwarding entry.
func ClassifyPortRisk(extPort int, proto, desc string, lease int) (risk, reason string) {
	if msg, risky := highRiskPorts[extPort]; risky {
		return "high", msg
	}
	lowerDesc := strings.ToLower(desc)
	if lease == 0 && (desc == "" || strings.Contains(lowerDesc, "unknown")) {
		return "high", "Regla permanente sin descripción clara"
	}
	if lease == 0 {
		return "medium", "Regla permanente (sin expiración automática)"
	}
	return "low", "Regla temporal con expiración automática"
}

func discoverIGDLocation(ctx context.Context, localIP string) (location string, responderIP string) {
	var bindIP net.IP
	if parsed := net.ParseIP(localIP).To4(); parsed != nil {
		bindIP = parsed
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		return "", ""
	}
	defer conn.Close()

	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	targets := []string{
		"urn:schemas-upnp-org:device:InternetGatewayDevice:1",
		"urn:schemas-upnp-org:device:InternetGatewayDevice:2",
	}
	for _, st := range targets {
		msg := "M-SEARCH * HTTP/1.1\r\n" +
			"HOST: 239.255.255.250:1900\r\n" +
			"MAN: \"ssdp:discover\"\r\n" +
			"MX: 1\r\n" +
			"ST: " + st + "\r\n\r\n"
		_, _ = conn.WriteToUDP([]byte(msg), dst)
	}

	_ = conn.SetReadDeadline(time.Now().Add(950 * time.Millisecond))
	buf := make([]byte, 4096)
	for ctx.Err() == nil {
		n, addr, err := conn.ReadFromUDP(buf)
		if n > 0 && addr != nil {
			for _, line := range strings.Split(string(buf[:n]), "\n") {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 9 && strings.EqualFold(trimmed[:9], "LOCATION:") {
					return strings.TrimSpace(trimmed[9:]), addr.IP.String()
				}
			}
		}
		if err != nil {
			break
		}
	}
	return "", ""
}

func fetchPortMappingEntry(ctx context.Context, client *http.Client, controlURL, serviceType string, index int) (UPnPMapping, bool) {
	soapBody := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetGenericPortMappingEntry xmlns:u="%s">
      <NewPortMappingIndex>%d</NewPortMappingIndex>
    </u:GetGenericPortMappingEntry>
  </s:Body>
</s:Envelope>`, serviceType, index)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL, strings.NewReader(soapBody))
	if err != nil {
		return UPnPMapping{}, false
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#GetGenericPortMappingEntry"`, serviceType))

	resp, err := client.Do(req)
	if err != nil {
		return UPnPMapping{}, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		return UPnPMapping{}, false
	}

	mapping, err := ParsePortMappingResponse(body, index)
	if err != nil {
		return UPnPMapping{}, false
	}
	return mapping, true
}

func isPrivateHTTPURL(rawURL, expectedIP string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	if expectedIP != "" && host != expectedIP {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

func extractXMLTag(data []byte, tag string) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		if se, ok := tok.(xml.StartElement); ok && strings.EqualFold(se.Name.Local, tag) {
			var val string
			if err := dec.DecodeElement(&val, &se); err == nil {
				return val
			}
			return ""
		}
	}
}
