package router

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"netpulse/internal/wifi"
)

// MikroTikDriver implements RouterOS v7+ REST API wireless channel inspection and updates.
type MikroTikDriver struct{}

func (d *MikroTikDriver) Name() string { return "mikrotik" }

type mikrotikSystemResource struct {
	BoardName string `json:"board-name"`
	Platform  string `json:"platform"`
	Version   string `json:"version"`
}

type mikrotikWirelessIface struct {
	ID           string `json:".id"`
	Name         string `json:"name"`
	Band         string `json:"band"`
	Frequency    string `json:"frequency"`
	ChannelWidth string `json:"channel-width"`
}

func (d *MikroTikDriver) TestConnection(ctx context.Context, cfg AutoPilotConfig) (AutoPilotStatus, error) {
	base, client, err := mikrotikClient(cfg)
	if err != nil {
		return AutoPilotStatus{}, err
	}

	var sysRes mikrotikSystemResource
	if err := mikrotikGetJSON(ctx, client, cfg, base+"/rest/system/resource", &sysRes); err != nil {
		return AutoPilotStatus{}, fmt.Errorf("no se pudo conectar a MikroTik REST API: %w", err)
	}

	ifaces, _, err := fetchMikroTikRadios(ctx, client, cfg, base)
	if err != nil {
		return AutoPilotStatus{}, err
	}

	radios := make([]RadioChannelInfo, 0, len(ifaces))
	for _, w := range ifaces {
		radios = append(radios, parseMikroTikRadio(w))
	}

	model := strings.TrimSpace(sysRes.BoardName)
	if model == "" {
		model = "MikroTik RouterOS"
	}
	if sysRes.Version != "" {
		model = fmt.Sprintf("%s (v%s)", model, sysRes.Version)
	}

	return AutoPilotStatus{
		Connected: true,
		Driver:    d.Name(),
		Model:     model,
		Message:   fmt.Sprintf("Conectado a %s (%d interfaces Wi-Fi).", model, len(radios)),
		Radios:    radios,
	}, nil
}

func (d *MikroTikDriver) ApplyChannel(ctx context.Context, cfg AutoPilotConfig, iface, band string, targetCh int) (int, error) {
	base, client, err := mikrotikClient(cfg)
	if err != nil {
		return 0, err
	}

	freqMHz := wifi.ChannelToCenterFreqMHz(band, targetCh)
	if freqMHz <= 0 {
		return 0, fmt.Errorf("canal %d inválido para banda %s", targetCh, band)
	}

	ifaces, endpoint, err := fetchMikroTikRadios(ctx, client, cfg, base)
	if err != nil {
		return 0, err
	}

	var targetID string
	var prevCh int
	for _, w := range ifaces {
		if w.Name == iface || w.ID == iface {
			targetID = w.ID
			if targetID == "" {
				targetID = w.Name
			}
			prevCh = parseMikroTikRadio(w).CurrentChannel
			break
		}
	}
	if targetID == "" {
		return 0, fmt.Errorf("no se encontró la interfaz inalámbrica %q en MikroTik", iface)
	}

	patchURL := fmt.Sprintf("%s%s/%s", base, endpoint, url.PathEscape(targetID))
	var payload map[string]interface{}
	if endpoint == "/rest/interface/wifi" {
		payload = map[string]interface{}{"channel.frequency": strconv.Itoa(freqMHz)}
	} else {
		payload = map[string]interface{}{"frequency": strconv.Itoa(freqMHz)}
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return prevCh, err
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return prevCh, fmt.Errorf("error enviando PATCH a MikroTik: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msgBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return prevCh, fmt.Errorf("MikroTik rechazó el cambio (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(msgBytes)))
	}
	return prevCh, nil
}

func fetchMikroTikRadios(ctx context.Context, client *http.Client, cfg AutoPilotConfig, base string) ([]mikrotikWirelessIface, string, error) {
	var ifaces []mikrotikWirelessIface
	if err := mikrotikGetJSON(ctx, client, cfg, base+"/rest/interface/wireless", &ifaces); err == nil {
		return ifaces, "/rest/interface/wireless", nil
	}
	if err := mikrotikGetJSON(ctx, client, cfg, base+"/rest/interface/wifi", &ifaces); err == nil {
		return ifaces, "/rest/interface/wifi", nil
	}
	return nil, "", errors.New("no se encontraron interfaces wireless/wifi en RouterOS REST API")
}

func parseMikroTikRadio(w mikrotikWirelessIface) RadioChannelInfo {
	freqInt, _ := strconv.Atoi(strings.TrimSpace(w.Frequency))
	band := "2.4GHz"
	ch := 0
	if freqInt >= 5000 {
		band = "5GHz"
		_, ch, _ = wifi.FrequencyKHzToBandChannel(uint32(freqInt * 1000))
	} else if freqInt >= 2400 {
		_, ch, _ = wifi.FrequencyKHzToBandChannel(uint32(freqInt * 1000))
	} else if strings.Contains(strings.ToLower(w.Band), "5g") {
		band = "5GHz"
	}

	width := 20
	if strings.Contains(w.ChannelWidth, "160") {
		width = 160
	} else if strings.Contains(w.ChannelWidth, "80") {
		width = 80
	} else if strings.Contains(w.ChannelWidth, "40") {
		width = 40
	}

	name := w.Name
	if name == "" {
		name = w.ID
	}
	return RadioChannelInfo{
		InterfaceName:  name,
		Band:           band,
		CurrentChannel: ch,
		WidthMHz:       width,
	}
}

func mikrotikClient(cfg AutoPilotConfig) (string, *http.Client, error) {
	if !isPrivateHost(cfg.Host) {
		return "", nil, errors.New("host inválido: por seguridad, Auto-Piloto solo conecta a IPs privadas de tu LAN")
	}
	scheme := "http"
	port := cfg.Port
	if cfg.UseTLS {
		scheme = "https"
		if port <= 0 {
			port = 443
		}
	} else if port <= 0 {
		port = 80
	}
	base := fmt.Sprintf("%s://%s:%d", scheme, strings.TrimSpace(cfg.Host), port)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Local LAN router self-signed cert
	}
	return base, &http.Client{Timeout: 5 * time.Second, Transport: tr}, nil
}

func mikrotikGetJSON(ctx context.Context, client *http.Client, cfg AutoPilotConfig, rawURL string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}
