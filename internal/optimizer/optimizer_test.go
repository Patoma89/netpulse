package optimizer

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"
	"unsafe"
)

func TestShellExecuteInfoWLayout(t *testing.T) {
	if sz := unsafe.Sizeof(shellExecuteInfoW{}); sz != 112 {
		t.Fatalf("unexpected shellExecuteInfoW size on amd64: got %d, want 112", sz)
	}
}

func TestValidateIPv4List(t *testing.T) {
	ips, err := ValidateIPv4List(" 1.1.1.1 ", "1.0.0.1")
	if err != nil || len(ips) != 2 || ips[0] != "1.1.1.1" || ips[1] != "1.0.0.1" {
		t.Fatalf("unexpected valid list: ips=%v err=%v", ips, err)
	}

	if _, err := ValidateIPv4List("1.1.1.1; calc.exe"); err == nil {
		t.Fatalf("expected error on command injection string")
	}
	if _, err := ValidateIPv4List("2606:4700:4700::1111"); err == nil {
		t.Fatalf("expected error for non-IPv4 address")
	}
	if _, err := ValidateIPv4List("", "   "); err == nil {
		t.Fatalf("expected error for empty list")
	}
}

func TestBuildDNSQuery(t *testing.T) {
	pkt := BuildDNSQuery("discord.com", 0x1234)
	if len(pkt) < 20 {
		t.Fatalf("unexpected short DNS query length: %d", len(pkt))
	}
	if binary.BigEndian.Uint16(pkt[0:2]) != 0x1234 {
		t.Fatalf("unexpected TXID")
	}
	if binary.BigEndian.Uint16(pkt[4:6]) != 1 {
		t.Fatalf("expected QDCOUNT=1")
	}
}

func TestDNSBenchmarkerWithMock(t *testing.T) {
	bench := NewDNSBenchmarkerWithQuery(func(ctx context.Context, serverIP, domain string, timeout time.Duration) (float64, error) {
		switch serverIP {
		case "1.1.1.1":
			return 6.2, nil
		case "8.8.8.8":
			return 14.5, nil
		case "192.168.1.1":
			return 28.0, nil
		case "9.9.9.9":
			return 0, errors.New("timeout")
		default:
			return 20.0, nil
		}
	})

	res := bench.Run(context.Background(), "Ethernet", 7, []string{"192.168.1.1"})
	if res.FastestID != "cloudflare" {
		t.Fatalf("expected fastest cloudflare, got %q", res.FastestID)
	}
	if !res.Results[0].Recommended || res.Results[0].Provider.ID != "cloudflare" {
		t.Fatalf("expected first sorted result to be recommended cloudflare, got %+v", res.Results[0])
	}

	foundCurrent := false
	for _, r := range res.Results {
		if r.Provider.ID == "current-isp" && r.Provider.IsCurrent {
			foundCurrent = true
		}
	}
	if !foundCurrent {
		t.Fatalf("expected current-isp entry to be included when custom DNS differs from presets")
	}
}

func TestUPnPParsersAndRiskClassifier(t *testing.T) {
	igdXML := []byte(`<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <friendlyName>Archer AX55</friendlyName>
    <deviceList>
      <device>
        <deviceType>urn:schemas-upnp-org:device:WANDevice:1</deviceType>
        <deviceList>
          <device>
            <deviceType>urn:schemas-upnp-org:device:WANConnectionDevice:1</deviceType>
            <serviceList>
              <service>
                <serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
                <controlURL>/ctl/IPConn</controlURL>
              </service>
            </serviceList>
          </device>
        </deviceList>
      </device>
    </deviceList>
  </device>
</root>`)

	name, st, ctrl, err := ParseIGDDescription(igdXML, "http://192.168.1.1:1900/rootDesc.xml")
	if err != nil {
		t.Fatalf("ParseIGDDescription failed: %v", err)
	}
	if name != "Archer AX55" || st != "urn:schemas-upnp-org:service:WANIPConnection:1" || ctrl != "http://192.168.1.1:1900/ctl/IPConn" {
		t.Fatalf("unexpected IGD parsed values: name=%q st=%q ctrl=%q", name, st, ctrl)
	}

	soapMapping := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetGenericPortMappingEntryResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
      <NewExternalPort>3389</NewExternalPort>
      <NewProtocol>TCP</NewProtocol>
      <NewInternalPort>3389</NewInternalPort>
      <NewInternalClient>192.168.1.40</NewInternalClient>
      <NewEnabled>1</NewEnabled>
      <NewPortMappingDescription>Remote Desktop</NewPortMappingDescription>
      <NewLeaseDuration>0</NewLeaseDuration>
    </u:GetGenericPortMappingEntryResponse>
  </s:Body>
</s:Envelope>`)

	m, err := ParsePortMappingResponse(soapMapping, 0)
	if err != nil {
		t.Fatalf("ParsePortMappingResponse failed: %v", err)
	}
	if m.ExternalPort != 3389 || m.Risk != "high" {
		t.Fatalf("expected high risk for port 3389, got %+v", m)
	}

	if risk, _ := ClassifyPortRisk(51413, "TCP", "Transmission", 0); risk != "medium" {
		t.Fatalf("expected medium risk for permanent described port, got %q", risk)
	}
	if risk, _ := ClassifyPortRisk(3074, "UDP", "Xbox Live", 3600); risk != "low" {
		t.Fatalf("expected low risk for temporary leased port, got %q", risk)
	}
}
