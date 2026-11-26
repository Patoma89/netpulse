package router

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestValidateChannelChangeGuardrails(t *testing.T) {
	// 1. Public IP rejected
	err := ValidateChannelChange(ChannelChangeRequest{
		Host: "8.8.8.8", Band: "2.4GHz", TargetChannel: 6, ConfirmedDisconnect: true,
	})
	if err == nil {
		t.Fatalf("expected public IP 8.8.8.8 to be rejected")
	}

	// 2. Unconfirmed disconnect rejected
	err = ValidateChannelChange(ChannelChangeRequest{
		Host: "192.168.1.1", Band: "2.4GHz", TargetChannel: 6, ConfirmedDisconnect: false,
	})
	if err == nil {
		t.Fatalf("expected unconfirmed disconnect to be rejected")
	}

	// 3. 2.4 GHz channel 13 rejected
	err = ValidateChannelChange(ChannelChangeRequest{
		Host: "192.168.1.1", Band: "2.4GHz", TargetChannel: 13, ConfirmedDisconnect: true,
	})
	if err == nil {
		t.Fatalf("expected 2.4GHz channel 13 to be rejected")
	}

	// 4. 5 GHz DFS channel 100 rejected unless AllowDFS=true
	err = ValidateChannelChange(ChannelChangeRequest{
		Host: "192.168.1.1", Band: "5GHz", TargetChannel: 100, AllowDFS: false, ConfirmedDisconnect: true,
	})
	if err == nil {
		t.Fatalf("expected DFS channel 100 to be rejected when AllowDFS=false")
	}

	err = ValidateChannelChange(ChannelChangeRequest{
		Host: "192.168.1.1", Band: "5GHz", TargetChannel: 100, AllowDFS: true, ConfirmedDisconnect: true,
	})
	if err != nil {
		t.Fatalf("expected DFS channel 100 to be allowed when AllowDFS=true: %v", err)
	}

	// 5. Valid safe channels
	if err := ValidateChannelChange(ChannelChangeRequest{
		Host: "192.168.1.1", Band: "2.4GHz", TargetChannel: 11, ConfirmedDisconnect: true,
	}); err != nil {
		t.Fatalf("expected 2.4GHz Ch 11 valid: %v", err)
	}
	if err := ValidateChannelChange(ChannelChangeRequest{
		Host: "10.0.0.1", Band: "5GHz", TargetChannel: 36, ConfirmedDisconnect: true,
	}); err != nil {
		t.Fatalf("expected 5GHz Ch 36 valid: %v", err)
	}
}

func TestParseOpenWrtWirelessUCI(t *testing.T) {
	sample := `
wireless.radio0=wifi-device
wireless.radio0.type='mac80211'
wireless.radio0.band='2g'
wireless.radio0.channel='6'
wireless.radio0.htmode='HT20'
wireless.default_radio0=wifi-iface
wireless.default_radio0.device='radio0'
wireless.radio1=wifi-device
wireless.radio1.type='mac80211'
wireless.radio1.band='5g'
wireless.radio1.channel='36'
wireless.radio1.htmode='HE80'
`
	radios := ParseOpenWrtWirelessUCI(sample)
	if len(radios) != 2 {
		t.Fatalf("expected 2 radios, got %d (%+v)", len(radios), radios)
	}
	if radios[0].InterfaceName != "radio0" || radios[0].Band != "2.4GHz" || radios[0].CurrentChannel != 6 {
		t.Fatalf("unexpected radio0: %+v", radios[0])
	}
	if radios[1].InterfaceName != "radio1" || radios[1].Band != "5GHz" || radios[1].CurrentChannel != 36 || radios[1].WidthMHz != 80 {
		t.Fatalf("unexpected radio1: %+v", radios[1])
	}
}

func TestMikroTikDriverWithHTTPServer(t *testing.T) {
	var patchedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/system/resource":
			_, _ = w.Write([]byte(`{"board-name":"hAP ax2","version":"7.15"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/interface/wireless":
			_, _ = w.Write([]byte(`[{".id":"*1","name":"wlan1","band":"5ghz-a/n/ac","frequency":"5180","channel-width":"20/40/80mhz"}]`))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/rest/interface/wireless/"):
			b, _ := io.ReadAll(r.Body)
			patchedBody = string(b)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	// 1. Loopback must be blocked by default in production
	drv := &MikroTikDriver{}
	if _, err := drv.TestConnection(context.Background(), AutoPilotConfig{Host: "127.0.0.1", Port: port}); err == nil {
		t.Fatalf("expected loopback 127.0.0.1 to be blocked by private-LAN guardrail")
	}

	// 2. Full TestConnection + ApplyChannel roundtrip against local mock server
	allowLoopbackForTest = true
	t.Cleanup(func() { allowLoopbackForTest = false })

	cfg := AutoPilotConfig{Driver: "mikrotik", Host: "127.0.0.1", Port: port, Username: "admin", Password: "pw"}
	st, err := drv.TestConnection(context.Background(), cfg)
	if err != nil {
		t.Fatalf("MikroTik TestConnection failed: %v", err)
	}
	if !st.Connected || len(st.Radios) != 1 || st.Radios[0].CurrentChannel != 36 {
		t.Fatalf("unexpected MikroTik status: %+v", st)
	}

	prevCh, err := drv.ApplyChannel(context.Background(), cfg, "wlan1", "5GHz", 149)
	if err != nil {
		t.Fatalf("MikroTik ApplyChannel failed: %v", err)
	}
	if prevCh != 36 || !strings.Contains(patchedBody, "5745") {
		t.Fatalf("unexpected MikroTik ApplyChannel: prevCh=%d body=%q", prevCh, patchedBody)
	}
}

func TestTR064DriverWithHTTPServer(t *testing.T) {
	var setChannelBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "GetInfo") {
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetInfoResponse xmlns:u="urn:dslforum-org:service:WLANConfiguration:2">
      <NewChannel>149</NewChannel>
      <NewStandard>a</NewStandard>
    </u:GetInfoResponse>
  </s:Body>
</s:Envelope>`))
			return
		}
		if strings.Contains(string(body), "SetChannel") {
			setChannelBody = string(body)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	allowLoopbackForTest = true
	t.Cleanup(func() { allowLoopbackForTest = false })

	drv := &TR064Driver{}
	cfg := AutoPilotConfig{Driver: "tr064", Host: "127.0.0.1", Port: port, Username: "admin", Password: "pw"}
	st, err := drv.TestConnection(context.Background(), cfg)
	if err != nil {
		t.Fatalf("TR064 TestConnection failed: %v", err)
	}
	if !st.Connected || len(st.Radios) == 0 || st.Radios[0].CurrentChannel != 149 {
		t.Fatalf("unexpected TR064 status: %+v", st)
	}

	prevCh, err := drv.ApplyChannel(context.Background(), cfg, "wlanconfig2", "5GHz", 36)
	if err != nil {
		t.Fatalf("TR064 ApplyChannel failed: %v", err)
	}
	if prevCh != 149 || !strings.Contains(setChannelBody, "<NewChannel>36</NewChannel>") {
		t.Fatalf("unexpected TR064 ApplyChannel: prevCh=%d body=%q", prevCh, setChannelBody)
	}
}

func TestCompareSnapshots(t *testing.T) {
	before := BeforeAfterSnapshot{
		RouterAvgMs:    18.0,
		RouterJitterMs: 6.5,
		WiFiCongestion: 78,
		Score:          65,
	}
	after := BeforeAfterSnapshot{
		RouterAvgMs:    2.2,
		RouterJitterMs: 0.8,
		WiFiCongestion: 12,
		Score:          96,
	}

	rep := CompareSnapshots(before, after)
	if !rep.HasBaseline || rep.DeltaScore != 31 || rep.DeltaCongestion != -66 || rep.DeltaRouterMs != -15.8 {
		t.Fatalf("unexpected comparison report: %+v", rep)
	}
	if !strings.Contains(rep.Verdict, "Mejora confirmada") {
		t.Fatalf("expected improvement verdict, got %q", rep.Verdict)
	}
}

type memHostKeyStore struct {
	keys map[string]string
}

func (m *memHostKeyStore) GetSSHHostFingerprint(hp string) (string, bool) {
	v, ok := m.keys[hp]
	return v, ok
}

func (m *memHostKeyStore) SaveSSHHostFingerprint(hp, fp string) error {
	m.keys[hp] = fp
	return nil
}

func TestOpenWrtTOFUHostKeyVerification(t *testing.T) {
	k1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey 1 failed: %v", err)
	}
	pub1, err := ssh.NewPublicKey(&k1.PublicKey)
	if err != nil {
		t.Fatalf("NewPublicKey 1 failed: %v", err)
	}

	k2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey 2 failed: %v", err)
	}
	pub2, err := ssh.NewPublicKey(&k2.PublicKey)
	if err != nil {
		t.Fatalf("NewPublicKey 2 failed: %v", err)
	}

	store := &memHostKeyStore{keys: make(map[string]string)}
	var captured string
	cb := buildTOFUHostKeyCallback("192.168.1.1:22", store, &captured)

	// First use saves fingerprint
	if err := cb("192.168.1.1", nil, pub1); err != nil {
		t.Fatalf("TOFU first-use failed: %v", err)
	}
	if captured == "" || store.keys["192.168.1.1:22"] != captured {
		t.Fatalf("expected fingerprint to be saved, captured=%q stored=%q", captured, store.keys["192.168.1.1:22"])
	}

	// Second use with same key succeeds
	if err := cb("192.168.1.1", nil, pub1); err != nil {
		t.Fatalf("TOFU matching key failed: %v", err)
	}

	// Mismatched key must be rejected
	if err := cb("192.168.1.1", nil, pub2); err == nil {
		t.Fatalf("expected mismatched SSH host key to be rejected")
	}
}
