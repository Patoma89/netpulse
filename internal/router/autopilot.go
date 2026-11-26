package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"netpulse/internal/ping"
	"netpulse/internal/vault"
	"netpulse/internal/wifi"
)

// Driver defines a safe Auto-Pilot connector for routers with official APIs/SSH/SOAP.
type Driver interface {
	Name() string
	TestConnection(ctx context.Context, cfg AutoPilotConfig) (AutoPilotStatus, error)
	ApplyChannel(ctx context.Context, cfg AutoPilotConfig, iface, band string, targetCh int) (prevCh int, err error)
}

// HostKeyStore persists TOFU SSH host key fingerprints for OpenWrt routers.
type HostKeyStore interface {
	GetSSHHostFingerprint(hostPort string) (string, bool)
	SaveSSHHostFingerprint(hostPort, fp string) error
}

var valid5GHzChannels = map[int]bool{
	36: true, 40: true, 44: true, 48: true,
	52: true, 56: true, 60: true, 64: true,
	100: true, 104: true, 108: true, 112: true,
	116: true, 120: true, 124: true, 128: true,
	132: true, 136: true, 140: true, 144: true,
	149: true, 153: true, 157: true, 161: true, 165: true,
}

var allowLoopbackForTest bool

func isPrivateHost(host string) bool {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || ip.To4() == nil {
		return false
	}
	return ip.IsPrivate() || (allowLoopbackForTest && ip.IsLoopback())
}

// ValidateChannelChange enforces all safety guardrails before touching router radio state.
func ValidateChannelChange(req ChannelChangeRequest) error {
	if !isPrivateHost(req.Host) {
		return errors.New("host inválido: por seguridad, Auto-Piloto solo opera contra IPs privadas de tu red local")
	}
	if !req.ConfirmedDisconnect {
		return errors.New("debés confirmar que el cambio reinicia la radio Wi-Fi y desconecta dispositivos unos segundos")
	}

	switch req.Band {
	case "2.4GHz":
		if req.TargetChannel < 1 || req.TargetChannel > 11 {
			return fmt.Errorf("canal %d bloqueado en 2.4 GHz (solo canales seguros 1..11)", req.TargetChannel)
		}
	case "5GHz":
		if !valid5GHzChannels[req.TargetChannel] {
			return fmt.Errorf("canal %d inválido en 5 GHz", req.TargetChannel)
		}
		if wifi.IsDFSChannel(req.TargetChannel) && !req.AllowDFS {
			return fmt.Errorf("canal %d es DFS (riesgo de cortes por radar); desbloqueá DFS explícitamente si deseás usarlo", req.TargetChannel)
		}
	default:
		return fmt.Errorf("banda inválida: %q (usá 2.4GHz o 5GHz)", req.Band)
	}

	return nil
}

type vaultRouterPayload struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
}

func vaultKeyForDriver(driver string) string {
	return "router:" + strings.ToLower(strings.TrimSpace(driver))
}

// AutoPilotManager coordinates driver dispatch, DPAPI credential persistence, channel guardrails, and rollback.
type AutoPilotManager struct {
	mu         sync.RWMutex
	vault      *vault.Vault
	pinger     ping.Pinger
	drivers    map[string]Driver
	lastBackup *ChannelBackup
	lastConfig *AutoPilotConfig
}

// NewAutoPilotManager constructs an AutoPilotManager with OpenWrt (TOFU SSH), MikroTik, and TR-064 drivers.
func NewAutoPilotManager(v *vault.Vault, p ping.Pinger, hk HostKeyStore) *AutoPilotManager {
	return &AutoPilotManager{
		vault:  v,
		pinger: p,
		drivers: map[string]Driver{
			"openwrt":  &OpenWrtDriver{HostKeys: hk},
			"mikrotik": &MikroTikDriver{},
			"tr064":    &TR064Driver{},
		},
	}
}

// LoadSavedConfig returns stored non-secret connection fields (host/port/user/useTLS) and HasSavedCreds flag.
// Raw passwords are never returned to the frontend.
func (m *AutoPilotManager) LoadSavedConfig(driver string) AutoPilotConfig {
	cfg := AutoPilotConfig{Driver: driver}
	if m.vault == nil {
		return cfg
	}
	user, rawJSON, err := m.vault.GetSecret(vaultKeyForDriver(driver))
	if err != nil {
		return cfg
	}
	var payload vaultRouterPayload
	if err := json.Unmarshal([]byte(rawJSON), &payload); err == nil {
		cfg.Host = payload.Host
		cfg.Port = payload.Port
		cfg.Username = payload.Username
		cfg.UseTLS = payload.UseTLS
		cfg.HasSavedCreds = payload.Password != ""
	} else {
		cfg.Username = user
		cfg.HasSavedCreds = rawJSON != ""
	}
	return cfg
}

// SaveCredentials stores router credentials encrypted in Windows Credential Manager.
func (m *AutoPilotManager) SaveCredentials(cfg AutoPilotConfig) error {
	if m.vault == nil {
		return errors.New("bóveda de credenciales no disponible")
	}
	payload, err := json.Marshal(vaultRouterPayload{
		Host:     strings.TrimSpace(cfg.Host),
		Port:     cfg.Port,
		Username: strings.TrimSpace(cfg.Username),
		Password: cfg.Password,
		UseTLS:   cfg.UseTLS,
	})
	if err != nil {
		return err
	}
	return m.vault.SetSecret(vaultKeyForDriver(cfg.Driver), cfg.Username, string(payload))
}

// ClearCredentials deletes stored credentials for driver from Windows Credential Manager.
func (m *AutoPilotManager) ClearCredentials(driver string) error {
	if m.vault == nil {
		return nil
	}
	err := m.vault.DeleteSecret(vaultKeyForDriver(driver))
	if errors.Is(err, vault.ErrNotFound) {
		return nil
	}
	return err
}

func (m *AutoPilotManager) resolvePassword(driver, provided string) string {
	if provided != "" || m.vault == nil {
		return provided
	}
	_, rawJSON, err := m.vault.GetSecret(vaultKeyForDriver(driver))
	if err != nil {
		return ""
	}
	var payload vaultRouterPayload
	if err := json.Unmarshal([]byte(rawJSON), &payload); err == nil {
		return payload.Password
	}
	return rawJSON
}

// TestConnection verifies credentials against the router and returns detected Wi-Fi radios.
func (m *AutoPilotManager) TestConnection(ctx context.Context, cfg AutoPilotConfig) (AutoPilotStatus, error) {
	drv, ok := m.drivers[cfg.Driver]
	if !ok {
		return AutoPilotStatus{}, fmt.Errorf("driver no soportado: %q", cfg.Driver)
	}
	cfg.Password = m.resolvePassword(cfg.Driver, cfg.Password)

	status, err := drv.TestConnection(ctx, cfg)
	if err != nil {
		return AutoPilotStatus{}, err
	}

	if cfg.SaveCredentials && cfg.Password != "" {
		_ = m.SaveCredentials(cfg)
	}
	return status, nil
}

// ApplyChannel validates guardrails, backs up current channel, applies the change, and verifies gateway ping.
func (m *AutoPilotManager) ApplyChannel(ctx context.Context, req ChannelChangeRequest) (ChannelChangeResult, error) {
	if err := ValidateChannelChange(req); err != nil {
		return ChannelChangeResult{}, err
	}
	drv, ok := m.drivers[req.Driver]
	if !ok {
		return ChannelChangeResult{}, fmt.Errorf("driver no soportado: %q", req.Driver)
	}

	cfg := AutoPilotConfig{
		Driver:   req.Driver,
		Host:     strings.TrimSpace(req.Host),
		Port:     req.Port,
		Username: strings.TrimSpace(req.Username),
		Password: m.resolvePassword(req.Driver, req.Password),
		UseTLS:   req.UseTLS,
	}

	prevCh, err := drv.ApplyChannel(ctx, cfg, req.InterfaceName, req.Band, req.TargetChannel)
	if err != nil {
		return ChannelChangeResult{}, err
	}

	backup := ChannelBackup{
		Driver:          req.Driver,
		Host:            cfg.Host,
		InterfaceName:   req.InterfaceName,
		Band:            req.Band,
		PreviousChannel: prevCh,
		AppliedChannel:  req.TargetChannel,
		SavedAt:         time.Now().Format(time.RFC3339),
	}

	m.mu.Lock()
	m.lastBackup = &backup
	cfgCopy := cfg
	m.lastConfig = &cfgCopy
	m.mu.Unlock()

	connOK := m.verifyGatewayConnectivity(ctx, cfg.Host)
	msg := fmt.Sprintf("Canal actualizado a %d en %s (%s).", req.TargetChannel, req.InterfaceName, req.Band)
	if !connOK {
		msg += " La radio puede seguir reiniciando; esperá unos segundos o usá Rollback si no reconecta."
	} else {
		msg += " Conectividad con el router verificada."
	}

	backupCopy := backup
	return ChannelChangeResult{
		Success:         true,
		Message:         msg,
		PreviousChannel: prevCh,
		AppliedChannel:  req.TargetChannel,
		ConnectivityOK:  connOK,
		Backup:          &backupCopy,
	}, nil
}

// Rollback restores the previous channel captured before the last ApplyChannel operation.
func (m *AutoPilotManager) Rollback(ctx context.Context) (ChannelChangeResult, error) {
	m.mu.RLock()
	backup := m.lastBackup
	cfgPtr := m.lastConfig
	m.mu.RUnlock()

	if backup == nil || cfgPtr == nil || backup.PreviousChannel <= 0 {
		return ChannelChangeResult{}, errors.New("no hay un canal anterior respaldado para restaurar")
	}

	drv, ok := m.drivers[backup.Driver]
	if !ok {
		return ChannelChangeResult{}, fmt.Errorf("driver no soportado: %q", backup.Driver)
	}

	if _, err := drv.ApplyChannel(ctx, *cfgPtr, backup.InterfaceName, backup.Band, backup.PreviousChannel); err != nil {
		return ChannelChangeResult{}, err
	}

	connOK := m.verifyGatewayConnectivity(ctx, backup.Host)
	return ChannelChangeResult{
		Success:         true,
		Message:         fmt.Sprintf("Rollback completado: canal restaurado a %d en %s.", backup.PreviousChannel, backup.InterfaceName),
		PreviousChannel: backup.AppliedChannel,
		AppliedChannel:  backup.PreviousChannel,
		ConnectivityOK:  connOK,
	}, nil
}

// LastBackup returns a copy of the latest ChannelBackup, if present.
func (m *AutoPilotManager) LastBackup() *ChannelBackup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.lastBackup == nil {
		return nil
	}
	cp := *m.lastBackup
	return &cp
}

func (m *AutoPilotManager) verifyGatewayConnectivity(ctx context.Context, host string) bool {
	if m.pinger == nil {
		return true
	}
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return false
		}
		res := m.pinger.Ping(ctx, host, 900*time.Millisecond)
		if res.Success {
			return true
		}
		time.Sleep(400 * time.Millisecond)
	}
	return false
}
