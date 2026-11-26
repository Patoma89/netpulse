package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// KnownDevice stores user-assigned metadata and presence history for a discovered LAN device.
type KnownDevice struct {
	Alias          string `json:"alias"`
	Trusted        bool   `json:"trusted"`
	Category       string `json:"category"`
	FirstSeen      string `json:"firstSeen"`
	LastSeen       string `json:"lastSeen"`
	SeenCount      int    `json:"seenCount"`
	Online         bool   `json:"online"`
	LastTransition string `json:"lastTransition"`
}

// NetworkProfile isolates trusted LAN devices and baseline gateway MAC per network environment.
type NetworkProfile struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	GatewayIP   string          `json:"gatewayIp"`
	GatewayMAC  string          `json:"gatewayMac"`
	Subnet      string          `json:"subnet"`
	TrustedMACs map[string]bool `json:"trustedMacs"`
	LastSeen    string          `json:"lastSeen"`
}

// AppConfig holds non-secret local preferences persisted in %APPDATA%\NetPulse\config.json.
type AppConfig struct {
	PrivacyMode          bool                      `json:"privacyMode"`
	Language             string                    `json:"language"`
	Theme                string                    `json:"theme"`
	PingTarget           string                    `json:"pingTarget"`
	KnownDevices         map[string]KnownDevice    `json:"knownDevices"`
	NetworkProfiles      map[string]NetworkProfile `json:"networkProfiles"`
	MinimizeToTray       bool                      `json:"minimizeToTray"`
	BackgroundRadarAlert bool                      `json:"backgroundRadarAlert"`
	StartWithWindows     bool                      `json:"startWithWindows"`
	UpdateRepo           string                    `json:"updateRepo"`
	SSHHostFingerprints  map[string]string         `json:"sshHostFingerprints"`
}

// Store provides thread-safe local persistence for AppConfig.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  AppConfig
}

// DefaultConfig returns safe local-first defaults (Discord Privacy Mode enabled).
func DefaultConfig() AppConfig {
	return AppConfig{
		PrivacyMode:         true,
		Language:            "es",
		Theme:               "dark",
		PingTarget:          "1.1.1.1",
		UpdateRepo:          "Patoma89/netpulse",
		KnownDevices:        make(map[string]KnownDevice),
		NetworkProfiles:     make(map[string]NetworkProfile),
		SSHHostFingerprints: make(map[string]string),
	}
}

// DefaultPath resolves %APPDATA%\NetPulse\config.json (falling back to os.UserConfigDir).
func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		return "", errors.New("could not determine user config directory")
	}
	return filepath.Join(base, "NetPulse", "config.json"), nil
}

// NewStore opens or initializes the config store at the default %APPDATA% path.
func NewStore() (*Store, error) {
	p, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return NewStoreAt(p)
}

// NewStoreAt opens or initializes the config store at a specific file path.
func NewStoreAt(path string) (*Store, error) {
	s := &Store{
		path: path,
		cfg:  DefaultConfig(),
	}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Path returns the backing config file path.
func (s *Store) Path() string {
	return s.path
}

// Load reads config from disk or initializes defaults if the file does not exist yet.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.cfg = DefaultConfig()
			return s.saveLocked()
		}
		return err
	}

	cfg := DefaultConfig()
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
	}
	if cfg.Language != "en" {
		cfg.Language = "es"
	}
	if cfg.Theme != "light" {
		cfg.Theme = "dark"
	}
	if cfg.PingTarget == "" {
		cfg.PingTarget = "1.1.1.1"
	}
	if cfg.UpdateRepo == "" || cfg.UpdateRepo == "netpulse-community/netpulse" {
		cfg.UpdateRepo = "Patoma89/netpulse"
	}
	if cfg.KnownDevices == nil {
		cfg.KnownDevices = make(map[string]KnownDevice)
	}
	if cfg.NetworkProfiles == nil {
		cfg.NetworkProfiles = make(map[string]NetworkProfile)
	}
	for id, prof := range cfg.NetworkProfiles {
		if prof.TrustedMACs == nil {
			prof.TrustedMACs = make(map[string]bool)
			cfg.NetworkProfiles[id] = prof
		}
	}
	if cfg.SSHHostFingerprints == nil {
		cfg.SSHHostFingerprints = make(map[string]string)
	}
	s.cfg = cfg
	return nil
}

// Save writes the current config atomically to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// Get returns a defensive copy of the current AppConfig.
func (s *Store) Get() AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.copyLocked()
}

// SetPrivacyMode updates PrivacyMode and persists it to disk.
func (s *Store) SetPrivacyMode(enabled bool) (AppConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.PrivacyMode = enabled
	if err := s.saveLocked(); err != nil {
		return AppConfig{}, err
	}
	return s.copyLocked(), nil
}

// SetPingTarget validates, updates, and persists the external Dual-Hop ping target.
func (s *Store) SetPingTarget(target string) (AppConfig, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return AppConfig{}, errors.New("ping target cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.PingTarget = trimmed
	if err := s.saveLocked(); err != nil {
		return AppConfig{}, err
	}
	return s.copyLocked(), nil
}

func (s *Store) copyLocked() AppConfig {
	out := s.cfg
	out.KnownDevices = make(map[string]KnownDevice, len(s.cfg.KnownDevices))
	for k, v := range s.cfg.KnownDevices {
		out.KnownDevices[k] = v
	}
	out.NetworkProfiles = make(map[string]NetworkProfile, len(s.cfg.NetworkProfiles))
	for k, v := range s.cfg.NetworkProfiles {
		out.NetworkProfiles[k] = cloneProfile(v)
	}
	out.SSHHostFingerprints = make(map[string]string, len(s.cfg.SSHHostFingerprints))
	for k, v := range s.cfg.SSHHostFingerprints {
		out.SSHHostFingerprints[k] = v
	}
	return out
}

func cloneProfile(p NetworkProfile) NetworkProfile {
	cp := p
	cp.TrustedMACs = make(map[string]bool, len(p.TrustedMACs))
	for mac, trusted := range p.TrustedMACs {
		cp.TrustedMACs[mac] = trusted
	}
	return cp
}

// SetUIPreferences updates and persists language ("es"|"en") and theme ("dark"|"light").
func (s *Store) SetUIPreferences(language, theme string) (AppConfig, error) {
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang != "en" {
		lang = "es"
	}
	th := strings.ToLower(strings.TrimSpace(theme))
	if th != "light" {
		th = "dark"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Language = lang
	s.cfg.Theme = th
	if err := s.saveLocked(); err != nil {
		return AppConfig{}, err
	}
	return s.copyLocked(), nil
}

// SetAutostartPreference updates and persists StartWithWindows preference.
func (s *Store) SetAutostartPreference(enabled bool) (AppConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.StartWithWindows = enabled
	if err := s.saveLocked(); err != nil {
		return AppConfig{}, err
	}
	return s.copyLocked(), nil
}

// RecordNetworkPresence updates active network profile, detects unexpected gateway MAC changes,
// and updates per-device SeenCount / LastSeen / Online transitions.
func (s *Store) RecordNetworkPresence(
	profileID string,
	defaultName string,
	gatewayIP string,
	gatewayMAC string,
	subnet string,
	onlineMACs []string,
) (profile NetworkProfile, gatewayMACChanged bool, previousGatewayMAC string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.NetworkProfiles == nil {
		s.cfg.NetworkProfiles = make(map[string]NetworkProfile)
	}
	if s.cfg.KnownDevices == nil {
		s.cfg.KnownDevices = make(map[string]KnownDevice)
	}

	now := time.Now().Format(time.RFC3339)
	normGW := NormalizeMAC(gatewayMAC)
	id := strings.TrimSpace(profileID)
	if id == "" && normGW != "" {
		id = normGW
	}
	if id == "" && gatewayIP != "" {
		id = "gw:" + strings.TrimSpace(gatewayIP) + "/" + strings.TrimSpace(subnet)
	}

	if id != "" {
		existing, ok := s.cfg.NetworkProfiles[id]
		if !ok {
			name := strings.TrimSpace(defaultName)
			if name == "" {
				name = "Red Local"
			}
			existing = NetworkProfile{
				ID:          id,
				Name:        name,
				GatewayIP:   strings.TrimSpace(gatewayIP),
				GatewayMAC:  normGW,
				Subnet:      strings.TrimSpace(subnet),
				TrustedMACs: make(map[string]bool),
				LastSeen:    now,
			}
		} else {
			if existing.TrustedMACs == nil {
				existing.TrustedMACs = make(map[string]bool)
			}
			if existing.GatewayMAC != "" && normGW != "" && existing.GatewayMAC != normGW {
				gatewayMACChanged = true
				previousGatewayMAC = existing.GatewayMAC
			} else if existing.GatewayMAC == "" && normGW != "" {
				existing.GatewayMAC = normGW
			}
			if gatewayIP != "" {
				existing.GatewayIP = strings.TrimSpace(gatewayIP)
			}
			if subnet != "" {
				existing.Subnet = strings.TrimSpace(subnet)
			}
			existing.LastSeen = now
		}
		s.cfg.NetworkProfiles[id] = existing
		profile = cloneProfile(existing)
	}

	onlineSet := make(map[string]bool, len(onlineMACs))
	for _, m := range onlineMACs {
		norm := NormalizeMAC(m)
		if norm != "" {
			onlineSet[norm] = true
		}
	}

	// Update existing known devices' online state and transitions.
	for mac, dev := range s.cfg.KnownDevices {
		if onlineSet[mac] {
			if dev.FirstSeen == "" {
				dev.FirstSeen = now
			}
			dev.LastSeen = now
			dev.SeenCount++
			if !dev.Online {
				dev.LastTransition = "online @" + time.Now().Format("15:04")
			}
			dev.Online = true
			s.cfg.KnownDevices[mac] = dev
		} else if dev.Online {
			dev.Online = false
			dev.LastTransition = "offline @" + time.Now().Format("15:04")
			s.cfg.KnownDevices[mac] = dev
		}
	}

	// Ensure newly seen devices also get baseline presence entries if not yet tracked.
	for mac := range onlineSet {
		if _, exists := s.cfg.KnownDevices[mac]; !exists {
			s.cfg.KnownDevices[mac] = KnownDevice{
				FirstSeen:      now,
				LastSeen:       now,
				SeenCount:      1,
				Online:         true,
				LastTransition: "online @" + time.Now().Format("15:04"),
			}
		}
	}

	if err = s.saveLocked(); err != nil {
		return NetworkProfile{}, false, "", err
	}
	return profile, gatewayMACChanged, previousGatewayMAC, nil
}

// ConfirmGatewayMAC updates the pinned baseline gateway MAC for a network profile after user verification.
func (s *Store) ConfirmGatewayMAC(profileID, newMAC string) error {
	id := strings.TrimSpace(profileID)
	norm := NormalizeMAC(newMAC)
	if id == "" || norm == "" {
		return errors.New("profile id and gateway MAC are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.NetworkProfiles == nil {
		return errors.New("network profile not found")
	}
	prof, ok := s.cfg.NetworkProfiles[id]
	if !ok {
		return errors.New("network profile not found")
	}
	prof.GatewayMAC = norm
	prof.LastSeen = time.Now().Format(time.RFC3339)
	s.cfg.NetworkProfiles[id] = prof
	return s.saveLocked()
}

// RenameNetworkProfile sets a user-friendly name for a saved network profile (e.g., "Casa" or "Oficina").
func (s *Store) RenameNetworkProfile(profileID, name string) (NetworkProfile, error) {
	id := strings.TrimSpace(profileID)
	trimmedName := strings.TrimSpace(name)
	if id == "" || trimmedName == "" {
		return NetworkProfile{}, errors.New("profile id and name are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.NetworkProfiles == nil {
		s.cfg.NetworkProfiles = make(map[string]NetworkProfile)
	}
	prof, ok := s.cfg.NetworkProfiles[id]
	if !ok {
		prof = NetworkProfile{
			ID:          id,
			TrustedMACs: make(map[string]bool),
		}
	}
	prof.Name = trimmedName
	s.cfg.NetworkProfiles[id] = prof
	if err := s.saveLocked(); err != nil {
		return NetworkProfile{}, err
	}
	return cloneProfile(prof), nil
}

// SetDeviceTrustedForProfile sets or removes trust for a MAC within a specific network profile.
func (s *Store) SetDeviceTrustedForProfile(profileID, mac string, trusted bool) error {
	id := strings.TrimSpace(profileID)
	norm := NormalizeMAC(mac)
	if id == "" || norm == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.NetworkProfiles == nil {
		return nil
	}
	prof, ok := s.cfg.NetworkProfiles[id]
	if !ok {
		return nil
	}
	if prof.TrustedMACs == nil {
		prof.TrustedMACs = make(map[string]bool)
	}
	if trusted {
		prof.TrustedMACs[norm] = true
	} else {
		delete(prof.TrustedMACs, norm)
	}
	s.cfg.NetworkProfiles[id] = prof
	return s.saveLocked()
}

// TrustAllForProfile marks all given MACs as trusted inside the active network profile.
func (s *Store) TrustAllForProfile(profileID string, macs []string) error {
	id := strings.TrimSpace(profileID)
	if id == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.NetworkProfiles == nil {
		return nil
	}
	prof, ok := s.cfg.NetworkProfiles[id]
	if !ok {
		return nil
	}
	if prof.TrustedMACs == nil {
		prof.TrustedMACs = make(map[string]bool)
	}
	for _, m := range macs {
		norm := NormalizeMAC(m)
		if norm != "" {
			prof.TrustedMACs[norm] = true
		}
	}
	s.cfg.NetworkProfiles[id] = prof
	return s.saveLocked()
}

// SetTrayPreferences updates tray minimization and background intruder alert preferences.
func (s *Store) SetTrayPreferences(minimizeToTray, backgroundRadarAlert bool) (AppConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.MinimizeToTray = minimizeToTray
	s.cfg.BackgroundRadarAlert = backgroundRadarAlert
	if err := s.saveLocked(); err != nil {
		return AppConfig{}, err
	}
	return s.copyLocked(), nil
}

// GetSSHHostFingerprint returns the stored TOFU SHA-256 fingerprint for a router host:port.
func (s *Store) GetSSHHostFingerprint(hostPort string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fp, ok := s.cfg.SSHHostFingerprints[strings.TrimSpace(hostPort)]
	return fp, ok && fp != ""
}

// SaveSSHHostFingerprint persists a TOFU SHA-256 SSH host key fingerprint.
func (s *Store) SaveSSHHostFingerprint(hostPort, fp string) error {
	key := strings.TrimSpace(hostPort)
	if key == "" || strings.TrimSpace(fp) == "" {
		return errors.New("host and fingerprint are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.SSHHostFingerprints == nil {
		s.cfg.SSHHostFingerprints = make(map[string]string)
	}
	s.cfg.SSHHostFingerprints[key] = strings.TrimSpace(fp)
	return s.saveLocked()
}

// ClearSSHHostFingerprint removes a stored TOFU SSH host key fingerprint.
func (s *Store) ClearSSHHostFingerprint(hostPort string) error {
	key := strings.TrimSpace(hostPort)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.SSHHostFingerprints == nil {
		return nil
	}
	delete(s.cfg.SSHHostFingerprints, key)
	return s.saveLocked()
}

// NormalizeMAC canonicalizes a MAC address to uppercase colon-separated XX:XX:XX:XX:XX:XX format.
func NormalizeMAC(mac string) string {
	trimmed := strings.TrimSpace(strings.ToUpper(mac))
	if trimmed == "" {
		return ""
	}
	hexOnly := make([]byte, 0, 12)
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') {
			hexOnly = append(hexOnly, c)
		} else if c != ':' && c != '-' && c != '.' {
			return trimmed
		}
	}
	if len(hexOnly) == 12 {
		return string(hexOnly[0:2]) + ":" + string(hexOnly[2:4]) + ":" + string(hexOnly[4:6]) + ":" +
			string(hexOnly[6:8]) + ":" + string(hexOnly[8:10]) + ":" + string(hexOnly[10:12])
	}
	return strings.ReplaceAll(trimmed, "-", ":")
}

// UpsertKnownDevice saves or updates metadata for a LAN device keyed by normalized MAC.
func (s *Store) UpsertKnownDevice(mac string, dev KnownDevice) (AppConfig, error) {
	norm := NormalizeMAC(mac)
	if norm == "" {
		return AppConfig{}, errors.New("mac address cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.KnownDevices == nil {
		s.cfg.KnownDevices = make(map[string]KnownDevice)
	}

	existing, exists := s.cfg.KnownDevices[norm]
	if exists && existing.FirstSeen != "" {
		dev.FirstSeen = existing.FirstSeen
	} else if strings.TrimSpace(dev.FirstSeen) == "" {
		dev.FirstSeen = time.Now().Format(time.RFC3339)
	}

	dev.Alias = strings.TrimSpace(dev.Alias)
	dev.Category = strings.TrimSpace(dev.Category)
	s.cfg.KnownDevices[norm] = dev

	if err := s.saveLocked(); err != nil {
		return AppConfig{}, err
	}
	return s.copyLocked(), nil
}

// TrustDevices batch-marks multiple devices as trusted while preserving existing aliases/categories/firstSeen.
func (s *Store) TrustDevices(devices map[string]KnownDevice) (AppConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.KnownDevices == nil {
		s.cfg.KnownDevices = make(map[string]KnownDevice)
	}

	now := time.Now().Format(time.RFC3339)
	for mac, incoming := range devices {
		norm := NormalizeMAC(mac)
		if norm == "" {
			continue
		}
		existing, exists := s.cfg.KnownDevices[norm]
		merged := existing
		if !exists {
			merged = KnownDevice{
				Alias:     strings.TrimSpace(incoming.Alias),
				Category:  strings.TrimSpace(incoming.Category),
				FirstSeen: strings.TrimSpace(incoming.FirstSeen),
			}
		} else {
			if merged.Alias == "" {
				merged.Alias = strings.TrimSpace(incoming.Alias)
			}
			if merged.Category == "" {
				merged.Category = strings.TrimSpace(incoming.Category)
			}
			if merged.FirstSeen == "" {
				merged.FirstSeen = strings.TrimSpace(incoming.FirstSeen)
			}
		}
		if merged.FirstSeen == "" {
			merged.FirstSeen = now
		}
		merged.Trusted = true
		s.cfg.KnownDevices[norm] = merged
	}

	if err := s.saveLocked(); err != nil {
		return AppConfig{}, err
	}
	return s.copyLocked(), nil
}
