package wifi

import (
	"testing"
	"unsafe"
)

func TestWlanStructABIAmd64(t *testing.T) {
	if sz := unsafe.Sizeof(wlanInterfaceInfo{}); sz != 532 {
		t.Fatalf("unexpected wlanInterfaceInfo size: got %d, want 532", sz)
	}
	if sz := unsafe.Sizeof(wlanBSSEntry{}); sz != 360 {
		t.Fatalf("unexpected wlanBSSEntry size: got %d, want 360", sz)
	}
	if off := unsafe.Offsetof(wlanBSSEntry{}.UlChCenterFrequency); off != 92 {
		t.Fatalf("unexpected UlChCenterFrequency offset: got %d, want 92", off)
	}
	if off := unsafe.Offsetof(wlanBSSEntry{}.UlIeOffset); off != 352 {
		t.Fatalf("unexpected UlIeOffset offset: got %d, want 352", off)
	}
}

func TestFrequencyKHzToBandChannelAndDFS(t *testing.T) {
	cases := []struct {
		freqKHz uint32
		band    string
		ch      int
	}{
		{2412000, "2.4GHz", 1},
		{2437000, "2.4GHz", 6},
		{2462000, "2.4GHz", 11},
		{2484000, "2.4GHz", 14},
		{5180000, "5GHz", 36},
		{5260000, "5GHz", 52},
		{5500000, "5GHz", 100},
		{5745000, "5GHz", 149},
		{5825000, "5GHz", 165},
		{5975000, "6GHz", 5},
	}

	for _, tc := range cases {
		band, ch, _ := FrequencyKHzToBandChannel(tc.freqKHz)
		if band != tc.band || ch != tc.ch {
			t.Fatalf("FrequencyKHzToBandChannel(%d) = (%q, %d), want (%q, %d)", tc.freqKHz, band, ch, tc.band, tc.ch)
		}
	}

	for _, nonDFS := range []int{36, 48, 149, 165} {
		if IsDFSChannel(nonDFS) {
			t.Fatalf("expected channel %d to be non-DFS", nonDFS)
		}
	}
	for _, dfs := range []int{52, 64, 100, 144} {
		if !IsDFSChannel(dfs) {
			t.Fatalf("expected channel %d to be DFS", dfs)
		}
	}
}

func TestParseIEs(t *testing.T) {
	// 1. HT Operation 40 MHz secondary-above on Ch 6 -> Center 8, Wi-Fi 4
	htBlob := []byte{
		ieHTCaps, 2, 0x02, 0x00,
		ieHTOp, 2, 6, 0x01,
	}
	ht := ParseIEs(htBlob, 6, "2.4GHz", 7, 0x0010)
	if ht.WidthMHz != 40 || ht.CenterChannel != 8 || ht.Generation != "Wi-Fi 4" {
		t.Fatalf("unexpected HT parse: %+v", ht)
	}

	// 2. VHT Operation 80 MHz center 42, Wi-Fi 5
	vhtBlob := []byte{
		ieVHTCaps, 4, 0, 0, 0, 0,
		ieVHTOp, 3, 1, 42, 0,
	}
	vht := ParseIEs(vhtBlob, 36, "5GHz", 8, 0x0010)
	if vht.WidthMHz != 80 || vht.CenterChannel != 42 || vht.Generation != "Wi-Fi 5" {
		t.Fatalf("unexpected VHT parse: %+v", vht)
	}

	// 3. Extension IE 35 (HE / Wi-Fi 6) + RSN WPA3-SAE
	rsnWPA3 := []byte{
		0x01, 0x00, // version 1
		0x00, 0x0F, 0xAC, 0x04, // group cipher CCMP
		0x01, 0x00, // 1 pairwise
		0x00, 0x0F, 0xAC, 0x04,
		0x01, 0x00, // 1 AKM
		0x00, 0x0F, 0xAC, 0x08, // SAE
	}
	heBlob := []byte{ieExtension, 1, extHECaps, ieRSN, byte(len(rsnWPA3))}
	heBlob = append(heBlob, rsnWPA3...)
	he := ParseIEs(heBlob, 149, "5GHz", 9, 0x0010)
	if he.Generation != "Wi-Fi 6" || he.Security != "WPA3" {
		t.Fatalf("unexpected HE/WPA3 parse: %+v", he)
	}

	// 4. Truncated / malformed IE blob must not panic
	malformed := []byte{ieRSN, 40, 0x01, 0x00}
	_ = ParseIEs(malformed, 1, "2.4GHz", 7, 0)
}

func TestAnalyzeSpectrum24GHzTriadAndPhysics(t *testing.T) {
	// Heavy neighbors on Ch 1, Ch 3 (overlap), and Ch 6; Ch 11 completely clean.
	// Own AP on Ch 6 (IsCurrent: true) must NOT penalize Ch 6.
	aps := []AccessPoint{
		{SSID: "MiRed", Band: "2.4GHz", Channel: 6, WidthMHz: 20, RSSI: -40, IsCurrent: true},
		{SSID: "Vecino1", Band: "2.4GHz", Channel: 1, WidthMHz: 20, RSSI: -48},
		{SSID: "Vecino3", Band: "2.4GHz", Channel: 3, WidthMHz: 20, RSSI: -52},
		{SSID: "Vecino6", Band: "2.4GHz", Channel: 6, WidthMHz: 20, RSSI: -50},
	}

	_, _, rec24, _ := AnalyzeSpectrum(aps, "2.4GHz", 6)
	if rec24.RecommendedChannel != 11 {
		t.Fatalf("expected recommended channel 11, got %d (%+v)", rec24.RecommendedChannel, rec24)
	}
	if !rec24.ShouldSwitch {
		t.Fatalf("expected ShouldSwitch=true when Ch 6 is congested vs clean Ch 11")
	}

	// Physics check: 1 strong neighbor (-45 dBm) on Ch 1 vs 4 weak neighbors (-90 dBm) on Ch 6
	physicsAPs := []AccessPoint{
		{SSID: "Fuerte1", Band: "2.4GHz", Channel: 1, WidthMHz: 20, RSSI: -45},
		{SSID: "Debil6a", Band: "2.4GHz", Channel: 6, WidthMHz: 20, RSSI: -90},
		{SSID: "Debil6b", Band: "2.4GHz", Channel: 6, WidthMHz: 20, RSSI: -90},
		{SSID: "Debil6c", Band: "2.4GHz", Channel: 6, WidthMHz: 20, RSSI: -90},
		{SSID: "Debil6d", Band: "2.4GHz", Channel: 6, WidthMHz: 20, RSSI: -90},
		{SSID: "Ocupado11", Band: "2.4GHz", Channel: 11, WidthMHz: 20, RSSI: -50},
	}
	_, _, physRec, _ := AnalyzeSpectrum(physicsAPs, "2.4GHz", 1)
	if physRec.RecommendedChannel != 6 {
		t.Fatalf("expected milliwatt physics to pick Ch 6 (-90 dBm x4) over Ch 1 (-45 dBm), got Ch %d", physRec.RecommendedChannel)
	}
}

func TestAnalyzeSpectrum5GHzAvoidsDFS(t *testing.T) {
	// Even when DFS Ch 100 has 0 neighbors and non-DFS channels have mild neighbors,
	// recommendation MUST choose a non-DFS channel.
	aps := []AccessPoint{
		{SSID: "V36", Band: "5GHz", Channel: 36, CenterChannel: 36, WidthMHz: 20, RSSI: -72},
		{SSID: "V149", Band: "5GHz", Channel: 149, CenterChannel: 149, WidthMHz: 20, RSSI: -84},
	}

	scores5, _, _, _ := AnalyzeSpectrum(aps, "5GHz", 100)
	_ = scores5
	_, s5, _, rec5 := AnalyzeSpectrum(aps, "5GHz", 100)
	if IsDFSChannel(rec5.RecommendedChannel) {
		t.Fatalf("expected non-DFS recommendation, got DFS channel %d", rec5.RecommendedChannel)
	}
	if !rec5.ShouldSwitch {
		t.Fatalf("expected ShouldSwitch=true when currently on DFS channel 100")
	}
	for _, s := range s5 {
		if s.Channel == 100 && !s.IsDFS {
			t.Fatalf("expected channel 100 to be marked IsDFS=true")
		}
	}
}

func TestAnalyzeSpectrum6GHz(t *testing.T) {
	aps := []AccessPoint{
		{SSID: "WiFi6E-Vecino", Band: "6GHz", Channel: 5, CenterChannel: 5, WidthMHz: 80, RSSI: -48},
	}
	scores6, rec6 := AnalyzeSpectrum6GHz(aps, "6GHz", 5)
	if len(scores6) == 0 {
		t.Fatalf("expected 6GHz PSC channel scores")
	}
	if rec6.RecommendedChannel == 5 || !rec6.ShouldSwitch {
		t.Fatalf("expected 6GHz to recommend moving off congested Ch 5, got %+v", rec6)
	}
}
