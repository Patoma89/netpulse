package router

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TR064Driver implements SOAP WLANConfiguration control for TR-064 compatible routers (e.g., FRITZ!Box).
type TR064Driver struct{}

func (d *TR064Driver) Name() string { return "tr064" }

var tr064Endpoints = []struct {
	iface   string
	path    string
	service string
	band    string
}{
	{iface: "wlanconfig1", path: "/upnp/control/wlanconfig1", service: "urn:dslforum-org:service:WLANConfiguration:1", band: "2.4GHz"},
	{iface: "wlanconfig2", path: "/upnp/control/wlanconfig2", service: "urn:dslforum-org:service:WLANConfiguration:2", band: "5GHz"},
}

func (d *TR064Driver) TestConnection(ctx context.Context, cfg AutoPilotConfig) (AutoPilotStatus, error) {
	base, client, err := tr064Client(cfg)
	if err != nil {
		return AutoPilotStatus{}, err
	}

	radios := make([]RadioChannelInfo, 0, 2)
	var firstErr error

	for _, ep := range tr064Endpoints {
		ch, bandHint, err := tr064GetInfo(ctx, client, cfg, base+ep.path, ep.service)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		band := ep.band
		if bandHint == "5GHz" || ch >= 36 {
			band = "5GHz"
		} else if bandHint == "2.4GHz" {
			band = "2.4GHz"
		}
		radios = append(radios, RadioChannelInfo{
			InterfaceName:  ep.iface,
			Band:           band,
			CurrentChannel: ch,
			WidthMHz:       20,
		})
	}

	if len(radios) == 0 {
		if firstErr != nil {
			return AutoPilotStatus{}, fmt.Errorf("no se detectó servicio TR-064 WLANConfiguration: %w", firstErr)
		}
		return AutoPilotStatus{}, errors.New("no se detectaron radios WLANConfiguration por TR-064")
	}

	return AutoPilotStatus{
		Connected: true,
		Driver:    d.Name(),
		Model:     "Router TR-064",
		Message:   fmt.Sprintf("Conectado por TR-064 (%d radios WLAN detectados).", len(radios)),
		Radios:    radios,
	}, nil
}

func (d *TR064Driver) ApplyChannel(ctx context.Context, cfg AutoPilotConfig, iface, band string, targetCh int) (int, error) {
	base, client, err := tr064Client(cfg)
	if err != nil {
		return 0, err
	}

	selected := tr064Endpoints[0]
	found := false
	for _, ep := range tr064Endpoints {
		if ep.iface == iface || (iface == "" && ep.band == band) {
			selected = ep
			found = true
			break
		}
	}
	if !found && band == "5GHz" {
		selected = tr064Endpoints[1]
	}

	prevCh, _, _ := tr064GetInfo(ctx, client, cfg, base+selected.path, selected.service)

	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetChannel xmlns:u="%s">
      <NewChannel>%d</NewChannel>
    </u:SetChannel>
  </s:Body>
</s:Envelope>`, selected.service, targetCh)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+selected.path, strings.NewReader(soapBody))
	if err != nil {
		return prevCh, err
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#SetChannel"`, selected.service))

	resp, err := client.Do(req)
	if err != nil {
		return prevCh, fmt.Errorf("error enviando SetChannel TR-064: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return prevCh, fmt.Errorf("TR-064 rechazó SetChannel (HTTP %d)", resp.StatusCode)
	}
	return prevCh, nil
}

func tr064GetInfo(ctx context.Context, client *http.Client, cfg AutoPilotConfig, endpointURL, service string) (int, string, error) {
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetInfo xmlns:u="%s"/>
  </s:Body>
</s:Envelope>`, service)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, strings.NewReader(soapBody))
	if err != nil {
		return 0, "", err
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#GetInfo"`, service))

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	_ = resp.Body.Close()
	if err != nil {
		return 0, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	chStr := extractSOAPTag(body, "NewChannel")
	std := extractSOAPTag(body, "NewStandard")
	ch, _ := strconv.Atoi(strings.TrimSpace(chStr))
	bandHint := ""
	if strings.Contains(strings.ToLower(std), "a") || ch >= 36 {
		bandHint = "5GHz"
	} else if ch >= 1 && ch <= 14 {
		bandHint = "2.4GHz"
	}
	return ch, bandHint, nil
}

func tr064Client(cfg AutoPilotConfig) (string, *http.Client, error) {
	if !isPrivateHost(cfg.Host) {
		return "", nil, errors.New("host inválido: por seguridad, Auto-Piloto solo conecta a IPs privadas de tu LAN")
	}
	scheme := "http"
	port := cfg.Port
	if cfg.UseTLS {
		scheme = "https"
		if port <= 0 {
			port = 49443
		}
	} else if port <= 0 {
		port = 49000
	}
	base := fmt.Sprintf("%s://%s:%d", scheme, strings.TrimSpace(cfg.Host), port)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return base, &http.Client{Timeout: 5 * time.Second, Transport: tr}, nil
}

func extractSOAPTag(data []byte, tag string) string {
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
