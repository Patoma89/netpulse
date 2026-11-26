package router

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var validUCIIdent = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

// OpenWrtDriver implements safe SSH/UCI wireless configuration with TOFU host key verification.
type OpenWrtDriver struct {
	HostKeys HostKeyStore
}

func (d *OpenWrtDriver) Name() string { return "openwrt" }

func (d *OpenWrtDriver) TestConnection(ctx context.Context, cfg AutoPilotConfig) (AutoPilotStatus, error) {
	client, fp, err := dialOpenWrtSSH(ctx, cfg, d.HostKeys)
	if err != nil {
		return AutoPilotStatus{}, err
	}
	defer client.Close()

	modelOut, _ := runSSHCommand(client, "cat /tmp/sysinfo/model 2>/dev/null || uname -n")
	uciOut, err := runSSHCommand(client, "uci show wireless")
	if err != nil {
		return AutoPilotStatus{}, fmt.Errorf("error leyendo 'uci show wireless': %w", err)
	}

	radios := ParseOpenWrtWirelessUCI(uciOut)
	model := strings.TrimSpace(modelOut)
	if model == "" {
		model = "OpenWrt Router"
	}

	return AutoPilotStatus{
		Connected:       true,
		Driver:          d.Name(),
		Model:           model,
		HostFingerprint: fp,
		Message:         fmt.Sprintf("Conectado a %s (%d radios detectados, huella SSH verificada).", model, len(radios)),
		Radios:          radios,
	}, nil
}

func (d *OpenWrtDriver) ApplyChannel(ctx context.Context, cfg AutoPilotConfig, iface, band string, targetCh int) (int, error) {
	if !validUCIIdent.MatchString(iface) {
		return 0, fmt.Errorf("nombre de radio OpenWrt inválido: %q", iface)
	}

	client, _, err := dialOpenWrtSSH(ctx, cfg, d.HostKeys)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	uciOut, err := runSSHCommand(client, "uci show wireless")
	if err != nil {
		return 0, fmt.Errorf("no se pudo leer configuración actual: %w", err)
	}
	prevCh := 0
	for _, r := range ParseOpenWrtWirelessUCI(uciOut) {
		if r.InterfaceName == iface {
			prevCh = r.CurrentChannel
			break
		}
	}

	cmd := fmt.Sprintf("uci set wireless.%s.channel='%d' && uci commit wireless && wifi reload", iface, targetCh)
	if _, err := runSSHCommand(client, cmd); err != nil {
		return prevCh, fmt.Errorf("error aplicando canal en OpenWrt: %w", err)
	}
	return prevCh, nil
}

// buildTOFUHostKeyCallback verifies the router's SSH host key using Trust-On-First-Use (TOFU).
func buildTOFUHostKeyCallback(hostPort string, store HostKeyStore, capturedFP *string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		if capturedFP != nil {
			*capturedFP = fp
		}
		if store == nil {
			return nil
		}
		saved, exists := store.GetSSHHostFingerprint(hostPort)
		if !exists {
			return store.SaveSSHHostFingerprint(hostPort, fp)
		}
		if saved != fp {
			return fmt.Errorf(
				"alerta de seguridad SSH (TOFU): la huella del router %s cambió (guardada %s, recibida %s). Si actualizaste el firmware, restablecé la huella en NetPulse",
				hostPort, saved, fp,
			)
		}
		return nil
	}
}

// ParseOpenWrtWirelessUCI extracts radio devices, bands, and channels from `uci show wireless`.
func ParseOpenWrtWirelessUCI(output string) []RadioChannelInfo {
	byRadio := make(map[string]*RadioChannelInfo)
	order := make([]string, 0)

	ensure := func(name string) *RadioChannelInfo {
		if r, ok := byRadio[name]; ok {
			return r
		}
		r := &RadioChannelInfo{InterfaceName: name, WidthMHz: 20}
		byRadio[name] = r
		order = append(order, name)
		return r
	}

	isRadioType := make(map[string]bool)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "wireless.") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimPrefix(line[:eq], "wireless.")
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `'"`)

		dot := strings.IndexByte(key, '.')
		if dot < 0 {
			if val == "wifi-device" {
				isRadioType[key] = true
				ensure(key)
			}
			continue
		}

		section := key[:dot]
		option := key[dot+1:]
		r := ensure(section)

		switch option {
		case "band":
			if val == "2g" {
				r.Band = "2.4GHz"
			} else if val == "5g" {
				r.Band = "5GHz"
			}
		case "hwmode":
			if r.Band == "" {
				if strings.Contains(val, "a") {
					r.Band = "5GHz"
				} else {
					r.Band = "2.4GHz"
				}
			}
		case "channel":
			if ch, err := strconv.Atoi(val); err == nil {
				r.CurrentChannel = ch
			}
		case "htmode":
			if strings.Contains(val, "160") {
				r.WidthMHz = 160
			} else if strings.Contains(val, "80") {
				r.WidthMHz = 80
			} else if strings.Contains(val, "40") {
				r.WidthMHz = 40
			}
		}
	}

	out := make([]RadioChannelInfo, 0, len(order))
	for _, name := range order {
		if len(isRadioType) > 0 && !isRadioType[name] {
			continue
		}
		r := byRadio[name]
		if r.Band == "" {
			if r.CurrentChannel >= 36 {
				r.Band = "5GHz"
			} else {
				r.Band = "2.4GHz"
			}
		}
		out = append(out, *r)
	}
	return out
}

func dialOpenWrtSSH(ctx context.Context, cfg AutoPilotConfig, store HostKeyStore) (*ssh.Client, string, error) {
	if !isPrivateHost(cfg.Host) {
		return nil, "", errors.New("host inválido: por seguridad, Auto-Piloto solo conecta a IPs privadas de tu LAN")
	}
	port := cfg.Port
	if port <= 0 {
		port = 22
	}
	user := strings.TrimSpace(cfg.Username)
	if user == "" {
		user = "root"
	}

	addr := net.JoinHostPort(strings.TrimSpace(cfg.Host), strconv.Itoa(port))
	var capturedFP string

	sshCfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		HostKeyCallback: buildTOFUHostKeyCallback(addr, store, &capturedFP),
		Timeout:         5 * time.Second,
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("no se pudo conectar por SSH a %s: %w", addr, err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, sshCfg)
	if err != nil {
		_ = conn.Close()
		return nil, "", fmt.Errorf("falló autenticación o verificación SSH: %w", err)
	}
	return ssh.NewClient(c, chans, reqs), capturedFP, nil
}

func runSSHCommand(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}
