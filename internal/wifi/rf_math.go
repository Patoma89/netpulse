package wifi

import (
	"fmt"
	"math"
)

var (
	channels24GHz     = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	cleanTriad24GHz   = map[int]bool{1: true, 6: true, 11: true}
	channels5GHz      = []int{36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140, 144, 149, 153, 157, 161, 165}
	safeNonDFS5GHzMap = map[int]bool{36: true, 40: true, 44: true, 48: true, 149: true, 153: true, 157: true, 161: true, 165: true}
	channels6GHzPSC   = []int{5, 21, 37, 53, 69, 85, 101, 117, 133, 149, 165, 181, 197, 213}
	allowed6GHzPSCMap = map[int]bool{5: true, 21: true, 37: true, 53: true, 69: true, 85: true, 101: true, 117: true, 133: true, 149: true, 165: true, 181: true, 197: true, 213: true}
)

// AnalyzeSpectrum calculates physical milliwatt interference per channel for 2.4 GHz and 5 GHz
// and recommends optimal non-overlapping / non-DFS channels.
func AnalyzeSpectrum(
	aps []AccessPoint,
	currentBand string,
	currentChannel int,
) (scores24 []ChannelScore, scores5 []ChannelScore, rec24 BandRecommendation, rec5 BandRecommendation) {
	var aps24, aps5 []AccessPoint
	for _, ap := range aps {
		if ap.IsCurrent {
			continue
		}
		switch ap.Band {
		case "2.4GHz":
			aps24 = append(aps24, ap)
		case "5GHz":
			aps5 = append(aps5, ap)
		}
	}

	scores24 = score24GHz(aps24, currentBand, currentChannel)
	scores5 = score5GHz(aps5, currentBand, currentChannel)

	rec24 = buildRecommendation("2.4GHz", scores24, cleanTriad24GHz, currentBand, currentChannel)
	markRecommended(scores24, rec24.RecommendedChannel)

	rec5 = buildRecommendation("5GHz", scores5, safeNonDFS5GHzMap, currentBand, currentChannel)
	markRecommended(scores5, rec5.RecommendedChannel)

	return scores24, scores5, rec24, rec5
}

// AnalyzeSpectrum6GHz scores 6 GHz Preferred Scanning Channels (PSC) for Wi-Fi 6E / Wi-Fi 7 radios.
func AnalyzeSpectrum6GHz(aps []AccessPoint, currentBand string, currentChannel int) ([]ChannelScore, BandRecommendation) {
	var aps6 []AccessPoint
	for _, ap := range aps {
		if !ap.IsCurrent && ap.Band == "6GHz" {
			aps6 = append(aps6, ap)
		}
	}

	scores := make([]ChannelScore, 0, len(channels6GHzPSC))
	for _, ch := range channels6GHzPSC {
		candFreq := float64(ChannelToCenterFreqMHz("6GHz", ch))
		candLow := candFreq - 10.0
		candHigh := candFreq + 10.0

		var totalMW float64
		coCount := 0
		overlapCount := 0

		for _, ap := range aps6 {
			apCenterCh := ap.CenterChannel
			if apCenterCh == 0 {
				apCenterCh = ap.Channel
			}
			apCenterFreq := float64(ChannelToCenterFreqMHz("6GHz", apCenterCh))
			if apCenterFreq == 0 {
				apCenterFreq = float64(ap.FrequencyMHz)
			}
			width := float64(ap.WidthMHz)
			if width < 20 {
				width = 20
			}
			apLow := apCenterFreq - width/2.0
			apHigh := apCenterFreq + width/2.0

			overlapMHz := math.Min(candHigh, apHigh) - math.Max(candLow, apLow)
			if overlapMHz <= 0 {
				continue
			}

			weight := overlapMHz / 20.0
			if ap.Channel == ch {
				coCount++
			} else {
				overlapCount++
			}
			totalMW += rssiToMilliwatts(ap.RSSI) * weight
		}

		dbm, cong := aggregateToMetrics(totalMW)
		scores = append(scores, ChannelScore{
			Channel:         ch,
			Band:            "6GHz",
			CongestionPct:   cong,
			QualityScore:    100 - cong,
			InterferenceDBm: dbm,
			CoChannelCount:  coCount,
			OverlapCount:    overlapCount,
			IsDFS:           false,
			IsCurrent:       currentBand == "6GHz" && currentChannel == ch,
		})
	}

	rec := buildRecommendation("6GHz", scores, allowed6GHzPSCMap, currentBand, currentChannel)
	markRecommended(scores, rec.RecommendedChannel)
	return scores, rec
}

func score24GHz(aps []AccessPoint, currentBand string, currentChannel int) []ChannelScore {
	scores := make([]ChannelScore, 0, len(channels24GHz))

	for _, ch := range channels24GHz {
		var totalMW float64
		coCount := 0
		overlapCount := 0

		for _, ap := range aps {
			apCenter := ap.CenterChannel
			if apCenter == 0 {
				apCenter = ap.Channel
			}
			dist := math.Abs(float64(ch - apCenter))

			halfWidthCh := 2.0
			if ap.WidthMHz >= 40 {
				halfWidthCh = 4.0
			}
			reach := halfWidthCh + 2.0 // 4 channels for 20 MHz, 6 channels for 40 MHz

			var weight float64
			if ch == ap.Channel || dist == 0 {
				weight = 1.0
				coCount++
			} else if dist < reach {
				// Partial spectral overlap is destructive due to ACI/non-decoded contention.
				weight = 1.25 * (1.0 - dist/reach)
				overlapCount++
			}

			if weight > 0 {
				totalMW += rssiToMilliwatts(ap.RSSI) * weight
			}
		}

		dbm, cong := aggregateToMetrics(totalMW)
		scores = append(scores, ChannelScore{
			Channel:         ch,
			Band:            "2.4GHz",
			CongestionPct:   cong,
			QualityScore:    100 - cong,
			InterferenceDBm: dbm,
			CoChannelCount:  coCount,
			OverlapCount:    overlapCount,
			IsDFS:           false,
			IsCurrent:       currentBand == "2.4GHz" && currentChannel == ch,
		})
	}

	return scores
}

func score5GHz(aps []AccessPoint, currentBand string, currentChannel int) []ChannelScore {
	scores := make([]ChannelScore, 0, len(channels5GHz))

	for _, ch := range channels5GHz {
		candFreq := float64(ChannelToCenterFreqMHz("5GHz", ch))
		candLow := candFreq - 10.0
		candHigh := candFreq + 10.0

		var totalMW float64
		coCount := 0
		overlapCount := 0

		for _, ap := range aps {
			apCenterCh := ap.CenterChannel
			if apCenterCh == 0 {
				apCenterCh = ap.Channel
			}
			apCenterFreq := float64(ChannelToCenterFreqMHz("5GHz", apCenterCh))
			if apCenterFreq == 0 {
				apCenterFreq = float64(ap.FrequencyMHz)
			}
			width := float64(ap.WidthMHz)
			if width < 20 {
				width = 20
			}
			apLow := apCenterFreq - width/2.0
			apHigh := apCenterFreq + width/2.0

			overlapMHz := math.Min(candHigh, apHigh) - math.Max(candLow, apLow)
			if overlapMHz <= 0 {
				continue
			}

			weight := overlapMHz / 20.0
			if ap.Channel == ch {
				coCount++
			} else {
				overlapCount++
			}
			totalMW += rssiToMilliwatts(ap.RSSI) * weight
		}

		dbm, cong := aggregateToMetrics(totalMW)
		scores = append(scores, ChannelScore{
			Channel:         ch,
			Band:            "5GHz",
			CongestionPct:   cong,
			QualityScore:    100 - cong,
			InterferenceDBm: dbm,
			CoChannelCount:  coCount,
			OverlapCount:    overlapCount,
			IsDFS:           IsDFSChannel(ch),
			IsCurrent:       currentBand == "5GHz" && currentChannel == ch,
		})
	}

	return scores
}

func buildRecommendation(
	band string,
	scores []ChannelScore,
	allowed map[int]bool,
	currentBand string,
	currentChannel int,
) BandRecommendation {
	rec := BandRecommendation{
		Band:   band,
		Status: "no_data",
	}
	if len(scores) == 0 {
		return rec
	}

	bestIdx := -1
	for i, s := range scores {
		if !allowed[s.Channel] {
			continue
		}
		if bestIdx == -1 || s.InterferenceDBm < scores[bestIdx].InterferenceDBm {
			bestIdx = i
		}
	}
	if bestIdx == -1 {
		return rec
	}

	best := scores[bestIdx]
	rec.RecommendedChannel = best.Channel
	rec.BestCongestion = best.CongestionPct

	// Check if user is currently on this band
	if currentBand == band && currentChannel > 0 {
		rec.CurrentChannel = currentChannel
		curScore, found := findChannelScore(scores, currentChannel)
		if found {
			rec.CurrentCongestion = curScore.CongestionPct
		}

		// Keep current channel if it's already an allowed clean channel within 3% congestion of best.
		if allowed[currentChannel] && found && (curScore.CongestionPct-best.CongestionPct) <= 3 {
			rec.RecommendedChannel = currentChannel
			rec.BestCongestion = curScore.CongestionPct
			rec.ImprovementPct = 0
			rec.ShouldSwitch = false
			rec.Status = "optimal"
			rec.Headline = fmt.Sprintf("Canal %d ya es óptimo en %s", currentChannel, band)
			rec.Detail = fmt.Sprintf(
				"Tu red opera en el canal limpio %d con %d%% de congestión vecina. No hace falta cambiarlo.",
				currentChannel, curScore.CongestionPct,
			)
			return rec
		}

		improvement := curScore.CongestionPct - best.CongestionPct
		if improvement < 0 {
			improvement = 0
		}
		rec.ImprovementPct = improvement
		rec.ShouldSwitch = true
		rec.Status = "switch_recommended"
		rec.Headline = fmt.Sprintf("Cambiá del Canal %d al Canal %d (%s)", currentChannel, best.Channel, band)
		if band == "2.4GHz" && !allowed[currentChannel] {
			rec.Detail = fmt.Sprintf(
				"Tu red está en el canal %d (solapado, %d%% congestión). Pasar al canal limpio %d (%d%% congestión) evita interferencia destructiva de canales adyacentes.",
				currentChannel, curScore.CongestionPct, best.Channel, best.CongestionPct,
			)
		} else if band == "5GHz" && IsDFSChannel(currentChannel) {
			rec.Detail = fmt.Sprintf(
				"Tu red usa un canal DFS (%d), expuesto a cortes por radar. El canal seguro no-DFS %d tiene %d%% de congestión.",
				currentChannel, best.Channel, best.CongestionPct,
			)
		} else {
			rec.Detail = fmt.Sprintf(
				"El canal %d tiene %d%% de congestión frente a %d%% en el canal %d (mejora estimada de %d%% menos interferencia RF).",
				currentChannel, curScore.CongestionPct, best.CongestionPct, best.Channel, improvement,
			)
		}
		return rec
	}

	// User is connected on the other band or disconnected: still recommend best channel for this band.
	rec.Status = "optimal"
	rec.Headline = fmt.Sprintf("Mejor canal en %s: Canal %d", band, best.Channel)
	if band == "5GHz" {
		rec.Detail = fmt.Sprintf(
			"Entre los canales seguros sin DFS, el Canal %d es el más despejado (%d%% de congestión estimada).",
			best.Channel, best.CongestionPct,
		)
	} else {
		rec.Detail = fmt.Sprintf(
			"Entre los canales limpios sin solapamiento (1, 6, 11), el Canal %d presenta menor interferencia (%d%% de congestión).",
			best.Channel, best.CongestionPct,
		)
	}
	return rec
}

func markRecommended(scores []ChannelScore, recCh int) {
	for i := range scores {
		scores[i].IsRecommended = scores[i].Channel == recCh
	}
}

func findChannelScore(scores []ChannelScore, ch int) (ChannelScore, bool) {
	for _, s := range scores {
		if s.Channel == ch {
			return s, true
		}
	}
	return ChannelScore{}, false
}

func rssiToMilliwatts(rssi int) float64 {
	if rssi < -100 {
		rssi = -100
	}
	if rssi > -20 {
		rssi = -20
	}
	return math.Pow(10.0, float64(rssi)/10.0)
}

func aggregateToMetrics(totalMW float64) (dbm float64, congestionPct int) {
	if totalMW <= 0 {
		return -95.0, 0
	}
	dbm = 10.0 * math.Log10(totalMW)
	if dbm < -95.0 {
		dbm = -95.0
	}
	dbm = math.Round(dbm*10.0) / 10.0

	// Map -95 dBm (noise floor = 0%) to -35 dBm (saturated = 100%)
	pct := int(math.Round(((dbm + 95.0) / 60.0) * 100.0))
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return dbm, pct
}
